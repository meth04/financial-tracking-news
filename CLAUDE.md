# CLAUDE.md — Build Instructions for Financial News Intelligence Crawler

You are Claude Code working inside this repository. Your task is to build a production-quality local application in Go that collects fresh U.S. financial news, deduplicates and clusters it, scores importance with an LLM, and presents it in a dark dense dashboard inspired by `design/ui-reference.png`.

## Non-negotiable product requirements

1. Backend must be written in Go.
2. The application must run locally on a personal machine.
3. Use PostgreSQL as the primary datastore.
4. Crawl cadence defaults to every 10 minutes.
5. Freshness window is 72 hours. Anything older than 72 hours is `outdated`.
6. Do not crawl historical backfills older than 72 hours for MVP.
7. UI must default-hide outdated articles/clusters.
8. Always persist raw fetched items before normalization/dedup/LLM.
9. Do not lose articles because of parser, dedup, or LLM errors.
10. Each unique article may call the LLM exactly once. Store the raw LLM JSON output forever within retention.
11. Max concurrent LLM calls is 3.
12. LLM errors must retry with exponential backoff.
13. Do not bypass paywalls, login walls, CAPTCHAs, bot protection, or website ToS.
14. Prefer official API/RSS/free public feeds. HTML scraping is allowed only for public pages where robots/ToS permit it.
15. Build readable, testable, maintainable code with clear logs and metrics.

## LLM configuration

Default LLM config:

```env
LLM_BASE_URL=http://localhost:8317/v1
LLM_API_KEY=sk-my-key-is-empty
LLM_MODEL=gemini-3.1-flash-lite-preview
LLM_MAX_CONCURRENCY=3
```

Assume the local service is OpenAI-compatible unless runtime testing proves otherwise. Implement the LLM client behind an interface so the endpoint format can be adjusted.

## Definition of fresh/outdated

- `fresh`: `published_at >= now() - 72 hours`.
- `stale`: fetched but missing reliable published time; keep, but mark `time_confidence=low` and show only if fetched within 72 hours.
- `outdated`: `published_at < now() - 72 hours`.
- API responses must default to `fresh_only=true`.
- UI must have a small toggle to include outdated data for debugging, but default off.

## Architecture to implement

Use this shape unless there is a strong technical reason to adjust:

```text
cmd/finnews                 CLI entrypoint
internal/config             config loading/validation
internal/db                 pgx pool, migrations helpers, queries
internal/source             source adapter interfaces
internal/source/rss         RSS adapter
internal/source/sec         SEC adapter
internal/source/fed         Fed RSS adapter if needed
internal/source/federalreg  Federal Register API adapter
internal/crawler            scheduler, fetcher, raw persistence
internal/normalize          canonical URL, content extraction, text cleaning
internal/dedup              exact/near duplicate logic
internal/cluster            event clustering and update detection
internal/llm                LLM client, prompt builder, parser
internal/jobs               PostgreSQL-backed job queue
internal/api                HTTP API server
internal/web                optional embedded static frontend
web                         React + TypeScript frontend
config                      YAML source config
migrations or db            SQL migrations/schema
```

## Recommended Go libraries

- HTTP router: `github.com/go-chi/chi/v5`
- Database: `github.com/jackc/pgx/v5/pgxpool`
- Migrations: `github.com/golang-migrate/migrate/v4` or a small internal migrator using SQL files
- RSS parsing: `github.com/mmcdole/gofeed`
- HTML parsing: `github.com/PuerkitoBio/goquery`
- Logging: standard `log/slog` preferred
- UUID: `github.com/google/uuid`
- YAML: `gopkg.in/yaml.v3`

Do not over-engineer. Prefer clear code over clever abstractions.

## Frontend requirements

Use Vite + React + TypeScript unless already initialized differently.

The UI style must follow `docs/UI_STYLE_GUIDE.md`:

- Black/dark background.
- Compact table layout.
- Top stat pills.
- Tabs and filters.
- Dense rows.
- Badges for source, impact, event type, status.
- Pagination/page size controls.
- Green upward change text for fresh/new/impact deltas.
- Purple/blue category pills.
- Source-health and LLM queue views.

## Dedup/clustering requirement

Never implement only a naive duplicate filter. Implement these layers:

1. Exact duplicate: canonical URL, title hash, normalized content hash.
2. Near duplicate: SimHash or shingled Jaccard/MinHash-style approximation.
3. Event clustering: group multiple articles about the same market event.
4. Update detection: if a new article belongs to an existing event but has new facts, mark relation as `update`, not `duplicate`.

See `docs/DEDUP_CLUSTERING.md`.

## LLM one-call rule

For every normalized article that is not an exact duplicate, create one `llm_jobs` row.

The job worker must:

- Lock pending jobs using `FOR UPDATE SKIP LOCKED`.
- Respect `LLM_MAX_CONCURRENCY=3`.
- Mark job `running` with a heartbeat.
- On success: save `article_llm_analysis.raw_json`, parsed fields, and mark job `done`.
- On failure: increase attempts, set `next_run_at` using backoff, store error.
- After max attempts: mark `failed`, but keep the article visible with `analysis_status=failed`.

Never call the LLM inside a tight loop without queue control.

## Source policy

Implement source adapters from `config/sources.seed.yaml`. Each source must declare:

- name
- type: `rss`, `api`, or `html`
- credibility_score
- full_content_allowed
- enabled
- crawl_interval_minutes
- max_age_hours
- rate_limit
- respect_robots

If a source disallows full content crawling, store title, excerpt, canonical URL, source, and published_at only.

## API requirements

Implement `api/openapi.yaml` endpoints:

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

## Testing requirement

Create tests for:

- config validation
- freshness filtering
- URL canonicalization
- content hash stability
- exact duplicate detection
- near duplicate detection
- event update classification
- LLM JSON parser with malformed JSON
- job retry/backoff
- API list filters

## Completion criteria

You are done only when:

1. `make db-up` works.
2. `make migrate` works.
3. `make seed` works.
4. `make run` starts backend.
5. `make crawl-once` fetches at least seeded RSS/API sources or gracefully reports no fresh items.
6. LLM worker runs with max 3 concurrency.
7. UI renders a dashboard matching the dark compact style.
8. `make test` passes.
9. README explains local setup, config, source policy, and LLM endpoint.
10. No secrets are hardcoded except the requested local placeholder key in `.env.example`.

## Development process

Work in small steps. After each major feature, run tests or at least compile. If a source fails due to network or changed format, do not block the whole app; log source health and continue.
