# ADR-034 — Inline Wizard in the TUI Shell

## Status

accepted

## Date

2026-09-14

## Context

Initial configuration flows (`oh init`, `oh team init`, `oh project add`) used two
distinct UI mechanisms in the TUI:

1. **Standalone wizards** — `views.RunWizard()` creates a separate `tview.Application`
   (alt-screen) with a full multi-step wizard (step bar, info panel, back navigation).
   Used by `oh init` (15 steps), `oh team init` (5 steps), `oh project add` (8 steps).

2. **Chained modals** — The TUI shell chained `ShowInputModal` → `ShowSelectModal` →
   `ShowInlineForm` via nested closures with `time.Sleep(50ms)` between each modal.
   Used by `actionTeamInit()` in the TUI.

Both approaches had problems:

- **Standalone wizard**: the alt-screen switch is jarring — the TUI shell disappears,
  a new tview.Application takes over the terminal, then the shell reappears. The omnibar,
  toasts, and all shell context are lost during the wizard. For the first-run wizard
  (`init_wizard.go`), the wizard ran *before* shell creation, preventing any shell
  interaction.

- **Chained modals**: the TUI team init only covered 3 of 5 CLI steps (missing global
  config, notifications, policies), collected only 3 of 6 identity fields (missing
  gitlab/mattermost/tracker username), did not commit-and-push members, had no back
  navigation, no step progress bar, and required `time.Sleep(50ms)` between modals to
  avoid tview freezes.

The functional gap between the CLI wizard (complete) and the TUI (incomplete) created
an inconsistent experience for users who preferred the TUI.

## Decision

We decided to create an **`InlineWizardView`** component that implements the `views.View`
interface and runs inside the existing TUI shell, pushed onto the router stack via a new
`shell.PushView(v)` method.

### Principles

1. **Reuse of `WizardStep` type** — the component accepts the same `WizardStep` struct
   (`wizard.go:28-75`) as the standalone wizard, allowing step definitions to be shared
   between CLI and TUI.

2. **Standard View** — `InlineWizardView` implements `View` (`ID`, `Title`, `Mount`,
   `Unmount`, `HandleKey`, `StatusHints`) and follows the same lifecycle as all other
   shell views. `Esc` naturally pops the wizard via the shell's global handler.

3. **Ephemeral push** — the wizard is created dynamically by an omnibar action and pushed
   via `PushView` without being pre-registered in the router registry. On completion,
   the user pops back to the previous view or navigates to a target view.

4. **Integrated layout** — the wizard builds its own internal layout (StepBar + header +
   swappable content + info panel + hints bar) in the content Flex provided by `Mount()`.
   The shell omnibar remains visible and accessible.

### `InlineWizardView` Component

```
┌─────────────────────────────────────────────────────┐
│ ● Repo ─── ◆ Config ─── ○ Identity ─── ○ Notifs    │  StepBar
├─────────────────────────────────────────────────────┤
│ ◆ 2/5 — Global configuration                       │  Header
├─────────────────────────────────────────────────────┤
│ [Form / CustomView / Spinner]                       │  Content
├─────────────────────────────────────────────────────┤
│ ✓ Repo: git@gitlab.com:team/state.git               │  Info panel
├─────────────────────────────────────────────────────┤
│ ctrl+s submit · ctrl+b back · esc skip              │  Hints
└─────────────────────────────────────────────────────┘
```

### `PushView` Method on `ShellAccess`

Added to the `ShellAccess` interface (`view.go`) and implemented on `Shell` (`shell.go`).
Pushes an ephemeral view onto the router stack without pre-registration, with automatic
`ShellAccess` injection if the view implements `shellAware`.

### Integrated Wizards

| Wizard | Omnibar command | Steps | File |
|--------|----------------|-------|------|
| Team init | `team init` | 6 (repo, HTTPS creds, config, identity, notifs, policies) | `tui_team_actions.go` |
| Hub init | `init` | 5 (welcome, provider, auth mode, credentials, first project) | `init_wizard.go` |
| Project add | `project add` | 8 (identity, beads, provider, provider config, agents, MCP, team, deploy) | `tui_actions.go` |

The first-run wizard (triggered when `DefaultProvider == ""`) is now pushed inline
after shell startup instead of running in a separate tview.Application before shell
creation.

## Consequences

### Positive

- Full functional parity between CLI and TUI for all 3 init wizards
- Team init TUI covers all 6 steps with 6 identity fields + commit-and-push
- Smooth user experience — no alt-screen switch, shell chrome preserved
- Reusable `InlineWizardView` component for any future wizard
- Back navigation (`Ctrl+B`), step bar, info panel, validation, async spinner
- Summary screen with contextual navigation proposal
- CLI standalone wizards (`RunWizard`) remain as `--no-tui` fallback

### Negative / Trade-offs

- Two wizard implementations coexist (`RunWizard` standalone + `InlineWizardView` inline)
- The omnibar stays visible during the wizard (5 fewer lines for content)
- Steps are defined twice for team init (CLI in `team.go`, TUI in `tui_team_actions.go`) — a later refactoring could factor them out

## Alternatives Considered

| Alternative | Reason for Rejection |
|-------------|---------------------|
| `SuspendAndExec` + standalone wizard | Jarring alt-screen switch — shell disappears and reappears, context lost |
| Enrich existing chained modals | No back navigation, no step bar, fragile nested closures, `time.Sleep` between modals |
| Replace `RunWizard` with `InlineWizardView` everywhere | CLI has no TUI shell — `RunWizard` is still needed for one-shot commands |

## Affected Files

- `views/inline_wizard.go` — `InlineWizardView` component (new)
- `views/inline_wizard_test.go` — 7 unit tests (new)
- `views/view.go` — added `PushView(v View)` to `ShellAccess`
- `shell/shell.go` — `PushView` implementation with `shellAware` injection
- `cmd/tui_team_actions.go` — rewrote `actionTeamInit()` as 6 inline steps
- `cmd/init_wizard.go` — added `buildFirstRunInlineWizard()`
- `cmd/tui.go` — first-run pushed inline after shell startup
- `cmd/tui_actions.go` — added `actionProjectAdd()` and `buildProjectAddInlineWizard()`
- `cmd/tui_commands.go` — omnibar commands `init` and `project.add`
