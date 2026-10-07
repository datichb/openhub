> [Lire en français](cli-deploy.fr.md)

# CLI Reference — Deployment (removed in v5)

`oh deploy` and `oh sync` copied agents, skills and configuration into every project (`.opencode/`, `opencode.json`). **oh v5 no longer deploys anything into projects**: every session starts from a **session bundle** built at launch, outside the project (`~/.oh/bundles/<hash>/`, immutable), from its workflow. See [ADR-043](../architecture/adr/043-session-bundle-deploy-removal.en.md).

## Migration aliases

```
oh deploy [former flags…]
oh sync [former flags…]
```

During v5.x, both commands remain **migration aliases**: they accept every former flag and argument (`-p`, `--check`, `--diff`, `--all`…, ignored, `--help` included), change nothing, print this message on standard error and exit with **code 0** (existing scripts do not break):

```
! oh deploy no longer exists: oh no longer deploys anything into projects, every session starts from a bundle built at launch, outside the project.
  Show the bundle of a workflow: oh bundle show <workflow> (oh bundle build to build it).
  Remove the former deployed files (.opencode/agents, skills…): oh migrate deploy-cleanup.
```

## v5 equivalents

| Former command | In v5 |
|---|---|
| `oh deploy [-p <project>]` | Nothing to do: [`oh run <workflow>`](cli-workflows.en.md#oh-run) builds the bundle at every launch. To build it in advance: [`oh bundle build <workflow> [-p <project>]`](cli-workflows.en.md#oh-bundle-build--show) |
| `oh deploy --check`, `--diff` | [`oh bundle show <workflow> [-p <project>] [--budget] [--json]`](cli-workflows.en.md#oh-bundle-build--show) (agents, skills, permissions, budget) |
| `oh sync [--all]` | Nothing to do: the bundle follows the hub, team, project and workflow at every launch |
| Leftovers of former deployments | [`oh migrate deploy-cleanup [--dry-run] [--diff] [--yes] [--json] [-p <project>]`](cli-infra.en.md#oh-migrate-deploy-cleanup) |

The files left by former deployments (`.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json`, unchanged oh keys in `opencode.json`) are removed with `oh migrate deploy-cleanup`; oh offers it once at startup (the TUI shows its cleanup screen, omnibar `cleanup`). See the [v5 migration guide](../guides/migration-v5.en.md).
