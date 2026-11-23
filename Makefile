-include .env
export

.PHONY: help db-up db-down db-reset migrate-up migrate-down migrate-new sqlc run test vet fmt check

help: ## List commands
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

db-up: ## Start local Postgres
	docker compose up -d --wait

db-down: ## Stop local Postgres (keeps data)
	docker compose down

db-reset: ## Wipe local database and start fresh
	docker compose down -v
	docker compose up -d

migrate-up: ## Apply all migrations (as owner)
	goose -dir migrations postgres "$(MIGRATE_DATABASE_URL)" up

migrate-down: ## Roll back the last migration
	goose -dir migrations postgres "$(MIGRATE_DATABASE_URL)" down

migrate-new: ## Create a migration: make migrate-new name=add_reviews
	goose -dir migrations create $(name) sql

sqlc: ## Regenerate Go code from queries.sql files
	sqlc generate

run: ## Run the API
	go run ./cmd/api

test: ## Run all tests
	go test ./...

vet: ## Static checks
	go vet ./...

fmt: ## Format Go code
	gofmt -w .

check: fmt vet test ## Everything that must pass before a task is done
