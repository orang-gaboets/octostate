---
name: graphify
description: "Optional for architecture exploration, dependency tracing, impact analysis, or whole-branch review in this repository. Fall back to normal repo inspection when Graphify is unavailable or stale."
---

# Graphify

Use Graphify as an optional workflow for architecture exploration, dependency
tracing, impact analysis, and whole-branch review in this worktree. Use normal
repository inspection when Graphify is unavailable or unnecessary.

## Entry points

- `references/query.md` for questions against an existing graph
- `references/update.md` for incremental refreshes after files change

## Guidance

- Prefer the existing graph in `graphify-out/graph.json` only when it is
  current for the files you need to inspect.
- If the graph may be stale or files changed, refresh it with `graphify update .`
  or fall back to normal source inspection.
- Keep the repository-local graph workflow scoped to this repo.
