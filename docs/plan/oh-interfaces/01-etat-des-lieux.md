# État des lieux détaillé (inventaire du 05/11/2026)

Source : analyse en lecture seule de `cli/` sur `main` = `fe0bd908` (la fusion de `feat/v54-n` est faite, `feat/v54-l` est encore ouvert dans `../openhub-wt-v54-l`). **Revérifier les numéros de ligne au démarrage de chaque lot.**

## 1. Chiffres clés

| Élément | Valeur |
|---|---|
| Fichiers `cmd/tui*.go` hors tests | 50 fichiers, environ 11 060 lignes |
| `tuiShell` dans `cmd` | 230 occurrences (214 dans `cmd/tui*`, 16 ailleurs) |
| Références `views.X` dans `cmd` | environ 1 010 ; 77 fichiers importent `views`, dont 32 hors `tui*` |
| Fichiers de `cmd` qui importent tview ou tcell | 25 (dont 8 `tui_*`) |
| `i18n.T` dans `cmd/tui_*` | environ 596 |
| `SuspendAndExec` | 7 fichiers : `tui_commands`, `tui_wiki`, `tui_editor`, `tui_sessions`, `tui_launch`, `run_start`, `run_request` |
| `WizardStep` | 57 littéraux, 102 références dans 23 fichiers de `cmd` ; `runCLIWizard` utilisé par 8 commandes |
| Commandes de l'omnibar | 76 entrées (`cmd/tui_commands.go`), dont 28 par `ViewID` et 2 en `RunsDirect` |
| Vues qui importent un paquet métier | `teamstate` 21, `config` 12, `domain` 8, `workflow` 7, `app` 7, `tracker`/`provider`/`limits`/`beads` 3 chacun, etc. |
| Vues construites avec `*app.App` | 7 : Doctor, Status, MCP, Provider, Worktree, Policies, Models |
| Tests des vues | 113 fichiers, 519 tests ; 54 goldens dans `views/testdata` |
| Tests du branchement `cmd/tui_*` | 30 fichiers, 122 tests ; **29 fichiers de branchement sur 49 sans test du même nom** |
| Tests touchés par N2 | environ 22 fichiers, environ 121 tests (`scriptWizard` ×16, `typeInto`/`choose`/`tick` ×79, `onKeys` ×5) |

## 2. Branchement par domaine

