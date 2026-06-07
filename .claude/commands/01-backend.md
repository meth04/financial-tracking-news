# /project:01-backend

Build the backend API, persistence layer, and domain models.

Read:

- `docs/DATA_MODEL.md`
- `db/schema.sql`
- `docs/API_CONTRACT.md`
- `api/openapi.yaml`

## Tasks

1. Implement Go domain models:
   - Source
   - SourceRun
   - RawItem
   - Article
   - ArticleDuplicate
   - LLMJob
   - ArticleLLMAnalysis
   - EventCluster
   - EventArticle
   - EventUpdate

2. Implement repository layer with pgx:
   - sources CRUD/list enabled
   - source run create/update
   - raw item insert idempotently
   - article insert/update/list/detail
   - duplicate relation insert
   - LLM job enqueue/pick/complete/fail
   - LLM analysis insert/get
   - cluster insert/update/list/detail
   - source health queries
   - stats queries

3. Implement API routes:
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

4. Implement filtering:
   - fresh_only default true
   - q search
   - source
   - ticker
   - event_type
   - impact
   - sentiment
   - status
   - pagination

5. Implement JSON response shapes that are frontend-friendly.

6. Add tests for:
   - freshness filters
   - repo insert idempotency where practical
   - API query parsing
   - health route

## Constraints

- API must not expose internal stack traces.
- Fresh-only must be default.
- Outdated articles must be hidden unless requested.

## Verify

Run:

```bash
go test ./...
go build ./cmd/finnews
```
