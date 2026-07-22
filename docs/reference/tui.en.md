# TUI Reference — OpenHub

> Complete documentation for the OpenHub terminal user interface (TUI).

## Launch

```bash
oh          # Launches the TUI (interactive terminal detected)
oh --no-tui # Forces classic CLI mode
```

The TUI launches automatically when `oh` is executed without a subcommand in an interactive terminal. Environment variables that disable the TUI: `CI=true`, `TERM=dumb`, `OH_RICH_TUI=0`.

## Design Principles

The TUI follows an **omnibar-first** design inspired by fuzzy launchers (fzf, Telescope) and opencode's minimalist approach:

- **Single interaction point**: the omnibar handles all commands
- **Content-first**: maximum screen space dedicated to content
- **Contextual**: the interface adapts to the current view
- **Minimal shortcuts**: only 4 global keybindings to memorize

## Layout

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                      CONTENT AREA                                 │
│              (adapts to the active view)                          │
│                                                                   │
├───────────────────────────────────────────────────────────────────┤
│  > _  Ctrl+P command · Esc back · Ctrl+Q quit                    │ Omnibar (1 row)
└───────────────────────────────────────────────────────────────────┘
```

## Global Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` | Activate the omnibar |
| `Esc` | Go back (previous view) |
| `Ctrl+Q` | Quit the TUI |
| Any letter | Activate omnibar with that character (if not consumed by the view) |

That's it. Four shortcuts. Everything else goes through the omnibar.

## The Omnibar

The omnibar is always visible at the bottom of the screen. It has two modes:

### Passive Mode (default)

Displays contextual hints for the active view:

```
│  Ctrl+P command · j/k nav · Enter open · h/l columns            │
```

### Active Mode (input)

Accepts text input with fuzzy suggestions displayed above:

```
│  ● Audit Sécurité      Audit de sécurité (OWASP, injections)    │
│  ○ Audit Performance   Audit performance (N+1, mémoire, CPU)    │
│  ○ Audit Architecture  Audit d'architecture (couplage, patterns) │
├───────────────────────────────────────────────────────────────────┤
│  > audit_                                                         │
```

### Omnibar Controls

| Key | Action |
|-----|--------|
| Type | Filter commands fuzzy |
| `Down` / `Tab` | Move selection down |
| `Up` / `Shift+Tab` | Move selection up |
| `Enter` | Execute selected command |
| `Esc` | Dismiss and return to content |

## Available Commands

### Sessions

| Command | Aliases | Description |
|---------|---------|-------------|
| `start` | session, code, launch | Launch an opencode session (with mode selector) |
| `start dev` | dev, ticket | Dev-oriented session |
| `start onboard` | onboard | Project onboarding session |
| `audit` | — | Launch an audit (type selector) |
| `audit security` | secu | Security audit (OWASP) |
| `audit performance` | perf | Performance audit |
| `audit architecture` | archi | Architecture audit |
| `audit accessibility` | a11y | Accessibility audit |
| `audit ecodesign` | eco | Environmental impact audit |
| `audit observability` | obs | Observability audit |
| `review` | rev, cr | Launch a code review |
| `review standard` | — | Classic code review |
| `review adversarial` | adversarial | Adversarial review |
| `review edge` | edge | Edge-case focused review |
| `review complete` | complete, all | Full review (all modes) |
| `debug` | dbg | Debug session (with issue prompt) |
| `quick` | q, fast | Launch opencode directly |
| `parallel` | par, multi | Parallel sessions view |

### Projects

| Command | Aliases | Description |
|---------|---------|-------------|
| `board` | kanban, tasks | Project kanban board |
| `projects` | proj, list | Projects list |
| `deploy` | dep, push | Deploy agents/skills to the active project |
| `sync` | synchronize | Synchronize all projects |

### Configuration

| Command | Aliases | Description |
|---------|---------|-------------|
| `config` | cfg, settings, hub | Hub configuration |
| `models` | mod, model, llm | Model configuration |
| `provider` | prov, api | LLM provider settings |
| `mcp` | servers | MCP servers |

### System

| Command | Aliases | Description |
|---------|---------|-------------|
| `status` | stat, info | System status |
| `doctor` | health, check | Health diagnostics |
| `metrics` | met, stats, tokens | Usage statistics |
| `plugins` | plug, extensions | Plugin management |
| `upgrade` | up, update | Update opencode |
| `help` | ?, aide, shortcuts | Help and shortcuts |

### Navigation

| Command | Aliases | Description |
|---------|---------|-------------|
| `home` | accueil, welcome | Return to splash screen |
| `quit` | exit, q | Quit the TUI |

### Team (when enabled)

| Command | Aliases | Description |
|---------|---------|-------------|
| `team board` | team kanban | Team kanban board |
| `team status` | team stat | Team status |
| `team activity` | activite, feed | Team activity feed |
| `takeover briefs` | takeover | Takeover context briefs |
| `worktrees` | wt | Git worktree management |
| `patterns` | pat | Team patterns |
| `policies` | pol, rules | Team policies |

## View-Specific Shortcuts

When a view is active, it may have additional shortcuts that work without activating the omnibar. These are displayed in the omnibar's passive hint text.

### Board View

| Key | Action |
|-----|--------|
| `h` / `l` | Move between columns |
| `j` / `k` | Navigate items |
| `r` | Refresh |
| `Enter` | Open item detail |

### Projects View

| Key | Action |
|-----|--------|
| `a` | Add project |
| `d` | Delete project |
| `r` | Rename project |
| `Enter` | Configure project |

### Config View

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate keys |
| `Enter` | Edit selected value |

## Opencode Sessions

When a session is launched (Start, Audit, Review, Debug, Quick):
1. The TUI suspends itself
2. opencode takes full terminal control
3. When opencode exits, the TUI resumes exactly where it was
4. A toast confirms the session outcome
