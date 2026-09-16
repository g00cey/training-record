package llmeval

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEvaluateRoundTrip(t *testing.T) {
	var gotAuth string
	var gotBody HistoryPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"summary": "全体的に順調です。",
			"strengths": ["頻度が安定している"],
			"concerns": ["ACWRがやや高め"],
			"suggestions": ["来週はデロードを検討"]
		}`))
	}))
	defer srv.Close()

	payload := HistoryPayload{
		PeriodType: "biweekly",
		AsOf:       "2026-09-16",
		PeriodFrom: "2026-09-02",
		PeriodTo:   "2026-09-16",
		Profile:    ProfilePayload{BodyweightKg: 86, HeightCm: 170, MaxHrEst: 180},
	}
	c := New(Config{URL: srv.URL, APIKey: "lk", Timeout: 5 * time.Second})
	res, err := c.Evaluate(context.Background(), payload)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if gotAuth != "Bearer lk" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotBody.PeriodType != "biweekly" || gotBody.AsOf != "2026-09-16" {
		t.Errorf("request body mismatch: %+v", gotBody)
	}
	if res.Summary != "全体的に順調です。" {
		t.Errorf("summary = %q", res.Summary)
	}
	if len(res.Strengths) != 1 || len(res.Concerns) != 1 || len(res.Suggestions) != 1 {
		t.Errorf("lists: %+v", res)
	}
}

func TestEvaluateMarkdownFencedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("評価結果です:\n```json\n{\"summary\":\"OK\",\"strengths\":[],\"concerns\":[],\"suggestions\":[]}\n```\n以上。"))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL})
	res, err := c.Evaluate(context.Background(), HistoryPayload{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if res.Summary != "OK" {
		t.Errorf("summary = %q", res.Summary)
	}
}

func TestEvaluateHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Timeout: 2 * time.Second})
	_, err := c.Evaluate(context.Background(), HistoryPayload{})
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrHTTP || he.Status != 500 {
		t.Fatalf("want ErrHTTP 500, got %v", err)
	}
}

func TestEvaluateTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Timeout: 50 * time.Millisecond})
	_, err := c.Evaluate(context.Background(), HistoryPayload{})
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrTimeout {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestEvaluateUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := New(Config{URL: url, Timeout: 500 * time.Millisecond})
	_, err := c.Evaluate(context.Background(), HistoryPayload{})
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrUnreachable {
		t.Fatalf("want ErrUnreachable, got %v", err)
	}
}

func TestEvaluateNoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("解析できませんでした"))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL})
	_, err := c.Evaluate(context.Background(), HistoryPayload{})
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrDecode {
		t.Fatalf("want ErrDecode, got %v", err)
	}
}

func TestNormalizeTrimAndCap(t *testing.T) {
	res := &Result{
		Summary:     "  順調です  ",
		Strengths:   []string{" 良い ", "", "  "},
		Concerns:    make([]string, 0),
		Suggestions: nil,
	}
	n := Normalize(res)
	if n.Summary != "順調です" {
		t.Errorf("summary = %q", n.Summary)
	}
	if len(n.Strengths) != 1 || n.Strengths[0] != "良い" {
		t.Errorf("strengths = %v", n.Strengths)
	}
	if len(n.Concerns) != 0 || len(n.Suggestions) != 0 {
		t.Errorf("empty lists should stay empty: %+v", n)
	}
}

func TestNormalizeCapsListSize(t *testing.T) {
	items := make([]string, 20)
	for i := range items {
		items[i] = "item"
	}
	n := Normalize(&Result{Strengths: items})
	if len(n.Strengths) != maxListItems {
		t.Errorf("strengths len = %d, want %d", len(n.Strengths), maxListItems)
	}
}

func TestNormalizeCapsRuneLength(t *testing.T) {
	long := make([]rune, 5000)
	for i := range long {
		long[i] = 'あ'
	}
	n := Normalize(&Result{Summary: string(long)})
	if got := len([]rune(n.Summary)); got != maxSummaryRunes {
		t.Errorf("summary runes = %d, want %d", got, maxSummaryRunes)
	}
}
