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
//
// biweekly は固定の2週間（日曜〜土曜・domain.BiweeklyPeriodOf のグリッド:
// 10/4〜10/17, 10/18〜10/31, 11/1〜11/14, ...）を1期間1行で保持する:
//   - 進行中の期間 → 毎日の実行で評価を上書き更新（履歴の先頭に常に
//     「今の2週間」の評価が並ぶ。記録がまだ無い期間は評価を作らない）
//   - 期間の終了後の最初の実行 → その期間の確定評価（全14日分のデータ対象）
//     へ置き換える（記録が無かった期間も1件だけ保存される）
// bimonthly は従来どおり直近 historyWeeks 週間の評価を毎日1行追加する。
// ---------------------------------------------------------------------------

// RunTrainingEvaluations fetches training history once, then asks the LLM
// evaluation server for a "bimonthly" (直近 historyWeeks 週間) evaluation and
// one or two "biweekly" evaluations on the fixed two-week grid (see the
// package comment) and persists them. Each period is attempted independently:
// a failure in one does not block the others. Callers (the -evaluate-training
// CLI subcommand) should treat any non-empty errs as a failure while still
// keeping whatever was saved.
func (s *Service) RunTrainingEvaluations(ctx context.Context, llm *llmeval.Client, now time.Time, historyWeeks int) (biweekly, bimonthly *domain.TrainingEvaluation, errs []error) {
	if historyWeeks <= 0 {
		historyWeeks = 8
	}
	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, nil, []error{fmt.Errorf("get profile: %w", err)}
	}

	// 進行中の2週間期間と、直近の完了済み期間（終了日 < 今日）を求める。
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, domain.JST)
	curFrom, curTo := domain.BiweeklyPeriodOf(now)
	prevFrom, prevTo := curFrom, curTo
	if !curTo.Before(today) { // 期間終了日 >= 今日 = まだ完了していない
		prevFrom = curFrom.AddDate(0, 0, -domain.BiweeklyPeriodDays)
		prevTo = curTo.AddDate(0, 0, -domain.BiweeklyPeriodDays)
	}

	// 生データの取得範囲は bimonthly の窓と biweekly の対象期間の両方をカバーする。
	fetchFrom := now.AddDate(0, 0, -7*historyWeeks).Format(domain.DateLayout)
	if pf := prevFrom.Format(domain.DateLayout); pf < fetchFrom {
		fetchFrom = pf
	}
	strengths, err := s.st.StrengthSessionsInRange(fetchFrom, "")
	if err != nil {
		return nil, nil, []error{fmt.Errorf("load strength sessions: %w", err)}
	}
	spins, err := s.st.SpinSessionsInRange(fetchFrom, "")
	if err != nil {
		return nil, nil, []error{fmt.Errorf("load spin sessions: %w", err)}
	}
	m := computeLoadMetricsFrom(now, prof, strengths, spins)

	// biweekly ①: 直近の完了済み2週間を確定させる。まだ評価が無い期間、または
	// 期間中の暫定評価（評価日 <= 期間終了日）しか無い場合は、全14日分の
	// データで評価し直して置き換える。
	pf, pt := prevFrom.Format(domain.DateLayout), prevTo.Format(domain.DateLayout)
	existing, err := s.st.TrainingEvaluationForPeriod("biweekly", pf, pt)
	if err != nil {
		errs = append(errs, fmt.Errorf("biweekly: %w", err))
	} else if existing == nil || !biweeklyFinal(existing, pt) {
		mPrev := computeLoadMetricsFrom(prevTo, prof, strengths, spins)
		rec, err := s.runOneEvaluation(ctx, llm, "biweekly", now,
			prevFrom, prevTo, prof, mPrev, strengths, spins, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("biweekly: %w", err))
		} else {
			biweekly = rec
		}
	}

	// biweekly ②: 進行中の2週間を毎日上書き更新する（まだ記録が無い期間は
	// 評価を作らない）。期間の最終日はデータが前日までの暫定評価になるが、
	// 期間終了後の最初の実行で ① が全14日分の確定評価へ置き換える。
	cf := curFrom.Format(domain.DateLayout)
	if hasSessionsSince(strengths, spins, cf) {
		rec, err := s.runOneEvaluation(ctx, llm, "biweekly", now,
			curFrom, curTo, prof, m, strengths, spins, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("biweekly: %w", err))
		} else {
			biweekly = rec
		}
	}

	bimonthly, err = s.runOneEvaluation(ctx, llm, "bimonthly", now,
		now.AddDate(0, 0, -7*historyWeeks), now, prof, m, strengths, spins, false)
	if err != nil {
		errs = append(errs, fmt.Errorf("bimonthly: %w", err))
	}
	return biweekly, bimonthly, errs
}

