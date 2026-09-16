package service

import (
	"context"
	"fmt"
	"time"

	"training-record/internal/domain"
	"training-record/internal/llmeval"
)

// ---------------------------------------------------------------------------
// LLM トレーニング評価（Phase 8・日次バッチ）
//
// GET /api/advice / GET /api/load-report とは異なり、この評価は毎回再計算せず
// `server -evaluate-training` バッチが1日1回だけ生成し DB に保存する。
// ---------------------------------------------------------------------------

// biweeklyWindowDays is the fixed window for the "biweekly" evaluation
// (直近2週間), independent of historyWeeks (which only bounds the raw-data
// fetch and the "bimonthly" window).
const biweeklyWindowDays = 14

// RunTrainingEvaluations fetches historyWeeks of training history once, then
// asks the LLM evaluation server for a "biweekly" (直近2週間) and a
// "bimonthly" (直近 historyWeeks 週間) evaluation and persists both. Each
// period is attempted independently: a failure in one does not block the
// other. Callers (the -evaluate-training CLI subcommand) should treat any
// non-empty errs as a failure while still keeping whatever was saved.
func (s *Service) RunTrainingEvaluations(ctx context.Context, llm *llmeval.Client, now time.Time, historyWeeks int) (biweekly, bimonthly *domain.TrainingEvaluation, errs []error) {
	if historyWeeks <= 0 {
		historyWeeks = 8
	}
	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, nil, []error{fmt.Errorf("get profile: %w", err)}
	}
	from := now.AddDate(0, 0, -7*historyWeeks).Format(domain.DateLayout)
	strengths, err := s.st.StrengthSessionsInRange(from, "")
	if err != nil {
		return nil, nil, []error{fmt.Errorf("load strength sessions: %w", err)}
	}
	spins, err := s.st.SpinSessionsInRange(from, "")
	if err != nil {
		return nil, nil, []error{fmt.Errorf("load spin sessions: %w", err)}
	}
	m := computeLoadMetricsFrom(now, prof, strengths, spins)

	biweekly, err = s.runOneEvaluation(ctx, llm, "biweekly",
		now, now.AddDate(0, 0, -biweeklyWindowDays), prof, m, strengths, spins)
	if err != nil {
		errs = append(errs, fmt.Errorf("biweekly: %w", err))
	}
	bimonthly, err = s.runOneEvaluation(ctx, llm, "bimonthly",
		now, now.AddDate(0, 0, -7*historyWeeks), prof, m, strengths, spins)
	if err != nil {
		errs = append(errs, fmt.Errorf("bimonthly: %w", err))
	}
	return biweekly, bimonthly, errs
}

func (s *Service) runOneEvaluation(
	ctx context.Context, llm *llmeval.Client, periodType string,
	now, periodFrom time.Time, prof domain.Profile, m *loadMetrics,
	strengths []domain.StrengthSession, spins []domain.SpinSession,
) (*domain.TrainingEvaluation, error) {
	from := periodFrom.Format(domain.DateLayout)
	to := now.Format(domain.DateLayout)

	payload := llmeval.HistoryPayload{
		PeriodType: periodType,
		AsOf:       to,
		PeriodFrom: from,
		PeriodTo:   to,
		Profile: llmeval.ProfilePayload{
			BodyweightKg: prof.BodyweightKg,
			HeightCm:     prof.HeightCm,
			MaxHrEst:     prof.MaxHrEst,
		},
		LoadMetrics: llmeval.LoadMetricsPayload{
			ACWR:             m.acwr,
			Zone:             m.zone,
			Acute7dTotal:     domain.RoundInt(m.acuteTotal),
			ChronicWeeklyAvg: domain.RoundInt(m.chronicWeekly),
		},
		Strength: toStrengthEvalPayload(strengthsSince(strengths, from)),
		Spin:     toSpinEvalPayload(spinsSince(spins, from)),
	}

	res, err := llm.Evaluate(ctx, payload)
	if err != nil {
		return nil, err
	}
	n := llmeval.Normalize(res)

	rec := domain.TrainingEvaluation{
		PeriodType:  periodType,
		EvaluatedAt: domain.NowRFC3339(),
		PeriodFrom:  from,
		PeriodTo:    to,
		Summary:     n.Summary,
		Strengths:   n.Strengths,
		Concerns:    n.Concerns,
		Suggestions: n.Suggestions,
	}
	id, err := s.st.InsertTrainingEvaluation(rec, n.Raw)
	if err != nil {
		return nil, fmt.Errorf("persist evaluation: %w", err)
	}
	rec.ID = id
	return &rec, nil
}

func strengthsSince(strengths []domain.StrengthSession, from string) []domain.StrengthSession {
	out := []domain.StrengthSession{}
	for _, ss := range strengths {
		if ss.Date >= from {
			out = append(out, ss)
		}
	}
	return out
}

func spinsSince(spins []domain.SpinSession, from string) []domain.SpinSession {
	out := []domain.SpinSession{}
	for _, sp := range spins {
		if sp.Date >= from {
			out = append(out, sp)
		}
	}
	return out
}

func toStrengthEvalPayload(strengths []domain.StrengthSession) []llmeval.StrengthSessionPayload {
	out := make([]llmeval.StrengthSessionPayload, 0, len(strengths))
	for _, ss := range strengths {
		exs := make([]llmeval.StrengthExercisePayload, 0, len(ss.Exercises))
		for _, e := range ss.Exercises {
			exs = append(exs, llmeval.StrengthExercisePayload{
				Name: e.Name, Weight: e.Weight, Reps: e.Reps, Sets: e.Sets,
			})
		}
		out = append(out, llmeval.StrengthSessionPayload{Date: ss.Date, Notes: ss.Notes, Exercises: exs})
	}
	return out
}

func toSpinEvalPayload(spins []domain.SpinSession) []llmeval.SpinSessionPayload {
	out := make([]llmeval.SpinSessionPayload, 0, len(spins))
	for _, sp := range spins {
		out = append(out, llmeval.SpinSessionPayload{
			Date:            sp.Date,
			DurationMinutes: sp.DurationMinutes,
			AvgHeartRate:    sp.AvgHeartRate,
			MaxHeartRate:    sp.MaxHeartRate,
			RPE:             sp.RPE,
			DistanceKm:      sp.DistanceKm,
		})
	}
	return out
}

// LatestTrainingEvaluation serves GET /api/training-evaluations/bimonthly
// (nil, nil when the batch has never run for periodType).
func (s *Service) LatestTrainingEvaluation(periodType string) (*domain.TrainingEvaluation, error) {
	return s.st.LatestTrainingEvaluation(periodType)
}

// RecentTrainingEvaluations serves GET /api/training-evaluations/biweekly.
func (s *Service) RecentTrainingEvaluations(periodType string, limit int) ([]domain.TrainingEvaluation, error) {
	return s.st.RecentTrainingEvaluations(periodType, limit)
}
