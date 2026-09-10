package service

import (
	"sort"
	"testing"
	"time"

	"training-record/internal/domain"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation(domain.DateLayout, s, domain.JST)
	if err != nil {
		panic(err)
	}
	return t
}

func fp(f float64) *float64 { return &f }

func sess(date, notes string, exs ...domain.Exercise) domain.StrengthSession {
	return domain.StrengthSession{Date: date, Notes: notes, Exercises: exs}
}

func exr(name string, weight *float64, reps int) domain.Exercise {
	return domain.Exercise{Name: name, Weight: weight, Reps: reps, Sets: 1}
}

func spn(date string) domain.SpinSession {
	return domain.SpinSession{Date: date, DurationMinutes: 30}
}

// --- frequency ------------------------------------------------------------

func TestFrequencyAdvice(t *testing.T) {
	now := at("2026-09-15") // 14-day window: 2026-09-01 .. 2026-09-15

	cases := []struct {
		name       string
		strengths  []domain.StrengthSession
		spins      []domain.SpinSession
		wantStatus string
		wantMaxRun int
		wantTotal  int
	}{
		{
			name:       "long_off: nothing in window",
			strengths:  []domain.StrengthSession{sess("2026-08-01", "")},
			wantStatus: "long_off", wantMaxRun: 0, wantTotal: 0,
		},
		{
			name:       "low: 2 non-consecutive strength, no spin",
			strengths:  []domain.StrengthSession{sess("2026-09-02", ""), sess("2026-09-10", "")},
			wantStatus: "low", wantMaxRun: 1, wantTotal: 2,
		},
		{
			name:       "rest_needed: two consecutive days",
			strengths:  []domain.StrengthSession{sess("2026-09-08", ""), sess("2026-09-09", "")},
			wantStatus: "rest_needed", wantMaxRun: 2, wantTotal: 2,
		},
		{
			name: "good: 4 non-consecutive strength + 3 spin = 7",
			strengths: []domain.StrengthSession{
				sess("2026-09-02", ""), sess("2026-09-05", ""), sess("2026-09-08", ""), sess("2026-09-11", ""),
			},
			spins:      []domain.SpinSession{spn("2026-09-03"), spn("2026-09-06"), spn("2026-09-09")},
			wantStatus: "good", wantMaxRun: 1, wantTotal: 7,
		},
		{
			name: "priority: a 3-day run wins over otherwise-good totals",
			strengths: []domain.StrengthSession{
				sess("2026-09-02", ""), sess("2026-09-05", ""),
				sess("2026-09-08", ""), sess("2026-09-09", ""), sess("2026-09-10", ""),
				sess("2026-09-12", ""), sess("2026-09-13", ""), sess("2026-09-14", ""),
			},
			wantStatus: "rest_needed", wantMaxRun: 3, wantTotal: 8,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := frequencyAdvice(tc.strengths, tc.spins, now)
			if f.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q (%+v)", f.Status, tc.wantStatus, f)
			}
			if f.MaxConsecutiveWithin24h != tc.wantMaxRun {
				t.Errorf("maxConsecutiveWithin24h = %d, want %d", f.MaxConsecutiveWithin24h, tc.wantMaxRun)
			}
			if f.Total != tc.wantTotal {
				t.Errorf("total = %d, want %d", f.Total, tc.wantTotal)
			}
			if f.WindowDays != 14 || f.Message == "" {
				t.Errorf("bad window/message: %+v", f)
			}
		})
	}
}

