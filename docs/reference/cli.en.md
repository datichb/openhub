> [Lire en francais](cli.fr.md)

# CLI Reference

`oh` without a command opens the TUI (in an interactive terminal; otherwise the help). `oh -p <project>` opens the TUI directly on that project. `oh help` (or `oh --help`) shows the command overview: one line per top-level command, by section, with the advanced commands (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`) apart (about sixty lines; built from the commands themselves). `oh help --all` (or `oh --help --all`) shows the complete list: every visible subcommand with its flags. `oh <command> --help` (or `oh help <command>`) details a command (flags, examples). The whole help follows the interface language (`oh config language`).

## Global Flags

Accepted by every command.

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--verbose` | `-v` | `false` | Verbose output (debug-level logs; otherwise: warnings only) |
| `--log-format` | | `pretty` | Format of the logs on standard error: `pretty` or `json` |
| `--no-tui` | | `false` | Disable the rich TUI (inline prompts only) |
| `--help` | `-h` | | Help of the command |

---

## Table of Contents

| Section | File | Key Commands |
|---------|------|--------------|
| [Sessions](cli-sessions.en.md) | `cli-sessions.en.md` | [Session management (v5)](cli-sessions.en.md#session-management-v5): `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|trace\|resume\|stop\|delete\|open\|fetch\|resolve`; `oh budget show\|set\|unset\|raise\|report`; `oh history`; `oh beads` |
| [Workflows](cli-workflows.en.md) | `cli-workflows.en.md` | `oh run`; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive`; `oh bundle build\|show` |
| [Bricks](cli-bricks.en.md) | `cli-bricks.en.md` | `oh brick list\|show\|new\|edit\|publish\|history` (team brick catalogue) |
| [Projects](cli-projects.en.md) | `cli-projects.en.md` | `oh project list\|add\|remove\|rename\|move\|configure`; `oh mr create`; `oh wiki check\|pending\|stats` |
| [Deployment (migration aliases)](cli-deploy.en.md) | `cli-deploy.en.md` | `oh deploy`, `oh sync` (removed in v5: migration message) |
| [Configuration](cli-config.en.md) | `cli-config.en.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.en.md) | `cli-infra.en.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.en.md) | `cli-mcp.en.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Team](cli-team.en.md) | `cli-team.en.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Tools](cli-tools.en.md) | `cli-tools.en.md` | `oh skill check`, `oh eval`, `oh worktree *`, `oh secrets *`, `oh metrics` (deprecated), `oh dashboard` (deprecated), `oh board`, `oh version`, `oh completion` |

Internal commands (hidden from the help, not documented in detail): `oh daemon run` (the `ohd` daemon, started automatically by oh), `oh runner install|run` (CI side of remote execution, used by the `oh-runner` pipeline), `oh mcp serve workflow` (the `workflow` MCP server injected into every session bundle), `oh mcp serve wiki` (the project `wiki` MCP server).

---

## Exit Codes

The codes are defined in one place (`cli/cmd/exitcodes.go`). A script can test them.

| Code | Constant | Meaning |
|------|----------|---------|
| `0` | `ExitOK` | Success. For `oh doctor`, also when only warnings (⚠) are shown: the optional checks (container engine, `bd`, gateway reach from a container) never fail the command, unless `execution.runtime = container` makes the container mandatory |
| `1` | `ExitFailure` | Failure: the message is written to standard error. For example: `oh workflow validate` or `oh skill check` finding errors, `oh run --headless` with a failed session or exceeding `--timeout`, `oh doctor` when a mandatory check fails (✗), internal error (panic: oh asks to report the bug) |
| `2` | `ExitUsage` | Wrong usage: unknown command or option, or removed command (`oh deploy`, `oh sync`, and in 5.1 `oh start`, `oh audit`, `oh review`, `oh debug`, `oh service`, `oh takeover-brief enrich`: the migration message is shown). Also a confirmation asked without a terminal and without `--yes`: the action is refused (`workflow publish\|archive\|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `worktree cleanup`, `session delete\|resolve`, `migrate deploy-cleanup`, `run --recap`) |
| `3` | `ExitPending` | Waiting: `oh run --headless` with a session waiting for a decision (checkpoint) that nobody can answer without an interface. The session is left running: `oh session approve <id>` or `oh session answer <id>`. If another session failed, the code is `1` |
| `bd` code | | `oh beads` passes the exit code of `bd` through |

`oh doctor` checks: a mandatory check (system, `git`, configuration, database, session tool, workflow integrity…) that fails gives `1`. An optional check that fails is shown as a warning and does not change the code.

## Short options

A letter has one meaning in every command. The former forms deprecated in 5.0 were removed in 5.1: a removed option gives the unknown-option error (`unknown shorthand flag: 'p' in -p`, exit code 2).

