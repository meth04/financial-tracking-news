#!/usr/bin/env bash
set -euo pipefail

if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

docker compose -f docker-compose.example.yml up -d postgres

echo "Waiting for PostgreSQL..."
for i in {1..30}; do
  if docker compose -f docker-compose.example.yml exec -T postgres pg_isready -U finnews -d finnews >/dev/null 2>&1; then
    echo "PostgreSQL is ready"
    break
  fi
  sleep 1
 done

go mod tidy

echo "Run migrations with: make migrate"
echo "Seed sources with: make seed"
echo "Start backend with: make run"
