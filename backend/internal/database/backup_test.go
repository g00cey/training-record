package database_test

import (
	"os"
	"path/filepath"
	"testing"

	"training-record/internal/database"
)

// TestBackupCapturesWAL verifies that Backup produces a standalone file that
// contains data which (as in production) is still resident in the -wal sidecar
// rather than checkpointed into the main training.db.
func TestBackupCapturesWAL(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "training.db")
	db, err := database.Open(srcPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO strength_sessions (date, notes) VALUES ('2026-09-10', 'backup test')`,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}

	dest := filepath.Join(dir, "backup.db")
	if err := database.Backup(db, dest); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Re-running to the same path must succeed (VACUUM INTO refuses to
	// overwrite, so Backup removes the previous file first).
	if err := database.Backup(db, dest); err != nil {
		t.Fatalf("second backup: %v", err)
	}

	// The snapshot is self-contained: openable without any -wal sidecar.
	if _, err := os.Stat(dest + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("backup unexpectedly left a -wal sidecar: %v", err)
	}
	bdb, err := database.Open(dest)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer bdb.Close()

	var notes string
	if err := bdb.QueryRow(
		`SELECT notes FROM strength_sessions WHERE date = '2026-09-10'`,
	).Scan(&notes); err != nil {
		t.Fatalf("backup missing row: %v", err)
	}
	if notes != "backup test" {
		t.Errorf("notes = %q, want %q", notes, "backup test")
	}
}
