# Guide — Using the OpenHub TUI

> Practical guide for navigating and working in the TUI interface.

## Quick Start

```bash
oh
```

The TUI displays a splash screen with quick-start hints and the omnibar at the bottom.

## Core Workflow

### 1. Launch a command

At any time, press `Ctrl+P` or start typing:
- The omnibar activates
- Type a command name (fuzzy matching)
- Press `Enter` to execute

### 2. Start a coding session

```
> start
```

Select the mode (Standard, Dev, Onboard). The TUI suspends, opencode starts. When you exit opencode, the TUI resumes.

### 3. Quick access to specific actions

Type the command directly — fuzzy matching finds it fast:
```
> secu        → launches security audit
> dep         → deploys to active project
> doc         → opens doctor diagnostics
```

### 4. Manage projects

```
> projects
```

In the projects view:
- `a` to add a new project
- `d` to delete
- `Enter` to configure
- `r` to rename

### 5. Edit configuration

```
> config
```

Navigate with `j`/`k`, press `Enter` to edit a value.

### 6. Check system health

```
> doctor
```

Checks run automatically. Press `r` to re-run.

## Tips

- **Any letter activates the omnibar** — no need for `Ctrl+P` if the current view doesn't use that key
- `Esc` always goes back (previous view, or dismiss omnibar)
- `Ctrl+Q` quits at any time
- Toasts (top-right) confirm action results (success/error)
- All commands support fuzzy matching — type partial words, abbreviations, or aliases
- Views have contextual shortcuts shown in the omnibar hint text

## Session Types

| Type | What it does |
|------|-------------|
| Start Standard | Interactive opencode session |
| Start Dev | Development-oriented session (ticket workflow) |
| Start Onboard | Project onboarding session |
| Audit (6 types) | Specialized code audit with dedicated prompts |
| Review (4 modes) | Code review with varying depth and focus |
| Debug | Debug session with issue description |
| Quick | Direct opencode launch (no selection) |
