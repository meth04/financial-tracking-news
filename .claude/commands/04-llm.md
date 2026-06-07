# /project:04-llm

Implement LLM queue, client, prompt rendering, strict JSON parsing, and analysis persistence.

Read:

- `docs/LLM_CONTRACT.md`
- `prompts/article_analysis_system.md`
- `prompts/article_analysis_user_template.md`

## Tasks

1. Implement LLM client interface:

```go
type Analyzer interface {
    AnalyzeArticle(ctx context.Context, article Article) (*AnalysisResult, string, error)
}
```

The second return value is raw response text.

2. Implement OpenAI-compatible chat completions client:
   - base URL config
   - bearer API key
   - model config
   - timeout
   - response extraction

3. Implement prompt rendering from prompt files.

4. Implement strict parser:
   - extract JSON object even if accidentally wrapped in Markdown
   - validate required fields
   - normalize enum values
   - clamp scores 0-100
   - reject malformed JSON with useful error

5. Implement PostgreSQL-backed worker pool:
   - max concurrency = config `llm.max_concurrency`, default 3
   - job locking with `FOR UPDATE SKIP LOCKED`
   - heartbeat
   - retry backoff
   - max attempts
   - graceful shutdown

6. Enforce one-call rule:
   - do not enqueue if analysis exists
   - unique llm_jobs.article_id
   - before calling model, recheck analysis does not exist

7. On success:
   - save `article_llm_analysis`
   - update article status
   - invoke clusterer
   - mark job done

8. On failure:
   - store error
   - schedule retry
   - after max attempts mark failed

9. Expose queue status in API.

## Tests

- parser valid JSON
- parser Markdown fenced JSON
- parser malformed JSON
- score clamping
- enum normalization
- retry backoff schedule
- max concurrency with a fake analyzer
- one-call rule with existing analysis

## Verify

Run:

```bash
go test ./...
```

If local LLM is available, run a manual job. If not available, the app must fail gracefully and keep jobs pending/failed.
