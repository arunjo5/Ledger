package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ledger/internal/ledger"
)

func TestReconcileEndpointHealthy(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")
	doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))

	rec := doReq(h, "POST", "/admin/reconcile", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}

	var result ledger.ReconcileResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("reconcile not ok: %+v", result.Checks)
	}
}
