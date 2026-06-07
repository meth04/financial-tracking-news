# Deduplication, Clustering, and Update Detection

## Core idea

Do not treat deduplication as a binary delete/keep decision. Financial news often repeats across outlets, but sometimes a repeated story contains a new fact. The system must classify relationships.

## Relationship types

### exact duplicate

Same URL, same content hash, or same normalized title from the same source in a short time window.

Action:

- Do not enqueue another LLM job.
- Link to canonical article via `article_duplicates`.
- Store raw item anyway.

### near duplicate

Different URL/source but highly similar article, no new facts.

Action:

- Do not create a separate event update.
- Attach article to same event with relation `duplicate`.
- It may still be shown inside cluster sources list.

### related

Same event, different angle or context, but no direct new market-moving fact.

Action:

- Attach to cluster with relation `related`.

### update

Same event, but article adds a meaningful new fact, number, quote, filing, agency decision, earnings guidance, market reaction, or official confirmation/denial.

Action:

- Attach to cluster with relation `update`.
- Create `event_updates` row.
- Refresh `event_clusters.last_updated_at`.
- Recalculate cluster importance.

### new event

Article does not fit an active cluster.

Action:

- Create new event cluster.

## Exact duplicate algorithm

Normalize URL:

- lowercase scheme/host
- remove fragments
- remove common tracking query params: `utm_*`, `fbclid`, `gclid`, `mc_cid`, `mc_eid`, `cmpid`, `source`
- normalize trailing slash

Normalize title:

- lowercase
- collapse whitespace
- remove repeated punctuation
- trim source suffixes like `| Reuters`, `- CNBC` when detected

Normalize content:

- strip boilerplate
- collapse whitespace
- remove share/cookie text

Hashes:

- `title_hash = sha256(normalized_title)`
- `content_hash = sha256(normalized_content_text)` if content length >= 200 chars

Decision:

```text
if canonical_url matches existing article -> exact duplicate
else if content_hash matches existing article -> exact duplicate
else if same source + title_hash matches within 72h -> exact duplicate or source update candidate
```

Important exception:

If same source + same title appears later with different content hash and a higher word count/new published time, treat as `candidate_update`, not immediate duplicate.

## Near duplicate algorithm

MVP options:

1. Implement SimHash over normalized content tokens.
2. Implement shingled Jaccard over 5-word shingles.
3. Use PostgreSQL trigram title similarity as a cheap candidate selector.

Recommended MVP:

- Use pg_trgm on normalized title to find candidates.
- Use SimHash Hamming distance on content.
- Use simple key-fact comparison after LLM analysis.

Candidate selection:

```sql
SELECT id, normalized_title, simhash, published_at
FROM articles
WHERE published_at >= now() - interval '72 hours'
  AND id <> $1
  AND similarity(normalized_title, $2) > 0.35
ORDER BY published_at DESC
LIMIT 50;
```

Similarity thresholds:

```text
title similarity >= 0.92 -> likely duplicate
content simhash hamming distance <= 3 -> likely duplicate
content simhash hamming distance <= 8 -> candidate same event
same ticker/event_type + title similarity >= 0.65 -> candidate same event
```

Do not rely on one threshold. Combine signals.

## Event clustering algorithm

After LLM analysis exists, build an `EventSignature`:

```go
type EventSignature struct {
    EventType string
    EventTitle string
    DedupEventKey string
    Tickers []string
    Assets []string
    Sectors []string
    KeyFacts []string
    PublishedAt time.Time
    SourceCredibility int
}
```

Find candidate clusters:

1. Same `dedup_event_key` within 72h.
2. Same primary ticker + event_type within 72h.
3. Similar event title within 72h.
4. Macro/fed/regulation events: same event_type + overlapping key facts/assets.

Score candidate cluster:

```text
score = 0
+40 if dedup_event_key exact match
+20 if event_type match
+20 if primary ticker/asset overlap
+10 if sector overlap
+20 if event_title similarity high
+20 if key_facts overlap
-20 if time gap > 36h and no continuing update signal
```

Decision:

```text
score >= 70 -> same event
score 50-69 -> same event candidate; use stricter update/duplicate logic
score < 50 -> new event
```

## Update detection

For same-event candidates, decide duplicate vs related vs update.

Signals for `update`:

- new numeric data: EPS, revenue, CPI, jobs, yield, guidance, vote count, deal value
- new official action: filed, approved, rejected, sued, fined, sanctioned
- new named entity: company, regulator, executive, country, agency
- new timestamp/status: preliminary -> confirmed, rumor -> official, pending -> completed
- material price/market reaction
- LLM `new_information` is non-empty and `novelty_score >= 35`

Signals for `duplicate`:

- same facts, same numbers, same quote, different wording
- novelty_score < 20
- high content similarity and no new facts

Signals for `related`:

- same broad event but article is analysis/commentary/background
- novelty_score 20-34
- no new official fact

Pseudo decision:

```go
if exactDuplicate { return duplicate }
if sameEvent {
    if noveltyScore >= 35 && hasNewFacts { return update }
    if contentSimilarity >= 0.90 && !hasNewFacts { return duplicate }
    return related
}
return newEvent
```

## Handling same source repost/update

Case: Source A posts X at T, then Source A posts X at T+1 with new detail.

Rules:

1. Same source + same canonical URL:
   - If content hash changed and published/modified time changed, keep new raw version.
   - Create new article version or update article content history.
   - For MVP, create a new article row with canonical URL version suffix in metadata if unique constraint blocks insertion, then link via `article_duplicates` or `event_articles`.
2. Same source + new URL + similar title:
   - Run near duplicate and LLM analysis.
   - If LLM reports new facts, classify as `update`.
3. Never discard same-source similar article before checking for new facts when it is later than the previous article.

## MVP simplification

If versioning is too much for first pass:

- Keep raw item always.
- For canonical URL conflict, update existing article's `updated_at`, but also create an `article_versions` table or store old/new raw IDs in metadata.
- Still create an `event_update` if new facts are detected.

Prefer implementing `article_versions` in code if time allows; schema can be extended.
