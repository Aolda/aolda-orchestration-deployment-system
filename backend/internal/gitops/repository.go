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
	"time"
)

type Repository struct {
	Dir         string
	Remote      string
	Branch      string
	AuthorName  string
	AuthorEmail string
	Timeout     time.Duration

	mu sync.Mutex
}

type MissingFileError struct {
	Path string
}

func (e MissingFileError) Error() string {
	return fmt.Sprintf("managed repository is missing required file %s", e.Path)
}

func (r *Repository) WithRead(ctx context.Context, fn func(repoDir string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.sync(ctx); err != nil {
		return err
	}

	return fn(r.Dir)
}

func (r *Repository) WithWrite(
	ctx context.Context,
	commitMessage string,
	fn func(repoDir string) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.sync(ctx); err != nil {
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

	return nil
}

func (r *Repository) EnsureFile(ctx context.Context, relativePath string) error {
	return r.WithRead(ctx, func(repoDir string) error {
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
	})
}

func (r *Repository) sync(ctx context.Context) error {
	if strings.TrimSpace(r.Dir) == "" {
		return fmt.Errorf("managed git directory is required when git mode is enabled")
	}
	if strings.TrimSpace(r.Remote) == "" {
		return fmt.Errorf("git remote is required when git mode is enabled")
	}
	if strings.TrimSpace(r.Branch) == "" {
		return fmt.Errorf("git branch is required when git mode is enabled")
	}

	gitDir := filepath.Join(r.Dir, ".git")
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

	if err := r.run(ctx, "-C", r.Dir, "remote", "set-url", "origin", r.Remote); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "fetch", "origin", r.Branch, "--prune"); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "checkout", "-B", r.Branch, "origin/"+r.Branch); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "reset", "--hard", "origin/"+r.Branch); err != nil {
		return err
	}
	if err := r.run(ctx, "-C", r.Dir, "clean", "-fd"); err != nil {
		return err
	}

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
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never")

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

func (r *Repository) effectiveTimeout() time.Duration {
	if r.Timeout <= 0 {
		return 15 * time.Second
	}
	return r.Timeout
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
