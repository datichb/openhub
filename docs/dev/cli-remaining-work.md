# Travail restant — CLI oh v2.0.0

> Backlog structuré pour atteindre une release production-ready.
> Mis à jour le 6 juillet 2026 — **Migration 100% terminée.** Tous les blocs (P0, P1, P2 Blocs 1-5, Audit v2, Phases A-F) sont DONE.
>
> **Prêt pour release v2.0.0.**

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

## P2 — Bloc 1 : Parité fonctionnelle — ✅ DONE

> 11 commandes complétées + 3 tâches transverses.

| # | Commande | Livrable |
|---|----------|----------|
| 1.1 | `start --worktree` | Nouveau package `internal/worktree/` (Slug, SiblingPath, ResolveOrCreate, List, IsMerged, CleanupMerged), flag `-w`, prompt interactif, auto-cleanup, deploy dans worktree |
| 1.2 | `deploy --check/--diff` | Nouveau module `internal/deploy/diff.go` (ComputeDiff, FormatDiffReport), comparaison SHA-256 hub↔projet, exit code 1 si stale |
| 1.3 | `init` enrichi | Wizard MCP multi-select, tracker bd init, auto-deploy agents/skills, git excludes, summary |
| 1.4 | `sync --all/--dry-run` | Multi-projet, dry-run avec diff, sync complet (agents+skills+config+MCP) |
| 1.5 | `metrics --period` | Filtrage 7d/30d/all, section AI Savings (cache ratio, estimations), per-project cache % |
| 1.6 | `audit` +4 types | accessibility, ecodesign, observability, privacy — validation type, prompts enrichis |
| 1.7 | `project rename/move/configure` | 3 sous-commandes, wizard interactif, ProjectStore.Update() |
| 1.8 | `config unset/language` | Suppression de clé, changement de langue avec validation |
| 1.9 | `service setup/status/remove` | Wizard token + keychain, status enrichi (env/keychain/missing), remove avec delete keychain |
| 1.10 | `doctor` enrichi | Checks optionnels bd/fzf, validation clés API (env + keychain), messages guidance |
| 1.11 | `worktree cleanup` | Sous-commande + `--base` flag, détection via `git branch --merged` |

**Tâches transverses livrées :**
- `ProjectStore.Update()` déjà dans l'interface domain + implémenté SQLite
- `hub.toml` enrichi avec `WorktreeConfig` (auto_cleanup, base_branch)
- Helper `runAgentSession()` factorisé pour audit/review/debug

---

## P2 — Audit qualité (initial) — ✅ DONE

> 16 findings corrigés sur 5 axes.

| Sévérité | Count | Corrections clés |
|----------|-------|------------------|
| CRITICAL | 1 | Session ID jamais généré → `uuid.New().String()` |
| HIGH | 3 | URL injection MCP (PathEscape + url.Values), branch name validation (--/.. rejetés), beads proxy (DisableFlagParsing) |
| MEDIUM | 7 | fileHash streaming, erreurs silencieuses corrigées, config.Reset thread-safe, isSubPath idiomatique, board bd detection, findHubDir guidance, MCP graceful shutdown |
| LOW | 5 | Nil pointer guards, HTTP timeout 30s, io.LimitReader 50MB, ProjectStatus validation |

**Fichiers impactés :** 16 fichiers modifiés (cmd/, internal/config, internal/deploy, internal/worktree, internal/mcp/)

---

## P2 — Bloc 2 : i18n (initial) — ✅ DONE

> 272 clés fr/en, hook dynamique Cobra, 12 fichiers cmd migrés.

---

## P2 — Bloc 3 : TUI Polish — ✅ DONE

> Board enrichi (scroll, mouse, détection bd), Dashboard empty state, guard terminal.

---

## P2 — Bloc 4 : Keychain Fallback — ✅ DONE

> AES-256-GCM + Argon2id, stratégie A+B, 14 tests.

| # | Tâche | Livrable |
|---|-------|----------|
| 4.1 | `internal/storage/filecrypt/store.go` | Implémente `domain.SecretStore` — AES-256-GCM, atomic write (tmp+rename), permissions 0600 |
| 4.2 | Format de stockage | `~/.oh/secrets.enc` — header binaire OHSF v1 + salt + nonce + ciphertext |
| 4.3 | Dérivation de clé | Argon2id (t=3, memory=64MB, threads=4, keyLen=32) — conforme OWASP |
| 4.4 | Détection auto (`resolveSecretStore`) | Probe keychain via `Get` (lecture seule, pas de prompt macOS), fallback filecrypt |
| 4.5 | Stratégie passphrase (A+B) | 1. Terminal → prompt huh (création + confirmation, min 8 chars) ; 2. `OH_PASSPHRASE` env var ; 3. Secrets=nil + slog.Warn |
| 4.6 | Tests | 14 tests (round-trip, wrong passphrase, corruption, delete, list, empty store, atomic write, concurrency, env var, overwrite, verify) |

**Clés i18n ajoutées :** `secrets.fallback.*` (8 clés fr/en)

---

## Audit v2 + Phases d'amélioration — ✅ DONE

