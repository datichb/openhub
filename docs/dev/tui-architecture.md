# TUI Architecture — Developer Guide

> Internal architecture of the omnibar-first TUI shell, interfaces, and guide for adding new commands/views.

## Overview

```
oh (no args) → cmd/tui.go → shell.New(cfg).Run()
                 ├─ cmd/tui_commands.go : buildCommands()  ← commands of the omnibar
                 └─ cmd/tui_views.go    : buildViews()     ← views and their wiring (services, callbacks)

cli/internal/
├── glyph/              ← Table of the symbols of the session states and decision kinds (CLI and TUI)
└── tui/
    ├── theme/          ← Single source of truth for colors and styles
    │   ├── colors.go   ← Hex constants (Catppuccin Mocha)
    │   ├── helpers.go  ← ColorTag(hex), TagColor, TagReset (tview tags, empty with NO_COLOR)
    │   ├── tcell.go    ← tcell.Color vars
    │   ├── lipgloss.go ← lipgloss.Color + Style vars
    │   ├── huh.go      ← AurumTheme() for charmbracelet/huh
    │   ├── modal.go, button.go ← modal and button styles
    │   ├── nocolor.go  ← NO_COLOR support
    │   └── icons.go    ← Icon constants
    ├── common/         ← Terminal capability detection (detect.go: --no-tui, TTY)
    ├── progress/       ← Spinner of the CLI (non-TUI)
    ├── components/     ← BubbleTea inline (floating, summary)
    └── v2/
        ├── shell/      ← Main application container
        │   ├── shell.go          ← Shell struct (Pages + Content + Omnibar), global key handler
        │   ├── command.go        ← Command struct + CommandRegistry + fuzzy search
        │   ├── omnibar.go        ← Persistent omnibar widget
        │   ├── overlay.go, modal.go, confirm.go ← inline overlays, modals, confirmations
        │   ├── toast.go, notifications.go, tui_log_handler.go ← toasts and their history
        │   ├── selection.go, clipboard.go ← mouse selection and copy
        │   ├── help.go           ← help overlay (built from the keys package)
        │   └── layout.go         ← terminal size levels (N20)
        ├── router/     ← Navigation stack (push/pop/replace)
        ├── layout/     ← Unified layout builder (header, sidebar, content, status bar)
        ├── keys/       ← Key convention: one keys.Action per action (see tui-conventions.md)
        ├── widgets/    ← Reusable tview primitives (SectionedList, InlineSelect, StepBar, ListDetailPage, graphs…)
        └── views/      ← All navigable views
            ├── view.go          ← View, ShellAccess and the optional interfaces
            ├── home*.go         ← Hub Home (home.go, home_common.go, home_layout.go)
            ├── project_mode_view.go, team_mode_view.go ← Project Home and Team Home
            ├── action_list.go   ← ActionList component (items, detail, actions)
            ├── list_page.go     ← List + detail page shared by the list views
            ├── standalone.go    ← RunInlineWizardStandalone (wizard without the shell)
            ├── sync.go          ← Async git pull of the team views
            └── …                ← one file (or a few) per view: sessions_*, settings_*, launch_form*, wiki_*…
```

## Shell Architecture

```go
type Shell struct {
    app           *tview.Application
    pages         *tview.Pages       // "main" page + overlay pages (toasts, suggestions, inline overlays)
    root          *tview.Flex        // vertical layout: content + suggestions + omnibar
    content       *tview.Flex        // swappable content zone (managed by router)
    omnibar       *Omnibar           // persistent input at bottom
    router        *router.Router     // view navigation stack
    registry      *CommandRegistry   // flat command list for omnibar
    notifications *NotificationStore // history of the toasts (Notifications view)
    selection     *SelectionManager  // mouse selection and copy
    activeMode    views.Mode         // Hub, Project, or Team (ADR-032)
    activeProject *views.ActiveProject
    activeTeam    *views.ActiveTeam
    ctx           context.Context    // cancelled on exit
}
```

Layout:
```
Pages
└── "main" page
    └── Flex (column)
        ├── content (proportion 1) ← full screen
        ├── suggestions            ← omnibar suggestions, when shown
        └── omnibar (fixed rows)   ← always visible (can be hidden: SetOmnibarVisible)
```

