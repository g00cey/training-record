package service

import (
	"fmt"
	"sort"
	"time"

	"training-record/internal/apperr"
	"training-record/internal/domain"
)

// ---------------------------------------------------------------------------
// GET /api/summary
// ---------------------------------------------------------------------------

// SummaryResult is the GET /api/summary payload.
type SummaryResult struct {
	PeriodDays               int     `json:"periodDays"`
	Since                    string  `json:"since"`
	StrengthSessionsInPeriod int     `json:"strengthSessionsInPeriod"`
	SpinSessionsInPeriod     int     `json:"spinSessionsInPeriod"`
	TotalStrengthSessions    int     `json:"totalStrengthSessions"`
	TotalSpinSessions        int     `json:"totalSpinSessions"`
	LatestStrengthDate       *string `json:"latestStrengthDate"`
	LatestSpinDate           *string `json:"latestSpinDate"`
	LatestRoutineDate        *string `json:"latestRoutineDate"`
}

// Summary implements the legacy `summary` command.
func (s *Service) Summary(days int) (*SummaryResult, error) {
	since := time.Now().In(domain.JST).AddDate(0, 0, -days).Format(domain.DateLayout)
	d, err := s.st.Summary(since)
	if err != nil {
		return nil, err
	}
	return &SummaryResult{
		PeriodDays:               days,
		Since:                    since,
		StrengthSessionsInPeriod: d.StrengthInPeriod,
		SpinSessionsInPeriod:     d.SpinInPeriod,
		TotalStrengthSessions:    d.TotalStrength,
		TotalSpinSessions:        d.TotalSpin,
		LatestStrengthDate:       d.LatestStrength,
		LatestSpinDate:           d.LatestSpin,
		LatestRoutineDate:        d.LatestRoutineDate,
	}, nil
}

// ---------------------------------------------------------------------------
// GET /api/calendar
// ---------------------------------------------------------------------------

// CalendarStrength is a day's strength entry in the calendar view.
type CalendarStrength struct {
	Kind          string  `json:"kind"`
	ExerciseCount int     `json:"exerciseCount"`
	VolumeLoad    float64 `json:"volumeLoad"`
}

// CalendarSpin is a day's spin entry in the calendar view.
type CalendarSpin struct {
	DurationMinutes int  `json:"durationMinutes"`
	RPE             *int `json:"rpe"`
}

// CalendarDay is one active day.
type CalendarDay struct {
	Date     string            `json:"date"`
	Strength *CalendarStrength `json:"strength"`
	Spin     *CalendarSpin     `json:"spin"`
}

// CalendarResult is the GET /api/calendar payload.
type CalendarResult struct {
	Month string        `json:"month"`
	Days  []CalendarDay `json:"days"`
}

// Calendar builds the month view. month is "YYYY-MM".
func (s *Service) Calendar(month string) (*CalendarResult, error) {
	first, err := time.ParseInLocation("2006-01", month, domain.JST)
	if err != nil {
		return nil, apperr.Invalidf("month must be YYYY-MM")
	}
	from := first.Format(domain.DateLayout)
	to := first.AddDate(0, 1, -1).Format(domain.DateLayout)

	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, err
	}
	strengths, err := s.st.StrengthSessionsInRange(from, to)
	if err != nil {
		return nil, err
	}
	spins, err := s.st.SpinSessionsInRange(from, to)
	if err != nil {
		return nil, err
	}

	byDate := map[string]*CalendarDay{}
	order := []string{}
	get := func(date string) *CalendarDay {
		if d, ok := byDate[date]; ok {
			return d
		}
		d := &CalendarDay{Date: date}
		byDate[date] = d
		order = append(order, date)
		return d
	}
	for _, ss := range strengths {
		d := get(ss.Date)
		d.Strength = &CalendarStrength{
			Kind:          domain.SessionKind(ss.Notes),
			ExerciseCount: len(ss.Exercises),
			VolumeLoad:    domain.RoundInt(domain.SessionVolumeLoad(ss.Exercises, prof.BodyweightKg)),
		}
	}
	for _, sp := range spins {
		d := get(sp.Date)
		d.Spin = &CalendarSpin{DurationMinutes: sp.DurationMinutes, RPE: sp.RPE}
	}
	sort.Strings(order)
	days := make([]CalendarDay, 0, len(order))
	for _, date := range order {
		days = append(days, *byDate[date])
	}
	return &CalendarResult{Month: month, Days: days}, nil
}

