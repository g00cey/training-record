package httpapi

import (
	"net/http"

	"training-record/internal/service"
)

func (h *Handlers) listSpin(w http.ResponseWriter, r *http.Request) error {
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
	if limit < 0 || offset < 0 {
		return badRequest("limit/offset must be >= 0")
	}
	sessions, total, err := h.svc.ListSpin(from, to, limit, offset)
	if err != nil {
		return err
	}
	items := make([]spinSessionDTO, 0, len(sessions))
	for i := range sessions {
		items = append(items, toSpinDTO(&sessions[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
	return nil
}

func (h *Handlers) getSpin(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	s, err := h.svc.GetSpin(date)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toSpinDTO(s))
	return nil
}

// parseSpinInput reads a full POST/PUT spin body (snake_case per api.md,
// camelCase also accepted).
func parseSpinInput(r *http.Request, requireDate bool) (service.SpinInput, error) {
	m, err := readObject(r)
	if err != nil {
		return service.SpinInput{}, err
	}
	var in service.SpinInput

	if requireDate {
		date, present, e := mString(m, "date")
		if e != nil {
			return in, e
		}
		if !present || !validDate(date) {
			return in, badRequest("date must be YYYY-MM-DD")
		}
		in.Date = date
	}

	dur, present, e := mInt(m, "duration_minutes", "durationMinutes")
	if e != nil {
		return in, e
	}
	if !present || dur == nil {
		return in, unprocessable("duration_minutes is required")
	}
	if *dur < 0 {
		return in, unprocessable("duration_minutes must be >= 0")
	}
	in.DurationMinutes = *dur

	if v, _, e := mInt(m, "avg_heart_rate", "avgHeartRate"); e != nil {
		return in, e
	} else {
		in.AvgHeartRate = v
	}
	if v, _, e := mInt(m, "max_heart_rate", "maxHeartRate"); e != nil {
		return in, e
	} else {
		in.MaxHeartRate = v
	}
	if v, _, e := mInt(m, "rpe"); e != nil {
		return in, e
	} else if v != nil {
		if *v < 1 || *v > 10 {
			return in, unprocessable("rpe must be between 1 and 10")
		}
		in.RPE = v
	}
	if v, _, e := mFloat(m, "distance_km", "distanceKm"); e != nil {
		return in, e
	} else {
		in.DistanceKm = v
	}
	if v, present, e := mString(m, "notes"); e != nil {
		return in, e
	} else if present {
		in.Notes = v
	}
	return in, nil
}

func (h *Handlers) createSpin(w http.ResponseWriter, r *http.Request) error {
	in, err := parseSpinInput(r, true)
	if err != nil {
		return err
	}
	s, err := h.svc.CreateSpin(in)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, toSpinDTO(s))
	return nil
}

func (h *Handlers) putSpin(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	in, err := parseSpinInput(r, false)
	if err != nil {
		return err
	}
	s, created, err := h.svc.ReplaceSpin(date, in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, toSpinDTO(s))
	return nil
}

func (h *Handlers) patchSpin(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	m, err := readObject(r)
	if err != nil {
		return err
	}
	var p service.SpinPatch

	if v, present, e := mInt(m, "duration_minutes", "durationMinutes"); e != nil {
		return e
	} else if present {
		if v == nil {
			return unprocessable("duration_minutes cannot be null")
		}
		if *v < 0 {
			return unprocessable("duration_minutes must be >= 0")
		}
		p.DurationMinutes = v
	}
	if v, present, e := mInt(m, "avg_heart_rate", "avgHeartRate"); e != nil {
		return e
	} else if present {
		p.AvgHeartRate, p.AvgHRSet = v, true
	}
	if v, present, e := mInt(m, "max_heart_rate", "maxHeartRate"); e != nil {
		return e
	} else if present {
		p.MaxHeartRate, p.MaxHRSet = v, true
	}
	if v, present, e := mInt(m, "rpe"); e != nil {
		return e
	} else if present {
		if v != nil && (*v < 1 || *v > 10) {
			return unprocessable("rpe must be between 1 and 10")
		}
		p.RPE, p.RPESet = v, true
	}
	if v, present, e := mFloat(m, "distance_km", "distanceKm"); e != nil {
		return e
	} else if present {
		p.DistanceKm, p.DistanceSet = v, true
	}
	if v, present, e := mString(m, "notes"); e != nil {
		return e
	} else if present {
		p.Notes = &v
	}

	s, err := h.svc.PatchSpin(date, p)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, toSpinDTO(s))
	return nil
}

func (h *Handlers) deleteSpin(w http.ResponseWriter, r *http.Request) error {
	date, err := pathDate(r, "date")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteSpin(date); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
