---
updated: 2026-10-15
confidence: confirmed
agents: [onboarder, documentarian]
---

> [Read in English](stack.en.md)

# Stack technologique

Sources de la page — `CONFIRMÉ` · onboarder · 2026-10-06 · cli/go.mod, cli/.goreleaser.yml, cli/Makefile, docs/architecture/overview.fr.md

## Runtime

| Composant | Technologie | Version |
|-----------|------------|---------|
| Langage | Go | 1.26.4 |
| Binaire | Binaire statique unique | `CGO_ENABLED=0` |
| Framework TUI | [tview](https://github.com/rivo/tview) | base tcell |
| Invites en ligne (hors TUI) | [huh](https://github.com/charmbracelet/huh) | -- |
| Framework CLI | [Cobra](https://github.com/spf13/cobra) | v1.10+ |
| Configuration | [go-toml](https://github.com/pelletier/go-toml) v2 (lecture stricte de `hub.toml`) | -- |
| Base de données | SQLite (via modernc.org/sqlite) | Go pur, pas de CGO |
| Secrets | Trousseau système ([go-keyring](https://github.com/zalando/go-keyring)) + fallback AES-256-GCM | -- |
| Appels système | [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) | UID du pair sur le socket du démon, verrous de fichiers (Windows) |
| Moteur de conteneurs (optionnel) | Colima, Podman ou Docker CLI (CLI OCI, pas d'API Docker) | Runtime conteneur uniquement |
| Exécution distante (optionnelle) | GitLab CI (projet `oh-runner`, Kaniko ou Docker-in-Docker) | Runtime distant uniquement |

## Build et release

| Outil | Usage |
|-------|-------|
| [GoReleaser](https://goreleaser.com/) v2 | Compilation croisee, signature d'artefacts, GitHub Release |
| [Cosign](https://docs.sigstore.dev/cosign/) | Signature sans clé (Sigstore OIDC) |
| [golangci-lint](https://golangci-lint.run/) v2.1 | Linting Go (bodyclose, gocritic, misspell, nilerr) |
| [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) | Scan de vulnérabilités des dépendances Go |
| GitHub Actions | CI (matrice 3 OS) + automatisation release |
| Dependabot | Mises à jour hebdomadaires des dépendances |

## Intégration IA

| Composant | Technologie |
|-----------|-----------|
| Runtime IA | [OpenCode](https://opencode.ai/) V2 (>= 2.0.0, `opencode serve` ; V1 n'est plus pris en charge) |
| Protocole | MCP (Model Context Protocol) -- sous-processus stdin/stdout ; HTTP « streamable » via la passerelle du démon hors machine |
| Identifiants | Proxy d'identifiants de `ohd` (jeton `ohs_` par groupe ; les vraies clés restent dans le trousseau ; SigV4 pour les profils AWS) |
| Fournisseurs | Anthropic Claude, AWS Bedrock, OpenAI, OpenRouter (configurable) |
| Workflows | 24 workflows déclaratifs `oh/v1` et les alias `feature`, `cadrage` (YAML, embarqués via go:embed) |
| Agents | 18 agents avec architecture de skills hybride (Bucket A/B), dont l'agent d'entrée générique `conductor` |
| Skills | 181 skills de protocole + 11 annexes (embarqués via go:embed, livrés par le paquet de session) |
| Plugin | Plugin oh pour opencode (TypeScript, embarqué ; monde fermé et checkpoints) |

Nombres relevés dans `workflows/`, `agents/` et `skills/` — `CONFIRMÉ` · documentarian · 2026-10-15 · workflows/spec.yaml

## Intégrations externes (serveurs MCP)

| Serveur | Protocole | Capacités |
|---------|----------|-----------|
| Figma | REST API | Lecture mockups, tokens, composants |
| GitLab | REST API + GraphQL | Issues, MRs, discussions, labels |
| GitHub | REST API | Issues, PRs, check runs |
| Jira | REST API | Issues, transitions, commentaires |
| Linear | GraphQL | Issues, états, assignés |
| Google Slides | REST API | Présentations, slides (lecture seule) |
| Team | Local (git-backed) | Claims, wiki, événements, notifications |
| Workflow | Local (servi par oh) | État du workflow, checkpoints, sorties typées |

## Décisions de conception clés

- **Zéro dépendance au runtime** -- binaire statique unique, pas de Node.js, pas de Python ; opencode V2 s'installe avec son propre outil ; un moteur de conteneurs n'est utile que pour le runtime conteneur, optionnel
- **Contenu embarqué** -- agents/skills/permissions/workflows compilés dans le binaire via `go:embed`, assemblés au lancement en un paquet de session immuable (`~/.oh/bundles/<hash>/`)
- **SQLite Go pur** -- `modernc.org/sqlite` évite CGO pour des builds vraiment statiques
- **Configuration TOML** -- lisible par l'humain, compatible git, supporte la cascade hiérarchique
- **Trousseau système en priorité** -- secrets stockés dans le stockage sécurisé natif, fichier chiffré en fallback uniquement
