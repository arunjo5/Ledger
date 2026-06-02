package ledger

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUnknownAccount         = errors.New("unknown account")
	ErrIdempotencyKeyConflict = errors.New("idempotency key already used")
	ErrNotBalanced            = errors.New("transaction is not balanced per currency")
	ErrNotCommitted           = errors.New("transaction is not committed")
)

type EntryInput struct {
	AccountID string
	Currency  string
	Amount    int64
}

type NewTransaction struct {
	IdempotencyKey        string
	RequestHash           []byte
	Description           string
	ReversesTransactionID *string
	Entries               []EntryInput
}

// CreateTransaction records a pending intent, then commits entries and status atomically.
func (s *Store) CreateTransaction(ctx context.Context, in NewTransaction) (Transaction, error) {
	if err := s.ensureAccountsExist(ctx, in.Entries); err != nil {
		return Transaction{}, err
	}

	var txID string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		`insert into transactions (description, idempotency_key, request_hash, reverses_transaction_id)
		 values ($1, $2, $3, $4::uuid)
		 returning id::text, created_at`,
		in.Description, in.IdempotencyKey, in.RequestHash, in.ReversesTransactionID).
		Scan(&txID, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return Transaction{}, ErrIdempotencyKeyConflict
		}
		return Transaction{}, err
	}

	txn, err := s.commitEntries(ctx, txID, in, createdAt)
	if err != nil {
		s.markFailed(ctx, txID)
		return Transaction{}, err
	}
	return txn, nil
}

func (s *Store) commitEntries(ctx context.Context, txID string, in NewTransaction, createdAt time.Time) (Transaction, error) {
	dbTx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transaction{}, err
	}
	defer dbTx.Rollback(ctx)

	entries := make([]Entry, 0, len(in.Entries))
	for _, e := range in.Entries {
		var id int64
		var created time.Time
		err := dbTx.QueryRow(ctx,
			`insert into entries (transaction_id, account_id, currency, amount)
			 values ($1::uuid, $2::uuid, $3, $4)
			 returning id, created_at`,
			txID, e.AccountID, e.Currency, e.Amount).Scan(&id, &created)
		if err != nil {
			return Transaction{}, err
		}
		entries = append(entries, Entry{
			ID:            id,
			TransactionID: txID,
			AccountID:     e.AccountID,
			Currency:      e.Currency,
			Amount:        e.Amount,
			CreatedAt:     created,
		})
	}

	var committedAt time.Time
	if err := dbTx.QueryRow(ctx,
		`update transactions set status = 'committed', committed_at = now()
		 where id = $1::uuid returning committed_at`, txID).Scan(&committedAt); err != nil {
		return Transaction{}, err
	}

	txn := Transaction{
		ID:                    txID,
		Status:                StatusCommitted,
		Description:           in.Description,
		ReversesTransactionID: in.ReversesTransactionID,
		Entries:               entries,
		CreatedAt:             createdAt,
		CommittedAt:           &committedAt,
	}

	body, err := json.Marshal(txn)
	if err != nil {
		return Transaction{}, err
	}
	if _, err := dbTx.Exec(ctx,
		`update transactions set response_body = $2, response_status = 201 where id = $1::uuid`,
		txID, string(body)); err != nil {
		return Transaction{}, err
	}

	if err := dbTx.Commit(ctx); err != nil {
		if pgErrorCode(err) == "LG001" {
			return Transaction{}, ErrNotBalanced
		}
		return Transaction{}, err
	}
	return txn, nil
}

func (s *Store) markFailed(ctx context.Context, txID string) {
	_, _ = s.pool.Exec(ctx,
		`update transactions set status = 'failed' where id = $1::uuid and status = 'pending'`, txID)
}

func (s *Store) GetTransaction(ctx context.Context, id string) (Transaction, error) {
	var txn Transaction
	var status string
	err := s.pool.QueryRow(ctx,
		`select id::text, status, description, reverses_transaction_id::text, created_at, committed_at
		 from transactions where id = $1::uuid`, id).
		Scan(&txn.ID, &status, &txn.Description, &txn.ReversesTransactionID, &txn.CreatedAt, &txn.CommittedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, ErrNotFound
		}
		return Transaction{}, err
	}
	txn.Status = Status(status)

	entries, err := s.entriesByTransaction(ctx, id)
	if err != nil {
		return Transaction{}, err
	}
	txn.Entries = entries
	return txn, nil
}

func (s *Store) entriesByTransaction(ctx context.Context, txID string) ([]Entry, error) {
	rows, err := s.pool.Query(ctx,
		`select id, transaction_id::text, account_id::text, currency, amount, created_at
		 from entries where transaction_id = $1::uuid order by id`, txID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Currency, &e.Amount, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Balance derives per-currency balances from committed entries, optionally as of a past instant.
func (s *Store) Balance(ctx context.Context, accountID string, asOf *time.Time) ([]Balance, error) {
	rows, err := s.pool.Query(ctx,
		`select e.currency, sum(e.amount)::bigint
		 from entries e
		 join transactions t on t.id = e.transaction_id
		 where e.account_id = $1::uuid
		   and t.status = 'committed'
		   and ($2::timestamptz is null or t.committed_at <= $2)
		 group by e.currency
		 order by e.currency`, accountID, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := []Balance{}
	for rows.Next() {
		var b Balance
		if err := rows.Scan(&b.Currency, &b.Amount); err != nil {
			return nil, err
		}
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

func (s *Store) AccountEntries(ctx context.Context, accountID string, limit int) ([]Entry, error) {
	rows, err := s.pool.Query(ctx,
		`select id, transaction_id::text, account_id::text, currency, amount, created_at
		 from entries where account_id = $1::uuid
		 order by id desc limit $2`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Currency, &e.Amount, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ReverseTransaction posts the mirror image of a committed transaction.
func (s *Store) ReverseTransaction(ctx context.Context, originalID, idempotencyKey string, requestHash []byte) (Transaction, error) {
	original, err := s.GetTransaction(ctx, originalID)
	if err != nil {
		return Transaction{}, err
	}
	if original.Status != StatusCommitted {
		return Transaction{}, ErrNotCommitted
	}

	entries := make([]EntryInput, 0, len(original.Entries))
	for _, e := range original.Entries {
		entries = append(entries, EntryInput{
			AccountID: e.AccountID,
			Currency:  e.Currency,
			Amount:    -e.Amount,
		})
	}

	return s.CreateTransaction(ctx, NewTransaction{
		IdempotencyKey:        idempotencyKey,
		RequestHash:           requestHash,
		Description:           "reversal of " + originalID,
		ReversesTransactionID: &originalID,
		Entries:               entries,
	})
}

func (s *Store) ensureAccountsExist(ctx context.Context, entries []EntryInput) error {
	seen := map[string]bool{}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if !seen[e.AccountID] {
			seen[e.AccountID] = true
			ids = append(ids, e.AccountID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	var count int
	if err := s.pool.QueryRow(ctx,
		`select count(*) from accounts where id = any($1::uuid[])`, ids).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return ErrUnknownAccount
	}
	return nil
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
