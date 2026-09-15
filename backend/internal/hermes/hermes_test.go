package hermes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtractRoundTrip(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req extractRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotBody = req.ImageBase64
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"durationMinutes": 52,
			"avgHeartRate": 130,
			"maxHeartRate": 156,
			"distanceKm": 20.3,
			"hrZones": {"ウォームアップ":"8:16","無酸素":"12:05"},
			"freeNotes": "消費カロリー 480kcal",
			"uncertainFields": ["distanceKm"]
		}`))
	}))
	defer srv.Close()

	img := []byte("fake-jpeg-bytes")
	c := New(Config{URL: srv.URL, APIKey: "hk", Timeout: 5 * time.Second})
	res, err := c.Extract(context.Background(), img, "image/jpeg")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if gotAuth != "Bearer hk" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if want := base64.StdEncoding.EncodeToString(img); gotBody != want {
		t.Errorf("imageBase64 mismatch")
	}
	if *res.DurationMinutes != 52 || *res.AvgHeartRate != 130 || *res.MaxHeartRate != 156 {
		t.Errorf("numeric fields: %+v", res)
	}
	if *res.DistanceKm != 20.3 {
		t.Errorf("distanceKm = %v", *res.DistanceKm)
	}
	if res.HrZones["ウォームアップ"] != "8:16" || res.HrZones["無酸素"] != "12:05" {
		t.Errorf("hrZones: %v", res.HrZones)
	}
	if res.FreeNotes != "消費カロリー 480kcal" {
		t.Errorf("freeNotes = %q", res.FreeNotes)
	}
	if len(res.UncertainFields) != 1 || res.UncertainFields[0] != "distanceKm" {
		t.Errorf("uncertainFields: %v", res.UncertainFields)
	}
}

func TestExtractSnakeCaseKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"duration_minutes": 30,
			"avg_heart_rate": 148,
			"max_heart_rate": 172,
			"distance_km": null,
			"hr_zones": {"有酸素":"6:48"},
			"free_notes": "",
			"uncertain_fields": []
		}`))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL})
	res, err := c.Extract(context.Background(), []byte("x"), "image/png")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if *res.DurationMinutes != 30 || *res.AvgHeartRate != 148 || *res.MaxHeartRate != 172 {
		t.Errorf("fields: %+v", res)
	}
	if res.DistanceKm != nil {
		t.Errorf("distanceKm should be nil, got %v", *res.DistanceKm)
	}
	if res.HrZones["有酸素"] != "6:48" {
		t.Errorf("hrZones: %v", res.HrZones)
	}
}

func TestExtractMarkdownFencedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("抽出結果です:\n```json\n{\"durationMinutes\":45,\"avgHeartRate\":120}\n```\n以上。"))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL})
	res, err := c.Extract(context.Background(), []byte("x"), "image/jpeg")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if *res.DurationMinutes != 45 || *res.AvgHeartRate != 120 {
		t.Errorf("fields: %+v", res)
	}
}

func TestExtractHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Timeout: 2 * time.Second})
	_, err := c.Extract(context.Background(), []byte("x"), "image/jpeg")
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrHTTP || he.Status != 500 {
		t.Fatalf("want ErrHTTP 500, got %v", err)
	}
}

func TestExtractTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, Timeout: 50 * time.Millisecond})
	_, err := c.Extract(context.Background(), []byte("x"), "image/jpeg")
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrTimeout {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}

func TestExtractUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := New(Config{URL: url, Timeout: 500 * time.Millisecond})
	_, err := c.Extract(context.Background(), []byte("x"), "image/jpeg")
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrUnreachable {
		t.Fatalf("want ErrUnreachable, got %v", err)
	}
}

func TestExtractNoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("画像を読み取れませんでした"))
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL})
	_, err := c.Extract(context.Background(), []byte("x"), "image/jpeg")
	var he *Error
	if !errors.As(err, &he) || he.Kind != ErrDecode {
		t.Fatalf("want ErrDecode, got %v", err)
	}
}

func intPtr(n int) *int           { return &n }
func floatPtr(f float64) *float64 { return &f }