## Navigation Modes (ADR-032)

The TUI has 3 navigation modes, each with its own home page:

| Mode | Home View ID | Home View | Auto-detection |
|------|-------------|-----------|----------------|
| Hub | `home` | `HomeView` | Default, or multiple teams/projects |
| Project | `project.mode` | `ProjectModeView` | Single active project |
| Team | `team.mode` | `TeamModeView` | Single active team |

Mode switching: `Ctrl+T`, or selecting a team/project from the Hub Home.

### Home page shared patterns (`home_common.go`)

All 3 home views share common behavior extracted into `home_common.go`:

- **`homeHandleKey()`** — Common key handling: dual-column nav (h/l/Tab), Enter selection. (v5: the former `r` deploy refresh of ProjectModeView was removed with deployment.)
- **`splitBySectionID()`** — Column distribution using structural `SectionID` field (immune to i18n label changes).
- **`toSectionItem()`** — Converts generic `homeSectionItem` to `widgets.SectionItem`.

### Async data loading in home pages

Hub Home loads Teams and Projects asynchronously to avoid blocking the UI thread:

```go
// Mount() pattern:
1. v.items = v.buildStaticItems()       // System + Actions (instant)
2. adaptiveHomeMount(app, content, buildFn) // First frame with static items
3. go func() {                          // Background I/O
     items := v.buildItems()            // ListTeams + ListProjects (10-20ms)
     app.QueueUpdateDraw(func() {
       if v.mountGen != gen { return }  // Guard: discard stale goroutines
       v.items = items
       content.Clear()
       adaptiveHomeMount(...)           // Full rebuild with fresh data
     })
   }()
```

**Key constraint**: `buildFn` must read `v.items` at call-time (not captured slices) so that resize and async data arrival don't fight each other.

## Command Registry

All user-triggerable actions are `Command` structs in a flat registry:

```go
type Command struct {
    ID          string         // "start", "audit.security"
    Label       string         // Display text in suggestions
    Aliases     []string       // Additional fuzzy match terms
    Description string         // One-line help text
    Category    string         // Visual grouping in suggestions
    ViewID      string         // Navigate to view (or "")
    Action      func()         // Direct action callback (or nil)
    Enabled     func() bool    // Dynamic availability
    Priority    int            // Order when the omnibar query is empty
    RunsDirect  bool           // Run on the event loop (SuspendAndExec)
    Modes       []views.Mode   // Visible modes (nil = all)
}
```

Commands are defined in `cmd/tui_commands.go:buildCommands()`. Adding a new command requires only appending to this list. A view can also add commands while it is active (`CommandProvider.ContextCommands()`).

## View Interface

```go
type View interface {
    ID() string
    Title() string
    Mount(content *tview.Flex, app *tview.Application)
    Unmount()
    StatusHints() string
    HandleKey(event *tcell.EventKey) *tcell.EventKey
}
```

Key lifecycle:
- `Mount()`: called when navigating TO this view. Build widgets, add to content Flex.
- `Unmount()`: called when navigating AWAY. Cleanup timers, channels.
- `HandleKey()`: return `nil` to consume the event, return `event` to let it propagate to the shell (which then activates the omnibar for unhandled runes).
- `StatusHints()`: text displayed in omnibar passive mode. Build it with the `keys` package so that it follows the convention:

```go
func (v *MyView) StatusHints() string {
    return keys.Hints(keys.Navigate.Hint(), keys.Open.Hint(),
        keys.Create.Hint(), keys.Delete.Hint(), keys.Refresh.Hint(), keys.Back.Hint())
}
```

Optional interfaces (`views/view.go`), checked by the shell with a type assertion:

| Interface | Role |
|-----------|------|
| `InputCapturing` | The view keeps all the keys (forms, wizards); the shell keeps only Ctrl+Q/C, F1 and the selection |
| `ShellKeysPassing` | An `InputCapturing` view without text field lets the shell take `?` and Ctrl+P |
| `HintsRefresher` | Implemented by the shell: the view calls `RefreshHints()` when its keys change (tabs, batch) |
| `CommandProvider` | `ContextCommands()`: omnibar commands that exist only while the view is active |
| `LegendProvider` | A legend of the symbols shown by the view (help overlay) |
| `ModeAware`, `ModeInfoProvider` | Modes in which the view exists; information of the mode bar |
| `OverlayCloser` | The shell closes an overlay the view showed once what it asks is settled |

