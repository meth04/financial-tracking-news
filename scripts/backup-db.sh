#!/usr/bin/env bash
set -euo pipefail

mkdir -p backups
TS=$(date +%Y%m%d-%H%M%S)
OUT="backups/finnews-$TS.sql.gz"

docker compose -f docker-compose.example.yml exec -T postgres pg_dump -U finnews finnews | gzip > "$OUT"

echo "Backup written to $OUT"
