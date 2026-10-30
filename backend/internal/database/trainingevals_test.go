package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"training-record/internal/database"
	"training-record/internal/domain"
)

// evalFingerprint captures the full training_evaluations state so two fixup
// runs can be compared for idempotency.
func evalFingerprint(t *testing.T, db *sql.DB) string {
	t.Helper()
	var b string
	rows, err := db.Query(`SELECT id, period_type, evaluated_at, period_from, period_to, summary
		FROM training_evaluations ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var ptype, evaluatedAt, from, to, summary string
		if err := rows.Scan(&id, &ptype, &evaluatedAt, &from, &to, &summary); err != nil {
			t.Fatal(err)
		}
		b += ptype + "|" + evaluatedAt + "|" + from + "|" + to + "|" + summary + "|" + strconv.FormatInt(id, 10) + "\n"
	}
	return b
}

func addDays(t *testing.T, date string, days int) string {
	t.Helper()
	d, err := time.ParseInLocation(domain.DateLayout, date, domain.JST)
	if err != nil {
		t.Fatalf("parse %q: %v", date, err)
	}
	return d.AddDate(0, 0, days).Format(domain.DateLayout)
}

func TestCollapseLegacyBiweeklyEvaluations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "training.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db, os.DirFS("../..")); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 空の DB では no-op。
	if err := database.CollapseLegacyBiweeklyEvaluations(db); err != nil {
		t.Fatalf("collapse on empty DB: %v", err)
	}

	// 旧日次バッチのローリング窓行（period_from = 実行日-14, period_to = 実行日）。
	// 実行日 9/7〜9/13 は固定期間 [9/6〜9/19] に、9/20〜9/26 は [9/20〜10/3] に対応する。
	seedLegacy := func(runDates ...string) {
		t.Helper()
		for _, d := range runDates {
			if _, err := db.Exec(
				`INSERT INTO training_evaluations
				 (period_type, evaluated_at, period_from, period_to, summary, raw_response)
				 VALUES ('biweekly', ?, ?, ?, ?, '')`,
				d+"T03:00:00+09:00", addDays(t, d, -14), d, "run "+d); err != nil {
				t.Fatalf("seed legacy %s: %v", d, err)
			}
		}
	}
	seedLegacy("2026-09-07", "2026-09-08", "2026-09-09", "2026-09-10", "2026-09-11", "2026-09-12", "2026-09-13")
	seedLegacy("2026-09-20", "2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24", "2026-09-25", "2026-09-26")

	// すでに固定グリッド形式の行と bimonthly 行は触らない。
	if _, err := db.Exec(
		`INSERT INTO training_evaluations
		 (period_type, evaluated_at, period_from, period_to, summary, raw_response)
		 VALUES ('biweekly', '2026-09-05T03:00:00+09:00', '2026-08-23', '2026-09-05', '新形式', '')`); err != nil {
		t.Fatalf("seed fixed-grid row: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO training_evaluations
		 (period_type, evaluated_at, period_from, period_to, summary, raw_response)
		 VALUES ('bimonthly', '2026-09-13T03:00:00+09:00', '2026-07-22', '2026-09-16', 'bimonthly', '')`); err != nil {
		t.Fatalf("seed bimonthly row: %v", err)
	}

	if err := database.CollapseLegacyBiweeklyEvaluations(db); err != nil {
		t.Fatalf("collapse: %v", err)
	}

	// 期待: 各固定期間の最新行だけが残り、期間ラベルが固定グリッドに書き換わる。
	rows, err := db.Query(`SELECT period_from, period_to, summary, evaluated_at
		FROM training_evaluations WHERE period_type = 'biweekly' ORDER BY period_from`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got [][4]string
	for rows.Next() {
		var r [4]string
		if err := rows.Scan(&r[0], &r[1], &r[2], &r[3]); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	want := [][4]string{
		{"2026-08-23", "2026-09-05", "新形式", "2026-09-05T03:00:00+09:00"},
		{"2026-09-06", "2026-09-19", "run 2026-09-13", "2026-09-13T03:00:00+09:00"},
		{"2026-09-20", "2026-10-03", "run 2026-09-26", "2026-09-26T03:00:00+09:00"},
	}
	if len(got) != len(want) {
		t.Fatalf("biweekly rows after collapse = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("biweekly row[%d] = %v, want %v", i, got[i], w)
		}
	}

	// bimonthly 行は無傷。
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM training_evaluations WHERE period_type = 'bimonthly'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("bimonthly rows = %d, want 1", n)
	}

	// 2回目の実行は何も変えない（冪等）。
	before := evalFingerprint(t, db)
	if err := database.CollapseLegacyBiweeklyEvaluations(db); err != nil {
		t.Fatalf("collapse again: %v", err)
	}
	if after := evalFingerprint(t, db); after != before {
		t.Fatalf("collapse is not idempotent:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
