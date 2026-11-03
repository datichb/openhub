# Testing TUI — Developer Guide

> Patterns and conventions for testing TUI components. Architecture: [`tui-architecture.md`](tui-architecture.md); keys and components: [`tui-conventions.md`](tui-conventions.md).

## Principles

- **One test per scenario**, several assertions, each with a message (`assert.Equal(t, want, got, "what is checked")`). `testify/assert` for checks that let the test go on, `testify/require` for the ones it cannot go on without.
- **Tables** (`t.Run(tc.name, …)`) when only the data changes: a family of near-identical tests is one table.
- **Golden file for a screen**, explicit assertions for a behaviour (keys, callbacks, state). A test that stacks `assert.Contains` on a rendering is a golden in waiting.
- **Nothing that the compiler already checks**: `var _ View = (*MyView)(nil)` goes in the production file, not in a test. No test reads back what it has just assigned (struct fields, getters, constants), none only checks that a call does not panic.
- **Hermetic**: no real clipboard, terminal, keychain or hub (`testenv.Isolate()` in `TestMain`); an external command is replaced by a script in a temporary `PATH` (see `shell/clipboard_test.go`).

## Test Levels

| Level | What | How | Where |
|-------|------|-----|-------|
| Mount/Unmount | Lifecycle of every view | One table: content mounted, ID, title, `Unmount` releases the app | `views_test.go` (`TestViews_MountUnmount`) |
| Rendering | What the screen shows | `screenText` + `golden.Assert` | `views/*_test.go`, `testdata/` |
| Behaviour | Keys, callbacks, confirmations, state | `HandleKey`, `onLoop`, `harness`, fake shells | `views/*_test.go` |
| Pure logic | Router, command registry, fuzzy match, store, file | Assertions on state, tables | `router_test.go`, `command_test.go`, `notifications_test.go` |
| Widgets | Selection, filtering, graphs | `tcell.SimulationScreen`, goldens | `widgets/*_test.go` |
| Shell | New, NavigateHome, omnibar, overlays | Full shell instantiation | `shell_test.go` |
| Keys | The hints follow the convention; no key is read from a letter | `TestKeyConvention` (strict), `TestTUIKeysGoThroughKeys` | `keys/convention_test.go`, `archtest/tui_keys_test.go` |
| Style | No colour, tag or symbol written by hand | `TestUIStyleGuard` | `archtest/ui_style_test.go` |
| Asynchronous | A read or a write off the loop, applied on it | `runApp`/`runLoop`, `onLoop`, `require.Eventually` | `views/async_test.go` |

## Golden files

`internal/testutil/golden` compares an output with `testdata/<name>.golden` (the name may hold folders):

```go
golden.Assert(t, "checkpoint_card/verify_failed", cardBodyText(t, card))
```

- `go test ./internal/tui/v2/views -run '^TestCheckpointCardVerify$' -update` writes the file; review it like code (read it, then break one line of it and check that the test fails).
- A golden file that exists is never regenerated to make a test pass: if the rendering changed on purpose, the diff of the golden is the review.
- The `-update` flag is declared by `golden`: a test package must not declare its own `flag.Bool("update", …)` (the test binary would panic at start).
- Render with `screenText(t, primitive, w, h)` (`views/mode_hints_test.go`): the text as the screen shows it (tags resolved, escapes undone). `compactCardText` (`sessions_checkpoint_failed_test.go`) trims the empty lines for the cards.
- Set the language first (`frLocale(t)`): a golden is written for one language.
- A golden holds text, so it is written without colours when the colours are not what is checked: `t.Setenv("NO_COLOR", "1")` (`theme.Paint` then returns the text alone), as `TestSymbolLegend` does.
- A golden that changes on purpose is listed and justified in the commit that changes it (what changed on screen, and why); a lot that touches a screen says which goldens moved. A golden file can also be a plain text dump of a structure (`cmd/testdata/tui_commands_*.golden`, `tui_views_*.golden`: the commands and views that the TUI registers).

