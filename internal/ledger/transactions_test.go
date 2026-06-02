package ledger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

var keyCounter atomic.Int64

func nextKey() string {
	return fmt.Sprintf("test-key-%d", keyCounter.Add(1))
}

func mustAccount(t *testing.T, store *Store, label string) string {
	t.Helper()
	acc, err := store.CreateAccount(context.Background(), &label)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acc.ID
}

func commit(t *testing.T, store *Store, entries ...EntryInput) Transaction {
	t.Helper()
	res, err := store.CreateTransaction(context.Background(), NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("h"),
		Description:    "test",
		Entries:        entries,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	return res.Transaction
}

func usd(account string, amount int64) EntryInput {
	return EntryInput{AccountID: account, Currency: "USD", Amount: amount}
}

func TestCreateTransactionCommits(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	txn := commit(t, store, usd(a, -100), usd(b, 100))

	if txn.Status != StatusCommitted {
		t.Fatalf("status = %s, want committed", txn.Status)
	}
	if len(txn.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(txn.Entries))
	}
	if txn.CommittedAt == nil {
		t.Fatal("committed_at is nil")
	}
}

func TestConservationGlobalSumIsZero(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")
	c := mustAccount(t, store, "c")

	commit(t, store, usd(a, -500), usd(b, 500))
	commit(t, store, usd(b, -200), usd(c, 200))
	commit(t, store, usd(c, -50), usd(a, 50))

	rows, err := testPool.Query(context.Background(),
		`select currency, sum(amount)::bigint from entries group by currency`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var currency string
		var sum int64
		if err := rows.Scan(&currency, &sum); err != nil {
			t.Fatal(err)
		}
		if sum != 0 {
			t.Fatalf("currency %s sums to %d, want 0", currency, sum)
		}
	}
}

func TestUnbalancedLeavesNoPartialCommit(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	_, err := store.CreateTransaction(context.Background(), NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("h"),
		Entries:        []EntryInput{usd(a, -100), usd(b, 50)},
	})
	if !errors.Is(err, ErrNotBalanced) {
		t.Fatalf("err = %v, want ErrNotBalanced", err)
	}

	var entryCount int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from entries`).Scan(&entryCount); err != nil {
		t.Fatal(err)
	}
	if entryCount != 0 {
		t.Fatalf("entries = %d, want 0 (no partial commit)", entryCount)
	}

	var failed int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from transactions where status = 'failed'`).Scan(&failed); err != nil {
		t.Fatal(err)
	}
	if failed != 1 {
		t.Fatalf("failed transactions = %d, want 1", failed)
	}
}

func TestUnknownAccountRejected(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	missing := "00000000-0000-0000-0000-000000000000"

	_, err := store.CreateTransaction(context.Background(), NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("h"),
		Entries:        []EntryInput{usd(a, -100), usd(missing, 100)},
	})
	if !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("err = %v, want ErrUnknownAccount", err)
	}
}

func TestBalanceEqualsSumOfEntries(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	commit(t, store, usd(a, -700), usd(b, 700))
	commit(t, store, usd(a, 250), usd(b, -250))

	balances, err := store.Balance(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 1 || balances[0].Currency != "USD" || balances[0].Amount != -450 {
		t.Fatalf("balances = %+v, want USD -450", balances)
	}
}

func TestTimeTravelBalance(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	first := commit(t, store, usd(a, -100), usd(b, 100))
	commit(t, store, usd(a, -40), usd(b, 40))

	asOf := *first.CommittedAt
	balances, err := store.Balance(ctx, a, &asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 1 || balances[0].Amount != -100 {
		t.Fatalf("as-of balance = %+v, want USD -100", balances)
	}

	now, err := store.Balance(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if now[0].Amount != -140 {
		t.Fatalf("current balance = %d, want -140", now[0].Amount)
	}
}

func TestReverseTransaction(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	original := commit(t, store, usd(a, -100), usd(b, 100))

	res, err := store.ReverseTransaction(ctx, original.ID, nextKey(), []byte("h"))
	if err != nil {
		t.Fatal(err)
	}
	reversal := res.Transaction
	if reversal.ReversesTransactionID == nil || *reversal.ReversesTransactionID != original.ID {
		t.Fatalf("reverses = %v, want %s", reversal.ReversesTransactionID, original.ID)
	}

	balances, err := store.Balance(ctx, a, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 1 || balances[0].Amount != 0 {
		t.Fatalf("balance after reversal = %+v, want USD 0", balances)
	}
}

func TestGetTransactionNotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.GetTransaction(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestIdempotentReplayReturnsCached(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")
	in := NewTransaction{
		IdempotencyKey: nextKey(),
		RequestHash:    []byte("body-1"),
		Entries:        []EntryInput{usd(a, -100), usd(b, 100)},
	}

	first, err := store.CreateTransaction(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replayed {
		t.Fatal("first call should not be a replay")
	}

	second, err := store.CreateTransaction(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed {
		t.Fatal("second call should be a replay")
	}
	if second.Transaction.ID != first.Transaction.ID {
		t.Fatalf("ids differ: %s vs %s", first.Transaction.ID, second.Transaction.ID)
	}
	if !bytes.Equal(second.Body, first.Body) {
		t.Fatal("replay body differs from the original")
	}

	if got := transactionCount(t); got != 1 {
		t.Fatalf("transactions = %d, want 1", got)
	}
}

func TestIdempotencyConflictDifferentBody(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")
	key := nextKey()

	if _, err := store.CreateTransaction(context.Background(), NewTransaction{
		IdempotencyKey: key,
		RequestHash:    []byte("body-1"),
		Entries:        []EntryInput{usd(a, -100), usd(b, 100)},
	}); err != nil {
		t.Fatal(err)
	}

	_, err := store.CreateTransaction(context.Background(), NewTransaction{
		IdempotencyKey: key,
		RequestHash:    []byte("body-2"),
		Entries:        []EntryInput{usd(a, -100), usd(b, 100)},
	})
	if !errors.Is(err, ErrIdempotencyKeyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyConflict", err)
	}
}

func TestConcurrentSameKeyCreatesOne(t *testing.T) {
	store := newTestStore(t)
	a := mustAccount(t, store, "a")
	b := mustAccount(t, store, "b")

	const n = 8
	key := nextKey()
	hash := []byte("same-body")

	var wg sync.WaitGroup
	results := make([]CreateResult, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = store.CreateTransaction(context.Background(), NewTransaction{
				IdempotencyKey: key,
				RequestHash:    hash,
				Entries:        []EntryInput{usd(a, -100), usd(b, 100)},
			})
		}(i)
	}
	wg.Wait()

	var id string
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if id == "" {
			id = results[i].Transaction.ID
		} else if results[i].Transaction.ID != id {
			t.Fatalf("different transaction ids: %s and %s", id, results[i].Transaction.ID)
		}
	}

	if got := transactionCount(t); got != 1 {
		t.Fatalf("transactions = %d, want exactly 1", got)
	}
}

func transactionCount(t *testing.T) int {
	t.Helper()
	var count int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from transactions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
