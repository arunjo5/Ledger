package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"ledger/internal/ledger"
)

const defaultEntriesLimit = 50

func (h *handlers) accountBalance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "account id must be a uuid")
		return
	}

	asOf, err := parseAsOf(r.URL.Query().Get("as_of"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_as_of", err.Error())
		return
	}

	if !h.accountExists(w, r, id) {
		return
	}

	balances, err := h.store.Balance(r.Context(), id, asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not compute balance")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": id,
		"as_of":      asOf,
		"balances":   balances,
	})
}

func (h *handlers) accountEntries(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "account id must be a uuid")
		return
	}

	asOf, err := parseAsOf(r.URL.Query().Get("as_of"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_as_of", err.Error())
		return
	}

	if !h.accountExists(w, r, id) {
		return
	}

	entries, err := h.store.AccountEntries(r.Context(), id, asOf, parseLimit(r.URL.Query().Get("limit")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not load entries")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": id,
		"entries":    entries,
	})
}

func (h *handlers) accountExists(w http.ResponseWriter, r *http.Request, id string) bool {
	_, err := h.store.GetAccount(r.Context(), id)
	if err == nil {
		return true
	}
	if errors.Is(err, ledger.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "account not found")
		return false
	}
	writeError(w, http.StatusInternalServerError, "internal", "could not load account")
	return false
}

func parseAsOf(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, errors.New("as_of must be an RFC3339 timestamp")
	}
	return &t, nil
}

func parseLimit(s string) int {
	if s == "" {
		return defaultEntriesLimit
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultEntriesLimit
	}
	if n > 500 {
		return 500
	}
	return n
}
