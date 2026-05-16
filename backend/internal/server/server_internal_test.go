package server

import (
	"strings"
	"testing"
	"time"

	"github.com/aolda/aods-backend/internal/application"
)

func TestApplicationCatalogCacheForDriver(t *testing.T) {
	mysqlCache, err := applicationCatalogCacheForDriver("mysql", nil)
	if err != nil {
		t.Fatalf("expected mysql cache, got error: %v", err)
	}
	if _, ok := mysqlCache.(application.MariaDBApplicationCatalogCache); !ok {
		t.Fatalf("expected MariaDB cache, got %T", mysqlCache)
	}

	postgresCache, err := applicationCatalogCacheForDriver("postgres", nil)
	if err != nil {
		t.Fatalf("expected postgres cache, got error: %v", err)
	}
	if _, ok := postgresCache.(application.PostgresApplicationCatalogCache); !ok {
		t.Fatalf("expected Postgres cache, got %T", postgresCache)
	}

	_, err = applicationCatalogCacheForDriver("sqlite", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported application catalog database driver") {
		t.Fatalf("expected unsupported driver error, got %v", err)
	}
}

func TestOpenAODSSQLDBRejectsInvalidDriver(t *testing.T) {
	_, err := openAODSSQLDB("", "")
	if err == nil || !strings.Contains(err.Error(), "database driver is required") {
		t.Fatalf("expected missing driver error, got %v", err)
	}

	_, err = openAODSSQLDB("sqlite", "file:test.db")
	if err == nil || !strings.Contains(err.Error(), "unsupported database driver") {
		t.Fatalf("expected unsupported driver error, got %v", err)
	}
}

func TestChainCleanupRunsBothFunctions(t *testing.T) {
	var calls []string

	cleanup := chainCleanup(
		func() {
			calls = append(calls, "first")
		},
		func() {
			calls = append(calls, "second")
		},
	)
	cleanup()

	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("expected both cleanup functions in order, got %#v", calls)
	}

	chainCleanup(nil, func() {
		calls = append(calls, "third")
	})()
	if len(calls) != 3 || calls[2] != "third" {
		t.Fatalf("expected nil cleanup to be skipped, got %#v", calls)
	}
}

func TestDatabaseDurationAndDSNHelpers(t *testing.T) {
	if got := maxDuration(0, 5*time.Minute); got != 5*time.Minute {
		t.Fatalf("expected fallback duration, got %s", got)
	}
	if got := maxDuration(30*time.Second, 5*time.Minute); got != 30*time.Second {
		t.Fatalf("expected provided duration, got %s", got)
	}

	if got := mariaDBDSN("user:pass@tcp(localhost:3306)/aods"); got != "user:pass@tcp(localhost:3306)/aods?parseTime=true&loc=UTC" {
		t.Fatalf("unexpected MariaDB DSN: %s", got)
	}
	if got := mariaDBDSN("user:pass@tcp(localhost:3306)/aods?charset=utf8mb4"); got != "user:pass@tcp(localhost:3306)/aods?charset=utf8mb4&parseTime=true&loc=UTC" {
		t.Fatalf("unexpected MariaDB DSN with query: %s", got)
	}
	if got := mariaDBDSN("user:pass@tcp(localhost:3306)/aods?parseTime=true"); got != "user:pass@tcp(localhost:3306)/aods?parseTime=true" {
		t.Fatalf("expected existing parseTime to be preserved, got %s", got)
	}
}
