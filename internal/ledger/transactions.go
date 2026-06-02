package ledger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUnknownAccount         = errors.New("unknown account")
	ErrIdempotencyKeyConflict = errors.New("idempotency key already used with a different request")
	ErrPriorAttemptFailed     = errors.New("a prior request with this idempotency key failed")
	ErrInProgress             = errors.New("a request with this idempotency key is still being processed")
	ErrNotBalanced            = errors.New("transaction is not balanced per currency")
	ErrNotCommitted           = errors.New("transaction is not committed")
)

const (
	statusCreated           = 201
	idempotencyPollInterval = 25 * time.Millisecond
	idempotencyMaxWait      = 3 * time.Second
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

// CreateResult is the outcome of a create: a fresh commit or an idempotent replay.
type CreateResult struct {
	Transaction Transaction
	Status      int
	Body        []byte
	Replayed    bool
}

// CreateTransaction commits a transaction. A duplicate idempotency key resolves to the cached response.
func (s *Store) CreateTransaction(ctx context.Context, in NewTransaction) (CreateResult, error) {
	if err := s.ensureAccountsExist(ctx, in.Entries); err != nil {
		return CreateResult{}, err
	}

	txID, createdAt, err := s.insertPending(ctx, in)
	if err != nil {
		if isUniqueViolation(err) {
			return s.resolveExisting(ctx, in)
		}
		return CreateResult{}, err
	}

	txn, body, err := s.commitEntries(ctx, txID, in, createdAt)
	if err != nil {
		s.markFailed(ctx, txID)
		return CreateResult{}, err
	}
	return CreateResult{Transaction: txn, Status: statusCreated, Body: body}, nil
}

func (s *Store) insertPending(ctx context.Context, in NewTransaction) (string, time.Time, error) {
	var txID string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx,
		`insert into transactions (description, idempotency_key, request_hash, reverses_transaction_id)
		 values ($1, $2, $3, $4::uuid)
		 returning id::text, created_at`,
		in.Description, in.IdempotencyKey, in.RequestHash, in.ReversesTransactionID).
		Scan(&txID, &createdAt)
	return txID, createdAt, err
}

func (s *Store) commitEntries(ctx context.Context, txID string, in NewTransaction, createdAt time.Time) (Transaction, []byte, error) {
	dbTx, err := s.pool.Begin(ctx)
	if err != nil {
		return Transaction{}, nil, err
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
			return Transaction{}, nil, err
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
		return Transaction{}, nil, err
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
		return Transaction{}, nil, err
	}
	if _, err := dbTx.Exec(ctx,
		`update transactions set response_body = $2, response_status = $3 where id = $1::uuid`,
		txID, string(body), statusCreated); err != nil {
		return Transaction{}, nil, err
	}

	if err := dbTx.Commit(ctx); err != nil {
		if pgErrorCode(err) == "LG001" {
			return Transaction{}, nil, ErrNotBalanced
		}
		return Transaction{}, nil, err
	}
	return txn, body, nil
}

// resolveExisting returns the cached response for a duplicate key, or conflicts on a different body.
func (s *Store) resolveExisting(ctx context.Context, in NewTransaction) (CreateResult, error) {
	deadline := time.Now().Add(idempotencyMaxWait)
	for {
		c, found, err := s.lookupByKey(ctx, in.IdempotencyKey)
		if err != nil {
			return CreateResult{}, err
		}
		if !found {
			return CreateResult{}, ErrNotFound
		}
		if !bytes.Equal(c.requestHash, in.RequestHash) {
			return CreateResult{}, ErrIdempotencyKeyConflict
		}

		switch Status(c.status) {
		case StatusCommitted:
			status := statusCreated
			if c.responseStatus != nil {
				status = *c.responseStatus
			}
			var body []byte
			var txn Transaction
			if c.responseBody != nil {
				body = []byte(*c.responseBody)
				_ = json.Unmarshal(body, &txn)
			}
			return CreateResult{Transaction: txn, Status: status, Body: body, Replayed: true}, nil
		case StatusFailed:
			return CreateResult{}, ErrPriorAttemptFailed
		default:
			if time.Now().After(deadline) {
				return CreateResult{}, ErrInProgress
			}
			select {
			case <-ctx.Done():
				return CreateResult{}, ctx.Err()
			case <-time.After(idempotencyPollInterval):
			}
		}
	}
}

type cachedTx struct {
	status         string
	requestHash    []byte
	responseBody   *string
	responseStatus *int
}

func (s *Store) lookupByKey(ctx context.Context, key string) (cachedTx, bool, error) {
	var c cachedTx
	err := s.pool.QueryRow(ctx,
		`select status, request_hash, response_body, response_status
		 from transactions where idempotency_key = $1`, key).
		Scan(&c.status, &c.requestHash, &c.responseBody, &c.responseStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cachedTx{}, false, nil
		}
		return cachedTx{}, false, err
	}
	return c, true, nil
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
func (s *Store) ReverseTransaction(ctx context.Context, originalID, idempotencyKey string, requestHash []byte) (CreateResult, error) {
	original, err := s.GetTransaction(ctx, originalID)
	if err != nil {
		return CreateResult{}, err
	}
	if original.Status != StatusCommitted {
		return CreateResult{}, ErrNotCommitted
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
