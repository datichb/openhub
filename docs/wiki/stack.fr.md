---
page: stack
title: Stack technologique
confidence: CONFIRMED
sources:
  - cli/go.mod
  - cli/.goreleaser.yml
  - cli/Makefile
  - docs/architecture/overview.fr.md
last_updated: 2026-10-06
---

> [Read in English](stack.en.md)

# Stack technologique

## Runtime

| Composant | Technologie | Version |
|-----------|------------|---------|
| Langage | Go | 1.26.4 |
| Binaire | Binaire statique unique | `CGO_ENABLED=0` |
| Framework TUI | [tview](https://github.com/rivo/tview) | base tcell |
| Invites en ligne (hors TUI) | [huh](https://github.com/charmbracelet/huh) | -- |
| Framework CLI | [Cobra](https://github.com/spf13/cobra) | v1.10+ |
| Configuration | [Viper](https://github.com/spf13/viper) + TOML | -- |
| Base de donnees | SQLite (via modernc.org/sqlite) | Go pur, pas de CGO |
| Secrets | Trousseau systeme ([go-keyring](https://github.com/zalando/go-keyring)) + fallback AES-256-GCM | -- |
| Appels systeme | [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | UID du pair sur le socket du demon, verrous de fichiers (Windows) |
| Moteur de conteneurs (optionnel) | Colima, Podman ou Docker CLI (CLI OCI, pas d'API Docker) | Runtime conteneur uniquement |
| Execution distante (optionnelle) | GitLab CI (projet `oh-runner`, Kaniko ou Docker-in-Docker) | Runtime distant uniquement |

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
| Runtime IA | [OpenCode](https://opencode.ai/) V2 (>= 2.0.0, `opencode serve` ; V1 n'est plus pris en charge) |
| Protocole | MCP (Model Context Protocol) -- sous-processus stdin/stdout ; HTTP « streamable » via la passerelle du demon hors machine |
| Identifiants | Proxy d'identifiants de `ohd` (jeton `ohs_` par groupe ; les vraies cles restent dans le trousseau ; SigV4 pour les profils AWS) |
| Fournisseurs | Anthropic Claude, AWS Bedrock, OpenAI, OpenRouter (configurable) |
| Workflows | 12 workflows declaratifs `oh/v1` (YAML, embarques via go:embed) |
| Agents | 20 agents avec architecture de skills hybride (Bucket A/B), dont l'agent d'entree generique `conductor` |
| Skills | 184 skills de protocole + 14 annexes (embarques via go:embed, livres par le paquet de session) |
| Plugin | Plugin oh pour opencode (TypeScript, embarque ; monde ferme et checkpoints) |

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
| Workflow | Local (servi par oh) | Etat du workflow, checkpoints, sorties typees |

## Decisions de conception cles

- **Zero dependance au runtime** -- binaire statique unique, pas de Node.js, pas de Python ; opencode V2 s'installe avec son propre outil ; un moteur de conteneurs n'est utile que pour le runtime conteneur, optionnel
- **Contenu embarque** -- agents/skills/permissions/workflows compiles dans le binaire via `go:embed`, assembles au lancement en un paquet de session immuable (`~/.oh/bundles/<hash>/`)
- **SQLite Go pur** -- `modernc.org/sqlite` evite CGO pour des builds vraiment statiques
- **Configuration TOML** -- lisible par l'humain, compatible git, supporte la cascade hierarchique
- **Trousseau systeme en priorite** -- secrets stockes dans le stockage securise natif, fichier chiffre en fallback uniquement
