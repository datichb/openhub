# 13 — Carte du code actuel (référence, commit `a46f49b`)

Chemins relatifs à `cli/` sauf mention. Les numéros de ligne sont indicatifs (à revérifier avant modification).

> **Depuis la piste 3.E (fusion `ee629d4b`)** : `internal/deploy`, `internal/opencode`, `internal/parallel`, `internal/sweep`, `internal/llm`, `internal/headlesstrack`, `internal/plugin`, `cmd/deploy.go`, `cmd/sync.go`, `cmd/start_deploy.go`, `cmd/tui_deploy_actions.go`, `cmd/start_*_mode.go`, `cmd/v5_platform.go`, `views/workflow_view.go`, `views/project_agents_view.go`, l'ancien modèle `workflow/{base,resolve,validate,permissions}.go` et `EnsureWorktreeConfig` **n'existent plus** (§1, §3, §4, §5 ci-dessous = état historique). Ce qui servait encore est dans `internal/bricks` (frontmatter, assemblage, permissions, modèles, skills de stack, budget, MCP du projet) ; nettoyage : `internal/deploycleanup` ; métriques : `internal/sessionstats` ; Catalogue des briques : `views/bricks_view.go`.

## 1. Contenu embarqué et déploiement (à transformer / supprimer)

| Élément | Emplacement | Devenir |
|---|---|---|
| Embarquement `agents/`, `skills/`, `permissions/` | `internal/hubcontent/embed.go:19-97` → extrait dans `~/.oh/hub/` | **Garder** ; ajouter `workflows/` et `prompts/` du hub |
| `findHubDir()` | `cmd/deploy.go:208-214` | Réutiliser par BundleService |
| Plan de déploiement | `cmd/deploy.go:320-473` (`buildDeployPlan`), phases `:464-471` | **Supprimer** (logique utile → BundleService) |
| `DeployAgents` | `internal/deploy/deploy.go:226-292` | Supprimer ; assemblage → BundleService |
| `DeploySkills` / `ResolveNativeSkillRefs` | `deploy.go:297-387` | Supprimer ; fermeture des skills → BundleService (+ `requires:`) |
| `DeployConfig` | `deploy.go:390-552` (disable natifs `:20,:457-477`, plugin context-mode `:481-492`, compaction, `subagent_depth :511-513`, instructions `:567-583`) | Supprimer ; rendu → adaptateur |
| `DeployAgentConfig` | `internal/deploy/agent_config.go:26-148` (+ conversion permissions `:172-213`) | Logique → BundleService (permissions neutres) + adaptateur |
| `DeployMCP` | `internal/deploy/mcp.go:39-129` | Logique → BundleService/adaptateur (MCP déclarés par workflow) |
| `DeployTeamConfig` (`.opencode/team.json`) | `mcp.go:189-220` | Le serveur MCP `team` doit lire sa config autrement (env de session) |
| Snapshot/rollback | `deploy.go:90-218` | Supprimer |
| `.deploy-state` | `deploy.go:619-671` | Supprimer (sert de référence à `migrate deploy-cleanup`) |
| `context-manifest.json` | `internal/deploy/context_manifest.go` | Supprimer |
| `diff.go` (`ComputeDiff`) | `internal/deploy/diff.go:91` | Supprimer |
| Auto-deploy au démarrage | `cmd/start_deploy.go:19-97` | Supprimer |
| Inlining Bucket A | `internal/deploy/skill_assembly.go:20-64,87-108,154-210,215-242` | **Déplacer** dans BundleService ; corriger le bug (version statique au lieu de la version générée, `:53-61`) |
| Skills générées par le workflow | `internal/deploy/workflow_skillgen.go:21-53`, templates `internal/deploy/templates/*.md.tmpl` | Déplacer ; générées depuis le YAML |
| Stack skills | `internal/deploy/stack_skills.go:10-41` + `prompt.DetectStack` | Garder (BundleService) |
| Budget skills | `internal/deploy/skill_budget.go` (`~2,5 tokens/ligne :130-133`) | Remplacer par mesure réelle (O19) + `oh bundle show --budget` |
| Résolution permissions | `internal/deploy/permission_resolve.go:19-45`, `permissions/*.yaml` | Garder (BundleService), format neutre |
| Résolution modèles | `internal/deploy/model_resolve.go`, `domain/project.go:40-47` | Ajouter niveau workflow (O9) |
| Permissions dérivées du workflow | `internal/workflow/permissions.go:17-121`, `internal/deploy/workflow_permissions.go:17-61` | Garder ; sortie → `SubagentGraph` |
| Liens worktree `.opencode` | `internal/worktree/worktree.go:437-559` (`EnsureWorktreeConfig`) | Supprimer les liens agents/skills ; garder la création de worktree |

