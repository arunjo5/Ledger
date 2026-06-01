.PHONY: deps run test migrate clean up down

deps:
	go mod tidy

run:
	go run ./cmd/ledger serve

test:
	go test ./...

migrate:
	go run ./cmd/ledger migrate

up:
	docker compose up -d

down:
	docker compose down

clean:
	docker compose down -v
	go clean
	rm -f ledger
