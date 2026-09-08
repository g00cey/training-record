package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"training-record/internal/domain"
	"training-record/internal/service"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validDate(s string) bool {
	if !dateRe.MatchString(s) {
		return false
	}
	_, err := time.Parse(domain.DateLayout, s)
	return err == nil
}

// pathDate reads and validates a {date} path segment.
func pathDate(r *http.Request, name string) (string, error) {
	v := r.PathValue(name)
	if !validDate(v) {
		return "", badRequest("%s must be YYYY-MM-DD", name)
	}
	return v, nil
}

// pathInt64 reads and parses an integer path segment.
func pathInt64(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, badRequest("%s must be an integer", name)
	}
	return v, nil
}

// readObject decodes the request body into a key->raw map. An empty body
// yields an empty map. A non-object body is a bad_request.
func readObject(r *http.Request) (map[string]json.RawMessage, error) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, badRequest("cannot read request body")
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, badRequest("request body must be a JSON object")
	}
	return m, nil
}

func pick(m map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

// mString reads a string field. present is false when the key is absent.
func mString(m map[string]json.RawMessage, keys ...string) (val string, present bool, err error) {
	raw, ok := pick(m, keys...)
	if !ok {
		return "", false, nil
	}
	if isJSONNull(raw) {
		return "", true, nil
	}
	if e := json.Unmarshal(raw, &val); e != nil {
		return "", true, badRequest("%s must be a string", keys[0])
	}
	return val, true, nil
}

// mInt reads an int field. Returns (nil, true) for an explicit null.
func mInt(m map[string]json.RawMessage, keys ...string) (val *int, present bool, err error) {
	raw, ok := pick(m, keys...)
	if !ok {
		return nil, false, nil
	}
	if isJSONNull(raw) {
		return nil, true, nil
	}
	var n int
	if e := json.Unmarshal(raw, &n); e != nil {
		return nil, true, badRequest("%s must be an integer", keys[0])
	}
	return &n, true, nil
}

// mFloat reads a float field. Returns (nil, true) for an explicit null.
func mFloat(m map[string]json.RawMessage, keys ...string) (val *float64, present bool, err error) {
	raw, ok := pick(m, keys...)
	if !ok {
		return nil, false, nil
	}
	if isJSONNull(raw) {
		return nil, true, nil
	}
	var f float64
	if e := json.Unmarshal(raw, &f); e != nil {
		return nil, true, badRequest("%s must be a number", keys[0])
	}
	return &f, true, nil
}

type exerciseBody struct {
	Name   *string  `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   *int     `json:"reps"`
	Sets   *int     `json:"sets"`
	Notes  *string  `json:"notes"`
}

// parseExercises decodes and validates an "exercises" array.
func parseExercises(raw json.RawMessage) ([]service.ExerciseInput, error) {
	var rows []exerciseBody
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, badRequest("exercises must be an array of objects")
	}
	out := make([]service.ExerciseInput, 0, len(rows))
	for i, b := range rows {
		if b.Name == nil || strings.TrimSpace(*b.Name) == "" {
			return nil, unprocessable("exercises[%d].name is required", i)
		}
		if b.Reps == nil {
			return nil, unprocessable("exercises[%d].reps is required", i)
		}
		if *b.Reps < 0 {
			return nil, unprocessable("exercises[%d].reps must be >= 0", i)
		}
		sets := 1
		if b.Sets != nil {
			sets = *b.Sets
			if sets < 1 {
				sets = 1
			}
		}
		notes := ""
		if b.Notes != nil {
			notes = *b.Notes
		}
		out = append(out, service.ExerciseInput{
			Name:   strings.TrimSpace(*b.Name),
			Weight: b.Weight,
			Reps:   *b.Reps,
			Sets:   sets,
			Notes:  notes,
		})
	}
	return out, nil
}

// decodeBody strictly decodes the whole request body into v.
func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid JSON body")
	}
	return nil
}

// exercisesOrEmpty parses an optional "exercises" raw value, treating an
// absent/null value as an empty list.
func exercisesOrEmpty(raw json.RawMessage) ([]service.ExerciseInput, error) {
	if len(raw) == 0 || isJSONNull(raw) {
		return []service.ExerciseInput{}, nil
	}
	return parseExercises(raw)
}

func nowRFC3339() string { return domain.NowRFC3339() }

// qInt reads an integer query parameter with a default.
func qInt(r *http.Request, key string, def int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, badRequest("%s must be an integer", key)
	}
	return n, nil
}

// qDate reads an optional date query parameter (returns "" when absent).
func qDate(r *http.Request, key string) (string, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return "", nil
	}
	if !validDate(v) {
		return "", badRequest("%s must be YYYY-MM-DD", key)
	}
	return v, nil
}
