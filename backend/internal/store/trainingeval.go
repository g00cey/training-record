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

// TrainingEvaluationForPeriod returns the evaluation stored for the exact
// (periodType, periodFrom, periodTo) period, or (nil, nil) when none exists.
// The biweekly batch uses it to decide whether a fixed two-week period still
// needs its (final) evaluation.
func (s *Store) TrainingEvaluationForPeriod(periodType, from, to string) (*domain.TrainingEvaluation, error) {
	rec, err := scanTrainingEvaluation(s.db.QueryRow(
		`SELECT `+trainingEvalCols+` FROM training_evaluations
		 WHERE period_type = ? AND period_from = ? AND period_to = ?
		 ORDER BY evaluated_at DESC LIMIT 1`, periodType, from, to))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// UpsertTrainingEvaluation stores one evaluation, replacing any existing row
// of the same (period_type, period_from, period_to). The biweekly batch keeps
// exactly one row per fixed two-week period this way: 進行中の期間は毎日
// 上書き更新され、期間の終了後の最初の実行で確定評価へ置き換わる。
// bimonthly（日次のローリング評価）は引き続き InsertTrainingEvaluation を使う。
func (s *Store) UpsertTrainingEvaluation(rec domain.TrainingEvaluation, rawResponse string) (int64, error) {
	details, err := json.Marshal(trainingEvalDetails{
		Strengths:   rec.Strengths,
		Concerns:    rec.Concerns,
		Suggestions: rec.Suggestions,
	})
	if err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRow(
		`SELECT id FROM training_evaluations
		 WHERE period_type = ? AND period_from = ? AND period_to = ?
		 ORDER BY evaluated_at DESC LIMIT 1`,
		rec.PeriodType, rec.PeriodFrom, rec.PeriodTo).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		res, err := tx.Exec(
			`INSERT INTO training_evaluations
			 (period_type, evaluated_at, period_from, period_to, model, summary, details_json, raw_response)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			rec.PeriodType, rec.EvaluatedAt, rec.PeriodFrom, rec.PeriodTo, rec.Model, rec.Summary, string(details), rawResponse)
		if err != nil {
			return 0, err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
	case err != nil:
		return 0, err
	default:
		if _, err := tx.Exec(
			`UPDATE training_evaluations
			 SET evaluated_at = ?, model = ?, summary = ?, details_json = ?, raw_response = ?
			 WHERE id = ?`,
			rec.EvaluatedAt, rec.Model, rec.Summary, string(details), rawResponse, id); err != nil {
			return 0, err
		}
		// 1期間1行の契約を回復させる: 同じ期間を指す残りの行があれば掃除する。
		if _, err := tx.Exec(
			`DELETE FROM training_evaluations
			 WHERE period_type = ? AND period_from = ? AND period_to = ? AND id <> ?`,
			rec.PeriodType, rec.PeriodFrom, rec.PeriodTo, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// RecentTrainingEvaluations returns up to limit evaluations of periodType,
// newest first (empty slice, never nil, when none exist). period_from DESC
// breaks evaluated_at ties deterministically: 同一実行で進行中の期間と
// 直前期間の確定を続けて保存した場合に、進行中の期間が先頭に来る。
func (s *Store) RecentTrainingEvaluations(periodType string, limit int) ([]domain.TrainingEvaluation, error) {
	rows, err := s.db.Query(
		`SELECT `+trainingEvalCols+` FROM training_evaluations
		 WHERE period_type = ? ORDER BY evaluated_at DESC, period_from DESC LIMIT ?`, periodType, limit)
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
