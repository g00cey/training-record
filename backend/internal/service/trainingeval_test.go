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

// evalRequest captures the parts of the outbound evaluation payload the
// tests assert on.
type evalRequest struct {
	PeriodType string `json:"periodType"`
	AsOf       string `json:"asOf"`
	PeriodFrom string `json:"periodFrom"`
	PeriodTo   string `json:"periodTo"`
	Strength   []struct {
		Date string `json:"date"`
	} `json:"strengthSessions"`
}

func TestRunTrainingEvaluationsSavesBothPeriods(t *testing.T) {
	svc, st := testsupport.NewService(t)

	// 2026-09-16 (水) 時点の固定2週間グリッド:
	//   進行中の期間      = 2026-09-06 〜 2026-09-19
	//   完了済みの直近期間 = 2026-08-23 〜 2026-09-05
	for _, d := range []string{"2026-09-02", "2026-09-10"} {
		if _, err := st.CreateStrength(d, "フリーウェイト", []domain.Exercise{
			{Name: "ベンチプレス", Weight: fptr(60), Reps: 8, Sets: 3},
		}); err != nil {
			t.Fatalf("seed strength %s: %v", d, err)
		}
	}

	var got []evalRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body evalRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, body)
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

	// 完了済み期間の確定評価 + 進行中期間の毎日更新 + bimonthly の3回呼ぶ。
	if len(got) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d: %+v", len(got), got)
	}
	byType := map[string][]evalRequest{}
	for _, g := range got {
		byType[g.PeriodType] = append(byType[g.PeriodType], g)
	}
	if len(byType["biweekly"]) != 2 || len(byType["bimonthly"]) != 1 {
		t.Fatalf("LLM calls by type: biweekly=%d bimonthly=%d", len(byType["biweekly"]), len(byType["bimonthly"]))
	}

	// 完了済み期間（全14日分のデータで確定）: 9/2 のセッションのみ含む。
	fin := byType["biweekly"][0]
	if fin.PeriodFrom != "2026-08-23" || fin.PeriodTo != "2026-09-05" || fin.AsOf != "2026-09-16" {
		t.Errorf("finalize payload period = %s〜%s asOf=%s, want 2026-08-23〜2026-09-05", fin.PeriodFrom, fin.PeriodTo, fin.AsOf)
	}
	if len(fin.Strength) != 1 || fin.Strength[0].Date != "2026-09-02" {
		t.Errorf("finalize payload strength dates = %+v, want [2026-09-02]", fin.Strength)
	}

	// 進行中の期間（評価日までのセッション）: 9/10 のみ含む。
	cur := byType["biweekly"][1]
	if cur.PeriodFrom != "2026-09-06" || cur.PeriodTo != "2026-09-19" {
		t.Errorf("current payload period = %s〜%s, want 2026-09-06〜2026-09-19", cur.PeriodFrom, cur.PeriodTo)
	}
	if len(cur.Strength) != 1 || cur.Strength[0].Date != "2026-09-10" {
		t.Errorf("current payload strength dates = %+v, want [2026-09-10]", cur.Strength)
	}

	// 戻り値は進行中期間の方（履歴の先頭に来る評価）。
	if biweekly.PeriodType != "biweekly" || biweekly.PeriodFrom != "2026-09-06" || biweekly.PeriodTo != "2026-09-19" {
		t.Errorf("biweekly result = %s〜%s, want 2026-09-06〜2026-09-19", biweekly.PeriodFrom, biweekly.PeriodTo)
	}
	if bimonthly.PeriodFrom != "2026-07-22" || bimonthly.PeriodTo != "2026-09-16" {
		t.Errorf("bimonthly result = %s〜%s, want 2026-07-22〜2026-09-16", bimonthly.PeriodFrom, bimonthly.PeriodTo)
	}

	latest, err := svc.LatestTrainingEvaluation("bimonthly")
	if err != nil || latest == nil {
		t.Fatalf("latest bimonthly: %v %v", latest, err)
	}
	if latest.Summary != "順調です" {
		t.Errorf("summary = %q", latest.Summary)
	}

	// biweekly は1期間1行: 完了済み期間と進行中期間の2行だけ。
	recent, err := svc.RecentTrainingEvaluations("biweekly", 10)
	if err != nil || len(recent) != 2 {
		t.Fatalf("recent biweekly: %v %v", recent, err)
	}
	for _, p := range [][2]string{{"2026-08-23", "2026-09-05"}, {"2026-09-06", "2026-09-19"}} {
		rec, err := st.TrainingEvaluationForPeriod("biweekly", p[0], p[1])
		if err != nil || rec == nil {
			t.Fatalf("ForPeriod(%v): (%v, %v)", p, rec, err)
		}
	}
}

