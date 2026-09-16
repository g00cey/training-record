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
