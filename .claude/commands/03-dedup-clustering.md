# /project:03-dedup-clustering

Implement deduplication, near-duplicate detection, event clustering, and update detection.

Read:

- `docs/DEDUP_CLUSTERING.md`
- `docs/DATA_MODEL.md`

## Tasks

1. Implement exact duplicate checks:
   - canonical URL
   - content hash
   - same source + title hash within 72h

2. Implement special same-source update candidate logic:
   - same/similar title but newer published time and different content hash must not be blindly discarded.
   - mark as candidate for LLM and event update detection.

3. Implement near duplicate candidate search:
   - pg_trgm title similarity
   - simhash hamming distance
   - content length/word count sanity checks

4. Implement dedup decision function:

```go
type DedupDecision struct {
    Kind string // new, exact_duplicate, near_duplicate, candidate_update
    DuplicateOf *uuid.UUID
    Similarity float64
    Reason string
}
```

5. Integrate dedup into crawler pipeline before LLM queue.

6. Implement clusterer after LLM analysis:
   - build event signature
   - find candidate clusters within 72h
   - score candidates
   - create new cluster or attach article
   - classify relation as original, duplicate, related, update
   - create event_updates when needed

7. Update cluster aggregate fields:
   - article_count
   - source_count
   - update_count
   - importance_score
   - novelty_score
   - last_seen_at
   - last_updated_at
   - affected_tickers/sectors/assets

8. Add a repair command:

```bash
finnews cluster repair --fresh-only
```

## Tests

Create tests for:

- exact duplicate by URL
- exact duplicate by content hash
- near duplicate by similar content
- same-source T+1 update candidate
- event signature matching
- update vs duplicate classification

## Verify

Run:

```bash
go test ./...
```
