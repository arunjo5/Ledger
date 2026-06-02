package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiError{Error: msg, Code: code})
}

// decodeJSON reads a single optional JSON object; an empty body leaves dst at its zero value.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	if dec.More() {
		return errors.New("body must contain a single json object")
	}
	return nil
}

// decodeJSONWithHash decodes a required JSON body and returns the SHA-256 of the raw bytes.
func decodeJSONWithHash(w http.ResponseWriter, r *http.Request, dst any) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("request body is required")
		}
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("body must contain a single json object")
	}

	sum := sha256.Sum256(body)
	return sum[:], nil
}
