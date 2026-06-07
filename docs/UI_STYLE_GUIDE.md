# UI Style Guide — Dark Dense Financial News Dashboard

The user wants a UI style similar to `design/ui-reference.png`.

## Visual mood

- Dark, dense, technical, data-table centric.
- Similar feel to developer/index dashboards.
- Minimal decoration, high information density.
- Clear colored badges for categories/status.
- Compact controls and table rows.

## Layout

### Top header

Height around 88-110px.

Left:

- Product logo/title: `FinNewsIntel` or `MarketSignal`
- Subtitle: `Fresh U.S. financial news intelligence · updated every 10m`

Right stat pills:

- Fresh articles count
- Active events count
- High impact count
- LLM pending count
- Source errors count

Stat pills should look like compact rounded dark cards.

### Tab nav

Horizontal tabs:

- Articles
- Events
- Sources
- LLM Queue
- Settings

Active tab has thin cyan underline.

### Filter bar

One row, compact:

- Search input
- Event type select
- Impact select
- Source select
- Ticker input
- `Fresh only` checkbox, default true
- Pagination controls on right
- Page size select

### Main table

Dense table similar to screenshot.

For Articles page columns:

1. row number
2. Source
3. Published
4. Age
5. Impact
6. Novelty
7. Sentiment
8. Title / summary
9. Event Type
10. Tickers
11. Cluster
12. Status

For Events page columns:

1. row number
2. Event
3. Impact
4. Updated
5. Articles
6. Updates
7. Sources
8. Event Type
9. Tickers/Sectors
10. Sentiment

Rows should be clickable.

## Design tokens

Use CSS variables:

```css
:root {
  --bg: #0b0d0f;
  --panel: #101316;
  --panel-2: #15191d;
  --border: #24292f;
  --border-soft: #1b2025;
  --text: #e6edf3;
  --text-muted: #8b949e;
  --text-soft: #b7c0ca;
  --accent: #66d9ef;
  --accent-2: #7c5cff;
  --positive: #2ecc71;
  --negative: #ff5f56;
  --warning: #f5c542;
  --critical: #ff4d6d;
  --badge-bg: #26313d;
  --badge-purple: #6d5dfc;
  --badge-green: #2fbf71;
  --badge-blue: #3b82f6;
  --badge-yellow: #a87900;
  --badge-red: #b4233a;
}
```

## Typography

- Font: system UI, Inter if available.
- Table font size: 13px.
- Header font size: 18-22px.
- Muted metadata: 12px.
- Badges: 12px, font-weight 700.

## Badges

Impact:

- low: gray/blue
- medium: yellow
- high: orange/red
- critical: red/pink

Event types:

- fed/macro/rates: purple
- earnings/guidance: green
- sec_filing/regulation: blue
- crypto/commodity/forex: yellow
- company_news: slate

Status:

- fresh: green
- update: cyan
- duplicate: gray
- outdated: red/gray
- llm_failed: red
- pending: yellow

## Interactions

- Table row hover changes background subtly.
- Clicking article opens right-side detail drawer.
- Clicking event opens event timeline drawer.
- Filters update URL query params.
- Fresh only checkbox should be visually prominent because user cares about 72h freshness.

## Detail drawer

Article drawer:

- Title
- Source + published time + age
- Original URL button
- Summary VI
- Summary EN
- Impact/novelty/confidence bars
- Key facts
- New information
- Affected tickers/sectors/assets
- Cluster relation
- Raw LLM JSON collapsible

Event drawer:

- Event title
- Impact score
- Summary
- Timeline of updates
- Related articles grouped by relation: original, update, related, duplicate

## Empty/error states

- No fresh news: show compact empty state and next crawl time.
- Source failure: show in Sources tab, not as blocking full-app error.
- LLM unavailable: show pending/failed jobs but keep articles visible.

## Responsive behavior

Desktop first. On small screens:

- Keep header/tabs.
- Horizontal scroll for tables.
- Drawer becomes full-screen panel.
