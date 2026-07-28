# TUI Architecture — Developer Guide

> Internal architecture of the omnibar-first TUI shell, interfaces, and guide for adding new commands/views.

## Overview

```
cmd/tui.go
  └─ oh (no args) → shell.New(cfg).Run()

cli/internal/tui/
├── theme/              ← Single source of truth for colors and styles
│   ├── colors.go      ← Hex constants (Catppuccin Mocha)
│   ├── tcell.go       ← tcell.Color vars
│   ├── lipgloss.go    ← lipgloss.Color + Style vars
│   ├── huh.go         ← AurumTheme() for charmbracelet/huh
│   └── icons.go       ← Icon constants
├── v2/
│   ├── shell/         ← Main application container
│   │   ├── shell.go   ← Shell struct (Pages + Content + Omnibar)
│   │   ├── command.go ← Command struct + CommandRegistry + fuzzy search
│   │   ├── omnibar.go ← Persistent omnibar widget
│   │   └── toast.go   ← Temporary notifications
│   ├── router/        ← Navigation
│   │   └── router.go  ← Stack push/pop/replace
│   ├── views/         ← All navigable views
│   │   ├── view.go    ← View interface
│   │   ├── home.go    ← Splash screen
│   │   ├── board.go   ← Kanban
│   │   └── ...
│   └── widgets/       ← Reusable tview primitives
└── components/        ← BubbleTea inline (floating, summary)
```

## Shell Architecture

```go
type Shell struct {
    app       *tview.Application
    pages     *tview.Pages       // "main" page + overlay pages (toasts, suggestions)
    content   *tview.Flex        // swappable content zone (managed by router)
    omnibar   *Omnibar           // persistent input at bottom
    router    *router.Router     // view navigation stack
    registry  *CommandRegistry   // flat command list for omnibar
}
```

Layout:
```
Pages
└── "main" page
    └── Flex (column)
        ├── content (proportion 1) ← full screen
        └── omnibar (fixed 1 row)  ← always visible
```

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
}
```

Commands are defined in `cmd/tui.go:buildCommands()`. Adding a new command requires only appending to this list.

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
- `StatusHints()`: text displayed in omnibar passive mode.

## Key Event Flow

```
tview.Application.InputCapture (globalKeyHandler)
    │
    ├── Ctrl+Q → app.Stop()
    ├── Ctrl+P → omnibar.Activate()
    ├── Esc    → router.Pop() or noop
    │
    ├── [omnibar active?] → omnibar handles all input
    ├── [inline overlay?] → overlay handles input
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

## Inline Prompts (replacing modals)

Views that need user input call `ShellAccess` methods:

```go
type ShellAccess interface {
    ShowInputModal(title, currentValue string, onConfirm func(newValue string))
    ShowPasswordModal(title string, onConfirm func(value string))
    ShowSelectModal(title string, options []SelectOption, currentValue string, onConfirm func(value string))
    ShowMultiSelectModal(title string, options []SelectOption, selected []string, onConfirm func(selected []string))
    ShowScrollableModal(title, content string, actions []ModalAction)
    ShowToastMsg(msg string, success bool)
}
```

These now render as **inline overlays** in the `Pages` system rather than traditional popup modals.

## How to Add a New Command

### 1. Add to `cmd/tui.go:buildCommands()`

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

Same as before — implement the `View` interface and register in `buildViews()`.

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
func (v *MyView) StatusHints() string  { return "j/k nav · Enter action" }
func (v *MyView) Mount(content *tview.Flex, app *tview.Application) { ... }
func (v *MyView) Unmount() { ... }
func (v *MyView) HandleKey(event *tcell.EventKey) *tcell.EventKey { ... }
```

### 3. Register in `cmd/tui.go:buildViews()`

```go
allViews = append(allViews, views.NewMyView())
```

### 4. Add a command to reach it

```go
{ID: "myview", Label: "My View", ViewID: "my-view", Category: "..."},
```

### 5. Add a test

```go
func TestMyView_ImplementsView(t *testing.T) {
    var _ views.View = (*MyView)(nil)
}
```

## Async Pull Pattern (Team Views)

All 6 team views use a shared async pull pattern defined in `cli/internal/tui/v2/views/sync.go`.

### Helpers

```go
// syncAsync pulls git in background via a *teamstate.Repo.
// Shows a "Synchronisation..." toast if the pull takes > 1s.
// Calls onDone on the tview event loop after completion.
// On error, onDone is called with local (cached) data instead of failing.
func syncAsync(app *tview.Application, repo *teamstate.Repo, shell ShellAccess, onDone func())

// syncFuncAsync is identical but accepts a bare func() error instead of a *teamstate.Repo.
// Used by TeamBoardView, which receives its pull function via SyncFunc in its config struct.
func syncFuncAsync(app *tview.Application, pullFn func() error, shell ShellAccess, onDone func())
```

### Usage contract

- Called on `Mount()` to eagerly refresh data when the view is entered.
- Bound to the `r` key in each view's `HandleKey()` for manual refresh.
- The toast ("Synchronisation...") is displayed only when the pull exceeds 1 second, avoiding flicker on fast pulls.
- `onDone` is always invoked via `app.QueueUpdateDraw()` to guarantee execution on the tview event loop.
- On pull error, `onDone` runs with locally cached data so the view remains usable offline.

### Views using this pattern

| View | Pull source |
|------|------------|
| TeamStatusView | `*teamstate.Repo` via `syncAsync` |
| TeamMembersView | `*teamstate.Repo` via `syncAsync` |
| TeamClaimsView | `*teamstate.Repo` via `syncAsync` |
| TeamWikiView | `*teamstate.Repo` via `syncAsync` |
| TeamEventsView | `*teamstate.Repo` via `syncAsync` |
| TeamBoardView | `SyncFunc func() error` via `syncFuncAsync` |

---

## Testing Patterns

### Command registry (pure logic)

```go
func TestCommandRegistry_Search(t *testing.T) {
    r := NewCommandRegistry(commands)
    results := r.Search("audit")
    assert.Greater(t, len(results), 0)
}
```

### Shell lifecycle

```go
func TestShell_NavigateHome(t *testing.T) {
    s := New(cfg)
    s.NavigateHome("home")
    assert.Equal(t, "home", s.router.Current().ID())
}
```

### View Mount/Unmount

```go
func TestMyView_MountUnmount(t *testing.T) {
    v := NewMyView()
    content := tview.NewFlex()
    app := tview.NewApplication()
    v.Mount(content, app)
    assert.Greater(t, content.GetItemCount(), 0)
    v.Unmount()
}
```
