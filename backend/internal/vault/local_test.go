package vault

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalStoreCleanupStaleRemovesExpiredPendingCommitSecrets(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	store := LocalStore{RootDir: rootDir}

	if err := store.writeDocument(
		pathToFile(rootDir, "secret/aods/staging/req_old"),
		map[string]any{
			"path": "secret/aods/staging/req_old",
			"data": map[string]string{"TOKEN": "old"},
			"metadata": map[string]any{
				"status":    pendingCommitStatus,
				"createdAt": time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	); err != nil {
		t.Fatalf("write old staged secret: %v", err)
	}

	if err := store.writeDocument(
		pathToFile(rootDir, "secret/aods/staging/req_fresh"),
		map[string]any{
			"path": "secret/aods/staging/req_fresh",
			"data": map[string]string{"TOKEN": "fresh"},
			"metadata": map[string]any{
				"status":    pendingCommitStatus,
				"createdAt": time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	); err != nil {
		t.Fatalf("write fresh staged secret: %v", err)
	}

	if err := store.writeDocument(
		pathToFile(rootDir, "secret/aods/staging/req_other"),
		map[string]any{
			"path": "secret/aods/staging/req_other",
			"data": map[string]string{"TOKEN": "other"},
			"metadata": map[string]any{
				"status":    "finalized",
				"createdAt": time.Now().UTC().Add(-72 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	); err != nil {
		t.Fatalf("write non-pending staged secret: %v", err)
	}

	count, err := store.CleanupStale(context.Background(), time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("cleanup stale secrets: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 cleaned staged secret, got %d", count)
	}

	if _, err := filepath.Glob(filepath.Join(rootDir, "aods", "staging", "*.json")); err != nil {
		t.Fatalf("glob staging files: %v", err)
	}

	if _, err := store.Get(context.Background(), "secret/aods/staging/req_old"); err != nil {
		t.Fatalf("get old staged secret: %v", err)
	}

	if _, err := store.Get(context.Background(), "secret/aods/staging/req_fresh"); err != nil {
		t.Fatalf("get fresh staged secret: %v", err)
	}

	if matches, err := filepath.Glob(filepath.Join(rootDir, "aods", "staging", "req_old.json")); err != nil {
		t.Fatalf("glob old staged secret: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("expected old staged secret to be deleted, found %d matches", len(matches))
	}

	if matches, err := filepath.Glob(filepath.Join(rootDir, "aods", "staging", "req_fresh.json")); err != nil {
		t.Fatalf("glob fresh staged secret: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected fresh staged secret to remain, found %d matches", len(matches))
	}

	if matches, err := filepath.Glob(filepath.Join(rootDir, "aods", "staging", "req_other.json")); err != nil {
		t.Fatalf("glob non-pending staged secret: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected non-pending staged secret to remain, found %d matches", len(matches))
	}
}
