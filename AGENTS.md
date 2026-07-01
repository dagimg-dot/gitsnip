# AGENTS.md

See [ARCHITECTURE.md](./ARCHITECTURE.md) for project structure and design.

## Pre-commit

Run before any commit:

```
make test
go vet ./...
```

Fix all failures. No commit with broken tests or vet warnings.

## Commit rules

- Never commit without explicit user approval.
- Concise messages. No co-authors, no footers, no trailers.
- Separate concerns into separate commits (refactor != chore != feature).
- Keep the diff focused — no drive-by changes in unrelated files.
