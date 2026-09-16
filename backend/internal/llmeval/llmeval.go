// Package llmeval is a thin HTTP client for the LLM training-evaluation
// server (llm/server.py's POST /evaluate-training). It gathers no data
// itself; callers (internal/service) build the HistoryPayload from the
// existing session/profile stores and load-metric calculations, then call
// this client once per evaluation period ("biweekly" / "bimonthly").
//
// 契約:
//
//	POST {LLM_EVAL_API_URL}
//	Authorization: Bearer {LLM_EVAL_API_KEY}
//	Content-Type: application/json
//	{"periodType":"biweekly","asOf":"...","profile":{...},"loadMetrics":{...},
//	 "strengthSessions":[...],"spinSessions":[...]}
//	→ 200 {"summary":"...","strengths":[...],"concerns":[...],"suggestions":[...]}
package llmeval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultTimeout bounds a single evaluation call. The upstream vision/text
// LLM provider (OpenCode Go) can be slow, so keep the same budget as
// internal/hermes.
const DefaultTimeout = 90 * time.Second

// ErrorKind categorises a client failure for transport-layer mapping.
type ErrorKind int

const (
	ErrUnreachable ErrorKind = iota // network / DNS failure
	ErrTimeout                      // exceeded the client timeout
	ErrHTTP                         // non-200 response from the LLM server
	ErrDecode                       // response not parseable as the contract JSON
)

// Error is a typed llmeval client error.
type Error struct {
	Kind   ErrorKind
	Msg    string
	Status int // HTTP status from the LLM server (ErrHTTP only)
}

func (e *Error) Error() string { return e.Msg }

// Config configures a Client.
type Config struct {
	URL     string        // full endpoint URL (e.g. http://192.168.1.50:9000/evaluate-training)
	APIKey  string        // static bearer key (optional: empty = no auth header)
	Timeout time.Duration // 0 = DefaultTimeout
}

// Client calls the LLM training-evaluation API.
type Client struct {
	cfg Config
	hc  *http.Client
}

