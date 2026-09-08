package store

import (
	"database/sql"
	"errors"

	"training-record/internal/domain"
)

// RoutineHistoryEntry is one snapshot summary (date + exercise count).
type RoutineHistoryEntry struct {
	Date          string
	ExerciseCount int
}

// Preset is the metadata row for a named preset.
type Preset struct {
	Name      string
	SortOrder int
}

// PresetInfo is a preset plus a summary of its latest snapshot.
type PresetInfo struct {
	Name          string
	SortOrder     int
	ExerciseCount int
	LatestDate    *string
}

// ---------------------------------------------------------------------------
// preset metadata
// ---------------------------------------------------------------------------

// ListPresets returns every preset (sortOrder order) with latest-snapshot
// exercise count and date.
func (s *Store) ListPresets() ([]PresetInfo, error) {
	rows, err := s.db.Query(`
		SELECT p.name, p.sort_order,
		       (SELECT COUNT(*) FROM routine_snapshots r
		          WHERE r.preset = p.name
		            AND r.date = (SELECT MAX(date) FROM routine_snapshots WHERE preset = p.name)),
		       (SELECT MAX(date) FROM routine_snapshots WHERE preset = p.name)
		  FROM presets p
		 ORDER BY p.sort_order, p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PresetInfo{}
	for rows.Next() {
		var pi PresetInfo
		var latest sql.NullString
		if err := rows.Scan(&pi.Name, &pi.SortOrder, &pi.ExerciseCount, &latest); err != nil {
			return nil, err
		}
		if latest.Valid {
			d := latest.String
			pi.LatestDate = &d
		}
		out = append(out, pi)
	}
	return out, rows.Err()
}

// GetPreset returns the preset metadata, or (nil, nil) if it does not exist.
func (s *Store) GetPreset(name string) (*Preset, error) {
	var p Preset
	err := s.db.QueryRow(`SELECT name, sort_order FROM presets WHERE name = ?`, name).Scan(&p.Name, &p.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// CreatePreset inserts a new preset. sortOrder nil => appended at the end.
// ErrPresetExists if the name is taken.
func (s *Store) CreatePreset(name string, sortOrder *int) (*Preset, error) {
	var out *Preset
	err := s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM presets WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrPresetExists
		}
		order := 0
		if sortOrder != nil {
			order = *sortOrder
		} else {
			if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM presets`).Scan(&order); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO presets (name, sort_order, created_at) VALUES (?, ?, ?)`,
			name, order, domain.NowRFC3339()); err != nil {
			return err
		}
		out = &Preset{Name: name, SortOrder: order}
		return nil
	})
	return out, err
}

// EnsurePreset creates the preset (appended) if it does not already exist.
func (s *Store) EnsurePreset(name string) error {
	return s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM presets WHERE name = ?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		var order int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM presets`).Scan(&order); err != nil {
			return err
		}
		_, err := tx.Exec(
			`INSERT INTO presets (name, sort_order, created_at) VALUES (?, ?, ?)`,
			name, order, domain.NowRFC3339())
		return err
	})
}

