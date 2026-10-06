# TUI Reference — OpenHub

> Complete documentation for the OpenHub terminal user interface (TUI).

## Launch

```bash
oh          # Launches the TUI (interactive terminal detected)
oh --no-tui # Forces classic CLI mode
```

The TUI launches automatically when `oh` is executed without a subcommand in an interactive terminal. Environment variables that disable the TUI: `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

If no provider is configured (first launch), an inline setup wizard launches automatically inside the shell. See [InlineWizardView](tui-inline-wizard.en.md) for component details.

## Design Principles

The TUI follows an **omnibar-first** design inspired by fuzzy launchers (fzf, Telescope) and opencode's minimalist approach:

- **Single interaction point**: the omnibar handles all commands
- **Content-first**: maximum screen space dedicated to content
- **Contextual**: the interface adapts to the current view
- **Minimal shortcuts**: only 4 global keybindings to memorize

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                      CONTENT AREA                                 │
│              (adapts to the active view)                          │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  Ctrl+P command · Esc back · Ctrl+Q quit                    │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

## Global Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` | Activate the omnibar |
| `Esc` | Go back (previous view) |
| `Ctrl+Q` | Quit the TUI |
| Any letter | Activate omnibar with that character (if not consumed by the view) |

That's it. Four shortcuts. Everything else goes through the omnibar.

## The Omnibar

The omnibar is always visible at the bottom of the screen. It has two modes:

### Passive Mode (default)

Displays contextual hints for the active view:

```
│  Ctrl+P command · j/k nav · Enter open · h/l columns            │
```

### Active Mode (input)

Accepts text input with fuzzy suggestions displayed above:

```
│  ● Audit Sécurité      Audit de sécurité (OWASP, injections)    │
│  ○ Audit Performance   Audit performance (N+1, mémoire, CPU)    │
│  ○ Audit Architecture  Audit d'architecture (couplage, patterns) │
├───────────────────────────────────────────────────────────────────┤
│  > audit_                                                         │
```

### Omnibar Controls

| Key | Action |
|-----|--------|
| Type | Filter commands fuzzy |
| `Down` / `Tab` | Move selection down |
| `Up` / `Shift+Tab` | Move selection up |
| `Enter` | Execute selected command |
| `Esc` | Dismiss and return to content |

## Available Commands

### Workflows and sessions

| Command | Aliases | Description |
|---------|---------|-------------|
| `run <workflow>` | workflow id; former names: `dev` → `run ticket`, `start` → `run feature`, `onboard` → `run onboarding`, `audit`, `review` (`rev`, `cr`), `debug` (`dbg`), `feedback` → `run review-feedback` | Opens the launch form of the workflow (generated from the catalogue) |
| `run <workflow> ⟨ticket⟩` | — | On the board: workflow launched on the selected ticket |
| `workflows` | catalogue, wf | Workflow catalogue (read only) |
| `coder` | session, free | Free session (no workflow; opencode V1 or empty catalogue) |
| `sessions` | parallel, inbox | Sessions view |

### Launch form

Generated from the workflow YAML, in three steps: **Inputs** (one line per input: Beads ticket with a `Pick…` picker, checkbox for `bool`, list for `enum`, text area for `text`), **Options** (mode, runtime — unavailable environments show the reason —, location: base, existing worktrees, new worktree; opening), **Recap** (agents, first-turn budget, isolation, sessions and locations, warnings). `Ctrl+S` launches from any step, `Ctrl+B` goes back, `Esc` closes. A second launch while preparing is ignored.

### Start, board, catalogue

- **Start** (hub, project, team landings): ★ pinned (5 max), recent (3), suggestions when empty; in project/team mode, collapsed categories (Enter = pick a workflow). `*` pins or unpins (scope: hub, project or team depending on the landing). "All workflows (N)" opens the catalogue.
- **Board**: `a` on a ticket lists the workflows taking a Beads ticket; the form opens at the Options step, ticket prefilled.
- **Catalogue**: workflows by layer (version, risk, ⌂ ▣ ☁, validity), detail on the right; Enter launches, `*` pins.
- **Sessions view**: `e` "Chain with…" suggests the workflows taking an output of the session (branch, tickets), form prefilled.

### Projects

| Command | Aliases | Description |
|---------|---------|-------------|
| `board` | kanban, tasks | Project kanban board |
| `projects` | proj, list | Projects list |
| `deploy` | dep, push | Deploy agents/skills to the active project |
| `sync` | synchronize | Synchronize all projects |

### Configuration

