# Architecture

## High-level pipeline

```text
Scheduler
  -> Source Adapter
  -> Fetcher
  -> Raw Store
  -> Normalizer
  -> Freshness Gate
  -> Dedup Engine
  -> Article Store
  -> LLM Job Queue
  -> LLM Analysis Store
  -> Event Clusterer
  -> API
  -> UI
```

## Runtime processes

The app can run as one binary with subcommands:

```bash
finnews server
finnews worker
finnews crawl once
finnews migrate up
finnews seed sources
```

For MVP, `server` may also start scheduler and workers in goroutines, controlled by config. Keep subcommands available for debugging.

## Components

### Config

Loads YAML + env overrides. Validate required fields on startup.

### Source adapters

Interface:

```go
type Adapter interface {
    Name() string
    Fetch(ctx context.Context, since time.Time) ([]FetchedItem, error)
}
```

`FetchedItem` should support RSS/API/HTML outputs:

```go
type FetchedItem struct {
    SourceKey string
    RawURL string
    CanonicalURL string
    Title string
    Excerpt string
    ContentHTML string
    ContentText string
    Author string
    PublishedAt *time.Time
    FetchedAt time.Time
    RawPayload []byte
    ContentType string
    Metadata map[string]any
}
```

### Raw store

Every fetched item is inserted into `raw_items` before parsing-heavy processing. Use a raw hash to deduplicate raw payloads.

### Normalizer

Responsibilities:

- canonicalize URLs
- strip tracking params
- normalize title whitespace
- convert HTML to readable text where allowed
- calculate hashes
- calculate SimHash or shingles
- set time confidence

### Freshness gate

Reject or mark old items:

- If `published_at` exists and older than 72h: mark `outdated`; do not enqueue LLM.
- If missing `published_at`: allow only if fetched within 72h, mark `time_confidence=low`.

### Dedup engine

Responsibilities:

- exact duplicate detection
- near duplicate candidate search
- relation decision: duplicate/update/related/new

### LLM queue

Use PostgreSQL-backed jobs for simplicity and durability.

Query pattern:

```sql
SELECT * FROM llm_jobs
WHERE status = 'pending' AND next_run_at <= now()
ORDER BY priority DESC, created_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

### Clusterer

Runs after LLM analysis. It can also run periodically to repair clusters.

Decision inputs:

- LLM `dedup_event_key`
- event_type
- affected tickers/assets/sectors
- key facts
- title similarity
- content similarity
- source and time window

### API

Go HTTP API using chi.

### UI

React TypeScript dashboard consuming REST API.

## Idempotency

Every processing step must be safe to rerun. Use unique constraints and upserts:

- sources.key unique
- raw_items.raw_hash unique per source
- articles.canonical_url unique when present
- articles.content_hash unique-ish but do not globally reject if source/time differs; use dedup relation instead
- article_llm_analysis.article_id unique
- llm_jobs.article_id unique

## Error handling

A source failure must not stop other sources.

A parse failure must keep the raw item.

An LLM failure must keep the article and retry the job.

A cluster failure must not hide the article; show it as unclustered.

## Local deployment

Use Docker Compose for PostgreSQL. The Go app can run directly or in Docker.
