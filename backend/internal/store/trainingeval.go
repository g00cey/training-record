package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"training-record/internal/domain"
)

const trainingEvalCols = `id, period_type, evaluated_at, period_from, period_to, model, summary, details_json`

type trainingEvalDetails struct {
	Strengths   []string `json:"strengths"`
	Concerns    []string `json:"concerns"`
	Suggestions []string `json:"suggestions"`
}

func scanTrainingEvaluation(sc interface{ Scan(...any) error }) (*domain.TrainingEvaluation, error) {
	var rec domain.TrainingEvaluation
	var detailsRaw string
	if err := sc.Scan(&rec.ID, &rec.PeriodType, &rec.EvaluatedAt, &rec.PeriodFrom, &rec.PeriodTo,
		&rec.Model, &rec.Summary, &detailsRaw); err != nil {
		return nil, err
	}
	var d trainingEvalDetails
	if detailsRaw != "" {
		_ = json.Unmarshal([]byte(detailsRaw), &d)
	}
	rec.Strengths = d.Strengths
	rec.Concerns = d.Concerns
	rec.Suggestions = d.Suggestions
	return &rec, nil
}

// InsertTrainingEvaluation stores one LLM-generated evaluation (Phase 8) and
// returns its id.
func (s *Store) InsertTrainingEvaluation(rec domain.TrainingEvaluation, rawResponse string) (int64, error) {
	details, err := json.Marshal(trainingEvalDetails{
		Strengths:   rec.Strengths,
		Concerns:    rec.Concerns,
		Suggestions: rec.Suggestions,
	})
	if err != nil {
		return 0, err
	}
	res, err := s.db.Exec(
		`INSERT INTO training_evaluations
		 (period_type, evaluated_at, period_from, period_to, model, summary, details_json, raw_response)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.PeriodType, rec.EvaluatedAt, rec.PeriodFrom, rec.PeriodTo, rec.Model, rec.Summary, string(details), rawResponse)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LatestTrainingEvaluation returns the most recent evaluation of periodType,
// or (nil, nil) when none exists yet.
func (s *Store) LatestTrainingEvaluation(periodType string) (*domain.TrainingEvaluation, error) {
	rec, err := scanTrainingEvaluation(s.db.QueryRow(
		`SELECT `+trainingEvalCols+` FROM training_evaluations
		 WHERE period_type = ? ORDER BY evaluated_at DESC LIMIT 1`, periodType))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// RecentTrainingEvaluations returns up to limit evaluations of periodType,
// newest first (empty slice, never nil, when none exist).
func (s *Store) RecentTrainingEvaluations(periodType string, limit int) ([]domain.TrainingEvaluation, error) {
	rows, err := s.db.Query(
		`SELECT `+trainingEvalCols+` FROM training_evaluations
		 WHERE period_type = ? ORDER BY evaluated_at DESC LIMIT ?`, periodType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.TrainingEvaluation{}
	for rows.Next() {
		rec, err := scanTrainingEvaluation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}
