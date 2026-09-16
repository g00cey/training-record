package httpapi_test

import (
	"net/http/httptest"
	"testing"

	"training-record/internal/domain"
	"training-record/internal/testsupport"
)

func TestTrainingEvaluationBimonthlyNotFound(t *testing.T) {
	h, _ := testsupport.NewRouter(t, key)
	req := httptest.NewRequest("GET", "/api/training-evaluations/bimonthly", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
}

func TestTrainingEvaluationBimonthlyFound(t *testing.T) {
	h, st := testsupport.NewRouter(t, key)
	if _, err := st.InsertTrainingEvaluation(domain.TrainingEvaluation{
		PeriodType:  "bimonthly",
		EvaluatedAt: "2026-09-16T03:00:00+09:00",
		PeriodFrom:  "2026-07-22",
		PeriodTo:    "2026-09-16",
		Summary:     "順調です",
		Strengths:   []string{"頻度が良い"},
	}, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/training-evaluations/bimonthly", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["summary"] != "順調です" {
		t.Errorf("summary: %v", m["summary"])
	}
	if _, ok := m["rawResponse"]; ok {
		t.Errorf("raw_response must not be exposed: %v", m)
	}
}

func TestTrainingEvaluationBiweeklyEmpty(t *testing.T) {
	h, _ := testsupport.NewRouter(t, key)
	req := httptest.NewRequest("GET", "/api/training-evaluations/biweekly", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	evals, ok := m["evaluations"].([]any)
	if !ok || len(evals) != 0 {
		t.Fatalf("evaluations = %v, want empty array", m["evaluations"])
	}
}

func TestTrainingEvaluationBiweeklyCapsAtTen(t *testing.T) {
	h, st := testsupport.NewRouter(t, key)
	for i := 0; i < 15; i++ {
		if _, err := st.InsertTrainingEvaluation(domain.TrainingEvaluation{
			PeriodType:  "biweekly",
			EvaluatedAt: domain.NowRFC3339(),
			PeriodFrom:  "2026-09-02",
			PeriodTo:    "2026-09-16",
			Summary:     "評価",
		}, ""); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	req := httptest.NewRequest("GET", "/api/training-evaluations/biweekly?limit=100", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	evals, _ := m["evaluations"].([]any)
	if len(evals) != 10 {
		t.Fatalf("evaluations len = %d, want 10 (capped)", len(evals))
	}
}

func TestTrainingEvaluationRequiresAuth(t *testing.T) {
	h, _ := testsupport.NewRouter(t, key)
	req := httptest.NewRequest("GET", "/api/training-evaluations/bimonthly", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}
