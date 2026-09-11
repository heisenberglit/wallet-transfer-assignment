.PHONY: run test lint fmt migrate-up migrate-down up down

run:
	go run ./cmd/server

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

fmt:
	gofmt -l .

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	psql "$$DATABASE_URL" -f migrations/0001_init.up.sql

migrate-down:
	psql "$$DATABASE_URL" -f migrations/0001_init.down.sql
