package api

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"ledger/internal/ledger"
)

const (
	maxEntriesPerTx = 1000
	maxEntryAmount  = int64(1_000_000_000_000_000)
)

type createTransactionRequest struct {
	Description string           `json:"description"`
	Entries     []entryInputJSON `json:"entries"`
}

type entryInputJSON struct {
	AccountID string `json:"account_id"`
	Currency  string `json:"currency"`
	Amount    int64  `json:"amount"`
}

func (h *handlers) createTransaction(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if err := validateIdempotencyKey(key); err != nil {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
		return
	}

	var req createTransactionRequest
	hash, err := decodeJSONWithHash(w, r, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	entries, err := validateEntries(req.Entries)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_transaction", err.Error())
		return
	}

	txn, err := h.store.CreateTransaction(r.Context(), ledger.NewTransaction{
		IdempotencyKey: key,
		RequestHash:    hash,
		Description:    strings.TrimSpace(req.Description),
		Entries:        entries,
	})
	if err != nil {
		writeTransactionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txn)
}

func (h *handlers) getTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "transaction id must be a uuid")
		return
	}

	txn, err := h.store.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, ledger.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "transaction not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "could not get transaction")
		return
	}
	writeJSON(w, http.StatusOK, txn)
}

func (h *handlers) reverseTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isUUID(id) {
		writeError(w, http.StatusBadRequest, "invalid_id", "transaction id must be a uuid")
		return
	}

	key := r.Header.Get("Idempotency-Key")
	if err := validateIdempotencyKey(key); err != nil {
		writeError(w, http.StatusBadRequest, "missing_idempotency_key", err.Error())
		return
	}

	hash := sha256.Sum256([]byte("reverse:" + id))
	txn, err := h.store.ReverseTransaction(r.Context(), id, key, hash[:])
	if err != nil {
		writeTransactionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, txn)
}

func writeTransactionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ledger.ErrUnknownAccount):
		writeError(w, http.StatusBadRequest, "unknown_account", err.Error())
	case errors.Is(err, ledger.ErrNotBalanced):
		writeError(w, http.StatusBadRequest, "not_balanced", err.Error())
	case errors.Is(err, ledger.ErrIdempotencyKeyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", err.Error())
	case errors.Is(err, ledger.ErrNotCommitted):
		writeError(w, http.StatusConflict, "not_committed", "only a committed transaction can be reversed")
	case errors.Is(err, ledger.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "transaction not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "could not process transaction")
	}
}

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

func validateIdempotencyKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("Idempotency-Key header is required")
	}
	if len(key) > 255 {
		return errors.New("Idempotency-Key must be at most 255 characters")
	}
	return nil
}

func validateEntries(in []entryInputJSON) ([]ledger.EntryInput, error) {
	if len(in) < 2 {
		return nil, errors.New("a transaction needs at least two entries")
	}
	if len(in) > maxEntriesPerTx {
		return nil, fmt.Errorf("a transaction may have at most %d entries", maxEntriesPerTx)
	}

	entries := make([]ledger.EntryInput, 0, len(in))
	sums := map[string]int64{}
	for i, e := range in {
		if !isUUID(e.AccountID) {
			return nil, fmt.Errorf("entry %d: account_id must be a uuid", i)
		}
		if !currencyRe.MatchString(e.Currency) {
			return nil, fmt.Errorf("entry %d: currency must be three uppercase letters", i)
		}
		if e.Amount == 0 {
			return nil, fmt.Errorf("entry %d: amount must not be zero", i)
		}
		if e.Amount > maxEntryAmount || e.Amount < -maxEntryAmount {
			return nil, fmt.Errorf("entry %d: amount is out of range", i)
		}
		sums[e.Currency] += e.Amount
		entries = append(entries, ledger.EntryInput{
			AccountID: e.AccountID,
			Currency:  e.Currency,
			Amount:    e.Amount,
		})
	}

	var off []string
	for cur, sum := range sums {
		if sum != 0 {
			off = append(off, fmt.Sprintf("%s %+d", cur, sum))
		}
	}
	if len(off) > 0 {
		sort.Strings(off)
		return nil, fmt.Errorf("entries are not balanced per currency: %s", strings.Join(off, ", "))
	}
	return entries, nil
}
