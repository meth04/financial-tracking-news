# /project:05-frontend

Build the frontend dashboard.

Read:

- `docs/UI_STYLE_GUIDE.md`
- `design/ui-reference-notes.md`
- `api/openapi.yaml`

## Tasks

1. Initialize Vite + React + TypeScript in `web` if missing.
2. Build API client.
3. Build shared components:
   - AppShell
   - HeaderStats
   - Tabs
   - FilterBar
   - DenseTable
   - Badge
   - ScoreBar
   - Pagination
   - DetailDrawer
   - EmptyState
   - ErrorState
4. Build pages:
   - ArticlesPage
   - EventsPage
   - SourcesPage
   - LLMQueuePage
   - SettingsPage
5. Implement article detail drawer:
   - source/published/age
   - original URL
   - summary_vi/summary_en
   - impact/novelty/confidence
   - key facts
   - new information
   - tickers/sectors/assets
   - cluster relation
   - raw JSON collapsible
6. Implement event detail drawer:
   - event title
   - summary
   - impact
   - timeline updates
   - articles grouped by relation
7. Implement dark dense style matching reference screenshot.
8. Add fresh-only toggle, default true.
9. Persist filters in URL query params.
10. Add loading states and error states.

## Constraints

- Desktop-first, horizontal table scroll allowed.
- Do not use exact GoodAIList branding.
- Keep rows compact and readable.
- Use CSS variables from style guide.
- Avoid huge UI frameworks unless necessary.

## Verify

Run:

```bash
cd web
npm install
npm run build
npm run dev
```
