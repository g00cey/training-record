package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// EnsureRoutinePresets is an idempotent startup fixup run after migrations
// and bootstrap. It seeds the default presets and classifies any legacy
// routine_snapshots rows that still have preset=” into named presets.
//
//  1. seed 自重(0) / FW(1) only when the presets table is empty
//  2. classify routine_snapshots WHERE preset=”:
//     weight IS NULL -> 自重, otherwise -> FW
//  3. re-sequence sort_order to 0..n-1 within each (preset, date) group
//  4. defensively add any preset referenced by routine_snapshots but
//     missing from the presets table
//
// Once every row carries a preset this is a no-op.
func EnsureRoutinePresets(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)

	var presetCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM presets`).Scan(&presetCount); err != nil {
		return fmt.Errorf("count presets: %w", err)
	}
	if presetCount == 0 {
		if _, err := tx.Exec(
			`INSERT INTO presets (name, sort_order, created_at) VALUES ('自重', 0, ?), ('FW', 1, ?)`,
			now, now); err != nil {
			return fmt.Errorf("seed presets: %w", err)
		}
		log.Printf("presets: seeded defaults 自重(0) / FW(1)")
	}

	var unclassified int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM routine_snapshots WHERE preset = ''`).Scan(&unclassified); err != nil {
		return fmt.Errorf("count unclassified routine_snapshots: %w", err)
	}
	if unclassified > 0 {
		if _, err := tx.Exec(`UPDATE routine_snapshots SET preset = '自重' WHERE preset = '' AND weight IS NULL`); err != nil {
			return fmt.Errorf("classify bodyweight rows: %w", err)
		}
		if _, err := tx.Exec(`UPDATE routine_snapshots SET preset = 'FW' WHERE preset = '' AND weight IS NOT NULL`); err != nil {
			return fmt.Errorf("classify fw rows: %w", err)
		}
		if err := resequenceRoutineSnapshots(tx); err != nil {
			return fmt.Errorf("resequence sort_order: %w", err)
		}
		if err := addMissingPresets(tx, now); err != nil {
			return fmt.Errorf("add missing presets: %w", err)
		}
		log.Printf("presets: classified %d legacy routine_snapshots rows into presets", unclassified)
	}

	return tx.Commit()
}

// resequenceRoutineSnapshots renumbers sort_order to 0..n-1 within every
// (preset, date) group, ordered by the existing (sort_order, id).
func resequenceRoutineSnapshots(tx *sql.Tx) error {
	rows, err := tx.Query(
		`SELECT id, preset, date FROM routine_snapshots
		   WHERE preset != '' ORDER BY preset, date, sort_order, id`)
	if err != nil {
		return err
	}
	type row struct {
		id     int64
		preset string
		date   string
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.preset, &r.date); err != nil {
			rows.Close()
			return err
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	stmt, err := tx.Prepare(`UPDATE routine_snapshots SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	curKey := ""
	idx := 0
	for _, r := range all {
		key := r.preset + "\x00" + r.date
		if key != curKey {
			curKey = key
			idx = 0
		}
		if _, err := stmt.Exec(idx, r.id); err != nil {
			return err
		}
		idx++
	}
	return nil
}

// addMissingPresets inserts a presets row (appended sort_order) for every
// preset name present in routine_snapshots but absent from presets.
func addMissingPresets(tx *sql.Tx, now string) error {
	rows, err := tx.Query(
		`SELECT DISTINCT preset FROM routine_snapshots
		   WHERE preset != '' AND preset NOT IN (SELECT name FROM presets)`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, n := range names {
		if _, err := tx.Exec(
			`INSERT INTO presets (name, sort_order, created_at)
			 VALUES (?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM presets), ?)`,
			n, now); err != nil {
			return err
		}
		log.Printf("presets: added missing preset %q referenced by routine_snapshots", n)
	}
	return nil
}
