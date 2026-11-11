package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"training-record/internal/llmconsult"
	"training-record/internal/testsupport"
)

func TestConsultationsAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmconsult.RequestPayload
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.RequestMenuProposal {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"reply": "【メニュー案】\n1. 懸垂 10回 3セット",
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"reply": "直近の記録を確認しました。アドバイスします。",
			})
		}
	}))
	defer ts.Close()

	apiKey := "test-api-key"
	cc := llmconsult.New(llmconsult.Config{URL: ts.URL, Timeout: 2 * time.Second})
	router, _ := testsupport.NewRouterWithConsult(t, apiKey, cc)
	routerNoConsult, _ := testsupport.NewRouter(t, apiKey)

	// 1. Auth check (no token)
	{
		req := httptest.NewRequest("POST", "/api/consultations", bytes.NewReader([]byte(`{"goal":"筋肥大"}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", w.Code)
		}
	}

	// 2. Unconfigured check
	{
		req := httptest.NewRequest("POST", "/api/consultations", bytes.NewReader([]byte(`{"goal":"筋肥大"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		routerNoConsult.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("got %d, want 503", w.Code)
		}
	}

	// 3. Create consultation (Success)
	var createdID int64
	{
		req := httptest.NewRequest("POST", "/api/consultations", bytes.NewReader([]byte(`{"goal":"筋肥大したい"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got status %d, body: %s", w.Code, w.Body.String())
		}
		var res struct {
			ID       int64  `json:"id"`
			Goal     string `json:"goal"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if res.ID <= 0 || res.Goal != "筋肥大したい" || len(res.Messages) != 2 {
			t.Fatalf("unexpected res: %+v", res)
		}
		createdID = res.ID
	}

	// 4. List consultations
	{
		req := httptest.NewRequest("GET", "/api/consultations", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("list got status %d", w.Code)
		}
		var res struct {
			Items []struct {
				ID           int64  `json:"id"`
				MessageCount int    `json:"messageCount"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal list: %v", err)
		}
		if res.Total != 1 || len(res.Items) != 1 || res.Items[0].ID != createdID {
			t.Fatalf("unexpected list res: %+v", res)
		}
	}

	// 5. Get consultation
	{
		req := httptest.NewRequest("GET", "/api/consultations/1", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("get got status %d", w.Code)
		}
	}

	// 6. Add message
	{
		body := `{"content":"週3回できます","requestMenuProposal":false}`
		req := httptest.NewRequest("POST", "/api/consultations/1/messages", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("add message got status %d, body: %s", w.Code, w.Body.String())
		}
	}

	// 7. Request menu proposal
	{
		body := `{"content":"","requestMenuProposal":true}`
		req := httptest.NewRequest("POST", "/api/consultations/1/messages", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("proposal got status %d, body: %s", w.Code, w.Body.String())
		}
		var res struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if len(res.Messages) != 6 {
			t.Fatalf("expected 6 messages, got %d", len(res.Messages))
		}
	}

	// 8. Delete consultation
	{
		req := httptest.NewRequest("DELETE", "/api/consultations/1", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("delete got status %d", w.Code)
		}

		// Verify 404
		reqGet := httptest.NewRequest("GET", "/api/consultations/1", nil)
		reqGet.Header.Set("Authorization", "Bearer "+apiKey)
		wGet := httptest.NewRecorder()
		router.ServeHTTP(wGet, reqGet)
		if wGet.Code != http.StatusNotFound {
			t.Fatalf("expected 404 after delete, got %d", wGet.Code)
		}
	}
}
