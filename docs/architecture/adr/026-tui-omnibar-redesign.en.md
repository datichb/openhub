# ADR-026: TUI Omnibar-First Redesign

- **Status:** Accepted
- **Date:** 2026-07-20
- **Deciders:** benjamin.datiche

## Context and Problem Statement

The TUI had grown to include too many interaction mechanisms:
- Hierarchical sidebar menu (TreeView with vim-style nav)
- Command palette (Ctrl+P overlay)
- 6 types of modals (input, password, select, multi-select, scrollable, confirm)
- Session launchers (specialized modal)
- Header with breadcrumb
- Status bar with contextual hints
- Flash messages in status bar
- Responsive breakpoints changing layout
- View-specific shortcuts (non-standardized)

This created cognitive overload: users couldn't predict whether selecting a menu item would navigate to a view, open a modal, or execute an action directly.

## Decision Drivers

- Power users who use the tool daily and memorize shortcuts
- Balanced mix of actions (sessions, projects, config, system) — no dominant workflow
- Desire for an interface like opencode: airy, simple, single interaction point
- fzf/Telescope inspiration: fuzzy search as the primary interaction pattern

## Considered Options

1. **Simplify existing** — Keep sidebar + palette, reduce items
2. **Tabs/sections** — Replace tree with horizontal navigation
3. **Omnibar-first** — Single command input replaces menu, palette, header, status bar

## Decision Outcome

Chosen option: **Omnibar-first** — complete redesign removing the sidebar menu, header, status bar, and modal system in favor of a single persistent omnibar at the bottom of the screen.

### Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                      CONTENT (full screen)                        │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  [contextual hints / command input]                          │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

### Key design decisions

1. **Flat command registry** replaces hierarchical menu tree. All actions are commands with ID, aliases, and fuzzy-searchable metadata.
2. **Omnibar replaces 5 components**: sidebar menu, command palette, header (breadcrumb), status bar (hints), session launchers.
3. **Inline prompts** replace modal overlays. Inputs, selects, and scrollable content appear in the content zone, not as popup overlays.
4. **4 global shortcuts only**: `Ctrl+P`, `Esc`, `Ctrl+Q`, `Tab` — everything else via omnibar or view-specific keys.
5. **Any printable key activates the omnibar** if the current view doesn't consume it — zero-friction command access.
6. **Views retain `HandleKey`** for contextual shortcuts (j/k, a/d, h/l) — the omnibar only captures unhandled runes.

### Consequences

**Positive:**
- Radically simpler mental model (one input for all actions)
- Maximum screen real estate for content
- Consistent interaction pattern across all features
- Lower learning curve (type what you want, fuzzy match finds it)
- Easier to add new commands (no tree restructuring needed)

**Negative:**
- Loss of discoverability from visible menu (mitigated by omnibar showing all commands on empty query)
- Views that needed complex multi-modal workflows (team init) use chained inline prompts
- Power users who memorized menu structure need to re-learn (low cost given the simplification)

## Components Removed

- `v2/menu/` package (entire directory)
- `shell/header.go`
- `shell/statusbar.go`
- `shell/palette.go`
- `shell/launcher.go`
- `shell/flash.go`
- `shell/responsive.go`

## Components Added

- `shell/command.go` — Command struct + CommandRegistry with fuzzy search
- `shell/omnibar.go` — Persistent input widget with suggestion overlay

## Links

- Supersedes: [ADR-025: TUI Unified Shell](./025-tui-unified-shell.en.md)
