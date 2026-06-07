# Known Limitations

- Generic HTML adapters are intentionally conservative and collect only visible public links/metadata; they do not extract full article pages unless the source config allows full content.
- SEC EDGAR MVP polls a configured watchlist instead of the whole market.
- Cluster repair command is present as a CLI placeholder; normal clustering is triggered after successful LLM analysis.
- The backend API and Vite frontend run as separate local processes in development. Docker builds frontend assets, but the Go server currently exposes the REST API rather than embedded static UI routing.
- Source robots/ToS enforcement is policy-driven and conservative; advanced robots.txt caching can be expanded later.
- Integration tests requiring a live PostgreSQL instance are not enabled by default; current tests are unit/handler tests that run without external services.
