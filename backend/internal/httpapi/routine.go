package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"training-record/internal/service"
)

func (h *Handlers) getRoutine(w http.ResponseWriter, r *http.Request) error {
	s, err := h.svc.CurrentRoutine()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toRoutineDTO(s))
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
	s, err := h.svc.RoutineByDate(date)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toRoutineDTO(s))
	return nil
}

type routineBody struct {
	Date      *string         `json:"date"`
	Exercises json.RawMessage `json:"exercises"`
}

type routineExerciseBody struct {
	Name   *string  `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   *int     `json:"reps"`
	Sets   *int     `json:"sets"`
}

func (h *Handlers) putRoutine(w http.ResponseWriter, r *http.Request) error {
	var b routineBody
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
	if len(b.Exercises) == 0 || isJSONNull(b.Exercises) {
		return unprocessable("exercises is required")
	}
	var rows []routineExerciseBody
	if err := json.Unmarshal(b.Exercises, &rows); err != nil {
		return badRequest("exercises must be an array of objects")
	}
	in := make([]service.RoutineExerciseInput, 0, len(rows))
	for i, row := range rows {
		if row.Name == nil || strings.TrimSpace(*row.Name) == "" {
			return unprocessable("exercises[%d].name is required", i)
		}
		if row.Reps == nil {
			return unprocessable("exercises[%d].reps is required", i)
		}
		if *row.Reps < 0 {
			return unprocessable("exercises[%d].reps must be >= 0", i)
		}
		sets := 1
		if row.Sets != nil && *row.Sets >= 1 {
			sets = *row.Sets
		}
		in = append(in, service.RoutineExerciseInput{
			Name: strings.TrimSpace(*row.Name), Weight: row.Weight, Reps: *row.Reps, Sets: sets,
		})
	}
	s, err := h.svc.ReplaceRoutine(date, in)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toRoutineDTO(s))
	return nil
}

func (h *Handlers) patchRoutineExercise(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name") // ServeMux has already percent-decoded this
	if strings.TrimSpace(name) == "" {
		return badRequest("exercise name is required")
	}
	m, err := readObject(r)
	if err != nil {
		return err
	}
	weight, _, err := mFloat(m, "weight")
	if err != nil {
		return err
	}
	reps, _, err := mInt(m, "reps")
	if err != nil {
		return err
	}
	sets, _, err := mInt(m, "sets")
	if err != nil {
		return err
	}
	if weight == nil && reps == nil && sets == nil {
		return badRequest("provide at least one of weight, reps, sets")
	}
	if reps != nil && *reps < 0 {
		return unprocessable("reps must be >= 0")
	}
	if sets != nil && *sets < 1 {
		return unprocessable("sets must be >= 1")
	}
	action, routineDate, err := h.svc.PatchRoutineExercise(name, weight, reps, sets)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"action":      action,
		"exercise":    name,
		"routineDate": routineDate,
	})
	return nil
}
