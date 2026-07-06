# 022 — CLI Migration from Bash to Go

## Status

accepted

## Context

The original CLI (`oc`) was a collection of ~60 shell scripts (bash) totaling ~15,000 lines of code, orchestrated by a central dispatcher (`oc.sh`). It depended on external tools: jq, Node.js, sqlite3, bun, curl, and GNU parallel.

An analysis conducted in June 2026 (`docs/dev/cli-migration-analysis.md`) identified 17 structural problems:

- **Fragility**: text parsing with awk/sed/grep, no type safety, silent failures
- **External dependencies**: 6+ runtime tools required, platform-specific behavior
- **Testing**: BATS tests were slow, flaky, and incomplete (no unit isolation)
- **Performance**: 30+ subprocess spawns per deploy, repeated file reads (api-keys.local.md read 30+ times)
- **Distribution**: no binary — users had to clone the repo and create shell aliases
- **Secrets**: API keys stored in plain-text markdown files
- **i18n**: not supported — all strings hardcoded in French
- **Configuration**: JSON files parsed with jq, no schema validation, no migration path
- **MCP servers**: TypeScript processes requiring Node.js runtime

These problems were intrinsic to the shell architecture and could not be resolved through incremental refactoring.

## Decision

We decided to rewrite the CLI entirely in Go, under the new name `oh`:

- **Architecture**: monorepo — Go code in `cli/` coexisting with `agents/` and `skills/`
- **Stack**: Go 1.26 + Cobra (commands) + Viper (config) + BubbleTea (TUI) + huh (forms) + lipgloss (styling)
- **Distribution**: single static binary via GoReleaser + Homebrew tap (`datichb/openhub/oh`)
- **Configuration**: TOML (`~/.oh/hub.toml`) replacing JSON (`config/hub.json`)
- **Secrets**: OS keychain (go-keyring) with AES-256-GCM encrypted file fallback
- **Projects & Sessions**: SQLite database (replacing markdown files)
- **MCP servers**: native Go implementations (replacing TypeScript in `servers/`)
- **i18n**: JSON-based bilingual system (fr/en), 474 keys with parity enforcement
- **Migration strategy**: Strangler Fig — Go CLI developed in parallel, then bash removed

The migration was executed in 7 development phases plus 3 priority passes (P0, P1, P2 with 5 blocs), an audit cycle (33 findings, 6 remediation phases A-F), and a final cleanup bloc. Total effort: ~12 days spread over June-July 2026.

Reference documents:
- `docs/dev/cli-migration-plan-v2.md` — full technical plan
- `docs/dev/cli-remaining-work.md` — backlog tracking (100% complete)
- `docs/dev/cli-migration-analysis.md` — initial problem analysis

## Consequences

### Positive

- **Zero runtime dependencies** — single ~5.5 MB binary, only requires git
- **Cross-platform** — darwin/amd64, darwin/arm64, linux/amd64, linux/arm64
- **Robust test suite** — 213 tests across 24 packages, race-condition checked
- **Instant startup** — no interpreter boot, no module resolution
- **Homebrew distribution** — `brew install datichb/openhub/oh`
- **Interactive TUI** — dashboard, kanban board, picker, forms (BubbleTea)
- **Full i18n** — 474 keys, bilingual (fr/en), parity test enforced
- **Secure secrets** — OS keychain with encrypted fallback (Argon2id + AES-256-GCM)
- **Native MCP** — Go servers with no Node.js dependency
- **Structured logging** — `log/slog` with `--verbose` flag
- **Shell completion** — bash, zsh, fish, powershell

### Negative / Trade-offs

- **Breaking migration** for existing users — mitigated by `MIGRATION.md` guide with full command equivalence table
- **Features removed** — `oh conventions`, `oh agent create/edit`, `oh skills install/remove`, session state machine, Node.js installer (judged non-essential or handled natively by opencode)
- **Go toolchain required** to contribute to CLI code (vs. editing shell scripts)
- **Binary size** — ~5.5 MB vs. ~0 for shell scripts (acceptable for the gains)

## Alternatives Rejected

| Alternative | Reason for rejection |
|-------------|---------------------|
| TypeScript + Bun | Initially recommended (June 2026 analysis). Rejected: runtime dependency (Node/Bun), no native binary without bundling, CLI ecosystem less mature than Go (Cobra/Viper/BubbleTea) |
| Python (Click/Typer) | Runtime dependency, complex distribution (pyinstaller/shiv), no native TUI comparable to BubbleTea |
| Rust (clap + ratatui) | Longer development time, steeper learning curve, smaller ecosystem for interactive CLI forms |
| Incremental bash refactoring | Structurally impossible — the 17 identified problems are inherent to shell scripting (no types, no packages, no testability) |
| Hybrid approach (keep some bash) | Maintenance burden of two systems, user confusion between `oc` and `oh` entry points |
