-include .env

DB_URL = postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
COMPOSE = docker compose --env-file .env -f docker/docker-compose.yml

.PHONY: help build run dev lint migrate-up migrate-down docker-up docker-down

help:
	@echo "Makefile commands:"
	@echo "  build          - Build the Go application"
	@echo "  run            - Run the built application"
	@echo "  dev            - Run the application in development mode"
	@echo "  lint           - Run golangci-lint on the codebase"
	@echo "  migrate-up     - Apply database migrations"
	@echo "  migrate-down   - Rollback database migrations"

build:
	go build -o bin/api.exe cmd/api/main.go

run: build
	bin\api.exe

dev: 
	go run cmd/api/main.go

lint:
	golangci-lint run ./...

migrate-up:
	migrate -path db/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_URL)" down

docker-up:
	$(COMPOSE) up -d

docker-down:
	$(COMPOSE) down