# /project:02-crawler-sources

Implement crawler scheduler, source adapters, raw persistence, and normalization.

Read:

- `docs/SOURCE_POLICY.md`
- `config/sources.seed.yaml`
- `docs/ARCHITECTURE.md`

## Tasks

1. Define source adapter interface.
2. Implement RSS adapter using gofeed.
3. Implement generic HTML list adapter only for permitted public pages.
4. Implement Federal Register API adapter with publication date filters.
5. Implement SEC EDGAR watchlist adapter:
   - load ticker -> CIK mapping from SEC company_tickers URL
   - fetch `data.sec.gov/submissions/CIK##########.json`
   - filter forms configured in source config
   - only keep filings within 72h
   - create article title like `AAPL 8-K filed 2026-...`
6. Implement Treasury press releases adapter if generic HTML adapter is insufficient.
7. Implement BLS latest releases adapter if generic HTML adapter is insufficient.
8. Implement crawler service:
   - runs each enabled source
   - records source_runs
   - saves raw_items first
   - normalizes items into articles
   - applies 72h freshness gate
   - records source health
9. Implement normalizer:
   - URL canonicalization
   - title normalization
   - content text cleaning
   - hash calculation
   - word count
   - simhash calculation
10. Implement manual crawl endpoint to call crawler service asynchronously.

## Constraints

- Do not bypass paywalls or blocked pages.
- Respect `full_content_allowed`.
- Every source error must be isolated.
- Items older than 72h must not enqueue LLM.
- Missing published_at: allow only if fetched within 72h and mark low confidence.

## Tests

Add tests for:

- canonical URL stripping tracking params
- title normalization
- content hash stability
- freshness gate
- RSS item parsing using fixture XML
- Federal Register API URL construction
- SEC form filtering using fixture JSON

## Verify

Run:

```bash
go test ./...
make crawl-once
```
