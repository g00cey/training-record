package store

import (
	"database/sql"
	"sort"

	"training-record/internal/domain"
)

// GetProfile returns the single profile row (id=1).
func (s *Store) GetProfile() (domain.Profile, error) {
	var p domain.Profile
	err := s.db.QueryRow(
		`SELECT bodyweight_kg, height_cm, max_hr_est FROM profile WHERE id = 1`).
		Scan(&p.BodyweightKg, &p.HeightCm, &p.MaxHrEst)
	return p, err
}

// UpdateProfile applies the non-nil fields and returns the updated row.
func (s *Store) UpdateProfile(bodyweightKg, heightCm *float64, maxHrEst *int) (domain.Profile, error) {
	set := ""
	var args []any
	add := func(frag string, v any) {
		if set != "" {
			set += ", "
		}
		set += frag
		args = append(args, v)
	}
	if bodyweightKg != nil {
		add("bodyweight_kg = ?", *bodyweightKg)
	}
	if heightCm != nil {
		add("height_cm = ?", *heightCm)
	}
	if maxHrEst != nil {
		add("max_hr_est = ?", *maxHrEst)
	}
	if set != "" {
		add("updated_at = ?", domain.NowRFC3339())
		if _, err := s.db.Exec(`UPDATE profile SET `+set+` WHERE id = 1`, args...); err != nil {
			return domain.Profile{}, err
		}
	}
	return s.GetProfile()
}

// ExerciseNames returns the sorted DISTINCT union of exercise names from
// performed exercises and routine snapshots.
func (s *Store) ExerciseNames() ([]string, error) {
	seen := map[string]struct{}{}
	for _, q := range []string{
		`SELECT DISTINCT name FROM exercises`,
		`SELECT DISTINCT exercise_name FROM routine_snapshots`,
	} {
		rows, err := s.db.Query(q)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return nil, err
			}
			seen[n] = struct{}{}
		}
		rows.Close()
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// SummaryData is the raw material for GET /api/summary.
type SummaryData struct {
	StrengthInPeriod  int
	SpinInPeriod      int
	TotalStrength     int
	TotalSpin         int
	LatestStrength    *string
	LatestSpin        *string
	LatestRoutineDate *string
}

// Summary computes the counts for GET /api/summary since the given date.
func (s *Store) Summary(since string) (SummaryData, error) {
	var d SummaryData
	q := func(dst *int, query string, args ...any) error {
		return s.db.QueryRow(query, args...).Scan(dst)
	}
	if err := q(&d.StrengthInPeriod, `SELECT COUNT(*) FROM strength_sessions WHERE date >= ?`, since); err != nil {
		return d, err
	}
	if err := q(&d.SpinInPeriod, `SELECT COUNT(*) FROM spin_sessions WHERE date >= ?`, since); err != nil {
		return d, err
	}
	if err := q(&d.TotalStrength, `SELECT COUNT(*) FROM strength_sessions`); err != nil {
		return d, err
	}
	if err := q(&d.TotalSpin, `SELECT COUNT(*) FROM spin_sessions`); err != nil {
		return d, err
	}
	d.LatestStrength = s.scalarDate(`SELECT date FROM strength_sessions ORDER BY date DESC LIMIT 1`)
	d.LatestSpin = s.scalarDate(`SELECT date FROM spin_sessions ORDER BY date DESC LIMIT 1`)
	d.LatestRoutineDate = s.scalarDate(`SELECT date FROM routine_snapshots ORDER BY date DESC LIMIT 1`)
	return d, nil
}

func (s *Store) scalarDate(query string) *string {
	var v string
	if err := s.db.QueryRow(query).Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return nil
	}
	return &v
}
