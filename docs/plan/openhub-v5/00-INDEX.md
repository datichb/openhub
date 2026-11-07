# openhub v5 — Workflows multiples, paquets de session, exécution local/conteneur/distant

> Plan de build consolidé issu de la session d'étude du 05/10/2026 (commit de référence `a46f49b`, opencode **v2.0.20**).
> Ce dossier est la **source de vérité** pour les sessions de build. Chaque session de build doit commencer par lire ce fichier, puis le fichier de la phase concernée.

## Objectif en une phrase

Remplacer le workflow unique (exploité par « stratagèmes ») par des **workflows déclaratifs, extensibles et personnalisables** (YAML, stockés dans `team-state`), exécutés dans des **sessions isolées** (« monde fermé » : seuls les agents/skills du workflow sont visibles), en **local, conteneur (Colima/Podman) ou distant (GitLab CI)**, avec une logique qui reste dans `oh` et un **adaptateur** par outil (opencode V1/V2 aujourd'hui, autre demain).

## Fichiers

| Fichier | Contenu | Quand le lire |
|---|---|---|
| `00-INDEX.md` | Ce fichier : contexte, ordre, conventions de build, suivi | Toujours |
| `01-decisions.md` | Toutes les décisions validées (D*, O*, I*, S*, T*) | Toujours |
| `02-findings-opencode-v2.md` | Résultats des tests réels sur opencode 2.0.20 + recettes reproductibles + formes des payloads API | Phases 0, 3, 4, 5 |
| `03-architecture.md` | Couches, services, interfaces Go, modèles de données, schéma YAML, layout du paquet, migrations DB | Toujours |
| `04-phase-0.md` | Remettre en marche sous V2 + fondations | Phase 0 |
| `05-phase-1.md` | Workflows multiples déclaratifs | Phase 1 |
| `06-phase-2.md` | team-state, publication, solo | Phase 2 |
| `07-phase-3.md` | Paquet par session, checkpoints, console Sessions, suppression du déploiement | Phase 3 |
| `08-phase-4.md` | Conteneur local | Phase 4 |
| `09-phase-5.md` | Exécution distante GitLab CI + mise en place des runners | Phase 5 |
| `10-tui.md` | Spécification complète de la TUI (maquettes, vue par vue) | Toute phase touchant la TUI |
| `11-session-interaction.md` | Interaction utilisateur avec les sessions, multi-session, cycle de vie | Phases 0, 3 |
| `12-backlog-adr.md` | Backlog (BL-*) + liste des ADR à rédiger | Fin de phase |
| `13-codebase-map.md` | Carte du code actuel (fichiers:lignes), bugs relevés, éléments à supprimer | Toujours (référence) |
| `14-finalisation.md` | Vague 4 : anomalies, ADR 038–048, documentation, sortie v5 (pistes Q, ADR, DOC, REL + prompts) | Finalisation |
| `15-tests-manuels.md` | Plan de tests manuels de la release v5 (19 catégories, priorités, cases à cocher, suivi des anomalies) | Avant REL |
| `16-corrections.md` | Piste de corrections après la recette (A1–A43) : 4 pistes parallèles T (workflow ticket fiable), L (lancement et sessions), C (CLI, Doctor, moteur), U (TUI et fenêtres), non-régression, prompts | Après la recette |
| `18-recette-critiques.md` | Recette des points critiques N1–N27 et non-régression v5 (13 sections, P1/P2/P3, ⚠️ jamais fait en réel, suivi et anomalies `B<n>`) | Après la fusion de `17`, avant REL |
| `17-critiques.md` | Points critiques des audits du 08/10 et du 09/10 (N1–N27, dont l'audit parallèle `docs/dev/audit-v5.md` et le lot F0 de `feat/v5-fix-legacy-skills`) : décisions DC1–DC27, lot 0 (sondes opencode), pistes S (sécurité), G (passerelles, proxy), W (workflows, contenu, superposition), D (distant), R (robustesse), X (touches, petits terminaux), Rel (hygiène de release), non-régression, prompts | Après la fusion de C et U, avant REL |

### Points critiques des audits (planifiés le 09/10/2026)

Voir **`17-critiques.md`** (N1–N27, décisions DC1–DC27 validées, § Avancement) et la recette **`18-recette-critiques.md`**. Audit parallèle : `docs/dev/audit-v5.md` (dépôt). Pistes F0, Rel, W0, S, G, W, D, R, X et finitions **toutes fusionnées** dans `main` (`890c01da`, 09/10, **non poussé**) ; worktrees supprimés. **Reprise** : lire `17-critiques.md` § Avancement (dont « Points ouverts pour la prochaine session »), puis dérouler `18-recette-critiques.md`.

## Ordre des phases et dépendances

```
Phase 0 (V2 + fondations) ──▶ Phase 1 (workflows) ──┬──▶ Phase 2 (team-state)
                                                     └──▶ Phase 3 (paquet session, checkpoints, Sessions, suppression deploy)
                                                                  │
                                                                  ▼
                                                           Phase 4 (conteneur) ──▶ Phase 5 (distant CI)
```
- Phases 2 et 3 parallélisables après la phase 1.
- Phase 0 est **bloquante** : tous les lancements `oh` sont cassés sous opencode V2 (voir `02`, F1).

## Conventions pour les sessions de build

1. **Lire** `00`, `01`, `03`, `13`, puis le fichier de phase. Lire `02` pour tout ce qui touche opencode.
2. **Une session de build = un lot de tâches d'une phase** (ex. `P0-T03..P0-T05`). Ne pas déborder sur une autre phase.
3. **Respecter les conventions du dépôt** : Go dans `cli/`, i18n obligatoire (`cli/internal/i18n/locales/{fr,en}.json`, parité testée), tests à côté du code, docs bilingues (`*.fr.md` / `*.en.md`).
4. **Pas de logique métier dans `cli/cmd/tui_*.go`** : tout passe par les services (`03`, §2).
5. **Aucun nom d'agent opencode codé en dur hors de l'adaptateur** (règle « monde fermé », D13).
6. **Tests** : unitaires + golden (paquets générés) + tests de contrat contre un vrai `opencode serve` (sans LLM) derrière un build tag `integration` ; e2e LLM (Haiku) derrière le tag `e2e`.
7. **Mettre à jour la case de suivi** ci-dessous et le CHANGELOG **à la fusion dans `main`** (pas dans les branches de piste, voir « Règles de fusion », 4).
8. **Ne jamais écrire dans les fichiers utilisateur d'un projet** (`AGENTS.md`, `opencode.json` utilisateur…) hors de `oh migrate deploy-cleanup` (avec diff + confirmation).
9. Modèle LLM pour les tests e2e : `amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0`, région `eu-west-1` (la politique AWS refuse les profils non-UE).

## Prompt type pour démarrer une session de build

```
Contexte : dépôt /Users/benjamin.datiche/workspace/openhub (CLI Go `oh`).
Plan de référence : ~/.opencode/plan/openhub-v5/.
Lis 00-INDEX.md, 01-decisions.md, 03-architecture.md, 13-codebase-map.md,
puis <0N-phase-X.md> (et 02/10/11 si la phase touche opencode, la TUI ou les sessions).
Réalise les tâches <PX-Txx à PX-Tyy>. Respecte les conventions du §« Conventions » de 00-INDEX.
Avant de coder : vérifie les références fichier:ligne (elles datent du commit a46f49b ; la phase 0 a été fusionnée en `d404c012` : nouveaux paquets `adapters/opencodev2`, `bundle`, `credproxy`, `daemon`, `runsvc`, `sessionspec` — voir `04-phase-0.md` § écarts).
À la fin : tests verts, i18n fr/en, CHANGELOG, et mets à jour le tableau de suivi de 00-INDEX.md.
```

## Parallélisation (après la phase 0)

Le schéma `oh/v1` est **figé** (P1-T01, `cli/internal/workflow/schema.go`, `03` §3). Toute évolution doit être **additive** ; un changement incompatible se discute d'abord (toutes les pistes en dépendent).

### Pistes

| Piste | Worktree / branche | Tâches | Dépend de | Vague |
|---|---|---|---|---|
| **A. Moteur** ✅ fusionnée (`2cc5152a`) | ~~`../openhub-p1-engine` · `feat/v5-p1-engine`~~ (supprimés) | P1-T02 → T06, puis T11, T12, T13 | P1-T01 ✅ | 1 |
| **B. Briques** ✅ fusionnée (`d481af35`) | `../openhub-p1-bricks` · `feat/v5-p1-bricks` (supprimés) | P1-T07 → T10, T21, T23 | phase 0 | 1 |
| **C. Pilotage des sessions** ✅ fusionnée (`c560ef5e`) | ~~`../openhub-p3-sessions` · `feat/v5-p3-sessions`~~ (supprimés) | P3-T07 → T13, T15, T16, T18, T19, T21, T22 | phase 0 | 1 |
| **D. Conteneur** ✅ fusionnée (`83b0980d`) | ~~`../openhub-p4-container` · `feat/v5-p4-container`~~ (supprimés) | P4-T01 → T06 | phase 0 | 1 |
| **E. Workflows livrés** ✅ fusionnée (`bfff3401`, `0e7626fa`) | ~~`../openhub-p1-content` · `feat/v5-p1-content`~~ (supprimés) | P1-T14, T15 + reprise des mentions « Mode A–E » | A ✅ | 2 |
| **F. CLI + TUI phase 1** ✅ fusionnée (`038fb9fb`) | ~~`../openhub-p1-launch` · `feat/v5-p1-launch`~~ (supprimés) | P1-T16 → T20, T22, T24 → T28 + reprises d'audit phase 0 (`05` : `--recap`, double lancement, `--location`, `parseVersion`) | A ✅, B ✅ ; E pour les alias (T17) | 2 |
| **G. Checkpoints** ✅ fusionnée (`2270ae2b`) | ~~`../openhub-p3-checkpoints` · `feat/v5-p3-checkpoints`~~ (supprimés) | P3-T01 → T06, puis T17 ; T20 couvert par la fiche de F (P1-T22) | A ✅, C ✅ | 2 |
| **H. Passerelles** ✅ fusionnée (`df4a8f51`, CHANGELOG `b890679a`) | ~~`../openhub-p4-gateways` · `feat/v5-p4-gateways`~~ (supprimés) | P4-T07 (passerelle Beads, faux `bd` `cmd/oh-bd` à la place du bouchon `cmd/oh-bd-stub`) ; P4-T08 (passerelle MCP HTTP pour les serveurs MCP oh du paquet) ; en plus : `StartRequest.BeadsAllow` branché au lancement (`cmd/v5_run.go`, à la place de F) | C ✅, D ✅, G ✅ | 2 |
| **P2. team-state** ✅ fusionnée (`53955aa9` ; lot 2.D : piste 2.D, `0d479d95`) | ~~`../openhub-p2-teamstate` · `feat/v5-p2-teamstate`~~ (supprimés ; le lot 2.D partira d'une nouvelle branche) | P2-T01 → T12 (stockage, lock, `enforce`, gouvernance, solo, promote, brouillons, publication, impact, historique, catalogue d'équipe, CLI, migration v38) ; **lot 2.D** (TUI : catalogue modifiable, éditeur, publication, assistant solo) en vague 3 | A ✅, E ✅, F ✅ | 2 |
| Annexes | hors code | ~~P1-T29 (S8)~~ (Q4 + QB8), ~~ADR 038/044/047~~ (ADR 038 → 048 rédigés par la piste ADR, vague 4, fusion `f4d9dc8d`) (P5-T01 → T03 faits par la piste P5) | — | à tout moment |

### Vague 3 (lancée le 06/10/2026, après la fusion de toutes les pistes de la vague 2)

**Décision** : V1 abandonné (D3 révisé, P3-T30). Recettes réelles réparties dans les pistes (lot 0 de 3.E, fin de 2.D et de 4.C).

| Piste | Worktree / branche | Tâches | Dépend de |
|---|---|---|---|
| **2.D. TUI team-state** ✅ fusionnée (`0d479d95`, CHANGELOG `6d5fbdd7`) | ~~`../openhub-p2-tui` · `feat/v5-p2-tui`~~ (supprimés) | P2-T13 → T17 + points non branchés de `06` § « Reste à faire » (brouillons et intégrité dans `oh workflow list` et le catalogue, `validate` avec les briques d'équipe, fichier d'équipe illisible classé dans sa couche, rejeu automatique de la file hors ligne) ; recette réelle phase 2 faite (`--draft`, briques d'équipe, TUI sous pty) | — |
| **4.C. Exécution** ✅ fusionnée (`e897b7e5`, CHANGELOG `6820ca7b`) | ~~`../openhub-p4-exec` · `feat/v5-p4-exec`~~ (supprimés) | P4-T09 → T12 ; recette réelle phase 4 faite (critère 1 en Terminal.app : iTerm2 absent de la machine ; `bd` par un agent et ses sous-agents, Colima et Podman ; MCP `workflow` distant ; gitlab/team non testés, faute de jetons) | — |
| **R. Durcissement** ✅ terminée — lots R1 → R6 fusionnés (R1 `916e3fad` + CHANGELOG `fc594b61` ; R2 → R5 `d375592d` + CHANGELOG `1983878d` ; R6 `b9ba6b8f` + CHANGELOG `e3ebab19`) | ~~`../openhub-hardening` · `feat/v5-hardening`~~ (supprimés) | P3-T14 (I6) + reprises d'audit de `07` (proxy, démon, isolation par agent, vérification de la fenêtre de fermeture) et de `08` (M12, port du proxy et passerelles, Windows option A, tmux < 3.0) ; **signalés par 4.C** (`08` § Piste 4.C) : `Teardown` du runtime à la veille du démon (crochet `daemon.Options.Teardown`), démon arrêté (`IdleAfter`) pendant une longue construction d'image ; **signalé par P5** (BL-17) : crochet périodique du démon pour suivre les pipelines distants oh fermé (`remotesvc.Service.Track`) et entretenir le bail Beads ; rebaser sur P5 (`runsvc.StartRequest.SessionID`, `runsvc/adopt.go`, `opencodev2.setDetached`) — **fait** (rebase sur `6de2e646`) ; points signalés **faits au lot R6** : crochet `Teardown` à la veille, démon gardé actif pendant une construction d'image, crochet périodique de suivi distant (BL-17) | — |
| **3.E. Suppression du déploiement + V1** ✅ fusionnée (`ee629d4b`, CHANGELOG `4fb125b4`) | ~~`../openhub-p3-cleanup` · `feat/v5-p3-cleanup`~~ (supprimés) | Lot 0 : recette réelle phase 1 (`05` § Piste F) ; puis P3-T30 (abandon V1), P3-T23 → T29, **P2-T18** (vue Workflow historique, `config.WorkflowHubConfig`, `teamstate.WorkflowTeamConfig`, `domain.Project.WorkflowConfig`), migrations v36/v37 | — |
| **P5. Distant** ✅ fusionnée (`03b4d718`, CHANGELOG `6de2e646`) | ~~`../openhub-p5-remote` · `feat/v5-p5-remote`~~ (supprimés) | Lot 5.0 (P5-T01 → T03), 5.A, 5.B, 5.C (P5-T04 → T19), migration v39 ; **recette réelle à faire** dès que les runners sont installés (`09` § « Recette réelle ») ; **signalé à R** : crochet périodique du démon pour le suivi des pipelines oh fermé et le bail Beads | runners disponibles pour la recette |

Non lancés : ~~ADR 038–047 (+ 010/012/036)~~ (faits par la piste ADR de la vague 4, fusion `f4d9dc8d` ; statuts de 012, 016 et 036 appliqués par Q-bis, `fe9ed932`), ~~étude S8 (P1-T29)~~ (faite en Q4, option 2 mise en œuvre en QB8 : ADR-050), notes de version et `compatibility.json` final (3.E a posé le minimum 2.0 : `cli/internal/adapters/opencodev2/compatibility.json`, entrée `5.0` + défaut). **À valider** : ajout additif `entry.selectable` au schéma `oh/v1` (workflow `libre`, 3.E).

### Migrations SQLite réservées (fusionnées : v32 à v43 ; aucune réservée)

| Version | Contenu | Piste |
|---|---|---|
| v32 | `preferences` (épinglés, réglages UI) — ✅ fusionnée | B (P1-T21) |
| v33 | `pending_decisions` — ✅ fusionnée | C (P3-T07) |
| v34 | colonnes `sessions` : `workflow_layer, workflow_version, workflow_risk, location, outputs, parent_session_id` — ✅ fusionnée | F (P1-T16/T27) |
| v35 | colonne `sessions.checkpoint_state` — ✅ fusionnée | G (P3-T03) |
| v36 / v37 | neutraliser puis supprimer `projects.agents` — ✅ fusionnées | 3.E (P3-T25) |
| v38 | `projects.workflow_config` → `workflow_config_legacy` (neutralisée), migrée vers le team-state au démarrage (+ `[workflow]` du `config.toml` d'équipe et de `hub.toml`) — ✅ fusionnée | P2 (P2-T11) |
| v39 | colonne `sessions.remote_ref` (référence d'une session distante, JSON `domain.RemoteRef`) — ✅ fusionnée | P5 (P5-T08) |
| v40 | registre d'usage : `usage_sessions` (par session outil et jour, `root_id` = session oh), `usage_proxy` (par groupe et jour), `budget_extra` (relèvements) — ✅ fusionnée | R (P3-T14) |
| v41 | colonne `projects.exec_config` (config d'exécution du projet, JSON) — ✅ fusionnée | 4.C (P4-T09) |
| v42 | colonnes `sessions.start_ref` (point de départ git de la session, A23) et `sessions.tickets` (tickets de la session, A31) ; coût des sessions existantes = total du registre d'usage (A21) — ✅ fusionnée (`a3fbea34`) | L (corrections v5) |
| v43 | colonnes `sessions.worktree_auto`, `sessions.worktree_branch` + index de nettoyage (`17` N16) — ✅ fusionnée (`23e64493`) | R (points critiques) |
Besoin d'une autre migration : prendre le prochain numéro libre **et** l'inscrire ici avant de coder. Une piste qui fusionne avant une autre de numéro inférieur garde son numéro (migrations indépendantes, appliquées dans l'ordre).

### Espaces de clés i18n par piste

| Piste | Préfixes |
|---|---|
| A | `cmd.workflow.validate.*`, erreurs `workflow.*` |
| B | `tui.picker.beads.*`, `tui.prefs.*`, `cmd.skill.*` |
| C | `cmd.session.*`, `tui.sessions.*`, `tui.inbox.*`, `tui.notify.*` |
| D | `cmd.runtime.container.*`, `tui.doctor.container.*` |
| E | (contenu YAML) ; diagnostics `workflow.diag.plan_*`, `workflow.diag.precondition_*`, `workflow.diag.patch_unknown_precondition` |
| F | `cmd.run.*`, `cmd.bundle.*`, `cmd.workflow.list.*`, `cmd.workflow.show.*`, `cmd.alias.*`, `tui.start.*`, `tui.launch.*`, `tui.catalog.*` |
| G | `cmd.mcp.workflow.*`, `tui.checkpoint.*` |
| H | `cmd.gateway.beads.*`, `cmd.gateway.mcp.*` (fusionnés) |
| P2 | `cmd.workflow.{new,edit,diff,publish,history,restore,archive}.*`, `cmd.team.solo.*`, `cmd.team.promote.*`, `teamstate.workflow.*` ; en plus (fusionnés) : `workflow.diag.enforced_field(.hint)`, `workflow.diag.enforce_unknown_field` (préfixe de A), `cmd.run.flags.draft` modifiée, `cmd.run.draft_unsupported` supprimée |
| 2.D | `tui.catalog.edit.*`, `tui.editor.*`, `tui.publish.*`, `tui.history.*`, `tui.solo.*`, `tui.team.governance.*` |
| 4.C | `tui.pc.exec.*`, `tui.settings.exec.*`, `tui.launch.runtime.*`, `tui.doctor.container.*`, `cmd.doctor.gateway.*` |
| R | `cmd.proxy.*`, `cmd.daemon.*`, `cmd.budget.*`, `tui.budget.*`, `cmd.doctor.security.*` |
| 3.E | `cmd.migrate.cleanup.*`, `tui.cleanup.*`, `tui.bricks.*`, `cmd.deploy.removed.*`, `cmd.v1.unsupported.*` |
| P5 | `cmd.remote.*`, `cmd.runner.*`, `cmd.session.fetch.*`, `cmd.session.resolve.*`, `tui.remote.*`, `tui.settings.remote.*` |

### Règles de fusion

1. Chaque piste rebase **souvent** sur `main` ; fusion dans `main` à la fin de chaque lot (tests verts, `make test-integration` si l'adaptateur est touché).
2. **Ordre conseillé** : vague 1 = A (T02–T05) en premier, puis B, C, D. Vague 2 = **E au plus tôt** (au moins `ticket`, `feature`, `quick` : corrige l'absence d'aiguillage de `oh start`), puis F lot 1.C, puis G, H, P2 dans l'ordre où ils finissent.
3. Fichiers sensibles (conflits fréquents) : `internal/bundle/` (paquet réel ; pas `internal/services/bundle/`), `cmd/tui_session_actions.go`, `internal/tui/v2/shell/shell.go`, `internal/storage/sqlite/store.go`, `locales/{fr,en}.json`, `CHANGELOG.md`. Garder les modifications de ces fichiers petites et isolées.
4. **Le tableau « Suivi d'avancement » et le CHANGELOG sont mis à jour à la fusion**, pas dans les branches de piste.
5. **Vague 2 — propriété des fichiers partagés** :
   - `oh workflow` : `cmd/workflow.go` ne contient que la commande racine ; **chaque sous-commande s'enregistre dans son propre fichier** (`func init()` dans `cmd/workflow_<sub>.go`). F : `list`, `show` ; P2 : `new`, `edit`, `diff`, `publish`, `history`, `restore`, `archive`.
   - `internal/services/workflow/` : **créé par F** (`Catalog`, `Resolve` sur les couches disponibles). P2 commence par le stockage `internal/teamstate/workflows*.go`, puis ajoute ses méthodes (`SaveDraft`, `Publish`, `History`, `Restore`, `Impact`) **dans des fichiers séparés** après rebase sur F.
   - `internal/bundle/` : F branche `Request.Spec` au lancement (`cmd/v5_launch.go`) ; G ajoute ses règles de checkpoint (P3-T02) dans un **fichier séparé** (`bundle/checkpoints.go`) ; H lit `beads.allow` sans modifier l'assemblage.
   - Workflows YAML (`workflows/`, gabarits) : **propriété de E**. F et G utilisent des workflows de test sous `testdata/` tant que E n'a pas fusionné.
   - Plugin oh (`adapters/opencodev2/plugin`) : G seul (niveau 3) ; E (P1-T15) ne touche que le rendu (`plugins:`, `code_mode:`).
6. **Vague 3 — propriété des fichiers partagés** :
   - **3.E** est seul à supprimer du code ancien (déploiement, lancement V1, P2-T18, clés i18n orphelines). Les autres pistes ne suppriment rien hors de leur périmètre et signalent le code mort à 3.E.
   - **R** est propriétaire des internes du démon, du proxy (`credproxy`) et de l'authentification des passerelles (`gateway`) ; **P5** les réutilise sans les modifier (mode runner) et rebase après chaque lot de R. P5 est propriétaire du faux `bd` (`cmd/oh-bd`, mode journal).
   - TUI : **2.D** = catalogue (`views/workflows_view.go`), éditeur, publication, historique, assistant solo, détail d'équipe ; **4.C** = config projet (section Exécution), réglages Exécution, option conteneur de la fiche, Doctor conteneur ; **3.E** = mode projet (section Déploiement), omnibar `deploy`/`sync`, Catalogue des briques (`project.agents`), écran de nettoyage ; **P5** = option ☁ de la fiche, section « À récupérer », réglages Distant. Pour la fiche de lancement (`InlineWizardView`), 4.C et P5 ajoutent leur option dans des fichiers séparés.
   - `cmd/doctor*.go` : une vérification par fichier (4.C conteneur/passerelles, R sécurité, 3.E V1/restes de déploiement, P5 distant).
   - Ordre de fusion conseillé : R (correctifs démon/proxy) au plus tôt, puis 2.D, 4.C, 3.E dans l'ordre où ils finissent, P5 en dernier. **Réalisé** : R1, 2.D, 4.C, P5, puis R2 → R5, R6 et enfin 3.E (rebasée sur R6, fusion `ee629d4b`).

### Prompt pour une session de piste

```
Contexte : dépôt openhub (CLI Go `oh`), worktree <chemin>, branche <branche>.
Plan de référence : ~/.opencode/plan/openhub-v5/.
Lis 00-INDEX.md (dont « Parallélisation »), 01-decisions.md, 03-architecture.md, 13-codebase-map.md,
puis <fichier de phase> (et 02/10/11 si la piste touche opencode, la TUI ou les sessions).
Piste <X> : réalise <tâches>, un lot à la fois, avec un point de contrôle après chacun.
Migrations réservées : <vXX>. Clés i18n : <préfixes>. Schéma oh/v1 figé : évolutions additives uniquement.
Ne modifie ni le tableau de suivi de 00-INDEX ni le CHANGELOG (faits à la fusion) ;
note tes écarts dans le fichier de phase, section « Écarts constatés pendant le build ».
Rebase régulier sur main. Fin de lot : tests verts, i18n fr/en (parité), commit.
```

### Vague 4 — finalisation (planifiée le 06/10/2026)

Voir **`14-finalisation.md`** : pistes **Q** (anomalies), **ADR** (038–048), **DOC** (documentation) en parallèle, puis **Q-bis** (§5, « 100 % propre », lancée le 07/10), puis **REL** (sortie v5) seulement quand Q-bis est fusionnée et `main` vérifié. **Q-bis fusionnée et `main` vérifié le 07/10 (`fe9ed932`) : REL peut démarrer.** Prérequis : pousser `main` (non poussé).

| Piste | Worktree / branche | Statut |
|---|---|---|
| **Q** ✅ fusionnée (`e60a7038`, `main` **non poussé**) | ~~`../openhub-fixes` · `feat/v5-fixes`~~ (supprimés) | Lots Q1 → Q4 (§1) ; écarts : `14` § Écarts › Piste Q |
| **ADR** ✅ fusionnée (`f4d9dc8d`, `main` **non poussé**) | ~~`../openhub-adr` · `feat/v5-adr`~~ (supprimés) | ADR-038 → 048 fr/en (lots ADR-1 `261a9f08`, ADR-2 `c459670a`, ADR-3 `d83fda15`) ; statuts de 003, 006, 008, 010, 011, 014, 018, 019, 021, 024, 029, 033 mis à jour ; ~~à trancher : 012, 016, 036~~ (tranchés le 07/10, appliqués par Q-bis QB6) ; CHANGELOG non mis à jour |
| **DOC** ✅ fusionnée (`ab4d7cbe`, `main` **non poussé**) | ~~`../openhub-docs` · `feat/v5-docs`~~ (supprimés le 07/10) | DOC1 `83acacc6`, `5f3c4f3d`, `82d30cd2` ; DOC2 `66df17e2`, `9e5bd349` ; DOC3 `cf356b6c`, `2603cc30`, `370b3223`, `852d14e0`, `c8b78886` ; DOC4 `8729ba48` — guides, référence (couverture de `oh --help`, nouvelle page `reference/workflow-schema.*`), architecture (overview + tableau des ADR 038–048), diagrammes v5 (SVG mmdc, + `session-lifecycle`, `execution-topology`), README, docs/README, SECURITY (5.x seule supportée, confirmé le 07/10), wiki, docs/dev « historique », limites connues v5.0 ; parité CI et liens ✔ (sauf `adr/020-*`, piste ADR) ; constats pour Q et REL : `14` § Écarts › Piste DOC ; CHANGELOG non mis à jour |
| **Q-bis** ✅ fusionnée (`fe9ed932`, `main` **non poussé**) | ~~`../openhub-qbis` · `feat/v5-qbis`~~ (supprimés le 07/10) | QB1 `2c547970`, `2d7795b3`, `fea52e54`, `42f7ec83` ; QB2 `7b14a0a2`, `8926b96e`, `5104f944`, `d84036ab`, `eaeecbe9`, `be4014b4`, `2c2e8366`, sources neutres des entrées `from:` `472cdab7` ; QB3 `f5da2cb4`, `2d94d8df`, `b1797edb`, `dee0451f`, `125a93aa`, `0faa3f70` ; QB4 `f494b5da`, `66da9b1d` ; QB5 `aac2d4d2` ; QB7 `8d146591`, `bde7a2a7` (D19, ADR-049) ; QB8 `fe512389`, `5d4241af` (S8 option 2, ADR-050) ; QB6 `aed3dbd8`, `3dff7fab`. Aucune migration. `main` vérifié après fusion (build, vet, tests). Changements visibles et ruptures pour REL : `14` § Écarts › Piste Q-bis (entrée QB6) ; CHANGELOG `[Unreleased]` à jour |

### Corrections après la recette (planifiées le 08/10/2026)

Voir **`16-corrections.md`** (anomalies A1–A43 de `15` ; écarts par piste dans `16` § Écarts constatés). Ordre de fusion conseillé : T et L, puis C, puis U ; ensuite recette de non-régression, revue ergonomie/design de la TUI, REL.

| Piste | Worktree / branche | Statut |
|---|---|---|
| **T. Workflow `ticket` fiable** ✅ fusionnée (`69c15431`, T4 `719b70ba`, T5 `f81fc782`, 08/10, `main` **non poussé**) | ~~`../openhub-fix-ticket` · `feat/v5-fix-ticket`~~ (supprimés le 08/10) | T1 `17fa15cd` (A17, A36, A38 : champ additif `oh/v1` `checkpoints.unlocks`, verrou commit/push/fermeture par règles de session et passerelle Beads, imitation de checkpoint fermée par oh, fin de workflow → `completed` + sorties), T2 `27e73e5c` (A19, A16 : hooks de Beads par la passerelle, contournements refusés, démarrage du shell d'oh + vérification par session et Doctor ; `developer-rw` aligné sur le garde-fou git de L), T3 `5803a692` (A20, A27 : outils Python du projet, aucune installation système ; demande de l'utilisateur écrite telle quelle — **révision partielle d'O11, validée le 08/10**) ; pas de migration ; CHANGELOG `[Unreleased]` à jour ; `main` vérifié après fusion (build, vet, tests). T4 `cffdbac6` (demande de L : une session `completed` par la fin de son workflow passe par le crochet de fin du démon — réservations d'équipe terminées ou libérées, événement `session.complete` — ; un arrêt ultérieur ne le refait pas), T5 `6ebd7781` (review et annulation Beads en labels : bd 1.3 refuse les statuts `review` et `cancelled` ; consigne Python dans les étapes des agents de développement) ; constats : `16` § Écarts › Piste T |
| **L. Lancement et sessions** ✅ fusionnée (`a3fbea34`, `main` **non poussé**) | ~~`../openhub-fix-launch` · `feat/v5-fix-launch`~~ (supprimés le 08/10) | L1 `be7b16b4` (A18, A35), L2 `2c16962a` (A24), L3 `a661696c` (A21, A22, A23), L4 `852d8252` (A29, A31) ; migration v42 ; CHANGELOG `[Unreleased]` à jour ; `main` vérifié après fusion (build, vet avec tags, tests) |
| **C. CLI, Doctor, config, moteur** ✅ fusionnée (`7c05682e`, C4 `853a0157`, C5 `7df28ad1`, 08/10, `main` **non poussé**) | ~~`../openhub-fix-cli` · `feat/v5-fix-cli`~~ (supprimés le 08/10) | C1 `3d23c731` (A1, A2, A40 : comparaison semver avec pré-versions et builds `git describe`, mise à jour = avertissement, jamais de rétrogradation implicite ; identifiants selon la cascade du lancement ; registre Doctor unique CLI/TUI, `doctor_claims.go` de L et le contrôle du shell de T compris), C2 `2a6d8271` (A15, A3, A4, A11, A12, A13, A14, A25, A26, A28, A30, A5 : `oh config set|unset` des sections v5, team-states sous `OH_HOME`, `oh secrets set` sur l'entrée standard, cascade des modèles complète, erreurs traduites, pas d'assistant ni d'animation sans terminal, messages exacts, guide Beads zéro impact), C3 `b8ad29c8` + `9ac2022e` (A6, A7, A9, A10, A8 : `show`/`validate`/`bundle` jusqu'à la couche projet, `project:<id>` ; v38 archive les configurations inconnues ; validation du brouillon ; gabarit de départ pour un workflow sans prompt ; skills `plugin:` et frontend selon la pile) ; pas de migration ; CHANGELOG `[Unreleased]` à jour ; `main` vérifié après fusion (build, vet avec tags, tests) ; C4 `7c5abb23` (suite de A12 : messages d'`oh config`, `oh config model unset`, `oh review --publish` traduits ; suite de A5 : Doctor signale `CLAUDE.md`, `.cursor/`, `.claude/`, `.codex/`, `.agents/` et le bloc `.gitignore` de bd 1.3), C5 `df11d832` (Beads zéro impact nettoyé automatiquement à l'enregistrement d'un projet et à l'init de Beads, commit de `bd init` annulé s'il n'est pas poussé, `oh doctor --fix` ; `oh config list --all` ; permissions et consignes des plugins seulement avec le plugin, `permissions/plugin-tools.yaml` ; skills absentes signalées aux agents) ; constats : `16` § Écarts › Piste C |
| **U. TUI (fonctionnel) et fenêtres** ✅ fusionnée (`c3b36496`, 08/10, `main` **non poussé**) | ~~`../openhub-fix-tui` · `feat/v5-fix-tui`~~ (supprimés le 08/10) | U1 `84481aaf` (A43 : cause = onglets d'opencode mémorisés par dossier ; clients ouverts par oh sans onglets, capacité `adapters.SingleSessionClient` ; client déjà attaché ramené au premier plan, suivi des clients par le démon `GET /v1/clients`, `termlaunch.Focus`), U2 `93894563` (A37, A39, A41, A42, A33 fonctionnel) ; pas de migration ; CHANGELOG `[Unreleased]` à jour ; `main` vérifié après fusion (build, vet avec tags, tests) ; laissés à la revue design : mise en page du formulaire « Nouveau workflow », ASCII-art du nom d'équipe, libellés Patterns/Policies/Sync Tracker ; constats : `16` § Écarts › Piste U |

| Phase | Statut | Lots terminés | Notes |
|---|---|---|---|
| 0 | ✅ terminée, recette faite, **fusionnée dans `main`** (`d404c012`, poussée) | E1 (`ff4e9cf`), E2 (`cadc7ac`), E3 (`c211286`), E4 (`0b80dbb3`), E5 (`e3194407`), E6 (`4862ae1f`), E7 (`848b7845`), E8 (`6e6a81c2`), E9 (`2df6fbd9`), E10 (`165ac01a`), E11 (`984a3524`), E12 (`817ced64`), E13 (`c11cffad`), E14 durcissement (`084670c9`) | Paquets réels : `internal/sessionspec`, `internal/adapters` (interface), `internal/adapters/opencodev2` (au lieu de `adapters/opencode/v2`). Tests de contrat : `make test-integration` ; e2e LLM : `make test-e2e` (tags `integration e2e`) |
| 1 | ✅ terminée (toutes les pistes fusionnées : A, B, E, F) ; recette réelle faite (G : `oh run cadrage` ; 3.E lot 0 : `quick`, `ticket --tickets a,b --recap`, `brief-enrich --headless`, fiche de lancement sous pty, `07` § Piste 3.E) ; ~~reste P1-T29 (étude S8, annexe)~~ (faite : Q4, mise en œuvre QB8) | P1-T01 schéma `oh/v1` figé (`feat/v5-p1-schema`, fusionné dans `main`) ; **piste B** (P1-T07 `974ce301`, P1-T08 `9fa73071`, P1-T09/T10 `4aaf1dd9`, P1-T21 `e51f5e61`, P1-T23 `73418f61`) fusionnée dans `main` (`d481af35`) ; **piste A** (P1-T02..T05 `bfc03525`, P1-T06 `25c37aef`, P1-T11 `2c97ee40`, P1-T12 `55abd952`, P1-T13 `a48b2208`, intégration B `dcb2a03f`) fusionnée dans `main` (`2cc5152a`) ; **piste E** (P1-T14 : `0e3f5784`, `e9080527`, `b757e8c2` ; P1-T15 + modes A–E : `2e5ca819`) fusionnée dans `main` (`bfff3401`, `0e7626fa`) : 11 workflows livrés (`workflows/`, embarqués), `workflow.RenderPrompt`, ajouts `risk: plan` et `preconditions:`, auto-délégation explicite, `plugins:`/`code_mode:` appliqués au paquet, plus de modes A–E dans les skills ; **piste F** (`9ce5a284` semver, P1-T19 `f24b9a83`, P1-T18 `09b27489`, P1-T16 `6e0c19ff`, P1-T17 `8f45c639`, intégration E `0f0849cb`, lot 1.D `d6291723`, F3 `a960e004`, F4 `c11db7b8`, F5 `ac076ad7`) fusionnée dans `main` (`038fb9fb`, CHANGELOG `aeecdfee`) : `internal/services/workflow` (Catalog/Resolve/RenderPrompt/Chain, **couche hub seule** — P2 ajoute équipe/projet), `oh run` (multi-tickets, emplacements, worktrees O10, `--runtime container`, `--recap`, `--headless`), `oh bundle build|show`, `oh workflow list|show`, alias des anciennes commandes, migration v34, TUI « Démarrer », fiche de lancement, catalogue en lecture seule, « Enchaîner avec… » | Briques de B et moteur de A **branchés** au lancement par F (`Request.Spec`, `WorkflowModels`, `ExtraSkills/DenySkills`, `StrictIsolation`, `prefsvc`, sélecteur Beads dans la fiche). Migrations : au-delà de v31, une version absente est appliquée même après une plus récente (pistes indépendantes). Paquet : fichiers 0444, variable `{{oh.bundle}}` développée par l'adaptateur (`WithBundleRoot`) — chemin du paquet en local, point de montage `/opt/oh/bundle` en conteneur (`a90f7777`). `oh start` & co. = alias dépréciés de `oh run <wf>` (repli sur l'ancien lancement sans opencode V2 ou sans le workflow) ; `oh run --draft` disponible depuis la fusion de P2 (`53955aa9`). G écrit `workflow_outputs` dans `sessions.outputs` (v34) et confirme P3-T20 couvert par la fiche de F (`07` § Piste G, lot G4). Un worktree doit relancer `make embed-sync` (nouveau `hub/workflows`) ; `$TMPDIR/opencode` est partagé entre sessions : un sous-dossier par piste. `context-mode` (plugin V1) ne se charge pas sous V2 (`05` § Piste E). Écarts : `05-phase-1.md` |
| 2 | ✅ terminée (P2-T18 fait par 3.E), recette réelle faite, **fusionnée dans `main`** (lots 2.A–2.C `53955aa9`, CHANGELOG `08945947` ; lot 2.D `0d479d95`, CHANGELOG `6d5fbdd7` ; `main` **non poussé**) | **piste P2** : P2-T01/T02 `dfb9c4e8`, P2-T08 `361adfb4`, P2-T09/T10 `dc153246`, P2-T03→T07 `3a63e38a`, P2-T12 `052c3e8a`, P2-T11 `4f97e764` ; **piste 2.D** : lot 1 service/CLI `57cdfee7`, P2-T13/T15 `a6dc4497`, P2-T14 `07528d3f`, P2-T16/T17 `e0523678`, correctifs de recette `f11bc029` | Workflows d'équipe/projet dans le team-state, chargés par `WorkflowService` (`TeamState`) pour la CLI, la TUI et `oh run` ; `workflows.lock` + intégrité ; ajout additif `oh/v1` **`enforce:`** ; `config.TeamConfig.Solo` (`~/.oh/teams/<id>`, jamais équipe active, fonctions d'équipe coupées) ; transaction de publication `teamstate.Repo.Transact` (refaite après un push concurrent, file hors ligne `.git/oh-workflow-queue.json`, **rejouée à chaque synchronisation TUI**) ; catalogue de briques d'équipe fusionné dans `~/.oh/cache/bricks/` ; migration v38 **automatique** au démarrage. **2.D** : `Service.Workspace/Validate/NewDraftText/CheckText/Governance`, `workflow.DocEdit` (édition de l'arbre YAML), catalogue modifiable, éditeur 5 sections, Publication, Historique, assistant espace solo, « Passer en équipe », gouvernance dans `team.detail` ; widgets du graphe généralisés (`widgets.GraphModel`, l'ancien `ComputeLayout` n'est plus qu'un adaptateur pour la vue Workflow). **Pour 3.E** : P2-T18 (supprimer `views/workflow_view.go`, puis `ModelFromDefinition`/`ComputeLayout(WorkflowDefinition)` et le test `TestLegacyLayoutUnchanged` (`widgets/graph_test.go`, seul à utiliser `BaseWorkflow` côté widgets), `config.WorkflowHubConfig`, `teamstate.WorkflowTeamConfig`, `domain.Project.WorkflowConfig`) ; vue Équipes : **aucune clé `tui.teams.*` n'existe** (≈ 23 clés, affichage brut). Restent : BL-15 (invitation), BL-16 ; impact sans comparaison des permissions propres des agents ; `oh workflow new --file` avec un gabarit à créer. Écarts : `06-phase-2.md` |
| 3 | ✅ terminée (toutes les pistes fusionnées : C, G, R, 3.E ; `main` **non poussé**) | **piste C** (P3-T07/T12 `2c6bfb3c`, P3-T08/T09/T11 `daf49f4b`, P3-T10/T13 `16355ff0`, P3-T15 `8b3e35bc`, P3-T16/T18/T19 `f91e8289`, P3-T21/T22 `10c103e8`) fusionnée dans `main` (`c560ef5e`) ; **piste G** (P3-T01 `da5743d5`, P3-T02→T06 `8231712a`, P3-T17 `bd2e649c`, intégration E/F `e4c418ce`, `9e810c48`) fusionnée dans `main` (`2270ae2b`, CHANGELOG `6d1d6749`, `main` **non poussé**) | Migration v33 `pending_decisions` ; SessionService dans `internal/services/session` ; contrats pour la piste H (`runsvc.Service.SessionEnv`, réappliqué à chaque redémarrage du serveur). Piste G : migration v35 `sessions.checkpoint_state` ; serveur MCP `workflow` (`{{oh.bin}} mcp serve workflow`, seul MCP local d'oh dans les paquets : à servir par la passerelle en conteneur, P4-T08) ; CheckpointService dans `internal/services/checkpoint` (dans le démon) ; règles de session et `PATCH` des permissions ; plugin niveau 3 sur les écoutes du proxy (`/oh/v1/hooks/*`) ; sorties dans `sessions.outputs`. Lot 3.A et P3-T17 validés en réel (e2e Haiku `TestE2ECheckpointGating`, `oh run cadrage` + TUI sous pty) ; P3-T20 couvert par F. **Piste R** : R1 démon et proxy (`f62d86fc`, fusion `916e3fad`, CHANGELOG `fc594b61`) ; R2 port du proxy, R3 M12, R4 **P3-T14** (I6, migration v40, `internal/limits`, `oh budget`), R5 Windows option A, tmux, fenêtre de fermeture vérifiée (`a11aeb2b`, `c1e216b3`, `5922f91c`, `0bfdb3e3`, `d68b6e8c` ; fusion `d375592d`, CHANGELOG `1983878d`) ; R6 points signalés par 4.C/P5 (`465e5c9a`, fusion `b9ba6b8f`, CHANGELOG `e3ebab19`, `main` **non poussé**). **Piste 3.E** (lot 0 recette phase 1 `1d136af1`, P3-T30 abandon de V1 `f0aaa1f7`, P3-T23 → T26 + P3-T29 + P2-T18 `18d5d90a`, P3-T27 `66a1b882`, P3-T28 `b4c59ad1`) fusionnée dans `main` (`ee629d4b`, CHANGELOG `4fb125b4`) : opencode V2 obligatoire (`compatibility.json` minimum 2.0, Doctor), plus d'ancien lancement ni de `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/llm`, `internal/plugin` ; workflow `libre` + ajout additif `entry.selectable` (**à valider**) ; `internal/deploy` remplacé par `internal/bricks` (paquet sans workflow supprimé), `oh deploy`/`oh sync` = message de migration, migrations v36/v37, ancien modèle de workflow et vue Workflow supprimés ; Catalogue des briques (`project.agents`) ; MCP `team` par `OH_TEAM_ID` dans l'environnement de sa déclaration ; `oh migrate deploy-cleanup` (+ écran TUI, Doctor) ; métriques depuis `oh.db` (`internal/sessionstats`) ; guide `migration-v5.*`. Recette phase 1 faite (3 correctifs : second worktree, `PATH` du shell de session, rappel des checkpoints). **Signalés** : 4.C (`PATH` du shell en conteneur), E (orchestrator-dev : branche dédiée redemandée, réservation du ticket), R (`cmd.v5.windows_unsupported` inutilisée). Restent : choix de l'agent de `libre` dans la fiche TUI, origine « communauté » du Catalogue des briques, liens `.opencode` des anciens worktrees non nettoyés ; touche « relever » d'une décision `$` dans la vue Sessions (aujourd'hui `oh budget raise`) ; e2e Haiku du budget. Écarts : `07-phase-3.md` § Piste R |
| 4 | ✅ terminée (lots 4.A, 4.B, 4.C **fusionnés**, reprises d'audit faites par R), recette réelle faite ; restent les recettes Linux et Windows réelles | **piste D** (P4-T01 `c8d4019a`, P4-T02 `fb8c0a51`, P4-T03/T04 `1e96efcf`, P4-T05/T06 `6ceb532a`, correctif `{{oh.bundle}}` `a90f7777`) fusionnée dans `main` (`83b0980d`) ; **piste H** (P4-T07 `5132d082`, P4-T08 `c9ece8b1`, test de G stabilisé `ee85d8b5`) fusionnée dans `main` (`df4a8f51`, CHANGELOG `b890679a`) ; **piste 4.C** (P4-T09 `f0f01dcf`, P4-T10 `1718b0de`, P4-T11 `d5d861d2`, P4-T12 `e3f7fc91`, correctif identité git `2ffd5b04`) fusionnée dans `main` (`e897b7e5`, CHANGELOG `6820ca7b`, `main` **non poussé**) | Runtime conteneur (`internal/runtime`, `internal/runtime/container`), passerelles Beads et MCP (`internal/gateway`, faux `bd` `cmd/oh-bd`, jetons `ohg_…`), détails dans `08`. **4.C** : `domain.ProjectExecConfig` (migration v41 `projects.exec_config`) appliqué par `v5Request` au lancement et à la reprise (`cmd/v5_exec.go`) ; `oh run [workflow]` ; `workflow.Spec.PickRuntime` (option > projet > Réglages > workflow) ; `config.ExecutionConfig` (`[execution]` : runtime, engine, keep_images, opencode_version, strict_isolation ; `OH_CONTAINER_ENGINE` supprimée) ; `pinRuntime` ; `adapters.ServerGroup.IsolateUserConfig` (miroir de `XDG_CONFIG_HOME` sans `opencode/`) ; `ohruntime.Estimator` + `container.Runtime.Estimate` + `runsvc.Service.PrepareEstimate` ; `views/launch_form_runtime.go` (ligne d'état, `LaunchProgress`) ; Doctor par fichier (`registerDoctorCheck`, `cmd/doctor_container.go`, `cmd/doctor_gateway.go`, faits dans `container/doctor.go`) ; identité git de la machine en environnement statique de session (conteneur). Constats opencode 2.0.20 : shell de session sans l'environnement du serveur (ni `HOME`), sous-sessions sans l'environnement de session. **Signalés** : E (skills `orchestrator-workflow-modes`/`orchestrator-dev-protocol` absentes du paquet `ticket`), F (`attach = iterm` sans iTerm2 : client lancé dans le processus de `oh run`), R (`Teardown` à la veille, démon arrêté pendant une construction d'image), volume relatif qui crée un dossier vide dans le projet. Reprises d'audit (M12, port du proxy, Windows option A, tmux) **faites par R** (fusion `d375592d`, `08` § Piste R). Points signalés à R (`Teardown` à la veille, démon arrêté pendant une construction d'image) **faits au lot R6**. Restent : recette Linux réelle, MCP gitlab/team avec de vrais jetons, recette Windows réelle. Tests réels : `make test-container`. Écarts : `08-phase-4.md` § Pistes D, H, 4.C |
| 5 | ✅ code terminé et **fusionné dans `main`** (`03b4d718`, CHANGELOG `6de2e646`, `main` **non poussé**) ; **recette réelle à faire** (aucun runner disponible pendant le build) | **piste P5** : lot 5.0 P5-T01 → T03 (`0770492c`), lot 5.A P5-T04 → T08 (`783de589`), lot 5.B P5-T09 → T14 (`b2bb38b5`), lot 5.C P5-T15 → T19 (`f86bf0fc`), Dockerfile de la config Exécution (`8053ec79`) | Projet `oh-runner` par groupe GitLab, pipeline généré (`internal/remote/ciconfig`, Kaniko ou DinD), contrat machine ↔ job (`internal/remote` : variables, paquets, manifeste, instantané, résumé, export), client GitLab (`internal/remote/gitlab`, faux `gitlabtest`) ; `config.RemoteConfig` (`[[remote.targets]]`, `[remote.projects]`) ; `services/remote` (setup/check, envoi, suivi, récupération, rejeu) ; `services/remote/runner` (job, runtime du job, répondeur, journal, git, install) ; `oh remote setup|status`, `oh run --runtime remote`, `oh runner install|run` (caché), `oh session fetch|resolve` ; migration v39 ; TUI : Réglages › Distant, option ☁, section « À récupérer » (`g`), fenêtre de conflit. Ajouts hors piste : `runsvc.StartRequest.SessionID`, `runsvc.Service.AdoptSession`, `adapters.SessionPorter` (export/import opencode), `opencodev2.setDetached` garde les attributs du runtime, `beadswire.JournalEntry` (`seq`, `stdin`, `placeholder`), `teamstate.Claim.Remote`, `container.FakeBD`/`DefaultBaseDockerfile`, `make build-linux`. Critères : 1 et 3 couverts par les tests (répondeur, aucun secret dans les variables, paquets, environnement de session et artefacts ; essai local complet de `oh runner run` sans LLM), 2 par les tests de rejeu avec conflit simulé ; **les trois restent à valider en réel**. Limites : ~~suivi seulement CLI/TUI ouverte~~ (suivi par le démon oh fermé : fait par R, lot R6), MCP à jeton machine indisponibles dans le job, build args non transmis. Écarts : `09-phase-5.md` § « Écarts constatés pendant le build » |

## À ne pas oublier (nettoyage)

- **Notes de version v5** : le runtime v5 s'active pour tous les utilisateurs d'opencode V2 ; migrations v28–v31 ; `compatibility.json` à remettre à plat au moment de la sortie (entrées 4.2/5.0 incohérentes, minimum V1).

- La branche `feat/v5-phase0` est supprimée (jamais poussée).
- ~~Worktrees et branches fusionnés de la vague 4~~ : supprimés le 07/10 (`../openhub-fixes` · `feat/v5-fixes` déjà absents ; `../openhub-qbis` · `feat/v5-qbis` supprimés après la fusion `fe9ed932`). Seul reste le worktree principal sur `main`.
- ~~Entrée de trousseau de test `…ohsmoke-e9`~~ : supprimée (05/10/2026).
- ~~Environnement de test `/tmp/ohsmoke-e9`~~ : supprimé (05/10/2026). Pour recréer un environnement de test : `OH_HOME=<dir>`, `oh project add`, `[cli] setup_done = true` dans `hub.toml`, et une clé Bedrock dans le trousseau.

## Glossaire

| Terme | Définition |
|---|---|
| **Brique** | Agent ou skill réutilisable (`agents/`, `skills/`, catalogue d'équipe, skills communautaires) |
| **Workflow** | Définition YAML de l'enchaînement : agent d'entrée, graphe, checkpoints, entrées/sorties, ressources, environnements autorisés |
| **Paquet de session (bundle)** | Résultat compilé et haché d'un workflow résolu : config outil + agents + skills + plugin, stocké hors du projet |
| **SessionSpec** | Modèle neutre (indépendant de l'outil) décrivant une session à lancer |
| **Adaptateur** | Implémentation d'`adapters.ToolAdapter` pour un outil (`internal/adapters/<outil>` ; seul opencode V2 aujourd'hui) : Render, serveur, sessions, Attest, événements, décisions, capacités optionnelles neutres ; inscrite dans `adapters.Registry` par la seule racine de composition `cmd/v5_adapters.go` (D19, ADR-049) |
| **Monde fermé** | Invariant : le modèle ne voit que les agents/skills du paquet (+ agents système de l'outil) |
| **Groupe de serveur** | (version du workflow, projet, environnement) → un `opencode serve` partagé par les sessions du groupe |
| **Checkpoint** | Point de contrôle du workflow ; pause/validation humaine selon le mode ; machine à états dans `oh` |
| **Environnement d'exécution (runtime)** | `local` · `container` · `remote` |
| **team-state** | Dépôt git partagé de l'équipe (déjà existant) ; accueille désormais les workflows ; existe en **solo** pour les projets sans équipe |
