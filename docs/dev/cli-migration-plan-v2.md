# Plan de migration CLI v2 — oh (Go + Charm)

> Révision de l'analyse initiale (`cli-migration-analysis.md`) intégrant les
> nouvelles contraintes : TUI avancé (niveau lazygit/k9s), distribution Homebrew,
> binaire monolithique embarquant CLI + MCP servers + plugins.
>
> Date : 1er juillet 2026

---

## Résumé des décisions

| Sujet | Décision |
|-------|----------|
| Langage | Go |
| Nom du binaire | `oh` (remplacement de `oc`, collision OpenShift évitée) |
| TUI framework | Bubbletea + Lipgloss + Bubbles + Huh (écosystème Charm) |
| CLI framework | Cobra + Viper |
| Distribution | Homebrew tap via GoReleaser, binaire statique ~15 Mo |
| Données projets | SQLite (via modernc.org/sqlite, pur Go) |
| Configuration | TOML (`hub.toml`) |
| Secrets | OS Keychain (go-keyring) multi-OS + fallback chiffré |
| i18n | JSON par locale (`locales/en.json`, `locales/fr.json`) |
| Tests | Suite Go complète (`go test` + testify) |
| MCP servers | Réécrits en Go, intégrés au binaire (`oh mcp serve <name>`) |
| Plugin RTK | Réécrit en Go, intégré nativement au binaire |
| Plugin system futur | Subprocess JSON stdio (pattern Terraform/kubectl) |
| Dépendance opencode | Auto-download avec version pinée (pattern rustup/volta) |
| Rétro-compatibilité | Aucune — breaking change, re-deploy obligatoire |
| Repo | Monorepo — code Go dans `cli/` remplace `scripts/` |

---

## Justification du choix Go

### Contraintes déterminantes

1. **TUI avancé (niveau lazygit/k9s)** — L'écosystème Charm (Bubbletea) est
   la référence absolue pour les TUI Go. lazygit, k9s, gh-dash, soft-serve
   sont tous construits dessus.

2. **Distribution Homebrew** — Go produit un binaire statique ~15 Mo. Formula
   triviale, zéro dépendance runtime. GoReleaser génère automatiquement la
   formula et les builds multi-arch.

3. **Taille binaire** — 15 Mo (Go) vs 50-90 Mo (Bun compile) vs 80 Mo
   (Python/PyInstaller). Go gagne significativement.

4. **Binaire monolithique** — CLI + MCP servers + plugins dans un seul
   artefact. Go excelle pour ce pattern (compilation statique, pas de runtime
   externe).

### Langages écartés

| Langage | Raison d'exclusion |
|---------|-------------------|
| Python | Distribution Homebrew cauchemardesque (resource stanza par dep PyPI), binaire lourd (PyInstaller), startup lent (~80ms) |
| TypeScript/Bun | Binaire 50-90 Mo (embarque le runtime Bun), cross-compile limité, TUI moins mature que Charm |
| Rust | Courbe d'apprentissage trop élevée pour la vélocité requise, compilation lente, gain marginal (5 Mo vs 15 Mo) |

---

## Architecture

### Structure du binaire

```
oh (single binary ~15 Mo)
├── cmd/                  ← Cobra commands
│   ├── root.go           ← oh (racine, help, version)
│   ├── start.go          ← oh start
│   ├── init.go           ← oh init
│   ├── config.go         ← oh config [get|set|list|edit]
│   ├── project.go        ← oh project [register|rename|move|list|remove]
│   ├── board.go          ← oh board [--watch]
│   ├── dashboard.go      ← oh dashboard
│   ├── deploy.go         ← oh deploy
│   ├── mcp.go            ← oh mcp [serve|list]
│   └── ...               ← 30 commandes au total
├── internal/
│   ├── config/           ← Lecture/écriture hub.toml (Viper)
│   ├── project/          ← SQLite CRUD projets
│   ├── keychain/         ← Abstraction OS keychain + fallback fichier chiffré
│   ├── i18n/             ← Chargement locales JSON, T()
│   ├── tui/              ← Composants Bubbletea partagés
│   │   ├── picker/       ← Full-screen picker (agents, projets, MCP)
│   │   ├── board/        ← Kanban TUI (live update, mouse)
│   │   ├── dashboard/    ← Multi-panneaux
│   │   └── common/       ← Styles, spinner, progress, layout helpers
│   ├── opencode/         ← Gestion du binaire opencode (download, version, exec)
│   ├── prompt/           ← Prompt builder (détection stack, contexte)
│   ├── metrics/          ← SQLite métriques/sessions
│   ├── deploy/           ← Déploiement transactionnel (snapshot/commit/rollback)
│   ├── plugin/           ← Interface plugin + RTK natif + subprocess protocol
│   └── mcp/              ← Serveurs MCP intégrés
│       ├── protocol/     ← MCP stdio JSON-RPC
│       ├── figma/        ← Client Figma API + tools MCP
│       ├── gitlab/       ← Client GitLab API + tools MCP
│       └── gslides/      ← Client Google Slides API + tools MCP
├── locales/
│   ├── en.json
│   └── fr.json
├── go.mod
├── go.sum
├── main.go
├── Makefile
└── .goreleaser.yml
```

