package gitops

import (
	"context"
	"os"
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
