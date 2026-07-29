# ADR-031: Configuration View Consolidation

## Status

Accepted

## Date

2026-07-29

## Context

The TUI currently has 10 configuration-related views, built at different times with inconsistent interaction patterns:

| Problem | Impact |
|---|---|
| ConfigView and HubConfigView both edit hub.toml with 8 overlapping fields | Stale-read bugs; user confusion about which view is canonical |
| MCPView and MCPConfigView address the same domain (MCP services) but split read/write operations | User must navigate two views to manage one concern |
| MCPView project panel and ProjectConfigView MCP section both edit per-project MCP overrides | Duplicate mutation paths; changes in one are invisible in the other |
| 4 different base widgets across 10 views (TextView, List, Table, dual-List) | No consistent muscle memory |
| Auto-save (5 views) vs. explicit-save with `w` (3 views) vs. read-only (2 views) | Unpredictable persistence semantics |
| Space/Enter/e keybindings vary per view | Users cannot form a universal interaction mental model |
| Only 4/10 views implement CommandProvider for omnibar | Inconsistent discoverability |
| HubConfigView uses raw `term.ReadPassword(syscall.Stdin)` | Corrupts TUI rendering |

## Decision

Consolidate configuration views from 10 to 8, with a unified interaction contract:

### View inventory (after)

| View | ViewID | Purpose |
|---|---|---|
| SettingsView (renamed HubConfigView) | `settings` | Hub-level personal defaults |
| TeamsView (new) | `teams` | Multi-team management: list, add, remove, sync |
| TeamDetailView (replaces MCPConfigView) | `team.detail` | Single team config with resolution display (enforced/recommended/effective) |
| ProjectConfigView (enhanced) | `project.config` | Per-project overrides with source/enforced annotations |
| MCPView (simplified) | `mcp` | Hub-level MCP: tokens, write, enabled (project panel removed) |
| ProviderView | `provider` | Provider credential wizard |
| ModelsView (enhanced) | `models` | Model cascade with source column (team/hub/project) |
| SecretsView | `secrets` | Keychain credential management |

PluginsView and PoliciesView remain unchanged.

### Removed
- **ConfigView**: fully redundant with SettingsView (its exclusive fields migrated to SettingsView or ProviderView)
- **MCPConfigView**: absorbed into TeamDetailView (same concern, better integration)
- **MCPView project panel**: absorbed into ProjectConfigView (single mutation path for project MCP overrides)

### Unified interaction contract

| Key | Action | All views |
|---|---|---|
| j/k, arrows | Navigate | Yes |
| Enter | Edit selected item | Yes (where editable) |
| Space | Toggle (bool/tri-state/cycle) | Yes (where toggleable) |
| a | Add | Views with CRUD |
| d | Delete (with confirmation modal) | Views with CRUD |
| u | Undo last action | All (via UndoStack) |
| r | Refresh from source | All |
| v | Toggle view mode (simple/detailed) | Views with modes |
| ? | Help (contextual keybindings) | All |

### Persistence model: Auto-save + Undo

All views use auto-save (immediate persistence on every mutation) with a bounded undo stack (depth 10). No more explicit `w` to save, no more dirty tracking, no more "unsaved changes" warnings. The undo key (`u`) reverts the last auto-saved mutation.

### Base widget standardization

- `tview.List` (via `SectionedList` widget): default for all config editing views
- `tview.Table`: exception for ModelsView (tabular multi-column data)
- `tview.TextView`: only for read-only status pages (PluginsView)

### All views implement CommandProvider

Contextual omnibar commands on all 8 config views (was 4/10 before).

## Alternatives Considered

| Alternative | Rejected Because |
|---|---|
| Single unified "Settings" view with tabs for everything | Too monolithic; loses the benefit of focused domain views; tab bar eats vertical space in TUI |
| Keep all 10 views but just harmonize keybindings | Does not solve the duplicate-mutation and stale-read problems |
| Explicit-save everywhere (with dirty tracking) | Higher friction; users forget to save; auto-save + undo is simpler mental model |
| Auto-save without undo | Too risky for destructive operations (delete, toggle production settings) |

## Consequences

### Positive
- Single mutation path per setting — no more stale reads across views
- Consistent muscle memory across all config views
- Auto-save + undo: zero-friction editing with safety net
- `term.ReadPassword` eliminated (replaced by `ShowPasswordModal`)
- New TeamsView supports multi-team workflows (ADR-029)
- Resolution annotations (ADR-030) visible in ProjectConfigView and TeamDetailView

### Negative / Trade-offs
- Users familiar with the current ConfigView shortcut must learn the new SettingsView
- The MCPConfigView 3-column display is now one mode (`v`) in TeamDetailView — slightly less immediate
- PluginsView retains TextView (not List) — acceptable as it's a status page with minimal interaction
- SectionedList widget is a new abstraction to maintain
