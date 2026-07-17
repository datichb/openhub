# TUI Architecture — Guide développeur

> Architecture interne du shell TUI unifié, interfaces, et guide pour ajouter une nouvelle vue.

## Vue d'ensemble

```
cmd/root.go
  └─ oh (sans args) → shell.New(cfg).Run()

cli/internal/tui/
├── theme/              ← Source unique des couleurs et styles
│   ├── colors.go      ← Hex constants
│   ├── tcell.go       ← tcell.Color vars
│   ├── lipgloss.go    ← lipgloss.Color + Style vars
│   ├── huh.go         ← AurumTheme() pour charmbracelet/huh
│   └── icons.go       ← Constantes d'icônes
├── v2/
│   ├── shell/         ← Application principale
│   │   ├── shell.go   ← Shell struct (Pages + Header + Menu + Content + StatusBar)
│   │   ├── header.go  ← Header 3-sections avec breadcrumb
│   │   ├── statusbar.go ← StatusBar 3-sections
│   │   ├── toast.go   ← Notifications temporaires
│   │   └── modal.go   ← Modals de confirmation
│   ├── router/        ← Navigation
│   │   └── router.go  ← Stack push/pop/replace
│   ├── menu/          ← Menu sidebar
│   │   ├── menu.go    ← TreeView wrapper
│   │   └── items.go   ← Arbre de menu
│   ├── views/         ← Toutes les vues
│   │   ├── view.go    ← Interface View
│   │   ├── home.go    ← Dashboard/Home
│   │   ├── board.go   ← Kanban
│   │   └── ...
│   └── widgets/       ← Primitives réutilisables
└── components/        ← BubbleTea inline (floating, summary)
```

## Interface View

```go
// Package views defines the View interface contract.
package views

import (
    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
)

// View defines the contract for a navigable view in the TUI shell.
type View interface {
    // ID returns the unique identifier (e.g., "home", "board", "projects.list").
    ID() string

    // Title returns the localized display title for breadcrumb and menu.
    Title() string

    // Mount populates the content panel with this view's widgets.
    // Called when the view becomes active via the router.
    Mount(content *tview.Flex, app *tview.Application)

    // Unmount cleans up resources (stop timers, close channels).
    // Called when navigating away from this view.
    Unmount()

    // StatusHints returns contextual keybinding hints for the status bar center.
    StatusHints() string

    // HandleKey processes view-specific key events.
    // Return nil to consume the event, or return it to propagate to the shell.
    HandleKey(event *tcell.EventKey) *tcell.EventKey
}
```

### Compile-time check

Chaque vue implémentant l'interface doit inclure :

```go
var _ views.View = (*HomeView)(nil)
```

## Router — Cycle de vie

```
Push(viewB):
  1. router.stack = append(stack, viewB)
  2. viewA.Unmount()          ← nettoyage de l'ancienne vue
  3. shell.content.Clear()    ← vide le panel
  4. viewB.Mount(content, app) ← peuple le nouveau contenu
  5. shell.header.SetBreadcrumb(viewB.Title())
  6. shell.statusBar.SetHints(viewB.StatusHints())
  7. shell.menu.SetActive(viewB.ID())

Pop():
  1. viewB.Unmount()
  2. router.stack = stack[:len-1]
  3. viewA = stack[last]
  4. shell.content.Clear()
  5. viewA.Mount(content, app)
  6. Update header/status/menu
```

## Shell — Structure interne

```go
type Shell struct {
    app       *tview.Application
    pages     *tview.Pages       // "main" page + overlay pages
    header    *Header
    menu      *menu.Menu
    content   *tview.Flex        // zone swappable par le router
    statusBar *StatusBar
    router    *Router
}
```

Le root de l'application est `pages` :
- Page "main" (resize=true, visible=true) : layout Flex (header + middle + statusbar)
- Pages overlay ajoutées dynamiquement pour toasts et modals

## Menu — Arbre hiérarchique

Structure de données :

```go
type MenuItem struct {
    ID       string         // "sessions.start"
    Label    string         // Localisé via i18n
    ViewID   string         // Vue à ouvrir (ou "")
    Action   func()         // Callback pour actions non-vue
    Children []*MenuItem    // nil = feuille
    Expanded bool
    Enabled  func() bool    // Condition d'activation dynamique
}
```

