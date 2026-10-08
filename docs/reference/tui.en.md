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
- **Minimal shortcuts**: a few global keys, the rest in the omnibar or the view hint line

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
| `Esc` | Back (previous view); at the root of Project or Team mode: back to Hub mode |
| `Ctrl+T` | Team mode (selector when several teams); from Team mode: back to Hub |
| `Ctrl+Q` / `Ctrl+C` | Quit the TUI (when sessions are working: finish the step then sleep, background, or stop now) |
| `?` / `F1` | Help and shortcuts |
| `j` / `k`, `g` / `G` | Down / up, first / last item (in any list) |
| `d` | Dismiss the oldest visible toast |
| `Enter` | Activates the omnibar when the view does not use it |
| Any other letter | Activates the omnibar with that character (when the view does not use it) |

Everything else goes through the omnibar or the view keys (shown in the hint line).

The mode bar shows the sessions badge **`● N ⏸ M`** (N working sessions, M pending decisions); it turns to alert when a decision is waiting.

## The Omnibar

The omnibar is always visible at the bottom of the screen. It has two modes:

### Passive Mode (default)

Shows contextual hints for the active view:

```
│  Ctrl+P command · j/k nav · Enter open · h/l columns            │
```

### Active Mode (input)

Accepts input with fuzzy suggestions above:

```
│  ● run audit           Project audit (security, performance…)     │
│  ○ run review          Code review of a branch…                   │
│  ○ run debug           Diagnose a bug or an isolated problem      │
├───────────────────────────────────────────────────────────────────┤
│  > audit_                                                         │
```

### Omnibar Controls

| Key | Action |
|-----|--------|
| Typing | Filter commands (fuzzy, on the label and aliases) |
| `↓` / `Tab` | Move down in suggestions |
| `↑` / `Shift+Tab` | Move up in suggestions |
| `Enter` | Run the selected command |
| `Esc` | Close and return to content |

## Available Commands