## 2. Workflow actuel

| Élément | Emplacement |
|---|---|
| Types | `internal/workflow/types.go:60-75` (`WorkflowDefinition`, `Checkpoint`, `AgentSlot`, `ModesConfig`, `CircuitBreakerConfig`) |
| Base | `internal/workflow/base.go:15-296` |
| Overrides | `internal/workflow/override.go:8-25` |
| Résolution / validation | `internal/workflow/resolve.go`, `validate.go` |
| Cascade | `internal/config/workflow_resolve.go:49-99` |
| Team | `internal/teamstate/teamconfig.go:254-271` (`WorkflowTeamConfig{Overrides, Enforced}`) |
| Projet | `domain.Project.WorkflowConfig` |

Devenir : `WorkflowDefinition` étendu au schéma `oh/v1` (`03` §3) ; multi-définitions ; overrides réutilisés pour `extends`.

## 3. Lancement (à refaire)

| Élément | Emplacement |
|---|---|
| Launcher | `internal/launcher/launcher.go:53-242` (provider/credentials `:73-104`, session DB `:128-153`, `RunInteractive` `:168-178`) |
| opencode exec | `internal/opencode/opencode.go:90-185` (args `--agent/--prompt :134-157`, env `:159-185`), headless `:236-340` |
| Parallèle | `internal/opencode/parallel_runner.go:105-162`, `internal/parallel/server.go:54-56` (`OPENCODE_DATA_HOME`) |
| Stats | `internal/opencode/platform_adapter.go:129-169` (lecture `opencode.db`) |
| Abstraction | `internal/platform/platform.go:124` (`SessionPlatform`), `Capabilities{Parallel, Events}` |
| App | `internal/app/app.go` (conteneur de dépendances) |
| Ouverture terminal | `internal/termlaunch/termlaunch.go` + `applescript.go` (iTerm2, Terminal.app) — **à réutiliser (I2)** |
| Plugins opencode | `internal/plugin/rtk.go` + `rtk.ts` (plugin V1 global `~/.config/opencode/plugins/rtk.ts`) — précédent d'embarquement TS ; à inventorier sous V2 |
| Notifications d'équipe | `internal/notify/*` (Slack, Mattermost, Teams, Discord) — distinctes des notifications système à ajouter |

## 4. Commandes CLI concernées

| Commande | Fichier | Devenir |
|---|---|---|
| `oh start` (+ `--dev`, `--onboard`, `--parallel`, `--sweep`, `--worktree`, `-t`) | `cmd/start.go:70-265`, `start_dev_mode.go:29-189`, `start_parallel_mode.go`, `start_sweep_mode.go`, `start_worktree.go:17-90` | Alias vers `oh run <wf>` |
| `oh audit/review/debug` | `cmd/audit_review_debug.go:27-178` | Alias vers `oh run audit/review/debug` |
| `oh review feedback` | `cmd/review_feedback.go:108` | `oh run review-feedback` |
| `oh takeover-brief enrich` | `cmd/takeover_brief.go:187-250` | `oh run brief-enrich` (headless) |
| `oh deploy` | `cmd/deploy.go:48-136` | **Supprimé** → `oh bundle build/show` |
| `oh sync` | `cmd/sync.go:37-186` | **Supprimé** |
| `oh skill budget` | `cmd/skill.go:226` | Alias `oh bundle show --budget` |
| `oh project add/configure` (agents) | `cmd/project_add.go:106-115,308-311,404`, `cmd/project_configure.go:333` | Retirer la sélection d'agents |
| Assistant d'init (agents) | `cmd/init_steps.go:1240` | Retirer l'étape agents ; ajouter version opencode, conteneurs, espace solo |
| MCP | `cmd/mcp.go:624-687` (`oh mcp serve`, injection trousseau) | Garder ; ajouter `workflow` ; passerelle HTTP (phase 4) |

## 5. TUI (tview/tcell — **pas BubbleTea**)

