# graphify reference: incremental update and cluster-only

Load this when the user passed `--update` or `--cluster-only`.

## For `--update`

```bash
graphify update .
```

Use this after files change so the graph and report stay in sync with the current worktree.

## For `--cluster-only`

```bash
graphify cluster-only .
```

Use this when the graph already exists and only clustering or report regeneration is needed.
