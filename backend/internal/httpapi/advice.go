package httpapi

import "net/http"

// advice handles GET /api/advice — the Phase 6 injury-prevention advice
// aggregate. The service returns a camelCase-tagged view model directly.
func (h *Handlers) advice(w http.ResponseWriter, r *http.Request) error {
	rep, err := h.svc.Advice()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, rep)
	return nil
}