### Structure dans le monorepo

```
opencode-hub/
├── cli/                  ← NOUVEAU — tout le code Go
│   ├── cmd/
│   ├── internal/
│   ├── locales/
│   ├── main.go
│   ├── go.mod
│   └── ...
├── scripts/              ← SUPPRIMÉ en fin de migration
├── servers/              ← SUPPRIMÉ (migré dans cli/internal/mcp/)
├── plugins/              ← SUPPRIMÉ (RTK migré dans cli/internal/plugin/)
├── agents/               ← Conservé (fichiers .md, déployés par oh deploy)
├── skills/               ← Conservé (fichiers .md, déployés par oh deploy)
├── .opencode/            ← Conservé
├── opencode.json         ← Conservé (format upstream)
├── docs/
├── tests/                ← SUPPRIMÉ (remplacé par go test dans cli/)
└── ...
```

---

## Gestion de la dépendance opencode

### Modèle : auto-download avec version pinée

```toml
# hub.toml
[opencode]
version = "1.17.2"            # version pinée, mise à jour par oh upgrade
channel = "stable"            # stable | beta
auto_update = false           # check automatique au oh start
install_dir = "~/.oh/bin"     # stockage des binaires opencode
```

### Fonctionnement

1. `oh start` vérifie que `~/.oh/bin/opencode-1.17.2` existe
2. Si absent → téléchargement depuis les releases GitHub opencode
3. Vérification checksum SHA256
4. `exec ~/.oh/bin/opencode-1.17.2 [args]`
5. `oh upgrade opencode` → met à jour la version pinée + télécharge

### Compatibilité

- `oh doctor` vérifie la matrice de compatibilité oh ↔ opencode
- Un fichier `compatibility.json` embedded dans le binaire oh (via `//go:embed`)
  déclare les versions opencode supportées
- Avertissement si version opencode non testée, erreur si incompatible

---

## Plugin system

### Architecture hybride

| Type | Mécanisme | Usage |
|------|-----------|-------|
| **Natif** (compiled-in) | Interface Go `Plugin` implémentée dans le binaire | RTK, plugins core |
| **Externe** (subprocess) | Binaire/script lancé en subprocess, protocole JSON stdio | Plugins communautaires futurs |

### Interface Plugin (interne)

```go
type Plugin interface {
    Name() string
    Version() string
    Init(ctx context.Context, cfg PluginConfig) error
    // Hooks
    OnBeforeCommand(cmd string, args []string) (string, []string, error)
    OnAfterCommand(cmd string, result CommandResult) error
    OnSessionStart(session Session) error
    OnSessionEnd(session Session, stats SessionStats) error
}
```

### Protocole subprocess (futurs plugins externes)

- Communication : stdin/stdout JSON-RPC 2.0
- Lifecycle : `oh` lance le plugin subprocess au besoin, le kill après usage
- Discovery : `~/.oh/plugins/` contient les binaires de plugins
- Manifest : chaque plugin expose `<binary> --oh-plugin-info` → JSON metadata
- Handshake : échange de version du protocole au démarrage

---

## Migration des formats de données

| Source | Destination | Notes |
|--------|-------------|-------|
| `projects.md` | SQLite `projects.db` | Schema: id, name, path, language, tracker, labels, agents, mcp, status, created_at, updated_at |
| `hub.json` | `hub.toml` | Config hub + version opencode pinée |
| `api-keys.local.md` | OS Keychain | go-keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service). Fallback: fichier chiffré AES-256-GCM |
| `i18n.sh` (1793 LoC) | `locales/{en,fr}.json` | Clés structurées par namespace (`cmd.start.*`, `tui.picker.*`, etc.) |
| `opencode.json` | Inchangé | Format dicté par opencode upstream |

