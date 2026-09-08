package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
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
			preset+"|"+date+"|"+name+"|"+itoa(w)+"|"+itoa64(reps)+"|"+itoa64(sets)+"|"+itoa64(so)+"\n")...)
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
		b = append(b, []byte("PRESET|"+n+"|"+itoa64(so)+"\n")...)
	}
	pr.Close()
	return string(b)
}

func itoa(f float64) string { return itoa64(int64(f)) }
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestEnsureRoutinePresetsFromLegacyBootstrap(t *testing.T) {
	if _, err := os.Stat(legacyDB); err != nil {
		t.Skipf("legacy DB not present: %v", err)
	}
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := database.MaybeBootstrap(db, legacyDB, true); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Before fixup: every routine_snapshots row is preset='' (bootstrap
	// copies the legacy 4 tables verbatim).
	var unclassified int
	if err := db.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE preset = ''`).Scan(&unclassified); err != nil {
		t.Fatal(err)
	}
	if unclassified != 41 {
		t.Fatalf("expected 41 unclassified rows pre-fixup, got %d", unclassified)
	}

	if err := database.EnsureRoutinePresets(db); err != nil {
		t.Fatalf("ensure presets: %v", err)
	}

	// presets seeded
	names := map[string]int{}
	rows, err := db.Query(`SELECT name, sort_order FROM presets`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		var so int
		if err := rows.Scan(&n, &so); err != nil {
			t.Fatal(err)
		}
		names[n] = so
	}
	rows.Close()
	if names["自重"] != 0 || names["FW"] != 1 || len(names) != 2 {
		t.Fatalf("presets seeded wrong: %v", names)
	}

	// no row left unclassified
	if err := db.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE preset = ''`).Scan(&unclassified); err != nil {
		t.Fatal(err)
	}
	if unclassified != 0 {
		t.Fatalf("%d rows still preset='' after fixup", unclassified)
	}

	// 自重 = only NULL-weight rows; FW = only weighted rows
	var bad int
	db.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE preset='自重' AND weight IS NOT NULL`).Scan(&bad)
	if bad != 0 {
		t.Errorf("%d weighted rows landed in 自重", bad)
	}
	db.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE preset='FW' AND weight IS NULL`).Scan(&bad)
	if bad != 0 {
		t.Errorf("%d NULL-weight rows landed in FW", bad)
	}

	// history: each preset has the two legacy snapshot dates
	for _, preset := range []string{"自重", "FW"} {
		hr, err := db.Query(`SELECT date, COUNT(*) FROM routine_snapshots WHERE preset=? GROUP BY date ORDER BY date`, preset)
		if err != nil {
			t.Fatal(err)
		}
		var dates []string
		for hr.Next() {
			var d string
			var c int
			if err := hr.Scan(&d, &c); err != nil {
				t.Fatal(err)
			}
			dates = append(dates, d)
			if c == 0 {
				t.Errorf("%s %s has 0 exercises", preset, d)
			}
		}
		hr.Close()
		if len(dates) != 2 || dates[0] != "2026-07-03" || dates[1] != "2026-07-30" {
			t.Fatalf("%s history dates = %v, want [2026-07-03 2026-07-30]", preset, dates)
		}
	}

	// sort_order is 0..n-1 contiguous within each (preset, date) group
	gr, err := db.Query(`SELECT preset, date, COUNT(*), MIN(sort_order), MAX(sort_order), COUNT(DISTINCT sort_order)
		FROM routine_snapshots GROUP BY preset, date`)
	if err != nil {
		t.Fatal(err)
	}
	for gr.Next() {
		var p, d string
		var cnt, mn, mx, distinct int
		if err := gr.Scan(&p, &d, &cnt, &mn, &mx, &distinct); err != nil {
			t.Fatal(err)
		}
		if mn != 0 || mx != cnt-1 || distinct != cnt {
			t.Errorf("%s %s sort_order not 0..n-1: cnt=%d min=%d max=%d distinct=%d", p, d, cnt, mn, mx, distinct)
		}
	}
	gr.Close()

	// idempotent: a second (and third) run changes nothing
	fp1 := snapshotFingerprint(t, db)
	for i := 0; i < 2; i++ {
		if err := database.EnsureRoutinePresets(db); err != nil {
			t.Fatalf("ensure presets rerun %d: %v", i, err)
		}
	}
	if fp2 := snapshotFingerprint(t, db); fp2 != fp1 {
		t.Errorf("EnsureRoutinePresets not idempotent:\n--- first ---\n%s\n--- after rerun ---\n%s", fp1, fp2)
	}
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
