---
page: stack
title: Technology Stack
confidence: CONFIRMED
sources:
  - cli/go.mod
  - cli/.goreleaser.yml
  - cli/Makefile
last_updated: 2026-10-02
---

> [Lire en francais](stack.fr.md)

# Technology Stack

## Runtime

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.26.4 |
| Binary | Single static binary | `CGO_ENABLED=0` |
| TUI framework | [tview](https://github.com/rivo/tview) | tcell-based |
| CLI framework | [Cobra](https://github.com/spf13/cobra) | v1.9+ |
| Configuration | [Viper](https://github.com/spf13/viper) + TOML | -- |
| Database | SQLite (via modernc.org/sqlite) | Pure Go, no CGO |
| Secrets | OS Keychain ([go-keyring](https://github.com/zalando/go-keyring)) + AES-256-GCM fallback | -- |

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
| AI runtime | [OpenCode](https://opencode.ai/) |
| Protocol | MCP (Model Context Protocol) -- stdin/stdout subprocess |
| Providers | Anthropic Claude, AWS Bedrock, OpenAI (configurable) |
| Agents | 19 agents with hybrid skill architecture (Bucket A/B) |
| Skills | 197 protocol skills (embedded via go:embed) |

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

## Key Design Decisions

- **Zero dependencies at runtime** -- single static binary, no Docker, no Node.js, no Python
- **Embedded content** -- agents/skills/permissions compiled into binary via `go:embed`
- **Pure Go SQLite** -- `modernc.org/sqlite` avoids CGO for true static builds
- **TOML configuration** -- human-readable, git-friendly, supports hierarchical cascade
- **OS keychain first** -- secrets stored in native secure storage, encrypted file as fallback only
