package store

import (
	"database/sql"
	"errors"
	"fmt"

	"training-record/internal/domain"
)

const strengthCols = `id, date, COALESCE(notes,''), COALESCE(created_at,''), COALESCE(updated_at,'')`

func scanStrengthRow(sc interface{ Scan(...any) error }) (*domain.StrengthSession, error) {
	var s domain.StrengthSession
	if err := sc.Scan(&s.ID, &s.Date, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadExercises(q rowQuerier, sessionID int64) ([]domain.Exercise, error) {
	rows, err := q.Query(
		`SELECT id, name, weight, reps, COALESCE(sets,1), COALESCE(notes,''), COALESCE(sort_order,0)
		   FROM exercises WHERE session_id = ? ORDER BY sort_order, id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Exercise{}
	for rows.Next() {
		var e domain.Exercise
		var w sql.NullFloat64
		if err := rows.Scan(&e.ID, &e.Name, &w, &e.Reps, &e.Sets, &e.Notes, &e.SortOrder); err != nil {
			return nil, err
		}
		e.Weight = nfloat(w)
		out = append(out, e)
	}
	return out, rows.Err()
}

func getStrength(q rowQuerier, date string) (*domain.StrengthSession, error) {
	sess, err := scanStrengthRow(q.QueryRow(
		`SELECT `+strengthCols+` FROM strength_sessions WHERE date = ?`, date))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	exs, err := loadExercises(q, sess.ID)
	if err != nil {
		return nil, err
	}
	sess.Exercises = exs
	return sess, nil
}

// GetStrengthByDate returns the session for date, or (nil, nil) if absent.
func (s *Store) GetStrengthByDate(date string) (*domain.StrengthSession, error) {
	return getStrength(s.db, date)
}

// ListStrength returns sessions (with exercises) in the optional [from,to]
// range, newest first, plus the total count ignoring limit/offset.
func (s *Store) ListStrength(from, to string, limit, offset int) ([]domain.StrengthSession, int, error) {
	where, args := rangeClause("date", from, to)

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM strength_sessions`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.Query(
		`SELECT `+strengthCols+` FROM strength_sessions`+where+` ORDER BY date DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var sessions []domain.StrengthSession
	byID := map[int64]int{}
	var ids []int64
	for rows.Next() {
		s, err := scanStrengthRow(rows)
		if err != nil {
			return nil, 0, err
		}
		s.Exercises = []domain.Exercise{}
		sessions = append(sessions, *s)
		byID[s.ID] = len(sessions) - 1
		ids = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return []domain.StrengthSession{}, total, nil
	}

	marks, exArgs := placeholders(ids)
	exRows, err := s.db.Query(
		`SELECT session_id, id, name, weight, reps, COALESCE(sets,1), COALESCE(notes,''), COALESCE(sort_order,0)
		   FROM exercises WHERE session_id IN (`+marks+`) ORDER BY session_id, sort_order, id`, exArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer exRows.Close()
	for exRows.Next() {
		var sid int64
		var e domain.Exercise
		var w sql.NullFloat64
		if err := exRows.Scan(&sid, &e.ID, &e.Name, &w, &e.Reps, &e.Sets, &e.Notes, &e.SortOrder); err != nil {
			return nil, 0, err
		}
		e.Weight = nfloat(w)
		idx := byID[sid]
		sessions[idx].Exercises = append(sessions[idx].Exercises, e)
	}
	return sessions, total, exRows.Err()
}

func insertExercises(tx *sql.Tx, sessionID int64, exs []domain.Exercise, startOrder int) error {
	if len(exs) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(
		`INSERT INTO exercises (session_id, name, weight, reps, sets, notes, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for i, e := range exs {
		sets := e.Sets
		if sets < 1 {
			sets = 1
		}
		if _, err := stmt.Exec(sessionID, e.Name, pfloat(e.Weight), e.Reps, sets, e.Notes, startOrder+i); err != nil {
			return err
		}
	}
	return nil
}

// CreateStrength inserts a brand-new session; ErrDateExists if the date is
// already present.
func (s *Store) CreateStrength(date, notes string, exs []domain.Exercise) (*domain.StrengthSession, error) {
	var out *domain.StrengthSession
	err := s.tx(func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM strength_sessions WHERE date = ?`, date).Scan(&exists); err != nil {
			return err
		}
		if exists > 0 {
			return ErrDateExists
		}
		now := domain.NowRFC3339()
		res, err := tx.Exec(
			`INSERT INTO strength_sessions (date, notes, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			date, notes, now, now)
		if err != nil {
			return err
		}
		sid, _ := res.LastInsertId()
		if err := insertExercises(tx, sid, exs, 0); err != nil {
			return err
		}
		out, err = getStrength(tx, date)
		return err
	})
	return out, err
}

// ReplaceStrength upserts the session for date, replacing notes and all
// exercises. created reports whether the row was newly inserted.
func (s *Store) ReplaceStrength(date, notes string, exs []domain.Exercise) (sess *domain.StrengthSession, created bool, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		var sid int64
		row := tx.QueryRow(`SELECT id FROM strength_sessions WHERE date = ?`, date)
		scanErr := row.Scan(&sid)
		now := domain.NowRFC3339()
		if errors.Is(scanErr, sql.ErrNoRows) {
			created = true
			res, e := tx.Exec(
				`INSERT INTO strength_sessions (date, notes, created_at, updated_at) VALUES (?, ?, ?, ?)`,
				date, notes, now, now)
			if e != nil {
				return e
			}
			sid, _ = res.LastInsertId()
		} else if scanErr != nil {
			return scanErr
		} else {
			if _, e := tx.Exec(`UPDATE strength_sessions SET notes = ?, updated_at = ? WHERE id = ?`, notes, now, sid); e != nil {
				return e
			}
			if _, e := tx.Exec(`DELETE FROM exercises WHERE session_id = ?`, sid); e != nil {
				return e
			}
		}
		if e := insertExercises(tx, sid, exs, 0); e != nil {
			return e
		}
		sess, err = getStrength(tx, date)
		return err
	})
	return sess, created, err
}

// UpdateStrengthNotes sets only notes; (nil, nil) if the session is absent.
func (s *Store) UpdateStrengthNotes(date, notes string) (*domain.StrengthSession, error) {
	var out *domain.StrengthSession
	err := s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE strength_sessions SET notes = ?, updated_at = ? WHERE date = ?`,
			notes, domain.NowRFC3339(), date)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		out, err = getStrength(tx, date)
		return err
	})
	return out, err
}

// AppendStrengthExercises appends exercises to the session for date,
// creating the session if it does not exist (mirrors the legacy
// record-strength --append behaviour). When hasNotes is true, notes is
// appended to the existing notes with "; ".
func (s *Store) AppendStrengthExercises(date string, exs []domain.Exercise, notes string, hasNotes bool) (*domain.StrengthSession, error) {
	var out *domain.StrengthSession
	err := s.tx(func(tx *sql.Tx) error {
		var sid int64
		var curNotes string
		row := tx.QueryRow(`SELECT id, COALESCE(notes,'') FROM strength_sessions WHERE date = ?`, date)
		scanErr := row.Scan(&sid, &curNotes)
		now := domain.NowRFC3339()
		if errors.Is(scanErr, sql.ErrNoRows) {
			newNotes := ""
			if hasNotes {
				newNotes = notes
			}
			res, e := tx.Exec(
				`INSERT INTO strength_sessions (date, notes, created_at, updated_at) VALUES (?, ?, ?, ?)`,
				date, newNotes, now, now)
			if e != nil {
				return e
			}
			sid, _ = res.LastInsertId()
			if e := insertExercises(tx, sid, exs, 0); e != nil {
				return e
			}
			out, e = getStrength(tx, date)
			return e
		} else if scanErr != nil {
			return scanErr
		}

		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM exercises WHERE session_id = ?`, sid).Scan(&count); err != nil {
			return err
		}
		if err := insertExercises(tx, sid, exs, count); err != nil {
			return err
		}
		merged := curNotes
		if hasNotes && notes != "" {
			if curNotes != "" {
				merged = curNotes + "; " + notes
			} else {
				merged = notes
			}
		}
		if _, err := tx.Exec(`UPDATE strength_sessions SET notes = ?, updated_at = ? WHERE id = ?`, merged, now, sid); err != nil {
			return err
		}
		var e error
		out, e = getStrength(tx, date)
		return e
	})
	return out, err
}