// ---------------------------------------------------------------------------
// GET /api/volume
// ---------------------------------------------------------------------------

// VolumeParams are the query parameters for GET /api/volume.
type VolumeParams struct {
	Granularity string // "session" | "week"
	From        string
	To          string
	Exercise    string
}

type volumeSessionResult struct {
	Granularity string               `json:"granularity"`
	Points      []volumeSessionPoint `json:"points"`
}
type volumeSessionPoint struct {
	Date          string  `json:"date"`
	VolumeLoad    float64 `json:"volumeLoad"`
	ExerciseCount int     `json:"exerciseCount"`
}
type volumeWeekResult struct {
	Granularity string            `json:"granularity"`
	Points      []volumeWeekPoint `json:"points"`
}
type volumeWeekPoint struct {
	WeekStart    string  `json:"weekStart"`
	VolumeLoad   float64 `json:"volumeLoad"`
	SessionCount int     `json:"sessionCount"`
}
type volumeExerciseResult struct {
	Exercise string                `json:"exercise"`
	Points   []volumeExercisePoint `json:"points"`
}
type volumeExercisePoint struct {
	Date       string   `json:"date"`
	Weight     *float64 `json:"weight"`
	Reps       int      `json:"reps"`
	Sets       int      `json:"sets"`
	VolumeLoad float64  `json:"volumeLoad"`
}

// Volume implements GET /api/volume (session / week / per-exercise).
func (s *Service) Volume(p VolumeParams) (any, error) {
	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, err
	}
	sessions, err := s.st.StrengthSessionsInRange(p.From, p.To)
	if err != nil {
		return nil, err
	}

	if p.Exercise != "" {
		res := volumeExerciseResult{Exercise: p.Exercise, Points: []volumeExercisePoint{}}
		for _, ss := range sessions {
			for _, e := range ss.Exercises {
				if e.Name != p.Exercise {
					continue
				}
				res.Points = append(res.Points, volumeExercisePoint{
					Date:       ss.Date,
					Weight:     e.Weight,
					Reps:       e.Reps,
					Sets:       e.Sets,
					VolumeLoad: domain.RoundInt(domain.ExerciseVolumeLoad(e.Name, e.Weight, e.Reps, e.Sets, prof.BodyweightKg)),
				})
			}
		}
		return res, nil
	}

	if p.Granularity == "week" {
		type agg struct {
			vl    float64
			count int
		}
		buckets := map[string]*agg{}
		var weeks []string
		for _, ss := range sessions {
			wk := domain.MondayOf(ss.Date)
			b, ok := buckets[wk]
			if !ok {
				b = &agg{}
				buckets[wk] = b
				weeks = append(weeks, wk)
			}
			b.vl += domain.SessionVolumeLoad(ss.Exercises, prof.BodyweightKg)
			b.count++
		}
		sort.Strings(weeks)
		res := volumeWeekResult{Granularity: "week", Points: []volumeWeekPoint{}}
		for _, wk := range weeks {
			res.Points = append(res.Points, volumeWeekPoint{
				WeekStart:    wk,
				VolumeLoad:   domain.RoundInt(buckets[wk].vl),
				SessionCount: buckets[wk].count,
			})
		}
		return res, nil
	}

	// default: per-session
	res := volumeSessionResult{Granularity: "session", Points: []volumeSessionPoint{}}
	for _, ss := range sessions {
		res.Points = append(res.Points, volumeSessionPoint{
			Date:          ss.Date,
			VolumeLoad:    domain.RoundInt(domain.SessionVolumeLoad(ss.Exercises, prof.BodyweightKg)),
			ExerciseCount: len(ss.Exercises),
		})
	}
	return res, nil
}

// ---------------------------------------------------------------------------
// GET /api/summary/weekly  (port of cmd_weekly_summary)
// ---------------------------------------------------------------------------

