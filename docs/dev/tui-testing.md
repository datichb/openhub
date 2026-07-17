# Testing TUI — Guide développeur

> Patterns et conventions pour tester les composants TUI.

## Niveaux de test

| Niveau | Quoi | Comment | Fichier |
|--------|------|---------|---------|
| Interface check | Vue implémente View | `var _ View = (*MyView)(nil)` | `views_test.go` |
| Mount/Unmount | Lifecycle complet | Créer content + app, appeler Mount, vérifier items | `views_test.go` |
| Logique pure | Router, menu model | Assertions sur state sans écran | `router_test.go`, `menu_test.go` |
| Widget | Navigation, filtrage | `tcell.SimulationScreen` | `filterlist_test.go` |
| Actions | Callbacks ne paniquent pas | Appel avec nil dependencies | `tui_test.go` |

## Pattern standard — Test d'une vue

```go
func TestMyView_ImplementsView(t *testing.T) {
    var _ View = (*MyView)(nil)
}

func TestMyView_MountUnmount(t *testing.T) {
    v := NewMyView(nil) // nil app si pas de deps requises
    content := tview.NewFlex().SetDirection(tview.FlexRow)
    app := tview.NewApplication()

    v.Mount(content, app)
    assert.Greater(t, content.GetItemCount(), 0)
    assert.Equal(t, "myview", v.ID())
    assert.NotEmpty(t, v.Title())
    assert.NotEmpty(t, v.StatusHints())

    v.Unmount()
    assert.Nil(t, v.app)
}
```

## Pattern — Test du Router (logique pure)

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

## Pattern — Test avec SimulationScreen (widgets)

```go
func TestMenu_NavigateDown(t *testing.T) {
    screen := tcell.NewSimulationScreen("")
    screen.Init()
    screen.SetSize(80, 24)

    app := tview.NewApplication().SetScreen(screen)
    m := menu.New(testItems(), func(item *menu.MenuItem) {})
    app.SetRoot(m.Primitive(), true)

    go app.Run()
    defer app.Stop()

    // Simulate key press
    screen.InjectKey(tcell.KeyRune, 'j', 0)
    time.Sleep(50 * time.Millisecond)

    assert.Equal(t, "sessions", m.SelectedID())
}
```

## Pattern — Test fuzzy match (logique pure)

```go
func TestFuzzyMatch(t *testing.T) {
    tests := []struct {
        pattern, str string
        expected     bool
    }{
        {"st", "start", true},
        {"xyz", "start", false},
    }
    for _, tt := range tests {
        assert.Equal(t, tt.expected, fuzzyMatch(tt.pattern, tt.str))
    }
}
```

## Lancer les tests

```bash
make test-tui   # Tests TUI uniquement
make test-unit  # Tous les tests (mode -short)
make test       # Tous les tests complets
```

## Conventions

- Chaque vue a au minimum un test `ImplementsView` + `MountUnmount`
- Les vues avec `*app.App` dependency reçoivent `nil` dans les tests (nil-safe)
- Les mocks de View utilisent `mockView` struct (défini dans `router_test.go`)
- `t.Helper()` pour tous les setup helpers
- `testify/assert` pour les assertions, `testify/require` pour les fatales
