package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		"if [ \"$GCM_INTERACTIVE\" != \"Never\" ]; then exit 12; fi\n" +
		"if [ \"$GIT_CONFIG_COUNT\" != \"2\" ]; then exit 13; fi\n" +
		"if [ \"$GIT_CONFIG_KEY_0\" != \"gc.auto\" ]; then exit 14; fi\n" +
		"if [ \"$GIT_CONFIG_VALUE_0\" != \"0\" ]; then exit 15; fi\n" +
		"if [ \"$GIT_CONFIG_KEY_1\" != \"maintenance.auto\" ]; then exit 16; fi\n" +
		"if [ \"$GIT_CONFIG_VALUE_1\" != \"0\" ]; then exit 17; fi\n"
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

func TestRepositorySyncOnceCoalescesConcurrentFetch(t *testing.T) {
	logPath := installFakeGit(t, "if [ \"$cmd\" = \"fetch\" ]; then sleep 0.2; fi\n")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create git dir: %v", err)
	}

	repo := Repository{
		Dir:     dir,
		Remote:  "https://github.com/Aolda/aods-manifest.git",
		Branch:  "main",
		Timeout: time.Second,
		SyncTTL: time.Minute,
	}

	var wg sync.WaitGroup
	errs := make(chan error, 6)
	start := make(chan struct{})
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- repo.syncOnce(context.Background(), false)
		}()
	}

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("syncOnce returned error: %v", err)
		}
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read fake git log: %v", err)
	}
	if got := strings.Count(string(logData), "fetch origin main --prune"); got != 1 {
		t.Fatalf("expected one coalesced fetch, got %d\nlog:\n%s", got, string(logData))
	}
}

func TestRepositoryWithReadUsesStaleSnapshotWhileBackgroundSyncRuns(t *testing.T) {
	logPath := installFakeGit(t, "if [ \"$cmd\" = \"fetch\" ]; then sleep 0.25; fi\n")

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale snapshot: %v", err)
	}

	repo := Repository{
		Dir:         dir,
		Remote:      "https://github.com/Aolda/aods-manifest.git",
		Branch:      "main",
		Timeout:     time.Second,
		SyncTTL:     time.Second,
		lastSyncAt:  time.Now().Add(-2 * time.Second),
		lastSyncKey: "https://github.com/Aolda/aods-manifest.git|main",
	}

	start := time.Now()
	err := repo.WithRead(context.Background(), func(repoDir string) error {
		data, err := os.ReadFile(filepath.Join(repoDir, "snapshot.txt"))
		if err != nil {
			return err
		}
		if string(data) != "stale" {
			t.Fatalf("unexpected snapshot content: %s", data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithRead returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("expected stale read not to wait for background fetch, took %s", elapsed)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if repo.isSyncCacheFresh(filepath.Join(dir, ".git")) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !repo.isSyncCacheFresh(filepath.Join(dir, ".git")) {
		logData, _ := os.ReadFile(logPath)
		t.Fatalf("expected background sync to refresh cache\nlog:\n%s", string(logData))
	}
}

func installFakeGit(t *testing.T, commandBody string) string {
	t.Helper()

	originalPath := os.Getenv("PATH")
	originalLog, hadLog := os.LookupEnv("AODS_FAKE_GIT_LOG")

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "git.log")
	scriptPath := filepath.Join(tempDir, "git")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$*\" >> \"$AODS_FAKE_GIT_LOG\"\n" +
		"cmd=\"$1\"\n" +
		"if [ \"$cmd\" = \"-C\" ]; then cmd=\"$3\"; fi\n" +
		commandBody +
		"exit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	if err := os.Setenv("PATH", tempDir); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	if err := os.Setenv("AODS_FAKE_GIT_LOG", logPath); err != nil {
		t.Fatalf("set fake git log path: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
		if hadLog {
			_ = os.Setenv("AODS_FAKE_GIT_LOG", originalLog)
		} else {
			_ = os.Unsetenv("AODS_FAKE_GIT_LOG")
		}
	})

	return logPath
}