> Audit global (33 findings) + 6 phases de remédiation. Réalisé le 3 juillet 2026.

### Phase A — Bugs critiques (5 corrections)

| # | Fix | Fichier |
|---|-----|---------|
| A.1 | Supprimé `updateCmd` (nil panic sur `upgradeCmd.RunE`) | `cmd/misc.go` |
| A.2 | Fix extraction tar — condition `&&` → checks séparés + `filepath.Base` | `internal/opencode/download.go` |
| A.3 | Ajouté `recover()` top-level dans `Execute()` | `cmd/root.go` |
| A.4 | Fix `generateProjectID` slug vide → fallback "project" + trim dashes | `cmd/project_add.go` |
| A.5 | Pre-check `exec.LookPath("bd")` avec message actionnable | `cmd/misc.go` |

### Phase B — Sécurité (6 corrections)

| # | Fix | Fichier |
|---|-----|---------|
| B.1 | Checksum SHA256 **obligatoire** (refus strict si absent) | `internal/opencode/download.go` |
| B.2 | Validation `GITLAB_URL` (https only + bloquer IPs privées/loopback/link-local) | `internal/mcp/gitlab/server.go` |
| B.3 | Argon2id `time=1` → `time=3` (OWASP) | `internal/storage/filecrypt/store.go` |
| B.4 | Minimum passphrase 8 chars + clé i18n | `cmd/root.go` |
| B.5 | Permissions `~/.oh/` → 0700, `hub.toml` → 0600 | `sqlite/store.go`, `cmd/init.go` |
| B.6 | Ajouter `--` dans git worktree remove + flags avant path | `internal/worktree/worktree.go` |

### Phase D — Architecture (5 améliorations)

| # | Amélioration | Impact |
|---|-------------|--------|
| D.1 | `context.Context` propagé | 14 interfaces, 18 implémentations, ~30 call sites — cancellation/timeout ready |
| D.2 | Deploy plan builder extrait | `buildDeployPlan()` — 4 sites dupliqués → 1 helper |
| D.3 | Semver dedup → `internal/semver/` | Package partagé, 2 implémentations locales supprimées |
| D.5 | Schema versioning SQLite | Table `schema_migrations`, migrations numérotées, upgrade incrémental |
| D.6 | `log/slog` + `--verbose` flag | Logger structuré, 3 warnings migrés, base observabilité |

### Phase C — i18n complet (280 → 455 clés)

| # | Tâche | Résultat |
|---|-------|----------|
| C.1 | Cobra Short/Long descriptions | Toutes les commandes couvertes via `localizeCommands()` |
| C.2 | Table headers | 7 headers migrés vers `i18n.T()` |
| C.3 | Labels huh (options, titres, descriptions) | ~40 strings migrées |
| C.4 | Messages stdout/stderr (status, progress, labels) | ~60 strings migrées |
| C.5 | Erreurs user-facing (`fmt.Errorf`) | ~25 strings migrées |
| C.6 | Flag descriptions | `localizeCommands()` étendu pour localiser les flags via `pflag.VisitAll`, 37 clés ajoutées |
| C.7 | Parity test enrichi | `TestFormatVerbParity` — vérifie que les format verbs (%s, %d) matchent entre fr/en |

### Phase E — UX polish

| # | Amélioration | Résultat |
|---|-------------|----------|
| E.1 | `--json` output | 7 commandes list (project, agent, skills, worktree, mcp, config, status) |
| E.2 | Shell completion dynamique | `--project` (project IDs), `mcp serve` (server names), `plugin install`, `config get/unset` |
| E.3 | Confirmations destructives | `worktree cleanup`, `plugin remove`, `service remove` — `--force` bypass |
| E.4 | Provider configurable | `hub.toml` → `opencode.default_provider`, résolution : flag > config > "bedrock" |

### Phase F — Tests (+48 tests)

| # | Package | Tests ajoutés |
|---|---------|---------------|
| F.1 | `internal/mcp/protocol/` | 10 tests (JSON-RPC dispatch, initialize, errors, notifications) |
| F.2 | `internal/app/` | 8 tests (New, With*, nil safety, IO streams) |
| F.3 | `internal/mcp/gitlab/` | 12 tests (URL validation, private IPs, loopback, link-local) |
| F.4 | `cmd/` (helpers) | 11 tests (generateProjectID, buildDeployPlan, cmdI18nKey) |

---

## P2 — Bloc 5 : Cleanup repo + Documentation — ✅ DONE

> Réalisé le 6 juillet 2026.