| Command | Aliases | Description |
|---------|---------|-------------|
| `config` | cfg, settings, hub | Hub configuration |
| `models` | mod, model, llm | Model configuration |
| `provider` | prov, api | LLM provider settings |
| `mcp` | servers | MCP servers |

### System

| Command | Aliases | Description |
|---------|---------|-------------|
| `status` | stat, info | System status |
| `doctor` | health, check | Health diagnostics |
| `metrics` | met, stats, tokens | Usage statistics |
| `plugins` | plug, extensions | Plugin management |
| `upgrade` | up, update | Update opencode |
| `help` | ?, aide, shortcuts | Help and shortcuts |

### Navigation

| Command | Aliases | Description | Availability |
|---------|---------|-------------|--------------|
| `home` | accueil, welcome | Return to splash screen | Global |
| `project.mode` | project mode | Project Mode | `Ctrl+T` (Hub, Team modes) |
| `hub.mode` | hub mode | Hub Mode | `Ctrl+T` (Team, Project modes) |
| `workflow` | wf | Workflow configuration | Global |
| `quit` | exit, q | Quit the TUI | Global |

### Team (when enabled)

| Command | Aliases | Description |
|---------|---------|-------------|
| `team board` | team kanban | Team kanban board |
| `team status` | team stat | Team status |
| `team activity` | activite, feed | Team activity feed |
| `takeover briefs` | takeover | Takeover context briefs |
| `worktrees` | wt | Git worktree management |
| `patterns` | pat | Team patterns |
| `policies` | pol, rules | Team policies |

## View-Specific Shortcuts

When a view is active, it may have additional shortcuts that work without activating the omnibar. These are displayed in the omnibar's passive hint text.

### Board View (project)

| Key | Action |
|-----|--------|
| `h` / `←` | Previous column |
| `l` / `→` | Next column |
| `j` / `↑` | Previous item |
| `k` / `↓` | Next item |
| `r` | Refresh |
| `g` | Go to first item |
| `G` | Go to last item |
| `Enter` | Open item detail |

### Team Board View

The team board displays **5 columns**: TODO (`planned`), IN PROGRESS (`in_progress`), REVIEW (`review`), BLOCKED (`blocked`), DONE (`done`). Only active tickets appear — members with no current claims are not listed.

Tickets display compact label tags: `[AI]` (green) for `agent-reviewed`, `[!]` (yellow) for `needs-human-review`.

| Key | Action |
|-----|--------|
| `h` / `←` | Previous column |
| `l` / `→` | Next column |
| `j` / `↑` | Previous item |
| `k` / `↓` | Next item |
| `c` | Claim the selected ticket (assign to yourself) |
| `x` | Release the selected ticket |
| `t` | Transfer the selected ticket to another team member |
| `s` | Change the status of the selected ticket |
| `r` | Refresh (pulls latest from git + tracker if configured) |
| `g` | Go to first item |
| `G` | Go to last item |
| `q` / `Esc` | Back to hub |

### Projects View

| Key | Action |
|-----|--------|
| `a` | Add project |
| `d` | Delete project |
| `r` | Rename project |
| `Enter` | Configure project |

### Config View

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate keys |
| `Enter` | Edit selected value |

## Inline Wizards

Multi-step configuration flows (`team init`, `init`, `project add`) use an **InlineWizardView** component that runs inside the TUI shell without an alt-screen switch. The wizard is pushed onto the router stack via `shell.PushView(v)` and pops automatically on completion.

Shortcuts: `Ctrl+S` submit, `Ctrl+B` back, double `Esc` skip.

See [InlineWizardView Reference](tui-inline-wizard.en.md) for the full API and patterns for creating new wizards.

### Architecture — PushView

The `PushView(v View)` method on `ShellAccess` pushes an ephemeral view onto the router stack without pre-registration. The wizard automatically receives `ShellAccess` via the `shellAware` interface. This method is used by the `team init`, `init`, and `project add` omnibar actions.

## Opencode Sessions

When a session is launched (Start, Audit, Review, Debug, Quick):
1. The TUI suspends itself
2. opencode takes full terminal control
3. When opencode exits, the TUI resumes exactly where it was
4. A toast confirms the session outcome

## Team View Synchronisation

All team views (board, status, activity, takeover, patterns, policies) automatically pull the latest team-state git data when opened. Pressing `r` also triggers a pull before refreshing.

- A "Synchronising..." toast appears only if the pull takes more than 1 second
- On network error: a warning toast is shown and local data is used (no blocking)
