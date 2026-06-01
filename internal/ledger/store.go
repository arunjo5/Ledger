package ledger

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicateLabel = errors.New("account label already exists")
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateAccount(ctx context.Context, label *string) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx,
		`insert into accounts (label) values ($1)
		 returning id::text, label, created_at`, label).
		Scan(&a.ID, &a.Label, &a.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return Account{}, ErrDuplicateLabel
		}
		return Account{}, err
	}
	return a, nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx,
		`select id::text, label, created_at from accounts where id = $1::uuid`, id).
		Scan(&a.ID, &a.Label, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, err
	}
	return a, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.pool.Query(ctx,
		`select id::text, label, created_at from accounts order by created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Label, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