// biweeklyFinal reports whether rec is the finalized evaluation of the fixed
// period ending on periodTo: 期間終了日より後に評価された行は確定済みとみなす
// （期間中の毎日更新は評価日 <= 期間終了日になる）。
func biweeklyFinal(rec *domain.TrainingEvaluation, periodTo string) bool {
	if len(rec.EvaluatedAt) < len(domain.DateLayout) {
		return false
	}
	return rec.EvaluatedAt[:len(domain.DateLayout)] > periodTo
}

// hasSessionsSince reports whether any strength/spin session is dated on or
// after from (ascending slices from the store are fine).
func hasSessionsSince(strengths []domain.StrengthSession, spins []domain.SpinSession, from string) bool {
	for _, ss := range strengths {
		if ss.Date >= from {
			return true
		}
	}
	for _, sp := range spins {
		if sp.Date >= from {
			return true
		}
	}
	return false
}

func (s *Service) runOneEvaluation(
	ctx context.Context, llm *llmeval.Client, periodType string,
	asOf, periodFrom, periodTo time.Time, prof domain.Profile, m *loadMetrics,
	strengths []domain.StrengthSession, spins []domain.SpinSession, upsert bool,
) (*domain.TrainingEvaluation, error) {
	from := periodFrom.Format(domain.DateLayout)
	to := periodTo.Format(domain.DateLayout)
	asOfDay := asOf.Format(domain.DateLayout)
	// セッションの上限は期間終了日と評価日の早い方（進行中の期間は当日まで）。
	upper := to
	if upper > asOfDay {
		upper = asOfDay
	}

	payload := llmeval.HistoryPayload{
		PeriodType: periodType,
		AsOf:       asOfDay,
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
		Strength: toStrengthEvalPayload(strengthsBetween(strengths, from, upper)),
		Spin:     toSpinEvalPayload(spinsBetween(spins, from, upper)),
	}

	res, err := llm.Evaluate(ctx, payload)
	if err != nil {
		return nil, err
	}
	n := llmeval.Normalize(res)

	rec := domain.TrainingEvaluation{
		PeriodType:  periodType,
		EvaluatedAt: asOf.Format(time.RFC3339),
		PeriodFrom:  from,
		PeriodTo:    to,
		Summary:     n.Summary,
		Strengths:   n.Strengths,
		Concerns:    n.Concerns,
		Suggestions: n.Suggestions,
	}
	var id int64
	if upsert {
		id, err = s.st.UpsertTrainingEvaluation(rec, n.Raw)
	} else {
		id, err = s.st.InsertTrainingEvaluation(rec, n.Raw)
	}
	if err != nil {
		return nil, fmt.Errorf("persist evaluation: %w", err)
	}
	rec.ID = id
	return &rec, nil
}

func strengthsBetween(strengths []domain.StrengthSession, from, to string) []domain.StrengthSession {
	out := []domain.StrengthSession{}
	for _, ss := range strengths {
		if ss.Date >= from && ss.Date <= to {
			out = append(out, ss)
		}
	}
	return out
}

func spinsBetween(spins []domain.SpinSession, from, to string) []domain.SpinSession {
	out := []domain.SpinSession{}
	for _, sp := range spins {
		if sp.Date >= from && sp.Date <= to {
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