| Élément | Emplacement |
|---|---|
| Entrée | `cmd/root.go:45-51`, `cmd/tui.go:22-120` |
| Vues | `cmd/tui_views.go:77,271-1052` (`buildViews`) |
| Commandes omnibar | `cmd/tui_commands.go:14-551` |
| Lancement de sessions | `cmd/tui_session_actions.go:72-527` (`translateOhFlags :72-107`, `launchSessionWithPrompt :242-285`, `launchSessionForProject :289-303`, `launchSessionAtPath :308-333`, `launchDevSession :342-389`, `handleDevModeTUI :405-527`) |
| Board quick actions | `internal/tui/v2/views/board_quick_actions.go:132-604`, `cmd/tui_board.go:20-38,171-196` |
| Déploiement TUI | `cmd/tui_deploy_actions.go:27-95,213` |
| Router | `internal/tui/v2/router/router.go:47-162` |
| Shell | `internal/tui/v2/shell/shell.go` (modes `:1110-1173`, `SuspendAndExec :1017-1033`, clavier global `:1226-1380`, modales `:387-1010,1396-1444,1507-1695`) ; omnibar `shell/omnibar.go:374-514` ; commandes `shell/command.go:32-268` ; toasts `shell/toast.go` ; notifications `shell/notifications.go:104-180` |
| Accueil | `views/home.go:86-383`, `home_layout.go`, `home_common.go` |
| Mode projet | `views/project_mode_view.go:98-596` |
| Mode équipe | `views/team_mode_view.go:83-376` |
| Vue workflow (embryon) | `views/workflow_view.go` (734 l.), `widgets/workflowgraph.go` (440 l.), `widgets/graphlayout.go` |
| Vue parallel (vide) | `views/parallel_view.go:47`, enregistrée vide `cmd/tui_views.go:423` |
| Agents projet | `views/project_agents_view.go:57` |
| Config projet | `views/project_config_view.go:62,209-332` |
| Widgets réutilisables | `widgets/sectioned_list.go:38`, `cardcolumn.go:54`, `stepbar.go`, `spinner.go`, `undo_stack.go:17`, `filterlist.go:25`, `inlineselect.go:21` |
| Formulaires | `Shell.ShowInlineForm` (`views/inline_form.go` types), `views/config_field.go:17-314`, `views/inline_wizard.go:158`, `views/wizard*.go` |
| Thème | `internal/tui/theme/*` (Catppuccin, accent par mode `tcell.go:109-190`) |
| i18n | `internal/i18n/i18n.go:40-62`, `locales/{fr,en}.json`, `parity_test.go` |
| Tests | `views/views_test.go` (`mockShell` `:19-43`), `shell/*_test.go` (SimulationScreen), `cmd/tui_test.go` |

## 3 bis. Paquets ajoutés depuis (phases 0, 1, 4)

Voir les écarts de `04-phase-0.md` pour la phase 0 (`internal/sessionspec`, `internal/adapters`, `internal/adapters/opencodev2`, `internal/bundle`, `internal/credproxy`, `internal/daemon`, `internal/runsvc`) et de `05-phase-1.md` pour la phase 1.

