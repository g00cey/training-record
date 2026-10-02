package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"training-record/internal/database"
)

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestBootstrapSkippedWhenNotFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// DB has the schema applied, so fresh=false. The source path is irrelevant here.
	res, err := database.MaybeBootstrap(db, filepath.Join(t.TempDir(), "missing.db"), false)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result when not fresh, got %+v", res)
	}
	if got := countRows(t, db, "strength_sessions"); got != 0 {
		t.Errorf("strength_sessions should be empty, got %d", got)
	}
}

func TestBootstrapNoSourceIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if res, err := database.MaybeBootstrap(db, "", true); err != nil || res != nil {
		t.Errorf("empty source: res=%v err=%v", res, err)
	}
	if res, err := database.MaybeBootstrap(db, filepath.Join(t.TempDir(), "missing.db"), true); err != nil || res != nil {
		t.Errorf("missing source: res=%v err=%v", res, err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	for i := 0; i < 3; i++ {
		if err := database.Migrate(db, os.DirFS("../..")); err != nil {
			t.Fatalf("migrate pass %d: %v", i, err)
		}
	}
	// One row per migration file, recorded once regardless of repeated runs.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	want := len(migrationFiles(t))
	if n != want {
		t.Errorf("schema_migrations has %d rows, want %d", n, want)
	}
}

func migrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			out = append(out, e.Name())
		}
	}
	return out
}
