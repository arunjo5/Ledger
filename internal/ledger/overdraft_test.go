package ledger

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func protectedAccount(t *testing.T, label string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(),
		`insert into accounts (label, overdraft_protected) values ($1, true) returning id::text`,
		label).Scan(&id); err != nil {
		t.Fatalf("create protected account: %v", err)
	}
	return id
}

func usdBalance(t *testing.T, store *Store, account string) int64 {
	t.Helper()
	balances, err := store.Balance(context.Background(), account, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range balances {
		if b.Currency == "USD" {
			return b.Amount
		}
	}
	return 0
}

func TestOverdraftRejected(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	cash := mustAccount(t, store, "cash")
	customer := protectedAccount(t, "customer")

	commit(t, store, usd(cash, -100), usd(customer, 100))

	_, err := store.CreateTransaction(ctx, NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("h"),
		Entries:        []EntryInput{usd(customer, -150), usd(cash, 150)},
	})
	var overdraft *OverdraftError
	if !errors.As(err, &overdraft) {
		t.Fatalf("err = %v, want OverdraftError", err)
	}
	if overdraft.Account != "customer" || overdraft.Currency != "USD" {
		t.Fatalf("overdraft = %+v", overdraft)
	}
	if got := usdBalance(t, store, customer); got != 100 {
		t.Fatalf("customer balance = %d, want 100 (rejected withdrawal)", got)
	}
}

func TestNonProtectedCanGoNegative(t *testing.T) {
	store := newTestStore(t)
	cash := mustAccount(t, store, "cash")
	customer := mustAccount(t, store, "customer")

	commit(t, store, usd(cash, -500), usd(customer, 500))

	if got := usdBalance(t, store, cash); got != -500 {
		t.Fatalf("cash balance = %d, want -500 (non-protected may go negative)", got)
	}
}

func TestOverdraftConcurrentWithdrawalsCannotOverdraw(t *testing.T) {
	store := newTestStore(t)
	cash := mustAccount(t, store, "cash")
	customer := protectedAccount(t, "customer")

	commit(t, store, usd(cash, -100), usd(customer, 100))

	const n = 5
	var wg sync.WaitGroup
	var successes int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.CreateTransaction(context.Background(), NewTransaction{
				IdempotencyKey: nextKey(),
				RequestHash:    []byte("h"),
				Entries:        []EntryInput{usd(customer, -60), usd(cash, 60)},
			}); err == nil {
				atomic.AddInt64(&successes, 1)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("successful withdrawals = %d, want 1", successes)
	}
	if got := usdBalance(t, store, customer); got != 40 {
		t.Fatalf("customer balance = %d, want 40 (never overdrawn)", got)
	}
}
