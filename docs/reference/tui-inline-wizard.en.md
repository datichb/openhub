> [Lire en français](tui-inline-wizard.fr.md)

# Reference — InlineWizardView

> Reusable TUI component for multi-step wizards in the OpenHub shell.

## Overview

`InlineWizardView` is a view (`views.View`) that implements a multi-step wizard
directly inside the TUI shell, without creating a separate `tview.Application`. It is
pushed onto the router stack via `shell.PushView(v)` and behaves like any other shell view.

The component reuses the `WizardStep` type (`wizard.go`) for compatibility of step
definitions between the CLI standalone and TUI inline paths.

**Source file**: `cli/internal/tui/v2/views/inline_wizard.go`

---

## API

### `InlineWizardConfig`

```go
type InlineWizardConfig struct {
    ID                 string                           // unique identifier (e.g. "wizard.team.init")
    Title              string                           // title for the shell breadcrumb
    Steps              []WizardStep                     // wizard steps
    OnComplete         func(completed bool, err error)  // completion callback
    SummaryTargetView  string                           // target view after summary (e.g. "team.detail")
    SummaryTargetLabel string                           // link label (e.g. "View team config")
}
```

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
│ ◆ 2/5 — Global configuration                       │  Step header (2 rows, fixed)
├─────────────────────────────────────────────────────┤
│                                                     │
│ [Form / CustomView / Spinner]                       │  Content (expandable, weight=1)
│                                                     │
├─────────────────────────────────────────────────────┤
│ ── Summary ──                                       │
│ ✓ Repo: git@gitlab.com:team/state.git               │  Info panel (dynamic height)
│ ✓ Config: stale_days=3                              │
├─────────────────────────────────────────────────────┤
│ ctrl+s submit · ctrl+b back · esc skip              │  Hints bar (1 row, fixed)
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
| **Back** | `Ctrl+B` | Reset current → Pending, previous → Active, pop info |
| **Skip** | Double `Esc` | 1st Esc = hint, 2nd = skip (blocked if `Required`) |
| **Quit** | `Esc` at shell | Pop the wizard (return to previous view) |

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
pattern as `RunWizard` in `team.go`.

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

### CustomView (arbitrary content)

For non-form UIs (welcome screen, Yes/No selection, preview). The callback receives
`*tview.Application`, the `*tview.Flex` container, and `onDone`. It must add widgets
to the container and manage focus.

### Processing-only (no UI)

Neither `Form` nor `CustomView`. The wizard shows the spinner with the `Processing`
message and runs `OnDone` in a goroutine. Useful for long operations without user input.

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

## Differences with `RunWizard` (standalone)

| Aspect | `RunWizard` (standalone) | `InlineWizardView` (inline) |
|--------|--------------------------|----------------------------|
| `tview.Application` | Creates its own | Uses the shell's |
| Omnibar | Not available | Visible and accessible |
| Toasts | Not available | Functional |
| Navigation | `Ctrl+C` quits | `Esc` pops to previous view |
| Info panel | Dedicated side panel | Integrated in vertical layout |
| Lifecycle | Blocking (`Run()` → `WizardResult`) | Async (`Mount/Unmount`, `OnComplete` callback) |
| Usage | CLI one-shot, `--no-tui` | TUI shell interactive |
| Prerequisite | None | Active TUI shell |

**When to use which:**
- `RunWizard`: CLI one-shot commands (`oh init`, `oh team init`, `oh project add`)
- `InlineWizardView`: TUI actions (omnibar, first-run, view integration)
