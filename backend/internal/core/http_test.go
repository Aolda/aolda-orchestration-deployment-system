package core

import (
	"net/http"
	"net/http/httptest"
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
