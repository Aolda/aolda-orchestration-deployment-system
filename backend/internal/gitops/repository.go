package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

var ErrRepositoryLockBusy = errors.New("managed git repository lock is busy")

type nonBlockingLockContextKey struct{}

func WithNonBlockingLock(ctx context.Context) context.Context {
	return context.WithValue(ctx, nonBlockingLockContextKey{}, true)
}

type Repository struct {
	Dir         string
	Remote      string
	Branch      string
	AuthorName  string
	AuthorEmail string
	Timeout     time.Duration
	SyncTTL     time.Duration

	mu sync.RWMutex

	syncMu       sync.Mutex
	syncInFlight chan struct{}
	lastSyncErr  error

	lastSyncAt  time.Time
	lastSyncKey string
}

type MissingFileError struct {
	Path string
}

func (e MissingFileError) Error() string {
	return fmt.Sprintf("managed repository is missing required file %s", e.Path)
}

func (r *Repository) WithRead(ctx context.Context, fn func(repoDir string) error) error {
	return r.withRead(ctx, true, fn)
}

func (r *Repository) WithReadIfAvailable(ctx context.Context, fn func(repoDir string) error) error {
	return r.withRead(ctx, false, fn)
}

func (r *Repository) withRead(ctx context.Context, waitForLock bool, fn func(repoDir string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	waitForLock = shouldWaitForLock(ctx, waitForLock)

	if err := r.ensureReadSnapshot(ctx, waitForLock); err != nil {
		return err
	}

	unlock, err := r.acquireReadLock(ctx, waitForLock)
	if err != nil {
		return err
	}
	defer unlock()

	return fn(r.Dir)
}

func (r *Repository) WithWrite(
	ctx context.Context,
	commitMessage string,
	fn func(repoDir string) error,
) error {
	return r.withWrite(ctx, commitMessage, true, fn)
}

func (r *Repository) WithWriteIfAvailable(
	ctx context.Context,
	commitMessage string,
	fn func(repoDir string) error,
) error {
	return r.withWrite(ctx, commitMessage, false, fn)
}

func (r *Repository) withWrite(
	ctx context.Context,
	commitMessage string,
	waitForLock bool,
	fn func(repoDir string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	waitForLock = shouldWaitForLock(ctx, waitForLock)

	return r.withProcessLock(ctx, waitForLock, func() error {
		if err := r.acquireInProcessLock(ctx, waitForLock); err != nil {
			return err
		}
		defer r.mu.Unlock()

		if err := r.syncLocked(ctx, true); err != nil {
			return err
		}

		if err := fn(r.Dir); err != nil {
			return err
		}

		changed, err := r.hasChanges(ctx)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}

		if err := r.run(ctx, "-C", r.Dir, "add", "--all"); err != nil {
			return err
		}
		if err := r.run(
			ctx,
			"-C", r.Dir,
			"-c", "user.name="+r.AuthorName,
			"-c", "user.email="+r.AuthorEmail,
			"commit",
			"-m", commitMessage,
		); err != nil {
			return err
		}
		if err := r.run(ctx, "-C", r.Dir, "push", "origin", "HEAD:"+r.Branch); err != nil {
			return err
		}

		r.markSynced()
		return nil
	})
}

func (r *Repository) EnsureFile(ctx context.Context, relativePath string) error {
	if err := r.syncOnce(ctx, false, true); err != nil {
		return err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return func(repoDir string) error {
		path := filepath.Join(repoDir, filepath.Clean(relativePath))
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return MissingFileError{Path: relativePath}
		}
		if err != nil {
			return fmt.Errorf("stat required file %s: %w", relativePath, err)
		}
		if info.IsDir() {
			return MissingFileError{Path: relativePath}
		}
		return nil
	}(r.Dir)
}

func (r *Repository) sync(ctx context.Context, force bool) error {
	return r.syncOnce(ctx, force, true)
}

func (r *Repository) ensureReadSnapshot(ctx context.Context, waitForLock bool) error {
	if err := r.validateSettings(); err != nil {
		return err
	}

	gitDir := filepath.Join(r.Dir, ".git")
	if r.isSyncCacheFresh(gitDir) {
		return nil
	}
	if !r.hasLocalSnapshot(gitDir) {
		return r.syncOnce(ctx, false, waitForLock)
	}

	r.startBackgroundSync()
	return nil
}

func (r *Repository) syncOnce(ctx context.Context, force bool, waitForLock bool) error {
	if err := r.validateSettings(); err != nil {
		return err
	}

	gitDir := filepath.Join(r.Dir, ".git")
	for {
		r.syncMu.Lock()
		if !force && r.isSyncCacheFreshLocked(gitDir) {
			r.syncMu.Unlock()
			return nil
		}
		if r.syncInFlight != nil {
			inFlight := r.syncInFlight
			r.syncMu.Unlock()

			if !waitForLock {
				return fmt.Errorf("%w: sync in progress", ErrRepositoryLockBusy)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-inFlight:
			}

			if err := r.lastSyncError(); err != nil && !r.hasLocalSnapshot(gitDir) {
				return err
			}
			continue
		}

		inFlight := make(chan struct{})
		r.syncInFlight = inFlight
		r.syncMu.Unlock()

		err := r.withProcessLock(ctx, waitForLock, func() error {
			if err := r.acquireInProcessLock(ctx, waitForLock); err != nil {
				return err
			}
			defer r.mu.Unlock()
			return r.syncLocked(ctx, force)
		})
		r.finishSync(inFlight, err)
		return err
	}
}

func (r *Repository) startBackgroundSync() {
	gitDir := filepath.Join(r.Dir, ".git")

	r.syncMu.Lock()
	if r.syncInFlight != nil || r.isSyncCacheFreshLocked(gitDir) {
		r.syncMu.Unlock()
		return
	}
	inFlight := make(chan struct{})
	r.syncInFlight = inFlight
	r.syncMu.Unlock()

	go func() {
		ctx, cancel := r.backgroundSyncContext()
		defer cancel()

		err := r.withProcessLock(ctx, false, func() error {
			if !r.hasLocalSnapshot(gitDir) {
				if err := r.acquireInProcessLock(ctx, false); err != nil {
					return err
				}
				defer r.mu.Unlock()
				return r.syncLocked(ctx, false)
			}

			if err := r.syncRemote(ctx); err != nil {
				return err
			}

			if err := r.acquireInProcessLock(ctx, true); err != nil {
				return err
			}
			defer r.mu.Unlock()
			return r.refreshWorktree(ctx)
		})
		r.finishSync(inFlight, err)
	}()
}

func (r *Repository) finishSync(inFlight chan struct{}, err error) {
	r.syncMu.Lock()
	r.lastSyncErr = err
	if r.syncInFlight == inFlight {
		r.syncInFlight = nil
	}
	close(inFlight)
	r.syncMu.Unlock()
}

func (r *Repository) lastSyncError() error {
	r.syncMu.Lock()
	defer r.syncMu.Unlock()
	return r.lastSyncErr
}

func (r *Repository) validateSettings() error {
	if strings.TrimSpace(r.Dir) == "" {
		return fmt.Errorf("managed git directory is required when git mode is enabled")
	}
	if strings.TrimSpace(r.Remote) == "" {
		return fmt.Errorf("git remote is required when git mode is enabled")
	}
	if strings.TrimSpace(r.Branch) == "" {
		return fmt.Errorf("git branch is required when git mode is enabled")
	}
	return nil
}

func (r *Repository) syncLocked(ctx context.Context, force bool) error {
	gitDir := filepath.Join(r.Dir, ".git")
	if !force && r.isSyncCacheFresh(gitDir) {
		return nil
	}
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if err := os.RemoveAll(r.Dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("reset managed git directory: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(r.Dir), 0o755); err != nil {
			return fmt.Errorf("create managed git parent directory: %w", err)
		}
		if err := r.run(ctx, "clone", "--branch", r.Branch, "--single-branch", r.Remote, r.Dir); err != nil {
			return err
		}
	}

	if err := r.syncRemote(ctx); err != nil {
		return err
	}
	if err := r.refreshWorktree(ctx); err != nil {
		return err
	}

	return nil
}

func (r *Repository) syncRemote(ctx context.Context) error {
	if err := r.run(ctx, "-C", r.Dir, "remote", "set-url", "origin", r.Remote); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "fetch", "origin", r.Branch, "--prune"); err != nil {
		return err
	}
	return nil
}

