# Acceptance Criteria

Claude Code should consider the project complete when every item below is true.

## Build and run

- `go test ./...` passes.
- `go vet ./...` passes or documented exceptions exist.
- `make db-up` starts PostgreSQL.
- `make migrate` creates tables.
- `make seed` inserts sources from YAML.
- `make run` starts HTTP API.
- Frontend starts with `cd web && npm run dev`.

## Crawler

- Config-driven sources load from YAML.
- Scheduler runs every 10 minutes by default.
- Manual crawl endpoint works.
- Source failures do not crash whole app.
- Raw item is saved before processing.
- Items older than 72h are marked outdated or skipped from LLM.

## Dedup

- Canonical URL duplicate is detected.
- Content hash duplicate is detected.
- Similar title/content candidates are evaluated.
- Same-source updated article is not blindly discarded.
- Duplicate relationships are visible in article detail.

## LLM

- LLM jobs are persisted.
- Max concurrency is 3.
- Each article has at most one successful LLM analysis.
- Failed calls retry with backoff.
- Invalid JSON triggers retry.
- Raw JSON is stored.
- UI shows pending/running/failed/done counts.

## Clustering

- Articles with same event key are clustered.
- Related articles can attach to same event.
- New facts create event updates.
- Cluster importance updates when important update appears.
- Event detail shows timeline.

## API

- `/api/health` returns healthy status.
- `/api/stats` powers dashboard stats.
- `/api/articles` supports filters and fresh-only default.
- `/api/clusters` supports filters and fresh-only default.
- `/api/sources` shows source health.
- `/api/jobs/llm` shows queue state.

## UI

- Dark compact dashboard resembles `design/ui-reference.png` style.
- Main pages: Articles, Events, Sources, LLM Queue, Settings.
- Fresh-only filter is default on.
- Outdated news is hidden by default.
- Tables are dense and readable.
- Badges show impact/event type/status.
- Detail drawer shows LLM analysis and event relationships.

## Documentation

- README has setup instructions.
- README documents source policy and paywall restrictions.
- README documents LLM local endpoint.
- README documents freshness/outdated behavior.
