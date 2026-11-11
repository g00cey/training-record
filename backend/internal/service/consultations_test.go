package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"training-record/internal/domain"
	"training-record/internal/llmconsult"
	"training-record/internal/testsupport"
)

func TestService_Consultations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmconsult.RequestPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if req.RequestMenuProposal {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"reply": "【おすすめメニュー】\n- 懸垂: 自重 10回 × 3セット",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"reply": "こんにちは！トレーニングの目的を確認しました。",
			})
		}
	}))
	defer ts.Close()

	svc, st := testsupport.NewService(t)
	llm := llmconsult.New(llmconsult.Config{
		URL:     ts.URL,
		Timeout: 2 * time.Second,
	})

	// 1. Validation error: empty goal
	_, err := svc.StartConsultation(context.Background(), llm, "   ")
	if err == nil {
		t.Fatal("expected error on empty goal, got nil")
	}

	// Setup some history in DB
	today := domain.Today()
	_, _ = st.CreateStrength(today, "自重のみ", []domain.Exercise{
		{Name: "懸垂", Weight: nil, Reps: 10, Sets: 1},
	})

	// 2. Start consultation
	c, err := svc.StartConsultation(context.Background(), llm, "筋肥大したい")
	if err != nil {
		t.Fatalf("start consultation: %v", err)
	}
	if c.Goal != "筋肥大したい" {
		t.Errorf("got goal %q, want '筋肥大したい'", c.Goal)
	}
	if len(c.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(c.Messages))
	}
	if c.Messages[1].Content != "こんにちは！トレーニングの目的を確認しました。" {
		t.Errorf("unexpected reply: %s", c.Messages[1].Content)
	}

	// 3. Add message
	updated, err := svc.AddConsultationMessage(context.Background(), llm, c.ID, "週3回のペースでやりたいです", false)
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	if len(updated.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(updated.Messages))
	}

	// 4. Request menu proposal with empty content (fallback text)
	proposal, err := svc.AddConsultationMessage(context.Background(), llm, c.ID, "", true)
	if err != nil {
		t.Fatalf("request proposal: %v", err)
	}
	if len(proposal.Messages) != 6 {
		t.Fatalf("expected 6 messages, got %d", len(proposal.Messages))
	}
	if proposal.Messages[4].Content != "ここまでの相談内容をもとに、具体的なトレーニングメニューを提案してください。" {
		t.Errorf("got fallback message %q", proposal.Messages[4].Content)
	}
	if proposal.Messages[5].Content != "【おすすめメニュー】\n- 懸垂: 自重 10回 × 3セット" {
		t.Errorf("got reply %q", proposal.Messages[5].Content)
	}

	// 5. List
	list, total, err := svc.ListConsultations(10, 0)
	if err != nil {
		t.Fatalf("list consultations: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("got total %d, list len %d", total, len(list))
	}
	if list[0].MessageCount != 6 {
		t.Errorf("got message count %d, want 6", list[0].MessageCount)
	}

	// 6. Get
	got, err := svc.GetConsultation(c.ID)
	if err != nil {
		t.Fatalf("get consultation: %v", err)
	}
	if got.ID != c.ID || len(got.Messages) != 6 {
		t.Errorf("unexpected got: %+v", got)
	}

	// 7. Delete
	if err := svc.DeleteConsultation(c.ID); err != nil {
		t.Fatalf("delete consultation: %v", err)
	}
	_, err = svc.GetConsultation(c.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}
