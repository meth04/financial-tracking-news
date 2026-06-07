# Source Policy

## Principle

Use free, reputable, legally safer sources. Prefer official API/RSS feeds and public government/company releases. Do not bypass paywalls, login walls, CAPTCHAs, bot protection, or terms of service.

## Source tiers

### Tier 1 — official/government sources

These are highest priority because they are primary sources.

- SEC EDGAR filings and company submissions.
- Federal Reserve RSS feeds for press releases, monetary policy, speeches/testimony.
- BEA API/RSS/news releases.
- BLS public data/release pages.
- U.S. Treasury press releases and fiscal data APIs.
- Federal Register API for regulatory actions.
- CFTC, FDIC, OCC, CFPB, FTC, DOJ antitrust where financially relevant.

### Tier 2 — exchange/company/issuer sources

- NYSE/Nasdaq notices and market status pages.
- Company investor relations RSS feeds when available.
- Press release wires with public RSS/search pages, where permitted.

### Tier 3 — publisher RSS/free pages

- Use only if feed/page is public and terms allow automated access.
- If full content is not clearly allowed, store title/excerpt/link only.
- Do not attempt paywall extraction.

## Full content policy

`full_content_allowed=true` only when:

1. The source is official/public-domain-ish government content, or
2. The source provides full article content in RSS/API, or
3. The source explicitly permits crawling/republishing for this local use.

Otherwise:

- Store metadata, title, excerpt, URL, source, published_at.
- Do not scrape hidden/paywalled body.

## Freshness policy per source

Every source config must include:

```yaml
max_age_hours: 72
```

Adapters must request only fresh items when the API supports date filters. If not supported, adapters must discard older items after parsing published time.

## Recommended MVP seed sources

Use `config/sources.seed.yaml`. Start with official RSS/API sources.

### Federal Reserve

Use RSS feeds listed by the Federal Reserve RSS page. Useful feeds include:

- all press releases
- monetary policy press releases
- speeches
- testimony

### SEC EDGAR

Use official SEC JSON APIs:

- company tickers list
- submissions by CIK
- recent filings for 8-K, 10-Q, 10-K, 6-K, 20-F, S-1 and amendments

For MVP, poll a configured watchlist of major tickers instead of the entire market.

### BEA

Use BEA news release RSS and API metadata. Treat BEA releases as high-confidence macro events.

### BLS

Use public data API and release pages. Treat CPI, PPI, jobs, wages, productivity, and unemployment releases as high-impact candidates.

### Federal Register

Use API with publication date filters and agency/topic filters. Relevant agencies include SEC, CFTC, Treasury, Federal Reserve, FTC, DOJ, CFPB, FDIC, OCC.

### Treasury

Use Treasury press releases and fiscal data APIs where relevant. Focus on sanctions, rates, auctions, debt, refunding, fiscal data, and financial stability.

## Source config fields

```yaml
- key: fed_press_all
  name: Federal Reserve - All Press Releases
  type: rss
  url: https://www.federalreserve.gov/feeds/press_all.xml
  credibility_score: 100
  enabled: true
  full_content_allowed: true
  crawl_interval_minutes: 10
  max_age_hours: 72
  rate_limit_per_minute: 20
  respect_robots: true
```

## Source adapter behavior

Each adapter must:

1. Fetch with configured timeout and User-Agent.
2. Respect rate limits.
3. Parse published time.
4. Discard or mark outdated items.
5. Persist raw item first.
6. Return source health metrics.
7. Continue on item-level errors.

## Robots and ToS

For HTML adapters:

- Check robots.txt if `respect_robots=true`.
- Do not fetch disallowed paths.
- Use conservative rate limits.
- Do not mimic browsers to bypass restrictions.
- Do not rotate IPs/proxies to evade blocks.

## Practical quality scoring

`credibility_score` is source-level trust:

- 100: primary official source.
- 90-99: official exchange/regulator or direct company IR.
- 80-89: established wire/publisher feed with clear access.
- 70-79: reputable but less direct.
- below 70: generally avoid for MVP.
