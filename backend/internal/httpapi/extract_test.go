package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"training-record/internal/hermes"
	"training-record/internal/httpapi"
	"training-record/internal/testsupport"
)

// pngBytes is a minimal valid 1x1 PNG (http.DetectContentType -> image/png).
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// postImage builds a multipart body with an "image" field and posts it to
// /api/spin-extract with auth.
func postImage(h http.Handler, field string, content []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	var contentType string
	if field != "" {
		mw := multipart.NewWriter(&buf)
		if field == "image" {
			fw, _ := mw.CreateFormFile("image", "workout.png")
			_, _ = fw.Write(content)
		} else {
			_ = mw.WriteField(field, "x")
		}
		contentType = mw.FormDataContentType()
		_ = mw.Close()
	} else {
		buf.WriteString("not multipart")
	}
	req := httptest.NewRequest("POST", "/api/spin-extract", &buf)
	req.Header.Set("Authorization", "Bearer "+key)
	if field != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func hermesServer(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func TestSpinExtractSuccess(t *testing.T) {
	srv := hermesServer(200, `{
		"durationMinutes": 52,
		"avgHeartRate": 130,
		"maxHeartRate": 156,
		"distanceKm": 20.3,
		"hrZones": {
			"ウォームアップ": "8:16",
			"インテンシブ": "10:42",
			"有酸素": "6:48",
			"無酸素": "12:05",
			"最大酸素摂取量(高負荷)": "4:42"
		},
		"freeNotes": "消費カロリー 480kcal",
		"uncertainFields": ["distanceKm"]
	}`)
	defer srv.Close()

	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "hermes-key")
	w := postImage(h, "image", pngBytes)
	if w.Code != 200 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["durationMinutes"].(float64) != 52 || m["avgHeartRate"].(float64) != 130 || m["maxHeartRate"].(float64) != 156 {
		t.Errorf("numeric fields: %v", m)
	}
	if m["distanceKm"].(float64) != 20.3 {
		t.Errorf("distanceKm: %v", m["distanceKm"])
	}
	zones, ok := m["hrZones"].(map[string]any)
	if !ok {
		t.Fatalf("hrZones missing: %v", m)
	}
	if len(zones) != 5 {
		t.Errorf("hrZones should always have 5 keys, got %d: %v", len(zones), zones)
	}
	if zones["ウォームアップ"] != "8:16" || zones["最大酸素摂取量(高負荷)"] != "4:42" {
		t.Errorf("zone values: %v", zones)
	}
	uf, _ := m["uncertainFields"].([]any)
	if len(uf) != 1 || uf[0] != "distanceKm" {
		t.Errorf("uncertainFields: %v", m["uncertainFields"])
	}
	if m["freeNotes"] != "消費カロリー 480kcal" {
		t.Errorf("freeNotes: %v", m["freeNotes"])
	}
}

func TestSpinExtractNormalizesInvalidHermesValues(t *testing.T) {
	srv := hermesServer(200, `{
		"durationMinutes": 9999,
		"hrZones": {"謎ゾーン": "5:00", "有酸素": "bad"},
		"uncertainFields": []
	}`)
	defer srv.Close()

	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "")
	w := postImage(h, "image", pngBytes)
	if w.Code != 200 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	if m["durationMinutes"] != nil {
		t.Errorf("out-of-range duration should be null: %v", m["durationMinutes"])
	}
	zones := m["hrZones"].(map[string]any)
	for z, v := range zones {
		if v != nil {
			t.Errorf("zone %q should be null (invalid or unknown), got %v", z, v)
		}
	}
	uf, _ := m["uncertainFields"].([]any)
	if len(uf) != 2 {
		t.Errorf("uncertainFields = %v, want [durationMinutes, hrZones]", uf)
	}
}

func TestSpinExtractNotConfigured(t *testing.T) {
	h, _ := testsupport.NewRouter(t, key)
	w := postImage(h, "image", pngBytes)
	if w.Code != 503 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	errObj := m["error"].(map[string]any)
	if errObj["code"] != "hermes_not_configured" {
		t.Errorf("error code: %v", errObj)
	}
}

func TestSpinExtractHermesHTTPError(t *testing.T) {
	srv := hermesServer(500, `{"error":"boom"}`)
	defer srv.Close()

	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "")
	w := postImage(h, "image", pngBytes)
	if w.Code != 502 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	errObj := m["error"].(map[string]any)
	if errObj["code"] != "hermes_error" {
		t.Errorf("error code: %v", errObj)
	}
}

func TestSpinExtractHermesUnreachable(t *testing.T) {
	srv := hermesServer(200, `{}`)
	url := srv.URL
	srv.Close()

	h := testsupport.NewRouterWithHermes(t, key, url, "")
	w := postImage(h, "image", pngBytes)
	if w.Code != 502 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["error"].(map[string]any)["code"] != "hermes_unreachable" {
		t.Errorf("error code: %v", m["error"])
	}
}

func TestSpinExtractHermesTimeout(t *testing.T) {
	// hermes パッケージ側の短いタイムアウトで 504 への変換を確認する
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	svc, _ := testsupport.NewService(t)
	hc := hermes.New(hermes.Config{URL: srv.URL, Timeout: 50 * time.Millisecond})
	h := httpapi.NewRouter(svc, key, hc)
	w := postImage(h, "image", pngBytes)
	if w.Code != 504 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["error"].(map[string]any)["code"] != "hermes_timeout" {
		t.Errorf("error code: %v", m["error"])
	}
}

func TestSpinExtractBadRequest(t *testing.T) {
	srv := hermesServer(200, `{}`)
	defer srv.Close()
	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "")

	cases := []struct {
		name   string
		field  string
		body   []byte
		wantCk bool // auth は付けてある
	}{
		{"missing image field", "other", pngBytes, true},
		{"not multipart", "", nil, true},
		{"non-image bytes", "image", []byte("plain text, definitely not an image"), true},
		{"empty image", "image", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := postImage(h, tc.field, tc.body)
			if w.Code != 400 {
				t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
			}
			m := decode(t, w)
			if m["error"].(map[string]any)["code"] != "bad_request" {
				t.Errorf("error code: %v", m["error"])
			}
		})
	}
}

func TestSpinExtractOversizedImage(t *testing.T) {
	srv := hermesServer(200, `{}`)
	defer srv.Close()
	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "")

	// PNG ヘッダの後にゴミを詰めて上限超過（Content-Type 判定は通らない大きさ）
	big := append([]byte{}, pngBytes...)
	big = append(big, bytes.Repeat([]byte{0x00}, 11<<20)...)
	w := postImage(h, "image", big)
	if w.Code != 400 {
		t.Fatalf("code = %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "大きすぎ") {
		t.Errorf("body: %s", w.Body.String())
	}
}

func TestSpinExtractRequiresAuth(t *testing.T) {
	srv := hermesServer(200, `{}`)
	defer srv.Close()
	h := testsupport.NewRouterWithHermes(t, key, srv.URL, "")

	req := httptest.NewRequest("POST", "/api/spin-extract", strings.NewReader("x"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("code = %d, want 401", w.Code)
	}
}
