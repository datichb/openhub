---
page: stack
title: Stack technologique
confidence: CONFIRMED
sources:
  - cli/go.mod
  - cli/.goreleaser.yml
  - cli/Makefile
last_updated: 2026-10-02
---

> [Read in English](stack.en.md)

# Stack technologique

## Runtime

| Composant | Technologie | Version |
|-----------|------------|---------|
| Langage | Go | 1.26.4 |
| Binaire | Binaire statique unique | `CGO_ENABLED=0` |
| Framework TUI | [tview](https://github.com/rivo/tview) | base tcell |
| Framework CLI | [Cobra](https://github.com/spf13/cobra) | v1.9+ |
| Configuration | [Viper](https://github.com/spf13/viper) + TOML | -- |
| Base de donnees | SQLite (via modernc.org/sqlite) | Go pur, pas de CGO |
| Secrets | Trousseau systeme ([go-keyring](https://github.com/zalando/go-keyring)) + fallback AES-256-GCM | -- |

## Build et release

| Outil | Usage |
|-------|-------|
| [GoReleaser](https://goreleaser.com/) v2 | Compilation croisee, signature d'artefacts, GitHub Release |
| [Cosign](https://docs.sigstore.dev/cosign/) | Signature sans cle (Sigstore OIDC) |
| [golangci-lint](https://golangci-lint.run/) v2.1 | Linting Go (bodyclose, gocritic, misspell, nilerr) |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Scan de vulnerabilites des dependances Go |
| GitHub Actions | CI (matrice 3 OS) + automatisation release |
| Dependabot | Mises a jour hebdomadaires des dependances |

## Integration IA

| Composant | Technologie |
|-----------|-----------|
| Runtime IA | [OpenCode](https://opencode.ai/) |
| Protocole | MCP (Model Context Protocol) -- sous-processus stdin/stdout |
| Fournisseurs | Anthropic Claude, AWS Bedrock, OpenAI (configurable) |
| Agents | 19 agents avec architecture de skills hybride (Bucket A/B) |
| Skills | 197 skills de protocole (embarques via go:embed) |

## Integrations externes (serveurs MCP)

| Serveur | Protocole | Capacites |
|---------|----------|-----------|
| Figma | REST API | Lecture mockups, tokens, composants |
| GitLab | REST API + GraphQL | Issues, MRs, discussions, labels |
| GitHub | REST API | Issues, PRs, check runs |
| Jira | REST API | Issues, transitions, commentaires |
| Linear | GraphQL | Issues, etats, assignees |
| Google Slides | REST API | Presentations, slides (lecture seule) |
| Team | Local (git-backed) | Claims, wiki, evenements, notifications |

## Decisions de conception cles

- **Zero dependance au runtime** -- binaire statique unique, pas de Docker, pas de Node.js, pas de Python
- **Contenu embarque** -- agents/skills/permissions compiles dans le binaire via `go:embed`
- **SQLite Go pur** -- `modernc.org/sqlite` evite CGO pour des builds vraiment statiques
- **Configuration TOML** -- lisible par l'humain, compatible git, supporte la cascade hierarchique
- **Trousseau systeme en priorite** -- secrets stockes dans le stockage securise natif, fichier chiffre en fallback uniquement
