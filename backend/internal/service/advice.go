package service

import (
	"fmt"
	"strings"
	"time"

	"training-record/internal/domain"
)

// ---------------------------------------------------------------------------
// GET /api/advice  (Phase 6 injury-prevention advice)
//
// Aggregates the SKILL.md / docs/domain.md rules into one payload shared by
// the Web UI advice card and the Hermes `advice` subcommand. Natural-language
// phrasing / form-guide quoting happens on the Hermes side.
// ---------------------------------------------------------------------------

// AdviceACWR is the acute:chronic load ratio block (same maths as load-report).
type AdviceACWR struct {
	Value         float64 `json:"value"`
	Zone          string  `json:"zone"` // safe|caution|warning|low|slightly_low|no_data
	Acute7d       int     `json:"acute7d"`
	ChronicWeekly int     `json:"chronicWeekly"`
}

// AdviceFrequency is the 14-day training-frequency block.
type AdviceFrequency struct {
	WindowDays              int    `json:"windowDays"`
	StrengthSessions        int    `json:"strengthSessions"`
	SpinSessions            int    `json:"spinSessions"`
	Total                   int    `json:"total"`
	MaxConsecutiveWithin24h int    `json:"maxConsecutiveWithin24h"`
	Status                  string `json:"status"` // good|low|rest_needed|long_off
	Message                 string `json:"message"`
}

// AdviceOverloadExercise is one exercise whose weight rose vs its last actual.
type AdviceOverloadExercise struct {
	Name            string   `json:"name"`
	PrevWeight      *float64 `json:"prevWeight"`
	LatestWeight    *float64 `json:"latestWeight"`
	ChangePct       float64  `json:"changePct"`
	Status          string   `json:"status"` // ok|caution|warning
	LastIncreasedOn string   `json:"lastIncreasedOn"`
}

// AdviceOverload is the progressive-overload block.
type AdviceOverload struct {
	WeeklyVolumeChangePct float64                  `json:"weeklyVolumeChangePct"`
	Status                string                   `json:"status"` // ok|caution|warning
	Exercises             []AdviceOverloadExercise `json:"exercises"`
}

// AdviceDeload is the deload-week block.
type AdviceDeload struct {
	LastDeloadDate *string `json:"lastDeloadDate"`
	WeeksSince     *int    `json:"weeksSince"`
	Due            bool    `json:"due"`
	Message        string  `json:"message"`
}

// AdviceWatchExercise flags a high-risk exercise whose weight increased.
type AdviceWatchExercise struct {
	Name            string  `json:"name"`
	Reason          string  `json:"reason"` // "weight_increased"
	From            float64 `json:"from"`
	To              float64 `json:"to"`
	On              string  `json:"on"`
	FormGuideAnchor string  `json:"formGuideAnchor"`
}

// AdviceFlaggedSession is a session whose notes matched a pain keyword.
type AdviceFlaggedSession struct {
	Date    string   `json:"date"`
	Matched []string `json:"matched"`
	Notes   string   `json:"notes"`
}

// AdviceWarningSigns is the warning-signs block.
type AdviceWarningSigns struct {
	FlaggedSessions []AdviceFlaggedSession `json:"flaggedSessions"`
	StopNow         []string               `json:"stopNow"`
	Monitor         []string               `json:"monitor"`
}

// AdviceReport is the GET /api/advice payload.
type AdviceReport struct {
	AsOf                string                `json:"asOf"`
	ACWR                AdviceACWR            `json:"acwr"`
	Frequency           AdviceFrequency       `json:"frequency"`
	ProgressiveOverload AdviceOverload        `json:"progressiveOverload"`
	Deload              AdviceDeload          `json:"deload"`
	WatchExercises      []AdviceWatchExercise `json:"watchExercises"`
	WarningSigns        AdviceWarningSigns    `json:"warningSigns"`
}

// painKeywords are matched (substring) against strength_sessions.notes.
var painKeywords = []string{"痛", "違和感", "しびれ", "痺れ", "ロッキング", "肉離れ", "張り"}

// watchExerciseFragments are the fixed high-risk exercises (partial match).
var watchExerciseFragments = []string{
	"ショルダープレス", "サイドレイズ", "ディップス", "スカルクラッシャー",
	"懸垂", "ダンベルデッドリフト", "フロントラックスクワット",
}

var stopNowChecklist = []string{
	"鋭い/刺す痛み（特に片側）",
	"関節の引っかかり・ロッキング感",
	"めまい・吐き気・胸の痛み",
	"筋肉の『プチッ』（肉離れの疑い）",
}

