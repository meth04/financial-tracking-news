# Data Model

This document describes the target PostgreSQL data model. `db/schema.sql` is the executable version.

## Table overview

- `sources`: configured data sources.
- `source_runs`: every crawl run per source.
- `raw_items`: raw fetched payloads.
- `articles`: normalized news articles.
- `article_duplicates`: exact/near duplicate relationships.
- `llm_jobs`: durable LLM queue.
- `article_llm_analysis`: one LLM result per article.
- `event_clusters`: market event groups.
- `event_articles`: article-to-cluster relationship.
- `event_updates`: meaningful new developments in a cluster.
- `app_logs`: optional DB-backed important logs.

## Key constraints

- One source key is unique.
- One LLM analysis per article.
- One LLM job per article.
- Raw items are never deleted immediately after failure.
- Articles can be outdated, but default queries hide them.

## Important status fields

### articles.status

- `new`: inserted, not fully processed.
- `normalized`: normalized and fresh.
- `duplicate`: exact/near duplicate of another article.
- `llm_pending`: queued for LLM.
- `llm_done`: LLM analysis exists.
- `llm_failed`: LLM failed after retries.
- `clustered`: attached to event cluster.
- `outdated`: older than 72 hours.
- `error`: processing error but not deleted.

### event_articles.relation

- `original`: first or canonical article for cluster.
- `duplicate`: no meaningful new facts.
- `related`: same event, different angle.
- `update`: same event with meaningful new facts.

### llm_jobs.status

- `pending`
- `running`
- `done`
- `failed`

## Freshness fields

Use both `published_at` and `fetched_at`.

- `published_at`: source-provided date, if reliable.
- `fetched_at`: time our app fetched it.
- `time_confidence`: `high`, `medium`, `low`.
- `is_outdated`: generated or stored boolean based on max age.

## Index strategy

Add indexes for:

- articles published_at desc
- articles source_id
- articles status
- articles content_hash
- articles simhash
- clusters last_updated_at desc
- clusters importance_score desc
- jobs status + next_run_at
- GIN indexes on JSONB/ticker arrays if implemented with JSONB

For MVP, arrays can be `text[]`; JSON fields can be `jsonb`.
