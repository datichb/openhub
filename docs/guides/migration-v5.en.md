> [Lire en français](migration-v5.fr.md)

# Migrating to oh v5

oh v5 starts every session from a **workflow** (`oh run <workflow>`), in a **session bundle** built outside the project, on an **opencode V2** server. This guide lists what changes for a v4 user.

## 1. opencode V2 required

opencode V1 is no longer supported. Without opencode V2 (minimum version: 2.0.0), oh refuses to start a session and says why:

```text
opencode 1.18.29 is no longer supported: oh v5 requires opencode V2 (2.0.0 or later).
```

1. Install opencode V2 with its own tooling (`brew install anomalyco/tap/opencode`, or https://opencode.ai).
2. Check with `oh doctor`: the **opencode V2** line must be green.

`oh upgrade opencode`, the managed install in `~/.oh/bin` and the `[opencode] version`, `channel`, `auto_update`, `install_dir` keys of `hub.toml` are removed (ignored if they remain). `[opencode] default_provider` stays.

## 2. Former commands: aliases of `oh run`

They still work during v5.x, with a warning that gives the command to use:

| Former command | v5 equivalent |
|---|---|
| `oh start` | `oh run feature` |
| `oh start --agent <id>` | `oh run libre --agent <id>` |
| `oh start --dev [-t <id>]` | `oh run ticket --tickets <id>` |
| `oh start --onboard [--refresh]` | `oh run onboarding [-i refresh=true]` |
| `oh start --parallel --tickets a,b` | `oh run ticket --tickets a,b` |
| `oh start --sweep <goal>` | `oh run sweep -i goal=<goal>` |
| `oh start --resume <id>` | `oh session attach <id> --how here` |
| `oh audit`, `oh review`, `oh debug` | `oh run audit`, `oh run review`, `oh run debug` |
| `oh review feedback <mr>` | `oh run review-feedback` (discussions are read by oh) |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |

No longer working:

- `oh start --parallel` **without** `--tickets` (refused); `--max-sessions`, `--priority`, `--sweep-branch-prefix` have no effect.
- `oh start --agent` combined with `--dev`, `--onboard`, `--parallel` or `--sweep` (refused).
- The monitor and merge view of the parallel mode: follow the sessions in the **Sessions** view or `oh session list`.

## 3. Free session: the `libre` workflow

The former launch "by agent" becomes the shipped **`libre`** workflow: the agent of your choice (`orchestrator` by default) and the agents it may call, without checkpoints, in a closed world.

```bash
oh run libre --agent debugger -i request="test X fails since yesterday"
```

In the TUI: the `coder` command, or opening a session from the Worktrees view.

## 4. Plugins

`oh plugin` and the Plugins view (global opencode V1 plugins, RTK included) are removed. Plugins are declared per workflow (`plugins:`); see [Shipped workflows](../reference/workflows.en.md#plugins-and-code-mode).

## 5. Metrics

`oh metrics`, the dashboard and the Metrics view now read the oh session registry (`~/.oh/oh.db`), fed by the daemon for every v5 session. Sessions started outside oh, or before v5, are no longer counted.

## 6. No more deployment into projects

`oh deploy` and `oh sync` are removed: every session starts from a bundle built at launch, outside the project (`oh bundle show <workflow>` to see it). During v5.x, both commands print a migration message. Also removed:

- the **Agents** and **Deployment** steps of the init wizard, `oh project add` and `oh project configure` (the bundle holds the agents of the workflow; the per-project agent selection, column `projects.agents`, is dropped by migrations v36 and v37);
- the Deployment section of the project mode, the `deploy`/`sync` omnibar commands and the drift toast;
- `[deploy] disable_native_agents` (the closed world of the bundle always hides the native agents); `[deploy] instruction_files` stays;
- the `.opencode` links of worktrees (and the `s` resync of the Worktrees view);
- the former Workflow view (replaced by the workflow catalogue and editor).

The `team` MCP server reads the team of the session from its environment (`OH_TEAM_ID`) instead of `.opencode/team.json`.

## 7. Brick catalogue

The former project "Agents" view (selection of the deployed agents) becomes the read-only **Brick catalogue**: agents and skills, origin (hub or team catalogue), estimated cost, loaded skills, dependencies (`requires`) and the workflows that ship them. Access: Project config › "Bricks", or the `bricks` omnibar command. To change the agents of a session, edit (or extend) its workflow.
