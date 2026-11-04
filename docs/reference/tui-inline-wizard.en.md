> [Lire en français](tui-inline-wizard.fr.md)

# Reference — InlineWizardView

> Reusable TUI component for multi-step wizards in the OpenHub shell.

## Overview

`InlineWizardView` is a view (`views.View`) that implements a multi-step wizard
directly inside the TUI shell, without creating a separate `tview.Application`. It is
pushed onto the router stack via `shell.PushView(v)` and behaves like any other shell view.

The steps are `WizardStep` values (`wizard_step.go`), the same in the TUI shell and in
the wizards of the CLI, which run this view alone (`RunInlineWizardStandalone`).

**Source file**: `cli/internal/tui/v2/views/inline_wizard.go`

---

## API

### `InlineWizardConfig`

```go
type InlineWizardConfig struct {
    ID                     string                          // unique ID (e.g. "wizard.team.init")
    Title                  string                          // title for the shell breadcrumb
    Steps                  []WizardStep                    // wizard steps
    OnComplete             func(completed bool, err error) // completion callback
    SummaryTargetView      string                          // target view after the summary (e.g. "team.detail")
    SummaryTargetLabel     string                          // link label (e.g. "View team config")
    SummaryTargetViewFunc  func() string                   // variant computed when the summary renders (takes precedence)
    SummaryTargetLabelFunc func() string                   // same for the label
    Groups                 []StepGroup                     // non-empty: "grouped" layout (see below)
}
```

### `WizardStep`

Type shared by the TUI and the CLI wizards (`wizard_step.go`):

| Field | Role |
|-------|------|
| `ID` | Optional identifier (label updates by name) |
| `Label` | Label in the step bar and the info panel |
| `Form` | Builds a `*tview.Form`; must call `onDone()` on confirmation |
| `CustomView` | Free content in the given container (takes precedence over `Form`) |
| `Validate` | Called before `OnDone`; a non-empty text blocks with that message |
| `OnDone` | Side effects (writes, calls), run with the spinner |
| `Processing` | Spinner message while `OnDone` runs |
| `InfoFields` | Key/value pairs added to the info panel after success |
| `Skip` / `SkipIf` | Step already satisfied / condition evaluated just before rendering |
| `Required` | No « Skip the step » button in the `Esc` question (the wizard can still be quit) |
| `SidebarHidden` | Excludes the step from the sidebar in grouped mode (intro pages) |

### Grouped mode (`Groups`)

With `Groups` (`StepGroup{Label, StartIdx}`), content is centered, the info panel moves to the right and the step bar shows the groups instead of the steps. Used by the first-run wizard (`oh init`: Language, Provider, Team).

### Constructor

```go
func NewInlineWizardView(cfg InlineWizardConfig) *InlineWizardView
```

### Shell injection

`InlineWizardView` implements the implicit `shellAware` interface (`SetShell(ShellAccess)`).
Injection is automatic when the wizard is pushed via `shell.PushView(v)`.

---

## Layout

```
┌─────────────────────────────────────────────────────┐
│ ● Repo ─── ◆ Config ─── ○ Identity ─── ○ Notifs    │  StepBar (1 row, fixed)
├─────────────────────────────────────────────────────┤
│ Step 2/5 — Global configuration                    │  Step header (2 rows, fixed)
├─────────────────────────────────────────────────────┤
│                                                     │
│ [Form / CustomView / Spinner]                       │  Content (expandable, weight=1)
│                                                     │
├─────────────────────────────────────────────────────┤
│ ── Summary ──                                       │
│ ✓ Repo: git@gitlab.com:team/state.git               │  Info panel (dynamic height)
│ ✓ Config: stale_days=3                              │
├─────────────────────────────────────────────────────┤
│ ctrl+s submit · ctrl+b/alt+← back · esc quit        │  Hints bar (1 row, fixed)
└─────────────────────────────────────────────────────┘
```

Widgets used (all existing in the design system):

