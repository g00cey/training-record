// Package store is the SQL persistence layer. Methods return domain types
// and generic sentinel errors (ErrNotFound / ErrDateExists); mapping to
// HTTP status codes happens in the service / httpapi layers.
package store

import (
	"database/sql"
	"errors"
	"strings"
)

// Store wraps the application database.
type Store struct {
	db *sql.DB
}

// New builds a Store over db.
func New(db *sql.DB) *Store { return &Store{db: db} }

// DB exposes the underlying handle (used by tests).
func (s *Store) DB() *sql.DB { return s.db }

// Sentinel errors.
var (
	ErrNotFound         = errors.New("not found")
	ErrExerciseNotFound = errors.New("exercise not found")
	ErrDateExists       = errors.New("date already exists")
	ErrPresetExists     = errors.New("preset already exists")
)

// rowQuerier is satisfied by *sql.DB and *sql.Tx.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

func (s *Store) tx(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// --- scan helpers -----------------------------------------------------------

func nfloat(v sql.NullFloat64) *float64 {
	if v.Valid {
		x := v.Float64
		return &x
	}
	return nil
}

func nint(v sql.NullInt64) *int {
	if v.Valid {
		x := int(v.Int64)
		return &x
	}
	return nil
}

func pfloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func pint(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

// placeholders returns "?, ?, ?" for n and the args slice for ids.
func placeholders(ids []int64) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	marks := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args[i] = id
	}
	return strings.Join(marks, ", "), args
}

// rangeClause builds an optional "date BETWEEN" filter.
func rangeClause(col, from, to string) (string, []any) {
	var conds []string
	var args []any
	if from != "" {
		conds = append(conds, col+" >= ?")
		args = append(args, from)
	}
	if to != "" {
		conds = append(conds, col+" <= ?")
		args = append(args, to)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
