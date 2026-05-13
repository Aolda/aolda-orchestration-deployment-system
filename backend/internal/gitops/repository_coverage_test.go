package gitops

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositorySyncValidatesRequiredSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		repo func(t *testing.T) *Repository
		want string
	}{
		{
			name: "dir",
			repo: func(t *testing.T) *Repository {
				return &Repository{Remote: "https://example/repo.git", Branch: "main"}
			},
			want: "managed git directory",
		},
		{
			name: "remote",
			repo: func(t *testing.T) *Repository {
				return &Repository{Dir: t.TempDir(), Branch: "main"}
			},
			want: "git remote",
		},
		{
			name: "branch",
			repo: func(t *testing.T) *Repository {
				return &Repository{Dir: t.TempDir(), Remote: "https://example/repo.git"}
			},
			want: "git branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.repo(t).sync(context.Background(), false)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q validation error, got %v", tt.want, err)
			}
		})
	}
}

func TestRepositoryHelperMethods(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		Dir:     filepath.Join(t.TempDir(), "managed"),
		Remote:  "https://user:secret-token@github.com/Aolda/aods-manifest.git",
		Branch:  "main",
		Timeout: 2 * time.Second,
		SyncTTL: time.Minute,
	}
	if repo.effectiveTimeout() != 2*time.Second {
		t.Fatalf("expected explicit timeout, got %s", repo.effectiveTimeout())
	}
	repo.Timeout = 0
	if repo.effectiveTimeout() != 15*time.Second {
		t.Fatalf("expected default timeout, got %s", repo.effectiveTimeout())
	}
	if !strings.HasSuffix(repo.lockFilePath(), ".managed.lock") {
		t.Fatalf("unexpected lock path: %s", repo.lockFilePath())
	}
	if (&Repository{}).lockFilePath() == "" {
		t.Fatal("expected default lock path")
	}
	if repo.syncCacheKey() != repo.Remote+"|main" {
		t.Fatalf("unexpected sync cache key: %s", repo.syncCacheKey())
	}
	if repo.isSyncCacheFresh(filepath.Join(repo.Dir, ".git")) {
		t.Fatal("expected cache to be stale when git dir is missing")
	}

	redacted := redactArgs([]string{"clone", repo.Remote})
	if strings.Contains(redacted, "secret-token") || !strings.Contains(redacted, "%3Credacted%3E") {
		t.Fatalf("expected tokenized remote to be redacted, got %s", redacted)
	}
	if got := redactRemote("https://github.com/Aolda/aods-manifest.git"); got != "https://github.com/Aolda/aods-manifest.git" {
		t.Fatalf("expected remote without credentials to pass through, got %s", got)
	}
	missingFileMessage := (MissingFileError{Path: "platform/projects.yaml"}).Error()
	if !strings.Contains(missingFileMessage, "platform/projects.yaml") {
		t.Fatalf("unexpected missing file error: %s", missingFileMessage)
	}
}
