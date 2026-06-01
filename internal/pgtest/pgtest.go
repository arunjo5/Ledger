package pgtest

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"ledger/internal/db"
	"ledger/migrations"
)

// Start launches a throwaway Postgres, applies migrations, and returns a pool
// with a cleanup function that closes the pool and terminates the container.
func Start(ctx context.Context) (*pgxpool.Pool, func(), error) {
	container, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("ledger"),
		postgres.WithUsername("ledger"),
		postgres.WithPassword("ledger"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("start container: %w", err)
	}
	terminate := func() { _ = container.Terminate(context.Background()) }

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		terminate()
		return nil, nil, fmt.Errorf("connection string: %w", err)
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		terminate()
		return nil, nil, fmt.Errorf("open pool: %w", err)
	}

	if _, err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		pool.Close()
		terminate()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	cleanup := func() {
		pool.Close()
		terminate()
	}
	return pool, cleanup, nil
}