var monitorChecklist = []string{
	"同じ部位の違和感が2週間以上",
	"トレーニング中だけ出る痛み",
	"日常生活に支障",
	"翌日に痛みが強くなる",
}

// Advice builds the injury-prevention advice report for "now" (JST).
func (s *Service) Advice() (*AdviceReport, error) {
	return s.adviceAt(time.Now().In(domain.JST))
}

func (s *Service) adviceAt(now time.Time) (*AdviceReport, error) {
	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, err
	}
	// Full history: progressive-overload / watch need the previous actual
	// of each current exercise, deload needs a multi-week VL series.
	strengths, err := s.st.StrengthSessionsInRange("", "")
	if err != nil {
		return nil, err
	}
	spins, err := s.st.SpinSessionsInRange("", "")
	if err != nil {
		return nil, err
	}

	m := computeLoadMetricsFrom(now, prof, strengths, spins)

	rep := &AdviceReport{
		AsOf: m.today,
		ACWR: AdviceACWR{
			Value:         m.acwr,
			Zone:          m.zone,
			Acute7d:       int(domain.RoundInt(m.acuteTotal)),
			ChronicWeekly: int(domain.RoundInt(m.chronicWeekly)),
		},
		Frequency:           frequencyAdvice(strengths, spins, now),
		ProgressiveOverload: overloadAdvice(strengths, now, prof.BodyweightKg),
		Deload:              deloadAdvice(strengths, now, prof.BodyweightKg),
		WatchExercises:      watchExercisesAdvice(strengths),
		WarningSigns:        warningSignsAdvice(strengths, now),
	}
	return rep, nil
}

// --- frequency -----------------------------------------------------------

func frequencyAdvice(strengths []domain.StrengthSession, spins []domain.SpinSession, now time.Time) AdviceFrequency {
	day14 := now.AddDate(0, 0, -14).Format(domain.DateLayout)

	var strengthDates []string
	for _, ss := range strengths {
		if ss.Date >= day14 {
			strengthDates = append(strengthDates, ss.Date)
		}
	}
	spinCount := 0
	for _, sp := range spins {
		if sp.Date >= day14 {
			spinCount++
		}
	}
	sCount := len(strengthDates)
	total := sCount + spinCount
	maxRun := longestConsecutiveRun(strengthDates)

	f := AdviceFrequency{
		WindowDays:              14,
		StrengthSessions:        sCount,
		SpinSessions:            spinCount,
		Total:                   total,
		MaxConsecutiveWithin24h: maxRun,
	}
	switch {
	case total == 0:
		f.Status = "long_off"
		f.Message = "直近2週間トレーニング記録なし。再開時は重量を落としてリハビリ的に始める"
	case maxRun >= 2:
		f.Status = "rest_needed"
		f.Message = fmt.Sprintf("筋トレが最長%d日連続。連続日は間隔を空けて休息を確保する", maxRun)
	case sCount < 3:
		f.Status = "low"
		f.Message = fmt.Sprintf("直近2週で筋トレ%d回。頻度がやや少ない（目安 週3回以上）", sCount)
	default:
		f.Status = "good"
		f.Message = fmt.Sprintf("直近2週で計%d回（筋トレ%d / スピン%d）。良好", total, sCount, spinCount)
	}
	return f
}

// longestConsecutiveRun returns the longest run of consecutive calendar
// days among the given ascending, unique date strings (0 when empty).
func longestConsecutiveRun(datesAsc []string) int {
	if len(datesAsc) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(datesAsc); i++ {
		prev, err1 := time.ParseInLocation(domain.DateLayout, datesAsc[i-1], domain.JST)
		this, err2 := time.ParseInLocation(domain.DateLayout, datesAsc[i], domain.JST)
		if err1 != nil || err2 != nil {
			cur = 1
			continue
		}
		if diff := this.Sub(prev); diff > 0 && diff <= 24*time.Hour {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 1
		}
	}
	return best
}

// --- progressive overload ---------------------------------------------------

func overloadStatusFromPct(pct float64) string {
	switch {
	case pct > 15:
		return "warning"
	case pct > 10:
		return "caution"
	default:
		return "ok"
	}
}