// WeeklyExercise mirrors one exercise row in the weekly summary.
type WeeklyExercise struct {
	Name   string   `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   int      `json:"reps"`
	Sets   int      `json:"sets"`
	Notes  string   `json:"notes"`
}

// WeeklyStrengthSession is one strength session in the weekly summary.
type WeeklyStrengthSession struct {
	Date      string           `json:"date"`
	Notes     string           `json:"notes"`
	Exercises []WeeklyExercise `json:"exercises"`
}

// WeeklyStrengthStats is the strength block of the weekly summary.
type WeeklyStrengthStats struct {
	TotalSessions          int                     `json:"totalSessions"`
	AvgExercisesPerSession int                     `json:"avgExercisesPerSession"`
	Sessions               []WeeklyStrengthSession `json:"sessions"`
}

// WeeklySpinSession is one spin session in the weekly summary.
type WeeklySpinSession struct {
	Date     string            `json:"date"`
	Duration int               `json:"duration"`
	AvgHr    *int              `json:"avgHr"`
	MaxHr    *int              `json:"maxHr"`
	Rpe      *int              `json:"rpe"`
	Zones    map[string]string `json:"zones"`
}

// WeeklySpinStats is the spin block of the weekly summary.
type WeeklySpinStats struct {
	TotalSessions    int                 `json:"totalSessions"`
	TotalMinutes     int                 `json:"totalMinutes"`
	AvgDuration      int                 `json:"avgDuration"`
	AvgHeartRate     *int                `json:"avgHeartRate"`
	MaxHeartRateEver *int                `json:"maxHeartRateEver"`
	Sessions         []WeeklySpinSession `json:"sessions"`
}

// WeeklySummaryBlock is the "summary" object.
type WeeklySummaryBlock struct {
	TotalTrainingSessions int                  `json:"totalTrainingSessions"`
	SpinSessions          *WeeklySpinStats     `json:"spinSessions"`
	StrengthSessions      *WeeklyStrengthStats `json:"strengthSessions"`
}

// WeeklyResult is the GET /api/summary/weekly payload.
type WeeklyResult struct {
	Period     string             `json:"period"`
	Summary    WeeklySummaryBlock `json:"summary"`
	Evaluation []string           `json:"evaluation"`
	Advice     []string           `json:"advice"`
}

func roundHalfUp(x float64) int {
	if x < 0 {
		return int(x - 0.5)
	}
	return int(x + 0.5)
}

// WeeklySummary implements the legacy `weekly-summary` command.
func (s *Service) WeeklySummary() (*WeeklyResult, error) {
	now := time.Now().In(domain.JST)
	since := now.AddDate(0, 0, -7).Format(domain.DateLayout)
	today := now.Format(domain.DateLayout)

	spins, err := s.st.SpinSessionsInRange(since, "")
	if err != nil {
		return nil, err
	}
	strengths, err := s.st.StrengthSessionsInRange(since, "")
	if err != nil {
		return nil, err
	}

	res := &WeeklyResult{
		Period: fmt.Sprintf("%s ~ %s", since, today),
		Advice: []string{
			"故障予防のため、2日に1回の頻度を維持",
			"デロード週を4-6週間に1回設ける",
			"痛みがある場合は無理をしない",
		},
	}

	var spinStats *WeeklySpinStats
	if len(spins) > 0 {
		totalMin := 0
		var avgSum, avgCnt int
		var maxEver *int
		sessions := make([]WeeklySpinSession, 0, len(spins))
		for _, sp := range spins {
			totalMin += sp.DurationMinutes
			if sp.AvgHeartRate != nil {
				avgSum += *sp.AvgHeartRate
				avgCnt++
			}
			if sp.MaxHeartRate != nil {
				if maxEver == nil || *sp.MaxHeartRate > *maxEver {
					v := *sp.MaxHeartRate
					maxEver = &v
				}
			}
			sessions = append(sessions, WeeklySpinSession{
				Date:     sp.Date,
				Duration: sp.DurationMinutes,
				AvgHr:    sp.AvgHeartRate,
				MaxHr:    sp.MaxHeartRate,
				Rpe:      sp.RPE,
				Zones:    domain.ParseHeartRateZones(sp.Notes),
			})
		}
		spinStats = &WeeklySpinStats{
			TotalSessions: len(spins),
			TotalMinutes:  totalMin,
			AvgDuration:   roundHalfUp(float64(totalMin) / float64(len(spins))),
			Sessions:      sessions,
		}
		if avgCnt > 0 {
			v := roundHalfUp(float64(avgSum) / float64(avgCnt))
			spinStats.AvgHeartRate = &v
		}
		spinStats.MaxHeartRateEver = maxEver
	}

	var strengthStats *WeeklyStrengthStats
	if len(strengths) > 0 {
		sessions := make([]WeeklyStrengthSession, 0, len(strengths))
		totalEx := 0
		for _, ss := range strengths {
			exs := make([]WeeklyExercise, 0, len(ss.Exercises))
			for _, e := range ss.Exercises {
				exs = append(exs, WeeklyExercise{
					Name: e.Name, Weight: e.Weight, Reps: e.Reps, Sets: e.Sets, Notes: e.Notes,
				})
			}
			totalEx += len(ss.Exercises)
			sessions = append(sessions, WeeklyStrengthSession{Date: ss.Date, Notes: ss.Notes, Exercises: exs})
		}
		avg := 0
		if len(strengths) > 0 {
			avg = roundHalfUp(float64(totalEx) / float64(len(strengths)))
		}
		strengthStats = &WeeklyStrengthStats{
			TotalSessions:          len(strengths),
			AvgExercisesPerSession: avg,
			Sessions:               sessions,
		}
	}

	total := len(spins) + len(strengths)
	res.Summary = WeeklySummaryBlock{
		TotalTrainingSessions: total,
		SpinSessions:          spinStats,
		StrengthSessions:      strengthStats,
	}

	eval := []string{}
	switch {
	case total >= 7:
		eval = append(eval, "✅ トレーニング頻度：優秀（7回以上/週）")
	case total >= 5:
		eval = append(eval, "✅ トレーニング頻度：良好（5-6回/週）")
	case total >= 3:
		eval = append(eval, "⚠️ トレーニング頻度：普通（3-4回/週）")
	default:
		eval = append(eval, "⚠️ トレーニング頻度：やや少ない（2回以下/週）")
	}
	if spinStats != nil {
		if spinStats.TotalSessions >= 3 {
			eval = append(eval, "✅ スピンバイク：頻度良好")
		} else if spinStats.TotalSessions == 1 {
			eval = append(eval, "⚠️ スピンバイク：1回のみ。もう少し増やすとより良い")
		}
		if spinStats.AvgHeartRate != nil && *spinStats.AvgHeartRate > 140 {
			eval = append(eval, "✅ 平均心拍数：高め。有酸素効果大")
		} else if spinStats.AvgHeartRate != nil && *spinStats.AvgHeartRate < 120 {
			eval = append(eval, "💡 平均心拍数：やや低め。強度を上げると効果的")
		}
	}
	if strengthStats != nil {
		if strengthStats.TotalSessions >= 3 {
			eval = append(eval, "✅ 筋トレ：頻度良好")
		} else if strengthStats.TotalSessions == 1 {
			eval = append(eval, "⚠️ 筋トレ：1回のみ。継続が大切")
		}
	}
	if total >= 5 {
		eval = append(eval, "🎯 総合評価：本周は充実したトレーニングウィークでした！")
	} else {
		eval = append(eval, "💪 総合評価：来週はもう少しトレーニングを増やしてみましょう")
	}
	res.Evaluation = eval
	return res, nil
}

// ---------------------------------------------------------------------------
// GET /api/load-report  (port of training_load_analysis.py)
// ---------------------------------------------------------------------------

// LoadProfile is the profile block of the load report.
type LoadProfile struct {
	BodyweightKg float64 `json:"bodyweightKg"`
	HeightCm     float64 `json:"heightCm"`
	Bmi          float64 `json:"bmi"`
}

// LoadAcute is the acute (7-day) block.
type LoadAcute struct {
	StrengthSessions int     `json:"strengthSessions"`
	SpinSessions     int     `json:"spinSessions"`
	VolumeLoad       float64 `json:"volumeLoad"`
	SpinTrimp        float64 `json:"spinTrimp"`
	Total            float64 `json:"total"`
	VolumeLoadPerBw  float64 `json:"volumeLoadPerBw"`
}

// LoadChronic is the chronic (28-day) block.
type LoadChronic struct {
	StrengthSessions int     `json:"strengthSessions"`
	WeeklyVolumeLoad float64 `json:"weeklyVolumeLoad"`
}

// LoadBreakdown is one exercise line of the latest-session breakdown.
type LoadBreakdown struct {
	Name         string  `json:"name"`
	VolumeLoad   float64 `json:"volumeLoad"`
	IsBodyweight bool    `json:"isBodyweight"`
}

// LoadReport is the GET /api/load-report payload.
type LoadReport struct {
	AsOf                   string          `json:"asOf"`
	Profile                LoadProfile     `json:"profile"`
	Acute7d                LoadAcute       `json:"acute7d"`
	Chronic28d             LoadChronic     `json:"chronic28d"`
	ACWR                   float64         `json:"acwr"`
	Zone                   string          `json:"zone"`
	LatestSessionBreakdown []LoadBreakdown `json:"latestSessionBreakdown"`
	Recommendations        []string        `json:"recommendations"`
}

// LoadReport implements the legacy training_load_analysis.py report.
func (s *Service) LoadReport() (*LoadReport, error) {
	now := time.Now().In(domain.JST)
	today := now.Format(domain.DateLayout)
	day7 := now.AddDate(0, 0, -7).Format(domain.DateLayout)
	day28 := now.AddDate(0, 0, -28).Format(domain.DateLayout)

	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, err
	}
	strengths, err := s.st.StrengthSessionsInRange(day28, "")
	if err != nil {
		return nil, err
	}
	spins, err := s.st.SpinSessionsInRange(day28, "")
	if err != nil {
		return nil, err
	}

	type svl struct {
		date string
		vl   float64
	}
	var sVLs []svl
	var acuteStrengthVL, chronicStrengthVL float64
	acuteStrengthCount := 0
	for _, ss := range strengths {
		vl := domain.SessionVolumeLoad(ss.Exercises, prof.BodyweightKg)
		sVLs = append(sVLs, svl{ss.Date, vl})
		chronicStrengthVL += vl
		if ss.Date >= day7 {
			acuteStrengthVL += vl
			acuteStrengthCount++
		}
	}

	var acuteSpinTrimp, chronicSpinTrimp float64
	acuteSpinCount := 0
	for _, sp := range spins {
		t := domain.TRIMP(sp.DurationMinutes, sp.AvgHeartRate, prof.MaxHrEst)
		chronicSpinTrimp += t
		if sp.Date >= day7 {
			acuteSpinTrimp += t
			acuteSpinCount++
		}
	}

	acuteTotal := acuteStrengthVL + acuteSpinTrimp
	chronicWeekly := (chronicStrengthVL + chronicSpinTrimp) / 4
	acwr := 0.0
	if chronicWeekly > 0 {
		acwr = domain.Round2(acuteTotal / chronicWeekly)
	}

	rep := &LoadReport{
		AsOf: today,
		Profile: LoadProfile{
			BodyweightKg: prof.BodyweightKg,
			HeightCm:     prof.HeightCm,
			Bmi:          domain.Round1(prof.BodyweightKg / ((prof.HeightCm / 100) * (prof.HeightCm / 100))),
		},
		Acute7d: LoadAcute{
			StrengthSessions: acuteStrengthCount,
			SpinSessions:     acuteSpinCount,
			VolumeLoad:       domain.RoundInt(acuteStrengthVL),
			SpinTrimp:        domain.Round1(acuteSpinTrimp),
			Total:            domain.RoundInt(acuteTotal),
			VolumeLoadPerBw:  domain.Round1(acuteStrengthVL / prof.BodyweightKg),
		},
		Chronic28d: LoadChronic{
			StrengthSessions: len(strengths),
			WeeklyVolumeLoad: domain.RoundInt(chronicStrengthVL / 4),
		},
		ACWR:                   acwr,
		Zone:                   domain.ACWRZone(acwr),
		LatestSessionBreakdown: []LoadBreakdown{},
	}

	if len(strengths) > 0 {
		latest := strengths[len(strengths)-1]
		bd := make([]LoadBreakdown, 0, len(latest.Exercises))
		for _, e := range latest.Exercises {
			bd = append(bd, LoadBreakdown{
				Name:         e.Name,
				VolumeLoad:   domain.RoundInt(domain.ExerciseVolumeLoad(e.Name, e.Weight, e.Reps, e.Sets, prof.BodyweightKg)),
				IsBodyweight: domain.IsBodyweight(e.Name),
			})
		}
		sort.SliceStable(bd, func(i, j int) bool { return bd[i].VolumeLoad > bd[j].VolumeLoad })
		rep.LatestSessionBreakdown = bd
	}

	var rec []string
	switch {
	case acwr > 1.5:
		rec = append(rec, "今週は負荷を40%減らすデロードを推奨")
	case acwr > 1.3:
		rec = append(rec, "負荷増加は控えめに。2-3種目ずつローテーションで")
	case acwr < 0.6:
		rec = append(rec, "もう少し負荷を上げる余地があります")
	default:
		rec = append(rec, "現状のペースを維持してください")
	}
	rec = append(rec, "4-6週間に1回、デロード週を設けると故障予防に効果的")
	rep.Recommendations = rec

	return rep, nil
}