| Domaine | Fichiers `cmd/` (lignes) | Structures et interfaces alimentées | Couplages notables |
|---|---|---|---|
| **Sessions** | `tui_sessions` (610), `tui_sessions_tabs` (296), `tui_batch` (102), `tui_chain` (178), `tui_checkpoints` (81), `_verify` (22), `_wiki` (39), `tui_remote` (179), `tui_session_actions` (61) ; partie sessions de `tui_lifecycle` (199) | `*tuiSessions` implémente 14 interfaces : SessionsBackend, SessionDeleter, SessionChainer, SessionNexter, SessionTracer, SessionReportLister, SessionFanouter, SessionRemote, SessionBatcher, CheckpointBackend, CheckpointReportReader, CheckpointWikiDecider, CheckpointWikiEditor ; SessionsSectionConfig | Globales `tuiSess` (caches sous mutex), `tuiChainSuggestions`/`tuiChainMu`, `chainNextByID`, `remoteTrack` ; `SuspendAndExec` (Attach) ; badge `SetSessionsBadge` ; 6 goroutines |
| **Lancement** | `tui_launch` (573), `tui_launch_inputs` (141), `_runtime` (76), `_remote` (42), `tui_start` (325), `tui_card_import` (91) | LaunchFormConfig (10 fonctions sur 29 champs), StartSectionConfig (5) | `tuiStartWiring` (cache Démarrer), `boardRequestInputs` ; `tuiRunCache` (ne garde que les succès) ; `SuspendAndExec` ; libellés de catégorie traduits dans `cmd` (`tui_start.go:149`) |
| **Projets** | `tui_views_project` (301), `tui_actions` (279, assistant d'ajout, tview direct), `tui_project_team` (186), partie de `tui_solo` (118) | ProjectsViewConfig (fonction + 7 setters), ProjectModeConfig (6), ProjectConfigViewConfig (5), ProjectMCPViewConfig (3), ProjectModelsViewConfig (2), ProjectWikiViewConfig (6) | `projectRemover`/`Renamer`/`Mover` appellent le store directement (en double avec `project_remove.go`) ; sélecteur de projet `huh` dans `start_helpers.go` |
| **Équipe** | `tui_views_team` (248), `tui_team_board` (390), `tui_board` (129), `tui_board_card` (259), `tui_team_columns` (288), `_editor` (326, tview), `_keys` (477, tview), `tui_team_autosync` (70), `tui_team_tracker` (675), `tui_tracker` (49), `tui_team_rejoin` (585), `tui_team_new` (114), `tui_team_actions` (214), `tui_solo` (118), `tui_team_mcp_check` (116) | TeamDetailViewConfig (**23** fonctions), TeamsViewDeps (11), BoardQuickActions (13), TeamBoardViewConfig (9) + BoardActions (6), TeamModeConfig (8), TeamMCPViewConfig (7), TeamModelsViewConfig (2) | `teamDetailView` (référence globale à une vue), `pickedTeam`, `columnUI` (crochet de test) ; `actionTrackerDiscovery` fait 453 lignes ; `teamstate` importé par 16 `tui_*` |
| **Wiki** | `tui_wiki` (143), `tui_checkpoints_wiki` (39) | ProjectWikiViewConfig (6) ; Wiki/Patterns par `ResolveTeamFunc` | `SuspendAndExec` (éditeur) ; `wiki_view` lit `teamstate` |
| **Workflows** | `tui_catalog` (278), `tui_editor` (208), `tui_publish` (129), `tui_bricks` (67), `tui_workflow_sync` (84) | WorkflowCatalogConfig (12), WorkflowEditorConfig (4/13), PublishViewConfig (3), HistoryViewConfig (4), BricksViewConfig (2) | `tuiQueueFlushing` (sync.Map) ; `SuspendAndExec` ($EDITOR) ; l'éditeur importe `workflow` (modèle dans la vue) |
| **Administration** | `tui_doctor` (131), `tui_gc` (65), `tui_cleanup` (156, tview, `*shell.Shell` en paramètre), `tui_history_actions` (96), `tui_views_hub` (299, partie provider, mcp, settings, secrets), `tui_views` (305, partie worktree, status, metrics, doctor, notifications, policies) | SettingsViewConfig (8), ProviderViewConfig (5), MCPViewConfig (4), SecretsViewConfig (4), GCViewConfig (2), DeployCleanupConfig (2), WorktreeViewConfig (2), NotificationsViewConfig, MetricsViewConfig ; 7 vues construites avec `*app.App` | Globales `views.DoctorChecks` et `views.DoctorOffline` (avec un `os.Setenv` dans un `init`), `ansiRe` |
| **Transverse** | `tui` (231), `tui_commands` (698), `tui_helpers` (279), `tui_views` (305), `tui_views_hub` (partie home, board, teams), `tui_keys_notice` (56), `tui_lifecycle` (199) | HomeViewConfig (8), BoardViewConfig (5), ShellAccess, registre des commandes | `tuiShell`, `pickedTeam` ; `queueOnShell` ; globales de `views` (`ToolName`, `teamSyncHook`, niveau de mise en page, `wizardTermHeight`) |

## 3. État global à supprimer ou à rendre explicite

**Dans `cmd`, lié au TUI :**
- `tuiShell` (`tui.go:23`), `application` et `store` (`root.go:31,34`)
- `tuiSess` (`tui_sessions.go:37`), `tuiStartWiring` (`tui_start.go:206`), `boardRequestInputs` (`:250`)
- `teamDetailView` (`tui_views_team.go:49`), `pickedTeam` (`tui_helpers.go:71`)
- `tuiChainMu`/`tuiChainSuggestions` (`tui_chain.go:25-26`), `chainNextByID` (`:122`), `remoteTrack` (`tui_remote.go:25`)
- `tuiQueueFlushing` (`tui_workflow_sync.go:21`), `columnUI` et `columnEditorRoles` (`tui_team_columns.go:23,42`)
- `runOnLoop` (`launch_config.go:20`, pointe vers `tuiShell`), `heldNotices` (`mapping_notices.go:37`)

**Crochets de test** (à garder, mais sous forme d'injection) : `confirmForm`, `availableWizardHeight`, `cliConfirm`, `projectPick`, `ticketFactsOf`, `cardFactsOf`, `sessionOutputType`, `sessionReportBody`, `sessionTraceDigest`, `runCLIWizard`.

**`tuiShell` hors des fichiers `tui_*`** :
- **`team_resolve.go:89-109,260`** : `ResolveTeam`, qui est du métier, lit `tuiShell.Mode()` et `ActiveTeam()`. Le résultat dépend donc de l'état de l'UI. **Prérequis de tout le reste.**
- `beads_clean.go:141`, `config_notices.go:31`, `hub_config.go:36`, `init_wizard.go:285`, `launch_config.go:21`, `mapping_notices.go:75`, `team_helpers.go:632`, `project_exec.go:101` (lit `tuiStartWiring`).

**Dans `views`, écrites depuis `cmd`** :
- `ToolName` (`toolname.go:5`), `teamSyncHook` (`sync.go:57`)
- `DoctorChecks` (`doctor_view.go:281`), `DoctorOffline` (`:37`)
- `currentLevel` (`layout_level.go:50`), `wizardTermHeight` (`inline_wizard_step.go:409`)
- `keys.owned` (`keys.go:292`), `modeRoots` (`shell/modes.go:71`)

## 4. Logique métier dans les vues, à déplacer

| Vue | Logique (fichier:ligne) | Destination |
|---|---|---|
| `teamboard_view.go` | `applyFilters` :755 ; tri des onglets :851-879 et des personnes assignées :1535 ; `ticketLabelParts` :820 ; `ticketContext` :1176 (rôle de colonne, `Work`) ; `statusMoves` :1385 (transitions) ; synchronisation, pull et rechargement :494-566 ; prise, libération et transfert :1257-1334, avec des messages d'interface composés dans la vue | `appsvc/team/board` |
| `teamboard_card_actions.go` | `cardEntries` :112 (menu selon le rôle de colonne) ; `doTicketEntry` :140 (selon la source) ; `reviewEntries` :191 | `appsvc/team/board` (actions disponibles = données) |
| `board_data.go` | `FetchTeamTicketsWithColumns` :67 ; `MapClaimStatus(WithConfig)` :128-171 — **règles en double dans `serve_api.go:51-121`** | `appsvc/team/board` (une seule implémentation) |
| `board_columns.go` | `DefaultColumns` :25, `ColumnsFromConfig` :40 | `appsvc/team/board` |
| `sessions_tree.go`, `sessions_view.go` | `fanoutGroups` :18 ; sections et rétention `sessionsSectionsGrouped` :54 ; `sortedDecisions` :82 ; `sessionsSections` :260 ; `questionForm` :915 | Le classement par section devient une donnée du contrat (`Session.Section`) ; le tri reste côté client |
| `sessions_batch.go` | `Failed` :79, `visible` :209 | `appsvc/sessions` |
| `launch_form_model.go` | `newLaunchModel` :205 (valeurs par défaut tirées de la spec) ; `validate` :284 ; `typeError` :345 ; `LaunchExisting.Allows` :117 ; import de `workflow` | `appsvc/launch` (`LaunchDraft`) |
| `config_validator.go` | `FieldValidator.Validate` :37 | `ui/forms` (validation par champ) |
| `settings_view.go`, `settings_exec.go` | `buildFields` :385-654 (schéma des champs de `hub.toml`, environ 270 lignes) ; `config.Patch` :779 ; `doSave` :795 ; `ErrExternalModification` :832 ; `ParseEnvAllow`, `checkVersion` | `appsvc/settings` (le schéma devient un formulaire `ui/forms`) |
| `policies_view.go` | `policy.New(store)` :61 ; modèles :66 ; `teamstate.NewRepo` :280 ; `mutate` :624-657 ; `policyCheckContext` :810-867 (**copie de `cmd/policies.go:153-216`**) ; correspondance formulaire → `Policy` :985-1037 | `appsvc/policies` |
| `secrets_view.go` | `buildEntries` :224 ; `computeFallback` :278 (**règles en double avec `cmd/secrets.go`**) ; `maskedValue` :300 ; **écritures dans le trousseau sur la boucle d'affichage** :476, :504-513, :568 ; exception `archtest` vers `storage/keychain` | `appsvc/secrets` |
| `models_view.go` | **`config.Update` et `Projects.Update` sur la boucle d'affichage** :455, :478, :535, :566 ; règle de portée :479-533 | `appsvc/models` |
| `worktree_view.go` | `worktree.Remove`/`Prune` :508-542 ; lecture de `Sessions`/`Projects` :437 | `appsvc/worktrees` |
| `team_detail_*`, `team_mcp_view`, `team_models_view` | `ConfigKeysSet` :449 ; `syncLoad` ; `ticketPatternRule` ; `SaveMemberField` | `appsvc/team` |
| Lectures de `teamstate` | `sync.go:44` (`Pull`) ; `view_helpers:62`, `activity_view:134`, `teamstatus_view:125`, `takeover_view:172`, `wiki_view:171`, `patterns_view:162` (`NewRepo`) | `appsvc/team`, `appsvc/wiki` |
| `status_view`, `provider_view`, `mcp_view` | lecture de `Config`/`Projects`/`ToolVersion` ; `provider.Detect(All)` ; `config.ReadMCPToken` | `appsvc/status`, `appsvc/providers`, `appsvc/mcp` |
| `cmd/tui_team_actions.go:85` | `LoadConfig` appelé dans `WizardStep.Form` (I/O au montage) | N2 (effet côté serveur) |

## 5. Textes et icônes produits comme données (à remplacer par des données brutes)

- **Sessions** :
  - `StateLabel` (`stateLabel`, `cmd/session.go:222`), `StateIcon`, `KindIcon`, `DecisionSummary`, `fanoutLabel`, `SessionLine{Icon, Label, Desc}` ;
  - timeline `checkpoint.TimelineText` ; `fanoutLine` ; messages « toast_one/many » ;
  - session distante : `StateLabel` écrasé par `RemoteDetail`, lignes avec `theme.Sym*` (`tui_remote.go:69,100-124`).
- **Lancement, éditeur, catalogue** : origine avec `theme.SymModified` ; récapitulatif `InfoField` (`launchRecap`) ; `SymWarning`.
- **Textes renvoyés par les backends puis affichés en message** :
  - sessions : `DeleteSession` (`launch.DeleteReportText`), `RelaunchFanout`, `CheckpointWikiEdit` ;
  - équipe : `TrackerSyncFunc`/`OpenSyncFunc` (`syncSummary`), `TestMCP`, `OnArchive` ;
  - administration : `doctorFixRun`, `planBeadsFix`.
- **Texte anglais en dur** : `"%s: %d fetched, %d created, %d updated"` (`tui_team_tracker.go:80`) ; `formatSyncResultModal` (:609-615).
- **Contenu de fenêtres composé dans `cmd`** : `tui_cleanup.go:78-116`, éditeur de colonnes, `tui_history_actions`, ligne de l'outil (`tui_views.go:193`), `tui_lifecycle.go:192`.
- **Badge** : `views.SessionsBadge` (glyphes).

## 6. Code en double entre la CLI et le TUI (à fusionner dans `appsvc`)

| Sujet | CLI | TUI |
|---|---|---|
| Liste des sessions (correspondance, libellés) | `session.go:147-191` | `tui_sessions.go:299-336`, `:235`, `:271` |
| Réveil, attache, navigateur | `session.go:88,98` | `tui_sessions.go:494-556,567` |
| Suppression | `session_delete.go:28` | `tui_sessions.go:467-490`, `tui_launch.go:252` |
| Étapes suivantes | `session_next.go:51` | `tui_sessions_tabs.go:65` |
| Trace | `session_trace.go:32` | `tui_sessions_tabs.go:97` |
| Lots | `session_batch.go:26` | `tui_batch.go` |
| Décisions, envoi, interruption | `session_control.go` | `tui_sessions.go:396-448` |
| **Regroupement du board** | **`serve_api.go:51-121`** | **`views/board_data.go:67-171`, `tui_team_board.go:128`** |
| Prise, libération, transfert, statut | `claim.go:138-377`, `start_ticket_claims.go:63-88` | `tui_team_board.go:202-321`, `tui_board_card.go:191-211` (séquence répétée 3 fois) |
| Traduction des erreurs de synchronisation du tracker | `team_sync_tracker.go:107` | `tui_team_tracker.go:90` (clés i18n différentes) |
| Retour dans l'équipe | `team_rejoin_cli.go` | `tui_team_rejoin.go` |
| Projets (supprimer, renommer, déplacer) | `project_remove.go:41,67` | `tui_views_project.go` (appels directs au store) |
| Contexte git des politiques | `policies.go:153-216` | `policies_view.go:810-867` |
| Repli des secrets | `secrets.go:66-284` | `secrets_view.go:224-313` |
| Lancement | `run_start.go`, `run_resume.go` | `tui_launch.go:178-282` |
| Takeover | `takeover_brief.go:128` | `tui_team_actions.go:163-214` |

Déjà partagé, et **bon modèle à suivre** : solo, archivage et suppression d'équipe, équipe d'un projet (`team_solo.go`, `teams_lifecycle.go`, `project_set_team.go`).

## 7. Ce que le contrat doit couvrir côté shell

- **`ShellAccess`** (`views/view.go:164-218`) et les méthodes de `*Shell` utilisées par `cmd` : fenêtres (saisie, mot de passe, liste, choix multiple, texte défilant, saisie vérifiée), `Confirm`, `ShowInlineForm`, messages, navigation (`NavigateTo`, `PushView`, `PopView`, `NavigateHome`, `RemountIf`), modes et contexte actif, `SetSessionsBadge`, `RefreshModeBar`, `RefreshHints`, `SuspendAndExec`, `RequestQuit`/`CancelQuit`, `OfferKeysNotice`.
  - **Ce qui devient du contrat** : les événements (notifications, badge), les intentions (navigation), le `Scope` (mode et contexte actif), les capacités (suspension).
  - **Ce qui reste propre au TUI** : les fenêtres, les messages, la pile de navigation.
- **Notifications** : `shell/toast.go` (niveaux, durées, déduplication sur 5 s, v5.4), `shell/notifications.go` (fichier JSONL, rotation). Le lot N de la v5.4 vient d'en modifier une partie : **revérifier**.
- **Commandes** : `shell.Command` et `CommandRegistry`, `views.ContextCommand`. Pour le web, il faudra un catalogue des commandes (identifiant, libellé en code, catégorie, modes) que le contrat exposera. Les actions appellent `appsvc`.
- **Touches** : `keys.Action` (identifiant, contexte, raccourcis, libellé). Le contrat expose des **identifiants d'action**, jamais un `tcell.EventKey`.

## 8. Tests : ce qui protège, ce qui manque

- **Ce qui protège** :
  - 54 goldens de vues et 11 dans `cmd/testdata`, dont `tui_commands_*` et `tui_views_*` (`tui_registry_test.go:75,91`), le filet le plus direct pour `buildCommands`/`buildViews` ;
  - les 121 tests d'assistants ;
  - `TestTUISessionsBackendIsQuiet` (`cmd/tui_sessions_quiet_test.go:28`), qui sert de base aux tests de caractérisation ;
  - `testenv.Isolate`, `sqlitetest.Seed`, `golden` (`-update`).
- **Ce qui manque** :
  - des tests de caractérisation du branchement (sortie des backends figée en JSON) ;
  - un harnais de contrat ;
  - des faux réutilisables hors des fichiers `_test` (`fakeSessionsBackend` est privé à `shell`) ;
  - un faux serveur d'interface ;
  - `make test-tui` qui couvre `cmd/tui_*` ;
  - une mesure de référence des durées.
- **Windows** : `./cmd` n'est pas testé sous Windows. Déplacer le branchement vers `internal/appsvc` ajoute de la couverture Windows, à condition d'**inscrire `appsvc` dans `.github/windows-test-packages.txt`**.
- **`cmd` est le chemin critique des tests** : 378 s sous `-race`, peu parallélisables à cause des globales. Le refactoring devrait le réduire.
