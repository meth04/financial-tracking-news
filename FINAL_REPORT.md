# Final Report

## What was built

- Bootstrapped a complete Go backend under `cmd/finnews` and `internal/*`.
- Added PostgreSQL migration execution for `db/schema.sql` and source seeding from `config/sources.seed.yaml`.
- Implemented crawler scheduler and manual crawl flow with raw-item-first persistence.
- Implemented RSS, Federal Register API, SEC EDGAR watchlist, and conservative generic HTML source adapters.
- Implemented normalization, URL canonicalization, title/content hashing, word counts, and SimHash helpers.
- Implemented exact duplicate decisions, near-duplicate helper logic, same-source T+1 update candidate handling, event clustering, relation classification, and event update creation.
- Implemented OpenAI-compatible LLM client, strict JSON parser, durable PostgreSQL job worker, retry backoff, one-analysis-per-article enforcement, and max concurrency configuration defaulting to 3.
- Implemented REST API endpoints from `api/openapi.yaml` for health, stats, articles, clusters, sources, queue status, crawl-once, and retry-failed-LLM.
- Built a Vite React TypeScript dashboard with dark dense table styling, filters, stat pills, badges, source health, LLM queue monitor, article drawer, and event timeline drawer.
- Added Dockerfile, Docker Compose, Makefile targets, tests, README updates, and known limitations.

## How to run

```bash
cp .env.example .env
make db-up
make migrate
make seed
make run
```

Frontend:

```bash
cd web
npm install
npm run dev
```

Manual crawl:

```bash
make crawl-once
```

Tests/builds:

```bash
make test
make lint
make build
cd web && npm run build
```

## Main endpoints

- `GET /api/health`
- `GET /api/stats`
- `GET /api/articles`
- `GET /api/articles/{id}`
- `GET /api/clusters`
- `GET /api/clusters/{id}`
- `GET /api/sources`
- `GET /api/jobs/llm`
- `POST /api/admin/crawl-once`
- `POST /api/admin/retry-failed-llm`

## Directory structure

- `cmd/finnews`: CLI entrypoint.
- `internal/config`: YAML/env config loading and validation.
- `internal/db`: pgx pool, migrations, repositories, models.
- `internal/source/*`: source adapter interfaces and implementations.
- `internal/crawler`: scheduler and crawl pipeline.
- `internal/normalize`: canonicalization, text normalization, hashing, SimHash.
- `internal/dedup`: exact/near duplicate and update classification logic.
- `internal/cluster`: event clustering and update attachment.
- `internal/llm`: LLM client, parser, worker queue.
- `internal/jobs`: retry backoff helper.
- `internal/api`: chi HTTP API.
- `internal/ops`: retention helper.
- `web`: React TypeScript frontend.
- `db`: PostgreSQL schema.
- `docs`: architecture, operations, and limitations.

## Verification performed

Passed:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/finnews`
- `npm --prefix web install`
- `npm --prefix web run build`

Could not run in this environment because the commands are not installed:

- `make db-up` (`make` not found)
- direct Docker Compose fallback (`docker` not found)

## Remaining limitations

- Generic HTML crawling is intentionally conservative and metadata-first.
- SEC adapter uses the configured ticker watchlist, not the whole market.
- Cluster repair CLI is a placeholder; normal clustering runs after LLM success.
- PostgreSQL integration tests are not enabled by default.
- The Vite frontend is run separately in development; Go serves the REST API.
