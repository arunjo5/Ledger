package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"ledger/internal/api"
	"ledger/internal/config"
	"ledger/internal/db"
	"ledger/internal/ledger"
	"ledger/migrations"
)

func main() {
	cfg := config.Load()

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "serve":
		if err := serve(cfg); err != nil {
			log.Fatal(err)
		}
	case "migrate":
		if err := migrate(cfg); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}

func serve(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	store := ledger.NewStore(pool)
	if n, err := store.RecoverPending(ctx); err != nil {
		return err
	} else if n > 0 {
		log.Printf("recovered %d pending transactions", n)
	}

	return api.NewServer(cfg, store).Run(ctx)
}

func migrate(cfg config.Config) error {
	ctx := context.Background()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := db.Migrate(ctx, pool, migrations.FS)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		log.Println("no new migrations")
		return nil
	}
	for _, name := range applied {
		log.Printf("applied %s", name)
	}
	return nil
}
