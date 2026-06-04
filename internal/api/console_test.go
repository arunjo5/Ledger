package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ledger/internal/ledger"
)

func TestHealthEndpoint(t *testing.T) {
	h := reset(t)
	rec := doReq(h, "GET", "/health", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" || body["database"] != "up" {
		t.Fatalf("body = %v", body)
	}
}

func TestListTransactionsEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")
	doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))
	doReq(h, "POST", "/transactions", transferBody(a, b, 200), key("k2"))

	rec := doReq(h, "GET", "/transactions", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var resp struct {
		Transactions []ledger.TxSummary `json:"transactions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Transactions) != 2 {
		t.Fatalf("len = %d, want 2", len(resp.Transactions))
	}
	if resp.Transactions[0].Lines != 2 {
		t.Fatalf("lines = %d, want 2", resp.Transactions[0].Lines)
	}
}

func TestOverviewEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")
	doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))

	rec := doReq(h, "GET", "/overview", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var ov ledger.Overview
	json.Unmarshal(rec.Body.Bytes(), &ov)
	if ov.Total != 1 || ov.Committed != 1 {
		t.Fatalf("overview = %+v", ov)
	}
}

func TestResetEndpoint(t *testing.T) {
	h := reset(t)
	rec := doReq(h, "POST", "/demo/reset", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}

	ovRec := doReq(h, "GET", "/overview", "", nil)
	var ov ledger.Overview
	json.Unmarshal(ovRec.Body.Bytes(), &ov)
	if ov.Total != 6 || ov.Committed != 5 || ov.Failed != 1 {
		t.Fatalf("after reset: %+v", ov)
	}
}