func (r *Repository) refreshWorktree(ctx context.Context) error {
	if err := r.run(ctx, "-C", r.Dir, "checkout", "-B", r.Branch, "origin/"+r.Branch); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "reset", "--hard", "origin/"+r.Branch); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "clean", "-fd"); err != nil {
		return err
	}

	r.markSynced()
	return nil
}

func (r *Repository) hasChanges(ctx context.Context) (bool, error) {
	var stdout bytes.Buffer
	if err := r.runOutput(ctx, &stdout, nil, "-C", r.Dir, "status", "--porcelain"); err != nil {
		return false, err
	}
	return strings.TrimSpace(stdout.String()) != "", nil
}

func (r *Repository) run(ctx context.Context, args ...string) error {
	return r.runOutput(ctx, nil, nil, args...)
}

func (r *Repository) runOutput(
	ctx context.Context,
	stdout *bytes.Buffer,
	stderr *bytes.Buffer,
	args ...string,
) error {
	execCtx, cancel := r.commandContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=Never",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=gc.auto",
		"GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=maintenance.auto",
		"GIT_CONFIG_VALUE_1=0",
	)

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	if stdout != nil {
		cmd.Stdout = stdout
	} else {
		cmd.Stdout = &stdoutBuffer
	}
	if stderr != nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = &stderrBuffer
	}

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderrBuffer.String())
		if message == "" {
			message = strings.TrimSpace(stdoutBuffer.String())
		}
		redactedArgs := redactArgs(args)
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git %s timed out after %s", redactedArgs, r.effectiveTimeout())
		}
		if message != "" {
			return fmt.Errorf("git %s: %s", redactedArgs, message)
		}
		return fmt.Errorf("git %s: %w", redactedArgs, err)
	}

	return nil
}

