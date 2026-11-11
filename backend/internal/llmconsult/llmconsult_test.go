package llmconsult

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_Consult_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("got method %s, want POST", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Errorf("got Authorization %q, want 'Bearer secret'", auth)
		}
		var req RequestPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Goal != "筋肥大" {
			t.Errorf("got goal %q, want '筋肥大'", req.Goal)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responsePayload{Reply: "おすすめのメニューです"})
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, APIKey: "secret", Timeout: 2 * time.Second})
	reply, err := c.Consult(context.Background(), RequestPayload{
		Goal: "筋肥大",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "おすすめのメニューです" {
		t.Errorf("got %q, want 'おすすめのメニューです'", reply)
	}
}

func TestClient_Consult_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"BadRequest","message":"goal is required"}`, http.StatusBadRequest)
	}))
	defer ts.Close()

	c := New(Config{URL: ts.URL, Timeout: 2 * time.Second})
	_, err := c.Consult(context.Background(), RequestPayload{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var llmErr *Error
	if !isError(err, &llmErr) || llmErr.Kind != ErrHTTP || llmErr.Status != 400 {
		t.Errorf("got err %v, want ErrHTTP with status 400", err)
	}
}

func isError(err error, target **Error) bool {
	if e, ok := err.(*Error); ok {
		*target = e
		return true
	}
	return false
}
