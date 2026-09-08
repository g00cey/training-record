package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"training-record/internal/database"
)

const legacyDB = "../../../skill/.hermes/home/.hermes/training-logs/training.db"

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestBootstrapImportsLegacyDB(t *testing.T) {
	if _, err := os.Stat(legacyDB); err != nil {
		t.Skipf("legacy DB not present: %v", err)
	}

	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	fresh, err := database.HasAppSchema(db)
	if err != nil {
		t.Fatalf("HasAppSchema: %v", err)
	}
	if fresh {
		t.Fatal("brand new DB unexpectedly reports existing schema")
	}
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	res, err := database.MaybeBootstrap(db, legacyDB, true /* fresh */)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if res == nil {
		t.Fatal("bootstrap returned nil result")
	}

	want := map[string]int{
		"strength_sessions": 36,
		"exercises":         559,
		"spin_sessions":     5,
		"routine_snapshots": 41,
	}
	for table, n := range want {
		if got := countRows(t, db, table); got != n {
			t.Errorf("%s: got %d rows, want %d", table, got, n)
		}
	}
	if res.StrengthSessions != 36 || res.Exercises != 559 || res.SpinSessions != 5 || res.RoutineSnapshots != 41 {
		t.Errorf("result counts mismatch: %+v", res)
	}

	// routine_snapshots must contain exactly two snapshot dates.
	rows, err := db.Query(`SELECT date, COUNT(*) FROM routine_snapshots GROUP BY date ORDER BY date`)
	if err != nil {
		t.Fatalf("group query: %v", err)
	}
	defer rows.Close()
	var dates []string
	counts := map[string]int{}
	for rows.Next() {
		var d string
		var c int
		if err := rows.Scan(&d, &c); err != nil {
			t.Fatal(err)
		}
		dates = append(dates, d)
		counts[d] = c
	}
	if len(dates) != 2 {
		t.Fatalf("expected 2 routine snapshot dates, got %d (%v)", len(dates), dates)
	}
	if counts["2026-07-03"] != 19 || counts["2026-07-30"] != 22 {
		t.Errorf("snapshot sizes mismatch: %v", counts)
	}

	// profile default row must exist.
	var bw float64
	if err := db.QueryRow(`SELECT bodyweight_kg FROM profile WHERE id = 1`).Scan(&bw); err != nil {
		t.Fatalf("profile row missing: %v", err)
	}
	if bw != 86.0 {
		t.Errorf("profile bodyweight_kg = %v, want 86", bw)
	}

	// ids are preserved from the source.
	var maxID int64
	if err := db.QueryRow(`SELECT MAX(id) FROM exercises`).Scan(&maxID); err != nil {
		t.Fatal(err)
	}
	if maxID != 559 {
		t.Errorf("max exercise id = %d, want 559 (ids not preserved)", maxID)
	}

	// A second pass on a now-populated DB must NOT re-import.
	again, err := database.HasAppSchema(db)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := database.MaybeBootstrap(db, legacyDB, !again)
	if err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}
	if res2 != nil {
		t.Errorf("second bootstrap imported again: %+v", res2)
	}
	if got := countRows(t, db, "exercises"); got != 559 {
		t.Errorf("exercises row count changed after second pass: %d", got)
	}
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
	res, err := database.MaybeBootstrap(db, legacyDB, false)
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
