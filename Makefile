.PHONY: deps run test migrate seed clean up down

deps:
	go mod tidy

run:
	go run ./cmd/ledger serve

test:
	go test ./...

migrate:
	go run ./cmd/ledger migrate

seed:
	go run ./cmd/ledger seed

up:
	docker compose up -d

down:
	docker compose down

clean:
	docker compose down -v
	go clean
	rm -f ledger
