.PHONY: run build test tidy vet fmt migrate

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

migrate:
	psql "$(DATABASE_URL)" -f migrations/0001_init.sql
