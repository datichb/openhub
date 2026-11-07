# Plan — Release v5.0.0

> ## ✅ ÉTAT AU 22/10/2026 AU SOIR : PLAN TERMINÉ — v5.0.0 PUBLIÉE
> - Tag `v5.0.0` sur `26a111d9` (CI verte sur les 3 OS). Release GitHub publiée (latest), 4 archives, sommes et signature Cosign vérifiées, notes avec liens absolus. Tap `datichb/homebrew-tap` en 5.0.0 (`brew info datichb/tap/openhub` → stable 5.0.0). Détail : § A.3 Journal.
> - `main` = `github/main` = `26a111d9`. Aucun worktree ni branche de travail restants. Dossier `openhub-wt/` supprimé ; mesures et sources conservées dans `~/.opencode/plan/mesures/` (`v50-inventaire.md`, `v50-entrees-lots.md`, `v50-goreleaser-snapshot.log`, `audit-v5-recheck.md`, `RESUME-final.md`).
> - Audit v5 : 63 FIXED / 18 PARTIAL / 2 OPEN (avant le tag ; D-01 réglé par le tag, à reporter dans `docs/dev/audit-v5.md`).
> - **Suite : § S ci-dessous (v5.0.1), puis `release-v5.1.md`.**

> Rédigé le 22/10/2026, à partir des analyses du 22/10 (mode équipe, S-38 et release, affichage, architecture, reste de l'audit).
> Périmètre : `cli/` et la documentation du dépôt. La CI réelle et les validations avec de vrais agents (évals, e2e) sont faites **à la main par l'utilisateur**, hors de ce plan.
> Suite : `release-v5.1.md`.
> Ordre réel : V1 → V2 (+ V2-D garde des agents) → tests → **X-1 → X-2 → X-3 (§ X)** → tests → V3 → V4.
> Plan précédent (clos) : `simplification-tests.md`.

---

## D. Décisions prises (22/10)

1. Version d'opencode recommandée : **2.0.25** (celle de la CI).
2. Détail du CHANGELOG : **archivé** dans `docs/dev/changelog-v5-lots.md` ; `[5.0.0]` ne garde qu'un résumé.
3. Mode équipe : publié en **bêta** (avertissement + limites connues) ; correction complète en v5.1 (piste B).
4. Confirmation sans terminal : **refus sauf `--yes`** (code de sortie 2), partout. Rupture annoncée dans le CHANGELOG et `MIGRATION.md`.
5. Les bugs trouvés pendant l'analyse du 22/10 sont corrigés dans cette version (V1-C, V2-A, V2-B).

---

## R. Règles (reprises de `simplification-tests.md`)

- **R1. Interdits** : aucun `git push`, aucun tag, aucune MR/PR, aucune commande qui publie ; aucune éval, aucun e2e, aucun appel de modèle ; pas de modification de `~/.oh`, `~/.config/opencode` ni de la config git globale. `goreleaser` seulement en `check` et `--snapshot` (V4). **Jamais** `oh purge` (même avec `HOME`/`OH_HOME` temporaires : il lance `pkill -f "oh serve"` et `pkill -f "oh mcp"` sur toute la machine), ni aucune commande destructrice réelle hors des dossiers de test.
- **R2. Un worktree par session**, créé directement dans ce dossier :
  ```bash
  cd /Users/benjamin.datiche/workspace/openhub
  git worktree add ../openhub-wt-v50-<lot> -b feat/v50-<lot> main
  ```
  Un commit par sujet, messages au format du dépôt (`fix: …`, `feat: …`, `docs: …` + `(v5.0)`). Pendant le travail, la session ne lance **que les tests de ses paquets**.
- **R3. Fusion locale** sur `main`, une branche à la fois, dans l'ordre de la vague (`git merge --no-ff`), puis `git worktree remove` et `git branch -d`. Si `main` a bougé : `git rebase main`, relancer ses tests. Conflit sur un fichier d'un autre lot : s'arrêter et le signaler.
- **R4. Modèle** : `amazon-bedrock/eu.anthropic.claude-sonnet-5-5` (région : eu-west-1 accepté, voir `simplification-tests.md` R4).
- **R5. Fichiers partagés** :
  - `CHANGELOG.md` : **seul V3-A** l'écrit ; les autres lots rédigent leur entrée dans leur récapitulatif.
  - `docs/dev/audit-v5.md` : **seul V4**.
  - Locales `cli/internal/i18n/locales/{fr,en}.json` : chaque lot n'ajoute ou ne supprime que des clés sous **son préfixe** ; fusions l'une après l'autre.
- **R6. Au plus 3 sessions en parallèle.**
- **Lot terminé** quand : tests de ses paquets verts (avec `-race` pour les paquets concurrents), `go vet` et `golangci-lint` propres sur ses paquets (`GOLANGCI_LINT_CACHE` propre au worktree), récapitulatif écrit (fichiers touchés, entrée CHANGELOG proposée, goldens modifiés et pourquoi, risques).
- **Sessions de tests** : on ne travaille plus sur les tests eux-mêmes, donc **pas de session entre chaque vague**. Une seule session de tests complète **après la vague 2** (tout le code est fusionné) et une vérification finale avant le tag (V4). Contenu : `go vet ./...`, `golangci-lint run`, `make test` (sans integration, e2e, container, eval), goldens modifiés limités à ceux annoncés par les lots.

---

## V1. Vague 1 (3 sessions en parallèle)

### V1-A — S-38 : empreinte d'opencode 2.0.25
- **Fichiers** : `cli/internal/adapters/opencodev2/{pinned_integrity.json, pins.go, compatibility.json, compatibility.go}` et leurs tests (`linux_binary_pin_test.go`, `linux_binary_test.go`) ; `cli/internal/runtime/container/real_test.go` ; nouveau `.github/scripts/opencode-pin.sh` (lecture seule) ; nouveau test dans `cli/internal/docstest` ; facultatif : avertissement dans `cli/cmd/doctor_container.go`.
- **Contenu** :
  - script : pour les 4 cibles Linux (`linux-x64`, `linux-x64-musl`, `linux-arm64`, `linux-arm64-musl`), lit `dist.integrity` et `dist.tarball` sur le registre npm, télécharge le `.tgz`, recalcule `sha512 | base64`, échoue en cas d'écart, affiche le fragment JSON (et, en option, les SHA-256 darwin de la CI) ;
  - épingler **2.0.25** (garder 2.0.20) ;
  - `opencode_recommended` dans `compatibility.json` (`default` et `oh_versions."5.0"`), `CompatRange.OpencodeRecommended`, `RecommendedVersion()` ;
  - `TestPinnedIntegrityTable` utilise `RecommendedVersion()` au lieu de `"2.0.20"` en dur ;
  - `TestRecommendedVersionIsPinned` (dans la plage, 4 cibles) ; `TestCIVersionIsRecommended` (docstest : `OPENCODE_VERSION` de `.github/workflows/ci.yml` = version recommandée) ;
  - compléter le commentaire de procédure de `pins.go` avec le script.
- **Attention** : le script a besoin du registre npm. Sans accès réseau, s'arrêter et demander les valeurs à l'utilisateur. Ne jamais écrire le JSON depuis un test Go.
- **Risque** : une empreinte fausse bloque le téléchargement pour tous → double vérification par le script.
- **Effort** : 1 à 2 sessions.

### V1-B — Mode équipe en bêta (+ colonne REVIEW, vote MCP)
- **Fichiers** :
  - nouveau `cli/internal/beads/status.go` : `LabelReadyForReview`, `BoardColumn(status string, labels []string) string` (reprend `normalizeStatus` ; `in_progress` + label → `review`) ;
  - `cli/cmd/board.go` (`fetchTickets` garde les labels), `cli/cmd/tui_helpers.go`, `cli/internal/tui/v2/views/board.go` (champ `Labels`), `cli/internal/prompt/dev.go` (utilise la constante) ;
  - `cli/internal/mcp/team/server.go` : vote approuvé depuis `in_progress` → `review` puis `done` (ne pas ouvrir `in_progress → done` dans `ValidTransitions`) ;
  - avertissement bêta : `cli/cmd/team.go` (`teamPreRunE`, modèle `printRemoteExperimentalWarning`), `cli/cmd/start_dev_mode.go` (premier claim), `cli/cmd/doctor_claims.go` (contrôle `teamBetaDoctorCheck`), en-tête de `views/teamboard_view.go` et `views/team_detail_view.go` ;
  - docs : bandeau et section « Limites connues » dans `docs/guides/team-setup.{fr,en}.md`, `docs/guides/team-testing-guide.*`, `docs/reference/cli-team.*` ;
  - locales : préfixes `cmd.team.beta.*`, `tui.team.beta.*`.
- **Limites connues à écrire** : claim libéré à l'arrêt même avec `cp-2` en attente ; pas d'expiration (reprise manuelle après `takeover.stale_days`) ; libération perdue hors ligne ou si le démon s'arrête ; `oh team release` en cas d'oubli ; le doctor signale les orphelins.
- **Tests** : tableau `BoardColumn` (label, `deferred`, `pinned`, statut personnalisé `review`) ; `oh board` montre le ticket labellisé en REVIEW ; vote approuvé → `done` + `agent-reviewed`, refusé → `in_progress` ; avertissement une seule fois sur stderr.
- **Effort** : 2 sessions.

### V1-C — Bugs rapides : Politiques, `./hub.toml`, GoReleaser
- **Politiques** :
  - nouveau `cli/internal/services/policy` : `Add`, `Update`, `Remove(ctx, name, Policy)` via `teamstate.LoadPolicies` + `SavePolicies` (sous verrou d'écriture) ;
  - `cli/internal/tui/v2/views/policies_view.go` : appel par callback injecté, dans `safego.Go` + `QueueUpdateDraw` ; supprimer `writePolicyToml` et `openOrCreatePoliciesFile` ;
  - `cli/cmd/policies.go` : utilise le service.
  - Note CHANGELOG : réécrire `policies.toml` fait perdre ses commentaires.
- **`./hub.toml`** : `cli/internal/config/config.go` : supprimer `v.AddConfigPath(".")` ; test : un `hub.toml` du dossier courant est ignoré.
- **GoReleaser** : `cli/.goreleaser.yml` : `release.prerelease: auto`, `brews[].skip_upload: auto` ; `.github/workflows/release.yml` : étape qui extrait `## [5.0.0]` du CHANGELOG dans `$RUNNER_TEMP/notes.md` et `--release-notes` (ne pas écrire dans `dist/`).
- **Tests** : service policy (ajout, mise à jour, suppression, TOML valide) ; la vue n'écrit plus sur la boucle ; config.
- **Effort** : 1 session.

Ordre de fusion : V1-C → V1-A → V1-B.

---

## V2. Vague 2 (3 sessions en parallèle)

### V2-A — Jokers communs (F-12) et confirmation unique
- **Jokers** :
  - nouveau `cli/internal/wildcard` : `Match(pattern, s)` par caractère (rune), `*` et `?` ; test en table avec caractères accentués ;
  - utilisé par `cli/internal/limits/limits.go`, `cli/internal/credproxy/proxy.go`, `cli/internal/bundle/permissions.go` (corrige le comptage par octets) ;
  - `adapters/opencodev2/render.go` garde sa copie (règle D19) + test d'équivalence avec `wildcard.Match` ;
  - `services/session/trace.go` et `evalrun/assert.go` : choix explicite (`Match` ou `MatchStar`), documenté ;
  - `envfilter` reste tel quel (sémantique de préfixe).
- **Confirmation** :
  - `confirmAction(cmd, question string, defaultYes bool) (bool, error)` dans `cli/cmd/confirm.go` : `--yes` → oui ; sans terminal → refus avec message (« confirmation requise : relancez avec --yes ») et code 2 ; sinon formulaire `huh` aux boutons traduits ;
  - remplace `confirm()` de `cli/cmd/workflow_teamcli.go` (appelants : `workflow_publish.go`, `workflow_archive.go`, `workflow_restore.go`, `brick_publish.go`) et les 12 `huh.NewConfirm` (`v5_existing.go`, `project_remove.go`, `migrate_cleanup.go`, `run.go`, `misc.go`, `purge.go` ×2, `worktree.go`, `teams.go`, `session_resolve.go`) ; les commandes concernées gagnent `--yes` si elles ne l'ont pas ;
  - vérifier que le garde global empêche les agents de lancer `oh workflow publish` (sinon l'ajouter et le signaler).
- **Tests** : `wildcard` ; `confirmAction` : `--yes`, sans terminal (refus, code 2, message), avec terminal (formulaire) ; non-régression d'une commande par famille.
- **Effort** : 1 à 2 sessions.

### V2-B — Chargements asynchrones (4 vues sans garde, 2 fausses)
- **Fichiers** : nouveau `cli/internal/tui/v2/views/async.go` (`loadSlot` avec `Invalidate`, `load[T](app, ctx, slot, fetch, apply)` : `fetch` hors boucle sous `safego`, `apply` sur la boucle seulement si la génération est courante) ; `history_view.go`, `metrics_view.go` (plus de lecture de `v.mode`, `v.period`, `v.app` hors boucle), `doctor_view.go`, `gc_view.go` ; `settings_view.go` et `mcp_view.go` (lectures `loadServices` et `buildFields` sorties de la boucle).
- **Hors périmètre** : la migration des ~11 autres vues (v5.1, lot A3).
- **Tests** sous `-race` : monter, démonter, remonter → l'ancien résultat est ignoré ; `TestViews_MountUnmount` vert.
- **Effort** : 1 session.

### V2-C — Inventaire de l'état final (lecture seule)
- **Livrable** : `/Users/benjamin.datiche/workspace/openhub-wt/release/inventaire.md` (hors du dépôt, aucun commit).
- **Contenu**, vérifié dans le code et les docs actuels : touches de la TUI (`tui/v2/keys`), alias des workflows (`aliases:` des YAML), nombre d'agents et de workflows, codes de sortie (`docs/reference/cli.*`), `PipelineSchema` (`remote/contract.go`), options courtes, clés de configuration ajoutées ou retirées ; liste dédoublonnée des ⚠ et « Rupture » de `[Unreleased]` (état final seulement) ; tableau des contradictions (une vingtaine repérées le 22/10, voir § A.1) ; liste des ruptures absentes de `MIGRATION.md` et `migration-v5.*`.
- **Effort** : 1 session.

Ordre de fusion : V2-B → V2-A (V2-C ne fusionne rien).

→ **Session de tests complète** (seule sur la machine), voir R6. Si quelque chose casse : correctif dans un nouveau worktree avant la vague 3.

---

## X. Lots ramenés de la v5.1 (décidé le 22/10, après la vague 2)

> À lancer **après la session de tests complète de la vague 2** (et après la fusion de V2-D, garde des agents). Trois vagues de 3 lots, puis une **deuxième session de tests complète**, puis V3 et V4. Le mode équipe reste en **bêta** (B2 à B6 de la v5.1 restent en v5.1). Le TOML strict (X4) est une rupture acceptée pour la 5.0.
> Mêmes règles (§ R). Worktrees `../openhub-wt-v50-x<n>`, branches `feat/v50-x<n>`. Chaque session lance `make sync-content` avant `go test ./cmd/`. Vérification de l'orchestrateur après chaque fusion : `go build ./...`, `go vet ./...`, `internal/archtest`, `internal/i18n`, tests des paquets du lot, et `go test -short ./...` à la fin de chaque vague.
> Fichiers partagés : locales avec un préfixe par lot ; `docs/reference/cli.{fr,en}.md` touché par X3 puis X6 puis X9 (vagues différentes) ; `CHANGELOG.md` toujours réservé à V3-A (chaque lot propose son entrée et la liste de ses ruptures dans son récapitulatif).

### Vague X-1 (3 sessions en parallèle)

**X1 — Sécurité et petits bugs**
- S-20 : `cli/internal/workflow/patch.go` (~l.118-123, 302-308) : ajout d'un agent ou de `skills.extra` **refusé** dans une couche de projet (contenu du dépôt), avertissement gardé dans une couche d'équipe ; vérifier que l'agent ajouté respecte `risk:`. Diagnostic clair et testé.
- S-11 : `cli/internal/adapters/opencodev2/attest.go` (~l.96-115) : l'avertissement « plugin inconnu » (hors isolation stricte) remonte dans le récapitulatif de lancement et dans `oh doctor` (`cli/cmd/doctor_security.go` ou équivalent) ; pas de changement du défaut.
- F-07 : `cli/cmd/v5_inputs.go` : sur un remote GitHub (`config.ParseGitRemote`), message clair « review-feedback : entrées de MR GitLab seulement » au lieu d'une erreur de jeton ou de projet.
- Course `EnforcedURLs` : `cli/cmd/mcp_trust.go` `tuiEnforcedURLs` (~l.341), appelé hors de la boucle depuis la vue MCP (V2-B) alors que la boucle remplace `*a.Config` à l'enregistrement : travailler sur une copie (ou un instantané pris sur la boucle). Test `-race`.
- Locales : `cmd.workflow.patch.*` / `cmd.doctor.plugins.*` / `cmd.inputs.github.*` (selon les clés existantes).
- Tests : refus en couche de projet, avertissement en couche d'équipe ; avertissement plugin visible ; message GitHub ; course (`-race`).

**X2 — Garde-fous de la suite**
- C1 / A-03 : `cli/internal/archtest/tool_independence_test.go` (~l.50-59) : remplacer `strings.Contains(expr.String(), tag)` par une évaluation `constraint.Expr.Eval` (un fichier n'est exempté que s'il n'est **pas** compilé sans étiquette et l'est avec `integration`, `e2e` ou `container`) + test de `allowed()` avec `!integration`, `integration && linux`, `e2e || container`. Nouveau `archtest/layers_test.go` (`go list -deps`) : `internal/services/**`, `runsvc`, `daemon`, `domain` n'importent ni `internal/tui/**` ni `cmd` ; `internal/tui/**` n'importe pas `internal/storage/**` ; `internal/config` n'importe pas `database/sql`. Liste d'exceptions figée des violations actuelles (`cmd/*` → `storage/sqlite` hors racine de composition, `launcher/ui_cli.go` → `tui/theme`, `config/migrate.go` → `database/sql`…), qui échoue si elle grossit **ou** si une entrée n'existe plus.
- A-07 : `cli/internal/storage/sqlite/store.go` : chaque migration v24-v34 (et au-delà si besoin) a un `down` (`DROP COLUMN` ; pour v29 `DROP INDEX idx_sessions_group` d'abord) ou `irreversible: true` ; retirer les `irreversible: false` trompeurs ; test « chaque migration a un down ou est irréversible » + aller-retour sur les migrations réversibles ; une phrase de doc « revenir en arrière = restaurer une sauvegarde (`oh repair --restore`) » dans la page de référence adéquate.
- A-09 : nouveaux tests pour `cli/internal/mcp/figma`, `cli/internal/mcp/gslides` (`httptest`, modèle `mcp/jira`), `cli/internal/gateway/beadswire` (aller-retour du format), registre `cli/internal/adapters`, `cli/internal/sessionresults`, `cli/cmd/purge.go` (HOME temporaire, `--dry-run`), `cli/cmd/remote_setup.go` (faux GitLab).
- Aucune modification de code de production hors `store.go` (sauf correction d'un bug trouvé par un test, à signaler).

**X3 — Aide et aide-mémoire**
- U-01 : `cli/cmd/help.go` (`helpCommands` ~l.171-193, sections ~l.54-66) : par défaut une ligne par commande de premier niveau (sans sous-commandes ni options) ; `oh help --all` garde l'affichage complet actuel ; section « Avancé » (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`) ; `teams` devient un alias déprécié de `team` (champ `Deprecated`, comme U-02) si les sous-commandes se recoupent — sinon le documenter et s'arrêter. Adapter `help_test.go` (`TestHelpSectionsCoverEveryCommand`) et `docs/reference/cli.{fr,en}.md`.
- U-07 : aide de la vue Sessions (clé de `fr.json`/`en.json` ~l.4155, 21 entrées) réduite à ~8 touches essentielles + « ? aide » ; test dans `cli/internal/tui/v2/keys/convention_test.go` (au plus 10 entrées ou 120 colonnes pour toute ligne d'aide d'une vue).
- **Pas** U-09 (dans X8, même fichier que `oh init`).
- Locales : `help.*`, `tui.sessions.hints*`.

→ Fusion X1 → X2 → X3, vérification `-short`.

### Vague X-2 (3 sessions en parallèle)

**X4 — Configuration : TOML strict, écriture sûre de `hub.toml`, clés héritées** *(rupture acceptée)*
- C3 : retrait de viper (`cli/internal/config/config.go` ~l.541-567, `legacy_keys.go`) : `os.ReadFile` + `toml.Unmarshal` (go-toml/v2, déjà utilisé par `Save`) ; valeurs par défaut appliquées sur la struct avant le décodage ; `applyLegacyKeys` depuis un `map[string]any` décodé à part ; message clair à la ligne et à la colonne en cas d'erreur de type. Corpus de `hub.toml` réels en test (clés en majuscules, `"true"` en chaîne, `[team]` historique, clés héritées) : documenter précisément ce qui était accepté et ne l'est plus. Retirer viper de `go.mod` si plus aucun usage (`go mod tidy`).
- C2 pour `config.Save` uniquement : écriture atomique avec `fsync` (et plus de nom temporaire fixe `hub.toml.tmp`). Utiliser le paquet `internal/fsutil` **créé par X5** si déjà fusionné ; sinon écrire l'écriture atomique localement dans `config` et la signaler (X5 la remplacera).
- A-08 : avertissement (une fois) à la lecture d'une clé héritée (`legacy_keys.go`, `mcpkeys.go`) et contrôle dans `oh doctor` ; fin annoncée en v6 ; page « Où se règle quoi » (cascades des modèles `bundle/models.go` et des restrictions `limits/limits.go`, origine de chaque réglage) dans `docs/reference/config.{fr,en}.md` ou une page dédiée.
- Locales : `cmd.config.*`, `cmd.doctor.legacy.*`.

**X5 — Écritures atomiques**
- Nouveau `cli/internal/fsutil` : `WriteFileAtomic(path, data, perm)` (`os.CreateTemp` dans le même dossier, write, `f.Sync()`, close, chmod, rename, fsync du dossier sous unix, rien sous Windows) + variante sans fsync pour les écritures fréquentes si utile ; tests (permissions, nettoyage du temporaire en cas d'erreur, remplacement).
- Migrer les sites critiques : `cli/internal/storage/filecrypt/store.go`, `storage/keychain/index.go`, `storage/sqlite/server_store.go` (mots de passe), `gateway/store.go`, `gateway/shellenv.go`, `tracker/syncstate.go`, `services/report/report.go`, `services/wiki/apply.go`, `services/session/trace.go`, `cmd/team_config.go`, `cmd/export.go`, `cmd/v5_beads_local.go`. **Ne pas toucher** `internal/config/` (X4). Les renommages de dossiers (`remote/fetch.go`, `workflow/bricks.go`) restent tels quels.

**X6 — Lancement, coûts, vocabulaire**
- F-13 : `oh run --dry-run` (`cli/cmd/run.go`, `runWorkflowCLI`) : prépare (`prepareWorkflowRun`), affiche le récapitulatif, les pauses et les avertissements, puis s'arrête sans démarrer ; refusé ou sans effet en distant (`prepareRemote` sauté) ; pas d'import d'environnement Bedrock en dry-run ; cas `--headless`. `oh budget report --by project|workflow --since <durée>` (`cli/cmd/budget.go`, `storage/sqlite/usage_store.go` + interface `domain`) : jointure externe sur `sessions.workflow_id` (sessions supprimées par `gc` comptées sous « inconnu »). « Annuler une session » = `oh session delete`, documenté dans `docs/reference/cli-sessions.*`.
- U-04 : `manual` accepté partout où l'on saisit un mode (`--mode`, `modes.default/allowed`, clés des maps `mode:`) et ramené à la valeur canonique `manuel` à la lecture (`cli/internal/workflow/schema.go` ~l.116-120) ; `aliases: [free]` dans `workflows/libre.yaml` ; ID TUI `coder` → `libre` avec `coder` en alias (`cli/cmd/tui_commands.go` ~l.27-29, `tui_commands_test.go`). Vérifier que `extends: hub:libre` reste valide.
- U-12 : « mode » réservé au mode de workflow dans `docs/reference/glossary.{fr,en}.md` et les libellés i18n (agent → « type d'agent », TUI → « vue Hub / Projet / Équipe », init → « type d'installation ») ; aucune clé YAML ni option ne change.
- Locales : `cmd.run.dry_run.*`, `cmd.budget.report.*`, et les libellés U-12 concernés.
- Ne touche pas `storage/sqlite/store.go` (X2, déjà fusionné) sauf nouvelle migration nécessaire, à éviter.

→ Fusion X5 → X4 → X6, vérification `-short`.

### Vague X-3 (3 sessions en parallèle)

**X7 — Service équipe**
- B1 (v5.1) : nouveau `cli/internal/services/team` (n'importe ni `cmd` ni `tui`) : `ResolveTeam(project)` (une règle, avec repli sur le hub) ; `TrackerEngine(ctx)` et `SyncTracker(ctx, opts{Pull bool})` avec erreurs typées (`ErrTeamNotConfigured`, `ErrNotCloned`, `ErrTrackerDisabled`, `ErrNoProjects`, `*CredentialsError`) traduites par la CLI, la TUI et le moteur automatique — **la TUI respecte `enabled = false`** ; `GitLabToken(ctx, teamID) (token, source)` : une seule cascade, testée en table (remplace `cmd/audit_review_debug.go` ~l.238, `team_rejoin_core.go` ~l.247, `team_helpers.go` ~l.190, `tracker/resolve.go` ~l.87, `cmd/mcp.go` ~l.790) ; `Init(ctx, InitParams)` (reprend `teamInitCore`). Fichiers : `cmd/{tui_tracker,team_sync_tracker,tui_team_tracker,team_helpers,team_rejoin_core,audit_review_debug,team,tui_team_actions,team_init_core,mcp}.go`.
- `cmd/team_steps.go` : écrit les politiques via `services/policy` (au lieu de `SavePolicies` + `CommitAndPush`).
- Ruptures à signaler : ordre de recherche du jeton GitLab unifié ; la TUI ne synchronise plus un tracker désactivé ; `oh review publish` trouve aussi la clé d'équipe.
- Ne touche pas `cmd/init_wizard.go` ni `cmd/init_steps.go` (X8) : si `teamInitCore` y est appelé, garder la signature et la faire déléguer au service.

**X8 — `oh init` en 3 écrans**
- U-10 : `cli/cmd/init_wizard.go` (~l.118-191), `cli/cmd/init_steps.go` : (1) accueil avec langue et type d'installation sur la même page ; (2) fournisseur (l'intro devient l'en-tête) ; (3) projet facultatif, prérempli avec le dossier courant ; MCP sorti de l'assistant (suggestion `oh mcp setup` dans le récapitulatif) ; groupe équipe seulement en `team` / `full` ; débuts de groupe calculés à partir des `step.ID` (fin des index en dur `LangStepIdx`, `providerGroupStart`, `teamGroupStart`, et du contournement `_ = origRequired`).
- U-09 : `init_steps.go` (~l.242) : la description de l'option sélectionnée toujours visible (une ligne sous le sélecteur), même si `availH < 30`.
- Tests `cmd/init*_test.go` : parcours solo (3 écrans), équipe, complet ; goldens éventuels relus et justifiés.
- Locales : `cmd.init.*`.

**X9 — `--yes` partout**
- C9 (partie options) : sur `purge`, `project remove`, `service remove`, `teams remove`, `worktree prune|cleanup`, `--force` (qui sert à sauter la confirmation) devient un alias **déprécié** de `--yes` (masqué de l'aide, avertissement), modèle `cmd/deprecated_options.go` (U-03) ; les `--force` qui veulent dire « outrepasser une sécurité » restent (`daemon stop --force`, `remote setup --force`, `worktree remove --force` / le `-f` de suppression forcée de `worktree cleanup`) — vérifier chaque cas et le documenter.
- `docs/reference/cli.{fr,en}.md`, aides traduites, tests des alias dépréciés (`deprecated_options_test.go`).
- Fichiers : `cmd/{purge,project_remove,misc,teams,worktree,deprecated_options}.go` et tests. Ne touche pas `cmd/confirm.go` sauf nécessité (V2-A l'a déjà posé).

→ Fusion X7 → X9 → X8, vérification `-short`.

→ **Deuxième session de tests complète** (vet, lint, `make test` en `-race`, goldens), puis V3 (V3-A reprend les entrées et ruptures des lots V1, V2 et X), puis V4.

### Estimation
9 lots, 3 vagues ; environ 45 min à 1 h par vague (rythme constaté le 22/10), soit 2 h 30 à 3 h, plus la deuxième session de tests.

---

## V3. Vague 3 (2 sessions en parallèle, documentation)

### V3-A — CHANGELOG `[5.0.0]`
- **Structure** : `## [Unreleased]` vide ; `## [5.0.0] — <date>` ; « En bref » (10 à 12 puces : runtime v5 et opencode V2, workflows `oh/v1`, workflows d'équipe et briques, checkpoints, conteneur, distant expérimental, fan-out, wiki vivant, sécurité, TUI refondue, mode équipe bêta) ; « ⚠ Ruptures et actions requises » dédoublonnées, par thème (opencode et installation, commandes et options, workflows et couches, configuration, sécurité visible, touches TUI, distant, base de données, confirmation `--yes`), chaque puce donne l'action et renvoie à `MIGRATION.md` ; puis **une seule** section par catégorie (Added, Changed, Fixed, Security, Removed), en synthèse.
- **Détail** : déplacé dans `docs/dev/changelog-v5-lots.md` (liens relatifs recalculés) ; lien depuis `[5.0.0]`. La section « Documentation » héritée de la 4.x (`CHANGELOG.md` ~l.1167-1188) part dans l'archive.
- Ajouter les entrées des lots V1 et V2 (depuis leurs récapitulatifs).
- **Règle** : tout est vérifié contre l'inventaire (V2-C) et le code, jamais contre les anciennes entrées.
- **Tests** : `go test ./internal/docstest/...` (`TestDocLinks`).
- **Effort** : 2 sessions (enchaînées dans la même branche).

### V3-B — MIGRATION, guides de migration, SECURITY
- `MIGRATION.md` : passage v4 → v5 en tête (ruptures de l'inventaire, `oh remote setup` à relancer, `execution.env_allow`, `oh mcp trust`, codes de sortie, options courtes, touches, alias `feature`/`cadrage`, clés `[llm] default_provider` et `[execution] tool_version`, confirmation `--yes`) ; partie « oc → oh » marquée historique.
- `docs/guides/migration-v5.{fr,en}.md` complétés (parité FR/EN obligatoire).
- `SECURITY.md` / `SECURITY.fr.md` : « garde-fous, pas barrières » ; limites assumées S-11, S-12, S-13, S-15, S-20, S-39 ; mode équipe bêta, distant expérimental, Windows moins vérifié (27 paquets exclus de la CI) ; empreinte opencode 2.0.25.
- `.github/pull_request_template.md` : retirer `oh deploy`.
- **Tests** : `docstest`.
- **Effort** : 1 à 2 sessions.

---

## V4. Préparation du tag (1 session)

- Nouveau `docs/dev/release-checklist.md` : CHANGELOG à jour (ADR-037), `goreleaser check`, `--snapshot`, CI verte sur les 3 OS, tag poussé par un humain (signature liée à `release.yml@refs/tags/v*`), vérification de la release (Homebrew, `install.sh`, `oh upgrade oh`).
- `docs/dev/audit-v5.md` : S-38 et N-1 → FIXED (2.0.25 épinglée, CI alignée) ; F-12 → FIXED ; F-02 et F-03 → « bêta assumée » (lien vers les limites connues) ; N-4 suit F-02 ; A-04 clos par justification (verrou de dernier niveau, sections courtes) ; D-01 noté « tag à poser » ; bilan recalculé.
- En local : `goreleaser check`, `goreleaser release --snapshot --clean`.
- Vérification finale : `go vet ./...`, `golangci-lint run`, `make test`.

→ **L'utilisateur** pousse, vérifie la CI sur les 3 OS, puis **pose le tag `v5.0.0`**.

---

## S. Suite immédiate : v5.0.1 (petite version, à faire en premier)

| # | Sujet | Détail |
|---|---|---|
| 1 | PR Dependabot restantes | #9 `bubbletea` 1.3.6 → 1.3.10, #12 `modernc.org/sqlite` 1.59.0 → 1.60.1 (touche la base : tests de migration et `-race` complets avant fusion). #10 et #13 fermées (incluses via `x/crypto` 0.58.0). |
| 2 | Audit | `docs/dev/audit-v5.md` : D-01 → FIXED (tag `v5.0.0`), bilan 64 / 17 / 2. |
| 3 | Documentation | `docs/guides/contributing.{fr,en}.md` § « Créer une release » périmé (exemple `v1.2.0`, rédaction sous `[Unreleased]`) → renvoyer à `docs/dev/release-checklist.md` ; retirer la piste « e2e et conteneur planifiés en CI » (A-06, P2 de l'audit) : décision de l'utilisateur, **pas d'e2e ni d'évals en CI**. |
| 4 | Petits restes de la v5.0 | texte français en dur de `oh service` (`cmd/misc.go:59-78`) ; confirmations encore en code 1 sans terminal (`gc`, `doctor --fix`, `repair`, `mcp trust`, `beads clean`, `doctor_db`) et boutons « Launch/Cancel » de `repair` → `confirmAction` ; `cmd/team_config.go` (assistant du tracker) sans l'étape « équipe » de la cascade du jeton ; `buildMCPTokenStep` mort ; SECURITY « verified on opencode 2.0.20 » à revérifier sur 2.0.25. |
| 5 | Vérifications manuelles laissées à l'utilisateur | évals et e2e (hors CI), `oh init` 3 écrans et défaut « solo », mode équipe bêta, plugins S-11 (format de config d'opencode lu par `oh doctor`), jeton d'équipe de `oh mr create`. |
| 6 | Environnement | `brew upgrade go` en local (Go 1.26.4 vulnérable ; la directive `toolchain go1.26.10` le télécharge sinon) ; fichier non suivi `docs/dev/rapport-modernisation-2026-10.html` (à committer, déplacer ou supprimer) ; dossier `../openhub-test/` (bac à sable manuel, 75 Mo) ; dossier temporaire d'OpenCode (~4,3 Go de fichiers des sessions précédentes) : à nettoyer avec l'accord de l'utilisateur. |

Leçons de la journée (à garder pour les plans suivants) :
- Les sessions Sonnet vont 5 à 10 fois plus vite que l'estimation initiale (10 à 45 min par lot) : un tour de 3 lots ≈ 45 min à 1 h avec la fusion.
- Vérification après **chaque** fusion : build, `go vet ./...`, `internal/archtest`, `internal/i18n`, tests du lot ; un lot qui touche `cmd/` lance `go test -short ./cmd/` en entier ; toujours enchaîner avec `&&` ou `set -e` (incident du 22/10 : une fusion locale malgré un `vet` en échec, annulée avant push).
- Simuler le runner en local avant de pousser : `HOME` vide et `GIT_CONFIG_GLOBAL=/dev/null`.
- `goreleaser release --snapshot` en local : toujours `--skip=sign`.
- Ne jamais lancer `oh purge` ni de commande destructrice réelle, même isolée.

---

## A. Annexes

### A.1 Contradictions repérées dans `[Unreleased]` (22/10)
Alias `secu/perf/dbg/feedback/retours` (retirés puis rétablis) ; aide `F1` → `?` et `Ctrl+P` ; touches Sessions `r/t/k/d/p/D` → `R`, `T`, `d` ; badge `● N ⏸ M` → `● N ◐ M` ; `--how` → `--attach` ; repli sur l'ancien lancement (supprimé) ; `oh start` → `feature` puis alias de `spec`/`implement-spec` ; `feature` et `cadrage` présentés comme workflows (alias) ; `request` de `spec` obligatoire → facultative ; `-m` de `feature` ; nombre d'agents et de workflows (16 / 18 / 24) ; mentions « non fait » ou « à venir » devenues faites (S-13, S-31, S-32…) ; `orchestrator-dev-parallel` créée puis retirée ; commits de l'onboarder ; `cp-1/2/3` ; `coordination-only` supprimée ; vue Nettoyage ; session libre « V1 » ; schéma du pipeline 2 → 5 ; épinglage 2.0.20 vs CI 2.0.25 ; section Documentation de la 4.x (SVG, `oh skill budget`, `--parallel`, `--sweep`).

### A.2 Estimation
13 sessions, 4 vagues, environ 2 jours.

### A.3 Journal
| Date | Étape | Résultat |
|---|---|---|
| 22/10/2026 | Vague 1 lancée | Worktrees `openhub-wt-v50-{s38,team-beta,quickfix}` (depuis `0b5c4836`), 3 sous-sessions Sonnet 5.5 EU en cours. Registre npm accessible depuis la machine. |
| 22/10/2026 | V1-A et V1-C fusionnées | V1-C `922c2a85` (service policy, `./hub.toml` ignoré, GoReleaser rc + notes depuis le CHANGELOG via `.github/scripts/release-notes.sh`) ; V1-A `0c00ac89` (2.0.25 épinglée, empreintes revérifiées par l'orchestrateur, `RecommendedVersion()`, `TestCIVersionIsRecommended`, avertissement doctor). Build et tests des paquets touchés verts après fusion. Durées réelles : V1-A < 10 min, V1-C ~20 min. Points ouverts : (1) `goreleaser check` échoue car `brews` est déprécié (→ `homebrew_casks`), à décider en V4 ; (2) V3-A doit créer `## [5.0.0]` avant tout tag rc ; (3) chaque nouveau worktree doit lancer `make sync-content` avant `go test ./cmd/` ; (4) `cmd/team_steps.go` écrit encore les politiques via `SavePolicies` + `CommitAndPush` (hors périmètre, à noter pour v5.1). |
| 22/10/2026 | V1-B fusionnée, vague 2 lancée | V1-B `8ea00199` (BoardColumn, vote MCP par review, avertissement bêta CLI/doctor/vues, limites connues FR/EN ; aucun golden modifié ; durée ~40 min). Après fusion : build et tests i18n, beads, mcp/team, prompt, docstest verts. Vague 1 terminée. Vague 2 lancée : worktrees `openhub-wt-v50-{confirm,async}`, V2-C en lecture seule vers `openhub-wt/release/inventaire.md`. |
| 22/10/2026 | Correctif D19 + V2-B fusionnés | V2-B a détecté que `archtest` échouait sur `main` depuis V1-A (`cmd/doctor_tool*.go` et `docstest/ci_opencode_test.go` importaient `opencodev2`, règle D19). Corrigé par l'orchestrateur (`38f9efbc`) : `toolPins()` dans la racine de composition `cmd/v5_adapters.go`, test de version CI déplacé dans `internal/adapters/opencodev2/ci_version_test.go`. Leçon : la vérification après fusion inclut désormais `internal/archtest`. V2-B `7715be6f` (`load[T]`, 6 vues ; aucun golden ; ~35 min). Après fusion : build, vet tui, `views` et `archtest` sous `-race` verts. Points ouverts V2-B pour v5.1 : `EnforcedURLs()` lit `a.Config` hors boucle (course possible pendant un enregistrement, copie de config côté `cmd`) ; `pinURL` (MCP), `refresh()` (Réglages) et `deployPane.scan()` (Nettoyage) restent sur la boucle (lot A3). |
| 22/10/2026 | V2-A fusionnée, V2-C livrée — **arrêt demandé par l'utilisateur après la vague 2** | V2-A (`confirmAction` : refus sans terminal sauf `--yes`, code 2, boutons traduits, `--yes` ajouté à `run`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup` ; `internal/wildcard`, permissions par caractère ; ~45 min). V2-C : `openhub-wt/release/inventaire.md` (48 ruptures dont 27 réelles pour un utilisateur 4.2.0, 2 seulement couvertes par MIGRATION ; 33 contradictions ; `SSH_AUTH_SOCK`/`GH_TOKEN` filtrés par `execution.env_allow` non documenté ; `oh config model -j` et `oh purge --include-opencode` supprimés sans alias ; `oh service` déprécié absent du CHANGELOG ; chiffres faux dans README/getting-started/skills). Corrections de l'orchestrateur après fusion : exception littérale `policies: add %s` devenue inutile (V1-C) ; `TestMetricsViewKeysReloadWithTheirPeriod` instable (ordre de deux chargements concurrents, V2-B) → `ElementsMatch`, stable sur 30 passages `-race`. Vérification : `go vet ./...` propre ; `go test -short ./...` vert sauf `cmd TestPreconditionsGit` une fois (17 s, sous charge, fichier non touché par la v5.0 ; vert en isolation ×5 et sur un 2e passage complet de `cmd`) → à surveiller en session de tests. **Décisions en attente** : (1) garde global des agents (`bundle/globalguard.go`, `ohControlDeny`) ne couvre ni `workflow publish|archive|restore`, ni `brick publish`, `purge`, `project remove`, `teams remove` (un agent peut passer `--yes`) ; (2) `goreleaser` `brews` déprécié → `homebrew_casks` ? Restes notés pour v5.1 : confirmations en code 1 (`gc`, `repair`, `doctor --fix`, `mcp trust`, `beads clean`, `doctor_db`), réponses tapées (`secrets delete`, `claim`, `review feedback`, `init`, `team_config`), boutons « Launch/Cancel » non traduits de `repair`. Prochaine étape : session de tests complète, puis vague 3. |
| 22/10/2026 | V2-D fusionnée | `cdd1fe5e` : `ohControlDeny` refuse `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove` (alias `rm`, `p`), `teams remove`, `--yes` compris ; lectures toujours permises ; SECURITY FR/EN, `cli-mcp`, `team-workflows` complétés ; aucun golden. bundle, archtest, docstest verts après fusion. Ouvert : `teams archive|restore` non ajoutés (à décider). Session de tests complète en cours sur `12eb4647` (avant V2-D ; V2-D sera couverte par la 2e session). |
| 22/10/2026 | **Session de tests complète n°1** (`12eb4647`, avant V2-D) | vet 0, lint 0 issue ; `go test ./... -race` : 4392 tests passés, 8 ignorés, **2 échecs liés à la charge** (charge moyenne 100 à 136 pendant le passage : sous-sessions, Chrome, VM) : `gitutil TestFingerprintDoesNotBlockOnFIFO` (délai) et `fanout TestDriverMergerKeepsAPartForThePendingChildren` (`context deadline exceeded` après 221 s) ; aucun des deux paquets n'a été modifié par la v5.0 ; verts en isolation `-race` (×5 et ×3). Goldens : aucun fichier `testdata` modifié depuis `0b5c4836`. Durées non exploitables (docstest 1087 s, cmd 836 s sous charge). **Vagues 1 et 2 validées.** Mesures : `openhub-wt/mesures-v50/`. À surveiller : ces deux tests ont un délai fixe sensible à la charge (déjà noté dans `simplification-tests.md` É.3). |
| 22/10/2026 | Vague X-1 lancée | Worktrees `openhub-wt-v50-{x1,x2,x3}` depuis `cdd1fe5e`. X1 inclut en plus `teams archive|restore` dans le garde des agents. |
| 22/10/2026 | Vague X-1 fusionnée | X1 `3be8f92d` (S-20 refus en couche de projet = `LayerProject` du team-state ; S-11 plugins visibles au lancement et dans le doctor via `adapters.UserPluginLister` — format de config d'opencode à vérifier à la main ; F-07 ; course `EnforcedURLs` par instantané ; garde `teams archive|restore`), X2 `db2deb3f` (`archtest` : contraintes évaluées, `TestLayers` avec 24 exceptions figées dont 20 fichiers `cmd` → `storage/sqlite` ; `down` pour v24, 25, 26, 27, 29, 31, 34 + aller-retour ; tests figma, gslides, beadswire, registre, sessionresults, purge, remote setup), X3 `0bf44af2` (`oh --help` 573 → 67 lignes, `oh help --all` identique à l'ancien ; `teams` reste distinct de `team` ; Sessions 8 touches + `?` liste tout via `HelpKeysProvider`). Correctif orchestrateur `bacdffbf` : `TestBinarySmoke/help` attendait l'ancienne aide (X3 n'avait lancé que `-run Help…`) → les lots qui touchent `cmd/` lancent désormais `go test -short ./cmd/` en entier. `go test -short ./...` vert après correctif. Bugs trouvés par X2, à corriger en X5 : `file_key=".."` non refusé (figma, gslides) ; id de session avec `../` non refusé (`sessionresults`). Ruptures à reporter au CHANGELOG : couche de projet qui ajoute agent ou skills → erreur ; `oh --all` seul affiche l'aide complète. |
| 22/10/2026 | X5 fusionnée | `internal/fsutil` (`WriteFileAtomic` avec fsync du fichier et du dossier, `NoSync` pour l'historique des notifications), 14 sites migrés (+ `deploycleanup` par l'orchestrateur, `4479c6fd`), identifiants vérifiés dans figma, gslides (`slide_id` aussi) et `sessionresults` (`ErrInvalidID`). Build, vet, archtest, i18n verts. À faire à la fusion de X4 : remplacer son écriture atomique locale de `config.Save` par `fsutil.WriteFileAtomic`. |
| 22/10/2026 | X6 fusionnée (avant X4, sans recouvrement) | `oh run --dry-run` (seule écriture : paquet de session adressé par son contenu ; ni claim, ni worktree, ni shim, ni question ; distant refusé), `oh budget report --by project|workflow --since --json`, `manual`→`manuel` à `Parse`, `free` alias de `libre`, commande TUI `coder`→`libre`, U-12 (libellés ; guides encore partiels). 1 golden modifié et justifié (`bricks_detail/hub_agent` : « Type d'agent »). Après fusion : build, vet, archtest, i18n, workflow, `cmd -short` verts. Conflit possible avec X8 sur `cmd.init.wizard_section_mode`. |
| 22/10/2026 | X4 fusionnée — vague X-2 terminée | Lecture stricte de `hub.toml` (go-toml, erreurs avec fichier/ligne/colonne/clé ; clé inconnue = avertissement ; casse conservée pour `remote.projects` et `binaries`), viper retiré, `Save` via `fsutil`, clés héritées annoncées une fois et listées par le doctor, `doctor --fix` ne restaure qu'une sauvegarde valide, page « Où se règle quoi ». Après fusion : build, vet, `go test -short ./...` **vert**. Copie du `hub.toml` réel de l'utilisateur chargée sans erreur ni avertissement (OH_HOME temporaire). |
| 22/10/2026 | X9 fusionnée (avant X7, sans recouvrement) | `--force`/`-f` alias déprécié de `--yes` sur `purge`, `project remove`, `service remove`, `teams remove` ; gardé sur `daemon stop`, `remote setup`, `worktree remove`, `worktree cleanup` (ce dernier supprime aussi les worktrees modifiés : sens « forcer »). Après fusion : build, vet, archtest, i18n, docstest, `cmd -short` verts. |
| 22/10/2026 | X8 puis X7 fusionnées — **vagues X terminées** | X8 (`oh init` : solo 8 → 3 écrans, équipe 4, complet 5 ; MCP sorti de l'assistant ; groupes calculés depuis les `step.ID` ; U-09 ; `hub.toml` produit identique à l'ancien code pour le même scénario ; 38 clés orphelines retirées). X7 (`internal/services/team` ; cascade unique du jeton GitLab dans `tracker.ResolveGitLabToken` ; la TUI respecte `enabled = false` ; `oh mr create` lit l'équipe du projet ; `team_steps` via `services/policy` ; aucune exception `TestLayers` changée). Après fusion : build, vet, `go test -short ./...` **vert**. Entrées CHANGELOG de tous les lots rassemblées dans `openhub-wt/release/entrees-lots.md` (source de V3). Points ouverts : défaut d'`oh init` passé de « complet » à « solo » (à valider) ; Entrée sur le projet prérempli avec `$HOME` enregistre le dossier personnel (risque préexistant) ; `cmd/team_config.go` (assistant du tracker) construit encore ses sources de jeton sans l'étape « équipe » ; `buildMCPTokenStep` n'est plus utilisé que par ses tests. |
| 22/10/2026 | 2e session de tests lancée + vague 3 lancée en parallèle | Tests complets sur `d4d0134a` (`openhub-wt/mesures-v50-2/`). V3-A (CHANGELOG [5.0.0], détail archivé dans `docs/dev/changelog-v5-lots.md`) et V3-B (MIGRATION, migration-v5 FR/EN, SECURITY FR/EN, modèle de PR, chiffres faux du README et des guides) lancées en parallèle : documentation seule, sources `openhub-wt/release/{inventaire,entrees-lots}.md`. |
| 22/10/2026 | **Session de tests complète n°2** (`d4d0134a`, après V2-D et X1 à X9) | vet 0, lint 0 issue ; `go test ./... -race` : **4770 passés, 0 échec**, 8 ignorés, 515 s (charge 30-45). Goldens : un seul fichier modifié depuis `0b5c4836`, `bricks_detail/hub_agent.golden` (X6, « Type d'agent », justifié). **Code de la v5.0 validé.** Mesures : `openhub-wt/mesures-v50-2/`. |
| 22/10/2026 | V3-A fusionnée | `CHANGELOG.md` 380 Ko → 171 Ko ; `[5.0.0]` ≈ 180 lignes (En bref 12 puces, 46 ruptures en 9 thèmes, une section par catégorie) ; détail des lots archivé tel quel dans `docs/dev/changelog-v5-lots.md` ; 33 contradictions résolues ; `release-notes.sh v5.0.0` produit la section ; docstest vert. **Incident** : la sous-session a lancé `oh purge --force` avec `HOME`/`OH_HOME` temporaires ; vérifié par l'orchestrateur : `~/.oh`, trousseau `openhub-oh`, `cli/bin/oh`, opencode intacts ; mais `purge` lance `pkill -f "oh serve"`/`"oh mcp"` sur toute la machine → règle R1 complétée, **défaut de `purge` à corriger** (motif trop large, non lié au hub). À faire en V4 : liens relatifs de `[5.0.0]` non résolus dans les notes GitHub → réécrits en URL absolues vers le tag par `release-notes.sh` ; date à ajuster au tag. |
| 22/10/2026 | V3-B fusionnée — vague 3 terminée | MIGRATION (4.x → 5.0 en tête : prérequis, 11 étapes, ruptures par thème, retour arrière ; partie oc → oh en historique), guides migration-v5 FR/EN (sections 1-10 gardées, 11-20 ajoutées), SECURITY FR/EN (limites assumées et maturité ; empreintes 2.0.20 et 2.0.25 ; S-20 ; garde des agents à jour), modèle de PR, chiffres (24 workflows, 180 skills). docstest vert. Ni le CHANGELOG ni MIGRATION ne citent `oh workflow migrate` (inexistant, cité à tort dans le plan X4 et entrees-lots ; la migration `[workflow]` tourne au démarrage). Restes v5.1 : `oh doctor --fix`, `oh gc` refusent sans terminal en code 1 (pas 2) ; texte français en dur de `oh service` (`cmd/misc.go:59-78`) ; SECURITY « verified on opencode 2.0.20 » non revérifié sur 2.0.25. |
| 22/10/2026 | V4 : contrôles GoReleaser (orchestrateur) | `goreleaser check` : configuration valide, seul avertissement `brews` déprécié (gardé pour la 5.0, décision utilisateur). `goreleaser release --snapshot --clean --skip=sign,publish,announce` dans le worktree de test sur `7c5e361f` : succès en 39 s, 4 archives (darwin/linux × amd64/arm64), formule Homebrew générée localement ; binaire darwin_arm64 lancé avec HOME/OH_HOME temporaires (`version`, `--help` compact). **Attention** : la signature Cosign n'est pas sautée par `--snapshot` seul ; toujours passer `--skip=sign` en local (sinon signature sans clé avec envoi au journal public). |
| 22/10/2026 | **V4 terminée — arrêt avant la mise en production** | V4-a (`oh purge` n'arrête plus que le démon de son hub (socket) et les serveurs d'outil enregistrés (adaptateur, identifiants vérifiés) ; plus de `pkill` ; le reste listé « à arrêter à la main » ; `release-notes.sh` réécrit les liens relatifs en URL absolues `blob|raw/<tag>/…`, testé sur awk macOS seulement). V4-b (`docs/dev/release-checklist.md`, lié depuis CONTRIBUTING et contributing FR/EN ; D-03 corrigé sur 12 pages ; `audit-v5.md` : **63 FIXED / 18 PARTIAL / 2 OPEN** (avant 52/19/12), OPEN restants F-03 (bêta assumée) et F-11, N-2 ; section « Lots du 22/10 (v5.0) »). Entrées CHANGELOG de V4-a ajoutées par l'orchestrateur. Vérification finale sur `main` : vet 0, lint 0 issue, `go test -short ./...` vert, notes de release sans lien relatif. `main` : 109 commits d'avance sur `github/main`, rien de poussé. **Reste à l'utilisateur** : push, CI 3 OS (surveiller `release-notes.sh` sous awk Linux au premier tag), validations manuelles (évals, e2e, mode équipe, plugins S-11 — format de config d'opencode —, `oh init` 3 écrans et défaut « solo », jeton GitLab d'équipe pour `oh mr create`), date de `[5.0.0]` au jour du tag, tag `v5.0.0` (rc possible : pré-version, sans Homebrew). |
| 22/10/2026 | CI rouge analysée et corrigée (`f0766ecb`) | Tous les runs de `main` (et les PR Dependabot) échouaient au scan `govulncheck` (GO-2026-6809 `crypto/x509` sur Go 1.26.9 ; Windows en 1.26.8 avec en plus `crypto/tls`, `net/http`, `os.Root`), **avant les tests** : aucun test n'avait tourné en CI sur le code du 21-22/10 (dernier run vert de main : 06/07 ; dernier run avec tests : 20/10, échecs corrigés depuis par ci-fix-2 mais jamais vérifiés). Correctif : `toolchain go1.26.10` dans `cli/go.mod`, `go-version-file: cli/go.mod` dans ci.yml (×2) et release.yml, scan déplacé après les tests (`if: !cancelled()`), scan ajouté à la release avant GoReleaser, entrée CHANGELOG Security. Local avec Go 1.26.10 : build, vet, lint 0, `go test -short ./...` vert, `govulncheck` : 0 vulnérabilité appelée. Note : le téléchargement de la toolchain dans `~/go/pkg/mod` est refusé dans l'environnement de l'orchestrateur (« operation not permitted ») → fait avec un GOMODCACHE temporaire ; si l'utilisateur rencontre la même erreur : `brew upgrade go`. Prochaine étape : push par l'utilisateur, premier vrai passage des tests sur les 3 OS. |
| 22/10/2026 | 1er vrai passage des tests en CI (`f0766ecb`) et correctifs (`bebf872c`) | Scan de vulnérabilités vert sur les 3 OS (Go 1.26.10). macOS **vert** (13 min 55), intégration verte. Ubuntu : 7 tests `services/policy` (V1-C) sans identité git sur le runner → TestMain `testenv.Isolate`. Windows : (1) « the tests changed the real hub » sur app, mcpregistry, mcp/team, services/remote, remote/runner : `testenv.Isolate` et `configtest.Home` ne fixaient que HOME, Windows lit USERPROFILE (suivi É.3) → USERPROFILE fixé aussi ; (2) fanout : session factice qui écrivait encore dans le dépôt pendant sa suppression → `wg` + `quit` dans le faux lanceur (attente d'1 h de `clean_test` rendue interruptible) ; (3) sessionresults : `a..`/`...` (Windows retire les points finaux, `...` pouvait désigner le parent) → id finissant par un point ou une espace refusé partout. Vérification locale : suite `-short` complète dans un environnement de runner simulé (HOME vide, `GIT_CONFIG_GLOBAL=/dev/null`) **verte** ; fanout, sessionresults, policy, testenv, config sous `-race` verts. « La CI tourne en boucle » : chaque push sur main fait relancer par Dependabot ses PR qui touchent les workflows (rebase), d'où de nouveaux runs ; pas de boucle réelle. |
| 22/10/2026 | CI : macOS et Ubuntu **verts** ; Windows : 1 échec (`bebf872c`) | `tui/v2/shell TestLayoutTooSmall50x12` : DATA RACE entre la lecture de l'écran simulé par le test (`GetContents` depuis la goroutine du test) et son dessin par la boucle tview ; course du test, pas du code ; vue seulement sous Windows (ordonnancement). Corrigé (`d’après fusion`) : `screenText`, `screen` (help_test) et l'écran de keys_test lisent ou copient les cellules sur la boucle. `shell` sous `-race` ×5 vert, lint 0. L'utilisateur n'utilise pas Windows pour l'instant : si un autre échec propre à Windows apparaît, le paquet passe dans `.github/windows-test-packages-excluded.txt` avec sa cause plutôt que de bloquer. |
| 22/10/2026 | CI `899c2c6b` : **Windows vert**, Ubuntu vert, intégration verte ; macOS : 1 échec instable | `daemon TestWatcherCheckpoints` (vert sur les 2 runs macOS précédents) : le test lisait `Waiting` juste après la disparition de la décision, alors que ce sont deux écritures → attente des deux (`Eventually`), `-race` ×10 vert. Fusionné. |
| 22/10/2026 | `main` vert sur les 3 OS (`33e4cd93`) ; PR Dependabot rebasées | #4 cosign-installer 4.1.2, #11 x/crypto 0.58.0, #14 goreleaser-action 7.3.0 : **CI verte** (5/5). #2 setup-go 7.0.0 : Go 1.26.10 bien lu depuis la directive `toolchain` (vérifié dans les journaux des 4 PR) ; 1 échec Ubuntu `daemon TestQueuedSessionsStartWhenASlotFrees`, instable et sans lien avec la PR : le test posait l'état « actif » en base pendant que le watcher pouvait encore réécrire « inactif » → il attend désormais que le watcher l'ait enregistré (`0586d007`, `-race` ×30 et ×10 `-cpu 1,4` verts). PR restantes pour après la 5.0 : #9 bubbletea, #10 x/sync, #12 sqlite, #13 x/term. |
| 22/10/2026 | PR Dependabot fusionnées par l'orchestrateur (demande de l'utilisateur) | #4 cosign-installer 4.1.2 (`beffaddc`), #11 x/crypto 0.58.0 (`6eb826f7`, entraîne x/term 0.47, x/sync 0.24, x/text 0.43, x/sys 0.49), #14 goreleaser-action 7.3.0 (`bddc7433`), en commit de fusion. Rebase de #2 (setup-go 7.0.0) redemandé. Local sur `bddc7433` (Go 1.26.10) : build, vet, `govulncheck` 0 vulnérabilité, `goreleaser release --snapshot --skip=sign,publish,announce` réussi (1 min 35). |
| 22/10/2026 | CI de `main` après les fusions Dependabot (`bddc7433`) : macOS, Windows, intégration verts, Ubuntu 1 échec instable | `procinfo TestGetChild` : arguments du fils lus avant son exec (vide) → attente (`Eventually`), `-race` ×30 vert. **Incident de l'orchestrateur** : une première version (`437c7cab`) a été commitée et fusionnée localement malgré un `go vet` en échec (commande enchaînée avec `;`) ; fusion annulée avant tout push (`reset --hard bddc7433`), correctif refait et vérifié avec `set -e`. PR #2 (setup-go 7.0.0) : CI 5/5 verte après rebase, MERGEABLE CLEAN. |
| 22/10/2026 | PR #2 setup-go 7.0.0 fusionnée (`98291a2f`) ; correctif procinfo rebasé dessus (`26a111d9`, 2 commits à pousser). PR #10 et #13 fermées par Dependabot (dépassées par x/crypto). Restent #9 bubbletea et #12 sqlite pour la 5.0.1. |
| 22/10/2026 | **Tag `v5.0.0` posé et poussé** par l'orchestrateur (demande de l'utilisateur) sur `26a111d9` (CI verte sur les 3 OS, run 39809190138). Workflow « Release — GoReleaser » run 39811978780 en cours. |
| 22/10/2026 | **v5.0.0 publiée** | Workflow « Release — GoReleaser » run 39811978780 vert (5 min). Release GitHub `v5.0.0` publiée le 22/10 à 16:17 UTC, non pré-version, `releases/latest` = v5.0.0 ; 4 archives (darwin/linux × amd64/arm64), `checksums.txt` et `checksums.txt.sigstore.json`. Notes : section [5.0.0] avec liens absolus vers `blob/v5.0.0/…` (0 lien relatif ; `MIGRATION.md` répond 200). Vérifié en local : somme SHA-256 de `openhub_darwin_arm64.tar.gz` OK ; `cosign verify-blob` (identité `release.yml@refs/tags/v*`, émetteur GitHub Actions) **Verified OK** ; binaire : `oh 5.0.0`, commit `26a111d9`. Tap `datichb/homebrew-tap` : commit `47f22df6` « Brew formula update for openhub version v5.0.0 », formule en 5.0.0 avec les 4 URL et sommes ; `brew info datichb/tap/openhub` → stable 5.0.0 (non installé sur la machine). **D-01 réglé.** |
