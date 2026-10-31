package domain_test

import (
	"testing"
	"time"

	"training-record/internal/domain"
)

func TestBiweeklyPeriodOfDate(t *testing.T) {
	cases := []struct {
		in       string
		from, to string
	}{
		{"2026-10-04", "2026-10-04", "2026-10-17"}, // 期間の最初の日曜
		{"2026-10-16", "2026-10-04", "2026-10-17"}, // 期間中間
		{"2026-10-17", "2026-10-04", "2026-10-17"}, // 期間の最後の土曜
		{"2026-10-18", "2026-10-18", "2026-10-31"}, // 次期間の最初の日曜
		{"2026-10-30", "2026-10-18", "2026-10-31"},
		{"2026-10-31", "2026-10-18", "2026-10-31"}, // 期間の最後の土曜
		{"2026-11-01", "2026-11-01", "2026-11-14"},
		{"2026-09-19", "2026-09-06", "2026-09-19"}, // アンカーより過去
		{"2026-09-20", "2026-09-20", "2026-10-03"}, // アンカーより過去
	}
	for _, c := range cases {
		from, to, ok := domain.BiweeklyPeriodOfDate(c.in)
		if !ok {
			t.Fatalf("BiweeklyPeriodOfDate(%q) not ok", c.in)
		}
		if from != c.from || to != c.to {
			t.Errorf("BiweeklyPeriodOfDate(%q) = (%s, %s), want (%s, %s)", c.in, from, to, c.from, c.to)
		}
	}

	if _, _, ok := domain.BiweeklyPeriodOfDate("not-a-date"); ok {
		t.Error("BiweeklyPeriodOfDate should reject unparseable dates")
	}
}

func TestBiweeklyPeriodOfGridInvariants(t *testing.T) {
	// アンカー前後広範囲で、常に「日曜始まり・14日間（to = from+13日）・
	// 入力日を含む」になっていることを確認する。
	start := time.Date(2025, 10, 1, 0, 0, 0, 0, domain.JST)
	for d := start; d.Before(start.AddDate(1, 6, 0)); d = d.AddDate(0, 0, 1) {
		from, to := domain.BiweeklyPeriodOf(d)
		day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, domain.JST)
		if from.Weekday() != time.Sunday {
			t.Errorf("%s: period from %s is not Sunday", day.Format(domain.DateLayout), from.Format(domain.DateLayout))
		}
		if got := from.AddDate(0, 0, 13); !got.Equal(to) {
			t.Errorf("%s: period to %s != from+13d (%s)", day.Format(domain.DateLayout), to.Format(domain.DateLayout), got.Format(domain.DateLayout))
		}
		if from.After(day) || day.After(to) {
			t.Errorf("%s: not contained in %s〜%s", day.Format(domain.DateLayout), from.Format(domain.DateLayout), to.Format(domain.DateLayout))
		}
	}
}

func TestBiweeklyPeriodOfNormalizesToJST(t *testing.T) {
	// 2026-10-17 16:00 UTC = 2026-10-18 01:00 JST → 10/18〜10/31 の期間。
	from, to := domain.BiweeklyPeriodOf(time.Date(2026, 10, 17, 16, 0, 0, 0, time.UTC))
	if got := from.Format(domain.DateLayout); got != "2026-10-18" {
		t.Errorf("from = %s, want 2026-10-18", got)
	}
	if got := to.Format(domain.DateLayout); got != "2026-10-31" {
		t.Errorf("to = %s, want 2026-10-31", got)
	}
}
