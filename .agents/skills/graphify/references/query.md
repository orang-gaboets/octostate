# graphify reference: query, path, explain

Load this when `graphify-out/graph.json` already exists and the user asks about this repository.

## Use the CLI

```bash
graphify query "QUESTION"
graphify query "QUESTION" --dfs
graphify query "QUESTION" --budget 2000
graphify path "NODE_A" "NODE_B"
graphify explain "NODE_NAME"
```

Prefer the existing graph for repository questions. Rebuild only when the user explicitly asks for `--update`, `--cluster-only`, or a fresh scan.

## Fallback

If the CLI is unavailable, load `graphify-out/graph.json` and traverse it directly with the local graph tooling. Answer only from nodes and edges present in the graph.
