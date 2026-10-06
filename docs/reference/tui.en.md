> [Lire en français](tui.fr.md)

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
| `workflows` | catalogue, wf, workflow | Workflow catalogue (drafts, publication, history; replaces the former Workflow view) |
| `review.publish` | publish, mr | Create the merge request of the current branch (GitLab API, suspended terminal; when GitLab writes are enabled) |
| `coder` | session, code, free, libre | Launch form of the `libre` workflow (entry agent of your choice, `orchestrator` by default) |
| `sessions` | parallel, inbox | Sessions view |
| `bricks` | briques, agents, skills | Bricks catalogue (agents and skills: origin, cost, workflows) |

### Launch form

Generated from the workflow YAML, in three steps: **Inputs** (one line per input: Beads ticket with a `Pick…` picker, checkbox for `bool`, list for `enum`, text area for `text`), **Options** (mode, runtime — unavailable environments show the reason —, location: base, existing worktrees, new worktree; opening), **Recap** (agents, first-turn budget, isolation, sessions and locations, warnings). `Ctrl+S` launches from any step, `Ctrl+B` goes back, `Esc` closes. A second launch while preparing is ignored. With several tickets, a "A single session for every ticket" checkbox is offered. When a precondition suggests another workflow, the recap offers a "Run <wf> first (then come back)" button.

### Start, board, catalogue

- **Start** (hub, project, team landings): ★ pinned (5 max), recent (3), suggestions when empty; in project/team mode, collapsed categories (Enter = pick a workflow). `*` pins or unpins (scope: hub, project or team depending on the landing). "All workflows (N)" opens the catalogue.
- **Board**: `a` on a ticket lists the workflows taking a Beads ticket; the form opens at the Options step, ticket prefilled.
- **Catalogue**: workflows by layer (version, risk, ⌂ ▣ ☁, validity), detail on the right; Enter launches, `*` pins. With a team-state (team or solo space), the catalogue is **editable**:
  - sections Hub (read only), Team, Project, **My drafts** (`✎`, error count, "+ new brick" badge for a team brick used for the first time) and **Integrity** (skipped published files, refused bricks); `✎` on a published workflow = you have a draft of it, `⏳` = publication waiting for the network;
  - `n` new (empty, extend or duplicate the selected workflow; team or project layer; id), `e` edit (on a hub workflow: extend it), `v` validate, `t` test the draft (launch form "✎ draft", local), `p` publish, `D` diff of the draft with the impact, `h` history, `x` archive (optional reason; on a draft: discard it), `r` reload; Enter on a draft = test it;
  - **Publish**: current → next version, validation, impact (widenings marked ⚠), new bricks, diff of the document and the template, governance ("Publication: any member"); mandatory message, `Ctrl+S` publishes once; offline, the publication is queued and replayed at the next team-state synchronization;
  - **History**: versions (author, date, message, current); Enter = diff with the current version, `r` = restore (published again as a new version).
  - **Editor** (`e`, `n`): five sections (`Tab` / `Shift+Tab`) — **General** (identity, security, run: each field shows the resolved value, `✎` when written in the draft, `← hub:ticket` its origin, 🔒 when locked by the parent workflow; Enter edits, `x` goes back to the inherited value), **Graph** (resolved workflow, inherited elements included: start column, checkpoints in order with their behavior in the shown mode — `m` to change it —, agents under the checkpoint they wait for, independent agents apart; Enter edits the agent or checkpoint, `a` adds a catalogue agent, `c` a checkpoint, `x` removes — an inherited element is disabled), **Inputs & prompt** (inputs, `a` to add, template, `P` to write it in `$EDITOR`, prompt preview with example values), **Resources** (extra/denied skills, MCP, Beads, plugins, outputs), **Bundle preview** (agents, skills, first-turn budget, depth, isolation, MCP; without an active project, project MCP servers are "project dependent"; findings: Enter goes to the field, or to the YAML at the right line);
  - `u` / `U` undo / redo, `y` raw YAML in `$EDITOR`, `w` (or `Ctrl+S`) saves the draft (refused while errors remain), `Esc`: with unsaved changes, choose "Save and leave" (valid draft), "Drop the changes" or "Keep editing". YAML comments and layout are kept; only the changed fields are written (a patch stays a patch).
- **Team detail**: "Workflows" section with the publication governance, read only (`[governance] publish`); for a solo space, "Space" line and "Switch to a team" action (empty remote repository).
- **Sessions view**: `e` "Chain with…" suggests the workflows taking an output of the session (branch, tickets), form prefilled; a launch put on hold by a precondition ("run onboarding then come back") comes first. When a workflow session ends (or declares its outputs), a toast announces the suggested follow-up and the session detail shows it ("↪ Chain with review (e)").

### Projects

| Command | Aliases | Description |
|---------|---------|-------------|
| `board` | kanban, tasks | Project kanban board |
| `projects` | proj, list | Projects list |

`deploy` and `sync` no longer exist (v5: nothing is deployed into projects anymore); leftovers of former deployments are removed with `cleanup`.

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
| `cleanup` | deploy-cleanup, nettoyage, migrate | Cleanup screen for former deployments (`oh migrate deploy-cleanup`) |
| `help` | ?, aide, shortcuts | Help and shortcuts |

`plugins` and `upgrade` were removed in v5 (plugins are declared per workflow; opencode V2 is installed with its own tool).

### Navigation

| Command | Aliases | Description | Availability |
|---------|---------|-------------|--------------|
| `home` | accueil, welcome | Return to splash screen | Global |
| `project.mode` | project mode | Project Mode | `Ctrl+T` (Hub, Team modes) |
| `hub.mode` | hub mode | Hub Mode | `Ctrl+T` (Team, Project modes) |
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

When a session is launched (launch form, `coder`, Start):
1. oh builds the session bundle and starts the session on the `opencode serve` server of its group
2. The opencode client opens alongside (iTerm2/Terminal tab or window, tmux, browser); the TUI stays usable
3. Closing opencode does not stop the session: follow it and reopen it from the **Sessions** view
4. Suspending the TUI is only a last resort (`attach = "suspend"`)

See [Sessions v5](../guides/sessions-v5.en.md).

## Team View Synchronisation

All team views (board, status, activity, takeover, patterns, policies) automatically pull the latest team-state git data when opened. Pressing `r` also triggers a pull before refreshing.

- A "Synchronising..." toast appears only if the pull takes more than 1 second
- On network error: a warning toast is shown and local data is used (no blocking)
