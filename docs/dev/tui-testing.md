# Testing TUI — Developer Guide

> Patterns and conventions for testing TUI components.

## Test Levels

| Level | What | How | File |
|-------|------|-----|------|
| Interface check | View implements View | `var _ View = (*MyView)(nil)` | `views_test.go` |
| Mount/Unmount | Full lifecycle | Create content + app, call Mount, verify items | `views_test.go` |
| Pure logic | Router, command registry | Assertions on state without screen | `router_test.go`, `command_test.go` |
| Widget | Filtering, selection | `tcell.SimulationScreen` | `filterlist_test.go` |
| Shell | New, NavigateHome, omnibar | Full shell instantiation | `shell_test.go` |
| Actions | Callbacks don't panic | Call with nil dependencies | `tui_test.go` |

## Pattern — View Test

```go
func TestMyView_ImplementsView(t *testing.T) {
    var _ View = (*MyView)(nil)
}

func TestMyView_MountUnmount(t *testing.T) {
    v := NewMyView(nil) // nil app if no deps required
    content := tview.NewFlex().SetDirection(tview.FlexRow)
    app := tview.NewApplication()

    v.Mount(content, app)
    assert.Greater(t, content.GetItemCount(), 0)
    assert.Equal(t, "myview", v.ID())
    assert.NotEmpty(t, v.Title())
    assert.NotEmpty(t, v.StatusHints())

    v.Unmount()
}
```

## Pattern — Router Test (pure logic)

```go
func TestRouter_PushPop(t *testing.T) {
    content := tview.NewFlex()
    app := tview.NewApplication()
    r := router.New(content, app, nil)

    home := &mockView{id: "home"}
    board := &mockView{id: "board"}

    r.Push(home)
    r.Push(board)
    assert.Equal(t, "board", r.Current().ID())

    r.Pop()
    assert.Equal(t, "home", r.Current().ID())
}
```

## Pattern — Command Registry Test (pure logic)

```go
func TestCommandRegistry_Search(t *testing.T) {
    r := NewCommandRegistry([]Command{
        {ID: "start", Label: "Start", Aliases: []string{"session"}},
        {ID: "board", Label: "Board", Aliases: []string{"kanban"}},
    })

    results := r.Search("kan")
    assert.Greater(t, len(results), 0)
    assert.Equal(t, "board", results[0].ID)
}
```

## Pattern — Shell Lifecycle Test

```go
func TestShell_NavigateHome(t *testing.T) {
    homeView := &testView{id: "home", title: "Home"}

    cfg := Config{
        ProjectName: "test",
        Commands:    []Command{{ID: "home", Label: "Home", ViewID: "home"}},
        Views:       []views.View{homeView},
        HomeViewID:  "home",
    }

    s := New(cfg)
    s.NavigateHome("home")
    assert.True(t, homeView.mounted)
    assert.Equal(t, "home", s.router.Current().ID())
}
```

## Pattern — Omnibar Activation Test

```go
func TestOmnibar_ActivateDeactivate(t *testing.T) {
    s := New(cfg) // with some commands and views
    s.NavigateHome("home")

    assert.False(t, s.omnibar.IsActive())
    s.omnibar.Activate()
    assert.True(t, s.omnibar.IsActive())
    s.omnibar.Deactivate()
    assert.False(t, s.omnibar.IsActive())
}
```

## Pattern — Fuzzy Match (pure logic)

```go
func TestFuzzyMatch(t *testing.T) {
    tests := []struct {
        pattern, str string
        expected     bool
    }{
        {"st", "start", true},
        {"xyz", "start", false},
        {"brd", "board", true},
    }
    for _, tt := range tests {
        assert.Equal(t, tt.expected, fuzzyMatch(tt.pattern, tt.str))
    }
}
```

## Running Tests

```bash
make test-tui   # TUI tests only
make test-unit  # All tests (short mode)
make test       # All tests complete
```

## Conventions

- Every view has at minimum `ImplementsView` + `MountUnmount` tests
- Views with `*app.App` dependency receive `nil` in tests (nil-safe)
- View mocks use `testView` struct (defined in `shell_test.go`)
- `t.Helper()` for all setup helpers
- `testify/assert` for assertions, `testify/require` for fatals
- Command tests verify fuzzy search ranking and `Enabled` filtering
