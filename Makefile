-include .env

DB_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
COMPOSE = docker compose --env-file .env -f docker/docker-compose.yml
MIGRATE = migrate -path db/migrations -database "$(DB_URL)"

.PHONY: help build run dev lint test migrate-up migrate-down migrate-version migrate-create docker-up docker-down

help:
	@echo Makefile commands:
	@echo   build                    - Build the Go application
	@echo   run                      - Build and run the application
	@echo   dev                      - Run with go run
	@echo   lint                     - Run golangci-lint on the codebase
	@echo   test                     - Run all tests
	@echo   migrate-up               - Apply all pending migrations
	@echo   migrate-down             - Roll back the most recent migration
	@echo   migrate-version          - Show the current migration version
	@echo   migrate-create name=NAME - Create a new pair of migration files
	@echo   docker-up                - Start Postgres and LocalStack
	@echo   docker-down              - Stop the containers

build:
	go build -o bin/api.exe cmd/api/main.go

run: build
	bin\api.exe

dev:
	go run cmd/api/main.go

lint:
	golangci-lint run ./...

test:
	go test ./...

migrate-up:
	@$(MIGRATE) up

# Rolls back one migration at a time, so a slip doesn't wipe the whole schema.
migrate-down:
	@$(MIGRATE) down 1

migrate-version:
	@$(MIGRATE) version

migrate-create:
	migrate create -ext sql -dir db/migrations -seq $(name)

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down
