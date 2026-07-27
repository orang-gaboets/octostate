---
name: graphify
description: "Use for local questions about this repository's codebase or file relationships. Prefer graphify query when graphify-out/graph.json already exists; use graphify update when files changed."
---

# Graphify

Use Graphify for repository-local codebase questions and refreshes in this worktree.

## Entry points

- `references/query.md` for questions against an existing graph
- `references/update.md` for incremental refreshes after files change

## Guidance

- Prefer the existing graph in `graphify-out/graph.json` when it is present.
- Use the update flow from this worktree when files change.
- Keep the repository-local graph workflow scoped to this repo.
