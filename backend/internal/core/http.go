package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

func WithCORS(next http.Handler, allowedOrigin string) http.Handler {
	origin := allowedOrigin
	if origin == "" {
		origin = "*"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, Authorization, X-AODS-User-Id, X-AODS-Username, X-AODS-Display-Name, X-AODS-Groups, X-Request-Id",
		)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			Details:   details,
			RequestID: RequestID(r.Context()),
			Retryable: retryable,
		},
	})
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
