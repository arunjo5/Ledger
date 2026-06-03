package ledger

import (
	"context"
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

var propCurrencies = []string{"USD", "EUR", "GBP"}

func TestPropertyConservationAndBalances(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := freshStore(rt)
		ctx := context.Background()
		accounts := makeAccounts(rt, store, rapid.IntRange(2, 5).Draw(rt, "accounts"))

		n := rapid.IntRange(0, 15).Draw(rt, "transactions")
		for i := 0; i < n; i++ {
			if _, err := store.CreateTransaction(ctx, NewTransaction{
				IdempotencyKey: nextKey(),
				RequestHash:    []byte("h"),
				Entries:        genBalanced(rt, accounts),
			}); err != nil {
				rt.Fatalf("balanced transaction failed: %v", err)
			}
		}

		assertConservation(rt)
		for _, acc := range accounts {
			assertBalanceMatchesSum(rt, store, acc)
		}
	})
}

func TestPropertyNoPartialCommitsUnderMixedLoad(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := freshStore(rt)
		ctx := context.Background()
		accounts := makeAccounts(rt, store, rapid.IntRange(2, 5).Draw(rt, "accounts"))

		committed := 0
		n := rapid.IntRange(0, 15).Draw(rt, "transactions")
		for i := 0; i < n; i++ {
			if rapid.Bool().Draw(rt, "valid") {
				if _, err := store.CreateTransaction(ctx, NewTransaction{
					IdempotencyKey: nextKey(),
					RequestHash:    []byte("h"),
					Entries:        genBalanced(rt, accounts),
				}); err != nil {
					rt.Fatalf("balanced transaction failed: %v", err)
				}
				committed++
			} else {
				if _, err := store.CreateTransaction(ctx, NewTransaction{
					IdempotencyKey: nextKey(),
					RequestHash:    []byte("h"),
					Entries:        genUnbalanced(rt, accounts),
				}); err == nil {
					rt.Fatal("unbalanced transaction should have been rejected")
				}
			}
		}

		assertConservation(rt)
		if got := countCommitted(rt); got != committed {
			rt.Fatalf("committed transactions = %d, want %d", got, committed)
		}
	})
}

func TestPropertyIdempotencyNoDuplicates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		store := freshStore(rt)
		ctx := context.Background()
		accounts := makeAccounts(rt, store, rapid.IntRange(2, 4).Draw(rt, "accounts"))

		keys := rapid.IntRange(1, 6).Draw(rt, "keys")
		for k := 0; k < keys; k++ {
			key := fmt.Sprintf("key-%d", k)
			hash := []byte(fmt.Sprintf("body-%d", k))
			entries := genBalanced(rt, accounts)
			retries := rapid.IntRange(1, 4).Draw(rt, "retries")
			for r := 0; r < retries; r++ {
				if _, err := store.CreateTransaction(ctx, NewTransaction{
					IdempotencyKey: key,
					RequestHash:    hash,
					Entries:        entries,
				}); err != nil {
					rt.Fatalf("idempotent submit failed: %v", err)
				}
			}
		}

		if got := countTransactions(rt); got != keys {
			rt.Fatalf("transactions = %d, want %d", got, keys)
		}
	})
}

func freshStore(rt *rapid.T) *Store {
	if _, err := testPool.Exec(context.Background(),
		`truncate entries, transactions, accounts restart identity cascade`); err != nil {
		rt.Fatalf("truncate: %v", err)
	}
	return NewStore(testPool)
}

func makeAccounts(rt *rapid.T, store *Store, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		acc, err := store.CreateAccount(context.Background(), nil)
		if err != nil {
			rt.Fatalf("create account: %v", err)
		}
		ids[i] = acc.ID
	}
	return ids
}

func genBalanced(rt *rapid.T, accounts []string) []EntryInput {
	legs := rapid.IntRange(1, 3).Draw(rt, "legs")
	var entries []EntryInput
	for i := 0; i < legs; i++ {
		cur := rapid.SampledFrom(propCurrencies).Draw(rt, "currency")
		amt := rapid.Int64Range(1, 1_000_000).Draw(rt, "amount")
		from, to := twoAccounts(rt, accounts)
		entries = append(entries,
			EntryInput{AccountID: from, Currency: cur, Amount: -amt},
			EntryInput{AccountID: to, Currency: cur, Amount: amt})
	}
	return entries
}

func genUnbalanced(rt *rapid.T, accounts []string) []EntryInput {
	cur := rapid.SampledFrom(propCurrencies).Draw(rt, "currency")
	a1 := rapid.Int64Range(1, 1000).Draw(rt, "a1")
	a2 := rapid.Int64Range(1, 1000).Draw(rt, "a2")
	if a1 == a2 {
		a2++
	}
	from, to := twoAccounts(rt, accounts)
	return []EntryInput{
		{AccountID: from, Currency: cur, Amount: -a1},
		{AccountID: to, Currency: cur, Amount: a2},
	}
}

func twoAccounts(rt *rapid.T, accounts []string) (string, string) {
	i := rapid.IntRange(0, len(accounts)-1).Draw(rt, "from")
	j := rapid.IntRange(0, len(accounts)-2).Draw(rt, "to")
	if j >= i {
		j++
	}
	return accounts[i], accounts[j]
}

func assertConservation(rt *rapid.T) {
	rows, err := testPool.Query(context.Background(),
		`select currency, sum(amount)::bigint from entries group by currency`)
	if err != nil {
		rt.Fatalf("conservation query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cur string
		var sum int64
		if err := rows.Scan(&cur, &sum); err != nil {
			rt.Fatalf("scan: %v", err)
		}
		if sum != 0 {
			rt.Fatalf("currency %s sums to %d, want 0", cur, sum)
		}
	}
	if err := rows.Err(); err != nil {
		rt.Fatalf("rows: %v", err)
	}
}

func assertBalanceMatchesSum(rt *rapid.T, store *Store, account string) {
	balances, err := store.Balance(context.Background(), account, nil)
	if err != nil {
		rt.Fatalf("balance: %v", err)
	}
	for _, b := range balances {
		var direct int64
		if err := testPool.QueryRow(context.Background(),
			`select coalesce(sum(amount), 0)::bigint from entries where account_id = $1::uuid and currency = $2`,
			account, b.Currency).Scan(&direct); err != nil {
			rt.Fatalf("direct sum: %v", err)
		}
		if b.Amount != direct {
			rt.Fatalf("account %s %s: balance %d != sum %d", account, b.Currency, b.Amount, direct)
		}
	}
}

func countCommitted(rt *rapid.T) int {
	var n int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from transactions where status = 'committed'`).Scan(&n); err != nil {
		rt.Fatalf("count committed: %v", err)
	}
	return n
}

func countTransactions(rt *rapid.T) int {
	var n int
	if err := testPool.QueryRow(context.Background(),
		`select count(*) from transactions`).Scan(&n); err != nil {
		rt.Fatalf("count transactions: %v", err)
	}
	return n
}
