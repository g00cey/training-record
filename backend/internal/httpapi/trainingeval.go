package httpapi

import (
	"net/http"

	"training-record/internal/apperr"
	"training-record/internal/domain"
)

// maxRecentTrainingEvaluations caps GET /api/training-evaluations/biweekly
// regardless of the requested limit (past 10 件まで確認可能).
const maxRecentTrainingEvaluations = 10

// trainingEvaluationBimonthly handles GET /api/training-evaluations/bimonthly
// — the latest 2ヶ月（直近8週間）評価。書き込みは -evaluate-training バッチのみ。
func (h *Handlers) trainingEvaluationBimonthly(w http.ResponseWriter, r *http.Request) error {
	rec, err := h.svc.LatestTrainingEvaluation("bimonthly")
	if err != nil {
		return err
	}
	if rec == nil {
		return apperr.NotFoundf("training evaluation not yet available")
	}
	writeJSON(w, http.StatusOK, toTrainingEvaluationDTO(rec))
	return nil
}

// trainingEvaluationBiweekly handles GET /api/training-evaluations/biweekly
// — 直近2週間評価の履歴（最新10件まで）。
func (h *Handlers) trainingEvaluationBiweekly(w http.ResponseWriter, r *http.Request) error {
	limit, err := qInt(r, "limit", maxRecentTrainingEvaluations)
	if err != nil {
		return err
	}
	if limit <= 0 || limit > maxRecentTrainingEvaluations {
		limit = maxRecentTrainingEvaluations
	}
	recs, err := h.svc.RecentTrainingEvaluations("biweekly", limit)
	if err != nil {
		return err
	}
	out := make([]trainingEvaluationDTO, 0, len(recs))
	for i := range recs {
		out = append(out, toTrainingEvaluationDTO(&recs[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"evaluations": out})
	return nil
}

// trainingEvaluationDTO is the public shape of a training_evaluations row.
// raw_response is intentionally excluded (audit-only).
type trainingEvaluationDTO struct {
	EvaluatedAt string   `json:"evaluatedAt"`
	PeriodFrom  string   `json:"periodFrom"`
	PeriodTo    string   `json:"periodTo"`
	Model       string   `json:"model"`
	Summary     string   `json:"summary"`
	Strengths   []string `json:"strengths"`
	Concerns    []string `json:"concerns"`
	Suggestions []string `json:"suggestions"`
}

func toTrainingEvaluationDTO(rec *domain.TrainingEvaluation) trainingEvaluationDTO {
	strengths, concerns, suggestions := rec.Strengths, rec.Concerns, rec.Suggestions
	if strengths == nil {
		strengths = []string{}
	}
	if concerns == nil {
		concerns = []string{}
	}
	if suggestions == nil {
		suggestions = []string{}
	}
	return trainingEvaluationDTO{
		EvaluatedAt: rec.EvaluatedAt,
		PeriodFrom:  rec.PeriodFrom,
		PeriodTo:    rec.PeriodTo,
		Model:       rec.Model,
		Summary:     rec.Summary,
		Strengths:   strengths,
		Concerns:    concerns,
		Suggestions: suggestions,
	}
}
