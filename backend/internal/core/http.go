package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type requestIDKey struct{}

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId"`
	Retryable bool           `json:"retryable,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

var requestSequence atomic.Uint64

var (
	urlCredentialPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)([^/\s:@]+):([^@\s/]+)@`)
	querySecretPattern   = regexp.MustCompile(`(?i)([?&](?:access_token|client_secret|id_token|password|refresh_token|secret|token)=)[^&\s]+`)
	bearerTokenPattern   = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/\-=]+`)
	knownTokenPattern    = regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]+|\bgithub_pat_[A-Za-z0-9_]+|\bhvs\.[A-Za-z0-9._-]+`)
)

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = fmt.Sprintf("req_%d_%06d", time.Now().UTC().UnixMilli(), requestSequence.Add(1))
		}

		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID)))
	})
}

func WithCORS(next http.Handler, allowedOrigin string, allowDevFallback bool) http.Handler {
	configuredOrigins := parseAllowedOrigins(allowedOrigin)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin, ok := selectCORSOrigin(r.Header.Get("Origin"), configuredOrigins, allowDevFallback); ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Headers")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization, X-AODS-User-Id, X-AODS-Username, X-AODS-Display-Name, X-AODS-Groups, X-Request-Id",
		)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		origins = append(origins, trimmed)
	}

	if len(origins) == 0 {
		return []string{"*"}
	}

	return origins
}

func selectCORSOrigin(requestOrigin string, configuredOrigins []string, allowDevFallback bool) (string, bool) {
	origin := strings.TrimSpace(requestOrigin)
	if origin == "" {
		for _, configuredOrigin := range configuredOrigins {
			if configuredOrigin == "*" {
				return "*", true
			}
		}
		return "", false
	}

	for _, configuredOrigin := range configuredOrigins {
		if configuredOrigin == "*" {
			return "*", true
		}
		if configuredOrigin == origin {
			return origin, true
		}
	}

	if allowDevFallback && isLoopbackOrigin(origin) {
		return origin, true
	}

	return "", false
}

func isLoopbackOrigin(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	hostname := strings.TrimSpace(parsed.Hostname())
	return hostname == "localhost" || hostname == "127.0.0.1"
}

func WithNotFoundJSON(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, pattern := mux.Handler(r)
		if pattern == "" {
			WriteError(
				w,
				r,
				http.StatusNotFound,
				"ROUTE_NOT_FOUND",
				"Route was not found.",
				map[string]any{"path": r.URL.Path},
				false,
			)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if body == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	code string,
	message string,
	details map[string]any,
	retryable bool,
) {
	requestID := RequestID(r.Context())
	logHTTPError(r, status, code, message, details, retryable, requestID)

	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: requestID,
			Retryable: retryable,
		},
	})
}

func logHTTPError(
	r *http.Request,
	status int,
	code string,
	message string,
	details map[string]any,
	retryable bool,
	requestID string,
) {
	if status < http.StatusBadRequest {
		return
	}

	attrs := []any{
		"requestId", requestID,
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"code", code,
		"message", message,
		"retryable", retryable,
	}
	if r.RemoteAddr != "" {
		attrs = append(attrs, "remoteAddr", r.RemoteAddr)
	}
	if sanitizedDetails := sanitizeLogDetails(details); len(sanitizedDetails) > 0 {
		attrs = append(attrs, "details", sanitizedDetails)
	}

	if status >= http.StatusInternalServerError {
		slog.Error("api request failed", attrs...)
		return
	}
	slog.Warn("api request rejected", attrs...)
}

func sanitizeLogDetails(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}

	sanitized := make(map[string]any, len(details))
	for key, value := range details {
		if isSensitiveLogKey(key) {
			sanitized[key] = "<redacted>"
			continue
		}
		sanitized[key] = sanitizeLogValue(value)
	}
	return sanitized
}

func sanitizeLogValue(value any) any {
	switch typed := value.(type) {
	case string:
		return truncateLogString(redactLogString(typed))
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, truncateLogString(redactLogString(item)))
		}
		return items
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, sanitizeLogValue(item))
		}
		return items
	case map[string]any:
		return sanitizeLogDetails(typed)
	default:
		return value
	}
}

func isSensitiveLogKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	switch normalized {
	case "authorization", "bearer", "clientsecret", "dockerconfigjson", "idtoken", "password", "refreshtoken", "secret", "token":
		return true
	default:
		return strings.HasSuffix(normalized, "password") || strings.HasSuffix(normalized, "token")
	}
}

func redactLogString(value string) string {
	redacted := urlCredentialPattern.ReplaceAllString(value, "${1}${2}:<redacted>@")
	redacted = querySecretPattern.ReplaceAllString(redacted, "${1}<redacted>")
	redacted = bearerTokenPattern.ReplaceAllString(redacted, "Bearer <redacted>")
	redacted = knownTokenPattern.ReplaceAllString(redacted, "<redacted>")
	return redacted
}

func truncateLogString(value string) string {
	const maxLogStringLength = 2048
	if len(value) <= maxLogStringLength {
		return value
	}
	return value[:maxLogStringLength] + "...<truncated>"
}

func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	if decoder.More() {
		return fmt.Errorf("request body must contain a single JSON object")
	}

	return nil
}
