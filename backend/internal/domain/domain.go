// Package domain holds the shared data model and the training-analysis
// primitives ported from the Hermes training-tracker skill
// (scripts/training_db.py and scripts/training_load_analysis.py).
package domain

import (
	"math"
	"strings"
	"time"
)

// JST is the fixed application timezone. The project is pinned to
// Asia/Tokyo, which observes no DST, so a fixed +09:00 offset is exact.
var JST = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Tokyo"); err == nil {
		return loc
	}
	return time.FixedZone("Asia/Tokyo", 9*60*60)
}()

// SetTZ overrides the application timezone from the TZ env var when it is
// set and loadable; otherwise the default (Asia/Tokyo) is kept.
func SetTZ(name string) {
	if name == "" {
		return
	}
	if loc, err := time.LoadLocation(name); err == nil {
		JST = loc
	}
}

// Today returns the current date in application-local time as YYYY-MM-DD.
func Today() string { return time.Now().In(JST).Format(DateLayout) }

// NowRFC3339 returns the current instant in application-local time.
func NowRFC3339() string { return time.Now().In(JST).Format(time.RFC3339) }

// DateLayout is the canonical date format used across the API and DB.
const DateLayout = "2006-01-02"

// DBTimeToRFC3339 normalises a timestamp read from the DB (either an
// RFC3339 string written by this app, or the legacy `YYYY-MM-DD HH:MM:SS`
// UTC form produced by SQLite's datetime('now')) to RFC3339 in JST.
func DBTimeToRFC3339(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(JST).Format(time.RFC3339)
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04:05.999999999"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t.In(JST).Format(time.RFC3339)
		}
	}
	return s
}

// ---------------------------------------------------------------------------
// Data model (returned by store, consumed by service / httpapi)
// ---------------------------------------------------------------------------

// Exercise is one performed-exercise row inside a strength session.
type Exercise struct {
	ID        int64
	Name      string
	Weight    *float64
	Reps      int
	Sets      int
	Notes     string
	SortOrder int
}

// StrengthSession is one calendar day of strength training.
type StrengthSession struct {
	ID        int64
	Date      string
	Notes     string
	CreatedAt string
	UpdatedAt string
	Exercises []Exercise
}

// SpinSession is one spin-bike workout.
type SpinSession struct {
	ID              int64
	Date            string
	DurationMinutes int
	AvgHeartRate    *int
	MaxHeartRate    *int
	RPE             *int
	DistanceKm      *float64
	Notes           string
	CreatedAt       string
	UpdatedAt       string
}

// RoutineExercise is one exercise inside a routine snapshot.
type RoutineExercise struct {
	ID        int64
	Name      string
	Weight    *float64
	Reps      int
	Sets      int
	SortOrder int
}

// RoutineSnapshot is the preset routine as of a given date.
type RoutineSnapshot struct {
	Date      string
	Exercises []RoutineExercise
}

// Profile is the single-row user profile.
type Profile struct {
	BodyweightKg float64
	HeightCm     float64
	MaxHrEst     int
}

// ---------------------------------------------------------------------------
// Volume Load (domain.md / training_load_analysis.py)
// ---------------------------------------------------------------------------

// BodyweightExercises is the current BODYWEIGHT_EXERCISES set from
// training_load_analysis.py. Rows with a NULL weight whose name is in this
// set count as profile.BodyweightKg; any other NULL weight counts as 0.
var BodyweightExercises = map[string]bool{
	"懸垂":          true,
	"懸垂レッグレイズ":    true,
	"ディップス":       true,
	"バックエクステンション": true,
	"ベンチレッグレイズ":   true,
}

// IsBodyweight reports whether name is a tracked bodyweight exercise.
func IsBodyweight(name string) bool { return BodyweightExercises[name] }

// EffectiveWeight returns the weight to use for Volume Load: the recorded
// weight when present, else the bodyweight for tracked bodyweight
// exercises, else 0.
func EffectiveWeight(name string, weight *float64, bodyweightKg float64) float64 {
	if weight != nil {
		return *weight
	}
	if BodyweightExercises[name] {
		return bodyweightKg
	}
	return 0
}

// ExerciseVolumeLoad is weight x reps x sets for one exercise row.
func ExerciseVolumeLoad(name string, weight *float64, reps, sets int, bodyweightKg float64) float64 {
	return EffectiveWeight(name, weight, bodyweightKg) * float64(reps) * float64(sets)
}

// SessionVolumeLoad sums ExerciseVolumeLoad over a session's exercises.
func SessionVolumeLoad(exs []Exercise, bodyweightKg float64) float64 {
	var total float64
	for _, e := range exs {
		total += ExerciseVolumeLoad(e.Name, e.Weight, e.Reps, e.Sets, bodyweightKg)
	}
	return total
}

// TRIMP is the cardio training-impulse: duration x (avgHR / maxHREst),
// rounded to 1 decimal. Returns 0 when avgHR is missing or zero.
func TRIMP(durationMin int, avgHR *int, maxHREst int) float64 {
	if avgHR == nil || *avgHR == 0 || maxHREst <= 0 {
		return 0
	}
	return math.Round(float64(durationMin)*(float64(*avgHR)/float64(maxHREst))*10) / 10
}

// AutoRPE derives an RPE from heart-rate data: round(avg/max*10) clamped
// to 1..10. ok is false when the inputs are insufficient.
func AutoRPE(avgHR, maxHR *int) (rpe int, ok bool) {
	if avgHR == nil || maxHR == nil || *maxHR <= 0 {
		return 0, false
	}
	v := int(math.Round(float64(*avgHR) / float64(*maxHR) * 10))
	if v < 1 {
		v = 1
	}
	if v > 10 {
		v = 10
	}
	return v, true
}

// SessionKind classifies a strength session from its free-text notes into
// one of: bodyweight_only / fw_only / bodyweight_and_fw / other.
func SessionKind(notes string) string {
	n := notes
	for _, neg := range []string{"自重なし", "自重無し", "自重ナシ"} {
		n = strings.ReplaceAll(n, neg, "")
	}
	hasBW := strings.Contains(n, "自重")
	hasFW := strings.Contains(n, "フリーウェイト") || strings.Contains(n, "FW") || strings.Contains(n, "ＦＷ")
	switch {
	case hasBW && hasFW:
		return "bodyweight_and_fw"
	case hasBW:
		return "bodyweight_only"
	case hasFW:
		return "fw_only"
	default:
		return "other"
	}
}

// ACWRZone maps an ACWR value to a risk zone. The branch order mirrors
// training_load_analysis.py exactly.
func ACWRZone(acwr float64) string {
	switch {
	case acwr == 0:
		return "no_data"
	case acwr >= 0.8 && acwr <= 1.3:
		return "safe"
	case acwr > 1.3 && acwr <= 1.5:
		return "caution"
	case acwr > 1.5:
		return "warning"
	case acwr < 0.6:
		return "low"
	default:
		return "slightly_low"
	}
}

// MondayOf returns the Monday (YYYY-MM-DD) of the ISO week containing date.
func MondayOf(date string) string {
	t, err := time.ParseInLocation(DateLayout, date, JST)
	if err != nil {
		return date
	}
	offset := (int(t.Weekday()) + 6) % 7 // Mon=0 .. Sun=6
	return t.AddDate(0, 0, -offset).Format(DateLayout)
}

// Round1 / Round2 / RoundInt are the rounding helpers used for API output.
func Round1(x float64) float64   { return math.Round(x*10) / 10 }
func Round2(x float64) float64   { return math.Round(x*100) / 100 }
func RoundInt(x float64) float64 { return math.Round(x) }