| Widget | Source | Usage |
|--------|--------|-------|
| `widgets.StepBar` | `stepbar.go` | Horizontal progress bar |
| `widgets.StatusBar` | `stepbar.go` | Contextual hints at the bottom |
| `widgets.Spinner` | `spinner.go` | Activity indicator during async ops |
| `tview.TextView` | tview | Step header, info panel |
| `tview.Form` | tview | Step forms |
| `tview.Flex` | tview | Main layout and swappable content area |

---

## Lifecycle

### Mount

1. Initializes step states (`StepPending`, `StepDone` for `Skip: true`)
2. Finds first non-skip step, marks it `StepActive`
3. Builds the layout (stepBar + header + content + infoPanel + hintsBar)
4. Renders the first active step

### HandleKey

- Delegates to the active step's form/customview (via `InputCapture` on the form)
- In summary mode: `Enter` → navigate to `SummaryTargetView`, `Esc` → pop

### Unmount

- Stops the spinner if running
- If wizard is not completed, calls `OnComplete(false, nil)` (abort)
- Nils out all tview references

---

## Step Navigation

| Action | Trigger | Behavior |
|--------|---------|----------|
| **Submit** | Form button / `Ctrl+S` | Validate → Spinner → OnDone → advance |
| **Back** | `Ctrl+B`, `Alt+←`, `PgUp` | Reset current → Pending, previous → Active, pop info |
| **Quit / Skip** | `Esc` | « Quit the wizard? » question in place of the hints bar: **Quit** (`OnComplete(false, nil)` then pop), **Skip the step** (non-`Required` steps), **Continue** (default focus; `Esc` too). An open drop-down list closes first. No more double `Esc` |

During a processing (the `OnDone` spinner), `Ctrl+B` and `Esc` do not act at once: they wait for the processing to end, and the hints bar shows "working… Ctrl+Q to quit". The recap also goes back with `Alt+←`.

---

## Creating a New Wizard

### Minimal Example

```go
func actionMyWizard() {
    a := MustApp()

    // Shared state across steps (captured by closures)
    var name, email string

    steps := []views.WizardStep{
        {
            Label:    "Identity",
            Required: true,
            Form: func(_ *tview.Application, onDone func()) *tview.Form {
                form := tview.NewForm()
                form.AddInputField("Name", "", 0, nil, func(t string) { name = t })
                form.AddInputField("Email", "", 0, nil, func(t string) { email = t })
                form.AddButton("Next", func() { onDone() })
                return form
            },
            Validate: func() string {
                if name == "" { return "Name required" }
                return ""
            },
            OnDone: func() error {
                return saveUser(name, email) // async operation
            },
            InfoFields: func() []views.InfoField {
                return []views.InfoField{
                    {Label: "Name", Value: name},
                    {Label: "Email", Value: email},
                }
            },
            Processing: "Saving...",
        },
        {
            Label: "Confirmation",
            CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
                tv := tview.NewTextView().SetDynamicColors(true)
                tv.SetText("All set! Press Enter.")
                tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
                    if e.Key() == tcell.KeyEnter { onDone(); return nil }
                    return e
                })
                container.AddItem(tv, 0, 1, true)
                app.SetFocus(tv)
            },
        },
    }

    wizard := views.NewInlineWizardView(views.InlineWizardConfig{
        ID:                 "wizard.my.feature",
        Title:              "My wizard",
        Steps:              steps,
        SummaryTargetView:  "settings",
        SummaryTargetLabel: "View settings",
        OnComplete: func(completed bool, err error) {
            if completed && err == nil {
                // Refresh application state
            }
        },
    })

    tuiShell.PushView(wizard)
}
```

### Key Patterns

**Shared state via closures**: variables shared across steps are declared before the
steps and captured by the `Form`, `OnDone`, `Validate`, `InfoFields` closures. Same
pattern as the CLI wizard in `team.go`.

