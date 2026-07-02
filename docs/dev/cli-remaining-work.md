# Travail restant — CLI oh v2.0.0

> Backlog structuré pour atteindre une release production-ready.
> Mis à jour le 2 juillet 2026 — Post P0+P1.
>
> **P0 et P1 terminés.** Ce document décrit le travail P2 restant avant la release v2.0.0.

---

## P0 — ✅ DONE

> Commits : `4c91f3a`, `885abc9`, `46ec451`

- [x] Auto-download opencode (download.go, SHA256 verify, symlink, progress bar)
- [x] Tests & qualité (golangci-lint, 11 tests intégration, nil-safety MustApp/TryApp)
- [x] Distribution (GoReleaser snapshot validé, install.sh curl script, RELEASING.md)

---

## P1 — ✅ DONE

> Commits : `d8a0000`, `0ab3ec9`, `0569727`, `21e75a0`

- [x] Deploy MCP injection (phase transactionnelle, validation token, skip avec warning)
- [x] Matrice de compatibilité (//go:embed, semver check, doctor + start warning)
- [x] Session tracking (lecture DB opencode, Exec→Run, metrics/dashboard réels)
- [x] Plugin RTK (embed rtk.ts, install/remove/status, vérification version RTK)

---

## P2 — Avant release v2.0.0

### Bloc 1 : Parité fonctionnelle commandes (~3-4 jours)

Objectif : iso-fonctionnel avec le bash CLI pour toutes les commandes conservées.

| # | Commande | Ce qui manque | Effort |
|---|----------|---------------|--------|
| 1.1 | `start` | `--worktree` : créer un git worktree + lancer opencode dedans | 0.5j |
| 1.2 | `deploy` | `--check` (freshness — détecte si les agents/skills ont changé depuis le dernier deploy), `--diff` (preview les changements avant d'écrire) | 0.5j |
| 1.3 | `init` | Wizard enrichi : sélection MCP interactive, intégration tracker (bd), deploy automatique à la fin du wizard | 0.5j |
| 1.4 | `sync` | Multi-projet (itérer sur tous les projets actifs), `--dry-run` | 0.5j |
| 1.5 | `metrics` | `--period` (7d/30d/all), section AI savings (lecture stats context-mode + RTK) | 0.5j |
| 1.6 | `audit` | Ajouter 4 types d'audit : accessibility, ecodesign, observability, privacy (actuellement 3 : security, performance, architecture) | 0.25j |
| 1.7 | `project` | Sous-commandes `rename`, `move` (update path en DB), `configure` (persister model/provider par projet) | 0.5j |
| 1.8 | `config` | `unset` (supprimer une clé), `language` (changer la langue), per-project model/provider/agent overrides | 0.5j |
| 1.9 | `service` | Setup wizard interactif (configure les tokens MCP dans le keychain), affichage status enrichi, `remove` | 0.5j |
| 1.10 | `doctor` | Checks supplémentaires : `bd` (tracker), `fzf`, validation clés API (test connectivité rapide) | 0.25j |
| 1.11 | `worktree` | `cleanup` : supprime les worktrees dont la branche est mergée dans main | 0.25j |

**Sous-tâches transverses Bloc 1 :**
- Ajouter `ProjectStore.Update()` pour supporter rename/move/configure
- Enrichir `hub.toml` Config struct : `DefaultProvider`, `DefaultModel`, `ProjectOverrides`
- Créer helper `runAgentSession(agentName, prompt)` pour audit/review/debug (évite la duplication)

---

### Bloc 2 : i18n complète (~1.5 jours)

Objectif : toutes les chaînes user-facing passent par `i18n.T()` / `i18n.Tf()`.

| # | Tâche | Détail |
|---|-------|--------|
| 2.1 | Identifier les strings hardcodées | ~43 dans cmd/*.go, ~10 dans internal/tui/views/ |
| 2.2 | Définir les clés JSON | ~50-60 nouvelles clés, structurées par namespace (`cmd.start.launching`, `cmd.deploy.phase_mcp`, `tui.board.no_tickets`, etc.) |
| 2.3 | Migrer cmd/*.go | Remplacer chaque string FR par `i18n.T("clé")` ou `i18n.Tf("clé", args...)` |
| 2.4 | Migrer TUI views | Board (colonnes, refresh, quit) + Dashboard (labels panels) |
| 2.5 | Cobra Short/Long | Pattern : `Short: i18n.T("cmd.start.short")` — un appel à init déféré |
| 2.6 | Test parité en/fr | CI test qui vérifie que les deux locales JSON ont les mêmes clés |
| 2.7 | Compléter `en.json` | Traduction de toutes les clés FR → EN |

---

### Bloc 3 : TUI Polish (~0.5-1 jour)

| # | Tâche | Détail |
|---|-------|--------|
| 3.1 | Board — détection `bd` absent | `exec.LookPath("bd")` → message actionnable : "Installez bd pour afficher les tickets" |
| 3.2 | Board — mouse support | `tea.WithMouseCellMotion()`, gérer `tea.MouseMsg` (clic colonne, scroll) |
| 3.3 | Dashboard — empty state | Si 0 sessions : guide "Lancez `oh start` pour commencer" au lieu de panneaux vides |
| 3.4 | Guard terminal trop petit | Si < 80 cols ou < 20 lignes → message d'erreur au lieu d'un render cassé |
| 3.5 | Board — scroll vertical | Si plus de tickets que la hauteur disponible dans une colonne, permettre le défilement |

---

### Bloc 4 : Keychain Fallback (~1 jour)

| # | Tâche | Détail |
|---|-------|--------|
| 4.1 | Créer `internal/storage/filecrypt/store.go` | Implémente `domain.SecretStore` avec AES-256-GCM |
| 4.2 | Format de stockage | `~/.oh/secrets.enc` — JSON chiffré, salt en header |
| 4.3 | Dérivation de clé | Argon2id (passphrase utilisateur + salt aléatoire) |
| 4.4 | Détection auto | Dans `app.go` : tenter go-keyring, si erreur → fallback filecrypt + warning console |
| 4.5 | Prompt passphrase | `huh.Input` avec masquage au premier usage, garder en mémoire pour la session |
| 4.6 | Tests | Cycle encrypt/decrypt, passphrase incorrecte → erreur claire, fichier corrompu → erreur + recovery |

---

### Bloc 5 : Cleanup repo + Documentation (~1 jour)

> Ce bloc est le dernier — il prépare directement la release.

| # | Tâche | Détail |
|---|-------|--------|
| 5.1 | Supprimer `scripts/` | 62 fichiers shell (cmd-*.sh, lib/*.sh, adapters/) |
| 5.2 | Supprimer `oc.sh`, `ocp.sh`, `uninstall.sh` | Entry points bash CLI |
| 5.3 | Supprimer `tests/` | 82 fichiers .bats |
| 5.4 | Supprimer `servers/` | MCP servers TypeScript (remplacés par Go dans `cli/internal/mcp/`) |
| 5.5 | Supprimer `plugins/rtk/*.test.ts`, `vitest.config.ts` | Source test TS (rtk.ts embedded dans le binaire Go) |
| 5.6 | Nettoyer `.github/workflows/ci.yml` | Retirer jobs : shellcheck, bats, validate-agents, check-staleness, version-consistency. Garder uniquement `go-cli`. |
| 5.7 | Réécrire `README.md` | Installation (brew + curl), Quick start (oh init → oh start), Commandes principales, Architecture |
| 5.8 | Créer `MIGRATION.md` | Guide `oc` → `oh` : breaking changes, équivalences commandes, config migration hub.json→hub.toml, comment re-deploy |
| 5.9 | Archiver | `docs/legacy/README-bash.md` (pour référence historique) |
| 5.10 | Mettre à jour `opencode.json` | Vérifier que les mcpServers pointent vers `oh mcp serve <name>` |

---

## Features supprimées (ne pas porter)

| Feature bash | Décision | Justification |
|---|---|---|
| `uninstall` | Supprimé | Géré par `brew uninstall oh` |
| `upgrade` (hub self-update via git pull) | Supprimé | Géré par `brew upgrade oh` |
| `agent create/edit/add-skill/remove-skill` | Supprimé | Gestion manuelle des fichiers .md, pas de valeur dans le CLI |
| `skills install/remove/update` (ctx7) | Supprimé | Gestion manuelle des fichiers skill |
| Session state machine | Supprimé | Opencode gère nativement les reprises de session |
| Session title generation | Supprimé | Opencode génère automatiquement des titres intelligents |
| Node.js installer | Supprimé | Go CLI n'a aucune dépendance Node.js |
| `update` (adapter + bd + skills) | Supprimé | `brew upgrade` pour oh, `oh upgrade opencode` pour opencode, `bd` se gère seul |

---

## Features reportées en P3 (post-release)

| Feature | Justification du report |
|---|---|
| Context cache (freshness check SHA-256) | 90% couvert par context-mode + CLAUDE.md. Le 10% restant (warning si fichier structurel modifié) peut être ajouté post-release |
| Dependency graph (analyse imports TS/JS) | Pertinence à étudier — feature avancée, usage incertain, probablement mieux dans un MCP dédié |
| AI savings reporting complet | Dépend de l'évolution des APIs context-mode et RTK. Implémentation basique dans `oh metrics` suffit pour v2.0 |
| `oh beads` enrichi (beyond exec `bd`) | L'intégration complète est complexe (sync, tracker setup). L'exec delegation vers `bd` suffit pour v2.0 |
| `oh yield` enrichi | La corrélation session-commit est complexe. Le ratio basique suffit pour v2.0 |

---

## Estimation totale

| Bloc | Effort estimé |
|------|---------------|
| Bloc 1 — Parité fonctionnelle | 3-4 jours |
| Bloc 2 — i18n | 1.5 jours |
| Bloc 3 — TUI Polish | 0.5-1 jour |
| Bloc 4 — Keychain Fallback | 1 jour |
| Bloc 5 — Cleanup + Documentation | 1 jour |
| **Total P2** | **~7-8.5 jours** |

---

## Ordre d'exécution

```
1. Bloc 1 — Parité fonctionnelle    [critique — iso bash CLI]
2. Bloc 2 — i18n                     [qualité — multilingue]
3. Bloc 3 — TUI Polish              [UX — rapide]
4. Bloc 4 — Keychain Fallback       [robustesse — edge cases]
5. Bloc 5 — Cleanup + Documentation [release gate — en dernier]
```

Après le Bloc 5 :
- Créer le repo `datichb/homebrew-openhub` (public, vide avec Formula/)
- Tag `v2.0.0` + `goreleaser release --clean`
- Vérifier Homebrew : `brew install datichb/openhub/oh`
- Annoncer la migration

---

## Métriques actuelles (post P0+P1)

| Métrique | Valeur |
|----------|--------|
| Tests | 103 |
| Packages testés | 20 |
| Linter | golangci-lint clean (11 linters) |
| Binaire | ~5.5 MB (stripped, CGO_ENABLED=0) |
| Commandes | 30 |
| Sous-commandes | 15 |
| Plateformes | darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 |

---

*Mis à jour le 2 juillet 2026 — Post P0+P1, session planification P2*
