> [Lire en français](cli-deploy.fr.md)

# CLI Reference — Deployment (removed in v5)

`oh deploy` and `oh sync` copied agents, skills and configuration into every project (`.opencode/`, `opencode.json`). **oh v5 no longer deploys anything into projects**: every session starts from a **bundle** built at launch, outside the project (`~/.oh/bundles/<hash>/`), from its workflow.

| Former command | In v5 |
|---|---|
| `oh deploy [-p <project>]` | nothing to do: `oh run <workflow>` builds the bundle. To see it: `oh bundle show <workflow> [-p <project>] [--budget]` |
| `oh deploy --check`, `--diff` | `oh bundle show <workflow>` (agents, skills, permissions, budget) |
| `oh sync [--all]` | nothing to do: the bundle follows the hub and the workflow at every launch |

During v5.x, `oh deploy` and `oh sync` remain aliases that print this migration message (exit code 0).

The files left by former deployments (`.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json`, oh keys in `opencode.json`) are removed with `oh migrate deploy-cleanup`. See the [v5 migration guide](../guides/migration-v5.en.md).