// ReorderPresets sets sort_order from the position of each name in order;
// names absent from order keep their relative order, appended after.
func (s *Store) ReorderPresets(order []string) ([]PresetInfo, error) {
	err := s.tx(func(tx *sql.Tx) error {
		rows, err := tx.Query(`SELECT name FROM presets ORDER BY sort_order, name`)
		if err != nil {
			return err
		}
		var current []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return err
			}
			current = append(current, n)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		exists := map[string]bool{}
		for _, n := range current {
			exists[n] = true
		}
		placed := map[string]bool{}
		final := make([]string, 0, len(current))
		for _, n := range order {
			if exists[n] && !placed[n] {
				final = append(final, n)
				placed[n] = true
			}
		}
		for _, n := range current {
			if !placed[n] {
				final = append(final, n)
				placed[n] = true
			}
		}

		stmt, err := tx.Prepare(`UPDATE presets SET sort_order = ? WHERE name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i, n := range final {
			if _, err := stmt.Exec(i, n); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.ListPresets()
}

// DeletePreset removes the preset and all of its routine_snapshots.
func (s *Store) DeletePreset(name string) (found bool, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		res, e := tx.Exec(`DELETE FROM presets WHERE name = ?`, name)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		found = true
		_, e = tx.Exec(`DELETE FROM routine_snapshots WHERE preset = ?`, name)
		return e
	})
	return found, err
}

// ---------------------------------------------------------------------------
// preset snapshots
// ---------------------------------------------------------------------------

// LatestPresetDate returns the newest snapshot date for preset, or "".
func (s *Store) LatestPresetDate(preset string) (string, error) {
	var d sql.NullString
	if err := s.db.QueryRow(`SELECT MAX(date) FROM routine_snapshots WHERE preset = ?`, preset).Scan(&d); err != nil {
		return "", err
	}
	if !d.Valid {
		return "", nil
	}
	return d.String, nil
}

func scanRoutineExercises(rows *sql.Rows) ([]domain.RoutineExercise, error) {
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

// PresetSnapshot returns the exercise rows for (preset, date), sort_order order.
func (s *Store) PresetSnapshot(preset, date string) ([]domain.RoutineExercise, error) {
	rows, err := s.db.Query(
		`SELECT exercise_name, weight, reps, COALESCE(sets,1), COALESCE(sort_order,0)
		   FROM routine_snapshots WHERE preset = ? AND date = ? ORDER BY sort_order, id`, preset, date)
	if err != nil {
		return nil, err
	}
	return scanRoutineExercises(rows)
}

// PresetHistory returns every snapshot of preset, newest first.
func (s *Store) PresetHistory(preset string) ([]RoutineHistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT date, COUNT(*) FROM routine_snapshots WHERE preset = ? GROUP BY date ORDER BY date DESC`, preset)
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

// ReplacePresetSnapshot rebuilds the (preset, date) snapshot from exs
// (sort_order = slice index).
func (s *Store) ReplacePresetSnapshot(preset, date string, exs []domain.RoutineExercise) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM routine_snapshots WHERE preset = ? AND date = ?`, preset, date); err != nil {
			return err
		}
		stmt, err := tx.Prepare(
			`INSERT INTO routine_snapshots (preset, date, exercise_name, weight, reps, sets, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
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
			if _, err := stmt.Exec(preset, date, e.Name, pfloat(e.Weight), e.Reps, sets, i, now); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdatePresetExercise patches weight/reps/sets of one exercise in the
// latest snapshot of preset (in place; no new history). A missing exercise
// is appended (action "added"). If the preset has no snapshot yet, one is
// created dated today. ErrNotFound if the preset does not exist.
func (s *Store) UpdatePresetExercise(preset, name string, weight *float64, reps, sets *int) (action, presetDate string, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		var n int
		if e := tx.QueryRow(`SELECT COUNT(*) FROM presets WHERE name = ?`, preset).Scan(&n); e != nil {
			return e
		}
		if n == 0 {
			return ErrNotFound
		}

		var d sql.NullString
		if e := tx.QueryRow(`SELECT MAX(date) FROM routine_snapshots WHERE preset = ?`, preset).Scan(&d); e != nil {
			return e
		}
		date := d.String
		if !d.Valid {
			date = domain.Today()
		}
		presetDate = date

		var cnt int
		if e := tx.QueryRow(
			`SELECT COUNT(*) FROM routine_snapshots WHERE preset = ? AND date = ? AND exercise_name = ?`,
			preset, date, name).Scan(&cnt); e != nil {
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
			action = "updated"
			if set == "" {
				return nil
			}
			args = append(args, preset, date, name)
			_, e := tx.Exec(`UPDATE routine_snapshots SET `+set+` WHERE preset = ? AND date = ? AND exercise_name = ?`, args...)
			return e
		}

		var nextOrder int
		if e := tx.QueryRow(
			`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM routine_snapshots WHERE preset = ? AND date = ?`,
			preset, date).Scan(&nextOrder); e != nil {
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
			`INSERT INTO routine_snapshots (preset, date, exercise_name, weight, reps, sets, sort_order, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			preset, date, name, pfloat(weight), r, st, nextOrder, domain.NowRFC3339()); e != nil {
			return e
		}
		action = "added"
		return nil
	})
	return action, presetDate, err
}

// ---------------------------------------------------------------------------
// combined views (legacy /api/routine compatibility)
// ---------------------------------------------------------------------------

// OrderedPresetNames returns preset names in sortOrder order.
func (s *Store) OrderedPresetNames() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM presets ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CombinedLatestRoutine merges every preset's latest snapshot in sortOrder
// order. presetCount is the number of preset rows (0 => caller returns 404).
func (s *Store) CombinedLatestRoutine() (date string, presets []string, exs []domain.RoutineExercise, presetCount int, err error) {
	names, err := s.OrderedPresetNames()
	if err != nil {
		return "", nil, nil, 0, err
	}
	presetCount = len(names)
	presets = []string{}
	exs = []domain.RoutineExercise{}
	for _, name := range names {
		d, e := s.LatestPresetDate(name)
		if e != nil {
			return "", nil, nil, presetCount, e
		}
		if d == "" {
			continue
		}
		rows, e := s.PresetSnapshot(name, d)
		if e != nil {
			return "", nil, nil, presetCount, e
		}
		if len(rows) == 0 {
			continue
		}
		if d > date {
			date = d
		}
		presets = append(presets, name)
		exs = append(exs, rows...)
	}
	return date, presets, exs, presetCount, nil
}

// CombinedRoutineHistory returns all-preset snapshots, counts summed per date.
func (s *Store) CombinedRoutineHistory() ([]RoutineHistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT date, COUNT(*) FROM routine_snapshots WHERE preset != '' GROUP BY date ORDER BY date DESC`)
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

// CombinedRoutineByDate merges every preset's snapshot for date. found is
// false when no preset has a snapshot on that date.
func (s *Store) CombinedRoutineByDate(date string) (presets []string, exs []domain.RoutineExercise, found bool, err error) {
	names, err := s.OrderedPresetNames()
	if err != nil {
		return nil, nil, false, err
	}
	presets = []string{}
	exs = []domain.RoutineExercise{}
	for _, name := range names {
		rows, e := s.PresetSnapshot(name, date)
		if e != nil {
			return nil, nil, false, e
		}
		if len(rows) == 0 {
			continue
		}
		found = true
		presets = append(presets, name)
		exs = append(exs, rows...)
	}
	return presets, exs, found, nil
}

// CrossPresetUpdateExercise finds the first preset (sortOrder order) whose
// latest snapshot contains name and updates it in place. ErrNotFound if no
// preset's latest snapshot has that exercise. This path never adds.
func (s *Store) CrossPresetUpdateExercise(name string, weight *float64, reps, sets *int) (preset, presetDate string, err error) {
	err = s.tx(func(tx *sql.Tx) error {
		rows, e := tx.Query(`SELECT name FROM presets ORDER BY sort_order, name`)
		if e != nil {
			return e
		}
		var names []string
		for rows.Next() {
			var n string
			if e := rows.Scan(&n); e != nil {
				rows.Close()
				return e
			}
			names = append(names, n)
		}
		if e := rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()

		for _, pn := range names {
			var d sql.NullString
			if e := tx.QueryRow(`SELECT MAX(date) FROM routine_snapshots WHERE preset = ?`, pn).Scan(&d); e != nil {
				return e
			}
			if !d.Valid {
				continue
			}
			var cnt int
			if e := tx.QueryRow(
				`SELECT COUNT(*) FROM routine_snapshots WHERE preset = ? AND date = ? AND exercise_name = ?`,
				pn, d.String, name).Scan(&cnt); e != nil {
				return e
			}
			if cnt == 0 {
				continue
			}
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
			preset = pn
			presetDate = d.String
			if set == "" {
				return nil
			}
			args = append(args, pn, d.String, name)
			_, e := tx.Exec(`UPDATE routine_snapshots SET `+set+` WHERE preset = ? AND date = ? AND exercise_name = ?`, args...)
			return e
		}
		return ErrNotFound
	})
	return preset, presetDate, err
}
