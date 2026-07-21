.PHONY: dev-up dev-down run fmt vet build

dev-up:
	docker compose up -d postgres

dev-down:
	docker compose down

# Migrations run automatically on startup - see internal/infrastructure/postgres/migrate.go
run:
	go run cmd/api/main.go

build:
	go build -o bin/api ./cmd/api

fmt:
	gofmt -w $$(find . -name '*.go')

vet:
	go vet ./...
