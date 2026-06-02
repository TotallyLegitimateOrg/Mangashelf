package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/config"
)

func TestOpenConfiguresSQLitePragmas(t *testing.T) {
	database, err := Open(context.Background(), config.Config{
		DatabasePath: filepath.Join(t.TempDir(), "mangashelf-test.db"),
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer database.SQL.Close()

	var busyTimeout int
	if err := database.SQL.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}

	var foreignKeys int
	if err := database.SQL.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := database.SQL.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}
}