## Key Event Flow

```
tview.Application.InputCapture (globalKeyHandler)
    │
    ├── Ctrl+Q / Ctrl+C → RequestQuit (BeforeQuit dialog of the config, then Stop)
    ├── terminal too small → only quitting works
    ├── Esc with a text selection → clears the selection
    ├── [omnibar active?] → omnibar handles all input
    ├── [inline overlay?] → overlay handles input
    ├── F1 → help overlay
    │
    ├── [view is InputCapturing?] → view.HandleKey(event), then tview focus delivery
    │
    ├── Esc → closes the toasts, then router.Pop() (or back to the hub)
    ├── Ctrl+P → omnibar.Activate(); Ctrl+T → toggle team mode; ? → help overlay
    │
    ├── view.HandleKey(event)
    │   ├── returns nil → consumed, done
    │   └── returns event → continues below
    │
    └── [rune?] → omnibar.ActivateWithRune(r)
```

## Omnibar

The omnibar (`shell/omnibar.go`) is a persistent widget with two modes:

1. **Passive**: `tview.TextView` showing contextual hints from `view.StatusHints()`
2. **Active**: `tview.InputField` with fuzzy-search suggestions in a `Pages` overlay

Activation: `Ctrl+P`, or any unhandled rune from `globalKeyHandler`.

## ShellAccess (what a view can ask of the shell)

Views reach the shell through `views.ShellAccess` (`views/view.go`), set by the shell with `SetShell`:

```go
type ShellAccess interface {
    Context() context.Context                       // cancelled on exit: use it in goroutines
    ShowInputModal(title, currentValue string, onConfirm func(newValue string))
    ShowPasswordModal(title string, onConfirm func(value string))
    ShowSelectModal(title string, options []SelectOption, currentValue string, onConfirm func(value string))
    ShowMultiSelectModal(title string, options []SelectOption, selected []string, onConfirm func(selected []string))
    ShowScrollableModal(title, content string, actions []ModalAction)
    ShowToastMsg(msg string, success bool)
    ShowInlineForm(cfg InlineFormConfig)
    Confirm(title, detail string, danger bool, onYes func()) // every destructive action goes through it

    NavigateTo(viewID string)
    SetProjectMode(project *ActiveProject)          // deprecated: SetMode + SetActiveProject
    SetActiveProject(project *ActiveProject)
    ActiveProject() *ActiveProject
    SetMode(mode Mode)                              // Hub, Project or Team
    Mode() Mode
    SetActiveTeam(team *ActiveTeam)
    ActiveTeam() *ActiveTeam

    PushView(v View)                                // ephemeral view (inline wizard), SetShell is called
    PopView() bool
    SetOmnibarVisible(visible bool)
}
```

These render as **inline overlays** in the `Pages` system rather than traditional popup modals. `OverlayCloser` and `HintsRefresher` are implemented by the shell on top of this interface. Views must be testable without the shell: `views_test.go` has a `mockShell`, and `standalone.go` has a minimal stub (`standaloneShellStub`) for wizards run outside the TUI.

## Display rules

- Every displayed text goes through `i18n.T` / `i18n.Tf`; the translations hold text only (no colour).
- Colours come from the theme: `theme.ColorTag(theme.SuccessHex)` + text + `theme.TagColor`, never a colour name such as `[yellow]`.
- A text that does not come from the code (agents, files, config, names) is escaped with `tview.Escape`. A literal bracket word that must stay visible is written `[word[]` (the tview escape), never `[word]` (it would be read as a colour or a style tag and disappear).
- See [`tui-conventions.md`](tui-conventions.md) for the keys, the `keys` package and the `ActionList` component, and [`tui-testing.md`](tui-testing.md) for the tests.

## How to Add a New Command

### 1. Add to `cmd/tui_commands.go:buildCommands()`

