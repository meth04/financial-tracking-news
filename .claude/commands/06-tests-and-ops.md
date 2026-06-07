# /project:06-tests-and-ops

Add testing, operations, retention, and reliability hardening.

Read:

- `docs/OPERATIONS.md`
- `docs/ACCEPTANCE_CRITERIA.md`

## Tasks

1. Add unit tests for core backend modules.
2. Add integration tests for repositories if local PostgreSQL is available.
3. Add fixtures for RSS, SEC, Federal Register, LLM responses.
4. Add retention job:
   - raw items older than config raw_retention_days
   - articles older than config article_retention_days
   - clusters older than config cluster_retention_days
   - never delete rows needed by FK without cascade planning
5. Add source health computation:
   - last run
   - last success
   - error count
   - fetched count
   - avg latency if available
6. Add `/api/health` details.
7. Add scripts:
   - `scripts/dev-setup.sh`
   - `scripts/run-local.sh`
   - `scripts/backup-db.sh`
8. Add Dockerfile for app.
9. Update Docker Compose so app can run with postgres.
10. Improve README troubleshooting.

## Verify

Run:

```bash
go test ./...
go build ./cmd/finnews
cd web && npm run build
```
