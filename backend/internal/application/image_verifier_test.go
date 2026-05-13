package application

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestParseImageReferenceVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		image      string
		registry   string
		repository string
		identifier string
		scheme     string
	}{
		{image: "nginx", registry: "registry-1.docker.io", repository: "library/nginx", identifier: "latest", scheme: "https"},
		{image: "library/redis:7", registry: "registry-1.docker.io", repository: "library/redis", identifier: "7", scheme: "https"},
		{image: "ghcr.io/aolda/demo:v1", registry: "ghcr.io", repository: "aolda/demo", identifier: "v1", scheme: "https"},
		{image: "localhost:5000/aolda/demo@sha256:abc", registry: "localhost:5000", repository: "aolda/demo", identifier: "sha256:abc", scheme: "https"},
		{image: "127.0.0.1:5000/demo:v1", registry: "127.0.0.1:5000", repository: "demo", identifier: "v1", scheme: "http"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			t.Parallel()

			ref, err := parseImageReference(tt.image)
			if err != nil {
				t.Fatalf("parse image reference: %v", err)
			}
			if ref.Registry != tt.registry || ref.Repository != tt.repository || ref.Identifier != tt.identifier || ref.Scheme != tt.scheme {
				t.Fatalf("unexpected ref: %#v", ref)
			}
		})
	}

	for _, image := range []string{"", "repo:", "repo@", "localhost:5000"} {
		t.Run("invalid "+image, func(t *testing.T) {
			t.Parallel()
			if _, err := parseImageReference(image); err == nil {
				t.Fatalf("expected invalid image %q to fail", image)
			}
		})
	}
}

func TestBearerChallengeAndTokenCredentialValidation(t *testing.T) {
	t.Parallel()

	challenge, ok := parseBearerChallenge(`Bearer realm="https://auth.example/token",service="registry.example",scope="repository:aolda/demo:pull"`)
	if !ok {
		t.Fatal("expected bearer challenge to parse")
	}
	if challenge.realm != "https://auth.example/token" || challenge.service != "registry.example" || challenge.scope != "repository:aolda/demo:pull" {
		t.Fatalf("unexpected challenge: %#v", challenge)
	}
	if _, ok := parseBearerChallenge("Basic realm=example"); ok {
		t.Fatal("expected non-bearer challenge not to parse")
	}

	verifier := RegistryImageVerifier{}
	if _, err := verifier.fetchRegistryToken(t.Context(), challenge, RegistryCredential{Username: "", Password: "token"}); err == nil {
		t.Fatal("expected incomplete registry credentials to fail")
	}
}

func TestRegistryCredentialHelpers(t *testing.T) {
	t.Parallel()

	if !(RegistryCredential{}).matches("ghcr.io") {
		t.Fatal("empty credential server should match any registry")
	}
	if !(RegistryCredential{Server: "https://ghcr.io/"}).matches("ghcr.io") {
		t.Fatal("expected normalized registry server to match")
	}
	if (RegistryCredential{Server: "registry.example"}).matches("ghcr.io") {
		t.Fatal("expected different registry server not to match")
	}
	if normalizeRegistryServer("https://ghcr.io/") != "ghcr.io" {
		t.Fatal("expected registry server normalization")
	}

	encoded, err := buildDockerConfigJSON("https://ghcr.io/", "alice", "secret")
	if err != nil {
		t.Fatalf("build docker config: %v", err)
	}
	if !strings.Contains(encoded, base64.StdEncoding.EncodeToString([]byte("alice:secret"))) {
		t.Fatalf("expected docker auth payload, got %s", encoded)
	}
	for _, input := range []struct {
		server   string
		username string
		password string
	}{
		{server: "", username: "alice", password: "secret"},
		{server: "ghcr.io", username: "", password: "secret"},
		{server: "ghcr.io", username: "alice", password: ""},
	} {
		if _, err := buildDockerConfigJSON(input.server, input.username, input.password); err == nil {
			t.Fatalf("expected invalid docker config input to fail: %#v", input)
		}
	}

	credential, err := registryCredentialFromSecret(map[string]string{
		"server":   "https://ghcr.io/",
		"username": " alice ",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("registry credential from secret: %v", err)
	}
	if credential.Server != "ghcr.io" || credential.Username != "alice" || credential.Password != "secret" {
		t.Fatalf("unexpected credential: %#v", credential)
	}
	if credential, err := registryCredentialFromSecret(nil); err != nil || credential != nil {
		t.Fatalf("expected empty secret to return nil credential, got %#v err=%v", credential, err)
	}
	if _, err := registryCredentialFromSecret(map[string]string{"server": "ghcr.io"}); err == nil {
		t.Fatal("expected incomplete stored credential to fail")
	}
}

func TestImageErrorMapping(t *testing.T) {
	t.Parallel()

	ref := imageReference{Original: "ghcr.io/aolda/demo:v1", Registry: "ghcr.io"}

	notFound := statusToImageError(ref, http.StatusNotFound, nil)
	var imageErr ImageValidationError
	if !errors.As(notFound, &imageErr) || imageErr.Code != "IMAGE_NOT_FOUND" {
		t.Fatalf("expected not found image error, got %v", notFound)
	}

	auth := statusToImageError(ref, http.StatusForbidden, nil)
	if !errors.As(auth, &imageErr) || imageErr.Code != "IMAGE_AUTH_REQUIRED" {
		t.Fatalf("expected auth image error, got %v", auth)
	}

	failure := imageCheckFailure(ref, errors.New("dial timeout"))
	if !errors.As(failure, &imageErr) || imageErr.Code != "IMAGE_CHECK_FAILED" || !strings.Contains(imageErr.Message, "dial timeout") {
		t.Fatalf("expected check failure image error, got %#v", imageErr)
	}

	payload := []byte(`{"errors":[{"code":"MANIFEST_INVALID"}]}`)
	statusFailure := statusToImageError(ref, http.StatusBadGateway, payload)
	if !errors.As(statusFailure, &imageErr) || imageErr.Code != "IMAGE_CHECK_FAILED" || !strings.Contains(imageErr.Message, "MANIFEST_INVALID") {
		t.Fatalf("expected registry error code in message, got %#v", imageErr)
	}
	if code := extractRegistryErrorCode([]byte(`not json`)); code != "" {
		t.Fatalf("expected invalid registry error payload to be ignored, got %q", code)
	}
}
