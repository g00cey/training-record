package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"training-record/internal/database"
)

// snapshotFingerprint captures the full routine_snapshots + presets state
// so two fixup runs can be compared for idempotency.
func snapshotFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b []byte
	rows, err := db.Query(`SELECT id, preset, date, exercise_name, COALESCE(weight,-999), reps, sets, sort_order
		FROM routine_snapshots ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, reps, sets, so int64
		var preset, date, name string
		var w float64
		if err := rows.Scan(&id, &preset, &date, &name, &w, &reps, &sets, &so); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		b = append(b, []byte(
			preset+"|"+date+"|"+name+"|"+strconv.FormatInt(int64(w), 10)+"|"+strconv.FormatInt(reps, 10)+"|"+strconv.FormatInt(sets, 10)+"|"+strconv.FormatInt(so, 10)+"\n")...)
	}
	rows.Close()
	pr, err := db.Query(`SELECT name, sort_order FROM presets ORDER BY sort_order, name`)
	if err != nil {
		t.Fatal(err)
	}
	for pr.Next() {
		var n string
		var so int64
		if err := pr.Scan(&n, &so); err != nil {
			pr.Close()
			t.Fatal(err)
		}
		b = append(b, []byte("PRESET|"+n+"|"+strconv.FormatInt(so, 10)+"\n")...)
	}
	pr.Close()
	return string(b)
}

func TestEnsureRoutinePresetsOnEmptyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.EnsureRoutinePresets(db); err != nil {
		t.Fatalf("ensure presets: %v", err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM presets`).Scan(&n)
	if n != 2 {
		t.Fatalf("expected 2 seeded presets on empty DB, got %d", n)
	}
	// user deletes one -> restart fixup must NOT resurrect it
	if _, err := db.Exec(`DELETE FROM presets WHERE name='自重'`); err != nil {
		t.Fatal(err)
	}
	if err := database.EnsureRoutinePresets(db); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT COUNT(*) FROM presets`).Scan(&n)
	if n != 1 {
		t.Fatalf("fixup resurrected a deleted preset: count=%d", n)
	}
}