func TestNormalizeValid(t *testing.T) {
	res := &Result{
		DurationMinutes: intPtr(52),
		AvgHeartRate:    intPtr(130),
		MaxHeartRate:    intPtr(156),
		DistanceKm:      floatPtr(20.3),
		HrZones: map[string]string{
			"ウォームアップ":      "8:16",
			"インテンシブ":       "10:42",
			"有酸素":          "6:48",
			"無酸素":          "12:05",
			"最大酸素摂取量(高負荷)": "4:42",
		},
		FreeNotes:       "  消費カロリー 480kcal  ",
		UncertainFields: []string{"freeNotes", "unknown-field", " freeNotes "},
	}
	n := Normalize(res)
	if n.DurationMinutes == nil || n.AvgHeartRate == nil || n.MaxHeartRate == nil || n.DistanceKm == nil {
		t.Fatalf("all fields should survive: %+v", n)
	}
	for _, z := range CanonicalZones {
		if _, ok := n.HrZones[z]; !ok {
			t.Errorf("zone %q missing", z)
		}
	}
	if n.FreeNotes != "消費カロリー 480kcal" {
		t.Errorf("freeNotes = %q", n.FreeNotes)
	}
	// unknown-field は除去され、freeNotes の重複は 1 つになる
	if len(n.UncertainFields) != 1 || n.UncertainFields[0] != "freeNotes" {
		t.Errorf("uncertainFields: %v", n.UncertainFields)
	}
}

func TestNormalizeOutOfRangeAndInvalid(t *testing.T) {
	res := &Result{
		DurationMinutes: intPtr(9999), // 範囲外 → nil + uncertain
		AvgHeartRate:    intPtr(20),   // 範囲外
		MaxHeartRate:    intPtr(999),  // 範囲外
		DistanceKm:      floatPtr(-1), // 範囲外
		HrZones: map[string]string{
			"ウォームアップ": "8:16", // OK
			"有酸素":     "6.48", // mm:ss でない → hrZones uncertain
			"謎ゾーン":    "5:00", // 未知ラベル → 破棄
		},
	}
	n := Normalize(res)
	if n.DurationMinutes != nil || n.AvgHeartRate != nil || n.MaxHeartRate != nil || n.DistanceKm != nil {
		t.Errorf("out-of-range fields should be nil: %+v", n)
	}
	if _, ok := n.HrZones["ウォームアップ"]; !ok {
		t.Errorf("valid zone dropped")
	}
	if len(n.HrZones) != 1 {
		t.Errorf("hrZones = %v", n.HrZones)
	}
	want := []string{"avgHeartRate", "distanceKm", "durationMinutes", "hrZones", "maxHeartRate"}
	if len(n.UncertainFields) != len(want) {
		t.Fatalf("uncertainFields = %v, want %v", n.UncertainFields, want)
	}
	for i, f := range want {
		if n.UncertainFields[i] != f {
			t.Fatalf("uncertainFields = %v, want %v", n.UncertainFields, want)
		}
	}
}

func TestNormalizeAvgGreaterThanMax(t *testing.T) {
	res := &Result{
		DurationMinutes: intPtr(30),
		AvgHeartRate:    intPtr(160),
		MaxHeartRate:    intPtr(140),
	}
	n := Normalize(res)
	// 値は残すが両方 uncertain に上げる
	if n.AvgHeartRate == nil || *n.AvgHeartRate != 160 || n.MaxHeartRate == nil || *n.MaxHeartRate != 140 {
		t.Errorf("values should be kept: %+v", n)
	}
	hasAvg, hasMax := false, false
	for _, f := range n.UncertainFields {
		if f == "avgHeartRate" {
			hasAvg = true
		}
		if f == "maxHeartRate" {
			hasMax = true
		}
	}
	if !hasAvg || !hasMax {
		t.Errorf("avg/max should be flagged: %v", n.UncertainFields)
	}
}

func TestNormalizeZoneAliases(t *testing.T) {
	res := &Result{
		DurationMinutes: intPtr(30),
		HrZones: map[string]string{
			"Z1":           "5:00",
			"Zone 2":       "10:00",
			"最大酸素摂取量（高負荷）": "1:00", // 全角括弧
			"最大酸素摂取量":      "2:00", // 接尾辞なし
			"aerobic":      "3:00",
		},
	}
	n := Normalize(res)
	for _, z := range []string{"ウォームアップ", "インテンシブ", "最大酸素摂取量(高負荷)", "有酸素"} {
		if _, ok := n.HrZones[z]; !ok {
			t.Errorf("alias for %q not mapped (got %v)", z, n.HrZones)
		}
	}
}

func TestNormalizeFreeNotesCap(t *testing.T) {
	long := make([]rune, 800)
	for i := range long {
		long[i] = 'あ'
	}
	res := &Result{DurationMinutes: intPtr(30), FreeNotes: string(long)}
	n := Normalize(res)
	if got := len([]rune(n.FreeNotes)); got != maxFreeNotesRunes {
		t.Errorf("freeNotes runes = %d, want %d", got, maxFreeNotesRunes)
	}
}
