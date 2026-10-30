package store_test

import (
	"testing"

	"training-record/internal/domain"
	"training-record/internal/testsupport"
)

func TestTrainingEvaluationRoundTrip(t *testing.T) {
	st := testsupport.NewStore(t)

	if rec, err := st.LatestTrainingEvaluation("biweekly"); err != nil || rec != nil {
		t.Fatalf("latest on empty table = (%+v, %v), want (nil, nil)", rec, err)
	}
	if list, err := st.RecentTrainingEvaluations("biweekly", 10); err != nil || len(list) != 0 {
		t.Fatalf("recent on empty table = (%v, %v), want (empty, nil)", list, err)
	}

	rec := domain.TrainingEvaluation{
		PeriodType:  "biweekly",
		EvaluatedAt: "2026-09-16T03:00:00+09:00",
		PeriodFrom:  "2026-09-02",
		PeriodTo:    "2026-09-16",
		Model:       "mimo-v2.5",
		Summary:     "順調です",
		Strengths:   []string{"頻度が良い"},
		Concerns:    []string{"ACWRがやや高め"},
		Suggestions: []string{"デロードを検討"},
	}
	id, err := st.InsertTrainingEvaluation(rec, `{"raw":"response"}`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == 0 {
		t.Fatalf("insert returned id 0")
	}

	got, err := st.LatestTrainingEvaluation("biweekly")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if got == nil {
		t.Fatalf("latest returned nil")
	}
	if got.Summary != rec.Summary || got.PeriodFrom != rec.PeriodFrom || got.PeriodTo != rec.PeriodTo {
		t.Errorf("latest mismatch: %+v", got)
	}
	if len(got.Strengths) != 1 || got.Strengths[0] != "頻度が良い" {
		t.Errorf("strengths mismatch: %v", got.Strengths)
	}

	// A different period type must not see this row.
	if rec, err := st.LatestTrainingEvaluation("bimonthly"); err != nil || rec != nil {
		t.Fatalf("bimonthly latest = (%+v, %v), want (nil, nil)", rec, err)
	}

	// Insert a second biweekly row (later evaluated_at) and check ordering.
	rec2 := rec
	rec2.EvaluatedAt = "2026-09-17T03:00:00+09:00"
	rec2.Summary = "2件目"
	if _, err := st.InsertTrainingEvaluation(rec2, ""); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	list, err := st.RecentTrainingEvaluations("biweekly", 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("recent len = %d, want 2", len(list))
	}
	if list[0].Summary != "2件目" {
		t.Errorf("recent[0] should be newest first: %+v", list[0])
	}

	limited, err := st.RecentTrainingEvaluations("biweekly", 1)
	if err != nil {
		t.Fatalf("recent limited: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("recent limited len = %d, want 1", len(limited))
	}
}

func TestTrainingEvaluationForPeriodAndUpsert(t *testing.T) {
	st := testsupport.NewStore(t)

	// まだ何も無い期間は nil。
	if rec, err := st.TrainingEvaluationForPeriod("biweekly", "2026-10-04", "2026-10-17"); err != nil || rec != nil {
		t.Fatalf("ForPeriod on empty table = (%+v, %v), want (nil, nil)", rec, err)
	}

	// upsert の初回は insert になる。
	rec := domain.TrainingEvaluation{
		PeriodType:  "biweekly",
		EvaluatedAt: "2026-10-30T03:00:00+09:00",
		PeriodFrom:  "2026-10-18",
		PeriodTo:    "2026-10-31",
		Summary:     "暫定です",
		Strengths:   []string{"頻度が良い"},
	}
	id1, err := st.UpsertTrainingEvaluation(rec, `{"raw":"1"}`)
	if err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if id1 == 0 {
		t.Fatalf("upsert insert returned id 0")
	}

	got, err := st.TrainingEvaluationForPeriod("biweekly", "2026-10-18", "2026-10-31")
	if err != nil || got == nil {
		t.Fatalf("ForPeriod after upsert: %v %v", got, err)
	}
	if got.Summary != "暫定です" || len(got.Strengths) != 1 {
		t.Errorf("upserted row mismatch: %+v", got)
	}

	// 同じ期間の2回目は上書き（1期間1行・id 不変）。
	rec.EvaluatedAt = "2026-10-31T03:00:00+09:00"
	rec.Summary = "確定です"
	id2, err := st.UpsertTrainingEvaluation(rec, `{"raw":"2"}`)
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if id2 != id1 {
		t.Errorf("upsert update id = %d, want %d (stable)", id2, id1)
	}
	list, err := st.RecentTrainingEvaluations("biweekly", 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("recent after upsert update: %v %v", list, err)
	}
	if list[0].Summary != "確定です" || list[0].EvaluatedAt != "2026-10-31T03:00:00+09:00" {
		t.Errorf("updated row mismatch: %+v", list[0])
	}

	// 違う期間は別行として insert される。
	rec2 := rec
	rec2.PeriodFrom, rec2.PeriodTo = "2026-11-01", "2026-11-14"
	rec2.Summary = "次の期間"
	if _, err := st.UpsertTrainingEvaluation(rec2, ""); err != nil {
		t.Fatalf("upsert other period: %v", err)
	}
	if list, err := st.RecentTrainingEvaluations("biweekly", 10); err != nil || len(list) != 2 {
		t.Fatalf("recent after other period: %v %v", list, err)
	}
	if rec, err := st.TrainingEvaluationForPeriod("biweekly", "2026-11-01", "2026-11-14"); err != nil || rec == nil || rec.Summary != "次の期間" {
		t.Fatalf("ForPeriod other period: %v %v", rec, err)
	}

	// 同じ期間を指す重複行があった場合は upsert が1行に正規化する。
	for i := 0; i < 2; i++ {
		if _, err := st.InsertTrainingEvaluation(domain.TrainingEvaluation{
			PeriodType: "biweekly", EvaluatedAt: "2026-10-31T23:00:00+09:00",
			PeriodFrom: "2026-11-01", PeriodTo: "2026-11-14", Summary: "重複",
		}, ""); err != nil {
			t.Fatalf("seed duplicate %d: %v", i, err)
		}
	}
	if _, err := st.UpsertTrainingEvaluation(rec2, ""); err != nil {
		t.Fatalf("upsert over duplicates: %v", err)
	}
	if list, err := st.RecentTrainingEvaluations("biweekly", 10); err != nil || len(list) != 2 {
		t.Fatalf("recent after dedupe: %v %v", list, err)
	}
	if rec, err := st.TrainingEvaluationForPeriod("biweekly", "2026-11-01", "2026-11-14"); err != nil || rec == nil || rec.Summary != "次の期間" {
		t.Fatalf("ForPeriod after dedupe: %v %v", rec, err)
	}
}
