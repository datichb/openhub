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
> secu        → opens the launch form of the audit workflow
> brick       → opens the brick catalogue
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
| `run <workflow>` | Opens the launch form of a catalogue workflow (Inputs → Options → Recap, `Ctrl+S` launches). Former names accepted as aliases: `start` → `run feature`, `dev` → `run ticket`, `onboard` → `run onboarding`, `audit`, `review`, `debug` |
| `coder` | Free session (form of the `libre` workflow) |
| `sessions` | **Sessions** view: to handle, running, sleeping, finished (aliases `parallel`, `inbox`) |
| `workflows` | Workflow catalogue (editor, publication, history) |
| `cleanup` | Cleanup screen for former deployments |

A session opens with the method chosen in Settings (new tab or window, tmux, browser, or current terminal with the TUI suspended). Closing opencode does not stop the session: the TUI stays the control tower (Sessions view). See [Sessions v5](sessions-v5.en.md).

### Project Management

| Command | View | Description |
|---------|------|-------------|
| `projects` | Projects list | All registered projects with status |
| `project add` | Wizard | Add a new project |
| `board` | Kanban board | Project tickets board (requires bd) |
| `bricks` | Brick catalogue | Agents and skills (hub/team origin, estimated cost, workflows using them), read-only |
| `project-config` | Project config | Per-project settings (provider, model, MCP, execution) |

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
| `settings` | General settings | Language, default provider, session opening and sleep, MCP, worktrees, tracker; Execution, Remote, Restrictions |
| `secrets` | Secrets & Tokens | Manage stored credentials |
| `teams` | Teams list | Multi-team management |
| `init` | Wizard | Reconfigure the hub (provider, credentials) |

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
| `team init` | Wizard | Initialize team features (6-step wizard) |

### System

| Command | View | Description |
|---------|------|-------------|
| `status` | System status | Hub and project health overview |
| `doctor` | Diagnostics | Run health checks (press `r` to re-run) |
| `metrics` | Agent metrics | Per-agent usage, cost, and duration stats |
| `notifications` | Notifications | Recent notification events |
| `worktrees` | Worktree manager | List, add, remove Git worktrees |
| `help` | Help | Keybindings and command reference |

### Navigation

| Command | Description |
|---------|-------------|
| `home` | Return to home/splash screen |
| `project mode` | Switch to project-scoped view |
| `hub mode` | Switch to hub-level (global) view |
| `quit` | Exit the TUI |

### Navigation Modes

The TUI provides three navigation modes that filter omnibar commands and adapt the Home page to the active context:

- **Hub** (default): overview of all projects and teams. Auto-selected when multiple projects/teams are configured.
- **Project** (`Ctrl+T` or selection from Home): focused on a single project. Omnibar filtered to project commands (sessions, board, project config).
- **Team** (`Ctrl+T` or selection from Home): focused on a single team. Omnibar filtered to team commands (team board, status, policies).

**Auto-detection**: 1 project configured → Project mode; 1 team → Team mode; otherwise → Hub.

**Transitions**:
- `Ctrl+T` to switch between modes
- Select a project or team from the Hub Home
- `hub mode` command to return to Hub mode

Each mode has its own Home page with context-appropriate shortcuts.

---

## Session Types Reference

### Audit types (7)

| Type | Focus area |
|------|-----------|
| Security | OWASP, dependencies, secrets, auth |
| Performance | Load, latency, memory, CPU |
| Architecture | Patterns, coupling, complexity |
| Accessibility | WCAG, ARIA, keyboard nav |
| Eco-design | Carbon footprint, resource usage |
| Observability | Logs, metrics, traces, alerts |
| Privacy | GDPR, personal data, consent, retention |

### Review modes (5)

| Mode | Approach |
|------|---------|
| Standard | Balanced review across all dimensions |
| Adversarial | Actively tries to break the code |
| Edge cases | Focuses on boundary conditions and error paths |
| Standard + Adversarial | Two reviews in parallel, unified report |
| All | Runs all modes in parallel, merges results |

---

## Tips

- **Fuzzy matching** — type any part of a command name. `sec` matches `run audit`, `rev` matches `run review`.
- **Mode switching** — the TUI adapts available commands to your current mode. Project-specific commands only appear in project mode.
- **Config editing** — in config views, navigate with `j`/`k`, press `Enter` to edit a value. Changes auto-save.
- **Contextual shortcuts** — each view shows available shortcuts in the omnibar hint text at the bottom.
- **Project auto-detection** — if you launched `oh` from a registered project directory, it starts in project mode for that project.

---

## Inline Wizards

The `team init`, `init`, and `project add` commands launch **multi-step inline wizards** directly in the TUI without leaving the shell. The omnibar and toasts remain accessible throughout the wizard.

### Wizard shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+S` | Submit the current step (same as the button) |
| `Ctrl+B` | Go back to the previous step |
| `Esc` (1x) | Shows a "press again to skip" hint |
| `Esc` (2x) | Skips the step (blocked if the step is required) |

### Summary screen

After the wizard finishes, a summary screen displays all configured values. Two choices:
- `Enter` — navigate to the detail view (team.detail, settings, projects.list)
- `Esc` — return to the previous view

### First-run

If no provider is configured, the initial setup wizard launches automatically when the TUI starts. It guides through provider selection, credentials, and adding a first project.
