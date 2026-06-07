.PHONY: dev db-up db-down migrate test lint run build frontend-dev frontend-build seed worker crawl-once app-up app-down

DB_COMPOSE=docker compose -f docker-compose.example.yml

db-up:
	@echo "SQLite is embedded; no database service needed. Run make migrate then make seed."

db-down:
	@echo "SQLite is embedded; no database service to stop."

app-up:
	$(DB_COMPOSE) up --build

app-down:
	$(DB_COMPOSE) down

migrate:
	go run ./cmd/finnews migrate up

seed:
	go run ./cmd/finnews seed sources

run:
	go run ./cmd/finnews server

worker:
	go run ./cmd/finnews worker

crawl-once:
	go run ./cmd/finnews crawl once

test:
	go test ./...

lint:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/finnews ./cmd/finnews

frontend-dev:
	cd web && npm run dev

frontend-build:
	cd web && npm run build
