package store_test

import (
	"errors"
	"testing"

	"training-record/internal/store"
	"training-record/internal/testsupport"
)

func TestConsultationsCRUD(t *testing.T) {
	st := testsupport.NewStore(t)

	// 1. Create
	c, err := st.CreateConsultation(
		"筋肥大したい",
		`{"asOf":"2026-11-10"}`,
		"筋肥大したい",
		"現在のメニューを見直しましょう",
	)
	if err != nil {
		t.Fatalf("create consultation: %v", err)
	}
	if c.ID <= 0 {
		t.Fatalf("invalid ID: %d", c.ID)
	}
	if c.Goal != "筋肥大したい" {
		t.Errorf("got goal %q, want '筋肥大したい'", c.Goal)
	}
	if len(c.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(c.Messages))
	}
	if c.Messages[0].Role != "user" || c.Messages[0].Content != "筋肥大したい" {
		t.Errorf("unexpected first message: %+v", c.Messages[0])
	}
	if c.Messages[1].Role != "assistant" || c.Messages[1].Content != "現在のメニューを見直しましょう" {
		t.Errorf("unexpected second message: %+v", c.Messages[1])
	}

	// 2. Get
	got, err := st.GetConsultation(c.ID)
	if err != nil {
		t.Fatalf("get consultation: %v", err)
	}
	if got.ID != c.ID || got.Goal != c.Goal || len(got.Messages) != 2 {
		t.Errorf("unexpected got: %+v", got)
	}

	// 3. Add Message
	updated, err := st.AddConsultationMessage(c.ID, "週4回できます", "では上半身と下半身で分割しましょう")
	if err != nil {
		t.Fatalf("add message: %v", err)
	}
	if len(updated.Messages) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(updated.Messages))
	}
	if updated.Messages[2].Content != "週4回できます" || updated.Messages[3].Content != "では上半身と下半身で分割しましょう" {
		t.Errorf("unexpected new messages: %+v, %+v", updated.Messages[2], updated.Messages[3])
	}

	// 4. List
	items, total, err := st.ListConsultations(10, 0)
	if err != nil {
		t.Fatalf("list consultations: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("got total %d, len %d; want 1, 1", total, len(items))
	}
	if items[0].MessageCount != 4 {
		t.Errorf("got message count %d, want 4", items[0].MessageCount)
	}

	// 5. Delete
	if err := st.DeleteConsultation(c.ID); err != nil {
		t.Fatalf("delete consultation: %v", err)
	}
	if _, err := st.GetConsultation(c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
