package ledger

import (
	"context"
	"strings"
	"testing"
)

func TestRecoveryFailsLingeringPending(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	txID, _, err := store.insertPending(ctx, NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("h"),
	})
	if err != nil {
		t.Fatal(err)
	}

	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")
	good := commit(t, store, usd(a, -100), usd(b, 100))

	n, err := store.RecoverPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("recovered %d, want 1", n)
	}

	failed, err := store.GetTransaction(ctx, txID)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", failed.Status)
	}
	if len(failed.Entries) != 0 {
		t.Fatalf("failed transaction has %d entries, want 0", len(failed.Entries))
	}

	stillCommitted, err := store.GetTransaction(ctx, good.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stillCommitted.Status != StatusCommitted {
		t.Fatalf("committed tx status = %s, want committed", stillCommitted.Status)
	}

	result, err := store.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("reconcile not ok after recovery: %+v", result.Checks)
	}
}

func TestReconcilePassesOnHealthyLedger(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	commit(t, store, usd(a, -500), usd(b, 500))
	commit(t, store, usd(b, -200), usd(a, 200))

	result, err := store.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("reconcile not ok: %+v", result.Checks)
	}
	if len(result.Checks) == 0 {
		t.Fatal("expected checks to be reported")
	}
}

func TestReconcileDetectsUnbalanced(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a := mustAccount(t, store, "a")

	if _, err := testPool.Exec(ctx,
		`alter table transactions disable trigger trg_transactions_balanced`); err != nil {
		t.Fatal(err)
	}
	defer testPool.Exec(ctx, `alter table transactions enable trigger trg_transactions_balanced`)

	var txID string
	if err := testPool.QueryRow(ctx,
		`insert into transactions (idempotency_key, request_hash) values ($1, $2) returning id::text`,
		nextKey(), []byte("h")).Scan(&txID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx,
		`insert into entries (transaction_id, account_id, currency, amount)
		 values ($1::uuid, $2::uuid, 'USD', 100)`, txID, a); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx,
		`update transactions set status = 'committed', committed_at = now() where id = $1::uuid`, txID); err != nil {
		t.Fatal(err)
	}

	result, err := store.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.OK {
		t.Fatal("reconcile should fail for an unbalanced committed transaction")
	}

	var caught bool
	for _, c := range result.Checks {
		if strings.Contains(c.Name, "balance") && !c.OK {
			caught = true
		}
	}
	if !caught {
		t.Fatalf("balance check should have failed: %+v", result.Checks)
	}
}
