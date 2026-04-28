package core

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithCORSEchoesMatchingConfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "http://localhost:5173,http://localhost:5174", false)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	request.Header.Set("Origin", "http://localhost:5174")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestWithCORSAllowsLoopbackOriginDuringDevFallback(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "http://localhost:5173", true)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	request.Header.Set("Origin", "http://localhost:5175")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5175" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestWithCORSRejectsUnexpectedOriginWithoutDevFallback(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "http://localhost:5173", false)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	request.Header.Set("Origin", "http://malicious.example")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow origin, got %q", got)
	}
}

func TestWithCORSFallsBackToWildcardWhenConfigured(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "", false)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	request.Header.Set("Origin", "http://localhost:5174")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestWriteErrorLogsRequestContextAndRedactsSecrets(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(
			w,
			r,
			http.StatusInternalServerError,
			"INTEGRATION_ERROR",
			"An unexpected integration error occurred.",
			map[string]any{
				"error": "git clone https://x-access-token:ghp_secret123@github.com/Aolda/aods-manifest.git failed with Bearer hvs.vaultSecret",
				"token": "should-not-appear",
			},
			true,
		)
	}))

	request := httptest.NewRequest(http.MethodPost, "/api/v1/applications/shared__app/sync", nil)
	request.Header.Set("X-Request-Id", "req-test")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.RequestID != "req-test" {
		t.Fatalf("expected response request id, got %q", response.Error.RequestID)
	}

	output := logs.String()
	for _, expected := range []string{
		"api request failed",
		"requestId=req-test",
		"method=POST",
		"path=/api/v1/applications/shared__app/sync",
		"status=500",
		"code=INTEGRATION_ERROR",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %s", expected, output)
		}
	}
	for _, leaked := range []string{"ghp_secret123", "hvs.vaultSecret", "should-not-appear"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("expected log to redact %q, got %s", leaked, output)
		}
	}
}
