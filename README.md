# Financial News Intelligence Crawler

Local Go + embedded SQLite application that collects fresh U.S. financial news from official/free sources, persists raw fetched items first, deduplicates and clusters articles, runs a durable LLM analysis queue, and exposes a dark dense React dashboard.

## Features

- Go backend with `finnews` CLI subcommands.
- Embedded SQLite datastore (`finnews.db` by default) with raw items, normalized articles, duplicates, LLM jobs, analyses, event clusters, and event updates.
- Config-driven RSS/API/HTML source adapters seeded from `config/sources.seed.yaml`.
- Default crawl cadence: **10 minutes**.
- Freshness window: **72 hours**. Outdated articles are retained for debugging but hidden by default in API/UI.
- Raw item persistence happens before normalization, deduplication, clustering, or LLM processing.
- Exact duplicate checks by canonical URL, content hash, and source/title window.
- Same-source T+1 update candidates are not blindly discarded.
- Near-duplicate helpers using SimHash/Jaccard-style similarity.
- LLM worker uses OpenAI-compatible local endpoint with durable PostgreSQL jobs, exponential backoff, and max concurrency **3**.
- React/Vite dashboard: Articles, Events, Sources, LLM Queue, Settings, filters, stat pills, badges, score bars, detail drawers, and event timeline.

## Requirements

- Go 1.25+ (tested with Go 1.26 locally)
- Node.js 20+ for frontend development
- Optional local OpenAI-compatible LLM service at `http://localhost:8317/v1`

## Quick start

```bash
cp .env.example .env
go run ./cmd/finnews migrate up
go run ./cmd/finnews seed sources
go run ./cmd/finnews server
```

In another terminal:

```bash
cd web
npm install
npm run dev
```

Open:

- Backend API: <http://localhost:8080/api/health>
- Frontend: <http://localhost:5173>

## CLI commands

```bash
go run ./cmd/finnews server              # API + scheduler + LLM worker
go run ./cmd/finnews worker              # LLM worker only
go run ./cmd/finnews crawl once          # crawl all enabled sources once
go run ./cmd/finnews crawl once fed_press_all
go run ./cmd/finnews migrate up          # execute db/schema.sql
go run ./cmd/finnews seed sources        # upsert config/sources.seed.yaml
go run ./cmd/finnews cluster repair --fresh-only
```

Make targets:

```bash
make db-up
make migrate
make seed
make run
make worker
make crawl-once
make test
make lint
make build
make frontend-dev
make frontend-build
```

## Configuration

Default app config is `config/app.example.yaml`. Environment overrides include:

```env
SQLITE_PATH=finnews.db
LLM_BASE_URL=http://localhost:8317/v1
LLM_API_KEY=sk-my-key-is-empty
LLM_MODEL=gemini-3.1-flash-lite-preview
LLM_MAX_CONCURRENCY=3
FRESHNESS_MAX_AGE_HOURS=72
```

The placeholder LLM key is intentionally non-secret for a local OpenAI-compatible service.

## Source policy

The app prioritizes official APIs/RSS feeds and public government/company sources. It must not bypass paywalls, login walls, CAPTCHAs, bot protection, robots restrictions, or website terms. If full-content crawling is not clearly allowed, source adapters store title/excerpt/link/source/published time only.

Seed sources are in `config/sources.seed.yaml`, including Federal Reserve RSS, BEA RSS, Federal Register API, SEC EDGAR watchlist, Treasury press release listing, and BLS latest releases.

## Freshness and outdated data

- `fresh`: `published_at >= now() - 72 hours`.
- `stale`: missing reliable published time, kept only if fetched within 72 hours with `time_confidence=low`.
- `outdated`: `published_at < now() - 72 hours`.

List APIs default to `fresh_only=true`; the UI Fresh only toggle is on by default. Retention is separate from freshness and keeps data for debugging/dedup context.

## LLM behavior

Default endpoint:

```text
LLM_BASE_URL=http://localhost:8317/v1
LLM_API_KEY=sk-my-key-is-empty
LLM_MODEL=gemini-3.1-flash-lite-preview
LLM_MAX_CONCURRENCY=3
```

Each unique, fresh, non-exact-duplicate article gets at most one successful LLM analysis (`article_llm_analysis.article_id` primary key and `llm_jobs.article_id` unique). Failed calls retry with backoff: 1m, 5m, 15m, 1h, then failed after max attempts. Articles remain visible if LLM fails.

## API endpoints

- `GET /api/health`
- `GET /api/stats`
- `GET /api/articles?fresh_only=true&page=1&page_size=50`
- `GET /api/articles/{id}`
- `GET /api/clusters`
- `GET /api/clusters/{id}`
- `GET /api/sources`
- `GET /api/jobs/llm`
- `POST /api/admin/crawl-once`
- `POST /api/admin/retry-failed-llm`

See `api/openapi.yaml` for the contract.

## Tests and checks

```bash
go test ./...
go vet ./...
go build ./cmd/finnews
cd web && npm install && npm run build
```

The test suite covers config validation, freshness-related defaults, URL canonicalization, title normalization, content hash stability, duplicate/update classification, near duplicate decisions, LLM JSON parsing (including fenced and malformed output), retry backoff, worker concurrency limit, API health/query defaults, RSS parsing, Federal Register URL construction, SEC form filtering, and cluster scoring.

## Troubleshooting

- If `make migrate` cannot connect, run `make db-up` and wait for the PostgreSQL health check.
- If LLM is unavailable, jobs stay pending/failed with retry metadata and articles remain visible.
- If a source changes format or fails, the source run is marked failed and other sources continue.
- Use Sources and LLM Queue tabs to inspect operational state.