```go
{
    ID:          "my.command",
    Label:       "My Command",
    Aliases:     []string{"mc", "myalias"},
    Description: "Does something useful",
    Category:    "Projets",
    Action:      myActionFunc,  // OR ViewID: "my-view"
},
```

### 2. If it navigates to a view, create the view

Implement the `View` interface and register the view in `cmd/tui_views.go:buildViews()`.

### 3. Done

No menu restructuring needed. The command appears in the omnibar immediately.

## How to Add a New View

### 1. Create the file

```
cli/internal/tui/v2/views/myview.go
```

### 2. Implement the interface

```go
type MyView struct { ... }

var _ View = (*MyView)(nil)

func (v *MyView) ID() string          { return "my-view" }
func (v *MyView) Title() string       { return "My View" }
func (v *MyView) StatusHints() string  { return keys.Hints(keys.Navigate.Hint(), keys.Open.Hint(), keys.Back.Hint()) }
func (v *MyView) Mount(content *tview.Flex, app *tview.Application) { ... }
func (v *MyView) Unmount() { ... }
func (v *MyView) HandleKey(event *tcell.EventKey) *tcell.EventKey { ... }
```

### 3. Register in `cmd/tui_views.go:buildViews()`

`buildViews` builds the services and callbacks the view needs, then returns the list of views (`allViews`):

```go
allViews = append(allViews, views.NewMyView(views.MyViewConfig{ /* services, callbacks */ }))
```

### 4. Add a command to reach it

```go
{ID: "myview", Label: "My View", ViewID: "my-view", Category: "..."},
```

### 5. Add a test

The `var _ View = (*MyView)(nil)` line of step 2 is the interface check (compile time): no test is written for it. Add the view to the `TestViews_MountUnmount` table of `views_test.go` (Mount, ID, Title, Unmount), and test its behaviour (keys, callbacks) and its rendering (golden) as described in [`tui-testing.md`](tui-testing.md).

## Async Pull Pattern (Team Views)

The team views share an async pull pattern defined in `cli/internal/tui/v2/views/sync.go`.

### Helpers

```go
// syncAsync pulls git in background via a teamstate.TeamStateWriter.
// Shows a "Synchronisation..." toast if the pull takes > 1s.
// Calls onDone on the tview event loop after completion, with the pull error (nil on success).
// On error, a warning toast tells that the data is local, then onDone still runs.
func syncAsync(app *tview.Application, repo teamstate.TeamStateWriter, shell ShellAccess, onDone func(pullErr error))

// syncFuncAsync is identical but accepts a bare func() error instead of a repo.
// Used by TeamBoardView, which receives its pull function via SyncFunc in its config struct.
func syncFuncAsync(app *tview.Application, pullFn func() error, shell ShellAccess, onDone func(pullErr error))
```

### Usage contract

- Called on `Mount()` to eagerly refresh data when the view is entered.
- Bound to the `r` key in each view's `HandleKey()` for manual refresh.
- The toast ("Synchronisation...") is displayed only when the pull exceeds 1 second, avoiding flicker on fast pulls.
- `onDone` is always invoked via `app.QueueUpdateDraw()` to guarantee execution on the tview event loop.
- On pull error, `onDone` runs with locally cached data so the view remains usable offline.
- A busy clone (`teamstate.ErrBusy`) is not an error: the background pull is skipped.
- After a successful pull, `AfterTeamSync` runs the registered hook in the background (replay of the offline workflow publications).

### Views using this pattern (14 calls, 10 views)

| View | Pull source |
|------|------------|
| ActivityView | `syncAsync` |
| PatternsView | `syncAsync` |
| PoliciesView | `syncAsync` |
| TakeoverView | `syncAsync` |
| TeamDetailView | `syncAsync` (2 calls) |
| TeamMCPView | `syncAsync` (2 calls) |
| TeamModelsView | `syncAsync` |
| TeamStatusView | `syncAsync` |
| WikiView | `syncAsync` |
| TeamBoardView | `SyncFunc func() error` via `syncFuncAsync` (3 calls) |

---

## Testing Patterns

The patterns (tables, golden files, mount/unmount, shell, keys) are in [`tui-testing.md`](tui-testing.md).
