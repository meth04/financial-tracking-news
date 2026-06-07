# API Contract

Base path: `/api`

See `api/openapi.yaml` for machine-readable contract.

## Defaults

- All list endpoints default to fresh-only data (`fresh_only=true`).
- Default page size: 50.
- Max page size: 200.
- Sorting defaults to newest or most important depending on endpoint.

## Important endpoints

### GET /api/health

Returns service health, DB status, scheduler status, worker status.

### GET /api/stats

Returns dashboard counters:

- fresh article count
- active cluster count
- high/critical impact count
- pending LLM jobs
- failed source count
- newest fetched time

### GET /api/articles

Query params:

- `q`
- `source`
- `ticker`
- `event_type`
- `impact`
- `sentiment`
- `fresh_only` default true
- `status`
- `page`
- `page_size`
- `sort`: `published_at`, `importance`, `source`, `ticker`
- `order`: `asc`, `desc`

### GET /api/articles/{id}

Returns article detail, LLM analysis, duplicate relation, cluster relation.

### GET /api/clusters

Query params:

- `q`
- `ticker`
- `event_type`
- `impact_min`
- `fresh_only`
- `page`
- `page_size`
- `sort`: `last_updated_at`, `importance`, `article_count`, `update_count`

### GET /api/clusters/{id}

Returns event cluster detail, articles, updates timeline.

### GET /api/sources

Returns source list with health stats.

### GET /api/jobs/llm

Returns LLM queue state.

### POST /api/admin/crawl-once

Triggers immediate crawl. Optional body:

```json
{"source_key":"fed_press_all"}
```

### POST /api/admin/retry-failed-llm

Resets failed LLM jobs to pending.