---

## Stack technique

| Besoin | Librairie | Justification |
|--------|-----------|---------------|
| CLI | `github.com/spf13/cobra` | Standard Go, completion shell native, help auto |
| Config | `github.com/spf13/viper` | TOML/env/defaults, intégration Cobra |
| TUI engine | `github.com/charmbracelet/bubbletea` | Architecture Elm, composable, référence |
| TUI styling | `github.com/charmbracelet/lipgloss` | CSS-like, tables, layout adaptatif |
| TUI widgets | `github.com/charmbracelet/bubbles` | Spinner, viewport, list, textinput, table, paginator, progress |
| TUI forms | `github.com/charmbracelet/huh` | Wizards interactifs (init, config) |
| Logging | `github.com/charmbracelet/log` | Structuré, coloré, intégré Charm |
| Markdown | `github.com/charmbracelet/glamour` | Render .md en terminal |
| SQLite | `modernc.org/sqlite` | Pur Go (pas de CGO), cross-compile trivial |
| Keychain | `github.com/zalando/go-keyring` | macOS/Windows/Linux natif |
| i18n | `github.com/nicksnyder/go-i18n/v2` | Traductions JSON, pluralization |
| HTTP | `net/http` stdlib | Appels API (Figma, GitLab, GSlides) |
| MCP | `github.com/mark3labs/mcp-go` | SDK MCP protocol |
| File lock | `github.com/gofrs/flock` | Cross-platform |
| Release | `goreleaser/goreleaser` | Multi-arch + Homebrew tap auto |
| Tests | `testing` + `github.com/stretchr/testify` | Assertions, suites |

> Note : `modernc.org/sqlite` est préférable à `go-sqlite3` car c'est du pur Go
> (pas de CGO), ce qui simplifie le cross-compile et la distribution.

---

## Phasage

### Phase 1 — Fondations (semaine 1-2)

**Objectif** : Infrastructure compilable, première commande fonctionnelle.

- Initialiser `cli/` : `go mod init`, structure de packages
- Cobra root command + `oh version`, `oh help`
- Modules : config (TOML), project (SQLite schema), keychain, i18n, tui/common
- Makefile + `.goreleaser.yml`
- CI : `go test`, `go vet`, `golangci-lint`

**Livrable** : `oh version` fonctionne, CI verte.

### Phase 2 — Registre et configuration (semaine 3-4)

**Objectif** : Gestion complète des projets et de la configuration.

- `oh init` (wizard Huh)
- `oh config` (get/set/list/edit)
- `oh project` (register, rename, move, list, remove)
- `oh status`
- `oh doctor`
- `oh remove`
- Picker TUI full-screen (Bubbletea, alternate screen, navigation clavier)

**Livrable** : gestion complète des projets.

### Phase 3 — Session et lancement (semaine 5-6)

**Objectif** : Pouvoir lancer une session opencode depuis `oh`.

- `oh start` (préparation contexte + exec opencode)
- `oh quick` (lancement rapide, sélection projet)
- `oh worktree` (gestion git worktrees)
- Modules : opencode (auto-download, version check, exec), prompt-builder, session-state

**Livrable** : on peut lancer opencode via `oh start`.

### Phase 4 — Déploiement et orchestration (semaine 7-8)

**Objectif** : Déploiement transactionnel, agents, skills, plugins.

- `oh deploy` (avec rollback atomique : snapshot → deploy → commit/rollback)
- `oh sync`
- `oh agent` (list, enable, disable, picker TUI)
- `oh skills`
- `oh plugin`
- `oh install` / `oh uninstall`

**Livrable** : deploy/sync fonctionnels avec rollback.

### Phase 5 — TUI avancé (semaine 9-10)

**Objectif** : Dashboard, board, métriques — full TUI interactif.

- `oh board` (kanban Bubbletea full-screen, live update, mouse support)
- `oh dashboard` (multi-panneaux : budget, sessions, projets)
- `oh metrics` (visualisation, sparklines, tables)
- `oh optimize`
- `oh yield` (sessions ↔ commits)

**Livrable** : TUI interactif complet.

### Phase 6 — MCP servers + Plugin (semaine 11-12)

**Objectif** : Intégrer les 3 MCP servers et le plugin RTK dans le binaire.

