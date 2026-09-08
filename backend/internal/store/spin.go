package store

import (
	"database/sql"
	"errors"

	"training-record/internal/domain"
)

const spinCols = `id, date, duration_minutes, avg_heart_rate, max_heart_rate, rpe, distance_km,
	COALESCE(notes,''), COALESCE(created_at,''), COALESCE(updated_at,'')`

func scanSpin(sc interface{ Scan(...any) error }) (*domain.SpinSession, error) {
	var sp domain.SpinSession
	var avg, max, rpe sql.NullInt64
	var dist sql.NullFloat64
	if err := sc.Scan(&sp.ID, &sp.Date, &sp.DurationMinutes, &avg, &max, &rpe, &dist,
		&sp.Notes, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
		return nil, err
	}
	sp.AvgHeartRate = nint(avg)
	sp.MaxHeartRate = nint(max)
	sp.RPE = nint(rpe)
	sp.DistanceKm = nfloat(dist)
	return &sp, nil
}

// GetSpinByDate returns the spin session for date, or (nil, nil).
func (s *Store) GetSpinByDate(date string) (*domain.SpinSession, error) {
	sp, err := scanSpin(s.db.QueryRow(`SELECT `+spinCols+` FROM spin_sessions WHERE date = ?`, date))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// ListSpin returns spin sessions in the optional [from,to] range, newest
// first, with the total count ignoring limit/offset.
func (s *Store) ListSpin(from, to string, limit, offset int) ([]domain.SpinSession, int, error) {
	where, args := rangeClause("date", from, to)
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM spin_sessions`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.Query(`SELECT `+spinCols+` FROM spin_sessions`+where+` ORDER BY date DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []domain.SpinSession{}
	for rows.Next() {
		sp, err := scanSpin(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *sp)
	}
	return out, total, rows.Err()
}

// CreateSpin inserts a new spin session; ErrDateExists if date is taken.
func (s *Store) CreateSpin(sp domain.SpinSession) (*domain.SpinSession, error) {
	var exists int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM spin_sessions WHERE date = ?`, sp.Date).Scan(&exists); err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, ErrDateExists
	}
	now := domain.NowRFC3339()
	_, err := s.db.Exec(
		`INSERT INTO spin_sessions (date, duration_minutes, avg_heart_rate, max_heart_rate, rpe, distance_km, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sp.Date, sp.DurationMinutes, pint(sp.AvgHeartRate), pint(sp.MaxHeartRate), pint(sp.RPE), pfloat(sp.DistanceKm), sp.Notes, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetSpinByDate(sp.Date)
}

// ReplaceSpin upserts the spin session for sp.Date. created reports a new
// row.
func (s *Store) ReplaceSpin(sp domain.SpinSession) (out *domain.SpinSession, created bool, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		var id int64
		scanErr := tx.QueryRow(`SELECT id FROM spin_sessions WHERE date = ?`, sp.Date).Scan(&id)
		now := domain.NowRFC3339()
		if errors.Is(scanErr, sql.ErrNoRows) {
			created = true
			_, e := tx.Exec(
				`INSERT INTO spin_sessions (date, duration_minutes, avg_heart_rate, max_heart_rate, rpe, distance_km, notes, created_at, updated_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				sp.Date, sp.DurationMinutes, pint(sp.AvgHeartRate), pint(sp.MaxHeartRate), pint(sp.RPE), pfloat(sp.DistanceKm), sp.Notes, now, now)
			return e
		} else if scanErr != nil {
			return scanErr
		}
		_, e := tx.Exec(
			`UPDATE spin_sessions SET duration_minutes = ?, avg_heart_rate = ?, max_heart_rate = ?, rpe = ?, distance_km = ?, notes = ?, updated_at = ?
			 WHERE date = ?`,
			sp.DurationMinutes, pint(sp.AvgHeartRate), pint(sp.MaxHeartRate), pint(sp.RPE), pfloat(sp.DistanceKm), sp.Notes, now, sp.Date)
		return e
	})
	if err != nil {
		return nil, false, err
	}
	out, err = s.GetSpinByDate(sp.Date)
	return out, created, err
}

// UpdateSpinFull overwrites every mutable column of the spin session for
// sp.Date (used by PATCH after the service merges the changes).
// (nil, nil) if the session is absent.
func (s *Store) UpdateSpinFull(sp domain.SpinSession) (*domain.SpinSession, error) {
	res, err := s.db.Exec(
		`UPDATE spin_sessions SET duration_minutes = ?, avg_heart_rate = ?, max_heart_rate = ?, rpe = ?, distance_km = ?, notes = ?, updated_at = ?
		 WHERE date = ?`,
		sp.DurationMinutes, pint(sp.AvgHeartRate), pint(sp.MaxHeartRate), pint(sp.RPE), pfloat(sp.DistanceKm), sp.Notes, domain.NowRFC3339(), sp.Date)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, nil
	}
	return s.GetSpinByDate(sp.Date)
}

// DeleteSpin removes the spin session for date.
func (s *Store) DeleteSpin(date string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM spin_sessions WHERE date = ?`, date)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// LastSpin returns the most recent spin session by date, or (nil, nil).
func (s *Store) LastSpin() (*domain.SpinSession, error) {
	sp, err := scanSpin(s.db.QueryRow(`SELECT ` + spinCols + ` FROM spin_sessions ORDER BY date DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return sp, nil
}

// SpinSessionsInRange returns spin sessions in [from,to] (bounds optional)
// ordered by date ascending.
func (s *Store) SpinSessionsInRange(from, to string) ([]domain.SpinSession, error) {
	where, args := rangeClause("date", from, to)
	rows, err := s.db.Query(`SELECT `+spinCols+` FROM spin_sessions`+where+` ORDER BY date ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SpinSession
	for rows.Next() {
		sp, err := scanSpin(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sp)
	}
	return out, rows.Err()
}
