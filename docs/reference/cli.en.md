> [Lire en francais](cli.fr.md)

# CLI Reference

`oh` without a command opens the TUI (in an interactive terminal; otherwise the help). `oh -p <project>` opens the TUI directly on that project. `oh help` (or `oh --help`) shows the command overview (every visible command, by section, with its flags; built from the commands themselves); `oh <command> --help` details a command (flags, examples). The whole help follows the interface language (`oh config language`).

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
| [Sessions](cli-sessions.en.md) | `cli-sessions.en.md` | [Session management (v5)](cli-sessions.en.md#session-management-v5): `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|trace\|resume\|stop\|delete\|open\|fetch\|resolve`; `oh budget show\|set\|unset\|raise`; `oh history`; `oh beads`; deprecated aliases `oh start`, `oh audit`, `oh review`, `oh debug` |
| [Workflows](cli-workflows.en.md) | `cli-workflows.en.md` | `oh run`; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive`; `oh bundle build\|show` |
| [Bricks](cli-bricks.en.md) | `cli-bricks.en.md` | `oh brick list\|show\|new\|edit\|publish\|history` (team brick catalogue) |
| [Projects](cli-projects.en.md) | `cli-projects.en.md` | `oh project list\|add\|remove\|rename\|move\|configure`; `oh wiki check\|pending\|stats` |
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
| `2` | `ExitUsage` | Wrong usage: unknown command or option, or removed command (`oh deploy`, `oh sync`: the migration message is shown) |
| `3` | `ExitPending` | Waiting: `oh run --headless` with a session waiting for a decision (checkpoint) that nobody can answer without an interface. The session is left running: `oh session approve <id>` or `oh session answer <id>`. If another session failed, the code is `1` |
| `bd` code | | `oh beads` passes the exit code of `bd` through |

`oh doctor` checks: a mandatory check (system, `git`, configuration, database, session tool, workflow integrity…) that fails gives `1`. An optional check that fails is shown as a warning and does not change the code.

## Short options

A letter has one meaning in every command. The former forms stay accepted for one version, with a warning, and are hidden from the help.

{{SHORTS_EN}}

## Symbols and colors

{{SYMBOLS_EN}}
