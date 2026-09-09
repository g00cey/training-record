// Package httpapi is the HTTP transport layer: routing (Go 1.22 ServeMux
// patterns), request decoding/validation, response encoding (camelCase
// JSON), the bearer-token auth middleware and the uniform error envelope.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"training-record/internal/apperr"
	"training-record/internal/service"
)

// Handlers holds the service dependency for all HTTP handlers.
type Handlers struct {
	svc *service.Service
}

// NewRouter builds the fully-wired HTTP handler (routes + auth middleware).
func NewRouter(svc *service.Service, apiKey string) http.Handler {
	h := &Handlers{svc: svc}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handle(h.health))

	// strength sessions
	mux.HandleFunc("GET /api/strength-sessions", handle(h.listStrength))
	mux.HandleFunc("POST /api/strength-sessions", handle(h.createStrength))
	mux.HandleFunc("GET /api/strength-sessions/{date}", handle(h.getStrength))
	mux.HandleFunc("PUT /api/strength-sessions/{date}", handle(h.putStrength))
	mux.HandleFunc("PATCH /api/strength-sessions/{date}", handle(h.patchStrength))
	mux.HandleFunc("DELETE /api/strength-sessions/{date}", handle(h.deleteStrength))
	mux.HandleFunc("POST /api/strength-sessions/{date}/exercises", handle(h.appendExercises))
	mux.HandleFunc("PUT /api/strength-sessions/{date}/exercises:reorder", handle(h.reorderExercises))
	mux.HandleFunc("PUT /api/strength-sessions/{date}/exercises/{id}", handle(h.putExercise))
	mux.HandleFunc("DELETE /api/strength-sessions/{date}/exercises/{id}", handle(h.deleteExercise))

	// spin sessions
	mux.HandleFunc("GET /api/spin-sessions", handle(h.listSpin))
	mux.HandleFunc("POST /api/spin-sessions", handle(h.createSpin))
	mux.HandleFunc("GET /api/spin-sessions/{date}", handle(h.getSpin))
	mux.HandleFunc("PUT /api/spin-sessions/{date}", handle(h.putSpin))
	mux.HandleFunc("PATCH /api/spin-sessions/{date}", handle(h.patchSpin))
	mux.HandleFunc("DELETE /api/spin-sessions/{date}", handle(h.deleteSpin))

	// presets (named menus)
	mux.HandleFunc("GET /api/presets", handle(h.listPresets))
	mux.HandleFunc("POST /api/presets", handle(h.createPreset))
	mux.HandleFunc("PUT /api/presets:reorder", handle(h.reorderPresets))
	mux.HandleFunc("GET /api/presets/{name}", handle(h.getPreset))
	mux.HandleFunc("PUT /api/presets/{name}", handle(h.putPreset))
	mux.HandleFunc("DELETE /api/presets/{name}", handle(h.deletePreset))
	mux.HandleFunc("GET /api/presets/{name}/history", handle(h.presetHistory))
	mux.HandleFunc("GET /api/presets/{name}/{date}", handle(h.getPresetSnapshot))
	mux.HandleFunc("PATCH /api/presets/{name}/exercises/{exerciseId}", handle(h.patchPresetExercise))
	mux.HandleFunc("POST /api/presets/{name}/exercises", handle(h.addPresetExercise))

	// routine (legacy compat: merged read view over all presets)
	mux.HandleFunc("GET /api/routine", handle(h.getRoutine))
	mux.HandleFunc("PUT /api/routine", handle(h.putRoutine))
	mux.HandleFunc("GET /api/routine/history", handle(h.routineHistory))
	mux.HandleFunc("GET /api/routine/{date}", handle(h.getRoutineByDate))
	mux.HandleFunc("PATCH /api/routine/exercises/{name}", handle(h.patchRoutineExercise))

	// master data / profile
	mux.HandleFunc("GET /api/exercises", handle(h.exercises))
	mux.HandleFunc("GET /api/profile", handle(h.getProfile))
	mux.HandleFunc("PUT /api/profile", handle(h.putProfile))

	// views / analytics
	mux.HandleFunc("GET /api/calendar", handle(h.calendar))
	mux.HandleFunc("GET /api/volume", handle(h.volume))
	mux.HandleFunc("GET /api/summary", handle(h.summary))
	mux.HandleFunc("GET /api/summary/weekly", handle(h.weeklySummary))
	mux.HandleFunc("GET /api/load-report", handle(h.loadReport))
	mux.HandleFunc("GET /api/sessions/last", handle(h.lastSessions))
	mux.HandleFunc("GET /api/history", handle(h.history))

	// JSON 404 for anything else
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, apperr.NotFoundf("no route for %s %s", r.Method, r.URL.Path))
	})

	return authMiddleware(apiKey, mux)
}

// --- response helpers -----------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: encode response: %v", err)
	}
}

type errEnvelope struct {
	Error errPayload `json:"error"`
}
type errPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// httpError is a transport-level error with an explicit status + code.
type httpError struct {
	status  int
	code    string
	message string
}

func (e *httpError) Error() string { return e.message }

func badRequest(format string, a ...any) *httpError {
	return &httpError{http.StatusBadRequest, "bad_request", fmt.Sprintf(format, a...)}
}
func unprocessable(format string, a ...any) *httpError {
	return &httpError{http.StatusUnprocessableEntity, "unprocessable", fmt.Sprintf(format, a...)}
}
func unauthorized(msg string) *httpError {
	return &httpError{http.StatusUnauthorized, "unauthorized", msg}
}

func writeErr(w http.ResponseWriter, err error) {
	var he *httpError
	var ae *apperr.E
	switch {
	case errors.As(err, &he):
		writeJSON(w, he.status, errEnvelope{errPayload{he.code, he.message}})
	case errors.As(err, &ae):
		status, code := mapAppErr(ae.Kind)
		writeJSON(w, status, errEnvelope{errPayload{code, ae.Msg}})
	default:
		log.Printf("httpapi: internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError,
			errEnvelope{errPayload{"internal", "internal server error"}})
	}
}

func mapAppErr(k apperr.Kind) (status int, code string) {
	switch k {
	case apperr.NotFound:
		return http.StatusNotFound, "not_found"
	case apperr.Conflict:
		return http.StatusConflict, "conflict"
	case apperr.Unprocessable:
		return http.StatusUnprocessableEntity, "unprocessable"
	default:
		return http.StatusBadRequest, "bad_request"
	}
}

// handle adapts an error-returning handler to http.HandlerFunc.
func handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			writeErr(w, err)
		}
	}
}
