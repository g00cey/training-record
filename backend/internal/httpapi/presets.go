package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"training-record/internal/service"
)

// ---------------------------------------------------------------------------
// shared body helpers
// ---------------------------------------------------------------------------

type routineExerciseBody struct {
	Name   *string  `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   *int     `json:"reps"`
	Sets   *int     `json:"sets"`
}

// parseRoutineExercises decodes and validates a routine/preset "exercises"
// array.
func parseRoutineExercises(raw json.RawMessage) ([]service.RoutineExerciseInput, error) {
	if len(raw) == 0 || isJSONNull(raw) {
		return nil, unprocessable("exercises is required")
	}
	var rows []routineExerciseBody
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, badRequest("exercises must be an array of objects")
	}
	out := make([]service.RoutineExerciseInput, 0, len(rows))
	for i, row := range rows {
		if row.Name == nil || strings.TrimSpace(*row.Name) == "" {
			return nil, unprocessable("exercises[%d].name is required", i)
		}
		if row.Reps == nil {
			return nil, unprocessable("exercises[%d].reps is required", i)
		}
		if *row.Reps < 0 {
			return nil, unprocessable("exercises[%d].reps must be >= 0", i)
		}
		sets := 1
		if row.Sets != nil && *row.Sets >= 1 {
			sets = *row.Sets
		}
		out = append(out, service.RoutineExerciseInput{
			Name: strings.TrimSpace(*row.Name), Weight: row.Weight, Reps: *row.Reps, Sets: sets,
		})
	}
	return out, nil
}

func pathName(r *http.Request, key string) (string, error) {
	v := strings.TrimSpace(r.PathValue(key)) // ServeMux already percent-decodes
	if v == "" {
		return "", badRequest("%s is required", key)
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// /api/presets
// ---------------------------------------------------------------------------

func (h *Handlers) listPresets(w http.ResponseWriter, r *http.Request) error {
	infos, err := h.svc.ListPresets()
	if err != nil {
		return err
	}
	out := make([]presetInfoDTO, 0, len(infos))
	for _, p := range infos {
		out = append(out, toPresetInfoDTO(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": out})
	return nil
}

type createPresetBody struct {
	Name      *string `json:"name"`
	SortOrder *int    `json:"sortOrder"`
}

func (h *Handlers) createPreset(w http.ResponseWriter, r *http.Request) error {
	var b createPresetBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if b.Name == nil || strings.TrimSpace(*b.Name) == "" {
		return unprocessable("name is required")
	}
	p, err := h.svc.CreatePreset(strings.TrimSpace(*b.Name), b.SortOrder)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"name": p.Name, "sortOrder": p.SortOrder})
	return nil
}

type reorderPresetsBody struct {
	Order []string `json:"order"`
}

func (h *Handlers) reorderPresets(w http.ResponseWriter, r *http.Request) error {
	var b reorderPresetsBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if len(b.Order) == 0 {
		return badRequest("order must be a non-empty array")
	}
	infos, err := h.svc.ReorderPresets(b.Order)
	if err != nil {
		return err
	}
	out := make([]presetInfoDTO, 0, len(infos))
	for _, p := range infos {
		out = append(out, toPresetInfoDTO(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": out})
	return nil
}

func (h *Handlers) getPreset(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	s, err := h.svc.GetPreset(name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toPresetSnapshotDTO(s))
	return nil
}

func (h *Handlers) presetHistory(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	entries, err := h.svc.PresetHistory(name)
	if err != nil {
		return err
	}
	snaps := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		snaps = append(snaps, map[string]any{"date": e.Date, "exerciseCount": e.ExerciseCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
	return nil
}

func (h *Handlers) getPresetSnapshot(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	s, err := h.svc.GetPresetSnapshot(name, date)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toPresetSnapshotDTO(s))
	return nil
}

type putPresetBody struct {
	Date      *string         `json:"date"`
	Exercises json.RawMessage `json:"exercises"`
}

func (h *Handlers) putPreset(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	var b putPresetBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	date := ""
	if b.Date != nil && *b.Date != "" {
		if !validDate(*b.Date) {
			return badRequest("date must be YYYY-MM-DD")
		}
		date = *b.Date
	}
	exs, err := parseRoutineExercises(b.Exercises)
	if err != nil {
		return err
	}
	s, err := h.svc.ReplacePreset(name, date, exs)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toPresetSnapshotDTO(s))
	return nil
}

func (h *Handlers) deletePreset(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	if err := h.svc.DeletePreset(name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handlers) patchPresetExercise(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	exerciseIDStr, err := pathName(r, "exerciseId")
	if err != nil {
		return err
	}
	exerciseID, err := strconv.ParseInt(exerciseIDStr, 10, 64)
	if err != nil {
		return badRequest("exerciseId must be an integer")
	}
	weight, reps, sets, err := parseExercisePatch(r)
	if err != nil {
		return err
	}
	action, presetDate, err := h.svc.PatchPresetExercise(name, exerciseID, weight, reps, sets)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action":     action,
		"exerciseId": exerciseID,
		"preset":     name,
		"presetDate": presetDate,
	})
	return nil
}

func (h *Handlers) addPresetExercise(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	var b routineExerciseBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if b.Name == nil || strings.TrimSpace(*b.Name) == "" {
		return unprocessable("name is required")
	}
	if b.Reps == nil {
		return unprocessable("reps is required")
	}
	if *b.Reps < 0 {
		return unprocessable("reps must be >= 0")
	}
	sets := 1
	if b.Sets != nil && *b.Sets >= 1 {
		sets = *b.Sets
	}
	exerciseID, presetDate, err := h.svc.AddPresetExercise(name, strings.TrimSpace(*b.Name), b.Weight, b.Reps, &sets)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"action":     "added",
		"exerciseId": exerciseID,
		"exercise":   strings.TrimSpace(*b.Name),
		"preset":     name,
		"presetDate": presetDate,
	})
	return nil
}

// parseExercisePatch reads a { weight?, reps?, sets? } partial-update body.
func parseExercisePatch(r *http.Request) (weight *float64, reps, sets *int, err error) {
	m, err := readObject(r)
	if err != nil {
		return nil, nil, nil, err
	}
	if weight, _, err = mFloat(m, "weight"); err != nil {
		return nil, nil, nil, err
	}
	if reps, _, err = mInt(m, "reps"); err != nil {
		return nil, nil, nil, err
	}
	if sets, _, err = mInt(m, "sets"); err != nil {
		return nil, nil, nil, err
	}
	if weight == nil && reps == nil && sets == nil {
		return nil, nil, nil, badRequest("provide at least one of weight, reps, sets")
	}
	if reps != nil && *reps < 0 {
		return nil, nil, nil, unprocessable("reps must be >= 0")
	}
	if sets != nil && *sets < 1 {
		return nil, nil, nil, unprocessable("sets must be >= 1")
	}
	return weight, reps, sets, nil
}

// ---------------------------------------------------------------------------
// legacy /api/routine (compat read view)
// ---------------------------------------------------------------------------

func (h *Handlers) getRoutine(w http.ResponseWriter, r *http.Request) error {
	c, err := h.svc.CurrentRoutine()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toCombinedRoutineDTO(c))
	return nil
}

func (h *Handlers) routineHistory(w http.ResponseWriter, r *http.Request) error {
	entries, err := h.svc.RoutineHistory()
	if err != nil {
		return err
	}
	snaps := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		snaps = append(snaps, map[string]any{"date": e.Date, "exerciseCount": e.ExerciseCount})
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snaps})
	return nil
}

func (h *Handlers) getRoutineByDate(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	c, err := h.svc.RoutineByDate(date)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toCombinedRoutineDTO(c))
	return nil
}

func (h *Handlers) putRoutine(w http.ResponseWriter, r *http.Request) error {
	return badRequest("use PUT /api/presets/{name}")
}

func (h *Handlers) patchRoutineExercise(w http.ResponseWriter, r *http.Request) error {
	name, err := pathName(r, "name")
	if err != nil {
		return err
	}
	weight, reps, sets, err := parseExercisePatch(r)
	if err != nil {
		return err
	}
	preset, presetDate, err := h.svc.CrossPatchRoutineExercise(name, weight, reps, sets)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action":     "updated",
		"exercise":   name,
		"preset":     preset,
		"presetDate": presetDate,
	})
	return nil
}
