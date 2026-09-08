package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

// BootstrapResult reports how many rows were imported per table.
type BootstrapResult struct {
	StrengthSessions int
	Exercises        int
	SpinSessions     int
	RoutineSnapshots int
}

// MaybeBootstrap imports the four core tables (ids preserved) from the
// legacy SQLite file at srcPath into db, but ONLY when fresh is true (the
// target DB had no schema before migrations ran) and srcPath is set and
// exists. Any other case is a no-op. All outcomes are logged.
func MaybeBootstrap(db *sql.DB, srcPath string, fresh bool) (*BootstrapResult, error) {
	if !fresh {
		log.Printf("bootstrap: skipped (existing database, migrations only)")
		return nil, nil
	}
	if srcPath == "" {
		log.Printf("bootstrap: no BOOTSTRAP_DB_PATH set, starting from an empty database")
		return nil, nil
	}
	if _, err := os.Stat(srcPath); err != nil {
		log.Printf("bootstrap: BOOTSTRAP_DB_PATH %q not found (%v), starting from an empty database", srcPath, err)
		return nil, nil
	}

	src, err := sql.Open("sqlite", "file:"+srcPath+"?mode=ro&_pragma=foreign_keys(0)")
	if err != nil {
		return nil, fmt.Errorf("open bootstrap source: %w", err)
	}
	defer src.Close()
	if err := src.Ping(); err != nil {
		return nil, fmt.Errorf("ping bootstrap source: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := &BootstrapResult{}

	// strength_sessions (legacy has no updated_at -> mirror created_at)
	if res.StrengthSessions, err = copyRows(src, tx,
		`SELECT id, date, COALESCE(notes,''), COALESCE(created_at, datetime('now')) FROM strength_sessions`,
		`INSERT INTO strength_sessions (id, date, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		func(scan func(...any) error, exec func(...any) error) error {
			var id int64
			var date, notes, createdAt string
			if err := scan(&id, &date, &notes, &createdAt); err != nil {
				return err
			}
			return exec(id, date, notes, createdAt, createdAt)
		},
	); err != nil {
		return nil, fmt.Errorf("copy strength_sessions: %w", err)
	}

	// exercises
	if res.Exercises, err = copyRows(src, tx,
		`SELECT id, session_id, name, weight, reps, COALESCE(sets,1), COALESCE(notes,''), COALESCE(sort_order,0) FROM exercises`,
		`INSERT INTO exercises (id, session_id, name, weight, reps, sets, notes, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(scan func(...any) error, exec func(...any) error) error {
			var id, sessionID int64
			var name, notes string
			var weight sql.NullFloat64
			var reps, sets, sortOrder int64
			if err := scan(&id, &sessionID, &name, &weight, &reps, &sets, &notes, &sortOrder); err != nil {
				return err
			}
			return exec(id, sessionID, name, nullFloat(weight), reps, sets, notes, sortOrder)
		},
	); err != nil {
		return nil, fmt.Errorf("copy exercises: %w", err)
	}

	// spin_sessions (legacy has no updated_at -> mirror created_at)
	if res.SpinSessions, err = copyRows(src, tx,
		`SELECT id, date, duration_minutes, avg_heart_rate, max_heart_rate, rpe, distance_km, COALESCE(notes,''), COALESCE(created_at, datetime('now')) FROM spin_sessions`,
		`INSERT INTO spin_sessions (id, date, duration_minutes, avg_heart_rate, max_heart_rate, rpe, distance_km, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		func(scan func(...any) error, exec func(...any) error) error {
			var id int64
			var date, notes, createdAt string
			var duration int64
			var avgHR, maxHR, rpe sql.NullInt64
			var distance sql.NullFloat64
			if err := scan(&id, &date, &duration, &avgHR, &maxHR, &rpe, &distance, &notes, &createdAt); err != nil {
				return err
			}
			return exec(id, date, duration, nullInt(avgHR), nullInt(maxHR), nullInt(rpe), nullFloat(distance), notes, createdAt, createdAt)
		},
	); err != nil {
		return nil, fmt.Errorf("copy spin_sessions: %w", err)
	}

	// routine_snapshots
	if res.RoutineSnapshots, err = copyRows(src, tx,
		`SELECT id, date, exercise_name, weight, reps, COALESCE(sets,1), COALESCE(sort_order,0), COALESCE(created_at, datetime('now')) FROM routine_snapshots`,
		`INSERT INTO routine_snapshots (id, date, exercise_name, weight, reps, sets, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		func(scan func(...any) error, exec func(...any) error) error {
			var id int64
			var date, name, createdAt string
			var weight sql.NullFloat64
			var reps, sets, sortOrder int64
			if err := scan(&id, &date, &name, &weight, &reps, &sets, &sortOrder, &createdAt); err != nil {
				return err
			}
			return exec(id, date, name, nullFloat(weight), reps, sets, sortOrder, createdAt)
		},
	); err != nil {
		return nil, fmt.Errorf("copy routine_snapshots: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	log.Printf("bootstrap: imported from %q -> strength_sessions=%d exercises=%d spin_sessions=%d routine_snapshots=%d",
		srcPath, res.StrengthSessions, res.Exercises, res.SpinSessions, res.RoutineSnapshots)
	return res, nil
}

func copyRows(src *sql.DB, tx *sql.Tx, selectSQL, insertSQL string,
	handle func(scan func(...any) error, exec func(...any) error) error) (int, error) {

	rows, err := src.Query(selectSQL)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	n := 0
	for rows.Next() {
		err := handle(
			func(dst ...any) error { return rows.Scan(dst...) },
			func(args ...any) error { _, e := stmt.Exec(args...); return e },
		)
		if err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func nullFloat(v sql.NullFloat64) any {
	if v.Valid {
		return v.Float64
	}
	return nil
}

func nullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