## Height and width of the terminal

The shell has three layout levels (`views.LevelFor`): **full** from 32 rows, **compact** from 16 to 31 rows, and **too small** under 60×16, where only the message and `Ctrl+Q` work. A test that renders a landing or a view decides its size on purpose:

- the shell tests run a 120×40 screen (`newKeyShell`, `runSimShell(t, cols, rows, cfg)`): full level. A test of the compact level asks for fewer rows (`layout_test.go`) and says so in its name;
- a view rendered alone is given `screenText(t, p, w, h)`: a golden written at 80×18 does not show what a 120×40 screen shows. Give enough rows for what is checked, no more, and the same size in every test of the same family;
- a text that scrolls (help overlay, scrollable modal, long list) is not all on the first screen: page with `PgDn`/`↓` until the text is seen, or render with the height that holds it (`TestHelpShowsTheSymbolLegend` pages; `TestHelpViewShowsTheSymbolLegend` gives 120 rows);
- a `SectionedList` or a table keeps the cursor and its section title on screen: a test of scrolling uses a height smaller than the list (`sectioned_list_draw_test.go`).

## Pattern — View mount (table row)

```go
{
    name: "my view", view: NewMyView(MyViewConfig{}), id: "my-view",
    released: func(v View) bool { return v.(*MyView).app == nil },
},
```

A view that needs the active project or team gets `shell: &mockShell{project: …}`. A view with an event loop (async refresh, `QueueUpdateDraw`) is run with `runApp(t, content)` and driven with `onLoop(app, func() { … })`.

## Pattern — Event loop: `runApp`, `runLoop`, `onLoop`

A view with an event loop (asynchronous read, `QueueUpdateDraw`) is driven on a real loop over a simulation screen:

```go
content := tview.NewFlex()
app := runApp(t, content)                 // a tview.Application on a 120×40 simulation screen, running
onLoop(app, func() { v.Mount(content, app) })
onLoop(app, func() { assert.Nil(t, v.HandleKey(runeKey('d'))) })
```

- `runApp` builds the application **with the simulation screen set** and starts it with `runLoop`. `runLoop` runs `app.Run()` and returns once the loop handles updates (`QueueUpdate(func() {})`); at the end of the test it stops the application and waits for `Run` to return. Without that wait a test that ends at once stops the application before `Run` starts: tview then makes a real screen (`tcell.NewScreen`: terminfo, global state, a data race under `-race`). A test that makes its own `tview.NewApplication()` and runs it uses `runLoop`, not a `go app.Run()` of its own.
- `onLoop(app, fn)` runs `fn` on the loop and returns when it is done: everything that touches a widget (calling `HandleKey`, reading `List.GetCurrentItem`, mounting) goes through it. A field of the view that the loop writes is read the same way, or the test has a data race.
- Never call `QueueUpdate` or `QueueUpdateDraw` from inside a callback already on the loop (it blocks the loop for good).

## Pattern — Asynchronous tests

A read off the loop (`load`, `syncLoad`) or a write (`persist`) ends later than the call that started it. The test waits for the result, not for a duration:

```go
require.Eventually(t, func() bool {
	n := 0
	onLoop(app, func() { n = len(v.items) })
	return n == 2
}, 2*time.Second, 10*time.Millisecond, "the list is loaded")
```

- Wait on the state that the user would see (the items, the text, a fake that recorded a call) with `require.Eventually`; never a bare `time.Sleep`, except to check that something does **not** happen (a short sleep, then `drain(app)`, `async_test.go`).
- A fake that the goroutine writes and the test reads has a mutex (`fakeWorktrees.state()`), or a channel the test reads with a timeout.
- To control the order, the fake `fetch` waits on a channel the test closes (`release`): a stale result (`slot.Invalidate()`, a newer load) is dropped, the latest one is applied — `TestLoad_StaleResultAfterRemountIsIgnored` is the model.
- `load` with a nil `app` runs `fetch` and `apply` inline: a test of the logic that does not need a loop passes `nil`.
- A panic in `fetch` or `work` is recovered and given to `apply` / `done` as an error: the test of a view checks the error state (`stateText`), not that the process lives.
- A destructive key is still tested in two steps (below), and the goroutine is awaited before the second.

