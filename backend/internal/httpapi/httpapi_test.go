package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"training-record/internal/testsupport"
)

const key = "testkey"

type client struct {
	t testing.TB
	h http.Handler
}

func newClient(t *testing.T) *client {
	h, _ := testsupport.NewRouter(t, key)
	return &client{t: t, h: h}
}

func (c *client) do(method, path, body string, auth bool) *httptest.ResponseRecorder {
	c.t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if auth {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	return w
}

func (c *client) req(method, path, body string) *httptest.ResponseRecorder {
	return c.do(method, path, body, true)
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

func TestHealthNoAuth(t *testing.T) {
	c := newClient(t)
	w := c.do("GET", "/api/health", "", false)
	if w.Code != 200 {
		t.Fatalf("health code = %d", w.Code)
	}
	m := decode(t, w)
	if m["status"] != "ok" || m["time"] == nil {
		t.Fatalf("health body: %v", m)
	}
}

func TestAuthRequired(t *testing.T) {
	c := newClient(t)
	for _, tc := range []struct {
		name string
		auth bool
		hdr  string
		code int
	}{
		{"no header", false, "", 401},
		{"wrong key", true, "Bearer nope", 401},
		{"ok", true, "Bearer " + key, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/exercises", nil)
			if tc.hdr != "" {
				req.Header.Set("Authorization", tc.hdr)
			}
			w := httptest.NewRecorder()
			c.h.ServeHTTP(w, req)
			if w.Code != tc.code {
				t.Fatalf("code = %d, want %d (%s)", w.Code, tc.code, w.Body.String())
			}
			if tc.code == 401 {
				m := decode(t, w)
				errObj, ok := m["error"].(map[string]any)
				if !ok || errObj["code"] != "unauthorized" || errObj["message"] == "" {
					t.Fatalf("error envelope: %v", m)
				}
			}
		})
	}
}

func TestStrengthLifecycle(t *testing.T) {
	c := newClient(t)

	// create
	w := c.req("POST", "/api/strength-sessions", `{
		"date":"2026-09-08","notes":"自重＋フリーウェイト",
		"exercises":[
			{"name":"ヒップストラスト","weight":40,"reps":40,"sets":1},
			{"name":"懸垂","weight":null,"reps":10}
		]}`)
	if w.Code != 201 {
		t.Fatalf("create code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["date"] != "2026-09-08" {
		t.Fatalf("create body: %v", m)
	}
	exs := m["exercises"].([]any)
	if len(exs) != 2 {
		t.Fatalf("exercises len = %d", len(exs))
	}
	ex0 := exs[0].(map[string]any)
	if ex0["sortOrder"].(float64) != 0 || ex0["weight"].(float64) != 40 {
		t.Fatalf("ex0: %v", ex0)
	}
	if exs[1].(map[string]any)["weight"] != nil {
		t.Fatalf("bodyweight exercise weight should be null: %v", exs[1])
	}

	// duplicate -> 409 conflict
	w = c.req("POST", "/api/strength-sessions", `{"date":"2026-09-08","exercises":[]}`)
	if w.Code != 409 || decode(t, w)["error"].(map[string]any)["code"] != "conflict" {
		t.Fatalf("dup code = %d (%s)", w.Code, w.Body.String())
	}

	// get
	w = c.req("GET", "/api/strength-sessions/2026-09-08", "")
	if w.Code != 200 {
		t.Fatalf("get code = %d", w.Code)
	}

	// get missing -> 404 not_found
	w = c.req("GET", "/api/strength-sessions/2000-01-01", "")
	if w.Code != 404 || decode(t, w)["error"].(map[string]any)["code"] != "not_found" {
		t.Fatalf("missing get: %d (%s)", w.Code, w.Body.String())
	}

	// bad date -> 400 bad_request
	w = c.req("GET", "/api/strength-sessions/not-a-date", "")
	if w.Code != 400 {
		t.Fatalf("bad date code = %d", w.Code)
	}

	// patch notes
	w = c.req("PATCH", "/api/strength-sessions/2026-09-08", `{"notes":"更新済み"}`)
	if w.Code != 200 || decode(t, w)["notes"] != "更新済み" {
		t.Fatalf("patch code = %d (%s)", w.Code, w.Body.String())
	}

	// append exercise
	w = c.req("POST", "/api/strength-sessions/2026-09-08/exercises", `{"exercises":[{"name":"腕立て伏せ","reps":30}],"notes":"自重のみ"}`)
	if w.Code != 200 {
		t.Fatalf("append code = %d (%s)", w.Code, w.Body.String())
	}
	m = decode(t, w)
	if len(m["exercises"].([]any)) != 3 {
		t.Fatalf("after append: %v", m["exercises"])
	}
	if m["notes"] != "更新済み; 自重のみ" {
		t.Fatalf("append notes join: %q", m["notes"])
	}

	// reorder
	var ids []int64
	for _, e := range m["exercises"].([]any) {
		ids = append(ids, int64(e.(map[string]any)["id"].(float64)))
	}
	body, _ := json.Marshal(map[string]any{"orderedIds": []int64{ids[2], ids[0], ids[1]}})
	w = c.req("PUT", "/api/strength-sessions/2026-09-08/exercises:reorder", string(body))
	if w.Code != 200 {
		t.Fatalf("reorder code = %d (%s)", w.Code, w.Body.String())
	}
	first := decode(t, w)["exercises"].([]any)[0].(map[string]any)
	if int64(first["id"].(float64)) != ids[2] {
		t.Fatalf("reorder did not apply: %v", first)
	}

	// update single exercise
	w = c.req("PUT", "/api/strength-sessions/2026-09-08/exercises/"+itoa(ids[0]),
		`{"name":"ヒップストラスト","weight":42,"reps":40,"sets":1,"notes":""}`)
	if w.Code != 200 {
		t.Fatalf("put exercise code = %d (%s)", w.Code, w.Body.String())
	}

	// delete exercise -> 204
	w = c.req("DELETE", "/api/strength-sessions/2026-09-08/exercises/"+itoa(ids[1]), "")
	if w.Code != 204 {
		t.Fatalf("delete exercise code = %d", w.Code)
	}

	// list
	w = c.req("GET", "/api/strength-sessions?limit=10", "")
	m = decode(t, w)
	if int(m["total"].(float64)) != 1 || len(m["items"].([]any)) != 1 {
		t.Fatalf("list: %v", m)
	}

	// PUT upsert new date -> 201
	w = c.req("PUT", "/api/strength-sessions/2026-09-09", `{"notes":"x","exercises":[]}`)
	if w.Code != 201 {
		t.Fatalf("put new code = %d", w.Code)
	}

	// delete session -> 204, then 404
	if w = c.req("DELETE", "/api/strength-sessions/2026-09-08", ""); w.Code != 204 {
		t.Fatalf("delete session code = %d", w.Code)
	}
	if w = c.req("DELETE", "/api/strength-sessions/2026-09-08", ""); w.Code != 404 {
		t.Fatalf("re-delete code = %d", w.Code)
	}

	// validation: missing reps -> 422 unprocessable
	w = c.req("POST", "/api/strength-sessions", `{"date":"2026-10-01","exercises":[{"name":"懸垂"}]}`)
	if w.Code != 422 || decode(t, w)["error"].(map[string]any)["code"] != "unprocessable" {
		t.Fatalf("missing reps: %d (%s)", w.Code, w.Body.String())
	}
}

func TestSpinRPEAuto(t *testing.T) {
	c := newClient(t)

	// rpe null + avg/max present -> round(130/167*10) = 8
	w := c.req("POST", "/api/spin-sessions", `{"date":"2026-07-12","duration_minutes":46,"avg_heart_rate":130,"max_heart_rate":167,"rpe":null}`)
	if w.Code != 201 {
		t.Fatalf("spin create code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["rpe"].(float64) != 8 {
		t.Fatalf("auto rpe = %v, want 8", m["rpe"])
	}
	if m["durationMinutes"].(float64) != 46 {
		t.Fatalf("camelCase durationMinutes missing: %v", m)
	}

	// explicit rpe kept
	w = c.req("PUT", "/api/spin-sessions/2026-07-13", `{"duration_minutes":11,"avg_heart_rate":133,"max_heart_rate":162,"rpe":5}`)
	if decode(t, w)["rpe"].(float64) != 5 {
		t.Fatalf("explicit rpe overwritten (%s)", w.Body.String())
	}

	// patch: clearing rpe then it re-derives from HR
	w = c.req("PATCH", "/api/spin-sessions/2026-07-13", `{"rpe":null}`)
	if w.Code != 200 || decode(t, w)["rpe"].(float64) != 8 {
		t.Fatalf("patch re-derive rpe: %d (%s)", w.Code, w.Body.String())
	}

	// rpe out of range -> 422
	w = c.req("POST", "/api/spin-sessions", `{"date":"2026-07-14","duration_minutes":25,"rpe":42}`)
	if w.Code != 422 {
		t.Fatalf("rpe range code = %d", w.Code)
	}

	// duplicate -> 409
	w = c.req("POST", "/api/spin-sessions", `{"date":"2026-07-12","duration_minutes":1}`)
	if w.Code != 409 {
		t.Fatalf("spin dup code = %d", w.Code)
	}

	if w = c.req("DELETE", "/api/spin-sessions/2026-07-12", ""); w.Code != 204 {
		t.Fatalf("spin delete code = %d", w.Code)
	}
}

func TestPresetEndpoints(t *testing.T) {
	c := newClient(t)

	// empty list
	if list := decode(t, c.req("GET", "/api/presets", ""))["presets"].([]any); len(list) != 0 {
		t.Fatalf("expected empty preset list, got %v", list)
	}

	// create
	w := c.req("POST", "/api/presets", `{"name":"自重","sortOrder":0}`)
	if w.Code != 201 || decode(t, w)["sortOrder"].(float64) != 0 {
		t.Fatalf("create 自重: %d (%s)", w.Code, w.Body.String())
	}
	if w = c.req("POST", "/api/presets", `{"name":"FW"}`); w.Code != 201 {
		t.Fatalf("create FW: %d (%s)", w.Code, w.Body.String())
	}
	// duplicate -> 409
	if w = c.req("POST", "/api/presets", `{"name":"自重"}`); w.Code != 409 ||
		decode(t, w)["error"].(map[string]any)["code"] != "conflict" {
		t.Fatalf("dup preset: %d (%s)", w.Code, w.Body.String())
	}

	// unknown preset -> 404
	if w = c.req("GET", "/api/presets/"+urlSeg("自重"), ""); w.Code != 404 {
		t.Fatalf("preset with no snapshot should 404: %d (%s)", w.Code, w.Body.String())
	}

	// PUT preset (creates first snapshot, explicit date)
	w = c.req("PUT", "/api/presets/"+urlSeg("自重"), `{"date":"2026-07-30","exercises":[
		{"name":"懸垂","weight":null,"reps":10,"sets":1},
		{"name":"ディップス","weight":null,"reps":30}
	]}`)
	if w.Code != 200 {
		t.Fatalf("put 自重: %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	if m["name"] != "自重" || m["date"] != "2026-07-30" || len(m["exercises"].([]any)) != 2 {
		t.Fatalf("put 自重 body: %v", m)
	}
	if _, ok := m["exercises"].([]any)[0].(map[string]any)["name"]; !ok {
		t.Fatalf("preset exercise must use key 'name': %v", m["exercises"])
	}

	// PUT again with a new (default = today) date -> history +1
	w = c.req("PUT", "/api/presets/"+urlSeg("自重"), `{"exercises":[{"name":"懸垂","weight":null,"reps":12}]}`)
	if w.Code != 200 {
		t.Fatalf("put 自重 #2: %d (%s)", w.Code, w.Body.String())
	}
	todayDate := decode(t, w)["date"].(string)
	hist := decode(t, c.req("GET", "/api/presets/"+urlSeg("自重")+"/history", ""))["snapshots"].([]any)
	if len(hist) != 2 {
		t.Fatalf("history should be 2 after second PUT: %v", hist)
	}
	if hist[0].(map[string]any)["date"] != todayDate {
		t.Fatalf("history not date-desc: %v", hist)
	}

	// GET current preset -> latest snapshot
	w = c.req("GET", "/api/presets/"+urlSeg("自重"), "")
	m = decode(t, w)
	if m["date"] != todayDate || len(m["exercises"].([]any)) != 1 {
		t.Fatalf("get current 自重: %v", m)
	}

	// GET historical snapshot by date
	w = c.req("GET", "/api/presets/"+urlSeg("自重")+"/2026-07-30", "")
	if w.Code != 200 || len(decode(t, w)["exercises"].([]any)) != 2 {
		t.Fatalf("get 自重 2026-07-30: %d (%s)", w.Code, w.Body.String())
	}
	if w = c.req("GET", "/api/presets/"+urlSeg("自重")+"/2000-01-01", ""); w.Code != 404 {
		t.Fatalf("missing dated snapshot: %d", w.Code)
	}

	// PATCH in-place: existing -> updated, same snapshot date
	w = c.req("PATCH", "/api/presets/"+urlSeg("自重")+"/exercises/"+urlSeg("懸垂"), `{"reps":15}`)
	m = decode(t, w)
	if w.Code != 200 || m["action"] != "updated" || m["preset"] != "自重" || m["presetDate"] != todayDate {
		t.Fatalf("patch updated: %d (%v)", w.Code, m)
	}
	// PATCH missing -> added
	w = c.req("PATCH", "/api/presets/"+urlSeg("自重")+"/exercises/"+urlSeg("新種目"), `{"reps":20}`)
	if decode(t, w)["action"] != "added" {
		t.Fatalf("patch added: %s", w.Body.String())
	}
	// history unchanged by PATCH (still 2)
	if h := decode(t, c.req("GET", "/api/presets/"+urlSeg("自重")+"/history", ""))["snapshots"].([]any); len(h) != 2 {
		t.Fatalf("PATCH must not add history: %v", h)
	}
	// PATCH unknown preset -> 404
	if w = c.req("PATCH", "/api/presets/"+urlSeg("未登録")+"/exercises/"+urlSeg("懸垂"), `{"reps":1}`); w.Code != 404 {
		t.Fatalf("patch unknown preset: %d", w.Code)
	}

	// list with summaries
	list := decode(t, c.req("GET", "/api/presets", ""))["presets"].([]any)
	if len(list) != 2 {
		t.Fatalf("preset list len: %v", list)
	}
	p0 := list[0].(map[string]any)
	if p0["name"] != "自重" || p0["sortOrder"].(float64) != 0 || p0["latestDate"] != todayDate {
		t.Fatalf("preset[0] summary: %v", p0)
	}

	// reorder
	w = c.req("PUT", "/api/presets:reorder", `{"order":["FW","自重"]}`)
	rp := decode(t, w)["presets"].([]any)
	if w.Code != 200 || rp[0].(map[string]any)["name"] != "FW" || rp[0].(map[string]any)["sortOrder"].(float64) != 0 {
		t.Fatalf("reorder: %d (%s)", w.Code, w.Body.String())
	}

	// delete preset -> snapshots gone
	if w = c.req("DELETE", "/api/presets/"+urlSeg("自重"), ""); w.Code != 204 {
		t.Fatalf("delete 自重: %d", w.Code)
	}
	if w = c.req("GET", "/api/presets/"+urlSeg("自重")+"/history", ""); w.Code != 404 {
		t.Fatalf("history after delete should 404 (preset gone): %d", w.Code)
	}
	if w = c.req("DELETE", "/api/presets/"+urlSeg("自重"), ""); w.Code != 404 {
		t.Fatalf("re-delete: %d", w.Code)
	}
}

func TestLegacyRoutineView(t *testing.T) {
	c := newClient(t)

	// no presets -> 404
	w := c.req("GET", "/api/routine", "")
	if w.Code != 404 || decode(t, w)["error"].(map[string]any)["message"] != "no routine" {
		t.Fatalf("empty routine: %d (%s)", w.Code, w.Body.String())
	}

	// PUT /api/routine is retired -> 400
	w = c.req("PUT", "/api/routine", `{"exercises":[{"name":"懸垂","reps":10}]}`)
	if w.Code != 400 || decode(t, w)["error"].(map[string]any)["message"] != "use PUT /api/presets/{name}" {
		t.Fatalf("PUT /api/routine should 400: %d (%s)", w.Code, w.Body.String())
	}

	// build two presets via /api/presets
	c.req("POST", "/api/presets", `{"name":"自重","sortOrder":0}`)
	c.req("POST", "/api/presets", `{"name":"FW","sortOrder":1}`)
	c.req("PUT", "/api/presets/"+urlSeg("自重"), `{"date":"2026-07-30","exercises":[
		{"name":"懸垂","weight":null,"reps":10},{"name":"ディップス","weight":null,"reps":30}]}`)
	c.req("PUT", "/api/presets/"+urlSeg("FW"), `{"date":"2026-07-28","exercises":[
		{"name":"ヒップストラスト","weight":40,"reps":40,"sets":1}]}`)

	// combined view: preset order 自重 -> FW, date = max latest date
	w = c.req("GET", "/api/routine", "")
	m := decode(t, w)
	if m["date"] != "2026-07-30" {
		t.Fatalf("combined date = %v, want max 2026-07-30", m["date"])
	}
	pr := m["presets"].([]any)
	if len(pr) != 2 || pr[0] != "自重" || pr[1] != "FW" {
		t.Fatalf("combined presets order: %v", pr)
	}
	exs := m["exercises"].([]any)
	if len(exs) != 3 || exs[0].(map[string]any)["name"] != "懸垂" || exs[2].(map[string]any)["name"] != "ヒップストラスト" {
		t.Fatalf("combined exercises order: %v", exs)
	}

	// combined history: all presets, summed per date
	snaps := decode(t, c.req("GET", "/api/routine/history", ""))["snapshots"].([]any)
	if len(snaps) != 2 {
		t.Fatalf("combined history: %v", snaps)
	}

	// combined by date
	w = c.req("GET", "/api/routine/2026-07-30", "")
	if w.Code != 200 || len(decode(t, w)["exercises"].([]any)) != 2 {
		t.Fatalf("routine by date: %d (%s)", w.Code, w.Body.String())
	}
	if w = c.req("GET", "/api/routine/2000-01-01", ""); w.Code != 404 {
		t.Fatalf("routine by missing date: %d", w.Code)
	}

	// cross-preset PATCH: 懸垂 only in 自重 -> in-place update there
	w = c.req("PATCH", "/api/routine/exercises/"+urlSeg("懸垂"), `{"reps":12}`)
	m = decode(t, w)
	if w.Code != 200 || m["action"] != "updated" || m["preset"] != "自重" || m["presetDate"] != "2026-07-30" {
		t.Fatalf("cross patch: %d (%v)", w.Code, m)
	}
	// unknown exercise -> 404 (this path never adds)
	if w = c.req("PATCH", "/api/routine/exercises/"+urlSeg("どこにもない"), `{"reps":1}`); w.Code != 404 {
		t.Fatalf("cross patch unknown: %d (%s)", w.Code, w.Body.String())
	}
}

func TestProfileAndExercises(t *testing.T) {
	c := newClient(t)

	w := c.req("GET", "/api/profile", "")
	m := decode(t, w)
	if m["bodyweightKg"].(float64) != 86 || m["heightCm"].(float64) != 170 || m["maxHrEst"].(float64) != 180 {
		t.Fatalf("profile defaults: %v", m)
	}

	w = c.req("PUT", "/api/profile", `{"bodyweightKg":85}`)
	if decode(t, w)["bodyweightKg"].(float64) != 85 {
		t.Fatalf("profile update (%s)", w.Body.String())
	}

	c.req("POST", "/api/strength-sessions", `{"date":"2026-09-01","exercises":[{"name":"懸垂","reps":10}]}`)
	w = c.req("GET", "/api/exercises", "")
	if list := decode(t, w)["exercises"].([]any); len(list) != 1 || list[0] != "懸垂" {
		t.Fatalf("exercises: %v", list)
	}
}

func TestCalendarVolumeSummary(t *testing.T) {
	c := newClient(t)
	c.req("POST", "/api/strength-sessions", `{"date":"2026-09-05","notes":"FWのみ","exercises":[{"name":"ヒップストラスト","weight":40,"reps":40,"sets":1}]}`)
	c.req("POST", "/api/strength-sessions", `{"date":"2026-09-06","notes":"自重のみ","exercises":[{"name":"懸垂","weight":null,"reps":10}]}`)
	c.req("POST", "/api/spin-sessions", `{"date":"2026-09-06","duration_minutes":30,"avg_heart_rate":140,"max_heart_rate":170}`)

	// calendar
	w := c.req("GET", "/api/calendar?month=2026-09", "")
	if w.Code != 200 {
		t.Fatalf("calendar code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	days := m["days"].([]any)
	if m["month"] != "2026-09" || len(days) != 2 {
		t.Fatalf("calendar days: %v", m)
	}
	d0 := days[0].(map[string]any)
	st0 := d0["strength"].(map[string]any)
	if st0["kind"] != "fw_only" || st0["volumeLoad"].(float64) != 1600 {
		t.Fatalf("calendar day0 strength: %v", st0)
	}
	d1 := days[1].(map[string]any)
	if d1["strength"].(map[string]any)["kind"] != "bodyweight_only" {
		t.Fatalf("calendar day1 kind: %v", d1["strength"])
	}
	if d1["spin"].(map[string]any)["durationMinutes"].(float64) != 30 {
		t.Fatalf("calendar day1 spin: %v", d1["spin"])
	}

	// bad month
	if w = c.req("GET", "/api/calendar?month=2026-9", ""); w.Code != 400 {
		t.Fatalf("bad month code = %d", w.Code)
	}

	// volume session
	w = c.req("GET", "/api/volume?granularity=session", "")
	m = decode(t, w)
	if m["granularity"] != "session" || len(m["points"].([]any)) != 2 {
		t.Fatalf("volume session: %v", m)
	}
	p0 := m["points"].([]any)[0].(map[string]any)
	if p0["date"] != "2026-09-05" || p0["volumeLoad"].(float64) != 1600 {
		t.Fatalf("volume point0: %v", p0)
	}

	// volume week
	w = c.req("GET", "/api/volume?granularity=week", "")
	m = decode(t, w)
	pts := m["points"].([]any)
	if m["granularity"] != "week" || len(pts) != 1 {
		t.Fatalf("volume week: %v", m)
	}
	if pts[0].(map[string]any)["weekStart"] != "2026-08-31" { // Mon of the week containing 2026-09-05/06
		t.Fatalf("weekStart: %v", pts[0])
	}
	if pts[0].(map[string]any)["sessionCount"].(float64) != 2 {
		t.Fatalf("sessionCount: %v", pts[0])
	}

	// volume per exercise
	w = c.req("GET", "/api/volume?exercise="+urlSeg("ヒップストラスト"), "")
	m = decode(t, w)
	if m["exercise"] != "ヒップストラスト" || len(m["points"].([]any)) != 1 {
		t.Fatalf("volume exercise: %v", m)
	}

	// summary
	w = c.req("GET", "/api/summary?days=3650", "")
	m = decode(t, w)
	if int(m["totalStrengthSessions"].(float64)) != 2 || int(m["totalSpinSessions"].(float64)) != 1 {
		t.Fatalf("summary totals: %v", m)
	}

	// weekly summary
	w = c.req("GET", "/api/summary/weekly", "")
	if w.Code != 200 {
		t.Fatalf("weekly code = %d (%s)", w.Code, w.Body.String())
	}
	m = decode(t, w)
	if _, ok := m["evaluation"].([]any); !ok {
		t.Fatalf("weekly evaluation missing: %v", m)
	}

	// load report
	w = c.req("GET", "/api/load-report", "")
	if w.Code != 200 {
		t.Fatalf("load-report code = %d (%s)", w.Code, w.Body.String())
	}
	m = decode(t, w)
	if _, ok := m["acwr"]; !ok {
		t.Fatalf("load-report missing acwr: %v", m)
	}
	if _, ok := m["zone"].(string); !ok {
		t.Fatalf("load-report missing zone: %v", m)
	}

	// sessions/last
	w = c.req("GET", "/api/sessions/last", "")
	m = decode(t, w)
	if m["strength"].(map[string]any)["date"] != "2026-09-06" {
		t.Fatalf("sessions/last strength: %v", m["strength"])
	}
	if m["spin"].(map[string]any)["date"] != "2026-09-06" {
		t.Fatalf("sessions/last spin: %v", m["spin"])
	}
}

func TestWeeklySummaryAlwaysObjects(t *testing.T) {
	c := newClient(t)

	// Empty DB: the week has neither spin nor strength data. Both blocks
	// must still be objects (never null) with zero counts and [] sessions.
	w := c.req("GET", "/api/summary/weekly", "")
	if w.Code != 200 {
		t.Fatalf("weekly code = %d (%s)", w.Code, w.Body.String())
	}
	m := decode(t, w)
	sum := m["summary"].(map[string]any)

	for _, key := range []string{"spinSessions", "strengthSessions"} {
		block, ok := sum[key].(map[string]any)
		if !ok {
			t.Fatalf("summary.%s is not an object: %#v", key, sum[key])
		}
		if block["totalSessions"].(float64) != 0 {
			t.Errorf("summary.%s.totalSessions = %v, want 0", key, block["totalSessions"])
		}
		sessions, ok := block["sessions"].([]any)
		if !ok || len(sessions) != 0 {
			t.Errorf("summary.%s.sessions must be an empty array, got %#v", key, block["sessions"])
		}
	}

	// Raw check: the JSON must not contain a null for these keys.
	if strings.Contains(w.Body.String(), `"spinSessions":null`) ||
		strings.Contains(w.Body.String(), `"strengthSessions":null`) {
		t.Fatalf("weekly summary still emits null blocks: %s", w.Body.String())
	}
}

func TestErrorEnvelopeShape(t *testing.T) {
	c := newClient(t)
	w := c.req("GET", "/api/strength-sessions/2099-01-01", "")
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", w.Header().Get("Content-Type"))
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Error.Code != "not_found" || env.Error.Message == "" {
		t.Fatalf("envelope: %+v", env)
	}
}

// helpers

func itoa(v int64) string {
	return strings.TrimSpace(string(mustJSON(v)))
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// urlSeg percent-encodes a path segment (so the router's PathValue decodes
// it back). A minimal encoder is enough for the Japanese names used here.
func urlSeg(s string) string {
	var b bytes.Buffer
	for _, r := range []byte(s) {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '~' {
			b.WriteByte(r)
		} else {
			b.WriteByte('%')
			const hex = "0123456789ABCDEF"
			b.WriteByte(hex[r>>4])
			b.WriteByte(hex[r&0xf])
		}
	}
	return b.String()
}
