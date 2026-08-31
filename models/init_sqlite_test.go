package models

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteInit_WALAndBusyTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pragma-test.db")
	Init(context.Background(), path)
	t.Cleanup(func() {
		_ = Close()
		DB = nil
	})

	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("DB.DB(): %v", err)
	}

	var journalMode string
	if err := sqlDB.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := sqlDB.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if busyTimeout < 5000 {
		t.Fatalf("busy_timeout = %d, want >= 5000", busyTimeout)
	}
}
