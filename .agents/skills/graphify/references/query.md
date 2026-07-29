# graphify reference: query, path, explain

Load this when `graphify-out/graph.json` already exists and the user needs
architecture exploration, dependency tracing, impact analysis, or whole-branch
review for this repository. If the graph does not exist yet, build it first
with the maintainer guide's `graphify extract . --code-only` setup.

## Use the CLI

```bash
graphify query "QUESTION"
graphify query "QUESTION" --dfs
graphify query "QUESTION" --budget 2000
graphify path "NODE_A" "NODE_B"
graphify explain "NODE_NAME"
graphify affected "PATH_OR_NODE"
```

Use the existing graph only when it appears current for the files involved.
If the graph may be stale or files changed, run `graphify update .` first or
fall back to normal source inspection.

## Fallback

If the CLI is unavailable, use normal repository inspection and the existing Go
and Git tooling. Use the saved graph only when it contains the nodes and edges
needed to answer the question.