func (r *Repository) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || r.effectiveTimeout() <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, r.effectiveTimeout())
}

func (r *Repository) backgroundSyncContext() (context.Context, context.CancelFunc) {
	timeout := r.effectiveTimeout()
	if timeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), timeout)
}

func shouldWaitForLock(ctx context.Context, waitForLock bool) bool {
	if !waitForLock {
		return false
	}
	nonBlocking, _ := ctx.Value(nonBlockingLockContextKey{}).(bool)
	return !nonBlocking
}

func (r *Repository) acquireReadLock(ctx context.Context, waitForLock bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.mu.TryRLock() {
		return r.mu.RUnlock, nil
	}
	if !waitForLock {
		return nil, fmt.Errorf("%w: in-process lock", ErrRepositoryLockBusy)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if r.mu.TryRLock() {
				return r.mu.RUnlock, nil
			}
		}
	}
}

func (r *Repository) acquireInProcessLock(ctx context.Context, waitForLock bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.mu.TryLock() {
		return nil
	}
	if !waitForLock {
		return fmt.Errorf("%w: in-process lock", ErrRepositoryLockBusy)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if r.mu.TryLock() {
				return nil
			}
		}
	}
}

func (r *Repository) withProcessLock(ctx context.Context, waitForLock bool, fn func() error) error {
	lockFile, err := r.acquireProcessLockWithPolicy(ctx, waitForLock)
	if err != nil {
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()

	return fn()
}

func (r *Repository) acquireProcessLock(ctx context.Context) (*os.File, error) {
	return r.acquireProcessLockWithPolicy(ctx, true)
}

func (r *Repository) tryAcquireProcessLock(ctx context.Context) (*os.File, error) {
	return r.acquireProcessLockWithPolicy(ctx, false)
}

func (r *Repository) acquireProcessLockWithPolicy(ctx context.Context, waitForLock bool) (*os.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	lockPath := r.lockFilePath()
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create git lock directory: %w", err)
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open git lock file: %w", err)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return lockFile, nil
		} else if !errors.Is(err, syscall.EWOULDBLOCK) {
			_ = lockFile.Close()
			return nil, fmt.Errorf("acquire git lock: %w", err)
		} else if !waitForLock {
			_ = lockFile.Close()
			return nil, fmt.Errorf("%w: process lock", ErrRepositoryLockBusy)
		}

		select {
		case <-ctx.Done():
			_ = lockFile.Close()
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (r *Repository) lockFilePath() string {
	dir := strings.TrimSpace(r.Dir)
	if dir == "" {
		return filepath.Join(os.TempDir(), ".aods-managed-gitops.lock")
	}
	return filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".lock")
}

func (r *Repository) effectiveTimeout() time.Duration {
	if r.Timeout <= 0 {
		return 15 * time.Second
	}
	return r.Timeout
}

func (r *Repository) isSyncCacheFresh(gitDir string) bool {
	r.syncMu.Lock()
	defer r.syncMu.Unlock()
	return r.isSyncCacheFreshLocked(gitDir)
}

func (r *Repository) isSyncCacheFreshLocked(gitDir string) bool {
	if r.SyncTTL <= 0 {
		return false
	}
	if _, err := os.Stat(gitDir); err != nil {
		return false
	}
	if r.lastSyncAt.IsZero() {
		return false
	}
	if r.lastSyncKey != r.syncCacheKey() {
		return false
	}
	return time.Since(r.lastSyncAt) < r.SyncTTL
}

func (r *Repository) hasLocalSnapshot(gitDir string) bool {
	_, err := os.Stat(gitDir)
	return err == nil
}

func (r *Repository) markSynced() {
	r.syncMu.Lock()
	r.lastSyncAt = time.Now()
	r.lastSyncKey = r.syncCacheKey()
	r.lastSyncErr = nil
	r.syncMu.Unlock()
}

func (r *Repository) syncCacheKey() string {
	return strings.TrimSpace(r.Remote) + "|" + strings.TrimSpace(r.Branch)
}

func redactArgs(args []string) string {
	redacted := make([]string, len(args))
	for i, arg := range args {
		redacted[i] = redactRemote(arg)
	}

	return strings.Join(redacted, " ")
}

func redactRemote(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}

	username := parsed.User.Username()
	if username == "" {
		username = "redacted"
	}

	if _, ok := parsed.User.Password(); !ok {
		return value
	}

	parsed.User = url.UserPassword(username, "<redacted>")
	return parsed.String()
}
