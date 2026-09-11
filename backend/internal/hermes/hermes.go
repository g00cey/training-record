// Package hermes is a thin HTTP client for the Hermes Agent image-extraction
// API used by POST /api/spin-extract (Phase 7). Hermes Agent receives a spin
// bike workout screenshot and returns structured exercise data as JSON.
//
// 契約（docs/hermes-integration.md Phase 7 の依頼文と同じ）:
//
//	POST {HERMES_API_URL}
//	Authorization: Bearer {HERMES_API_KEY}
//	Content-Type: application/json
//	{"imageBase64":"...","mimeType":"image/jpeg"}
//	→ 200 {"durationMinutes":52,"avgHeartRate":130,...,"hrZones":{...},...}
package hermes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultTimeout bounds a single Hermes extraction call. The nginx route for
// /bff/spin-extract allows 120s, so keep the client below it.
const DefaultTimeout = 90 * time.Second

// ErrorKind categorises a client failure for transport-layer mapping.
type ErrorKind int

const (
	ErrUnreachable ErrorKind = iota // network / DNS failure
	ErrTimeout                      // exceeded the client timeout
	ErrHTTP                         // non-200 response from Hermes
	ErrDecode                       // response not parseable as the contract JSON
)

// Error is a typed hermes client error.
type Error struct {
	Kind   ErrorKind
	Msg    string
	Status int // HTTP status from Hermes (ErrHTTP only)
}

func (e *Error) Error() string { return e.Msg }

// Config configures a Client.
type Config struct {
	URL     string        // full endpoint URL (e.g. http://192.168.1.50:9000/extract-spin)
	APIKey  string        // static bearer key (optional: empty = no auth header)
	Timeout time.Duration // 0 = DefaultTimeout
}

// Client calls the Hermes extraction API.
type Client struct {
	cfg Config
	hc  *http.Client
}

// New builds a Client. A zero URL yields a client that always fails; callers
// gate the feature on config instead (nil client = unconfigured).
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	return &Client{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// Result is the parsed (pre-normalisation) extraction payload from Hermes.
// Every field is optional: unreadable items arrive as null per the contract.
type Result struct {
	DurationMinutes *int
	AvgHeartRate    *int
	MaxHeartRate    *int
	DistanceKm      *float64
	HrZones         map[string]string // zone label (as returned) -> "mm:ss"
	FreeNotes       string
	UncertainFields []string
}

type extractRequest struct {
	ImageBase64 string `json:"imageBase64"`
	MimeType    string `json:"mimeType"`
}

// Extract sends the image to Hermes and parses the response. The caller is
// responsible for normalisation (Normalize).
func (c *Client) Extract(ctx context.Context, image []byte, mimeType string) (*Result, error) {
	body, err := json.Marshal(extractRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(image),
		MimeType:    mimeType,
	})
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
			return nil, &Error{ErrTimeout, "hermes extraction timed out", 0}
		}
		return nil, &Error{ErrUnreachable, "hermes unreachable: " + err.Error(), 0}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readErrBody(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("http %d", resp.StatusCode)
		}
		return nil, &Error{ErrHTTP, "hermes error: " + msg, resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &Error{ErrDecode, "read hermes response: " + err.Error(), 0}
	}
	return parsePayload(data)
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

// parsePayload decodes the Hermes response into a Result. It accepts both
// camelCase and snake_case keys, and defensively tolerates LLM-style
// responses wrapped in markdown code fences or surrounded by prose (the JSON
// object is taken from the first '{' to the last '}').
func parsePayload(data []byte) (*Result, error) {
	obj := extractJSONObject(data)
	if obj == nil {
		return nil, &Error{ErrDecode, "hermes response contains no JSON object", 0}
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(obj, &m); err != nil {
		return nil, &Error{ErrDecode, "hermes response is not a JSON object: " + err.Error(), 0}
	}

	res := &Result{HrZones: map[string]string{}}
	if v, ok := pick(m, "durationMinutes", "duration_minutes"); ok {
		res.DurationMinutes = asInt(v)
	}
	if v, ok := pick(m, "avgHeartRate", "avg_heart_rate"); ok {
		res.AvgHeartRate = asInt(v)
	}
	if v, ok := pick(m, "maxHeartRate", "max_heart_rate"); ok {
		res.MaxHeartRate = asInt(v)
	}
	if v, ok := pick(m, "distanceKm", "distance_km"); ok {
		res.DistanceKm = asFloat(v)
	}
	if v, ok := pick(m, "freeNotes", "free_notes", "notes"); ok {
		res.FreeNotes = asString(v)
	}
	if v, ok := pick(m, "uncertainFields", "uncertain_fields"); ok {
		var fields []string
		if err := json.Unmarshal(v, &fields); err == nil {
			res.UncertainFields = fields
		}
	}
	if v, ok := pick(m, "hrZones", "hr_zones", "zones"); ok {
		var zones map[string]json.RawMessage
		if err := json.Unmarshal(v, &zones); err == nil {
			for k, raw := range zones {
				if s := asString(raw); s != "" {
					res.HrZones[k] = s
				}
			}
		}
	}
	return res, nil
}

// extractJSONObject returns the byte slice from the first '{' to the last '}',
// or nil when the payload contains no object. Handles ```json fences.
func extractJSONObject(data []byte) []byte {
	s := strings.TrimSpace(string(data))
	if i := strings.Index(s, "```"); i >= 0 {
		// コードフェンス内を優先的に探す（フェンス外に JSON 無い prose を捨てる）
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

func pick(m map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func isNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

func asInt(raw json.RawMessage) *int {
	if isNull(raw) {
		return nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil
	}
	return &n
}

func asFloat(raw json.RawMessage) *float64 {
	if isNull(raw) {
		return nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil
	}
	return &f
}

func asString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
