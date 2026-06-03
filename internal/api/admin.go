package api

import "net/http"

func (h *handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.Reconcile(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "reconcile failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
