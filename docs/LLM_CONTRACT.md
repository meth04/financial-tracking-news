# LLM Contract

## Endpoint

Default endpoint is OpenAI-compatible:

```text
Base URL: http://localhost:8317/v1
API key: sk-my-key-is-empty
Model: gemini-3.1-flash-lite-preview
```

Use chat completions first:

```http
POST /v1/chat/completions
Authorization: Bearer sk-my-key-is-empty
Content-Type: application/json
```

If that fails, keep the LLM client interface isolated so a Gemini-native transport can be added without touching the rest of the app.

## One-call rule

Each unique, fresh, non-exact-duplicate article gets exactly one LLM call.

Do not call LLM again for:

- re-rendering UI
- reclustering
- retry after successful parse
- extracting another field later

If fields need changing later, add a schema version and explicit migration command, not automatic re-calls.

## Concurrency rule

At most 3 concurrent LLM calls globally.

Implementation:

- Worker pool size = `LLM_MAX_CONCURRENCY`.
- Job lock with `FOR UPDATE SKIP LOCKED`.
- Each worker processes one job at a time.

## Retry rule

Retry failed calls with exponential backoff:

```text
attempt 1 -> immediate
attempt 2 -> +1 minute
attempt 3 -> +5 minutes
attempt 4 -> +15 minutes
attempt 5 -> +1 hour
then failed
```

Retry on:

- network error
- timeout
- HTTP 429/500/502/503/504
- invalid JSON from model

Do not retry on:

- article missing
- article outdated before first processing
- article already has analysis

## Required JSON output

The model must output strict JSON only, no Markdown.

```json
{
  "schema_version": "1.0",
  "importance_score": 0,
  "market_impact": "low",
  "novelty_score": 0,
  "confidence": 0,
  "summary_vi": "",
  "summary_en": "",
  "event_title": "",
  "event_type": "other",
  "affected_tickers": [],
  "affected_companies": [],
  "affected_sectors": [],
  "affected_assets": [],
  "countries": [],
  "key_facts": [],
  "new_information": [],
  "risk_flags": [],
  "sentiment": "neutral",
  "time_sensitivity": "today",
  "dedup_event_key": ""
}
```

## Field rules

### importance_score

Integer 0-100.

- 0-20: low relevance
- 21-40: minor market relevance
- 41-60: relevant
- 61-80: important
- 81-100: critical/market-moving

### market_impact

One of:

- `low`
- `medium`
- `high`
- `critical`

### novelty_score

Integer 0-100. Measures how likely this article contains new information compared with typical repeated coverage. Since LLM does not see all previous articles, it should infer novelty from wording such as “new”, “updated”, “confirmed”, “reported”, “filed”, “announced”, “revised”. The clusterer will combine this with similarity signals.

### confidence

Integer 0-100. Model confidence in extraction.

### event_type

One of:

- `macro`
- `fed`
- `earnings`
- `guidance`
- `mna`
- `ipo`
- `sec_filing`
- `regulation`
- `lawsuit`
- `analyst_rating`
- `commodity`
- `crypto`
- `forex`
- `rates`
- `labor_market`
- `inflation`
- `geopolitical`
- `company_news`
- `market_move`
- `other`

### affected_tickers

Use U.S. tickers when confidently identifiable. Do not invent tickers. If uncertain, leave empty and add company name to `affected_companies`.

### key_facts

Array of short factual strings. Include numbers and dates when present.

### new_information

Array of facts that appear like updates/new developments.

### dedup_event_key

Stable lowercase key. Use pattern:

```text
<event_type>:<primary_entity_or_asset>:<short_event_slug>
```

Examples:

```text
fed:rate-decision:fomc-holds-rates
sec_filing:aapl:8k-guidance-update
earnings:nvda:q2-results
macro:us-cpi:inflation-report
```

## Prompt files

Use:

- `prompts/article_analysis_system.md`
- `prompts/article_analysis_user_template.md`

Render user template with article fields.

## Parser requirements

The parser must:

1. Extract JSON even if model wraps it accidentally in Markdown fences.
2. Validate required fields.
3. Clamp numeric scores to 0-100.
4. Normalize enum values.
5. Reject unsafe or malformed output and retry job.
6. Store raw model response in job error metadata if parsing fails.
