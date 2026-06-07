# /project:00-bootstrap

You are Claude Code. Bootstrap the repository for the Financial News Intelligence Crawler.

Read first:

- `CLAUDE.md`
- `docs/PROJECT_SPEC.md`
- `docs/ARCHITECTURE.md`
- `docs/DATA_MODEL.md`
- `docs/SOURCE_POLICY.md`
- `docs/UI_STYLE_GUIDE.md`

## Tasks

1. Inspect current repository contents.
2. Initialize a Go module if missing.
3. Create the target directory structure:

```text
cmd/finnews
internal/config
internal/db
internal/source
internal/source/rss
internal/source/sec
internal/source/federalreg
internal/crawler
internal/normalize
internal/dedup
internal/cluster
internal/llm
internal/jobs
internal/api
internal/ops
web
migrations
```

4. Choose libraries consistent with `CLAUDE.md`.
5. Create basic CLI with subcommands:

```bash
finnews server
finnews worker
finnews crawl once
finnews migrate up
finnews seed sources
```

6. Implement config loading from YAML and env overrides.
7. Implement database connection using pgxpool.
8. Add migration runner that can execute `db/schema.sql` or migration files.
9. Add source seed command for `config/sources.seed.yaml`.
10. Add structured logging with `log/slog`.
11. Add `.gitignore` suitable for Go + Node + local env.
12. Add/update README with actual local setup commands.

## Constraints

- Do not hardcode secrets except `.env.example` placeholder.
- Do not implement crawler logic yet except stubs.
- Make sure project compiles.

## Verify

Run:

```bash
go mod tidy
go test ./...
go build ./cmd/finnews
```
