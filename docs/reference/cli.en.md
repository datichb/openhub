> [Lire en francais](cli.fr.md)

# CLI Reference

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Enable verbose output (debug logging) |
| `--log-format` | | Log output format: `pretty` (default) or `json` |
| `--no-tui` | | Disable rich TUI (use inline prompts only) |

---

## Table of Contents

| Section | File | Key Commands |
|---------|------|--------------|
| [Sessions](cli-sessions.en.md) | `cli-sessions.en.md` | `oh start`, `oh review`, `oh audit`, `oh debug` |
| [Workflows](cli-workflows.en.md) | `cli-workflows.en.md` | `oh run`, `oh workflow list\|show\|validate`, `oh bundle build\|show` |
| [Projects](cli-projects.en.md) | `cli-projects.en.md` | `oh project list\|add\|remove\|rename\|move\|configure` |
| [Deployment](cli-deploy.en.md) | `cli-deploy.en.md` | `oh deploy`, `oh sync` (removed in v5) |
| [Configuration](cli-config.en.md) | `cli-config.en.md` | `oh config *`, `oh provider setup`, `oh config model` |
| [Infrastructure](cli-infra.en.md) | `cli-infra.en.md` | `oh init`, `oh doctor`, `oh repair`, `oh upgrade`, `oh migrate`, `oh purge`, `oh serve` |
| [MCP & Plugins](cli-mcp.en.md) | `cli-mcp.en.md` | `oh mcp *` |
| [Team](cli-team.en.md) | `cli-team.en.md` | `oh team *`, `oh teams *`, `oh conventions`, `oh policies` |
| [Tools](cli-tools.en.md) | `cli-tools.en.md` | `oh skill *`, `oh worktree *`, `oh secrets *`, `oh metrics` |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Error |
| `2` | Warning |
