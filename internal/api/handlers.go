package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"ledger/internal/ledger"
)

type handlers struct {
	store *ledger.Store
}

type createAccountRequest struct {
	Label *string `json:"label"`
}

func (h *handlers) createAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	label, err := normalizeLabel(req.Label)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_label", err.Error())
		return
	}

	acc, err := h.store.CreateAccount(r.Context(), label)
	if err != nil {
		if errors.Is(err, ledger.ErrDuplicateLabel) {
			writeError(w, http.StatusConflict, "label_taken", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "could not create account")
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (h *handlers) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.store.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not list accounts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (h *handlers) getAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "account id must be a uuid")
		return
	}

	acc, err := h.store.GetAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, ledger.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "account not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "could not get account")
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isUUID(s string) bool {
	return uuidRe.MatchString(s)
}

func normalizeLabel(label *string) (*string, error) {
	if label == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*label)
	if trimmed == "" {
		return nil, errors.New("label must not be empty")
	}
	if len(trimmed) > 100 {
		return nil, errors.New("label must be at most 100 characters")
	}
	return &trimmed, nil
}
