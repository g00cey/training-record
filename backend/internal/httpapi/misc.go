package httpapi

import (
	"net/http"
)

func (h *Handlers) exercises(w http.ResponseWriter, r *http.Request) error {
	names, err := h.svc.ExerciseNames()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"exercises": names})
	return nil
}

func (h *Handlers) getProfile(w http.ResponseWriter, r *http.Request) error {
	p, err := h.svc.GetProfile()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toProfileDTO(p))
	return nil
}

func (h *Handlers) putProfile(w http.ResponseWriter, r *http.Request) error {
	m, err := readObject(r)
	if err != nil {
		return err
	}
	bw, _, err := mFloat(m, "bodyweightKg", "bodyweight_kg")
	if err != nil {
		return err
	}
	ht, _, err := mFloat(m, "heightCm", "height_cm")
	if err != nil {
		return err
	}
	maxHR, _, err := mInt(m, "maxHrEst", "max_hr_est")
	if err != nil {
		return err
	}
	if bw != nil && *bw <= 0 {
		return unprocessable("bodyweightKg must be > 0")
	}
	if ht != nil && *ht <= 0 {
		return unprocessable("heightCm must be > 0")
	}
	if maxHR != nil && *maxHR <= 0 {
		return unprocessable("maxHrEst must be > 0")
	}
	p, err := h.svc.UpdateProfile(bw, ht, maxHR)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toProfileDTO(p))
	return nil
}

func (h *Handlers) lastSessions(w http.ResponseWriter, r *http.Request) error {
	st, sp, err := h.svc.LastSessions()
	if err != nil {
		return err
	}
	resp := map[string]any{"strength": nil, "spin": nil}
	if st != nil {
		resp["strength"] = toStrengthDTO(st)
	}
	if sp != nil {
		resp["spin"] = toSpinDTO(sp)
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handlers) history(w http.ResponseWriter, r *http.Request) error {
	limit, err := qInt(r, "limit", 10)
	if err != nil {
		return err
	}
	if limit < 0 {
		return badRequest("limit must be >= 0")
	}
	st, sp, err := h.svc.History(limit)
	if err != nil {
		return err
	}
	stItems := make([]strengthSessionDTO, 0, len(st))
	for i := range st {
		stItems = append(stItems, toStrengthDTO(&st[i]))
	}
	spItems := make([]spinSessionDTO, 0, len(sp))
	for i := range sp {
		spItems = append(spItems, toSpinDTO(&sp[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"strengthSessions": stItems,
		"spinSessions":     spItems,
	})
	return nil
}