func overloadAdvice(strengths []domain.StrengthSession, now time.Time, bw float64) AdviceOverload {
	day7 := now.AddDate(0, 0, -7).Format(domain.DateLayout)
	day14 := now.AddDate(0, 0, -14).Format(domain.DateLayout)

	var last7VL, prev7VL float64
	for _, ss := range strengths {
		vl := domain.SessionVolumeLoad(ss.Exercises, bw)
		switch {
		case ss.Date >= day7:
			last7VL += vl
		case ss.Date >= day14:
			prev7VL += vl
		}
	}
	changePct := 0.0
	if prev7VL > 0 {
		changePct = domain.Round1((last7VL - prev7VL) / prev7VL * 100)
	}

	out := AdviceOverload{
		WeeklyVolumeChangePct: changePct,
		Status:                overloadStatusFromPct(changePct),
		Exercises:             []AdviceOverloadExercise{},
	}
	if len(strengths) == 0 {
		return out
	}

	latestIdx := len(strengths) - 1
	latest := strengths[latestIdx]
	seen := map[string]bool{}
	for _, e := range latest.Exercises {
		if e.Weight == nil || seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		latestW := maxWeightOfName(latest, e.Name)
		prevW, _, ok := previousWeightOfName(strengths, latestIdx, e.Name)
		if !ok || latestW <= prevW {
			continue
		}
		lw, pw := latestW, prevW
		pct := domain.Round1((latestW - prevW) / prevW * 100)
		exStatus := "ok"
		if pct > 15 || shortConsecutiveIncrease(strengths, latestIdx, e.Name, latest.Date) {
			exStatus = "warning"
		} else if pct > 10 {
			exStatus = "caution"
		}
		out.Exercises = append(out.Exercises, AdviceOverloadExercise{
			Name:            e.Name,
			PrevWeight:      &pw,
			LatestWeight:    &lw,
			ChangePct:       pct,
			Status:          exStatus,
			LastIncreasedOn: latest.Date,
		})
	}

	// Overall status = worst of the weekly band and any flagged exercise.
	for _, ex := range out.Exercises {
		if ex.Status == "warning" {
			out.Status = "warning"
		} else if ex.Status == "caution" && out.Status == "ok" {
			out.Status = "caution"
		}
	}
	return out
}

// maxWeightOfName returns the greatest non-null weight recorded for name in
// the session (0 when the name has no weighted entry).
func maxWeightOfName(ss domain.StrengthSession, name string) float64 {
	max := 0.0
	found := false
	for _, e := range ss.Exercises {
		if e.Name == name && e.Weight != nil {
			if !found || *e.Weight > max {
				max = *e.Weight
				found = true
			}
		}
	}
	return max
}

// previousWeightOfName scans strengths[:beforeIdx] backwards for the most
// recent session containing name with a non-null weight.
func previousWeightOfName(strengths []domain.StrengthSession, beforeIdx int, name string) (weight float64, date string, ok bool) {
	for i := beforeIdx - 1; i >= 0; i-- {
		w := maxWeightOfName(strengths[i], name)
		if hasWeightedName(strengths[i], name) {
			return w, strengths[i].Date, true
		}
	}
	return 0, "", false
}

func hasWeightedName(ss domain.StrengthSession, name string) bool {
	for _, e := range ss.Exercises {
		if e.Name == name && e.Weight != nil {
			return true
		}
	}
	return false
}

// shortConsecutiveIncrease reports whether the exercise's previous actual
// was itself an increase and happened within 7 days of the latest session
// ("前回増量から間隔が短い").
func shortConsecutiveIncrease(strengths []domain.StrengthSession, latestIdx int, name, latestDate string) bool {
	prevW, prevDate, ok := previousWeightOfName(strengths, latestIdx, name)
	if !ok {
		return false
	}
	prevIdx := -1
	for i := latestIdx - 1; i >= 0; i-- {
		if strengths[i].Date == prevDate {
			prevIdx = i
			break
		}
	}
	if prevIdx < 0 {
		return false
	}
	ppW, _, ok := previousWeightOfName(strengths, prevIdx, name)
	if !ok || prevW <= ppW {
		return false // the previous actual was not itself an increase
	}
	d1, err1 := time.ParseInLocation(domain.DateLayout, prevDate, domain.JST)
	d2, err2 := time.ParseInLocation(domain.DateLayout, latestDate, domain.JST)
	if err1 != nil || err2 != nil {
		return false
	}
	return d2.Sub(d1) <= 7*24*time.Hour
}

// --- deload ---------------------------------------------------------------

