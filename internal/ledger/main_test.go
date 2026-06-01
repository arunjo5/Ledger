package ledger

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ledger/internal/pgtest"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	pool, cleanup, err := pgtest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgtest: %v\n", err)
		os.Exit(1)
	}
	testPool = pool

	code := m.Run()
	cleanup()
	os.Exit(code)
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`truncate entries, transactions, accounts restart identity cascade`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return NewStore(testPool)
}
