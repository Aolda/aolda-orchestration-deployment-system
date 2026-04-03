package gitops

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepositoryCommandContextAppliesDefaultTimeout(t *testing.T) {
	t.Parallel()

	repo := Repository{}
	ctx, cancel := repo.commandContext(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected command context deadline")
	}

	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > 15*time.Second {
		t.Fatalf("expected deadline near default timeout, got %s", remaining)
	}
}

func TestRepositoryCommandContextPreservesParentDeadline(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithTimeout(context.Background(), time.Second)
	defer cancelParent()

	repo := Repository{Timeout: 15 * time.Second}
	ctx, cancel := repo.commandContext(parent)
	defer cancel()

	parentDeadline, _ := parent.Deadline()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected command context deadline")
	}
	if !deadline.Equal(parentDeadline) {
		t.Fatalf("expected parent deadline to be preserved, got %s want %s", deadline, parentDeadline)
	}
}

func TestRepositoryRunOutputDisablesInteractiveGitPrompts(t *testing.T) {
	repo := Repository{}
	path := os.Getenv("PATH")
	defer func() {
		_ = os.Setenv("PATH", path)
	}()

	tempDir := t.TempDir()
	scriptPath := tempDir + "/git"
	script := "#!/bin/sh\n" +
		"if [ \"$GIT_TERMINAL_PROMPT\" != \"0\" ]; then exit 11; fi\n" +
		"if [ \"$GCM_INTERACTIVE\" != \"Never\" ]; then exit 12; fi\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	if err := os.Setenv("PATH", tempDir); err != nil {
		t.Fatalf("set PATH: %v", err)
	}

	if err := repo.runOutput(context.Background(), nil, nil, "status"); err != nil {
		t.Fatalf("expected git command env to be non-interactive, got %v", err)
	}
}

func TestRepositoryAcquireProcessLockBlocksConcurrentAccess(t *testing.T) {
	t.Parallel()

	repoA := Repository{Dir: filepath.Join(t.TempDir(), "managed-repo")}
	repoB := Repository{Dir: repoA.Dir}

	lockFile, err := repoA.acquireProcessLock(context.Background())
	if err != nil {
		t.Fatalf("acquire initial lock: %v", err)
	}
	defer func() {
		_ = lockFile.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = repoB.acquireProcessLock(ctx)
	if err == nil {
		t.Fatal("expected concurrent lock acquisition to fail")
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("expected concurrent lock acquisition to wait, got %s", time.Since(start))
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

func TestRepositorySyncCacheFreshness(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("create git dir: %v", err)
	}

	repo := Repository{
		Dir:         dir,
		Remote:      "https://github.com/Aolda/aods-manifest.git",
		Branch:      "main",
		SyncTTL:     3 * time.Second,
		lastSyncAt:  time.Now(),
		lastSyncKey: "https://github.com/Aolda/aods-manifest.git|main",
	}

	if !repo.isSyncCacheFresh(gitDir) {
		t.Fatal("expected sync cache to be fresh")
	}

	repo.lastSyncAt = time.Now().Add(-5 * time.Second)
	if repo.isSyncCacheFresh(gitDir) {
		t.Fatal("expected stale sync cache to be invalid")
	}

	repo.lastSyncAt = time.Now()
	repo.lastSyncKey = "https://github.com/Aolda/aods-manifest.git|staging"
	if repo.isSyncCacheFresh(gitDir) {
		t.Fatal("expected branch mismatch to invalidate sync cache")
	}
}
