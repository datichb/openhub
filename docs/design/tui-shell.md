# TUI Shell — Design Specification

> Design reference for the omnibar-first TUI shell of OpenHub.
> Palette, dimensions, visual states, components.

## Design Philosophy

1. **Content-first** — Maximum screen real estate for the active view
2. **Single interaction point** — The omnibar handles all command discovery and execution
3. **Minimal chrome** — No permanent header, sidebar, or status bar
4. **Contextual hints** — The omnibar displays relevant shortcuts for the active view
5. **Predictable** — Esc=back, Enter=action, Ctrl+Q=quit, any letter=command
6. **Immediate feedback** — Every action produces a toast notification

## Unified Palette (Catppuccin Mocha)

### Backgrounds (3 depth levels)

| Role | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| App | `#181825` | Mantle | Terminal background |
| Panel | `#1e1e2e` | Base | Content panels |
| Element | `#313244` | Surface0 | Omnibar, selection, hover |

### Text (3 contrast levels)

| Role | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| Primary | `#cdd6f4` | Text | Titles, content, active items |
| Secondary | `#a6adc8` | Subtext0 | Descriptions, categories |
| Muted | `#7f849c` | Overlay1 | Placeholders, disabled, hints |

### Semantic Colors

| Role | Hex | Catppuccin | Usage |
|------|-----|-----------|-------|
| Accent | `#89b4fa` | Blue | Omnibar borders, navigation |
| Action | `#fab387` | Peach | Active item, spinner |
| Success | `#a6e3a1` | Green | Confirmations, done |
| Warning | `#f9e2af` | Yellow | Non-blocking alerts |
| Error | `#f38ba8` | Red | Errors, failures |
| Info | `#b4befe` | Lavender | Running, in-progress |

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                                                                   │
│                    CONTENT AREA                                    │
│             (fills entire terminal minus 1 row)                   │
│                                                                   │
│            Padding: top=1, left=2, right=2                        │
│                                                                   │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  ◆ > _  hints · hints · hints                          Ctrl+P    │ Omnibar (1 row, bg: Element)
└───────────────────────────────────────────────────────────────────┘
```

### Dimensions

| Component | Size | Type |
|-----------|------|------|
| Content | terminal height - 1 | Flex (proportion 1) |
| Omnibar | 1 row | Fixed |
| Content padding | top=1, left=2, right=2 | — |

## Omnibar

### Passive Mode (hints)

```
│  Ctrl+P commande · j/k nav · Enter ouvrir · Esc retour           │
```

- Background: Element (`#313244`)
- Text: Secondary (`#a6adc8`)
- Accent keys: Blue (`#89b4fa`)

### Active Mode (input)

```
│  ◆ > start_                                                       │
```

- Background: Element (`#313244`)
- Label: Action color (`#fab387`) — `◆ >`
- Input text: Primary (`#cdd6f4`)
- Placeholder: Muted (`#7f849c`)

### Suggestions Overlay

```
│  ● Start Standard       Session interactive classique            │
│  ○ Start Dev            Session orientée développement           │
│  ○ Start Onboard        Session d'onboarding projet              │
├──────────────────────────────────────────────────────────────────┤
│  ◆ > start_                                                      │
```

- Positioned just above omnibar via Pages overlay
- Border: Normal (`#313244`)
- Selected item: bg Element, text Primary
- Non-selected: text Primary, description Muted
- Max 10 items visible

## Splash Screen (Home View)

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                                                                   │
│              ___                   _   _       _                  │
│             / _ \ _ __   ___ _ __ | | | |_   _| |__              │
│            | | | | '_ \ / _ \ '_ \| |_| | | | | '_ \            │
│            | |_| | |_) |  __/ | | |  _  | |_| | |_) |           │
│             \___/| .__/ \___|_| |_|_| |_|\__,_|_.__/            │
│                  |_|                                              │
│                                                                   │
│          ─────────────────────────────────────────                │
│                                                                   │
│            Ctrl+P  ouvrir l'omnibar       Esc  retour            │
│            start   lancer une session     quit quitter           │
│            board   kanban projet          help raccourcis         │
│                                                                   │
│          ─────────────────────────────────────────                │
│                                                                   │
│        Tapez n'importe quelle lettre pour chercher une commande  │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  ◆ > _  Ctrl+P commande · Ctrl+Q quitter                         │
└───────────────────────────────────────────────────────────────────┘
```

- Logo: Action color (`#fab387`)
- Hints keys: Accent (`#89b4fa`)
- Hints commands: Secondary (`#a6adc8`)
- Separators: Muted (`#7f849c`)
- Bottom message: Muted

## Toast Notifications

### Position

- Top-right of content area (via Grid overlay in Pages)
- Width: auto (message + 8 padding)
- Height: 3 rows
- Duration: 2.5s (success/info), 3.5s (error)

### Types

| Type | Border | Icon | Example |
|------|--------|------|---------|
| Success | Green | ✓ | `✓ Projet ajouté` |
| Error | Red | ✗ | `✗ Échec du deploy` |
| Warning | Yellow | ! | `! opencode non trouvé` |
| Info | Blue | ▸ | `▸ Sync en cours...` |

### Behavior

- Does not capture focus (inputs go to content/omnibar)
- Auto-dismiss after timeout
- Max 3 stacked

## Inline Prompts

When a command needs user input, the content zone shows an inline form:

### Input Prompt

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                                                                   │
│           Description du problème: [                        ]     │
│           Enter confirmer  Esc annuler                            │
│                                                                   │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  ◆ > _                                                            │
```

### Select Prompt (fzf-style)

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│        ┌─ Lancer une session · Enter sélectionner · Esc ─┐       │
│        │  Standard                                        │       │
│        │  Dev (ticket)                                    │       │
│        │  Onboard                                         │       │
│        └──────────────────────────────────────────────────┘       │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  ◆ > _                                                            │
```

## Icons (canonical)

| Constant | Char | Usage |
|----------|------|-------|
| `IconActive` | `◆` | Logo, omnibar label |
| `IconSuccess` | `✓` | Toast success |
| `IconError` | `✗` | Toast error |
| `IconWarning` | `!` | Toast warning |
| `IconArrow` | `▸` | Toast info |
| `IconDot` | `·` | Separator in hints |
