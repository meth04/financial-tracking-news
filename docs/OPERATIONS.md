# Operations

## Local setup

1. Copy `.env.example` to `.env`.
2. Start PostgreSQL:

```bash
make db-up
```

3. Run migrations:

```bash
make migrate
```

4. Seed sources:

```bash
make seed
```

5. Start backend:

```bash
make run
```

6. Start worker if not embedded in server:

```bash
make worker
```

7. Start frontend:

```bash
make frontend-dev
```

## Scheduled crawling

Default: every 10 minutes.

Run immediately:

```bash
make crawl-once
```

## Logs

Use structured logs with component names:

- `crawler`
- `source`
- `normalizer`
- `dedup`
- `llm_worker`
- `clusterer`
- `api`

Every log should include source key/article ID/job ID when relevant.

## Health checks

`GET /api/health` should report:

- app status
- database status
- scheduler status
- LLM worker status
- last successful crawl
- source failure count

## Backups

For local PostgreSQL:

```bash
./scripts/backup-db.sh
```

## Retention

Default retention:

- raw items: 7 days
- articles: 14 days
- clusters: 30 days

Main UI hides outdated >72h. Retention is for debugging and dedup context, not for normal reading.

## Failure handling

### Source down

- Mark latest `source_runs.status=failed`.
- Record error.
- Continue other sources.
- UI Sources tab shows failure.

### LLM down

- Jobs retry.
- Articles remain visible as `llm_pending` or `llm_failed`.
- UI shows queue state.

### DB down

- App startup should fail fast.
- Runtime DB failures should log and retry according to component.

## Source maintenance

Sources can change feed URLs or page structure. Keep adapters isolated and source health visible.