| Letter | Meaning | Commands |
|--------|---------|----------|
| `-a` | `--agent` | `oh run` |
| `-b` | `--base` (base branch) | `oh worktree cleanup` |
| `-d` | `--path` (folder) | `oh project add` |
| `-f` | `--force` (override a safety) | `oh worktree remove`, `oh worktree cleanup` |
| `-h` | `--help` | all |
| `-i` | `--input` (workflow input) | `oh run` |
| `-l` | `--language` | `oh project add`, `oh project configure` |
| `-m` | `--message`: a text (the request of `oh run`, the publication note, the message passed to the agent) | `oh run`, `oh brick publish`, `oh workflow publish`, `oh workflow archive`, `oh session approve`, `oh team notify test` |
| `-n` | `--name` | `oh project add` |
| `-o` | `--output` | `oh export`, `oh session trace` |
| `-p` | `--project` | every command that targets a project, `oh` |
| `-P` | `--provider` | `oh run`, `oh bundle build\|show`, `oh project configure` |
| `-s` | `--status` | `oh project list` |
| `-v` | `--verbose` | all |
| `-w` | `--workflow` | `oh config model show` |
| `-y` | `--yes` (no confirmation) | `oh doctor`, `oh gc`, `oh mcp trust`, `oh migrate deploy-cleanup`, `oh project add`, `oh session delete`, `oh session resolve`, `oh brick publish`, `oh workflow publish\|archive\|restore`, `oh run`, `oh purge`, `oh project remove`, `oh teams remove`, `oh worktree cleanup` |

The short forms deprecated in 5.0 no longer exist: `oh serve -p` (now `--port`), `oh metrics -d` (`--period`), `oh project configure -m` (`--model`), `oh session attach --how` (`--attach`). The long names do not change. The short options of `oh start`, `oh audit`, `oh review` and `oh debug` went with these commands (see [Migration from 5.0](../guides/migration-v5.en.md#from-50-to-51)).

`--force` no longer skips a confirmation: that is `--yes`. On `oh purge`, `oh project remove` and `oh teams remove`, `--force` and `-f` were removed in 5.1 (they were deprecated aliases of `--yes`). `--force` stays only where it overrides a safety: `oh daemon stop` (stop even though sessions are running), `oh remote setup` (replace a hand-written `.gitlab-ci.yml`), `oh worktree remove` and `oh worktree cleanup` (also remove worktrees with uncommitted changes). Likewise `oh repair --auto` was removed: `--yes` confirms the restoration.

**Deprecated in 5.1 (removed in v6).** `oh dashboard` and `oh metrics` still work, hidden from the help, and warn on the error output (`"oh dashboard" is deprecated and will be removed in v6: use the Metrics view of the TUI (oh, then Ctrl+P, Metrics)`) in the language of the hub. `oh dashboard` opens the TUI on that view; `oh metrics` keeps its text output on the standard output until v6.

**Opening a window.** One option, `--attach` (`auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, and `none` for `oh run`), in `oh run` as in `oh session attach`. The former name `--how` of `oh session attach` and the value `here` (former synonym of `suspend`) were removed in 5.1.

**See without launching.** `oh run --dry-run` prepares the launch (workflow resolved, bundle built, sessions planned), shows the recap, the pauses and the warnings, then stops (« dry-run: nothing was started », exit code 0): no session, no worktree, no ticket claim, no question. See [`oh run`](cli-workflows.en.md#oh-run).

**Cancel a session.** There is no « cancel » command: [`oh session stop`](cli-sessions.en.md#oh-session-stop) stops it and keeps its work, [`oh session delete`](cli-sessions.en.md#oh-session-delete) deletes it (worktree, local branch, files).

## Symbols and colors

A symbol stands for one thing, in the CLI (`oh session list`, `oh session inbox`) as in the TUI (Sessions view, mode bar badge, landing sections). Both read the same table (`cli/internal/glyph`).

**Session state**

| Symbol | State | Meaning |
|--------|-------|---------|
| `◓` | `preparing` | the session bundle is being prepared |
| `…` | `queued` | queued (session limit) |
| `●` | `active` | the agent is working |
| `◐` | `waiting` | waits for a decision from you |
| `○` | `idle` | turn over, waiting for an instruction |
| `◌` | `sleeping` | asleep (the server is stopped, the session can be resumed) |
| `✔` | `completed` | finished |
| `⊗` | `failed` | failed |
| `■` | `stopped` | stopped by the user |

**Waiting decision**

| Symbol | Kind | Meaning |
|--------|------|---------|
| `⏸` | checkpoint | a workflow gate to approve |
| `?` | question | the agent asks a question |
| `!` | permission | an action asks for your approval |
| `$` | budget | a restriction is reached |
| `✗` | error | the agent hit an error to handle |
| `⊘` | circuit breaker | too many subagent calls without you: delegation is denied until you dismiss it |

**Team event** (`oh team activity`, Activity and Team Status views: `cli/internal/glyph`, `Events`)

| Symbol | Events |
|--------|--------|
| `●` | `claim.taken` |
| `○` | `claim.released`, `workflow.archived` |
| `→` | `claim.transferred` |
| `✗` | `claim.conflict`, `review.rejected`, `wiki.rejected` |
| `✔` | `session.complete`, `review.approved`, `wiki.accepted`, `workflow.published` |
| `◐` | `review.ready` (waits for a person) |
| `⚠` | `audit.finding` |
| `✎` | `wiki.proposal` |
| `↺` | `workflow.restored` |

The phrase after the actor is the same in the CLI and the TUI (`Event.Describe`).

The mode bar badge, `● N  ◐ M`, counts live sessions (`●`) and waiting decisions (`◐`). Elsewhere, `✓` marks a successful command result, `✗` a failed one, `⚠` or `!` a warning.

**Colors.** `NO_COLOR` (any non-empty value, see [no-color.org](https://no-color.org)) removes the colors of the CLI (help, tables, forms, logs) and of the TUI; the symbols above are enough to read the state. Without `NO_COLOR`, the terminal decides (redirected output = no color). There is no `--no-color` option.
