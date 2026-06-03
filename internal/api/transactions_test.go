package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ledger/internal/ledger"
)

func doReq(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func makeAccount(t *testing.T, label string) string {
	t.Helper()
	acc, err := testStore.CreateAccount(context.Background(), &label)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acc.ID
}

func transferBody(from, to string, amount int64) string {
	return fmt.Sprintf(`{"description":"transfer","entries":[
		{"account_id":%q,"currency":"USD","amount":%d},
		{"account_id":%q,"currency":"USD","amount":%d}]}`, from, -amount, to, amount)
}

func key(v string) map[string]string { return map[string]string{"Idempotency-Key": v} }

func TestCreateTransactionEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	rec := doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body = %s", rec.Code, rec.Body.String())
	}

	var txn ledger.Transaction
	if err := json.Unmarshal(rec.Body.Bytes(), &txn); err != nil {
		t.Fatal(err)
	}
	if txn.Status != ledger.StatusCommitted {
		t.Fatalf("status = %s, want committed", txn.Status)
	}
	if len(txn.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(txn.Entries))
	}
}

func TestCreateTransactionMissingKey(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	rec := doReq(h, "POST", "/transactions", transferBody(a, b, 100), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestCreateTransactionUnbalanced(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	body := fmt.Sprintf(`{"entries":[
		{"account_id":%q,"currency":"USD","amount":-100},
		{"account_id":%q,"currency":"USD","amount":50}]}`, a, b)
	rec := doReq(h, "POST", "/transactions", body, key("k1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestCreateTransactionUnknownAccount(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	missing := "00000000-0000-0000-0000-000000000000"

	rec := doReq(h, "POST", "/transactions", transferBody(a, missing, 100), key("k1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestGetTransactionEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	created := doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))
	var txn ledger.Transaction
	json.Unmarshal(created.Body.Bytes(), &txn)

	rec := doReq(h, "GET", "/transactions/"+txn.ID, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}

func TestBalanceEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")
	doReq(h, "POST", "/transactions", transferBody(a, b, 250), key("k1"))

	rec := doReq(h, "GET", "/accounts/"+a+"/balance", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var resp struct {
		Balances []ledger.Balance `json:"balances"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Balances) != 1 || resp.Balances[0].Amount != -250 {
		t.Fatalf("balances = %+v, want USD -250", resp.Balances)
	}
}

func TestEntriesEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")
	doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))

	rec := doReq(h, "GET", "/accounts/"+a+"/entries", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var resp struct {
		Entries []ledger.Entry `json:"entries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(resp.Entries))
	}
}

func TestReverseEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	created := doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k1"))
	var txn ledger.Transaction
	json.Unmarshal(created.Body.Bytes(), &txn)

	rec := doReq(h, "POST", "/transactions/"+txn.ID+"/reverse", "", key("rev1"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var reversal ledger.Transaction
	json.Unmarshal(rec.Body.Bytes(), &reversal)
	if reversal.ReversesTransactionID == nil || *reversal.ReversesTransactionID != txn.ID {
		t.Fatalf("reverses = %v, want %s", reversal.ReversesTransactionID, txn.ID)
	}
}

func TestIdempotentReplayEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	first := doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("dup"))
	if first.Code != http.StatusCreated {
		t.Fatalf("first code = %d, body = %s", first.Code, first.Body.String())
	}

	second := doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("dup"))
	if second.Code != http.StatusCreated {
		t.Fatalf("replay code = %d, want 201", second.Code)
	}
	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatal("replay should set the Idempotent-Replayed header")
	}

	var t1, t2 ledger.Transaction
	json.Unmarshal(first.Body.Bytes(), &t1)
	json.Unmarshal(second.Body.Bytes(), &t2)
	if t1.ID != t2.ID {
		t.Fatalf("ids differ: %s vs %s", t1.ID, t2.ID)
	}

	var count int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from transactions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("transactions = %d, want 1", count)
	}
}

func TestIdempotencyConflictEndpoint(t *testing.T) {
	h := reset(t)
	a := makeAccount(t, "a")
	b := makeAccount(t, "b")

	doReq(h, "POST", "/transactions", transferBody(a, b, 100), key("k"))
	rec := doReq(h, "POST", "/transactions", transferBody(a, b, 200), key("k"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
}

func makeProtectedAccount(t *testing.T, label string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(),
		`insert into accounts (label, overdraft_protected) values ($1, true) returning id::text`,
		label).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestOverdraftEndpoint(t *testing.T) {
	h := reset(t)
	cash := makeAccount(t, "cash")
	customer := makeProtectedAccount(t, "customer")

	doReq(h, "POST", "/transactions", transferBody(cash, customer, 100), key("fund"))
	rec := doReq(h, "POST", "/transactions", transferBody(customer, cash, 150), key("withdraw"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "overdraft" {
		t.Fatalf("code = %v, want overdraft", body["code"])
	}
	if body["account"] != "customer" {
		t.Fatalf("account = %v, want customer", body["account"])
	}
}