Le menu utilise `tview.TreeView` avec :
- `SetGraphics(false)` — pas de lignes d'arbre
- `SetTopLevel(1)` — masque le noeud root
- Prefixes manuels via node text formatting

## Overlays (Toast + Modal)

### Architecture Pages

```
pages.AddPage("main", mainLayout, true, true)     ← fond
pages.AddPage("toast-1", toastGrid, true, true)   ← overlay temporaire
pages.AddPage("modal-confirm", modalGrid, true, true) ← modal
```

### Toast : non-focus

Le toast est ajouté sans `SetFocus` → les inputs continuent vers "main".

### Modal : capture focus

Le modal capture le focus via `app.SetFocus(modal)`. Esc/Enter dismiss le modal et restaure le focus sur le content.

## Comment ajouter une nouvelle vue

### 1. Créer le fichier

```
cli/internal/tui/v2/views/myview.go
```

### 2. Implémenter l'interface

```go
package views

import (
    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"

    "github.com/datichb/openhub/cli/internal/tui/theme"
)

// MyView displays the ... .
type MyView struct {
    app     *tview.Application
    // widgets internes
}

var _ View = (*MyView)(nil)

func NewMyView() *MyView { return &MyView{} }

func (v *MyView) ID() string    { return "myview" }
func (v *MyView) Title() string { return i18n.T("tui.myview.title") }

func (v *MyView) Mount(content *tview.Flex, app *tview.Application) {
    v.app = app
    // Construire widgets, les ajouter au content
    content.AddItem(myWidget, 0, 1, true)
}

func (v *MyView) Unmount() {
    // Arrêter les timers, fermer les channels
}

func (v *MyView) StatusHints() string {
    return "enter select · esc back"
}

func (v *MyView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
    // Gérer les touches spécifiques à cette vue
    return event // propager si non consommé
}
```

### 3. Enregistrer dans le menu

Dans `menu/items.go`, ajouter un `MenuItem` :

```go
{ID: "myview", Label: i18n.T("menu.myview"), ViewID: "myview"},
```

### 4. Enregistrer dans le registry

Dans `shell.go` ou un fichier de registre :

```go
shell.RegisterView(views.NewMyView())
```

### 5. Ajouter un test

```go
// views/myview_test.go
func TestMyView_ImplementsView(t *testing.T) {
    var _ views.View = (*MyView)(nil) // compile-time
}

func TestMyView_MountUnmount(t *testing.T) {
    v := NewMyView()
    content := tview.NewFlex()
    app := tview.NewApplication()
    
    v.Mount(content, app)
    assert.Greater(t, content.GetItemCount(), 0)
    
    v.Unmount()
    // Vérifier cleanup (pas de goroutine leak, etc.)
}
```

## Tests TUI — Patterns

### Logique pure (sans écran)

```go
func TestRouter_PushPop(t *testing.T) {
    r := router.New(mockShell)
    r.Push(mockViewA)
    r.Push(mockViewB)
    assert.Equal(t, "b", r.Current().ID())
    r.Pop()
    assert.Equal(t, "a", r.Current().ID())
}
```

### Avec SimulationScreen (widgets)

```go
func TestMenu_Navigate(t *testing.T) {
    screen := tcell.NewSimulationScreen("")
    screen.Init()
    screen.SetSize(80, 24)
    
    app := tview.NewApplication().SetScreen(screen)
    m := menu.New(testItems(), onSelect)
    app.SetRoot(m.Primitive(), true)
    
    go app.Run()
    defer app.Stop()
    
    screen.InjectKey(tcell.KeyRune, 'j', 0)
    time.Sleep(50 * time.Millisecond)
    
    assert.Equal(t, "sessions", m.SelectedID())
}
```

## Conventions de nommage

| Élément | Convention | Exemple |
|---------|-----------|---------|
| View struct | `*View` suffix | `HomeView`, `BoardView` |
| View ID | dot-separated, lowercase | `"projects.list"` |
| Menu item ID | dot-separated | `"sessions.start"` |
| Package | single word, lowercase | `shell`, `router`, `menu` |
| Test helper | `setup*` / `mock*` prefix | `setupTestShell(t)` |
