package httpapi

import (
	"encoding/json"
	"net/http"

	"training-record/internal/service"
)

func (h *Handlers) health(w http.ResponseWriter, r *http.Request) error {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   nowRFC3339(),
	})
	return nil
}

func (h *Handlers) listStrength(w http.ResponseWriter, r *http.Request) error {
	from, err := qDate(r, "from")
	if err != nil {
		return err
	}
	to, err := qDate(r, "to")
	if err != nil {
		return err
	}
	limit, err := qInt(r, "limit", 30)
	if err != nil {
		return err
	}
	offset, err := qInt(r, "offset", 0)
	if err != nil {
		return err
	}
	if limit < 0 {
		return badRequest("limit must be >= 0")
	}
	if offset < 0 {
		return badRequest("offset must be >= 0")
	}
	sessions, total, err := h.svc.ListStrength(from, to, limit, offset)
	if err != nil {
		return err
	}
	items := make([]strengthSessionDTO, 0, len(sessions))
	for i := range sessions {
		items = append(items, toStrengthDTO(&sessions[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
	return nil
}

func (h *Handlers) getStrength(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	s, err := h.svc.GetStrength(date)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toStrengthDTO(s))
	return nil
}

type strengthBody struct {
	Date      string          `json:"date"`
	Notes     *string         `json:"notes"`
	Exercises json.RawMessage `json:"exercises"`
}

func (h *Handlers) createStrength(w http.ResponseWriter, r *http.Request) error {
	var b strengthBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if !validDate(b.Date) {
		return badRequest("date must be YYYY-MM-DD")
	}
	exs, err := exercisesOrEmpty(b.Exercises)
	if err != nil {
		return err
	}
	notes := ""
	if b.Notes != nil {
		notes = *b.Notes
	}
	s, err := h.svc.CreateStrength(b.Date, notes, exs)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, toStrengthDTO(s))
	return nil
}

func (h *Handlers) putStrength(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	var b strengthBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	exs, err := exercisesOrEmpty(b.Exercises)
	if err != nil {
		return err
	}
	notes := ""
	if b.Notes != nil {
		notes = *b.Notes
	}
	s, created, err := h.svc.ReplaceStrength(date, notes, exs)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, toStrengthDTO(s))
	return nil
}

func (h *Handlers) patchStrength(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	m, err := readObject(r)
	if err != nil {
		return err
	}
	notes, present, err := mString(m, "notes")
	if err != nil {
		return err
	}
	if !present {
		return badRequest("notes is required")
	}
	s, err := h.svc.PatchStrengthNotes(date, notes)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toStrengthDTO(s))
	return nil
}

func (h *Handlers) deleteStrength(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteStrength(date); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type appendBody struct {
	Exercises json.RawMessage `json:"exercises"`
	Notes     *string         `json:"notes"`
}

func (h *Handlers) appendExercises(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	var b appendBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	exs, err := exercisesOrEmpty(b.Exercises)
	if err != nil {
		return err
	}
	notes := ""
	hasNotes := b.Notes != nil
	if hasNotes {
		notes = *b.Notes
	}
	s, err := h.svc.AppendExercises(date, exs, notes, hasNotes)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toStrengthDTO(s))
	return nil
}

type exercisePutBody struct {
	Name   *string  `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   *int     `json:"reps"`
	Sets   *int     `json:"sets"`
	Notes  *string  `json:"notes"`
}

func (h *Handlers) putExercise(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		return err
	}
	var b exercisePutBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if b.Name == nil || *b.Name == "" {
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
	notes := ""
	if b.Notes != nil {
		notes = *b.Notes
	}
	s, err := h.svc.UpdateExercise(date, id, service.ExerciseInput{
		Name: *b.Name, Weight: b.Weight, Reps: *b.Reps, Sets: sets, Notes: notes,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toStrengthDTO(s))
	return nil
}

func (h *Handlers) deleteExercise(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteExercise(date, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type reorderBody struct {
	OrderedIds []int64 `json:"orderedIds"`
}

func (h *Handlers) reorderExercises(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	var b reorderBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}
	if len(b.OrderedIds) == 0 {
		return badRequest("orderedIds must be a non-empty array")
	}
	s, err := h.svc.ReorderExercises(date, b.OrderedIds)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toStrengthDTO(s))
	return nil
}
