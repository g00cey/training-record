package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"training-record/internal/domain"
	"training-record/internal/llmeval"
	"training-record/internal/testsupport"
)

func TestRunTrainingEvaluationsSavesBothPeriods(t *testing.T) {
	svc, st := testsupport.NewService(t)

	if _, err := st.CreateStrength("2026-09-10", "フリーウェイト", []domain.Exercise{
		{Name: "ベンチプレス", Weight: fptr(60), Reps: 8, Sets: 3},
	}); err != nil {
		t.Fatalf("seed strength: %v", err)
	}

	var gotPeriodTypes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PeriodType string `json:"periodType"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotPeriodTypes = append(gotPeriodTypes, body.PeriodType)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"順調です","strengths":["頻度良好"],"concerns":[],"suggestions":[]}`))
	}))
	defer srv.Close()

	client := llmeval.New(llmeval.Config{URL: srv.URL, Timeout: 5 * time.Second})
	now := time.Date(2026, 9, 16, 3, 0, 0, 0, domain.JST)

	biweekly, bimonthly, errs := svc.RunTrainingEvaluations(context.Background(), client, now, 8)
	if len(errs) != 0 {
		t.Fatalf("unexpected errs: %v", errs)
	}
	if biweekly == nil || bimonthly == nil {
		t.Fatalf("expected both results, got biweekly=%v bimonthly=%v", biweekly, bimonthly)
	}
	if biweekly.PeriodType != "biweekly" || bimonthly.PeriodType != "bimonthly" {
		t.Errorf("period types: %q %q", biweekly.PeriodType, bimonthly.PeriodType)
	}
	if len(gotPeriodTypes) != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", len(gotPeriodTypes))
	}

	latest, err := svc.LatestTrainingEvaluation("bimonthly")
	if err != nil || latest == nil {
		t.Fatalf("latest bimonthly: %v %v", latest, err)
	}
	if latest.Summary != "順調です" {
		t.Errorf("summary = %q", latest.Summary)
	}

	recent, err := svc.RecentTrainingEvaluations("biweekly", 10)
	if err != nil || len(recent) != 1 {
		t.Fatalf("recent biweekly: %v %v", recent, err)
	}
}

func TestRunTrainingEvaluationsPartialFailure(t *testing.T) {
	svc, _ := testsupport.NewService(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			PeriodType string `json:"periodType"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.PeriodType == "biweekly" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"OK","strengths":[],"concerns":[],"suggestions":[]}`))
	}))
	defer srv.Close()

	client := llmeval.New(llmeval.Config{URL: srv.URL, Timeout: 5 * time.Second})
	now := time.Date(2026, 9, 16, 3, 0, 0, 0, domain.JST)

	biweekly, bimonthly, errs := svc.RunTrainingEvaluations(context.Background(), client, now, 8)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %v", errs)
	}
	if biweekly != nil {
		t.Errorf("biweekly should have failed: %+v", biweekly)
	}
	if bimonthly == nil {
		t.Fatalf("bimonthly should have succeeded")
	}
}

func fptr(f float64) *float64 { return &f }