**Dynamic pre-fill**: a step's `OnDone` can modify default values for subsequent steps.
The `Form` callback is called at render time (not at init), so it sees updated values.

**Conditional skip**: `SkipIf` is evaluated dynamically just before step rendering.
A step can become skippable based on choices made in previous steps.

**Processing-only**: a step with neither `Form` nor `CustomView` directly shows the
spinner + runs `OnDone`. Useful for pure processing steps (extraction, bundle build).

### Omnibar Integration

```go
// In tui_commands.go
commands = append(commands, shell.Command{
    ID:          "my.wizard",
    Label:       "My wizard",
    Aliases:     []string{"wizard"},
    Description: "Launch the configuration wizard",
    Category:    "Configuration",
    Action:      actionMyWizard,
})
```

---

## Step Types

### Form (most common)

The `Form` callback receives `*tview.Application` and `onDone func()`. It builds a
standard `*tview.Form`. The wizard automatically applies theming, wires `Ctrl+S`/`Ctrl+B`/`Esc`,
and manages focus.

```go
Form: func(_ *tview.Application, onDone func()) *tview.Form {
    form := tview.NewForm()
    form.AddInputField("Field", defaultValue, 0, nil, func(t string) { val = t })
    form.AddButton("Next", func() { onDone() })
    return form
}
```

### CustomView (arbitrary content)

For non-form UIs (welcome screen, Yes/No selection, preview). The callback receives
`*tview.Application`, the `*tview.Flex` container, and `onDone`. It must add widgets
to the container and manage focus.

```go
CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
    tv := tview.NewTextView().SetDynamicColors(true)
    tv.SetText("Custom content")
    tv.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
        if e.Key() == tcell.KeyEnter { onDone(); return nil }
        return e
    })
    container.AddItem(tv, 0, 1, true)
    app.SetFocus(tv)
}
```

### Processing-only (no UI)

Neither `Form` nor `CustomView`. The wizard shows the spinner with the `Processing`
message and runs `OnDone` in a goroutine. Useful for long operations without user input.

```go
{
    Label:      "Extraction",
    Processing: "Extracting hub content...",
    OnDone:     func() error { return hubcontent.Extract(hubcontent.HubContentDir()) },
    InfoFields: func() []views.InfoField {
        return []views.InfoField{{Label: "Hub", Value: "extracted"}}
    },
}
```

---

## Summary Screen

When all steps complete (or are skipped), the wizard automatically shows a summary screen
with all accumulated `InfoFields`, offering two navigation choices:

- `[Enter]` → `shell.NavigateTo(SummaryTargetView)` (if configured)
- `[Esc]` → pop the wizard (return to previous view)

The `OnComplete(true, nil)` callback fires at this point. It executes before the user
picks navigation, allowing state persistence (hub.toml writes, config reload, etc.) while
the summary is displayed.

---

## In the shell and alone (`RunInlineWizardStandalone`)

The wizards of the CLI (`oh init`, `oh provider setup`, `oh project add|configure|remove`, `oh policies add`, `oh patterns add`, `oh mcp setup`, `oh team init`) mount the same `InlineWizardView` in an application of their own (`views.RunInlineWizardStandalone`); the old wizard engine (`RunWizard`) is removed.

| Aspect | In the shell | Alone (CLI) |
|--------|--------------|-------------|
| `tview.Application` | The shell's | Its own, alt-screen for the length of the wizard |
| Omnibar | Visible and accessible | None |
| Toasts (`ShowToastMsg`) | Shell toasts | A notification in the hints bar, for a few seconds (`Notify`) |
| Quit | `Esc` → « Quit the wizard? » (the wizard is popped) | Same question; `Ctrl+C` quits at once |
| Result | `OnComplete(completed, err)` | `StandaloneResult`: `Completed`, `Aborted` (the user quit), `Err` (a step failed and the user quit) |
| What a step prints | Not applicable | Held until the wizard has left the screen (`oh` commands) |
| Summary screen | Proposes a navigation | Not shown: the command prints its own summary |
