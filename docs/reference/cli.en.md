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
| [Sessions](cli-sessions.en.md) | `cli-sessions.en.md` | [Session management (v5)](cli-sessions.en.md#session-management-v5): `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|trace\|resume\|stop\|delete\|open\|fetch\|resolve`; `oh budget show\|set\|unset\|raise\|report`; `oh history`; `oh beads`; deprecated aliases `oh start`, `oh audit`, `oh review`, `oh debug` |
| [Workflows](cli-workflows.en.md) | `cli-workflows.en.md` | `oh run`; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive`; `oh bundle build\|show` |
| [Bricks](cli-bricks.en.md) | `cli-bricks.en.md` | `oh brick list\|show\|new\|edit\|publish\|history` (team brick catalogue) |
| [Projects](cli-projects.en.md) | `cli-projects.en.md` | `oh project list\|add\|remove\|rename\|move\|configure`; `oh mr create`; `oh wiki check\|pending\|stats` |
| [Deployment (migration aliases)](cli-deploy.en.md) | `cli-deploy.en.md` | `oh deploy`, `oh sync` (removed in v5: migration message) |
| [Configuration](cli-config.en.md) | `cli-config.en.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.en.md) | `cli-infra.en.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.en.md) | `cli-mcp.en.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Team](cli-team.en.md) | `cli-team.en.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Tools](cli-tools.en.md) | `cli-tools.en.md` | `oh skill check`, `oh eval`, `oh worktree *`, `oh secrets *`, `oh metrics`, `oh dashboard`, `oh board`, `oh version`, `oh completion` |

Internal commands (hidden from the help, not documented in detail): `oh daemon run` (the `ohd` daemon, started automatically by oh), `oh runner install|run` (CI side of remote execution, used by the `oh-runner` pipeline), `oh mcp serve workflow` (the `workflow` MCP server injected into every session bundle), `oh mcp serve wiki` (the project `wiki` MCP server).

---

## Exit Codes

The codes are defined in one place (`cli/cmd/exitcodes.go`). A script can test them.

| Code | Constant | Meaning |
|------|----------|---------|
| `0` | `ExitOK` | Success. For `oh doctor`, also when only warnings (⚠) are shown: the optional checks (container engine, `bd`, gateway reach from a container) never fail the command, unless `execution.runtime = container` makes the container mandatory |
| `1` | `ExitFailure` | Failure: the message is written to standard error. For example: `oh workflow validate` or `oh skill check` finding errors, `oh run --headless` with a failed session or exceeding `--timeout`, `oh doctor` when a mandatory check fails (✗), internal error (panic: oh asks to report the bug) |
| `2` | `ExitUsage` | Wrong usage: unknown command or option, or removed command (`oh deploy`, `oh sync`: the migration message is shown). Also a confirmation asked without a terminal and without `--yes`: the action is refused (`workflow publish\|archive\|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete\|resolve`, `migrate deploy-cleanup`, `run --recap`) |
| `3` | `ExitPending` | Waiting: `oh run --headless` with a session waiting for a decision (checkpoint) that nobody can answer without an interface. The session is left running: `oh session approve <id>` or `oh session answer <id>`. If another session failed, the code is `1` |
| `bd` code | | `oh beads` passes the exit code of `bd` through |

`oh doctor` checks: a mandatory check (system, `git`, configuration, database, session tool, workflow integrity…) that fails gives `1`. An optional check that fails is shown as a warning and does not change the code.

## Short options

A letter has one meaning in every command. The former forms stay accepted for one version, with a warning, and are hidden from the help.

| Letter | Meaning | Commands | Former form (deprecated) |
|--------|---------|----------|--------------------------|
| `-a` | `--agent` | `oh run`, `oh start` | |
| `-A` | `--assignee` | `oh start` | |
| `-b` | `--base` (base branch) | `oh worktree cleanup` | `oh review -b` (now `--branch`) |
| `-d` | `--path` (folder) | `oh project add` | `oh metrics -d` (now `--period`) |
| `-f` | `--force` | `oh project remove`, `oh teams remove`, `oh worktree remove`, `oh worktree cleanup`, `oh service remove` | |
| `-h` | `--help` | all | |
| `-i` | `--input` (workflow input) | `oh run` | `oh debug -i` (now `--issue`) |
| `-l` | `--language` | `oh project add`, `oh project configure` | `oh start -l` (now `--label`) |
| `-m` | `--message`: a text (the request of `oh run`, the publication note, the message passed to the agent). `--prompt` of `oh start` is the request | `oh run`, `oh brick publish`, `oh workflow publish`, `oh workflow archive`, `oh session approve`, `oh team notify test`, `oh start` | `oh review -m` (now `--mode`), `oh project configure -m` (now `--model`) |
| `-n` | `--name` | `oh project add` | |
| `-o` | `--output` | `oh export`, `oh session trace` | |
| `-p` | `--project` | every command that targets a project, `oh` | `oh serve -p` (now `--port`) |
| `-P` | `--provider` | `oh run`, `oh bundle build\|show`, `oh project configure`, `oh start` | |
| `-r` | `--resume` | `oh start` | |
| `-s` | `--status` | `oh project list` | |
| `-t` | `--ticket` | `oh start` | `oh audit -t` (now `--type`) |
| `-v` | `--verbose` | all | |
| `-w` | `--workflow` | `oh config model show` | `oh start -w` (now `--worktree`) |
| `-y` | `--yes` (no confirmation) | `oh doctor`, `oh gc`, `oh mcp trust`, `oh migrate deploy-cleanup`, `oh project add`, `oh session delete`, `oh session resolve`, `oh brick publish`, `oh workflow publish\|archive\|restore`, `oh run`, `oh purge`, `oh project remove`, `oh teams remove`, `oh service remove`, `oh worktree cleanup` | |

A former form stays accepted for one version: it does the same as the new one, prints a warning ("Option -p of oh serve is deprecated, use --port.") and is gone from the help. The short forms of `oh start`, `oh audit`, `oh review` and `oh debug` (deprecated aliases of `oh run`) follow the same rule.

**Opening a window.** One option, `--attach` (`auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, and `none` for `oh run`), in `oh run` as in `oh session attach`. The former name `--how` of `oh session attach` is still accepted (warning), and so is the value `here`, former synonym of `suspend`.

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

The mode bar badge, `● N  ◐ M`, counts live sessions (`●`) and waiting decisions (`◐`). Elsewhere, `✓` marks a successful command result, `✗` a failed one, `⚠` or `!` a warning.

**Colors.** `NO_COLOR` (any non-empty value, see [no-color.org](https://no-color.org)) removes the colors of the CLI (help, tables, forms, logs) and of the TUI; the symbols above are enough to read the state. Without `NO_COLOR`, the terminal decides (redirected output = no color). There is no `--no-color` option.
