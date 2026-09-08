package store

import (
	"database/sql"
	"errors"

	"training-record/internal/domain"
)

// RoutineHistoryEntry is one snapshot summary.
type RoutineHistoryEntry struct {
	Date          string
	ExerciseCount int
}

// LatestRoutineDate returns the newest snapshot date, or "" if none exist.
func (s *Store) LatestRoutineDate() (string, error) {
	var date string
	err := s.db.QueryRow(`SELECT date FROM routine_snapshots ORDER BY date DESC LIMIT 1`).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return date, err
}

// RoutineByDate returns the exercise rows of the snapshot for date, ordered
// by sort_order. Empty slice (not nil) when the date has no rows.
func (s *Store) RoutineByDate(date string) ([]domain.RoutineExercise, error) {
	rows, err := s.db.Query(
		`SELECT exercise_name, weight, reps, COALESCE(sets,1), COALESCE(sort_order,0)
		   FROM routine_snapshots WHERE date = ? ORDER BY sort_order, id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RoutineExercise{}
	for rows.Next() {
		var e domain.RoutineExercise
		var w sql.NullFloat64
		if err := rows.Scan(&e.Name, &w, &e.Reps, &e.Sets, &e.SortOrder); err != nil {
			return nil, err
		}
		e.Weight = nfloat(w)
		out = append(out, e)
	}
	return out, rows.Err()
}

// RoutineHistory returns every snapshot with its exercise count, newest
// first.
func (s *Store) RoutineHistory() ([]RoutineHistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT date, COUNT(*) FROM routine_snapshots GROUP BY date ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RoutineHistoryEntry{}
	for rows.Next() {
		var e RoutineHistoryEntry
		if err := rows.Scan(&e.Date, &e.ExerciseCount); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReplaceRoutine rebuilds the snapshot for date from exs (sort_order =
// slice index).
func (s *Store) ReplaceRoutine(date string, exs []domain.RoutineExercise) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM routine_snapshots WHERE date = ?`, date); err != nil {
			return err
		}
		stmt, err := tx.Prepare(
			`INSERT INTO routine_snapshots (date, exercise_name, weight, reps, sets, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		now := domain.NowRFC3339()
		for i, e := range exs {
			sets := e.Sets
			if sets < 1 {
				sets = 1
			}
			if _, err := stmt.Exec(date, e.Name, pfloat(e.Weight), e.Reps, sets, i, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateRoutineExercise patches weight/reps/sets of one exercise in the
// latest snapshot (in place, mirroring the legacy update-exercise). A
// missing exercise is appended at the end. Returns the action
// ("updated"/"added") and the routine date. ErrNotFound if no routine
// exists at all.
func (s *Store) UpdateRoutineExercise(name string, weight *float64, reps, sets *int) (action, routineDate string, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		var date string
		if e := tx.QueryRow(`SELECT date FROM routine_snapshots ORDER BY date DESC LIMIT 1`).Scan(&date); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		routineDate = date

		var cnt int
		if e := tx.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE date = ? AND exercise_name = ?`, date, name).Scan(&cnt); e != nil {
			return e
		}
		if cnt > 0 {
			set := ""
			var args []any
			add := func(frag string, v any) {
				if set != "" {
					set += ", "
				}
				set += frag
				args = append(args, v)
			}
			if weight != nil {
				add("weight = ?", *weight)
			}
			if reps != nil {
				add("reps = ?", *reps)
			}
			if sets != nil {
				add("sets = ?", *sets)
			}
			if set == "" {
				action = "updated"
				return nil
			}
			args = append(args, date, name)
			if _, e := tx.Exec(`UPDATE routine_snapshots SET `+set+` WHERE date = ? AND exercise_name = ?`, args...); e != nil {
				return e
			}
			action = "updated"
			return nil
		}

		var nextOrder int
		if e := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM routine_snapshots WHERE date = ?`, date).Scan(&nextOrder); e != nil {
			return e
		}
		r := 10
		if reps != nil {
			r = *reps
		}
		st := 1
		if sets != nil {
			st = *sets
		}
		if _, e := tx.Exec(
			`INSERT INTO routine_snapshots (date, exercise_name, weight, reps, sets, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			date, name, pfloat(weight), r, st, nextOrder, domain.NowRFC3339()); e != nil {
			return e
		}
		action = "added"
		return nil
	})
	return action, routineDate, err
}
