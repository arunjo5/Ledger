package ledger

import (
	"context"
	"fmt"
	"time"
)

type seedAccount struct {
	label     string
	protected bool
}

type seedEntry struct {
	account  string
	currency string
	amount   int64
}

type seedTx struct {
	daysAgo     int
	description string
	entries     []seedEntry
}

var seedAccounts = []seedAccount{
	{"cash:operating", false},
	{"cash:forex_buffer", false},
	{"revenue:sales", false},
	{"fees:processing", false},
	{"liabilities:customer_a", true},
	{"liabilities:customer_b", true},
}

var seedTxs = []seedTx{
	{21, "Customer funding - wire", []seedEntry{{"cash:operating", "USD", -10000}, {"liabilities:customer_a", "USD", 10000}}},
	{10, "Internal transfer customer_a to customer_b", []seedEntry{{"liabilities:customer_a", "USD", -3000}, {"liabilities:customer_b", "USD", 3000}}},
	{5, "Processing fee", []seedEntry{{"liabilities:customer_b", "USD", -100}, {"fees:processing", "USD", 100}}},
	{2, "FX funding (EUR)", []seedEntry{{"cash:forex_buffer", "EUR", -4000}, {"liabilities:customer_a", "EUR", 4000}}},
	{1, "Customer payment - invoice 8821", []seedEntry{{"liabilities:customer_a", "USD", -2000}, {"revenue:sales", "USD", 2000}}},
}

// Seed rebuilds the demo dataset with backdated timestamps so time-travel
// balances visibly change. It bypasses the API to set committed_at in the past.
func (s *Store) Seed(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx,
		`truncate entries, transactions, accounts restart identity cascade`); err != nil {
		return err
	}

	now := time.Now()
	ids := map[string]string{}
	for _, a := range seedAccounts {
		var id string
		if err := s.pool.QueryRow(ctx,
			`insert into accounts (label, overdraft_protected, created_at) values ($1, $2, $3) returning id::text`,
			a.label, a.protected, now.Add(-30*24*time.Hour)).Scan(&id); err != nil {
			return err
		}
		ids[a.label] = id
	}

	for i, tx := range seedTxs {
		ts := now.Add(-time.Duration(tx.daysAgo) * 24 * time.Hour)
		if err := s.seedCommitted(ctx, fmt.Sprintf("seed-%d", i), ts, tx, ids); err != nil {
			return err
		}
	}

	_, err := s.pool.Exec(ctx,
		`insert into transactions (description, idempotency_key, request_hash, status, created_at)
		 values ($1, $2, $3, 'failed', $4)`,
		"Overdraft attempt - customer_b", "seed-failed", []byte("seed"), now.Add(-3*time.Hour))
	return err
}

func (s *Store) seedCommitted(ctx context.Context, key string, ts time.Time, tx seedTx, ids map[string]string) error {
	dbTx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer dbTx.Rollback(ctx)

	var txID string
	if err := dbTx.QueryRow(ctx,
		`insert into transactions (description, idempotency_key, request_hash, status, created_at)
		 values ($1, $2, $3, 'pending', $4) returning id::text`,
		tx.description, key, []byte("seed"), ts).Scan(&txID); err != nil {
		return err
	}
	for _, e := range tx.entries {
		if _, err := dbTx.Exec(ctx,
			`insert into entries (transaction_id, account_id, currency, amount) values ($1::uuid, $2::uuid, $3, $4)`,
			txID, ids[e.account], e.currency, e.amount); err != nil {
			return err
		}
	}
	if _, err := dbTx.Exec(ctx,
		`update transactions set status = 'committed', committed_at = $2 where id = $1::uuid`, txID, ts); err != nil {
		return err
	}
	return dbTx.Commit(ctx)
}
