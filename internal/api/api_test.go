package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ledger/internal/api"
	"ledger/internal/config"
	"ledger/internal/ledger"
	"ledger/internal/pgtest"
)

var (
	testPool  *pgxpool.Pool
	testStore *ledger.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, cleanup, err := pgtest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgtest: %v\n", err)
		os.Exit(1)
	}
	testPool = pool
	testStore = ledger.NewStore(pool)

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func reset(t *testing.T) http.Handler {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`truncate entries, transactions, accounts restart identity cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return api.NewServer(config.Config{}, testStore).Handler()
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestCreateAccountEndpoint(t *testing.T) {
	h := reset(t)
	rec := do(h, "POST", "/accounts", `{"label":"alice"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d, body = %s", rec.Code, rec.Body.String())
	}
	var acc ledger.Account
	if err := json.Unmarshal(rec.Body.Bytes(), &acc); err != nil {
		t.Fatal(err)
	}
	if acc.ID == "" {
		t.Fatal("missing id")
	}
	if acc.Label == nil || *acc.Label != "alice" {
		t.Fatalf("label = %v, want alice", acc.Label)
	}
}

func TestCreateAccountInvalidJSON(t *testing.T) {
	h := reset(t)
	rec := do(h, "POST", "/accounts", `{"label":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestCreateAccountDuplicateLabel(t *testing.T) {
	h := reset(t)
	do(h, "POST", "/accounts", `{"label":"cash"}`)
	rec := do(h, "POST", "/accounts", `{"label":"cash"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestGetAccountInvalidID(t *testing.T) {
	h := reset(t)
	rec := do(h, "GET", "/accounts/not-a-uuid", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestGetAccountNotFound(t *testing.T) {
	h := reset(t)
	rec := do(h, "GET", "/accounts/00000000-0000-0000-0000-000000000000", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d", rec.Code)
	}
}

func TestListAccountsEndpoint(t *testing.T) {
	h := reset(t)
	do(h, "POST", "/accounts", `{"label":"a"}`)
	do(h, "POST", "/accounts", `{"label":"b"}`)

	rec := do(h, "GET", "/accounts", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var resp struct {
		Accounts []ledger.Account `json:"accounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Accounts) != 2 {
		t.Fatalf("len = %d, want 2", len(resp.Accounts))
	}
}

func TestReconcileNotImplementedYet(t *testing.T) {
	h := reset(t)
	rec := do(h, "POST", "/admin/reconcile", "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("code = %d, want 501", rec.Code)
	}
}