func deloadAdvice(strengths []domain.StrengthSession, now time.Time, bw float64) AdviceDeload {
	if len(strengths) == 0 {
		return AdviceDeload{
			Due:     true,
			Message: "デロード週の記録なし。4〜6週に1回、ボリュームを40〜50%落とす週を設けると故障予防に有効",
		}
	}

	curMonday := domain.MondayOf(now.Format(domain.DateLayout))
	firstMonday := domain.MondayOf(strengths[0].Date)

	// continuous weekly VL series firstMonday..curMonday (gaps = 0)
	var weeks []string
	vlByWeek := map[string]float64{}
	for wk := firstMonday; wk <= curMonday; wk = nextMonday(wk) {
		weeks = append(weeks, wk)
		vlByWeek[wk] = 0
		if len(weeks) > 520 { // safety bound (~10 years)
			break
		}
	}
	for _, ss := range strengths {
		wk := domain.MondayOf(ss.Date)
		if _, ok := vlByWeek[wk]; ok {
			vlByWeek[wk] += domain.SessionVolumeLoad(ss.Exercises, bw)
		}
	}

	lastDeload := ""
	for i := 4; i < len(weeks); i++ {
		if weeks[i] == curMonday {
			continue // current week is in progress
		}
		avg4 := (vlByWeek[weeks[i-4]] + vlByWeek[weeks[i-3]] + vlByWeek[weeks[i-2]] + vlByWeek[weeks[i-1]]) / 4
		if avg4 > 0 && vlByWeek[weeks[i]] <= 0.55*avg4 {
			lastDeload = weeks[i]
		}
	}

	if lastDeload == "" {
		return AdviceDeload{
			Due:     true,
			Message: "デロード週の記録なし。4〜6週に1回、ボリュームを40〜50%落とす週を設けると故障予防に有効",
		}
	}
	weeksSince := weeksBetweenMondays(lastDeload, curMonday)
	due := weeksSince >= 5
	d := lastDeload
	ws := weeksSince
	msg := fmt.Sprintf("前回デロードから%d週間。まだ余裕あり", weeksSince)
	if due {
		msg = fmt.Sprintf("前回デロードから%d週間。今週ボリュームを40〜50%%落とすデロードを検討", weeksSince)
	}
	return AdviceDeload{LastDeloadDate: &d, WeeksSince: &ws, Due: due, Message: msg}
}

func nextMonday(monday string) string {
	t, err := time.ParseInLocation(domain.DateLayout, monday, domain.JST)
	if err != nil {
		return monday
	}
	return t.AddDate(0, 0, 7).Format(domain.DateLayout)
}

func weeksBetweenMondays(a, b string) int {
	ta, err1 := time.ParseInLocation(domain.DateLayout, a, domain.JST)
	tb, err2 := time.ParseInLocation(domain.DateLayout, b, domain.JST)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(tb.Sub(ta).Hours() / (24 * 7))
}

// --- watch exercises ----------------------------------------------------

func watchExercisesAdvice(strengths []domain.StrengthSession) []AdviceWatchExercise {
	out := []AdviceWatchExercise{}
	if len(strengths) == 0 {
		return out
	}
	latestIdx := len(strengths) - 1
	latest := strengths[latestIdx]
	seen := map[string]bool{}
	for _, e := range latest.Exercises {
		if e.Weight == nil || seen[e.Name] || !matchesWatchFragment(e.Name) {
			continue
		}
		latestW := maxWeightOfName(latest, e.Name)
		prevW, _, ok := previousWeightOfName(strengths, latestIdx, e.Name)
		if !ok || latestW <= prevW {
			continue
		}
		seen[e.Name] = true
		out = append(out, AdviceWatchExercise{
			Name:            e.Name,
			Reason:          "weight_increased",
			From:            prevW,
			To:              latestW,
			On:              latest.Date,
			FormGuideAnchor: e.Name,
		})
	}
	return out
}

func matchesWatchFragment(name string) bool {
	for _, frag := range watchExerciseFragments {
		if strings.Contains(name, frag) {
			return true
		}
	}
	return false
}

// --- warning signs ------------------------------------------------------

func warningSignsAdvice(strengths []domain.StrengthSession, now time.Time) AdviceWarningSigns {
	day28 := now.AddDate(0, 0, -28).Format(domain.DateLayout)
	flagged := []AdviceFlaggedSession{}
	for _, ss := range strengths {
		if ss.Date < day28 || ss.Notes == "" {
			continue
		}
		var matched []string
		for _, kw := range painKeywords {
			if strings.Contains(ss.Notes, kw) {
				matched = append(matched, kw)
			}
		}
		if len(matched) > 0 {
			flagged = append(flagged, AdviceFlaggedSession{Date: ss.Date, Matched: matched, Notes: ss.Notes})
		}
	}
	return AdviceWarningSigns{
		FlaggedSessions: flagged,
		StopNow:         stopNowChecklist,
		Monitor:         monitorChecklist,
	}
}
