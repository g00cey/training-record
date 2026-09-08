package httpapi

import (
	"net/http"
	"regexp"

	"training-record/internal/service"
)

var monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)

func (h *Handlers) calendar(w http.ResponseWriter, r *http.Request) error {
	month := r.URL.Query().Get("month")
	if !monthRe.MatchString(month) {
		return badRequest("month must be YYYY-MM")
	}
	res, err := h.svc.Calendar(month)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) volume(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	gran := q.Get("granularity")
	if gran == "" {
		gran = "session"
	}
	if gran != "session" && gran != "week" {
		return badRequest("granularity must be session or week")
	}
	from, err := qDate(r, "from")
	if err != nil {
		return err
	}
	to, err := qDate(r, "to")
	if err != nil {
		return err
	}
	res, err := h.svc.Volume(service.VolumeParams{
		Granularity: gran,
		From:        from,
		To:          to,
		Exercise:    q.Get("exercise"),
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) summary(w http.ResponseWriter, r *http.Request) error {
	days, err := qInt(r, "days", 30)
	if err != nil {
		return err
	}
	if days <= 0 {
		return badRequest("days must be > 0")
	}
	res, err := h.svc.Summary(days)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) weeklySummary(w http.ResponseWriter, r *http.Request) error {
	res, err := h.svc.WeeklySummary()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

func (h *Handlers) loadReport(w http.ResponseWriter, r *http.Request) error {
	res, err := h.svc.LoadReport()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}