The Command column gives the displayed label; an alias can be typed too. Each alias has a single meaning (`q`: quit, `hub`: Hub mode, `tokens`: secrets, `team config`: configure the project's team, `libre`: `run libre`). "Project": Project mode only; "Team": Team mode only; "Project/Team": both; otherwise everywhere.

### Workflows and sessions

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| `run <workflow>` | the workflow id; former names: `dev`, `start.dev` → `ticket` · `start`, `orchestrator` → `feature` · `fast` → `quick` · `secu`, `security`, `perf`, `archi`, `a11y` → `audit` · `rev`, `cr` → `review` · `dbg`, `debugger`, `diag` → `debug` · `onboard`, `start.onboard` → `onboarding` · `feedback`, `rf`, `retours` → `review-feedback` | — | Launch form of the workflow (one command per workflow of the catalogue) |
| Free session | `coder`, session, code, free | Project/Team | Launch form of the `libre` workflow (entry agent of your choice, `orchestrator` by default) |
| Sessions | `sessions`, parallel, par, multi, inbox, à traiter, decisions | — | Sessions view |
| Workflows | `workflows`, catalogue, catalog, wf, workflow | — | Workflow catalogue (drafts, publication, history) |
| Bricks | `bricks`, briques, agents, skills, catalogue des briques | — | Brick catalogue (agents and skills: origin, cost, workflows) |
| Review Publish | `review.publish`, publish, mr | Project/Team | Create the MR of the current branch (`oh review --publish`, terminal suspended); only present when GitLab write is enabled |

### Launch form

Generated from the workflow YAML (title "Launch · <workflow>"), in three steps:

1. **Inputs**: one line per input (Beads ticket with a `Pick…` picker, checkbox for `bool`, list for `enum`, text area for `text`); "This workflow has no input" otherwise.
2. **Options**: **Mode**; **Runtime** (`⌂ local`, `▣ container`, `☁ remote`; an unavailable environment shows the reason; in a container: engine, cached image or image to build with the estimated duration); **Location** (`base`, existing worktrees, `+ new worktree`); **Opening** (auto, iTerm2, Terminal.app, tmux, browser, this terminal, do not open).
3. **Recap**: agents, first-turn budget, isolation, sessions and locations ("N sessions · 1 server · one worktree per writing session"), warnings.

`Tab` next field, `Ctrl+S` launches from any step, `Ctrl+B` goes back, `Esc` closes. A second launch while preparing is ignored. With several tickets, a "A single session for every ticket" checkbox is offered. When a precondition suggests another workflow, the recap offers "Launch <wf> first (then come back)".

### Start, board, catalogue

- **Start** (home, project, team): ★ pinned (5 max per scope), **Recent** (3), suggestions when nothing is pinned nor recent (`ticket`, `feature`, `review`); the project default workflow comes first ("default" badge); in Project/Team mode, folded categories (Develop, Frame, Quality, Knowledge, Other; Enter = pick the workflow). `*` pins or unpins (scope: hub, project or team depending on the home). "All workflows (N)" opens the catalogue; "Free session" opens the `libre` form.
- **Board**: `a` on a ticket opens "Launch on <ticket>" with the workflows that take a Beads ticket; the form opens at the Options step, ticket prefilled.
- **Catalogue** (`workflows`): workflows by layer (Hub · built in, Team, Project; version, risk, ⌂ ▣ ☁, validity), detail on the right (entry, chain, inputs, runtimes). Enter launches, `*` pins. With a team-state (team or solo space), the catalogue is **editable**:
  - sections Hub (read only: `e` extend, `n` duplicate), Team, Project, **My drafts** (`✎`, error count, "new brick" badge for a team brick used for the first time) and **⚠ Integrity** (skipped published files, refused bricks); `✎` on a published workflow = you have a draft of it, `⏳` = publication waiting for the network;
  - `n` new (Empty, Extend a workflow (patch), Duplicate a workflow (copy); team or project layer; id; without a team-state, offers to create a solo space, then goes on with the new workflow form), `e` edit (on a hub workflow: extend it), `v` validate, `t` test the draft ("✎ draft" form, local), `p` publish, `D` draft diff with impact, `h` history, `x` archive (optional reason; on a draft: discard it), `r` reload; Enter on a draft = test it;
  - **Publish**: current version → next, validation, impact (widenings marked ⚠), new bricks, diff of the document and the template, governance ("Publication: any member"); message required, `Ctrl+S` publishes once; offline, the publication is queued and replayed at the next team-state sync (toast "N pending publication(s) replayed");
  - **History**: versions (author, date, message, current); Enter = diff with the current version, `r` = restore (republished as a new version).
  - **Editor** (`e`, `n`): five sections (`Tab` / `Shift+Tab`) — **General** (identity, security, runtime: each field shows the resolved value, or the written value (`✎`) when the draft sets it — even refused, the error says why; booleans as `on` / `off`, `← hub:ticket` its origin, 🔒 when locked by the parent workflow; Enter edits, `x` reverts to the inherited value), **Graph** (start column, checkpoints in order with their behavior in the displayed mode — `m` to change it —, agents under the checkpoint they wait for, independent agents apart; Enter edits, `a` adds an agent from the catalogue, `c` a checkpoint, `x` removes — an inherited element is disabled), **Inputs & prompt** (inputs, `a` add, template, `P` to write it in `$EDITOR`, prompt preview), **Resources** (added/denied skills, MCP, Beads, plugins, outputs), **Bundle preview** (agents, skills, first-turn budget, depth, isolation, MCP; diagnostics: Enter goes to the field, or to the YAML at the right line);
  - `u` / `U` undo / redo, `y` raw YAML in `$EDITOR`, `w` (or `Ctrl+S`) save the draft (refused while errors remain), `Esc`: with unsaved changes, "Save and quit", "Drop the changes" or "Keep editing". YAML comments and layout are kept; only changed fields are written.
- **Brick catalogue** (`bricks`): Agents and Skills sections (origin hub or team, ~tokens, number of workflows using them), detail on the right (kind, family, mode, skills, requires, loaded by, estimated cost, workflows); `/` search (id, name, description), `f` filter (all, agents, skills).
- **Team detail**: "Workflows" section with the publication governance, read only; for a solo space, "Space" line ("solo (local)") and "Switch to a team" action (URL of an empty remote repository); the tracker, notifications and collaboration, off in a solo space, are not shown.

### Sessions view

Sections: **To handle** (pending decisions), **Running**, **Sleeping**, **Finished, 7 days**, **To fetch** (finished remote sessions). The detail on the right shows the session (workflow, runtime ⌂ ▣ ☁, location, cost, decisions, outputs, suggested next step "↪ Chain with … (e)").

| Key | Action |
|-----|--------|
| `Enter` | Open the selected decision (checkpoint form, question, permission; settled elsewhere — tool interface, another window — the form closes by itself); on a session without decision: show / hide the feed |
| `y` / `n` | Permission: allow once / reject; checkpoint: validate / fix first |
| `x` | Dismiss an error, a budget overrun or a circuit breaker |
| `a` | Attach (open opencode on the session) |
| `A` | Pick the opening method (automatic, iTerm2, Terminal.app, tmux, browser, here) |
| `w` | Open in the browser |
| `t` | Show / hide the session feed |
| `m` | Send an instruction (taken at the next step) |
| `i` | Interrupt the current turn |
| `M` | Switch the model of the next steps (`provider/model`) |
| `s` | Stop the session (confirmation) |
| `c` | Resume a sleeping session |
| `o` | Results (merge request description) |
| `e` | Chain with… (workflows that take an output of the session) |
| `g` | Fetch a remote session (artifacts, Beads journal replay, conflicts) |
| `f` | Filter: active project / every project |
| `r` | Refresh |

**Checkpoint form** (`⏸ <checkpoint>`): session changes (`+N −M · K file(s)`, "Full diff"), last messages, timeline; then **Decide**: Validate, Fix first or Other instruction, with a message to the agent (required for the last two). A circuit breaker shows "Circuit breaker: N delegations in a row without the user".

**Remote fetch** (`g`): the session is imported, then "Replay the journal (N op.)"; a Beads conflict offers Keep local, Apply remote, Merge the notes or Later.

### Projects

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| Project Board | `board`, kanban, tasks, project board | Project | Kanban of the active project |
| Init Board | `board.init`, beads init, init board, init tickets | — | Initialize Beads in the project |
| Projects | `projects`, proj, list | — | Project list |
| Add project | `project.add`, project add, add project, nouveau projet | — | Project creation wizard |

`deploy` and `sync` no longer exist (v5: nothing is deployed into projects); leftovers of former deployments are removed with `cleanup`.

### Configuration

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| Settings | `settings`, config, cfg, hub config | — | Hub settings |
| Project Config | `project-config`, config projet, project config | Project | Project configuration (including Execution) |
| Models | `models`, mod, model, llm | — | Models |
| Provider | `provider`, prov, api | — | LLM provider |
| MCP | `mcp`, servers | — | MCP servers |
| Secrets & Tokens | `secrets`, tokens, credentials, keychain | — | Keychain secrets |
| Teams | `teams`, team, equipe, equipes | — | Team list |
| Team Configuration | `team-detail`, tracker, sync, team detail | Team | Team detail and configuration |
| Discover Tracker | `team.discover`, tracker discovery, discover, configurer tracker | Team | Configure the board columns from the tracker |
| Configure hub | `init`, setup, reconfigure, configurer | — | Setup wizard (first run) |

**Settings** (`settings`), in order: General, CLI, Opencode, Sessions (opening, iTerm2 style, sleep), **Session restrictions** (off when empty: max working sessions, budget per session, daily budget, memory cap, allowed models), **Execution** (default runtime, container engine, image cache, pinned opencode version, strict isolation), Workflows (link to the catalogue), MCP GitLab / Jira / Figma / Google Slides, Worktree, Tracker (local overrides), **Remote (GitLab CI)** (per target: instance · group, oh-runner project, runner tag, image builder Kaniko or Docker-in-Docker, architecture, maximum job duration, GitLab token; "Check or complete" runs `oh remote setup`). Keys: `Enter` / `e` edit, `Space` toggle, `u` undo, `r` reload.

**Project Config › Execution**: dev Dockerfile (empty = detected; none = oh default image Debian + git), build args (`KEY=value`, comma separated), cache volumes, default workflow, default runtime (empty = settings / workflow).

### System

| Command | Aliases | Description |
|---------|---------|-------------|
| Status | `status`, stat, info | System status |
| Doctor | `doctor`, doc, health, check | Health check |
| Metrics | `metrics`, met, stats, usage | Usage statistics |
| Notifications | `notifications`, notif, logs, messages, toasts, erreurs | Notification history |
| Clean the former deployments | `cleanup`, deploy-cleanup, nettoyage, migrate | Cleanup screen (`oh migrate deploy-cleanup`) |
| History Export / History Import | `history.export`, `history.import` (export history, import history…) | Export / import of the session history |
| Help | `help`, ?, aide, shortcuts | Help and shortcuts |

**Cleanup screen** ("Cleanup of the former deployments"): list of projects with `oh deploy` leftovers; "Show the diff" (`opencode.json` diff), "Clean", "Later". It is also offered once at startup when leftovers are found.

`plugins` and `upgrade` were removed in v5 (plugins declared per workflow; opencode V2 is installed with its own tool).

### Navigation

| Command | Aliases | Description | Availability |
|---------|---------|-------------|--------------|
| Home | `home`, accueil, welcome | Back to home | Everywhere |
| Project Mode | `project.mode`, projet, project, focus | Switch to Project mode | Hub and Team modes |
| Hub Mode | `hub.mode`, hub, complet, retour | Back to Hub mode | Team and Project modes |
| Quit | `quit`, exit, q | Quit the TUI | Everywhere |

Team mode is entered with `Ctrl+T` (or by picking a team on the home screen).

### Team

| Command | Aliases | Mode | Description |
|---------|---------|------|-------------|
| Team Board | `team.board`, team board, team kanban, equipe board | Team | Team kanban |
| Status | `team.status`, team stat, status team | Team | Team status |
| Activity | `team.activity`, activite, feed, activity | Team | Recent activity |
| Team History | `history.team`, team history, team sessions | Team | Session history of the team (Activity view) |
| Takeover Briefs | `team.briefs`, takeover, briefs, reprises | Team | Context takeover briefs |
| Patterns | `team.patterns`, pat, patterns | Team | Team patterns |
| Policies | `team.policies`, pol, rules, policies | Team | Team policies |
| Wiki | `team.wiki`, wiki, proposals, pending | Team | Wiki proposals |
| Sync Tracker | `team.sync`, sync tracker, synchroniser tracker | Team | Sync the tracker |
| Project team mode | `team.configure`, team projet, configurer equipe | Team | Team attached to the project |
| Initialize team | `team.init`, team init, initialiser | — | `team init` wizard |
| Rejoin a team | `team.rejoin`, team rejoin, rejoindre, rejoin | — | Rejoin an existing team |
| Worktrees | `worktrees`, wt, git worktree | — | Git worktree management |

## View-Specific Shortcuts

When a view is active, these keys work without activating the omnibar. They are recalled in the hint line. `j` / `k` (down / up) and `g` / `G` (first / last) work everywhere.

### Board View (project)

| Key | Action |
|-----|--------|
| `h` / `←` | Previous column |
| `l` / `→` | Next column |
| `Enter` | Ticket detail |
| `a` | Actions: launch a workflow on the ticket |
| `L` | Link the ticket to the tracker |
| `r` | Refresh |
| `i` | Initialize Beads (project without Beads) |

### Team Board View

Columns of the team-state `[board]`; by default **6 columns**: TODO, IN PROGRESS, REVIEW, VALIDATION, DONE, BLOCKED. Tickets show compact labels: `[AI]` (green) for `agent-reviewed`, `[!]` (yellow) for `needs-human-review`.

| Key | Action |
|-----|--------|
| `h` / `←`, `l` / `→` | Previous / next column |
| `[` / `]` | Previous / next project tab |
| `c` | Claim the ticket (assign yourself) |
| `x` | Release the ticket |
| `t` | Transfer to another member |
| `s` | Change the status |
| `a` | Quick actions (including launching a workflow) |
| `/` | Search |
| `f` | Filter |
| `r` | Refresh (tracker sync when configured, then git pull) |

### Projects View

| Key | Action |
|-----|--------|
| `Enter` | Configure the project |
| `a` | Add a project |
| `d` | Remove |
| `n` | Rename |
| `m` | Move (new path) |
| `p` | Switch to Project mode |
| `b` | Initialize Beads |
| `r` | Refresh |

### Config View (Settings)

| Key | Action |
|-----|--------|
| `Enter` / `e` | Edit the value |
| `Space` | Toggle a boolean |
| `u` | Undo the last change |
| `r` | Reload |

### Teams View

| Key | Action |
|-----|--------|
| `a` | Add a team |
| `d` | Remove |
| `s` | Sync |
| `u` | Undo |
| `r` | Refresh |

### Team Status, Activity, Takeover Briefs, Worktrees views

| View | Keys |
|------|------|
| Team status | `r` refresh |
| Activity | `t` today, `w` week, `0` all, `r` refresh |
| Takeover briefs | `Enter` see the brief, `e` enrich (AI, `brief-enrich` workflow), `r` refresh |
| Worktrees | `a` add, `d` remove, `o` open in a terminal, `p` prune, `x` remove merged worktrees, `r` refresh |

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