func TestRunTrainingEvaluationsBiweeklyUpsertsDaily(t *testing.T) {
	svc, st := testsupport.NewService(t)

	if _, err := st.CreateStrength("2026-09-10", "フリーウェイト", []domain.Exercise{
		{Name: "ベンチプレス", Weight: fptr(60), Reps: 8, Sets: 3},
	}); err != nil {
		t.Fatalf("seed strength: %v", err)
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"順調です","strengths":[],"concerns":[],"suggestions":[]}`))
	}))
	defer srv.Close()

	client := llmeval.New(llmeval.Config{URL: srv.URL, Timeout: 5 * time.Second})

	// 1日目 (9/16): 確定 [8/23〜9/5] + 進行中 [9/6〜9/19] + bimonthly = 3回。
	if _, _, errs := svc.RunTrainingEvaluations(context.Background(), client,
		time.Date(2026, 9, 16, 3, 0, 0, 0, domain.JST), 8); len(errs) != 0 {
		t.Fatalf("day 1 errs: %v", errs)
	}
	if calls != 3 {
		t.Fatalf("day 1 LLM calls = %d, want 3", calls)
	}

	// 2日目 (9/17): [8/23〜9/5] は確定済み（評価日 9/16 > 9/5）なのでスキップ。
	// 進行中 [9/6〜9/19] の上書き更新 + bimonthly = 2回。
	biweekly, _, errs := svc.RunTrainingEvaluations(context.Background(), client,
		time.Date(2026, 9, 17, 3, 0, 0, 0, domain.JST), 8)
	if len(errs) != 0 {
		t.Fatalf("day 2 errs: %v", errs)
	}
	if calls != 5 {
		t.Fatalf("day 2 total LLM calls = %d, want 5", calls)
	}
	if biweekly == nil || biweekly.PeriodFrom != "2026-09-06" {
		t.Fatalf("day 2 biweekly = %+v", biweekly)
	}

	// それでも biweekly は1期間1行のまま（上書きであって追加ではない）。
	recent, err := svc.RecentTrainingEvaluations("biweekly", 10)
	if err != nil || len(recent) != 2 {
		t.Fatalf("recent biweekly: %v %v", recent, err)
	}
	cur, err := st.TrainingEvaluationForPeriod("biweekly", "2026-09-06", "2026-09-19")
	if err != nil || cur == nil {
		t.Fatalf("ForPeriod current: %v %v", cur, err)
	}
	if cur.EvaluatedAt != "2026-09-17T03:00:00+09:00" {
		t.Errorf("current row evaluatedAt = %q, want 2026-09-17T03:00:00+09:00", cur.EvaluatedAt)
	}
	fin, err := st.TrainingEvaluationForPeriod("biweekly", "2026-08-23", "2026-09-05")
	if err != nil || fin == nil {
		t.Fatalf("ForPeriod finalized: %v %v", fin, err)
	}
	if fin.EvaluatedAt != "2026-09-16T03:00:00+09:00" {
		t.Errorf("finalized row evaluatedAt = %q, want 2026-09-16T03:00:00+09:00 (untouched)", fin.EvaluatedAt)
	}
}

func TestRunTrainingEvaluationsBiweeklyFinalizesCompletedPeriod(t *testing.T) {
	svc, st := testsupport.NewService(t)

	// 期間 [9/6〜9/19] の最初と最終日のセッション。
	for _, d := range []string{"2026-09-10", "2026-09-19"} {
		if _, err := st.CreateStrength(d, "フリーウェイト", []domain.Exercise{
			{Name: "ベンチプレス", Weight: fptr(60), Reps: 8, Sets: 3},
		}); err != nil {
			t.Fatalf("seed strength %s: %v", d, err)
		}
	}

	var finalizeStrengthDates []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body evalRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.PeriodType == "biweekly" && body.PeriodFrom == "2026-09-06" {
			for _, s := range body.Strength {
				finalizeStrengthDates = append(finalizeStrengthDates, s.Date)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"確定です","strengths":[],"concerns":[],"suggestions":[]}`))
	}))
	defer srv.Close()

	client := llmeval.New(llmeval.Config{URL: srv.URL, Timeout: 5 * time.Second})

	// 期間中の実行 (9/16): 進行中評価には 9/19（期間最終日）はまだ含まれない
	// （セッションの上限は評価日まで）。
	if _, _, errs := svc.RunTrainingEvaluations(context.Background(), client,
		time.Date(2026, 9, 16, 3, 0, 0, 0, domain.JST), 8); len(errs) != 0 {
		t.Fatalf("run 1 errs: %v", errs)
	}
	if len(finalizeStrengthDates) != 1 || finalizeStrengthDates[0] != "2026-09-10" {
		t.Fatalf("run 1 in-progress payload strength dates = %v, want [2026-09-10]", finalizeStrengthDates)
	}
	finalizeStrengthDates = nil

	// 期間終了後の最初の実行 (9/20 = 新期間 [9/20〜10/3] の日曜):
	// [9/6〜9/19] を全14日分のデータで確定し直す。新期間はまだ記録が
	// 無いので評価を作らない（bimonthly との2回のみ）。
	biweekly, _, errs := svc.RunTrainingEvaluations(context.Background(), client,
		time.Date(2026, 9, 20, 3, 0, 0, 0, domain.JST), 8)
	if len(errs) != 0 {
		t.Fatalf("run 2 errs: %v", errs)
	}
	if biweekly == nil || biweekly.PeriodFrom != "2026-09-06" || biweekly.PeriodTo != "2026-09-19" {
		t.Fatalf("run 2 biweekly = %+v, want the finalized 2026-09-06〜2026-09-19", biweekly)
	}
	if len(finalizeStrengthDates) != 2 ||
		finalizeStrengthDates[0] != "2026-09-10" || finalizeStrengthDates[1] != "2026-09-19" {
		t.Errorf("finalize payload strength dates = %v, want [2026-09-10 2026-09-19]", finalizeStrengthDates)
	}

	// 確定済みなので翌日以降はこの期間を再評価しない。
	if _, _, errs := svc.RunTrainingEvaluations(context.Background(), client,
		time.Date(2026, 9, 21, 3, 0, 0, 0, domain.JST), 8); len(errs) != 0 {
		t.Fatalf("run 3 errs: %v", errs)
	}
	fin, err := st.TrainingEvaluationForPeriod("biweekly", "2026-09-06", "2026-09-19")
	if err != nil || fin == nil {
		t.Fatalf("ForPeriod finalized: %v %v", fin, err)
	}
	if fin.EvaluatedAt != "2026-09-20T03:00:00+09:00" {
		t.Errorf("finalized evaluatedAt = %q, want 2026-09-20T03:00:00+09:00", fin.EvaluatedAt)
	}
	if fin.Summary != "確定です" {
		t.Errorf("finalized summary = %q", fin.Summary)
	}

	// 記録の無い新期間 [9/20〜10/3] の評価行は作られない。
	if rec, err := st.TrainingEvaluationForPeriod("biweekly", "2026-09-20", "2026-10-03"); err != nil || rec != nil {
		t.Fatalf("empty-period evaluation should not exist: %v %v", rec, err)
	}
	recent, err := svc.RecentTrainingEvaluations("biweekly", 10)
	if err != nil || len(recent) != 2 {
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