## Pattern — Behaviour with a fake shell

```go
sh := &viewShell{}
v.SetShell(sh)
onLoop(app, func() { v.HandleKey(runeKey('d')) })
require.Equal(t, 1, sh.confirmCount(), "d asks first")
assert.Equal(t, "opus", cfg.Models.Families["planning"], "nothing deleted before the confirmation")
sh.confirmed(t, app)
```

A destructive key is always checked in two steps: nothing happens before the confirmation, the effect after it.

## Pattern — Table

```go
tests := []struct {
    name  string
    in    string
    limit int
    want  string
}{
    {"fits", "abc", 3, "abc"},
    {"cut with an ellipsis", "abcdef", 4, "abc…"},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        assert.Equal(t, tc.want, clipText(tc.in, tc.limit), "clipText")
    })
}
```

## Pattern — Deep copy by reflection

`config_deepcopy_test.go` fills every field of a value, copies it, changes every field of the copy and checks that the original did not move: a new field is covered without touching the test. The fields that the copy functions still share are listed in `configSharedFields` / `teamConfigSharedFields` (to empty when the copy functions are fixed).

## Pattern — Keys convention

`TestKeyConvention` reads the hints lines of the TUI and fails on any key outside the convention. There is no list of tolerated deviations: fix the hint (build it with `keys.Hint`, it is conform by construction).

`TestTUIKeysGoThroughKeys` fails on a `switch ev.Rune() { case 'j': … }` or an `ev.Rune() == ' '` in `internal/tui/**` (outside `keys/`) and `cmd/tui_*.go`: a key is asked of `keys` (`keys.Down.Matches(ev)`) or declared with `keys.Own`. The list of exceptions (`archtest/testdata/tui_keys_exceptions.txt`) is empty. A widget whose keys are rewritten is **pinned first** (`widgets/keys_behavior_test.go`: each letter, its arrow, the limits, an unknown letter), so that the move onto `keys` shows what it changes.

## Pattern — Style guard

`TestUIStyleGuard` fails on a tview tag written by hand, a colour named in the code or a symbol of the table of meanings in a literal, in `internal/tui/**` and `cmd/tui_*.go`. The fix is in the code (`theme.Paint`, `theme.StyleTag`, `theme.Sym*`); the list `archtest/testdata/ui_style_exceptions.txt` is empty and nothing is added to it. The detection has its own test on a small source (`TestUIStyleDetection`): a new rule gets a line there.

## Running Tests

```bash
make test-tui                       # TUI tests only
go test ./internal/tui/... -short   # fast
go test ./internal/tui/... -race    # data races (event loop, goroutines)
go test ./internal/archtest/...     # style guard, keys guard, layers
go test -race -run 'TestTUI|Column|Keys' ./cmd/   # the TUI tests of cmd/, without -short
go test ./internal/i18n/... ./cmd/ -run 'Literal|Translated|Orphan|Help|Key'  # i18n guards
```

## Conventions

- `t.Helper()` for all setup helpers; `t.Cleanup` to stop an app or restore the language.
- Views with an `*app.App` dependency receive `nil` in tests (nil-safe).
- Shell mocks: `mockShell` (`views_test.go`) for the views, `testView` (`shell_test.go`) for the shell.
- Displayed texts are checked through `i18n.T` / `i18n.Tf` or a golden, never a hard-coded copy of a translation that another language would break.
- Home views: `splitItems` is checked on the `SectionID` structure (immune to the labels), the `mountGen` guard by `Mount → Unmount → Mount`.
