# /project:07-final-check

Perform final review and close gaps.

Read:

- `docs/ACCEPTANCE_CRITERIA.md`
- `CLAUDE.md`

## Tasks

1. Run all tests and builds.
2. Check that fresh-only default is true everywhere.
3. Check that older-than-72h articles do not appear in default UI/API.
4. Check that LLM max concurrency is 3 by default.
5. Check that each article can have only one LLM analysis.
6. Check that failed LLM jobs retry.
7. Check that source failures do not crash app.
8. Check that raw items are saved before normalization.
9. Check that same-source T+1 updates are not blindly discarded.
10. Check UI visually against `design/ui-reference.png`.
11. Update README with final exact commands.
12. Add a `docs/KNOWN_LIMITATIONS.md` file listing any remaining limitations.

## Final response format

When done, report:

- What was built
- How to run it
- What tests passed
- Any known limitations
- Next recommended improvements