func TestLongestConsecutiveRun(t *testing.T) {
	cases := []struct {
		in   []string
		want int
	}{
		{nil, 0},
		{[]string{"2026-09-01"}, 1},
		{[]string{"2026-09-01", "2026-09-03"}, 1},
		{[]string{"2026-09-01", "2026-09-02"}, 2},
		{[]string{"2026-09-01", "2026-09-02", "2026-09-03", "2026-09-07", "2026-09-08"}, 3},
	}
	for _, tc := range cases {
		if got := longestConsecutiveRun(tc.in); got != tc.want {
			t.Errorf("longestConsecutiveRun(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// --- progressive overload -------------------------------------------------

func TestOverloadWeeklyChangeBoundaries(t *testing.T) {
	now := at("2026-09-15")
	// prev7 = [2026-09-01, 2026-09-08), last7 = [2026-09-08, 2026-09-15].
	// One exercise, reps 10 sets 1 -> session VL = weight * 10.
	mk := func(prevVL, lastVL float64) []domain.StrengthSession {
		return []domain.StrengthSession{
			sess("2026-09-05", "", exr("バーベルX", fp(prevVL/10), 10)),
			sess("2026-09-10", "", exr("バーベルX", fp(lastVL/10), 10)),
		}
	}
	cases := []struct {
		name       string
		build      []domain.StrengthSession
		wantPct    float64
		wantStatus string
	}{
		{"exactly +10% -> ok", mk(1000, 1100), 10.0, "ok"},
		{"exactly +15% -> caution", mk(1000, 1150), 15.0, "caution"},
		{"just over +15% -> warning", mk(1000, 1160), 16.0, "warning"},
		{"prev window empty -> 0 / ok", []domain.StrengthSession{sess("2026-09-10", "", exr("バーベルX", fp(100), 10))}, 0.0, "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := overloadAdvice(tc.build, now, 86)
			if o.WeeklyVolumeChangePct != tc.wantPct {
				t.Errorf("weeklyVolumeChangePct = %v, want %v", o.WeeklyVolumeChangePct, tc.wantPct)
			}
			if o.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q (%+v)", o.Status, tc.wantStatus, o)
			}
		})
	}
}

func TestOverloadPerExerciseFlag(t *testing.T) {
	now := at("2026-09-15")
	// 40 -> 47 (+17.5%) vs the previous actual -> per-exercise warning.
	ss := []domain.StrengthSession{
		sess("2026-08-20", "", exr("ヒップストラスト", fp(40), 10)),
		sess("2026-09-10", "", exr("ヒップストラスト", fp(47), 10)),
	}
	o := overloadAdvice(ss, now, 86)
	if len(o.Exercises) != 1 {
		t.Fatalf("want 1 flagged exercise, got %+v", o.Exercises)
	}
	ex := o.Exercises[0]
	if ex.Name != "ヒップストラスト" || ex.ChangePct != 17.5 || ex.Status != "warning" {
		t.Errorf("exercise = %+v", ex)
	}
	if ex.PrevWeight == nil || *ex.PrevWeight != 40 || ex.LatestWeight == nil || *ex.LatestWeight != 47 {
		t.Errorf("weights = %+v", ex)
	}
	if ex.LastIncreasedOn != "2026-09-10" {
		t.Errorf("lastIncreasedOn = %q", ex.LastIncreasedOn)
	}
	if o.Status != "warning" {
		t.Errorf("overall status = %q, want warning", o.Status)
	}

	flat := []domain.StrengthSession{
		sess("2026-08-20", "", exr("ヒップストラスト", fp(40), 10)),
		sess("2026-09-10", "", exr("ヒップストラスト", fp(40), 10)),
	}
	if o := overloadAdvice(flat, now, 86); len(o.Exercises) != 0 {
		t.Errorf("unchanged weight must not be flagged: %+v", o.Exercises)
	}
}

// --- deload -------------------------------------------------------------

func weeklyMondaySessions(vlByWeek map[string]float64) []domain.StrengthSession {
	var out []domain.StrengthSession
	for wk, vl := range vlByWeek {
		out = append(out, sess(wk, "", exr("バーベルX", fp(vl), 1))) // reps 1 sets 1 -> VL = weight
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date }) // helpers assume ascending
	return out
}

func TestDeloadBoundary55Pct(t *testing.T) {
	now := at("2026-08-18") // current (in-progress) week starts 2026-08-17
	build := func(week5VL float64) []domain.StrengthSession {
		return weeklyMondaySessions(map[string]float64{
			"2026-07-06": 1000, "2026-07-13": 1000, "2026-07-20": 1000, "2026-07-27": 1000,
			"2026-08-03": week5VL, "2026-08-10": 1000, "2026-08-17": 1000,
		})
	}

	// exactly 55% of the preceding-4-week average (1000) -> deload week
	d := deloadAdvice(build(550), now, 86)
	if d.LastDeloadDate == nil || *d.LastDeloadDate != domain.MondayOf("2026-08-03") {
		t.Fatalf("VL=550 should be a deload week: %+v", d)
	}

	// just above 55% -> not a deload week -> no record at all
	d = deloadAdvice(build(551), now, 86)
	if d.LastDeloadDate != nil || d.WeeksSince != nil || !d.Due {
		t.Fatalf("VL=551 must not be a deload week: %+v", d)
	}
}

func TestDeloadNeverAndRecent(t *testing.T) {
	// steady volume, never a dip -> never deloaded -> due
	steady := weeklyMondaySessions(map[string]float64{
		"2026-07-06": 1000, "2026-07-13": 1000, "2026-07-20": 1000, "2026-07-27": 1000,
		"2026-08-03": 1000, "2026-08-10": 1000, "2026-08-17": 1000,
	})
	if d := deloadAdvice(steady, at("2026-08-18"), 86); !d.Due || d.LastDeloadDate != nil || d.WeeksSince != nil {
		t.Errorf("steady volume -> due, no record: %+v", d)
	}

	// empty history -> due, no record
	if d := deloadAdvice(nil, at("2026-09-01"), 86); !d.Due || d.LastDeloadDate != nil {
		t.Errorf("empty -> due, no record: %+v", d)
	}

	// a deload one week before the current week -> not due, weeksSince 1
	recent := weeklyMondaySessions(map[string]float64{
		"2026-07-27": 1000, "2026-08-03": 1000, "2026-08-10": 1000, "2026-08-17": 1000, "2026-08-24": 200,
	})
	d := deloadAdvice(recent, at("2026-08-31"), 86) // current week starts 2026-08-31
	if d.LastDeloadDate == nil || *d.LastDeloadDate != "2026-08-24" {
		t.Fatalf("recent deload not detected: %+v", d)
	}
	if d.Due || d.WeeksSince == nil || *d.WeeksSince != 1 {
		t.Errorf("recent deload -> not due, weeksSince 1: %+v", d)
	}
}

// --- watch exercises --------------------------------------------------------

func TestWatchExercisesPartialMatch(t *testing.T) {
	ss := []domain.StrengthSession{
		sess("2026-08-20", "",
			exr("ダンベルサイドレイズ", fp(8), 20),
			exr("インクライン・スカルクラッシャー", fp(12), 20),
			exr("ヒップストラスト", fp(40), 40)),
		sess("2026-09-10", "",
			exr("ダンベルサイドレイズ", fp(10), 20),       // +2kg, matches "サイドレイズ" -> flagged
			exr("インクライン・スカルクラッシャー", fp(12), 20), // unchanged -> not flagged
			exr("ヒップストラスト", fp(42), 40)),        // increased but not a watch exercise
	}
	w := watchExercisesAdvice(ss)
	if len(w) != 1 {
		t.Fatalf("want 1 watch exercise, got %+v", w)
	}
	if w[0].Name != "ダンベルサイドレイズ" || w[0].From != 8 || w[0].To != 10 ||
		w[0].On != "2026-09-10" || w[0].Reason != "weight_increased" ||
		w[0].FormGuideAnchor != "ダンベルサイドレイズ" {
		t.Errorf("watch exercise = %+v", w[0])
	}
}

// --- warning signs --------------------------------------------------------

func TestWarningSignsKeywords(t *testing.T) {
	now := at("2026-09-15") // 28-day window starts 2026-08-18
	ss := []domain.StrengthSession{
		sess("2026-09-06", "肩に違和感、少し痛みも"), // in window, two keywords
		sess("2026-09-08", "問題なし、絶好調"),    // in window, no keyword
		sess("2026-09-09", "腰に張りあり"),      // in window, 張り
		sess("2026-07-10", "激痛で中断"),       // out of window
		sess("2026-09-10", ""),            // empty notes
	}
	ws := warningSignsAdvice(ss, now)
	if len(ws.FlaggedSessions) != 2 {
		t.Fatalf("want 2 flagged sessions, got %+v", ws.FlaggedSessions)
	}
	if ws.FlaggedSessions[0].Date != "2026-09-06" {
		t.Errorf("first flagged = %+v", ws.FlaggedSessions[0])
	}
	// keyword order follows the fixed list: 痛 before 違和感
	if got := ws.FlaggedSessions[0].Matched; len(got) != 2 || got[0] != "痛" || got[1] != "違和感" {
		t.Errorf("matched = %v, want [痛 違和感]", got)
	}
	if ws.FlaggedSessions[1].Date != "2026-09-09" ||
		len(ws.FlaggedSessions[1].Matched) != 1 || ws.FlaggedSessions[1].Matched[0] != "張り" {
		t.Errorf("second flagged = %+v", ws.FlaggedSessions[1])
	}
	if len(ws.StopNow) != 4 || len(ws.Monitor) != 4 {
		t.Errorf("checklist lengths = %d / %d", len(ws.StopNow), len(ws.Monitor))
	}
}
