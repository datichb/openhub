> [Lire en francais](tui-usage.fr.md)

# Guide -- Using the OpenHub TUI

> Practical guide for navigating and working in the TUI interface.

## Quick Start

```bash
oh
```

The TUI displays a splash screen with quick-start hints and the omnibar at the bottom. The TUI organizes work into three modes: **Hub mode** (global settings), **Project mode** (per-project views), and **Team mode** (team collaboration).

---

## Navigation

### Omnibar

The omnibar is the primary navigation tool. Activate it by:
- **Typing any letter** when no input is focused
- **`Ctrl+P`** at any time

The omnibar supports fuzzy matching -- type partial words, abbreviations, or aliases:

```
> secu        → launches security audit
> dep         → deploys to active project
> doc         → opens doctor diagnostics
> tst         → team status
```

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` | Open omnibar |
| `Esc` | Back (previous view / dismiss omnibar) |
| `Ctrl+Q` | Quit |
| `j` / `k` | Navigate up/down in lists |
| `Enter` | Select / confirm |
| `Tab` | Switch between panes (where applicable) |

### Toasts

Action results appear as toasts in the top-right corner (success/error/info). They auto-dismiss after a few seconds.

---

## Available Views

### Sessions & Actions

| Command | Description |
|---------|-------------|
| `start` | Launch a session (shows mode picker: Standard, Dev, Onboard) |
| `start dev` | Dev mode directly (ticket workflow) |
| `start onboard` | Onboard mode directly (project discovery) |
| `quick` | Direct opencode launch (no mode selection) |
| `audit` | Audit launcher (picks audit type) |
| `audit security` | Security audit |
| `audit performance` | Performance audit |
| `audit architecture` | Architecture audit |
| `audit accessibility` | Accessibility audit |
| `audit ecodesign` | Eco-design audit |
| `audit observability` | Observability audit |
| `review` | Review launcher (picks review mode) |
| `review standard` | Standard code review |
| `review adversarial` | Adversarial review |
| `review edge` | Edge case review |
| `review complete` | Complete review (all modes) |
| `debug` | Debug session with issue description |
| `parallel` | Parallel sessions view (multi-ticket) |

When a session starts, the TUI suspends and opencode takes over. When you exit opencode, the TUI resumes.

### Project Management

| Command | View | Description |
|---------|------|-------------|
| `projects` | Projects list | All registered projects with status |
| `board` | Kanban board | Project tickets board (requires bd) |
| `deploy` | - | Deploy agents/skills to current project |
| `sync` | - | Sync agents/skills across all projects |
| `project-config` | Project config | Per-project settings (provider, model, MCP, agents) |

In the **Projects** view:
- `a` to add a new project
- `d` to delete
- `Enter` to configure
- `r` to rename

### Configuration

| Command | View | Description |
|---------|------|-------------|
| `models` | Model config | Default and per-agent model configuration |
| `provider` | Provider setup | LLM provider settings |
| `mcp` | MCP servers | MCP server enable/disable/status |
| `settings` | General settings | Language, opencode version, auto-update |
| `secrets` | Secrets & Tokens | Manage stored credentials |
| `teams` | Teams list | Multi-team management |

### Team (requires team-state repo)

| Command | View | Description |
|---------|------|-------------|
| `team-detail` | Team detail | Team configuration and members |
| `team board` | Team board | Kanban board across all team members |
| `team status` | Status | Team activity summary |
| `team activity` | Activity | Recent team events |
| `team briefs` | Takeover briefs | Available takeover contexts |
| `team patterns` | Patterns | Shared team patterns library |
| `team policies` | Policies | Team-enforced policies |
| `team sync` | - | Sync claims with external tracker |
| `team init` | - | Initialize team features |

### System

| Command | View | Description |
|---------|------|-------------|
| `status` | System status | Hub and project health overview |
| `doctor` | Diagnostics | Run health checks (press `r` to re-run) |
| `metrics` | Agent metrics | Per-agent usage, cost, and duration stats |
| `plugins` | Plugin manager | List/install/remove plugins |
| `notifications` | Notifications | Recent notification events |
| `worktrees` | Worktree manager | List, add, remove Git worktrees |
| `upgrade` | - | Upgrade oh or opencode |
| `help` | Help | Keybindings and command reference |

### Navigation

| Command | Description |
|---------|-------------|
| `home` | Return to home/splash screen |
| `project mode` | Switch to project-scoped view |
| `hub mode` | Switch to hub-level (global) view |
| `quit` | Exit the TUI |

---

## Session Types Reference

### Audit types (6)

| Type | Focus area |
|------|-----------|
| Security | OWASP, dependencies, secrets, auth |
| Performance | Load, latency, memory, CPU |
| Architecture | Patterns, coupling, complexity |
| Accessibility | WCAG, ARIA, keyboard nav |
| Eco-design | Carbon footprint, resource usage |
| Observability | Logs, metrics, traces, alerts |

### Review modes (4)

| Mode | Approach |
|------|---------|
| Standard | Balanced review across all dimensions |
| Adversarial | Actively tries to break the code |
| Edge cases | Focuses on boundary conditions and error paths |
| Complete | Runs all modes in parallel, merges results |

---

## Tips

- **Fuzzy matching** — type any part of a command name. `sec` matches `audit security`, `rev` matches `review`.
- **Mode switching** — the TUI adapts available commands to your current mode. Project-specific commands only appear in project mode.
- **Config editing** — in config views, navigate with `j`/`k`, press `Enter` to edit a value. Changes auto-save.
- **Contextual shortcuts** — each view shows available shortcuts in the omnibar hint text at the bottom.
- **Project auto-detection** — if you launched `oh` from a registered project directory, it starts in project mode for that project.
