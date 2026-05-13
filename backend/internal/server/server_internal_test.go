package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/core"
)

func TestDeploymentOperationLockKeyUsesRemoteAndBranch(t *testing.T) {
	t.Parallel()

	base := core.Config{
		GitRemote:  "https://github.com/Aolda/aods-manifest.git",
		GitRepoDir: "/tmp/aods-managed-gitops",
		GitBranch:  "main",
	}
	same := deploymentOperationLockKey(base)
	if !strings.HasPrefix(same, "git:") {
		t.Fatalf("expected git lock key prefix, got %q", same)
	}
	if same != deploymentOperationLockKey(base) {
		t.Fatal("expected lock key to be deterministic")
	}

	otherBranch := base
	otherBranch.GitBranch = "release"
	if same == deploymentOperationLockKey(otherBranch) {
		t.Fatal("expected branch to affect lock key")
	}

	local := base
	local.GitRemote = ""
	if same == deploymentOperationLockKey(local) {
		t.Fatal("expected local repo path fallback to produce a distinct key")
	}
}

func TestMariaDBDSNAddsParseTimeAndUTC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "no query",
			dsn:  "user:pass@tcp(db:3306)/aods",
			want: "user:pass@tcp(db:3306)/aods?parseTime=true&loc=UTC",
		},
		{
			name: "existing query",
			dsn:  "user:pass@tcp(db:3306)/aods?timeout=5s",
			want: "user:pass@tcp(db:3306)/aods?timeout=5s&parseTime=true&loc=UTC",
		},
		{
			name: "already configured",
			dsn:  "user:pass@tcp(db:3306)/aods?parseTime=true&loc=Local",
			want: "user:pass@tcp(db:3306)/aods?parseTime=true&loc=Local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := mariaDBDSN(tt.dsn); got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestHealthHandlerAndMaxDuration(t *testing.T) {
	t.Parallel()

	if got := maxDuration(0, 5*time.Second); got != 5*time.Second {
		t.Fatalf("expected fallback duration, got %s", got)
	}
	if got := maxDuration(2*time.Second, 5*time.Second); got != 2*time.Second {
		t.Fatalf("expected explicit duration, got %s", got)
	}

	response := httptest.NewRecorder()
	healthHandler(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health body, got %s", response.Body.String())
	}
}
