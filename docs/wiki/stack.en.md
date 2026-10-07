---
page: stack
title: Technology Stack
confidence: CONFIRMED
sources:
  - cli/go.mod
  - cli/.goreleaser.yml
  - cli/Makefile
  - docs/architecture/overview.en.md
last_updated: 2026-10-06
---

> [Lire en francais](stack.fr.md)

# Technology Stack

## Runtime

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.26.4 |
| Binary | Single static binary | `CGO_ENABLED=0` |
| TUI framework | [tview](https://github.com/rivo/tview) | tcell-based |
| Inline prompts (outside the TUI) | [huh](https://github.com/charmbracelet/huh) | -- |
| CLI framework | [Cobra](https://github.com/spf13/cobra) | v1.10+ |
| Configuration | [Viper](https://github.com/spf13/viper) + TOML | -- |
| Database | SQLite (via modernc.org/sqlite) | Pure Go, no CGO |
| Secrets | OS Keychain ([go-keyring](https://github.com/zalando/go-keyring)) + AES-256-GCM fallback | -- |
| System calls | [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | Daemon socket peer UID, file locks (Windows) |
| Container engine (optional) | Colima, Podman or Docker CLI (OCI CLI, no Docker API) | Container runtime only |
| Remote execution (optional) | GitLab CI (`oh-runner` project, Kaniko or Docker-in-Docker) | Remote runtime only |

## Build and Release

| Tool | Purpose |
|------|---------|
| [GoReleaser](https://goreleaser.com/) v2 | Cross-compilation, artifact signing, GitHub Release |
| [Cosign](https://docs.sigstore.dev/cosign/) | Keyless signing (Sigstore OIDC) |
| [golangci-lint](https://golangci-lint.run/) v2.1 | Go linting (bodyclose, gocritic, misspell, nilerr) |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Go dependency vulnerability scanning |
| GitHub Actions | CI (3-OS matrix) + Release automation |
| Dependabot | Weekly dependency updates |

## AI Integration

| Component | Technology |
|-----------|-----------|
| AI runtime | [OpenCode](https://opencode.ai/) V2 (>= 2.0.0, `opencode serve`; V1 is no longer supported) |
| Protocol | MCP (Model Context Protocol) -- stdin/stdout subprocess; streamable HTTP through the daemon gateway off the machine |
| Credentials | `ohd` credential proxy (per-group `ohs_` token; real keys stay in the keychain; SigV4 for AWS profiles) |
| Providers | Anthropic Claude, AWS Bedrock, OpenAI, OpenRouter (configurable) |
| Workflows | 12 declarative `oh/v1` workflows (YAML, embedded via go:embed) |
| Agents | 20 agents with hybrid skill architecture (Bucket A/B), including the generic entry agent `conductor` |
| Skills | 184 protocol skills + 14 annexes (embedded via go:embed, delivered through the session bundle) |
| Plugin | oh plugin for opencode (TypeScript, embedded; closed world and checkpoints) |

## External Integrations (MCP Servers)

| Server | Protocol | Capabilities |
|--------|----------|-------------|
| Figma | REST API | Read mockups, tokens, components |
| GitLab | REST API + GraphQL | Issues, MRs, discussions, labels |
| GitHub | REST API | Issues, PRs, check runs |
| Jira | REST API | Issues, transitions, comments |
| Linear | GraphQL | Issues, states, assignees |
| Google Slides | REST API | Presentations, slides (read-only) |
| Team | Local (git-backed) | Claims, wiki, events, notifications |
| Workflow | Local (served by oh) | Workflow status, checkpoints, typed outputs |

## Key Design Decisions

- **Zero dependencies at runtime** -- single static binary, no Node.js, no Python; opencode V2 is installed with its own tool; a container engine is only needed for the optional container runtime
- **Embedded content** -- agents/skills/permissions/workflows compiled into binary via `go:embed`, assembled at launch into an immutable session bundle (`~/.oh/bundles/<hash>/`)
- **Pure Go SQLite** -- `modernc.org/sqlite` avoids CGO for true static builds
- **TOML configuration** -- human-readable, git-friendly, supports hierarchical cascade
- **OS keychain first** -- secrets stored in native secure storage, encrypted file as fallback only
