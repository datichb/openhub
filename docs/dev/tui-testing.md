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
| Keys | The hints follow the convention | `TestKeyConvention` (strict) | `keys/convention_test.go` |

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

## Pattern — View mount (table row)

```go
{
    name: "my view", view: NewMyView(MyViewConfig{}), id: "my-view",
    released: func(v View) bool { return v.(*MyView).app == nil },
},
```

A view that needs the active project or team gets `shell: &mockShell{project: …}`. A view with an event loop (async refresh, `QueueUpdateDraw`) is run with `runApp(t, content)` and driven with `onLoop(app, func() { … })`.

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

## Running Tests

```bash
make test-tui                       # TUI tests only
go test ./internal/tui/... -short   # fast
go test ./internal/tui/... -race    # data races (event loop, goroutines)
go test ./internal/i18n/... ./cmd/ -run 'Literal|Translated|Orphan|Help|Key'  # i18n guards
```

## Conventions

- `t.Helper()` for all setup helpers; `t.Cleanup` to stop an app or restore the language.
- Views with an `*app.App` dependency receive `nil` in tests (nil-safe).
- Shell mocks: `mockShell` (`views_test.go`) for the views, `testView` (`shell_test.go`) for the shell.
- Displayed texts are checked through `i18n.T` / `i18n.Tf` or a golden, never a hard-coded copy of a translation that another language would break.
- Home views: `splitItems` is checked on the `SectionID` structure (immune to the labels), the `mountGen` guard by `Mount → Unmount → Mount`.
