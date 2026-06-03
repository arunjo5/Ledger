package api

import (
	"context"
	"net/http"
	"time"
)

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "unavailable", "database": "down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "database": "up"})
}

func (h *handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.Reconcile(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "reconcile failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handlers) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.store.Overview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not load overview")
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

func (h *handlers) reset(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Seed(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not reset demo data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true})
}