| Élément | Emplacement |
|---|---|
| Interface `Runtime`, `PathMap`, `Tool` | `internal/runtime/runtime.go`, `paths.go` (paquet `ohruntime`) |
| Runtime conteneur : détection, image, spec, lancement | `internal/runtime/container/{engine,image,spec,runtime}.go` ; faux `bd` embarqué `assets/` (généré) |
| Faux `bd` (client de la passerelle Beads ; modes `gateway` et `journal`, phase 5) | `cmd/oh-bd/main.go`, `journal.go` (lectures depuis l'instantané, écritures dans le journal), `lock_*.go` (`make embed-bd`, goreleaser, CI) ; format d'échange `internal/gateway/beadswire/wire.go` (`JournalEntry` : `seq`, `stdin`, `placeholder`) |
| GatewayService (passerelles, phase 4) | `internal/gateway/` : Beads `beads.go` (handler), `policy.go` (liste blanche, options refusées), `paths.go` (traduction conteneur → machine), `store.go` (jetons, empreintes seulement) ; MCP `mcp.go` (pont stdio ↔ HTTP) |
| Passerelles dans le démon | `daemon/gateway.go` (montage `/oh-gateway/`, `POST /v1/gateway/grants`, environnement des sous-sessions, route `/oh/v1/hooks/mcp/{name}`), `credproxy.Proxy.Mount` ; `~/.oh/run/gateway.json` |
| Branchement CLI des passerelles | `cmd/v5_gateway.go` (`gatewaySessionEnv`, `gatewayView`, `gatewayMCPCommand`), `cmd/daemon.go`, `cmd/v5_runtime.go` ; `StartRequest.BeadsAllow` renseigné dans `cmd/v5_run.go` |
| Environnement de session (passerelles) | `runsvc/env.go` : `SessionEnvRequest{Runtime, WorkflowID, BeadsAllow, GatewayURL}`, `~/.oh/sessions/<id>/beads.json`, `~/.oh/servers/<groupe>/gateway_url` |
| MCP oh distants en conteneur | `sessionspec/mcp.go` (`OhMCPName`), `adapters/opencodev2/mcp_remote.go` (`remoteOhMCP`) |
| Binaire opencode Linux (npm + intégrité) | `internal/adapters/opencodev2/linux_binary.go` (`ContainerTool`) |
| Lancement serveur en conteneur | `opencodev2.ServerOptions.Run/Paths` (`server.go`), `innerBundle` (`adapter.go`) |
| RunService et runtimes | `runsvc.Service.Runtimes`, `StartRequest.Runtime` (`service.go`) ; câblage `cmd/v5_runtime.go` (`v5Runtimes`) |
| 2e écoute du proxy | `credproxy.Proxy.Listen`, `POST /v1/proxy/listeners` (`daemon/handlers.go`) |
| Config Exécution du projet (4.C) | `domain.ProjectExecConfig` (`domain/project.go`), colonne `projects.exec_config` (v41, `storage/sqlite/project_store.go`) ; application au lancement et à la reprise, runtime préféré, identité git en conteneur : `cmd/v5_exec.go` (`applyProjectExec`, `runtimePrefs`, `pinRuntime`, `gitIdentityEnv`) ; `workflow.Spec.PickRuntime` (`workflow/runtime_pick.go`) ; TUI `views/project_config_exec.go` |
| Réglages Exécution (4.C) | `config.ExecutionConfig` (`[execution]`, `config/config.go`) ; `v5Runtimes(a)`, `v5ContainerRuntime(a)` (`cmd/v5_runtime.go`) ; isolation stricte `adapters.ServerGroup.IsolateUserConfig` → `opencodev2/config_home.go` ; TUI `views/settings_exec.go` |
| Estimation de préparation, fiche ▣ (4.C) | `ohruntime.Estimator` (`runtime/estimate.go`), `container.Runtime.Estimate` + durées de build (`container/estimate.go`), `runsvc.Service.PrepareEstimate` (`runsvc/estimate.go`) ; fiche `views/launch_form_runtime.go` (`RuntimeStatusFunc`, `LaunchProgress`), câblage `cmd/tui_launch_runtime.go` |
| Doctor conteneur et passerelles (4.C) | registre `registerDoctorCheck` (`cmd/v5_doctor.go`), `cmd/doctor_container.go`, `cmd/doctor_gateway.go` ; faits `container/doctor.go` (`ProjectImages`, `CheckTool`, `KeepID`, `NotShared`, `HostIP`, `ProbeHTTP`, `URLHost`) |
| SessionService (piste C) | `internal/services/session` ; vue Sessions `views/sessions_view.go`, câblage `cmd/tui_sessions.go` ; CLI `oh session …` |
| Durcissement démon/proxy (piste R) | `credproxy/routes.go` (liste blanche, modèle), `credproxy/usage.go` (flux, event-stream base64), `daemon/capability.go` (capacité d'émission), `daemon/peer*.go` (UID du pair), `daemon/proxyport.go` (port changé), `daemon/budget.go`, `queue.go`, `memory.go`, `proxyusage.go` (I6), `filelock.LockContext`, `daemon.GroupLockPath`, `daemon.ProxyURLPath` ; `cmd/v5_inprocess.go` (Windows, démon dans le processus) ; crochets du démon `Options.Teardown`/`Periodic` branchés dans `cmd/daemon_remote.go` (veille des conteneurs, suivi distant oh fermé) |
| Restrictions I6 (piste R) | `internal/limits` (`Resolve`, `limits.json`, `queued.json`, données des décisions `$`), `domain.UsageStore` + `sqlite.UsageStore` (v40), `runsvc/limits.go` (file, budget journalier), `services/session/budget.go` (`BudgetResolver`), `cmd/v5_limits.go`, `cmd/budget.go` (`oh budget`), `views/settings_budget.go`, Doctor `cmd/doctor_security.go`, `cmd/doctor_limits.go`, `cmd/doctor_inprocess.go` |
| Checkpoints (piste G) | `internal/services/checkpoint` (dans le démon), serveur MCP `internal/mcp/workflow`, règles `bundle/checkpoints.go`, injection `bundle/workflowmcp.go` ; fiche `views/sessions_checkpoint.go`, `cmd/tui_checkpoints.go` |
| Préférences et sélecteur Beads (piste B) | `internal/prefsvc`, `views/beads_picker.go` (+ `beads_picker_model.go`) |
| WorkflowService (piste F) | `internal/services/workflow` (importé `workflowsvc`) : `Catalog`, `Resolve`, `RenderPrompt`, `Chain`, `ByCategory` ; couche hub seule (équipe/projet : P2) |
| `oh run` et RunService (piste F) | `cmd/v5_launch.go` (`buildWorkflowBundle`), `runsvc/run.go` (`Plan`/`Start`), `runsvc/location.go` (O10), `runsvc/headless.go` (`AwaitTurn`), `cmd/v5_headless.go` ; alias `cmd/alias.go`, `cmd/start_alias.go` |
| TUI phase 1 (piste F) | `views/start_section.go`, `views/launch_form.go`, `views/workflows_view.go` (catalogue), `cmd/tui_start.go`, `cmd/tui_launch.go`, `cmd/tui_chain.go` |
| team-state workflows (piste P2, fusionnée) | `internal/teamstate/workflows.go` (portées, chemins), `workflows_lock.go`, `workflows_load.go` (intégrité, `PromptSource`), `governance.go`, `solo.go` ; `cmd/workflow_layers.go`, `cmd/workflow_doctor.go`, `cmd/team_solo.go`, `cmd/team_promote.go` ; service : `services/workflow/{teamstate,draft,impact,publish,history,bricks,edit}.go`, transaction `teamstate/transact.go` ; CLI `cmd/workflow_{new,edit,diff,publish,history,restore,archive,teamcli}.go` ; migration v38 `cmd/workflow_migrate.go`, `internal/workflow/legacy.go` |
| TUI team-state (piste 2.D, fusionnée) | service : `services/workflow/{workspace,validate,new,diff,textcheck}.go` ; édition YAML `internal/workflow/docedit.go` ; vues `views/workflows_view.go` (catalogue modifiable), `views/workflow_editor{,_model,_fields}.go`, `views/publish_view.go`, `views/history_view.go`, `views/team_detail_view.go` (gouvernance, espace solo), `views/sync.go` (`SetTeamSyncHook`) ; graphe `widgets/graphlayout.go` (`GraphModel`, `SpecGraphModel`, `ComputeModelLayout`), `widgets/workflowgraph.go` ; câblage `cmd/tui_{catalog,publish,editor,solo,workflow_sync}.go` ; solo `cmd/team_solo.go` (`attachProjectSolo`, `nextSoloID`), étape Équipe `buildProjectTeamStep` (`cmd/team_helpers.go`) |

| Distant — contrat machine ↔ job (phase 5) | `internal/remote` : `contract.go` (variables CI et du déclenchement, paquets, `PipelineSchema`), `manifest.go` (`Manifest`, `Snapshot`, `Summary`, `Export`), `pack.go` (tar.gz déterministes) ; pipeline généré `internal/remote/ciconfig` (goldens `testdata/`) ; client GitLab `internal/remote/gitlab` (+ faux `gitlabtest`) |
| Distant — config | `config/remote.go` (`RemoteConfig`, `RemoteTarget`, `MatchTarget`, `ParseGitRemote`) ; `[[remote.targets]]`, `[remote.projects]` |
| Distant — service machine | `services/remote` : `setup.go` (`Setup`, `Check`), `send.go` (`Send`), `machine.go` (`BdCLI`, `GitCLI`, `TakeSnapshot`), `image.go` (tags d'image), `track.go`, `fetch.go`, `replay.go` (`PlanReplay`, `ApplyReplay`) ; `services/workflow/remote.go` (`Resolution.RemotePlan`) ; `teamstate/claims_remote.go` ; `domain/remote.go` + `storage/sqlite/remote_store.go` (v39) |
| Distant — job (`oh runner`) | `services/remote/runner` : `job.go` (variables, `ScrubEnv`), `fetch.go`, `git.go`, `runtime.go` (`JobRuntime`, runtime `remote` du job), `responder.go` (S5, `CISecrets`), `run.go`, `team.go`, `install.go` ; câblage `cmd/runner.go` (démon en processus) |
| Distant — import et export | `runsvc/adopt.go` (`AdoptSession`), `adapters/porter.go` (`SessionPorter`), `opencodev2/porter.go` |
| Distant — CLI et TUI | `cmd/remote*.go` (`oh remote setup|status`, Doctor), `cmd/v5_remote.go` (`oh run --runtime remote`), `cmd/session_fetch.go`, `cmd/session_resolve.go`, `cmd/tui_remote.go`, `cmd/tui_launch_remote.go` ; `views/settings_remote.go`, `views/sessions_remote.go` |

## 6. Bugs relevés (à corriger dans les phases indiquées)

| # | Bug | Où | Phase |
|---|---|---|---|
| B1 | Lancements cassés sous V2 (`--agent`) | `internal/opencode/opencode.go:143` | 0 |
| B2 | `instructions` ignoré en V2 | `deploy.go:567-583` | 0 |
| B3 | `subagent_depth` ignoré en V2 (racine) | `deploy.go:511-513` | 0 |
| B4 | Deux notions de « projet actif » ; sélecteur huh pendant la TUI | `cmd/start_helpers.go:23-85` vs `shell.ActiveProject()`, `cmd/tui_views.go:224-229` | 0 |
| B5 | Équipe active ignorée par les vues équipe | `cmd/tui_helpers.go:25-36` | 0 |
| B6 | Lancement hors boucle d'événements après création de worktree | `views/board_quick_actions.go:341-353,443-488` | 0 |
| B7 | Incohérences : Quick = `""` vs `coder` = orchestrator ; Onboard (menus) sans prompt ; Audit (accueil) sans sélecteur ; pas de Debug à l'accueil | divers | 0 |
| B8 | Bucket A inline la version statique au lieu de la version générée | `skill_assembly.go:53-61`, `workflow_deploy.go:65-82` | 1 |
| B9 | Références vers `skills/templates/` jamais déployées | skills (prose) | 1 |
| B10 | Vue workflow : n'affiche que la couche courante ; `SetDirection` jamais réinitialisé | `workflow_view.go:145,430-431` | 2 (remplacée) |
| B11 | Vue parallel vide | `cmd/tui_views.go:423` | 3 (remplacée) |
| B12 | `TeamsProvider` jamais renseigné ; `AutoDetectMode` inutilisé | `cmd/tui.go:67-73`, `shell.go:1173` | 0 |
| B13 | Chaînes non traduites (« Wiki », « MCP Services », « Parallel », toast FR, « choisir un projet »…) | `tui_commands.go:465-467`, `team_mcp_view.go:85`, `parallel_view.go:50`, `team_detail_view.go:129`, `start_helpers.go:70` | au fil de l'eau |
| B14 | README annonce BubbleTea (TUI = tview) | `README*.md` | 0 |
| B15 | ADR-010 décrit l'ancienne implémentation bash | `docs/architecture/adr/010-*` | ADR |
| B16 | Skill `orchestrator-workflow-modes` parle de `.worktrees/<slug>/` ≠ layout CLI (dossiers frères) | `skills/orchestrator/orchestrator-workflow-modes.md:35-37` | 1 |
| B17 | `onboarder-profiles` marqué `bucket: B` mais listé en Bucket A ; `bucket:` jamais lu | `skills/planning/onboarder-profiles.md:4` | 1 |

## 7. Agents et skills (contenu)

- 19 agents (`agents/<famille>/<id>.md`) ; frontmatter : `id, label, description, mode, model, permission_base, permission, skills (Bucket A), native_skills (Bucket B), mcpServers`.
- ~197 skills (`skills/**`) ; frontmatter `name, description` (+ `bucket` non lu). Aucune dépendance déclarée.
- `orchestrator.md` (270 l. + 11 skills inlinées ≈ 1 634 l.) contient les **modes A–E** et le routage → à découper (phase 1).
- Checkpoints prompt : CP-onboard, CP-0, CP-spec, CP-audit, CP-feature, CP-1, CP-2, CP-3 (`orchestrator.md:249-259`).
