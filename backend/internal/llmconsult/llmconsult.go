// Package llmconsult is a thin HTTP client for the LLM training-consultation
// server (llm/server.py's POST /consult-training).
package llmconsult

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

// DefaultTimeout bounds a single consultation call.
const DefaultTimeout = 90 * time.Second

// ErrorKind categorises a client failure for transport-layer mapping.
type ErrorKind int

const (
	ErrUnreachable ErrorKind = iota // network / DNS failure
	ErrTimeout                      // exceeded the client timeout
	ErrHTTP                         // non-200 response from the LLM server
	ErrDecode                       // response not parseable as JSON
)

// Error is a typed llmconsult client error.
type Error struct {
	Kind   ErrorKind
	Msg    string
	Status int // HTTP status from the LLM server (ErrHTTP only)
}

func (e *Error) Error() string { return e.Msg }

// Config configures a Client.
type Config struct {
	URL     string        // full endpoint URL (e.g. http://spin-extraction:8646/consult-training)
	APIKey  string        // static bearer key (optional)
	Timeout time.Duration // 0 = DefaultTimeout
}

// Client calls the LLM training-consultation API.
type Client struct {
	cfg Config
	hc  *http.Client
}

// New builds a Client.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}
	return &Client{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// MessagePayload is one conversation turn sent to the consultation API.
type MessagePayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RequestPayload is the JSON sent to POST /consult-training.
type RequestPayload struct {
	Goal                string           `json:"goal"`
	Context             any              `json:"context"`
	Messages            []MessagePayload `json:"messages"`
	RequestMenuProposal bool             `json:"requestMenuProposal"`
}

type responsePayload struct {
	Reply string `json:"reply"`
	Error string `json:"error,omitempty"`
}

// Consult sends the conversation payload and returns the assistant's reply.
func (c *Client) Consult(ctx context.Context, payload RequestPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", &Error{ErrDecode, "encode request: " + err.Error(), 0}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return "", &Error{ErrUnreachable, "build request: " + err.Error(), 0}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		if isTimeout(err) {
			return "", &Error{ErrTimeout, "training consultation timed out", 0}
		}
		return "", &Error{ErrUnreachable, "llm consultation server unreachable: " + err.Error(), 0}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg := readErrBody(resp.Body)
		if msg == "" {
			msg = fmt.Sprintf("http %d", resp.StatusCode)
		}
		return "", &Error{ErrHTTP, "llm consultation error: " + msg, resp.StatusCode}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", &Error{ErrDecode, "read llm consultation response: " + err.Error(), 0}
	}

	var res responsePayload
	if err := json.Unmarshal(data, &res); err != nil {
		return "", &Error{ErrDecode, "decode llm consultation response: " + err.Error(), 0}
	}
	return strings.TrimSpace(res.Reply), nil
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
