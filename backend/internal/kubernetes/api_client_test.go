package kubernetes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIClientJSONTextPatchAndStream(t *testing.T) {
	t.Parallel()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("expected bearer token header, got %q", got)
		}
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true}`))
		case "/text":
			w.Write([]byte("plain text"))
		case "/patch":
			if r.Method != http.MethodPatch {
				t.Fatalf("expected PATCH, got %s", r.Method)
			}
			if got := r.Header.Get("Content-Type"); got != "application/merge-patch+json" {
				t.Fatalf("expected merge patch content type, got %q", got)
			}
			w.Write([]byte(`{"patched":true}`))
		case "/stream":
			w.Write([]byte("one\ntwo\n"))
		case "/empty":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "missing", http.StatusNotFound)
		}
	}))
	defer api.Close()

	client := &apiClient{
		BaseURL: api.URL,
		BearerTokenProvider: func(context.Context) (string, error) {
			return "test-token", nil
		},
		HTTPClient: api.Client(),
	}

	var jsonBody struct {
		OK bool `json:"ok"`
	}
	if err := client.GetJSON(context.Background(), "/json", &jsonBody); err != nil {
		t.Fatalf("get json: %v", err)
	}
	if !jsonBody.OK {
		t.Fatal("expected json response to decode")
	}

	text, err := client.GetText(context.Background(), "/text")
	if err != nil {
		t.Fatalf("get text: %v", err)
	}
	if text != "plain text" {
		t.Fatalf("unexpected text response: %q", text)
	}

	var patchBody struct {
		Patched bool `json:"patched"`
	}
	if err := client.PatchJSON(context.Background(), "/patch", []byte(`{"spec":{"paused":false}}`), &patchBody); err != nil {
		t.Fatalf("patch json: %v", err)
	}
	if !patchBody.Patched {
		t.Fatal("expected patch response to decode")
	}
	if err := client.PatchJSON(context.Background(), "/empty", []byte(`{}`), nil); err != nil {
		t.Fatalf("patch empty response: %v", err)
	}

	lines := []string{}
	if err := client.StreamText(context.Background(), "/stream", func(line string) error {
		lines = append(lines, line)
		return nil
	}); err != nil {
		t.Fatalf("stream text: %v", err)
	}
	if strings.Join(lines, ",") != "one,two" {
		t.Fatalf("unexpected stream lines: %#v", lines)
	}
}

func TestAPIClientErrors(t *testing.T) {
	t.Parallel()

	var nilClient *apiClient
	if err := nilClient.GetJSON(context.Background(), "/json", nil); err == nil {
		t.Fatal("expected nil client GetJSON to fail")
	}
	if _, err := nilClient.GetText(context.Background(), "/text"); err == nil {
		t.Fatal("expected nil client GetText to fail")
	}
	if err := nilClient.StreamText(context.Background(), "/stream", func(string) error { return nil }); err == nil {
		t.Fatal("expected nil client StreamText to fail")
	}

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad-json":
			w.Write([]byte(`{`))
		case "/not-found":
			http.Error(w, "not here", http.StatusNotFound)
		case "/stream-error":
			http.Error(w, "stream missing", http.StatusNotFound)
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer api.Close()

	client := &apiClient{
		BaseURL: api.URL,
		BearerTokenProvider: func(context.Context) (string, error) {
			return "", errors.New("token unavailable")
		},
		HTTPClient: api.Client(),
	}
	if err := client.GetJSON(context.Background(), "/json", nil); err == nil || !strings.Contains(err.Error(), "token unavailable") {
		t.Fatalf("expected token error, got %v", err)
	}

	client.BearerTokenProvider = nil
	var body map[string]any
	if err := client.GetJSON(context.Background(), "/bad-json", &body); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
	err := client.GetJSON(context.Background(), "/not-found", nil)
	var apiErr apiRequestError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound || !strings.Contains(apiErr.Message, "not here") {
		t.Fatalf("expected api request error, got %v", err)
	}
	if _, err := client.GetText(context.Background(), "/not-found"); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected text api request error, got %v", err)
	}
	if err := client.StreamText(context.Background(), "/stream-error", func(string) error { return nil }); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected stream api request error, got %v", err)
	}
	if err := client.StreamText(context.Background(), "/stream", func(string) error { return errors.New("emit failed") }); err == nil || !strings.Contains(err.Error(), "emit failed") {
		t.Fatalf("expected stream callback error, got %v", err)
	}
}
