# Project Spec — Financial News Intelligence Crawler

## Product goal

Build a local-first Go application that collects fresh U.S. financial news from trustworthy free sources, removes duplicates, groups articles into market events, detects event updates, and scores market importance through a local LLM endpoint.

The product is not just a crawler. It is a news intelligence dashboard.

## User goals

The user wants to:

- See only fresh financial news, not old archives.
- Avoid reading the same event repeated by many outlets.
- Know which news matters most.
- See when an existing event receives a meaningful new development.
- Filter by source, ticker, sector, event type, impact, sentiment, and freshness.
- Run everything locally on a personal machine.

## Freshness policy

- Default freshness window: 72 hours.
- News older than 72 hours is outdated.
- Outdated news should not appear in the main dashboard by default.
- Source adapters must reject known-old items early.
- If an item lacks a reliable `published_at`, use `fetched_at` and mark time confidence as low.
- Cluster activity is based on the freshest article/update in the cluster.

## Main entities

### Source

A configured data provider, such as an RSS feed, official API, or permitted public HTML page.

### Raw Item

The unmodified fetched payload. This is persisted first so parser/LLM failures do not lose data.

### Article

A normalized piece of news with title, URL, source, published time, content/excerpt, hashes, and status.

### LLM Analysis

One structured JSON analysis per unique article.

### Event Cluster

A group of articles about the same market-moving event.

### Event Update

A meaningful new fact or development inside an existing event cluster.

## Out of scope for MVP

- Paid APIs.
- Trading execution.
- User accounts.
- Mobile app.
- Breaking paywalls.
- Historical archive older than 72 hours.
- Complex vector database unless needed later.

## MVP feature list

1. Config-driven source registry.
2. 10-minute crawler scheduler.
3. Raw item persistence.
4. Normalization and content extraction.
5. Freshness filtering.
6. Dedup by URL/hash/title/content.
7. Near duplicate detection.
8. LLM queue with max concurrency 3.
9. LLM JSON analysis saved to DB.
10. Event clustering and update detection.
11. REST API.
12. Dark dashboard UI.
13. Source health dashboard.
14. LLM queue dashboard.
15. Logs and basic metrics.

## Quality bar

- Data loss resistant.
- Idempotent processing.
- Clear error handling.
- Easy to add a source adapter.
- Easy to inspect why an article was deduped or clustered.
- Tests cover core logic.