// UpdateExercise replaces one exercise row (id fixed) belonging to the
// session for date. Returns ErrNotFound if the session is absent,
// ErrExerciseNotFound if the id is not part of that session.
func (s *Store) UpdateExercise(date string, id int64, ex domain.Exercise) (*domain.StrengthSession, error) {
	var out *domain.StrengthSession
	err := s.tx(func(tx *sql.Tx) error {
		var sid int64
		if err := tx.QueryRow(`SELECT id FROM strength_sessions WHERE date = ?`, date).Scan(&sid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		sets := ex.Sets
		if sets < 1 {
			sets = 1
		}
		res, err := tx.Exec(
			`UPDATE exercises SET name = ?, weight = ?, reps = ?, sets = ?, notes = ? WHERE id = ? AND session_id = ?`,
			ex.Name, pfloat(ex.Weight), ex.Reps, sets, ex.Notes, id, sid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrExerciseNotFound
		}
		if _, err := tx.Exec(`UPDATE strength_sessions SET updated_at = ? WHERE id = ?`, domain.NowRFC3339(), sid); err != nil {
			return err
		}
		out, err = getStrength(tx, date)
		return err
	})
	return out, err
}

// DeleteExercise removes one exercise row from the session for date.
// found is false when either the session or the exercise is absent.
func (s *Store) DeleteExercise(date string, id int64) (found bool, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		var sid int64
		if e := tx.QueryRow(`SELECT id FROM strength_sessions WHERE date = ?`, date).Scan(&sid); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return nil
			}
			return e
		}
		res, e := tx.Exec(`DELETE FROM exercises WHERE id = ? AND session_id = ?`, id, sid)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n > 0 {
			found = true
			_, e = tx.Exec(`UPDATE strength_sessions SET updated_at = ? WHERE id = ?`, domain.NowRFC3339(), sid)
		}
		return e
	})
	return found, err
}

