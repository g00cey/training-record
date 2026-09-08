// Package database opens the SQLite database, runs embedded migrations and
// performs the one-time bootstrap import from a legacy training.db.
package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open opens (creating the parent directory and file if needed) the SQLite
// database at path with WAL journaling and foreign keys enabled.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite tolerates a single writer; keep one connection to avoid
	// SQLITE_BUSY under the app's light single-user load.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// HasAppSchema reports whether the core application schema already exists
// (used to decide whether the DB is brand new and bootstrap-eligible).
func HasAppSchema(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='strength_sessions'`,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

var migrationName = regexp.MustCompile(`^(\d+)_.*\.sql$`)

// Migrate applies every not-yet-applied migration from migrations/*.sql in
// fsys, recording each in schema_migrations.
func Migrate(db *sql.DB, fsys fs.FS) error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
	); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied := map[int]bool{}
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && migrationName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	count := 0
	for _, name := range names {
		m := migrationName.FindStringSubmatch(name)
		version, _ := strconv.Atoi(m[1])
		if applied[version] {
			continue
		}
		body, err := fs.ReadFile(fsys, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(body)) {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %s: %w", name, err)
			}
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("migration: applied %s", name)
		count++
	}
	if count == 0 {
		log.Printf("migration: schema up to date (%d applied)", len(applied))
	}
	return nil
}

// splitSQL splits a migration file into individual statements. The
// migration files contain only simple DDL plus one INSERT with no
// embedded semicolons or string literals containing ';', so a naive split
// (ignoring '--' line comments) is sufficient.
func splitSQL(s string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
