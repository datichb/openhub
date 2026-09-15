# ADR-032 — Contextual Team/Project Navigation

## Status

accepted

## Date

2026-09-07

## Context

The hub supports both solo projects (without a team) and team-based projects.
Currently, all views and omnibar commands are accessible from everywhere,
which creates several user experience problems:

- **Board confusion**: 3 "Board" entries in the omnibar (Project Board, Team Board,
  Home duplicate) with no clear distinction of their role.
- **Irrelevant commands**: team commands (Team Board, Team Status, Policies,
  Patterns) are visible even when no team is configured. Conversely, the Project
  Board (beads) is visible even without an active project.
- **Two disconnected ticket systems**: the team board (claims from the external
  tracker) and the project board (beads, development sub-tasks) have no link even
  though they are complementary. A tracker ticket should be decomposable into bead
  sub-tasks for development.
- **`ProjectModeView` exists** as a project context concept but is not fully
  realized. There is no equivalent "Team Mode".

The TUI's `resolveRepo()` uses `resolvedTeamConfig(a, project)` which requires a
`TeamID` on the project, whereas the `sync-tracker` CLI uses `a.Config.ActiveTeam()`
directly. This asymmetry means that tickets synchronized by the CLI do not appear
in the TUI.

## Decision

We decided to introduce **3 navigation modes** in the TUI, with automatic selection
based on configuration:

### Modes

| Mode | Activation | Available views | Board |
|------|-----------|-----------------|-------|
| **Hub** | Default (multi-team or multi-project without team) | Home, Settings, Providers, MCP, Projects, Teams | None (choose first) |
| **Team** | 1 team configured, or explicit selection | Team Board, Team Status, Activity, Policies, Patterns, Team Detail, Briefs | Team Board |
| **Project** | 1 active project, or explicit selection | Project Board, Project Config, Worktree, Sessions, Merge | Project Board |

### Automatic selection

- If only 1 team configured and no solo project → **Team mode** by default
- If no team and only 1 project → **Project mode** by default
- If multi-team or mix of team/solo → **Hub mode** (choose via Home)

### Omnibar filtering

Omnibar commands are filtered based on the active mode:
- In Team mode: only team commands + global commands (Settings, etc.)
- In Project mode: only project commands + global commands
- In Hub mode: all commands

### Tracker ↔ Beads link

Each bead ticket can reference a source ticket via a `source_ticket` field
(e.g., `gitlab:693`). On the team board, a `[3/7 tasks]` badge displays the
progress of linked bead sub-tasks. On the project board, a label identifies the
source tracker ticket.

### Navigation between modes

- `Ctrl+T`: switch to Team mode (or choose the team if there are several)
- `Ctrl+P`: omnibar (filtered by mode)
- Home: return to Hub mode

## Consequences

### Positive

- Clean interface: only relevant commands are visible
- Less confusion: a single board visible based on context
- The tracker ↔ beads link provides a complete view of the development cycle
- Automatic selection avoids an unnecessary choice when the config is simple
- Compatible with the existing architecture (ProjectModeView, shell, omnibar)

### Negative / Trade-offs

- TUI shell refactoring: the shell must maintain an `activeMode` and filter commands
- The tracker ↔ beads link requires a naming convention or a dedicated field on bead tickets
- Users accustomed to the flat omnibar will need to adapt to filtering
- Hub mode (multi-team) is an additional selection screen before accessing views

## Alternatives Considered

| Alternative | Rejected Because |
|-------------|-----------------|
| Single merged board (tracker + beads) | Too complex — the two systems have different data models. A tracker ticket has an assignee and a GitLab status, a bead ticket has sub-tasks and a complexity score. Merging would create a fragile abstraction. |
| Flat omnibar with filter tags | Does not solve the root problem — irrelevant commands remain listed, just filterable. The UX remains confusing. |
| Remove the project board (beads only via agents) | The project board is useful for local development visibility. Agents use it for sub-task tracking. Removing it would lose that visibility. |

## Affected Files

### Phase 1 (immediate — basic masking)

- `cli/cmd/tui_commands.go` — distinct labels, masking by config
- `cli/cmd/tui_team_board.go` — `ActiveTeam()` fallback in `resolveRepo()`
- `cli/internal/tui/v2/views/teamboard_view.go` — fix Mount empty-state
- `cli/internal/tui/v2/views/home.go` — remove omnibar duplicates

### Phase 2 (medium-term — tracker ↔ beads link)

- `cli/internal/beads/beads.go` — `source_ticket` field on bead tickets
- `cli/internal/tui/v2/views/teamboard_view.go` — `[N/M tasks]` badge per ticket
- `cli/internal/tui/v2/views/board_view.go` — source tracker label on tickets

### Phase 3 (long-term — contextual navigation)

- `cli/internal/tui/v2/shell/shell.go` — `activeMode` (hub/team/project), command filtering
- `cli/internal/tui/v2/views/home.go` — mode selection, auto-detection
- `cli/cmd/tui_commands.go` — `Mode` field on commands, dynamic filtering
- `cli/internal/tui/v2/views/view.go` — optional `ModeAware` interface on views

## Implementation Progress

> **Note (2026-09-14):** Phase 1 (basic masking — distinct labels, config-based command
> hiding, `ActiveTeam()` fallback fix, omnibar duplicate removal) has been implemented.
> Phase 2 (tracker ↔ beads link) and Phase 3 (full contextual navigation with mode
> switching) are pending.