// ReorderExercises sets sort_order from the position of each id in
// orderedIDs. Every id must belong to the session for date, else
// ErrExerciseNotFound. ErrNotFound if the session is absent.
func (s *Store) ReorderExercises(date string, orderedIDs []int64) (*domain.StrengthSession, error) {
	var out *domain.StrengthSession
	err := s.tx(func(tx *sql.Tx) error {
		var sid int64
		if err := tx.QueryRow(`SELECT id FROM strength_sessions WHERE date = ?`, date).Scan(&sid); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		for pos, id := range orderedIDs {
			res, err := tx.Exec(`UPDATE exercises SET sort_order = ? WHERE id = ? AND session_id = ?`, pos, id, sid)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return fmt.Errorf("%w: id %d", ErrExerciseNotFound, id)
			}
		}
		if _, err := tx.Exec(`UPDATE strength_sessions SET updated_at = ? WHERE id = ?`, domain.NowRFC3339(), sid); err != nil {
			return err
		}
		var err error
		out, err = getStrength(tx, date)
		return err
	})
	return out, err
}

// DeleteStrength removes the session for date (exercises cascade).
func (s *Store) DeleteStrength(date string) (found bool, err error) {
	res, err := s.db.Exec(`DELETE FROM strength_sessions WHERE date = ?`, date)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// LastStrength returns the most recent session by date, or (nil, nil).
func (s *Store) LastStrength() (*domain.StrengthSession, error) {
	var date string
	err := s.db.QueryRow(`SELECT date FROM strength_sessions ORDER BY date DESC LIMIT 1`).Scan(&date)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return getStrength(s.db, date)
}

// StrengthSessionsInRange returns sessions (with exercises) in [from,to]
// (either bound optional) ordered by date ascending. Used by analytics.
func (s *Store) StrengthSessionsInRange(from, to string) ([]domain.StrengthSession, error) {
	where, args := rangeClause("date", from, to)
	rows, err := s.db.Query(`SELECT `+strengthCols+` FROM strength_sessions`+where+` ORDER BY date ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []domain.StrengthSession
	byID := map[int64]int{}
	var ids []int64
	for rows.Next() {
		sess, err := scanStrengthRow(rows)
		if err != nil {
			return nil, err
		}
		sess.Exercises = []domain.Exercise{}
		sessions = append(sessions, *sess)
		byID[sess.ID] = len(sessions) - 1
		ids = append(ids, sess.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	marks, exArgs := placeholders(ids)
	exRows, err := s.db.Query(
		`SELECT session_id, id, name, weight, reps, COALESCE(sets,1), COALESCE(notes,''), COALESCE(sort_order,0)
		   FROM exercises WHERE session_id IN (`+marks+`) ORDER BY session_id, sort_order, id`, exArgs...)
	if err != nil {
		return nil, err
	}
	defer exRows.Close()
	for exRows.Next() {
		var sid int64
		var e domain.Exercise
		var w sql.NullFloat64
		if err := exRows.Scan(&sid, &e.ID, &e.Name, &w, &e.Reps, &e.Sets, &e.Notes, &e.SortOrder); err != nil {
			return nil, err
		}
		e.Weight = nfloat(w)
		sessions[byID[sid]].Exercises = append(sessions[byID[sid]].Exercises, e)
	}
	return sessions, exRows.Err()
}
