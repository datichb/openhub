# ADR-025: Unified TUI Shell with Integrated Navigation

## Status

Accepted

## Date

2026-07-17

## Context

The `oh` CLI has 50+ commands across 14 functional domains and 6 independent full-screen TUI views (dashboard, board, teamboard, wizard, picker, parallel). Each view spawns its own `tview.Application` — navigating between views requires quitting the process. The existing sidebar is a stub prepared for future menu navigation.

Identified problems:
1. No router/navigation — each command launches an independent app
2. No discovery menu — users must know CLI commands
3. Two incompatible theme systems (`v2/theme` tcell vs `common/` lipgloss)
4. No visual feedback (no toast, no modal, no breadcrumb)
5. Sidebar is a stub with a single static item

## Decision

Create a unified TUI shell accessible via bare `oh` (no arguments), built on:

### Technical Architecture

- **`View` interface**: common contract for all views (`ID()`, `Title()`, `Mount()`, `Unmount()`, `StatusHints()`, `HandleKey()`)
- **Stack-based Router**: push/pop/replace navigation with history (back via Esc)
- **Single Shell**: one persistent `tview.Application` managing header, menu, content, status bar
- **Pages overlay**: `tview.Pages` wrapper for modals and toasts above content
- **Hierarchical menu**: `tview.TreeView` in the sidebar with expandable/collapsible categories

### Unified Design

- **Single palette**: `tui/theme/` package with hex constants deriving both tcell AND lipgloss values
- **Dual-accent**: Azure (#64a0ff) for structure/navigation, Gold (#e0a030) for actions/CTA
- **3-section header**: logo | breadcrumb | meta-info
- **3-section status bar**: current view | contextual hints | global info

### OpenCode Integration

- **Phase 1**: Process switching via `app.Suspend()` → exec opencode → resume
- **Phase 2**: Monitoring panel via REST API `opencode serve` (headless)

### Entry Point

- `oh` with no arguments in an interactive terminal → launches the TUI shell
- Individual commands (`oh board`, `oh start`, etc.) remain functional standalone
- `--no-tui` flag forces classic CLI behavior

## Alternatives Considered

| Alternative | Rejection Reason |
|---|---|
| Keep views independent | No navigation, no discoverability, fragmented UX |
| Migrate everything to BubbleTea | Full rewrite, loss of 6 existing tview views |
| Use tmux/zellij for multiplexing | External dependency, complex user setup |
| Embed opencode as a tview sub-view | Impossible — opencode is a TypeScript binary with its own TUI |
| Dedicated `oh tui` command | Less discoverable than bare `oh` |

## Consequences

### Positive
- Unified UX: seamless navigation between all views without quitting
- Discoverability: menu exposes all available features
- Visual consistency: single palette, single design system
- Feedback: toast, modal, breadcrumb improve user comprehension
- Backward compatible: standalone commands remain functional

### Negative
- Increased TUI code complexity (router, lifecycle, overlay)
- Two access modes to maintain (direct CLI + TUI shell)
- Larger test surface (SimScreen for widgets)

### Neutral
- Existing layout (`layout.Build()`) preserved for standalone views
- huh/BubbleTea views (floating prompt, quick) remain in separate inline mode
- Phase 2 opencode (REST monitoring) is not blocking for launch

## Implementation

- Unified theme package: `cli/internal/tui/theme/`
- Shell: `cli/internal/tui/v2/shell/`
- Router: `cli/internal/tui/v2/router/`
- Menu: `cli/internal/tui/v2/menu/`
- View interface: `cli/internal/tui/v2/views/view.go`
- Entry point: `cli/cmd/root.go` (default RunE)