// New builds a Client. A zero URL yields a client that always fails;
// callers gate the feature on config instead (nil client = unconfigured).
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	return &Client{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// ProfilePayload is the profile block of the outbound history payload.
type ProfilePayload struct {
	BodyweightKg float64 `json:"bodyweightKg"`
	HeightCm     float64 `json:"heightCm"`
	MaxHrEst     int     `json:"maxHrEst"`
}

// LoadMetricsPayload is the acute/chronic load snapshot sent for context
// (same figures as GET /api/load-report's acwr/zone/acute7d/chronic28d).
type LoadMetricsPayload struct {
	ACWR             float64 `json:"acwr"`
	Zone             string  `json:"zone"`
	Acute7dTotal     float64 `json:"acute7dTotal"`
	ChronicWeeklyAvg float64 `json:"chronicWeeklyAvg"`
}

// StrengthExercisePayload is one exercise row inside StrengthSessionPayload.
type StrengthExercisePayload struct {
	Name   string   `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   int      `json:"reps"`
	Sets   int      `json:"sets"`
}

// StrengthSessionPayload is one strength session inside HistoryPayload.
type StrengthSessionPayload struct {
	Date      string                    `json:"date"`
	Notes     string                    `json:"notes"`
	Exercises []StrengthExercisePayload `json:"exercises"`
}

// SpinSessionPayload is one spin session inside HistoryPayload.
type SpinSessionPayload struct {
	Date            string   `json:"date"`
	DurationMinutes int      `json:"durationMinutes"`
	AvgHeartRate    *int     `json:"avgHeartRate"`
	MaxHeartRate    *int     `json:"maxHeartRate"`
	RPE             *int     `json:"rpe"`
	DistanceKm      *float64 `json:"distanceKm"`
}

// HistoryPayload is what backend sends to llm/'s POST /evaluate-training.
// PeriodType selects the prompt framing on the llm/ side ("biweekly" =
// 直近2週間, "bimonthly" = 直近8週間・約2ヶ月); Strength/Spin are pre-filtered
// by the caller to [PeriodFrom, PeriodTo].
type HistoryPayload struct {
	PeriodType  string                   `json:"periodType"`
	AsOf        string                   `json:"asOf"`
	PeriodFrom  string                   `json:"periodFrom"`
	PeriodTo    string                   `json:"periodTo"`
	Profile     ProfilePayload           `json:"profile"`
	LoadMetrics LoadMetricsPayload       `json:"loadMetrics"`
	Strength    []StrengthSessionPayload `json:"strengthSessions"`
	Spin        []SpinSessionPayload     `json:"spinSessions"`
}

// Result is the parsed (pre-normalisation) evaluation from the LLM server.
// Raw holds the full response body verbatim, kept only for audit
// persistence (never exposed via the public API).
type Result struct {
	Summary     string
	Strengths   []string
	Concerns    []string
	Suggestions []string
	Raw         string
}

// Evaluate sends the history payload and parses the response. The caller is
// responsible for normalisation (Normalize).
func (c *Client) Evaluate(ctx context.Context, payload HistoryPayload) (*Result, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &Error{ErrDecode, "encode request: " + err.Error(), 0}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{ErrUnreachable, "build request: " + err.Error(), 0}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, &Error{ErrTimeout, "training evaluation timed out", 0}
		}
		return nil, &Error{ErrUnreachable, "llm evaluation server unreachable: " + err.Error(), 0}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readErrBody(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("http %d", resp.StatusCode)
		}
		return nil, &Error{ErrHTTP, "llm evaluation error: " + msg, resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &Error{ErrDecode, "read llm evaluation response: " + err.Error(), 0}
	}
	res, err := parseResult(data)
	if err != nil {
		return nil, err
	}
	res.Raw = string(data)
	return res, nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if os.IsTimeout(err) {
		return true
	}
	var ne interface{ Timeout() bool }
	return errors.As(err, &ne) && ne.Timeout()
}

func readErrBody(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, 4096))
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// parseResult decodes the LLM server response into a Result. It defensively
// tolerates LLM-style responses wrapped in markdown code fences or
// surrounded by prose (the JSON object is taken from the first '{' to the
// last '}'), matching internal/hermes's parsing posture.
func parseResult(data []byte) (*Result, error) {
	obj := extractJSONObject(data)
	if obj == nil {
		return nil, &Error{ErrDecode, "llm evaluation response contains no JSON object", 0}
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(obj, &m); err != nil {
		return nil, &Error{ErrDecode, "llm evaluation response is not a JSON object: " + err.Error(), 0}
	}

	res := &Result{}
	if v, ok := m["summary"]; ok {
		res.Summary = asString(v)
	}
	if v, ok := m["strengths"]; ok {
		res.Strengths = asStringSlice(v)
	}
	if v, ok := m["concerns"]; ok {
		res.Concerns = asStringSlice(v)
	}
	if v, ok := m["suggestions"]; ok {
		res.Suggestions = asStringSlice(v)
	}
	return res, nil
}

// extractJSONObject returns the byte slice from the first '{' to the last
// '}', or nil when the payload contains no object. Handles ```json fences.
func extractJSONObject(data []byte) []byte {
	s := strings.TrimSpace(string(data))
	if i := strings.Index(s, "```"); i >= 0 {
		s = strings.ReplaceAll(s, "```json", "```")
		if j := strings.Index(s, "```"); j >= 0 {
			s = s[j+3:]
		}
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	first := strings.Index(s, "{")
	last := strings.LastIndex(s, "}")
	if first < 0 || last <= first {
		return nil
	}
	return []byte(s[first : last+1])
}

func asString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

func asStringSlice(raw json.RawMessage) []string {
	var xs []string
	if err := json.Unmarshal(raw, &xs); err != nil {
		return nil
	}
	return xs
}