- `oh mcp serve figma` — réécriture Figma MCP en Go
- `oh mcp serve gitlab` — réécriture GitLab MCP en Go
- `oh mcp serve gslides` — réécriture Google Slides MCP en Go
- Plugin RTK intégré nativement
- `oh mcp list`

**Livrable** : MCP fonctionnels depuis le binaire unique.

### Phase 7 — Finalisation (semaine 13-14)

**Objectif** : Feature parity complète, distribution.

- Commandes restantes : `oh audit`, `oh review`, `oh debug`, `oh conventions`, `oh beads`, `oh service`, `oh upgrade`, `oh update`
- Shell completion (zsh/bash/fish) via Cobra
- Homebrew tap + GoReleaser config finale
- Documentation utilisateur
- Suppression de `scripts/`, `servers/`, `plugins/rtk/`, `tests/`, `oc.sh`, `ocp.sh`, `install.sh`

**Livrable** : v2.0.0 released.

---

## Risques et mitigations

| Risque | Impact | Mitigation |
|--------|--------|-----------|
| `modernc.org/sqlite` perf pur Go | +2-5ms startup | Acceptable, pas de CGO = cross-compile trivial |
| MCP SDK Go (`mcp-go`) moins mature | Bugs protocole | Fallback implémentation custom stdio JSON-RPC |
| Bubbletea complexité board/dashboard | Temps dev accru | Exemples abondants, patterns composables bien documentés |
| `go-keyring` Linux sans D-Bus | Pas de keychain | Fallback fichier chiffré AES-256-GCM + passphrase |
| Taille binaire > 15 Mo avec MCP+SQLite | Distribution | Acceptable, UPX compressable si nécessaire |
| opencode breaking change upstream | Incompatibilité | Version pinée + matrice compatibilité + `oh doctor` |

---

## Distribution

### GoReleaser

- Targets : `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`
- Homebrew tap : `datichb/homebrew-tap`
- Formula auto-générée par GoReleaser
- Checksums SHA256 + signature

### Installation utilisateur

```bash
# Homebrew (recommandé)
brew install datichb/tap/openhub

# Binaire direct
curl -sSfL https://github.com/datichb/openhub/releases/latest/download/oh_$(uname -s)_$(uname -m).tar.gz | tar xz
sudo mv oh /usr/local/bin/

# Depuis les sources
cd cli && go build -o oh . && mv oh /usr/local/bin/
```

---

## Avancement

> Mis à jour le 1er juillet 2026 après complétion des 7 phases.

| Phase | Statut | Livrables |
|-------|--------|-----------|
| Phase 1 — Fondations | DONE | go mod, Cobra root, config TOML, i18n JSON, project SQLite, keychain, TUI styles, Makefile, GoReleaser, CI |
| Phase 2 — Registre et config | DONE | `oh init`, `oh config`, `oh project`, `oh status`, `oh doctor`, picker TUI full-screen |
| Phase 3 — Session et lancement | DONE | `oh start`, `oh quick`, `oh worktree`, modules opencode (exec/find), prompt (stack detect) |
| Phase 4 — Déploiement | DONE | `oh deploy` (transactionnel avec rollback), `oh sync`, `oh agent`, `oh skills`, `oh plugin` |
| Phase 5 — TUI avancé | DONE | `oh board` (kanban Bubbletea), `oh dashboard`, `oh metrics`, `oh optimize`, `oh yield` |
| Phase 6 — MCP servers | DONE | `oh mcp serve` (figma/gitlab/gslides), protocole JSON-RPC stdio |
| Phase 7 — Finalisation | DONE | `oh audit/review/debug/conventions/beads/service/upgrade`, shell completion |
| Post-Phase 7 — Production readiness | À FAIRE | Voir `docs/dev/cli-remaining-work.md` |

---

## Métriques du prototype

| Métrique | Valeur |
|----------|--------|
| Commandes | 30 |
| Binaire (darwin/arm64, stripped) | 12.8 Mo |
| Tests | 50 |
| LoC Go (hors go.sum) | ~4 500 |
| MCP servers intégrés | 3 |
| Startup estimé | <5ms |
| Compilation | ~3s (incrémental <1s) |

---

*Document produit le 1er juillet 2026 — Révision de cli-migration-analysis.md*
*Mis à jour le 1er juillet 2026 — Ajout avancement + métriques post-Phase 7*