| # | Tâche | Détail | Statut |
|---|-------|--------|--------|
| 5.1 | Supprimer `scripts/` | 62 fichiers shell (cmd-*.sh, lib/*.sh, adapters/) | ✅ |
| 5.2 | Supprimer `oc.sh`, `ocp.sh`, `uninstall.sh` | Entry points bash CLI | ✅ |
| 5.3 | Supprimer `tests/` | 82 fichiers .bats | ✅ |
| 5.4 | Supprimer `servers/` | MCP servers TypeScript (remplacés par Go dans `cli/internal/mcp/`) | ✅ |
| 5.5 | Supprimer `plugins/rtk/` | Tests TS, node_modules, vitest (rtk.ts embedded dans `cli/internal/plugin/`) | ✅ |
| 5.6 | Nettoyer `.github/workflows/ci.yml` | Retiré jobs legacy (shellcheck, bats, validate-agents, check-staleness, version-consistency). Ne reste que `go-cli`. | ✅ |
| 5.7 | Vérifier `README.md` | Déjà réécrit pour le CLI Go — aucune référence legacy restante | ✅ |
| 5.8 | Vérifier `MIGRATION.md` | Déjà créé (commit 3f2915c2) — cohérent avec le CLI actuel | ✅ |
| 5.9 | Archiver | Non — git history suffit comme référence | ✅ (skip) |
| 5.10 | `opencode.json` | Pas de mcpServers au hub level — déployé par `oh deploy` dans chaque projet | ✅ (N/A) |
| 5.11 | `.goreleaser.yml` | `depends_on "datichb/tap/bd"` déjà présent | ✅ (déjà fait) |

**Suppléments réalisés dans cette session :**
- Enrichissement du récap `oh start` (2 blocs lipgloss : Projet + Configuration)
- Ajout confirmation interactive avant lancement (`huh.NewConfirm()`)
- Flag `--yes`/`-y` pour bypass la confirmation
- Nouveau helper `opencode.ReadProjectConfig()` (model, plugins, compaction)
- Nouveau helper `worktree.CurrentBranch()` (branche git courante)
- Suppression de `config/` (hub.json.example obsolète)

---

## Features supprimées (ne pas porter)

| Feature bash | Décision | Justification |
|---|---|---|
| `uninstall` | Supprimé | Géré par `brew uninstall oh` |
| `upgrade` (hub self-update via git pull) | Supprimé | Géré par `brew upgrade oh` |
| `update` (adapter + bd + skills) | Supprimé | `brew upgrade` pour oh, `oh upgrade opencode` pour opencode, `bd` se gère seul |
| `agent create/edit/add-skill/remove-skill` | Supprimé | Gestion manuelle des fichiers .md, pas de valeur dans le CLI |
| `skills install/remove/update` (ctx7) | Supprimé | Gestion manuelle des fichiers skill |
| Session state machine | Supprimé | Opencode gère nativement les reprises de session |
| Session title generation | Supprimé | Opencode génère automatiquement des titres intelligents |
| Node.js installer | Supprimé | Go CLI n'a aucune dépendance Node.js |

---

## Features reportées en P3 (post-release)

| Feature | Justification du report |
|---|---|
| Context cache (freshness check SHA-256) | 90% couvert par context-mode + CLAUDE.md. Le 10% restant peut être ajouté post-release |
| Dependency graph (analyse imports TS/JS) | Pertinence à étudier — probablement mieux dans un MCP dédié |
| AI savings reporting complet | Dépend de l'évolution des APIs context-mode et RTK. Basique dans `oh metrics` suffit pour v2.0 |
| `oh beads` enrichi (beyond exec `bd`) | L'exec delegation vers `bd` suffit pour v2.0 |
| `oh yield` enrichi | La corrélation session-commit est complexe. Le ratio basique suffit pour v2.0 |
| Board — modal détail ticket | Overlay modal au clic sur un ticket. Report post-release. |

---

## Estimation restante

| Bloc | Effort estimé | Statut |
|------|---------------|--------|
| Bloc 1 — Parité fonctionnelle | 3-4 jours | ✅ DONE |
| Audit qualité (initial) | 0.5 jour | ✅ DONE |
| Bloc 2 — i18n (initial) | 2 jours | ✅ DONE |
| Bloc 3 — TUI Polish | 0.5 jour | ✅ DONE |
| Bloc 4 — Keychain Fallback | 1 jour | ✅ DONE |
| Audit v2 + Phases A-F | 2 jours | ✅ DONE |
| Bloc 5 — Cleanup + Documentation | 1 jour | ✅ DONE |
| **Total restant** | **0** | **Migration terminée** |

---

## Ordre d'exécution (restant)

```
Tout terminé — prêt pour release.
```

Prochaines étapes release :
- Créer le repo `datichb/homebrew-openhub` (public, vide avec Formula/)
- Tag `v2.0.0` + `goreleaser release --clean`
- Vérifier Homebrew : `brew install datichb/openhub/oh`
- Annoncer la migration

---

## Métriques actuelles (post Bloc 5)

| Métrique | Valeur |
|----------|--------|
| Tests | 213 |
| Packages testés | 24 |
| Linter / Vet | clean (0 issues) |
| Binaire | ~5.5 MB (stripped, CGO_ENABLED=0) |
| Commandes | 30 |
| Sous-commandes | 19 |
| Clés i18n | 474 (parité fr/en, format verb parity checked) |
| Locales supportées | fr, en |
| Plateformes | darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 |
| Features UX | `--json` (7 cmd), shell completion, `--verbose`, `--yes`, confirmations destructives, enriched pre-launch recap |
| Architecture | context.Context propagé, schema versioning, slog, semver partagé |

---

*Mis à jour le 6 juillet 2026 — Migration 100% terminée, prêt pour release v2.0.0*
