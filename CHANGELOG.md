# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Format : [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/)
Versioning : [Semantic Versioning](https://semver.org/lang/fr/)

---

## [Unreleased]

### Added — runtime v5 (phase 0, opencode V2)

- **Sessions sur le runtime v5** dès qu'opencode V2 est installé (l'ancien chemin de lancement ne fonctionne pas avec V2 ; pas encore pris en charge sous Windows) : paquet de session compilé hors du projet (`~/.oh/bundles/<hash>`), serveur `opencode serve` dédié par groupe (paquet, projet), « monde fermé » vérifié (seuls les agents/skills du paquet sont visibles), plugin oh embarqué (prompt d'agent ajouté au prompt de base d'opencode).
- **Démon `ohd`** (`oh daemon status|stop`) : proxy d'identifiants LLM (opencode ne reçoit qu'un jeton de session ; Bedrock jeton ou profil AWS SigV4, Anthropic, OpenRouter), suivi temps réel des sessions (état, coût, tokens), mise en veille des serveurs inactifs, reprise.
- **`oh session attach|list|stop`** ; ouverture des sessions dans un nouvel onglet/fenêtre (iTerm2, Terminal.app, tmux, navigateur) ; fermer la fenêtre ne coupe pas la session.
- **Clés LLM au niveau équipe** (`openhub.team.<équipe>.provider.<p>.token`, touche `K` dans Détail équipe).
- **Réglages `[session]`** (`attach`, `iterm_style`, `idle_sleep_minutes`) ; question à la fermeture de la TUI ; récapitulatif « pendant votre absence ».
- `--parallel`, `--sweep` et les exécutions sans interface passent par le runtime v5 (un serveur pour toute l'exécution).
- `OH_HOME` relocalise `~/.oh` ; nouvelles vérifications `oh doctor` (runtime v5, démon, git, terminal).

### Added — workflows déclaratifs (phase 1, en cours)

- Schéma de document **`oh/v1`** (`cli/internal/workflow/schema.go`) : entrées, agents et checkpoints ordonnés, textes localisés (`fr`/`en`), délégations explicites (`calls`), politiques distantes, sorties typées, plugins, runtimes. Pas encore utilisé par les lancements.
- **Moteur des workflows** :
  - Lecture stricte des fichiers `oh/v1` : champs inconnus, types, clés en double et documents multiples refusés ; toutes les erreurs sont listées avec ligne et colonne.
  - `extends` : un workflow d'équipe ou de projet complète celui de la couche inférieure ; chaque valeur résolue indique le fichier qui l'a posée. Les champs de sécurité (`risk`, `isolation`, `runtime.allowed`, `beads.allow`, checkpoints obligatoires, `remote`, `limits`) ne peuvent que se durcir ; un workflow de même identifiant doit étendre celui de la couche inférieure.
  - Validation complète : agents connus, graphe de délégation sans cycle, agents atteignables, variables des gabarits de prompt (`{{ .entrée }}`, `{{ .oh.project }}`…), skills et leurs `requires:`, `risk: read` sans agent qui écrit ni commande Beads d'écriture, checkpoints compatibles avec l'exécution distante.
  - **`oh workflow validate <fichier|id> [--layer] [--json] [--all]`**.
  - Paquet de session construit depuis un workflow : agents du workflow, délégations et profondeur maximale tirées du YAML, skills d'enchaînement générées (nouvelle carte du workflow `workflow/workflow-map`).
  - Nouvel agent **`conductor`** : agent d'entrée générique qui suit la carte du workflow.
- **Briques** :
  - **Niveau workflow dans la cascade des modèles** : `models.agents.<id>` puis `models.default` du workflow passent avant les réglages projet, hub et équipe. Identifiants complets acceptés (`amazon-bedrock/eu.anthropic…`, suffixe `#variante`).
  - **Dépendances entre skills** : champ `requires:` du frontmatter ; le paquet de session ajoute les skills requises (inlinées avec une skill inlinée, à la demande avec une skill à la demande). Dépendance introuvable, cycle, identifiant en double ou `name:` différent du nom de fichier bloquent la construction du paquet.
  - **Gabarits livrés avec les skills** : champ `annexes:` ; les fichiers de `skills/templates/` sont copiés dans le paquet à côté de la skill et lisibles par les agents sans demande d'autorisation.
  - **`oh skill check [--json]`** : vérifie le catalogue des skills (doublons, `requires:`, frontmatter, annexes manquantes ou inutilisées, skills citées par les agents mais absentes).
  - **Préférences** (migration v32 `preferences`) : workflows épinglés (5 par portée : hub, projet, équipe), récents tirés des sessions, suggestions par défaut, réglages d'interface.
  - **Sélecteur de tickets Beads en tview** : recherche, filtres de label et d'épopée, regroupement par épopée, aperçu, multi-sélection, tickets réservés signalés.
- **Workflows livrés par le hub** (`workflows/`, embarqués dans le binaire et extraits dans `~/.oh/hub/workflows/`) : `ticket`, `feature`, `quick`, `cadrage`, `onboarding`, `review`, `review-feedback`, `audit`, `debug`, `sweep`, `brief-enrich`, avec leurs gabarits de prompt (reprise des prompts de `--dev`, `--onboard`, `oh audit`, `oh review`, `oh debug`, `review feedback`, `takeover-brief enrich`). Référence : `docs/reference/workflows.{fr,en}.md`.
  - **Rendu des prompts** : entrées libres placées dans des balises de données (`<oh:data>`), traitées comme des données et jamais comme des consignes, tronquées à `max_length` ; identifiants Beads et branches vérifiés.
  - **`risk: plan`** : workflow qui ne modifie aucun fichier mais crée des tickets Beads, limité à `beads.allow` (utilisé par `cadrage`).
  - **`preconditions:`** : contrôles avant lancement (`path_exists`), avec proposition d'un autre workflow puis retour (`feature` et `cadrage` proposent `onboarding` sans contexte projet), ou refus du lancement (`on_fail: block`).
  - **`plugins:` et `code_mode:`** appliqués au paquet de session : plugins déclarés par spécification npm (avec `options`), installés par opencode, y compris en conteneur ; sans `code_mode: true`, l'outil `execute` reste refusé. `context-mode` (plugin opencode V1) ne se charge pas sous opencode V2.
- **Lancement des workflows** :
  - **`oh run <workflow>`** : entrées (`-i clé=valeur`), mode, exécution (`--runtime local|container`, disponibilité vérifiée), emplacement (`--location base|new|<worktree>` ; une session qui écrit reçoit un worktree si une autre session qui écrit occupe le dossier), plusieurs tickets (`--tickets a,b` : une session par ticket sur un seul serveur, un worktree chacune ; `--one-session`), récapitulatif et confirmation (`--recap`), lancement refusé si un autre est en cours dans le même dossier. Les préconditions du workflow sont évaluées : refus, ou proposition de lancer d'abord le workflow suggéré puis d'y revenir. `mcp:` du workflow : seuls les serveurs MCP listés (absent : ceux du projet).
  - **`oh run --headless [--output] [--timeout]`** : sans fenêtre, réponse écrite à la fin du tour (refusé si un checkpoint attend une validation) ; `oh takeover-brief enrich` passe par le workflow `brief-enrich`.
  - **`oh bundle build|show <workflow> [--budget] [--json]`** : contenu du paquet de session (agents, délégations, skills, MCP, isolation, budget estimé du premier tour) ; `oh skill budget <workflow>` en est un alias.
  - **`oh workflow list`** et **`oh workflow show <id> [--origin] [--json]`** (couche qui a posé chaque valeur).
  - Migration **v34** : colonnes de lancement des sessions (couche, version et risque du workflow, emplacement, sorties, session précédente).
- **TUI** :
  - **« Démarrer »** sur l'accueil, le mode projet et le mode équipe : workflows épinglés (`*`), récents, catégories, « Tous les workflows » ; « Session libre » sans workflow (opencode V1).
  - **Fiche de lancement** générée depuis le workflow : entrées (sélecteur de tickets Beads), options (mode, exécution avec disponibilité, emplacement, ouverture, une seule session), récapitulatif (agents, budget, sessions, avertissements, workflow à lancer d'abord) ; `Ctrl+S` lance.
  - Omnibar : une commande `run <workflow>` par workflow (anciens noms en alias), `run <workflow> ⟨ticket⟩` sur les boards, catalogue `workflows`, `review.publish`.
  - Board : `a` propose les workflows qui prennent un ticket (fiche ouverte sur le ticket).
  - **Catalogue des workflows** (lecture seule) : couches, version, risque, exécutions, validité, détail.
  - **« Enchaîner avec… »** (`e` dans la vue Sessions) : workflows qui prennent une sortie de la session (branche, tickets), fiche préremplie ; annonce en fin de session.

### Added — workflows d'équipe (phase 2, hors TUI)

- **Workflows dans le team-state** : `workflows/{published,drafts/<membre>,prompts,history/<id>}` pour l'équipe et `projects/<projet>/workflows/…` pour chaque projet, chargés comme couches `team` et `project` par `oh workflow list|show|validate`, `oh run` et la TUI.
- **`workflows.lock`** : version et empreinte (document et gabarit de prompt) de chaque workflow publié. Un fichier publié modifié à la main, jamais publié ou dont le gabarit a changé est **ignoré avec un avertissement** (`oh workflow validate`, `oh doctor` « Workflows d'équipe »).
- **Verrous `enforce: [champs]`** (ajout au schéma `oh/v1`, `"*"` pour tout le document) : une couche plus spécifique ne peut pas modifier un champ verrouillé.
- **Gouvernance** : `[governance] publish = "any_member"` dans `config.toml` (seule règle prise en charge ; une règle inconnue bloque la publication).
- **Espace solo** pour les projets sans équipe : **`oh team init --solo [--project]`** (team-state local sans remote dans `~/.oh/teams/<id>/`, `solo = true` dans `hub.toml`, fonctions d'équipe coupées) et **`oh team promote --remote <url>`** (partage sans perte : identifiant, historique, projets et workflows conservés).
- **Brouillons** validés à l'enregistrement, poussés avec le team-state mais visibles de leur seul auteur, avec leur propre gabarit de prompt ; **`oh run <wf> --draft`** (local seulement, refusé si le brouillon élargit la version publiée).
- **Publication** : synchronisation, revalidation, version suivante, historique, lock, commit et push ; **refaite au-dessus** d'une publication concurrente ; **file d'attente hors ligne** ; événements `workflow.published`, `workflow.restored`, `workflow.archived`.
- **Résumé d'impact** (risque, agents qui écrivent, exécution distante, checkpoints, Beads, budget, MCP, plugins, Code Mode…) ; **historique, restauration** (nouvelle version) et **archivage**.
- **Catalogue de briques d'équipe** (`catalog/agents`, `catalog/skills`) : agents et skills de l'équipe utilisés par ses workflows ; un identifiant du hub n'est remplacé qu'avec `extends: hub:<id>` explicite ; badge « nouvelle brique » dans l'impact.
- **`oh workflow new|edit|diff|publish|history|restore|archive`** : brouillon dans `$EDITOR` avec revalidation, diff et impact, confirmation si le workflow est élargi, `publish --retry` pour la file hors ligne ; `--project`, `--team`.
- **Migration des anciennes surcharges de workflow** (migration v38, automatique) : `[workflow]` du `config.toml` d'équipe, configuration de workflow des projets et `[workflow.overrides]` de `hub.toml` deviennent des workflows qui étendent `hub:feature` (brouillon puis publication ; espace solo créé pour un projet sans équipe) ; originaux archivés, éléments sans équivalent listés.

### Added — pilotage des sessions (phase 3, en cours)

- **Décisions en attente** (migration v33 `pending_decisions`) : permissions et questions des agents (sous-agents compris), erreurs et budget épuisé, recopiées en continu par le démon ; **première réponse gagne** entre oh, l'interface opencode et le navigateur.
- **`oh session inbox|approve|answer|dismiss|send|follow|interrupt|compact|model|fork|results|resume|open --browser`** ; `oh session list --json` avec les décisions ; un début d'identifiant suffit.
- **Suivi en direct** des sessions (agent courant, outils, messages, coût), diffusé par le démon ; résultats de session (fichiers modifiés, branche, coût) conservés après la mise en veille, description de MR (`oh session results --mr`).
- **Notifications système** quand une décision attend ou qu'une étape se termine sans personne attaché (`terminal-notifier` si installé, sinon `osascript` / `notify-send`) ; `[session] notify = "off"` pour les couper.
- **Vue Sessions** dans la TUI (remplace la vue « parallèle ») : À traiter avec fiches permission / question / alerte, sessions en cours, en veille et terminées, flux en direct, consigne, interruption, modèle, arrêt, reprise, navigateur, choix de la méthode d'ouverture (`A`, tmux en volet avec `iterm_style = "split"`). Badge `● N ⏸ M` sur tous les écrans et sections « Sessions » sur les accueils hub, projet et équipe.
- **Fermeture de la TUI** : choix par session qui travaille (finir l'étape puis veille, arrière-plan, arrêter) ; récap au retour avec les décisions arrivées pendant l'absence.
- Variables d'environnement propres à chaque session, réappliquées à la reprise.
- **Checkpoints contrôlés par oh** (sessions lancées depuis un workflow) : chaque paquet reçoit le serveur MCP **`workflow`** (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) ; un checkpoint en pause dans le mode attend la validation de l'utilisateur, un checkpoint automatique passe sans demande ; état de chaque session conservé (migration v35 `sessions.checkpoint_state`).
- **Agents verrouillés** (`after:` du workflow) refusés tant que leur checkpoint n'est pas passé, y compris dans les délégations imbriquées ; **coupe-circuit** (`circuit_breaker`) : délégations suspendues après N appels d'affilée sans intervention, alerte ✗ à classer.
- **Fiche de décision checkpoint** dans « À traiter » : résumé de l'agent, changements et diff complet, derniers messages, frise ; *Valider*, *Corriger d'abord* ou *Autre consigne* avec un message transmis à l'agent ; frise des checkpoints dans le détail de session (`✔ cp-1 → developer (3) → ⏸ cp-2`). `oh session approve <id> --decision once|fix|other|reject [-m …]`, `oh session dismiss` classe le coupe-circuit.
- Sorties déclarées par les agents (`workflow_outputs`) enregistrées avec la session et reprises par « Enchaîner avec… ».

### Added — exécution en conteneur (phase 4, en cours)

- **Runtime conteneur** (`oh run --runtime container`, option « Exécution » de la fiche de lancement) : le serveur opencode d'un groupe de sessions tourne dans un conteneur, piloté par la ligne de commande de Colima, Podman ou Docker (choix automatique ; `OH_CONTAINER_ENGINE` pour forcer un moteur). macOS et Linux.
  - Image par projet : Dockerfile de développement du projet (`Dockerfile.dev`, `dev.Dockerfile`, `.devcontainer/Dockerfile`, `Dockerfile`, sinon une base Debian minimale) plus une couche oh avec opencode à la même version que sur la machine (téléchargé depuis npm, empreinte vérifiée) et un faux `bd`. Images mises en cache et étiquetées par empreinte ; les deux plus récentes sont gardées par projet.
  - Le paquet de session est monté en lecture seule ; le projet et ses worktrees en lecture-écriture, avec l'utilisateur de la machine (fichiers créés à son nom) ; la configuration opencode de l'utilisateur n'est pas visible.
  - Port du serveur publié sur `127.0.0.1` seulement ; l'interface opencode de la machine s'y attache comme en local.
  - Aucun secret dans le conteneur : le proxy d'identifiants reste sur la machine et n'est joignable qu'avec le jeton de session.
- **Passerelle Beads** : dans un conteneur, `bd` envoie chaque commande au démon oh, qui lance le vrai `bd` sur la machine, dans le dossier de la session ; sortie et code retour sont relayés. Seules les commandes de `beads.allow` du workflow passent (lecture seule sans bloc `beads:`, rien avec une liste vide). Les options qui changent de base ou de dossier (`--db`, `-C`, `--global`…) et les fichiers hors des dossiers de la session sont refusés.
  - Jeton propre à chaque session, donné aussi aux sous-agents ; le démon n'en garde que l'empreinte. Il survit à un redémarrage du démon et devient inutilisable quand la session s'arrête ou se met en veille.
- **Passerelle MCP** : dans un conteneur, les serveurs MCP oh du paquet (gitlab, figma, jira, gslides, linear, github, team, workflow) tournent sur la machine, lancés par le démon, et sont joints en HTTP. Leurs jetons restent dans le trousseau : le conteneur ne voit que le jeton de session.
- `make test-container` : tests réels avec Colima et/ou Podman.

### Changed

- Les anciennes surcharges du workflow unique ne s'appliquent plus (migrées vers des workflows d'équipe ou de projet) : `oh deploy` (opencode V1) et l'ancien lancement utilisent le workflow de base ; la vue Workflow de la TUI est en lecture seule (remplacée par le catalogue et l'éditeur de workflows).
- **Anciennes commandes de lancement = alias dépréciés** des workflows (avertissement) : `oh start` → `feature`, `--dev`/`-t` → `ticket` (épopée : une session ou une par ticket), `--onboard` → `onboarding`, `--parallel` → `ticket --tickets`, `--sweep` → `sweep`, `--worktree` → `--location new`, `oh audit|review|debug`, `oh review feedback` → `review-feedback` ; sans opencode V2 ou sans le workflow, l'ancien lancement reste. `--agent` garde le lancement par agent (avec confirmation si `--recap`).
- TUI : les entrées codées en dur (Quick, Dev, Audit, Review, Debug, Onboard), leurs sous-menus et le sélecteur de tickets huh pendant la TUI sont remplacés par « Démarrer » et la fiche de lancement ; le board ne crée plus de worktree lui-même (emplacement choisi dans la fiche).

- Les anciens modes d'entrée A–E de l'orchestrator ont disparu des skills et des guides : chaque cas est un workflow (`feature`, `cadrage`, `onboarding`, `debug`, `ticket`) ; `orchestrator-dev` lit le mode de la session au lieu de le redemander (il ne le demande plus que hors workflow).
- Les migrations SQLite v28–v31 s'appliquent à la base locale (`servers`, `proxy_grants`, colonnes de suivi des sessions) ; `~/.oh` passe en 0700 et `oh.db` (avec ses fichiers WAL) en 0600.
- Au-delà de v31, une migration absente est appliquée même si une version plus récente l'est déjà (versions réservées par branches parallèles, fusionnées dans le désordre).
- Les fichiers du paquet de session (`~/.oh/bundles/<hash>`) sont en lecture seule.
- Skills : champ `bucket:` supprimé (jamais lu ; le bucket est décidé par le frontmatter de l'agent).
- `orchestrator` : les modes d'entrée A–E et les checkpoints écrits en dur sont retirés (ils deviennent des workflows) ; l'agent garde sa posture et ses contrats. `oh start` lance le workflow `feature`.

### Fixed

- Les surcharges de workflow de `hub.toml` n'étaient jamais chargées (clés non reconnues) et la vue Workflow de la TUI les perdait après les avoir enregistrées ; elles sont relues dans le fichier par la migration v38.
- `oh <commande> --help` affichait la page d'aide générale au lieu de l'aide de la commande.

- Démon `ohd` : les jetons du proxy sont restaurés après l'ouverture du socket (un lancement n'attend plus le chargement des identifiants AWS) ; jetons orphelins révoqués au redémarrage ; un serveur démarré dont la vérification échoue est arrêté avec son jeton ; un groupe n'est plus mis en veille pendant qu'un client le démarre ou le reprend ; attente des verrous bornée.
- Proxy d'identifiants : corps de plus de 64 Mio refusé (413) au lieu d'être tronqué ; requête refusée (502) quand les identifiants ne peuvent pas être appliqués (SigV4 expiré) au lieu de partir sans signature ; seuls les chemins d'inférence de chaque fournisseur sont relayés ; clé `model` en double ou de casse différente refusée quand une liste de modèles s'applique.
- Les questions des agents n'étaient suivies qu'à la reconnexion suivante du démon (session de l'événement `form.created` mal lue).
- Un toast affiché pendant un formulaire de la TUI lui prenait le focus (flèches sans effet) ; l'accueil reprenait aussi le focus en se reconstruisant.
- La commande `quit` de l'omnibar contournait la question de fermeture, et Échap sur cette question laissait la fermeture en suspens.
- Lancements cassés avec opencode V2 (`--agent` n'existe plus en mode interactif) ; version `opencode v2.x` mal lue par la vérification de compatibilité.
- Bascule silencieuse d'opencode vers ses modèles hébergés quand le modèle demandé est indisponible (fournisseur de la session imposé).
- Modèles Anthropic sur Bedrock sans profil d'inférence régional (`eu.`, `us.`…).
- TUI : projet et équipe actifs ignorés, lancements hors boucle d'événements, Quick/Onboard/Audit incohérents, Debug absent de l'accueil, Ctrl+T sans liste d'équipes.
- 7 skills sans `name:` ni `description:` (`team-awareness`, `team-coordination`, `parallel-coordination`…), non reconnues par opencode V2.
- Normalisation des modèles Bedrock : préfixe régional (`eu.anthropic…`) et variante `#…` cassaient l'identifiant.
- Skills d'orchestration et `docs/worktree.md` : les worktrees sont des dossiers frères du dépôt (`../<projet>-<branche>`), pas `.worktrees/<slug>`.
- Test `TestDeployAgentConfigE2E` en échec dans un clone neuf (fixture ignorée par `.gitignore`).
- Review parallèle (`oh review --mode standard+adversarial` / `all`) impossible sous opencode V2 : le reviewer ne pouvait pas lancer ses propres sessions (auto-délégation retirée du paquet). Une auto-délégation explicite (`task: { reviewer: allow }`, `calls: [reviewer]`) est désormais conservée.

### Documentation

- **Guide « Workflows d'équipe »** — `team-workflows.{en,fr}.md` : couches, arborescence, `workflows.lock`, verrous, brouillons et publication, impact, historique, catalogue de briques, gouvernance, espace solo, migration ; références `cli-workflows`, `cli-team`, `config` mises à jour
- **Cascade des modèles** — `model-resolution.{en,fr}.md` : niveau workflow ; niveaux équipe ajoutés à la version française
- **Rédaction des skills** — `authoring-skills.{en,fr}.md` : `requires:`, `annexes:`, `oh skill check`
- **Guide « Sessions v5 »** — `sessions-v5.{en,fr}.md` : ouverture, veille, reprise, commandes, clés LLM, variables d'environnement, limites ; `SECURITY` : proxy d'identifiants et limite connue du mode local
- **Référence MCP GitLab** — `mcp-gitlab.{en,fr}.md` : 11 outils documentés (5 read + 6 write)
- **Guide GitLab réécrit** — Noms d'outils v1→v2, variables d'env corrigées, mode write ajouté
- **Skills adapters GitLab corrigés** — Noms d'outils v1→v2 dans pathfinder, planner, onboarder ; outils supprimés (`list_gitlab_labels`, `list_gitlab_milestones`) remplacés par alternatives
- **Deny-lists orchestrateur** — Surface MCP GitLab mise à jour (3→11 outils interdits)
- **Référence CLI découpée** — `cli.{en,fr}.md` splitté en 1 index + 8 fichiers thématiques (sessions, projects, deploy, config, infra, mcp, team, tools)
- **24 entrées CLI ajoutées** — `--parallel`, `--sweep` (9 flags), `--ticket`, `review feedback`, `review --publish`, `oh purge`, `oh secrets cleanup`, `oh skill budget`, `oh team wiki` (3 sous-commandes)
- **docs/README.md** — Liens SECURITY + CONTRIBUTING ajoutés, section CLI reference mise à jour
- **ADR-037** — Politique de documentation (companion rule, changelog discipline, bilingue, CI gates)
- **CI gates** — Changelog freshness + parité bilingue (failure, pas warning)
- **PR template** — Checklist documentation intégrée
- **Wiki enrichi** — 3 pages bilingues (conventions, architecture, stack)
- **6 nouveaux guides bilingues** — parallel mode, sweep mode, review feedback, notifications, Google Slides, troubleshooting
- **SVG tui-navigation-modes** — Généré depuis le .mermaid source

## [4.2.0] — 2026-10-02

### Added

#### Home pages et navigation (ADR-032)

- **Home pages enrichies (ADR-032)** — Chaque mode (Hub, Projet, Équipe) dispose de sa propre page Home avec des accès rapides contextuels
  - Hub Home : +Doctor, +Secrets, +Ajouter un projet (nouvelle section "Actions rapides")
  - Project Home : +Quick, +Parallel, +Onboard, +Workflow (7 sessions au lieu de 4)
  - Team Home : +Debug, +Quick, +Workflow, +Sync Tracker, nouvelle section "Navigation"
- **Commande omnibar `workflow`** — Vue workflow accessible directement depuis l'omnibar (Priority 50, Configuration)
- **`g`/`G` dans SectionedList** — Aller au premier/dernier item (navigation vim)
- **Branch Git dans le header Project Home** — Affiche la branche courante à côté du path
- **Empty states enrichis** — Suggestions d'actions (Ctrl+P > projects, team init) dans les états vides

#### Board personnalisable (ADR-035) et discovery tracker

- **Colonnes de board personnalisables (ADR-035)** — Configuration `[board]` avec rôles sémantiques (`initial`, `active`, `terminal`, `blocked`), palette de couleurs DS cohérente, override par colonne
- **Wizard de discovery tracker** — Assistant TUI 5 étapes (connexion, discovery, colonnes, mappings, pool) + CLI interactif. Analyse automatique des labels/statuts du tracker, suggestion de mappings, configuration guidée
- **Éditeur de colonnes interactif** — Réordonnancement, ajout, renommage, suppression, cycle de rôle via `tview.List` dans le wizard step 3
- **Commande omnibar `Discover Tracker`** + touche `y` dans la vue Config équipe pour lancer le wizard
- **Étape optionnelle de discovery dans le wizard `team init`** — Propose la configuration tracker après l'initialisation de l'équipe
- **API `DiscoverProject`** — Récupération des labels (GitLab) et statuts workflow (Jira) pour la configuration guidée
- **Moteur heuristique bilingue FR/EN** — Suggestion de mappings label→colonne basée sur des patterns de nommage courants
- **Sync tracker sur touche `r`** — Le refresh du board effectue un sync API tracker complet (GitLab/Jira) avant le git pull
- **Récupération de tous les tickets par labels** — `autopoolUnassigned` récupère tous les tickets correspondant aux labels configurés
- **Pagination GitLab** — `ListAssignedIssues`, `ListUnassignedIssues`, `ListIssuesByLabels` suivent `X-Next-Page` au-delà de 20 résultats
- **Nettoyage automatique des claims terminés** — `CleanupDoneClaims` après chaque sync tracker avec support des colonnes terminales custom
- **Prévisualisation TOML** — Le summary du wizard affiche un snippet de la config générée
- **53 tests unitaires et d'intégration** — BoardConfig, ColumnsFromConfig, moteur heuristique, DiscoverProject (httptest), pagination

#### Système de thèmes ModeTheme

- **Système ModeTheme** — Palette de couleurs 4 niveaux par mode (Hub/Projet/Équipe) avec palettes Teal, Mauve et Peach
- **Migration complète vers ActiveMode** — 18 vues, shell, modals, aide, wizards et widgets migrés vers les couleurs contextuelles
- **Bannières colorées par mode** — Badges, mises en surbrillance de touches et bannières adaptées au mode actif
- **Palettes saturées et réalignées** — Orange (hub), bleu (projet), violet saturé (équipe), tokens globaux alignés avec le hub
- **Barre de mode persistante** — Gutter coloré sous l'omnibar, séparateur de mode dans l'omnibar

#### Refonte du wizard d'initialisation

- **Wizard TUI-first** — Refonte complète du wizard de première exécution avec étapes groupées, sidebar et pages d'introduction
- **Mode Express** — Sélecteur de mode (Express/Full) sur l'écran de bienvenue
- **Navigation et layout unifiés** — Infrastructure de navigation entre pages, layout adaptatif 3 paliers selon la taille du terminal
- **Détection automatique** — Nom de projet depuis le remote git, credentials provider existants, projet/équipe existants en cas de ré-init
- **Changement de langue en direct** — Tous les textes se mettent à jour au changement de locale
- **Étapes MCP consolidées** — 3 étapes de tokens MCP fusionnées en 1 page, ajout des champs URL GitLab/Jira
- **Sélection d'agents et récap deploy** — Nouvelle étape de sélection d'agents avec récapitulatif de déploiement
- **Validation et ergonomie** — Lockfile, pre-flight git, pre-warm secrets, détection tokens existants, hints contextuels sur les champs
- **Étape optionnelle équipe** — Setup équipe intégré au wizard init (rejoin / création)
- **Wizard-engine amélioré** — Feedback visuel pour étapes requises, navigation, hints, summary, timers, spinner différé

#### Abstraction plateforme (ADR-036)

- **`SessionPlatform` abstraction layer** — Nouvelle couche d'abstraction pour les backends de session (ADR-036, Phases 0-3)
- **Interface `ParallelRunner`** — Restructuration multi-backend pour le coordinateur parallèle
- **Migration vers `SessionServer`** — Coordinateur parallèle migré vers l'interface SessionServer

#### Mode parallèle et sweep

- **Moteur de récupération automatique** — Récupération des sessions échouées avec tickets (R13)
- **Contrôle d'admission budgétaire** — Admission basée sur les estimations de tickets
- **Messaging inter-agents** — Notifications du coordinateur pour la communication entre sessions
- **Double logging** — slog + fichier pour les sous-processus serve
- **Mode sweep** — Nouveau package avec config, splitter, collector, verifier et orchestrateur
  - Entrée CLI avec flags, fast path headless pour les sweeps single-task
  - Politique de merge généralisée, skill agent dédié
- **Type `Task` universel** — Généralisation du coordinateur parallèle (R10)

#### Système de review

- **`oh review --publish`** — Réimplémentation avec création réelle de MR, flag `--reviewer`
- **`oh review feedback`** — Nouvelle commande et intégration TUI pour le feedback de review
- **Skill feedback-mode** — Clause de confiance pour les findings humains
- **Outils MCP review** — Discussions MR, approbations, commentaires Jira via MCP
- **`PublishReviewBatch`** — Publication par lot, `team_review_verdict` MCP tool
- **`team_claim_flag_human_review`** — Nouvel outil MCP pour le flag de review humaine
- **Events review** — Types d'événements verdict, émission de conflit de claim, résumé de session

#### Collaboration équipe et wiki

- **Coordination skills** — Skills de coordination wirées aux 19 agents
- **URL MR dans le statut** — `oh team status` affiche l'URL de la merge request
- **Setup tracker après init** — Étape tracker proposée après `oh team init` / `rejoin`
- **Détection de tokens bot** — Récupération en cas de mismatch d'identité, prompt identité 3 options
- **Tokens GitLab par équipe** — Clés de token spécifiques par équipe
- **`oh team wiki`** — Commandes wiki, outil MCP `pending`, formatage des événements
- **WikiView TUI** — Navigation des pages et review des propositions

#### Intégrations externes

- **GitLab** — Client API léger pour les opérations MR, outil MCP `gitlab_reply_to_mr_discussion`
- **Linear** — Filtrage par état et assignee dans `handleListIssues`
- **GitHub** — Corrections endpoint check-runs (head SHA au lieu du numéro PR)

#### Commandes CLI

- **`oh purge`** — Reset complet du hub
- **`oh skill budget`** — Monitoring de la fenêtre de contexte
- **`oh secrets cleanup`** — Nettoyage des secrets avec filet de sécurité
- **`--log-format`** — Flag pour le logging JSON structuré
- **Auto-bootstrap** — Bootstrap automatique du hub au premier lancement TUI
- **Pipeline de lancement unifié** — Nouveau launcher avec pipeline de session unifié
- **Tracking headless** — Suivi des exécutions headless dans le session store (coût/tokens)
- **Centralisation credentials** — Résolution de credentials centralisée et injection dans les modes headless/parallèle
- **API REST et SSE** — État des exécutions parallèles exposé via REST et Server-Sent Events
- **Détection de sessions fantômes** — Détection basée sur le PID
- **Dénormalisation titre de session** — Titre copié depuis opencode.db vers oh.db

#### TUI

- **Mode picker review** — Sélecteur de mode review dans les quick-actions du board et la home
- **Runner standalone InlineWizardView** — Exécution isolée du wizard inline
- **Bannières bold avec soulignement** — Centrées dans tous les modes, Muted plus lumineux
- **URL MR dans le board** — Affichage de l'URL MR dans le détail ticket du board équipe
- **Omnibar masquée pendant les wizards** — Hints centrés par rapport au contenu
- **Moniteur parallèle enrichi** — 7 améliorations de données (R14)
- **Option Publish désactivée** — Affichage avec hint de setup quand WriteEnabled=false

#### Protocoles et skills

- **Auto-memory avec wiki par paliers** — Enrichissement wiki contextuel automatique
- **Pattern Design It Twice** — Variations UI dans le protocole designer
- **Glossaire domaine** — Template de page wiki glossaire avec cible d'enrichissement
- **Session split Enterprise** — Carte de décision pour le découpage de sessions
- **Récupération d'erreur subagent** — Protocole de récupération pour les échecs de sous-agents
- **Classification FACT/DECISION** — Nouvelle classification en Phase 2 du planner
- **Questionnement frontier** — Pour la planification Large/Enterprise
- **Concepts d'analyse d'architecture** — Ajout de concepts génériques
- **`questions_bloquantes` obligatoire** — Champ requis dans tous les handoff blocks
- **Support monorepo** — Détection et support dans 4 doc-skills
- **Prototype-protocol** — Skill injectable pour le designer
- **Page wiki décisions rejetées** — Gestion du cycle de vie des décisions rejetées
- **Bloc rétrospective** — Dans le récap CP-feature
- **Test seams et anti-patterns** — Review de conformité spec
- **Décomposition tracer bullet** — Règle dans la Phase 3 du planner
- **Understanding gate (Phase 2.5)** — Porte de compréhension avant la décomposition du plan
- **Détection signal "ungrillable"** — Avec option prototype
- **Parallélisme conditionnel étendu** — À tous les modes de workflow
- **Système de budget** — Remplace la limite hardcodée 'max 3' parallèle

#### Internationalisation

- **~641 chaînes extraites** — Extraction de chaînes hardcodées en 4 lots depuis 40+ fichiers Go
- **37 clés i18n discovery** — Clés pour le wizard discovery tracker (FR + EN)

#### Politiques

- **Scopes `all_files`/`modified_files`** — Implémentation réelle avec lecture de fichiers, état `NotEvaluable`, helper git diff

### Changed

- **Section "Board" renommée "Projet"** dans le Project Home pour éviter la redondance
- **Footer unifié** sur les 3 pages Home — même ordre de raccourcis
- **Logo Hub via figlet** — Remplace l'ASCII art hardcodé par `renderBanner("OPENHUB")`
- **Séparateur dual-column en BorderCard** — Conforme à Aurum
- **`changeStatus()` dynamique** — Options de statut depuis les colonnes configurées
- **`MapTrackerStatusWithBoard`** — Fallback catégorie via rôles de colonnes
- **`ClaimPoolTicket` configurable** — Accepte un `workStatus`
- **Palette de couleurs actives étendue** — 6 couleurs Catppuccin au lieu de 3
- **InlineWizardView : Ctrl+B/Enter après erreur** — Retour ou réessai après erreur `OnDone`
- **Summary multi-lignes** — Support des `InfoField` avec valeurs multi-lignes
- **Unification wizard init** — `oh init` utilise le wizard inline (Option C) au lieu du formulaire séparé
- **Convergence sessions TUI** — Tous les chemins de session TUI convergent vers le launcher
- **Simplification `oh start`** — Commandes de lancement simplifiées
- **Standardisation StatusHints** — Format uniforme dans toutes les vues
- **Unification styles tview** — `tview.Styles`, couleurs de sélection et boutons unifiés
- **Migration `BuildWizardPage`** — Engine Form, intro, deploy, welcome et team steps migrés
- **Extraction builders** — `buildHTTPSCredStep`, `buildMCPTokenStep` et 4 builders d'étapes team partagés
- **Découplage tracker/MCP** — Configuration tracker découplée de la config MCP
- **5 shared skills** — Création de 5 skills partagées pour éliminer la duplication Bucket B
- **`ActiveMode.AccentHex`** — Remplace le `AccentHex` statique par le token contextuel
- **Migration Platform** — Vues TUI et doctor migrés vers l'interface Platform (Phase 4)
- **Propagation contexte deploy** — Extraction de hash et clés i18n pour les descriptions MCP

### Fixed

#### Sécurité et fiabilité (audit pre-release)

- **Self-update cassé** — `extractBinary` vérifiait l'extension `.tar.gz` du fichier temporaire (sans extension). `oh upgrade oh` retournait systématiquement "unsupported archive format"
- **Remplacement binaire non-atomique** — Le double-rename lors du self-update pouvait laisser le système sans binaire en cas de crash. Remplacé par un staging dans le même répertoire + single rename POSIX atomique
- **XSS dashboard** — 16 injections `innerHTML` non-échappées dans le dashboard HTML. Ajout d'un helper `esc()` sur toutes les données dynamiques (noms de projets, tickets, membres, labels, acteurs, messages d'erreur)
- **CORS wildcard** — `Access-Control-Allow-Origin: *` remplacé par l'origin du serveur (`http://127.0.0.1:<port>`)
- **Data race `parallel_view`** — Écriture de `cfg.Sessions` hors event loop. Déplacée dans le callback `QueueUpdateDraw`
- **Data race `teamboard_view`** — `actionInProgress` (bool) lu/écrit sans synchronisation entre goroutines. Migré vers `atomic.Bool` (16 sites d'usage)
- **Data race `projects_view`** — Même pattern que `parallel_view`, corrigé identiquement
- **Race condition config** — `sync.Once` dans `Load()` non protégé par `cfgMu`, pouvait être reset par `Save()` concurrent. Remplacé par un flag `cfgLoaded` sous mutex unifié
- **Panic recovery goroutines** — Nouveau package `safego` avec `defer recover()` + stack trace. Appliqué aux 15 goroutines les plus risquées (réseau, git, tracker API, deploy, self-update, SSE broadcaster)

#### TUI et shell

- **Deadlock `QueueUpdateDraw`** — Évitement du deadlock depuis les callers de la boucle d'événements
- **Erreurs silencieuses propagées** — `config.Save` et deploy failures remontés en toasts d'erreur
- **Goroutines stale invalidées** — Invalidation des goroutines async stale lors du Unmount des vues
- **Centrage bannière** — Footer ancré au-dessus de l'omnibar
- **Omnibar bloquée** — Activation bloquée pendant les vues capturant l'input
- **Reset thème de mode** — Le thème se réinitialise en revenant au mode hub
- **Pointeurs stale après resize** (E1) — `v.list`/`v.dual` mis à jour dans le `buildFn` closure
- **Race condition `adaptiveHomeMount`** (T1) — `currentMode` utilise `atomic.Int32`
- **Guard `mountGen` manquant TeamModeView** (T4) — Goroutine async protégée
- **`actionInProgress` bloqué** — `syncFuncAsync` garantit `onDone` via `defer`
- **Colonnes non échappées** — `tview.Escape()` appliqué dans `NewCardColumn`
- **Pagination labels sans cap** — `maxLabelPages=50`, `maxIssuePages=100`
- **Path traversal IDs colonnes** — `Validate()` rejette `..`, `/`, `\`

#### Wizard et init

- **Layout welcome page** — Suppression doublons séparateur, espacement corrigé
- **Sélecteur de mode** — Centrage, indicateurs radio, largeur alignée, navigation flèches
- **Hauteur formulaire** — Calcul depuis le contenu pour dimensionnement correct
- **Navigation Tab** — `Tab->buttonForm` accessible quand des TextViews trailing existent
- **Écran d'erreur i18n** — Hints i18n au lieu du français hardcodé
- **Race condition wizard-engine** — `runWithSpinner` + guard nil `stepBar`
- **Step deploy** — CustomView pour préserver les deux boutons
- **Agents collectés via Validate** — Au lieu du bouton form
- **`SetupDone` / guard overwrite** — Propagation erreur MCP, alignement clés token
- **Troncature texte welcome** — Hauteur tv fixe

#### Intégrations

- **GitLab** — Warning HTTPS, déduplication résolution URL, `TargetBranch` dans `MRInfo`
- **Linear** — Propagation de contexte dans les appels API, guard mutation update vide
- **GitHub** — Head SHA au lieu du numéro PR pour l'endpoint check-runs
- **Deploy** — Injection `GITLAB_URL`/`JIRA_URL` dans l'environnement subprocess MCP, embed permission YAML, résolution community skills

#### Review et parallèle

- **Cap taille prompt feedback** — Prévention du dépassement de tokens
- **`duration_s`, `normalizeStatus`** — Correction champ durée et race `SetBoardStatuses`
- **Recovery ticker** — Ajout dans la boucle moniteur event-driven du mode parallèle
- **Signal handling** — Trap SIGINT/SIGTERM pour cleanup gracieux des sous-processus

#### Divers

- **Deadlock stdout headless** — Correction deadlock pipe et troncature silencieuse à 50MB
- **Upsert projet** — Upsert sur path dupliqué au lieu d'échouer
- **Format keychain unifié** — Format de clé keychain unifié CLI et TUI
- **Écriture `hub.toml` unifiée** — Via `config.Save()`, correction `ErrExternalModification` dans le wizard
- **Channel buffered MCP** — Prévention fuite de goroutine dans le serveur de protocole MCP
- **Auto-migration schema Beads** — Centralisation création commandes bd
- **0 issue golangci-lint** — Résolution de tous les issues lint
- **Validation identité GitLab** — Validation contre l'hôte du repo, pas le tracker
- **Token bot détecté** — Affichage champ password dans l'étape token GitLab, correction validation
- **Erreurs silencieuses loguées** — `CommitAndPush`, `Secrets.Set`, panics board init, `Pull` error

#### Internationalisation

- **64 chaînes extraites** — Extraction vers fichiers locale dans 6 fichiers Go
- **16 clés manquantes** — Wiki et review feedback commands
- **Labels et boutons init internationalisés** — R6-R7
- **Placeholder générique** — Pour les dropdowns non-région

### Security

- **Linear GraphQL** — Migration des interpolations de chaînes vers des variables GraphQL

### Performance

- **5 skills lourdes de Bucket A vers Bucket B** — Réduction du prompt de base
- **Déduplication concision** — Règles de concision dans `subagent-concision-posture`
- **Templates externalisés** — Blocs templates déplacés dans `skills/templates/`
- **Exemples verbeux élaguéd** — Exemples détaillés supprimés de 9 skills Bucket A

### Documentation

- ADR-032 EN créé + FR passé en `accepted`
- ADR-035 — Colonnes de board dynamiques et discovery tracker (FR + EN)
- ADR-036 — Abstraction plateforme (FR + EN)
- Mockups 3 modes dans `docs/design/tui-shell.md`
- Composant "Home Pages" dans `docs/design/aurum.md`
- Section "Modes de navigation" dans `docs/guides/tui-usage.{en,fr}.md`
- Commandes mode-aware dans `docs/reference/tui.{en,fr}.md`
- Section `[board]` dans la référence config (FR + EN)
- Entrées glossaire : Hub Mode, Project Mode, Team Mode
- Diagramme Mermaid `docs/diagrams/tui-navigation-modes.mermaid`
- `SECURITY.md` et `SECURITY.fr.md` — Politique de sécurité bilingue
- `CONTRIBUTING.md` enrichi — Guide de développement CLI (prérequis, build, tests, lint)
- ADR-012 corrigé — Renommage bilingue, suppression doublon, traduction EN
- ADR-020 — Traduction EN ajoutée
- `mcp-team.en.md` — Renommage pour cohérence bilingue

---

## [4.1.1] — 2026-09-16

### Fixed

- **Wizard d'init invisible après purge/re-install** — La goroutine async de `HomeView.Mount()` écrasait le wizard poussé par `PushView()` car `Unmount()` n'invalidait pas le compteur `mountGen`. Le guard `mountGen != gen` ne bloquait pas les callbacks stale lors d'un Unmount sans re-Mount
- **DrawFunc stale après navigation** — `adaptiveHomeMount()` installait un `SetDrawFunc` sur le `content` Flex partagé, mais `Flex.Clear()` (appelé par le router) ne le nettoyait pas. Un resize crossing de breakpoint après navigation pouvait reconstruire le layout de la vue précédente par-dessus la vue active. `router.mountLocked()` clear maintenant le DrawFunc avant chaque mount
- **workflowView sans guard de goroutine** — Les callbacks `QueueUpdateDraw` de `Mount()` pouvaient exécuter `content.Clear()` et `SetFocus()` sur des widgets stale après navigation. Ajout de `mountGen` + guards standard
- **Homogénéisation des guards mountGen** — Les 17 vues utilisant `mountGen` incrémentent maintenant le compteur dans `Unmount()` (invalidation explicite), et `HomeView`/`TeamModeView` utilisent le double-guard `v.app == nil || v.mountGen != gen` comme les 14 autres vues

---

## [4.0.1] — 2026-09-09

### Fixed

- **Navigation dual-column sur la home** — `Tab`/`Shift+Tab`/`h`/`l` permettent désormais de naviguer entre les colonnes gauche et droite en mode écran large (≥ 140 cols)
- **Logo non centré en mode 2 colonnes** — le logo ASCII art OPENHUB est maintenant centré horizontalement dans le layout dual-column
- **Pointeurs stale après resize** — `v.dual` et `v.list` dans `HomeView` n'étaient pas re-assignés après un rebuild adaptatif (passage single ↔ dual), causant des comportements erratiques du clavier
- **`h`/`l` activaient l'omnibar** — en mode dual, presser `h` à gauche ou `l` à droite ouvrait l'omnibar au lieu de rester dans la colonne courante

---

## [4.0.0] — 2026-09-09

### Added

#### Fonctionnalités team — Board & Claim lifecycle

- **Board kanban team — 5 colonnes fonctionnelles** — TODO (`planned`), IN PROGRESS (`in_progress`), REVIEW (`review`), BLOCKED (`blocked`), DONE (`done`) avec mapping 1-to-1 des statuses
- **`oh claim --planned`** — réserve un ticket en status `planned` (colonne TODO) sans démarrer immédiatement le travail
- **Transition automatique `planned → in_progress`** — `oh start --dev` détecte un claim `planned` existant et le passe automatiquement en `in_progress` au lancement de la session
- **Labels sur les claims** — champ `Labels []string` dans la struct `Claim` ; labels well-known : `agent-reviewed`, `needs-human-review`, `hub:done`
- **Label `agent-reviewed` automatique** — appliqué sur le claim quand l'événement `review.ready` est émis (fin de session avec review)
- **Actions board TUI wirées** — touches `c` (claim), `x` (release), `t` (transfer), `s` (status) entièrement fonctionnelles dans le shell TUI
- **Labels visuels sur le board** — `[AI]` (vert) pour `agent-reviewed`, `[!]` (jaune) pour `needs-human-review`, tags compacts pour les autres labels
- **Membres idle retirés du board** — le board n'affiche que des vrais tickets ; les membres sans claim n'apparaissent plus
- **Nettoyage automatique des claims `done`** — `CleanupDoneClaims` retire les claims en status `done` après `done_retention_days` jours (configurable, défaut : 7)
- **Événements claim émis** — `claim.taken`, `claim.released`, `claim.transferred` sont désormais émis dans le flux d'activité team lors des opérations correspondantes

#### Fonctionnalités team — Sync tracker externe

- **Package `cli/internal/tracker/`** — abstraction interface `Tracker` avec implémentations GitLab et Jira
- **`oh team sync-tracker`** — synchronise les claims avec le tracker externe (pull + push opt-in) avec affichage détaillé par projet
- **Sync GitLab** — ticket fermé → claim `done` ; ticket réouvert → claim `in_progress` ; labels tracker mirrorés sur le claim
- **Sync Jira** — basé sur `statusCategory.key` (`"done"` = closed, indépendant du workflow custom)
- **Auto-plan depuis le tracker** — issues assignées à un membre sans claim → claim `planned` créé automatiquement (configurable, limite par membre)
- **Push labels opt-in** — labels `agent-reviewed` et `hub:done` pushés sur GitLab/Jira si `push_labels = true` et `write_enabled` sur le MCP
- **Réutilisation config MCP** — le sync tracker utilise le même token que `[mcp.gitlab]` / `[mcp.jira]` dans `hub.toml` (aucune duplication de secret)
- **`last_sync_at` local** — timestamp de dernier sync par projet dans `~/.oh/sync-state.json` ; passé à l'API comme `updated_after` pour les fetches incrémentaux
- **Gestion des erreurs réseau** — rate limiting (429 avec `Retry-After`), token invalide (401/403), timeout 10s ; les erreurs partielles n'avortent pas le sync des autres projets

#### Fonctionnalités team — Fiabilité infra

- **Async pull sur toutes les vues team** — pull git en background à l'ouverture de chaque vue et sur `r` ; toast "Synchronisation..." uniquement si > 1 seconde
- **Fallback données locales** — erreur réseau sur pull → warning toast + affichage des données locales (pas de blocage UI)
- **`RWMutex` sur `teamstate.Repo`** — sérialise les opérations git (write lock) contre les lectures concurrentes de fichiers (read lock) ; élimine les races conditions entre le board timer et CommitAndPush
- **Pull best-effort post-CommitAndPush** — après chaque push réussi, un pull immédiat récupère les commits poussés en parallèle par les coéquipiers

### Changed

- **Board team** — la colonne REVIEW est désormais accessible (status `review`) ; les claims en status `review` n'étaient auparavant jamais affichés dans REVIEW
- **`oh claim`** — le status initial est `in_progress` par défaut (inchangé) ou `planned` avec `--planned`
- **`mapClaimStatus`** — renommée, mapping direct 1-to-1 sans exceptions ; suppression du mapping temporaire `review → done`

### Fixed

- **Actions board TUI non wirées** — `c/x/t/s` ne faisaient rien dans le shell TUI (le board était instancié avec une config vide `TeamBoardViewConfig{}`)
- **Colonne REVIEW inaccessible** — aucun status de claim ne mappait vers la colonne REVIEW
- **Pull synchrone bloquant dans les vues team** — `activity_view`, `takeover_view`, `patterns_view`, `policies_view` bloquaient le thread UI pendant le pull git

---

- **Audit complet P1-P5** — 31 tâches couvrant sécurité, CI/CD, robustesse, extensibilité et croissance marché

#### Sécurité & CI/CD

- **T01 — `govulncheck` en CI** — analyse des dépendances Go pour vulnérabilités connues, bloquant sur CVSS ≥ 7
- **T02 — Workflow de release automatisé** — `release.yml` : build multi-plateforme, packaging, publication GitHub Releases sur tag push `vX.Y.Z`
- **T03 — Matrice CI macOS + Windows** — `ci.yml` étendu avec `macos-latest` et `windows-latest` (amd64) en plus de `ubuntu-latest`
- **T04 — Seuil de couverture 60%** — gate de couverture dans CI via `go test -cover` ; le build échoue sous 60%
- **T05 — Support Windows amd64/arm64** — `GOOS=windows go build` fonctionnel ; chemins `%APPDATA%` et `%LOCALAPPDATA%` gérés dans `cli/internal/config/`
- **T06 — Secrets chiffrés dans les archives** — `oh export` chiffre les secrets avec AES-256-GCM avant inclusion dans le `.tar.gz`
- **T07 — Checksum SHA-256 sur les archives** — `oh export` génère un fichier `.sha256` adjacent à chaque archive
- **T08 — `oh doctor` affiche la mise à jour disponible** — intégration avec `oh upgrade oh --check` pour signaler une nouvelle version dans le rapport de santé

#### Robustesse & Architecture

- **T09 — `oh export [--output path]`** — sauvegarde transactionnelle de la DB SQLite, `hub.toml`, et secrets vers une archive `.tar.gz` horodatée
- **T10 — `oh import <file> [--overwrite] [--merge]`** — restauration depuis une archive ; `--overwrite` écrase, `--merge` fusionne les projets sans supprimer l'existant
- **T11 — `oh repair [--check-only] [--auto]`** — détection de corruption SQLite via `PRAGMA integrity_check` ; `--auto` tente `VACUUM` et reconstruction depuis la dernière sauvegarde
- **T12 — `oh upgrade oh [--check] [version]`** — auto-mise à jour du binaire `oh` pour les installations hors Homebrew ; `--check` affiche la version disponible sans modifier
- **T13 — Registre MCP dynamique** — découverte des serveurs MCP depuis `~/.oh/mcp/` en plus des 7 serveurs intégrés ; pas de recompilation requise
- **T14 — Registre de plugins dynamique** — découverte des plugins depuis `~/.oh/plugins/` avec interface `Plugin` (Name, Run) ; chargement au démarrage
- **T15 — `cli/internal/selfupdate/`** — package dédié à l'auto-mise à jour : téléchargement binaire signé, vérification checksum, remplacement atomique
- **T16 — `cli/internal/skillregistry/`** — package dédié à la gestion des skills communautaires : index distant, résolution de conflits de versions, désinstallation propre

#### Extensibilité & DX

- **T17 — `oh serve [--port 8080] [--readonly]`** — dashboard web local sur `127.0.0.1` uniquement ; API JSON + SPA embarquée ; `--readonly` désactive les mutations
- **T18 — `oh skill add <source>`** — installation d'un skill depuis l'index communautaire (nom court) ou une URL Git ; déploiement automatique dans `.opencode/skills/`
- **T19 — `oh skill list`** — liste les skills communautaires installés avec version, source et date d'installation
- **T20 — `oh skill remove <name>`** — désinstallation propre d'un skill communautaire et nettoyage du déploiement
- **T21 — `oh skill search [query]`** — recherche dans l'index communautaire distant ; affiche nom, description, auteur, étoiles

#### Croissance & Marché

- **T22 — Serveur MCP `github`** — 8 outils MCP : `github_list_issues`, `github_get_issue`, `github_create_issue`, `github_list_prs`, `github_get_pr`, `github_merge_pr`, `github_list_workflows`, `github_trigger_workflow` ; requiert `GITHUB_TOKEN`
- **T23 — Serveur MCP `jira`** — 6 outils MCP : `jira_list_issues`, `jira_get_issue`, `jira_create_issue`, `jira_transition_issue`, `jira_add_comment`, `jira_list_projects` ; requiert `JIRA_URL` + `JIRA_TOKEN`
- **T24 — Serveur MCP `linear`** — 7 outils MCP via GraphQL : `linear_list_issues`, `linear_get_issue`, `linear_create_issue`, `linear_update_issue`, `linear_list_teams`, `linear_list_projects`, `linear_add_comment` ; requiert `LINEAR_API_KEY`
- **T25 — Agent `benchmarker`** (qualite) — benchmarks de performance via Lighthouse (web vitals), k6 (charge), pprof (Go), py-spy (Python) ; rapport structuré avec seuils configurables
- **T26 — Agent `database`** (developer) — gestion de schémas, migrations Flyway/golang-migrate, optimisation de requêtes, audit sécurité base de données
- **T27 — Agent `infra`** (developer) — revue Terraform/Pulumi/K8s, estimation de coûts cloud, détection de mauvaises pratiques IaC, audit sécurité infrastructure
- **T28 — Agent `test-generator`** (qualite) — analyse des lacunes de couverture, génération de tests unitaires, d'intégration et property-based (QuickCheck/Hypothesis/fast-check)
- **T29 — Skills stack Go et Rust** — `dev-standards-golang` (auto-détecté via `go.mod`) et `dev-standards-rust` (auto-détecté via `Cargo.toml`) injectés automatiquement au déploiement
- **T30 — Notifications multi-canaux** — section `[notify]` dans `hub.toml` : Slack, Discord, Teams en plus de Mattermost ; format de webhook unifié
- **T31 — `oh metrics` — tableau télémétrie agents** — nouvelle section dans `oh metrics` : nom de l'agent, nombre de runs, taux de succès, durée moyenne, tokens consommés, coût estimé

### Changed

- **Architecture agents** — passage de 18 à 22 agents ; `benchmarker` et `test-generator` ajoutés aux agents primaires (qualite) ; `database` et `infra` ajoutés aux sous-agents (developer)
- **`oh metrics`** — section télémétrie agents ajoutée ; données lues depuis `opencode.db` (table `part`)
- **`oh doctor`** — intègre la vérification de mise à jour `oh` disponible via `selfupdate`
- **Déploiement stack skills** — détection étendue à `go.mod` (Go) et `Cargo.toml` (Rust) en plus des stacks existants

### Security

- **Archives `oh export` chiffrées** — les secrets (tokens MCP, clés API) sont chiffrés AES-256-GCM dans l'archive ; la clé de déchiffrement est demandée à l'import
- **`oh serve` restreint à 127.0.0.1** — le dashboard web n'écoute jamais sur les interfaces réseau externes ; pas de flag `--host` disponible
- **`govulncheck` en CI** — toute dépendance Go avec CVE ≥ 7.0 bloque le pipeline de release

---

## [3.9.1] — 2026-07-16

### Fixed

- **`oh team init`** — Steps 3 (Notifications) and 4 (Policies) were not displayed
  due to cascading form completion after step transitions. Added `justAdvanced` guard
  to prevent residual messages from triggering immediate completion on the next form.

### Changed

- **Aurum v2 "Floating Panels"** — Visual redesign with 3 depth levels:
  - Terminal bg → Panel (Surface `#1e1e2e`) → Element (SurfaceElem `#323248`)
  - Wizard uses nested rounded panels (outer + inner) instead of separators `━━━`
  - Bordures quasi-invisibles (même famille de couleur que les fonds)
  - Step bar and form inside raised element panel
  - Title in Ivory bold, footer in Lavender (`#8585a0`)

- **Palette refinement** — True color hex values with auto-fallback:
  - Primary: Copper chaud (`#e8a838`)
  - Subtle → Lavender (`#8585a0`) for better contrast on dark backgrounds
  - Muted → Ash (`#6e6e82`) for metadata
  - Error → Ruby adouci (`#ff5f5f`) less aggressive than pure red

- **Design doc** — Two theme variants documented (Mocha active, Bleu Nuit alternative).
  Anti-patterns section added: no hardcoded colors, no separator lines.

## [3.9.0] — 2026-07-15

### Added

- **Design System "Aurum"** — Direction artistique formalisée (`docs/design/aurum.md`).
  Palette : Gold (primaire), Amethyst (accent), Jade (success), Amber (warning),
  Ruby (error), Sapphire (info). Icônes : `● ◔ ○` pour les steps, `▸` pour les flèches,
  `━` pour les séparateurs bold.

- Nouvelles constantes de style : `Info`, `Accent`, `TextLight`, `Surface`, `Border`,
  `BorderActive`, `InfoStyle`, `AccentStyle`, `BoxSubtle`.

### Changed

- **`oh team init`** — Redesign complet du wizard :
  - Layout "step bar horizontal + full width" au lieu du côte-à-côte.
  - Fix critique : les sous-formulaires dans les callbacks `OnDone` causaient un conflit
    d'affichage avec l'alt-screen BubbleTea. Tous les forms sont maintenant inline dans
    les steps (plus de `huh.Form.Run()` dans les callbacks).
  - Les étapes en mode "existant" affichent directement le formulaire pré-rempli
    au lieu d'un Confirm → sous-form.
  - Esc skip une étape proprement.

- **Palette globale** — Migration de Purple (99) vers Gold (178) comme couleur primaire.
  Toutes les vues (dashboard, board, teamboard, parallel) migrent vers les constantes
  partagées `common.*`. Zéro couleur hardcodée restante.

- **Title bars** — Background Obsidian (235) + foreground Ivory (255) + accent Gold.
  Remplace l'ancien "white on purple".

- **Bordures actives** — Gold au lieu de Purple pour l'élément sélectionné/actif.

## [3.8.0] — 2026-07-15

### Changed

- **`oh team init`** — Wizard multi-étapes adaptatif avec sidebar côte-à-côte (BubbleTea).
  Crée automatiquement `config.toml` et `policies.toml` si absents.
  Détecte l'état du repo et propose uniquement les étapes pertinentes.
  Propose un set de policies recommandées (branch naming, commit format, max WIP, review required).
  Nouveau composant `wizard` réutilisable dans `tui/views/wizard/`.

- **`oh init`** — Indicateur de progression par sidebar (liste d'étapes ✓/→/·)
  au lieu des simples préfixes `[1/4]`. Utilise le composant `RenderSidebar` partagé.

### Added

- Composant `tui/common/wizard.go` — sidebar renderer réutilisable (statuts d'étapes avec icônes).
- Composant `tui/views/wizard/wizard.go` — modèle BubbleTea générique pour wizard côte-à-côte.
- `teamstate.Repo.HasConfig()`, `HasPolicies()`, `HasMember()` — détection d'état du repo.
- `teamstate.Repo.SavePolicies()` — écriture de policies.toml.
- `teamstate.Repo.UpdateMember()` — mise à jour d'un membre existant.
- Clés i18n `cmd.team.init.*` (FR + EN) — support bilingue du wizard.
- `team init` visible dans le help custom (`oh help`).

## [3.7.2] — 2026-07-15

### Added

- **`oh review --branch`** (`-b`) — spécifie la branche à reviewer (diff vs main).
  Fallback automatique : si non spécifié et branche courante est une feature branch,
  l'utilise comme cible. Fetch automatique si la branche n'existe pas localement.

### Fixed

- **`oh project add`** — `bd init` ne génère plus de fichiers d'instructions agent
  (`AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.agents/`) dans le projet cible.
  Ajout des flags `--skip-agents` et `--setup-exclude` pour que Beads utilise
  `.git/info/exclude` au lieu de modifier `.gitignore`.

### Changed

- Agent **reviewer** : permission `git fetch*` ajoutée pour permettre le fetch de
  branches distantes avant le diff. Workflow étape 1 mis à jour pour gérer le tag
  `[BRANCH:X]` et le fallback fetch/local.

---

## [3.7.0] — 2026-07-13

### Added

- **Team Policies** (`oh policies list/check/add`) — règles d'équipe enforceables
  (refuse/warn) avec types regex, boolean, limit, forbidden_pattern.
  Overrides par projet. MCP tool `team_policies`. Double enforcement CLI + agents.

- **Takeover Briefs** (`oh takeover-brief show/list/enrich`) — briefs de reprise
  générés automatiquement lors des transferts de tickets. Détection stale
  configurable. Enrichissement IA via `opencode run` (agent brief-enricher).
  MCP tool `team_takeover_brief`.

- **Team Board** (`oh team board`) — TUI Kanban interactif plein écran pour
  visualiser l'équipe (colonnes IDLE/IN PROGRESS/REVIEW/BLOCKED). Navigation
  clavier, mode détail avec sub-beads.

- **`oh team status --detail`** — affiche les sous-tickets et la progression
  sous chaque claim.

- **Patterns Library** (`oh patterns list/show/add/validate/remove`) — bibliothèque
  de décompositions réutilisables. Alimentée par le planner (auto), pathfinder
  (auto), ou manuellement. MCP tools `team_patterns_list/read/propose`.

- **Parallel Sessions** (`oh start --parallel --tickets bd-42,bd-43,bd-44`) —
  exécution parallèle de N agents sur N tickets dans des worktrees isolés.
  TUI de suivi temps réel, attach/detach vers les sessions individuelles,
  détection de conflits, merge séquentiel avec validation humaine.
  Flags: `--priority`, `--max-sessions`.

- **`opencode.RunHeadless()`** — exécution non-interactive d'opencode pour les
  tâches utilitaires (enrichissement briefs, futures automations).

- **5 nouveaux MCP tools** : `team_policies`, `team_takeover_brief`,
  `team_patterns_list`, `team_patterns_read`, `team_patterns_propose`

- **4 nouveaux skills** : `team-policies-enforcement`, `takeover-context-protocol`,
  `parallel-coordination`, `planner-patterns-protocol`

- **1 nouvel agent** : `brief-enricher` (read-only, enrichissement de briefs)

### Changed

- `oh claim` vérifie désormais la policy `max_ticket_wip` avant de réserver
- `oh claim transfer` génère automatiquement un takeover brief
- `oh claim` détecte les tickets stale et propose la reprise avec brief
- `team-awareness.md` enrichi avec les références policies, takeover, parallel
- `team-coordination.md` enrichi avec le protocole takeover
- Structure team-state étendue : `policies.toml`, `takeover-briefs/`, `patterns/`
- `TeamConfig` étendu : sections `[takeover]` et `[parallel]`
- `Claim` struct : nouveau champ `LastActivity`

---

## [3.6.0] — 2026-07-10

### Changed

- **`-p` = `--project` partout** — le short flag `-p` est désormais uniformément réservé à `--project` sur toutes les commandes :
  - `start`, `audit`, `review`, `debug`, `deploy`, `sync` : `-j` → `-p` pour `--project`
  - `start` : `--prompt` passe de `-p` à `-m`
  - `metrics` : `--period` passe de `-p` à `-d`
  - `project add` : `--path` passe de `-p` à `-d`
- **Réorganisation du help (`oh --help`)** — 8 sections thématiques au lieu de 6 :
  SESSION, PROJET, DÉPLOIEMENT, MCP, CONFIGURATION, ANALYTICS, ÉQUIPE, INFRASTRUCTURE
- Toutes les commandes et flags sont désormais visibles dans le help global

### Added

- Sections **MCP**, **ÉQUIPE** et **ANALYTICS** complètes dans le help
- Commandes `optimize`, `yield`, `team status/activity`, `claim`, `release`, `conventions check`, `config model *`, `provider setup` visibles dans le help
- `worktree` déplacé dans la section PROJET (au lieu d'INFRASTRUCTURE)

### Removed

- Commandes `service`, `service setup`, `service remove` retirées du help (dépréciées en v3.5.0, remplacées par `oh mcp`)

---

## [3.5.0] — 2026-07-10

### Added

- **`oh mcp enable <service> [--project]`** — active un service MCP au niveau hub ou pour un projet spécifique
- **`oh mcp disable <service> [--project]`** — désactive un service MCP (override explicite par projet possible)
- **`oh mcp reset <service> --project`** — supprime l'override projet, retour à la config hub
- **`oh mcp setup [--project]`** — wizard interactif de configuration token + options (remplace `oh service setup`)
- **`oh mcp status [--project]`** — tableau récapitulatif avec source effective (hub/project) et statut token
- **Champ `Enabled *bool`** sur `ProjectMCPService` — permet 3 états : hériter du hub (`nil`), forcer l'activation (`true`), forcer la désactivation (`false`)

### Changed

- **Cascade MCP projet** — les services listés dans `ProjectMCPConfig` n'écrasent plus la liste hub ; chaque service peut individuellement hériter, forcer on, ou forcer off
- Documentation de référence services (fr + en) réécrite pour les nouvelles commandes

### Deprecated

- **`oh service`** — toutes les sous-commandes (`setup`, `remove`, et status par défaut) affichent un message de dépréciation et renvoient vers les équivalents `oh mcp`

---

## [3.4.2] — 2026-07-10

### Changed

- **--project flag** — résolution par nom au lieu de l'ID généré ; l'ID reste accepté en fallback
- **Nom de projet unique** — contrainte d'unicité ajoutée (migration SQLite v12)
- **Autocomplétion shell** — propose les noms de projets au lieu des IDs
- **deploy output** — suppression des lignes "Source:" et "Target:" (normal + diff)

### Added

- `ProjectStore.GetByName()` — lookup par nom dans l'interface domain et le store SQLite

---

## [3.4.1] — 2026-07-09

### Changed

- **init wizard** — réorganisation en 4 étapes thématiques :
  [1/4] Configuration générale (langue, version),
  [2/4] Provider (choix + credentials immédiatement),
  [3/4] MCP (optionnel, sélection + tokens),
  [4/4] Premier projet (optionnel)
- MCP MultiSelect : description mise à jour avec hint "Espace pour sélectionner, Entrée pour valider"

### Fixed

- init : la configuration des credentials provider se fait désormais juste après le choix du provider (plus de décalage illogique)
- init : le hub.toml est écrit incrémentalement (provider d'abord, MCP ensuite) pour un flow plus naturel

---

## [3.2.0] — 2026-07-08

### Added

- **Deploy : per-agent config** — configuration individuelle par agent avec model cascade et skill assembly
- **Deploy : config-aware diff** (GAP-8/11) — comparaison filtrée par agents sélectionnés, détection de drift `opencode.json` via hash, persistance `.opencode/.deploy-state` après succès
- **Deploy : stack skills detection** (GAP-12) — détection automatique du stack technique (TypeScript, Python, React, Vue, Next.js, Django, FastAPI, Docker, GitHub Actions, GitLab CI, Jest, Vitest, etc.) et injection des skills correspondants dans le déploiement natif (Bucket B)
- Helpers exportés `prompt.HasGitHubActions()` et `prompt.HasGitLabCI()`

### Changed

- `oh config model show --json` produit uniquement du JSON structuré (plus de mélange human-readable + JSON)
- `ComputeDiff` accepte les paramètres `selectedAgents` et `srcHashFn` pour un hash custom des sources
- Dépendance `go-toml/v2` promue en dépendance directe

### Removed

- Mécanisme tracker (SyncPull, champ tracker, question wizard) supprimé des beads

### Fixed

- `ComputeDiff` hash désormais la sortie assemblée (skill inlining + strip hub fields) au lieu du fichier source brut — élimine les faux positifs de diff après un deploy frais
- `stripHubFields` ne matche que les clés YAML top-level (évite les faux positifs sur sous-clés indentées comme `permission.skill`)
- Guard défensif contre les agents avec `id` frontmatter vide dans `DeployAgentConfig`

---

## [3.1.0] — 2026-07-08

### Added

- **Team collaboration system** — centralized team coordination via a dedicated `team-state` Git repo
  - `oh team init` — interactive setup wizard (repo URL, member profile, Mattermost config)
  - `oh team status` — who is working on what (tabular view)
  - `oh team activity` — event feed (filtered by date, member, project)
  - `oh claim <ticket>` / `oh release` / `oh claim transfer` — ticket reservation with conflict detection
  - Team-state repo managed transparently in `~/.oh/team-state/`
  - Monthly JSONL event journal with automatic rotation
  - Cross-project wiki with proposal-based contributions (human review required)
  - Mattermost notifications (session.complete, review.ready, claim events, wiki proposals)
  
- **MCP server `team`** — AI agents access team data via MCP protocol
  - `team_members`, `team_claims`, `team_wiki_read`, `team_wiki_list`, `team_events` (read)
  - `team_wiki_write` (documentarian only, creates pending proposals)
  - `team_notify` (orchestrator-dev, reviewer, auditor)

- **GitLab bidirectional** — opt-in write capabilities for the GitLab MCP server
  - `gitlab_create_mr` — creates MR (detects existing by source_branch, no duplicates)
  - `gitlab_add_mr_note` — adds comments to MRs
  - `gitlab_update_issue` — updates state, labels, assignees
  - `gitlab_assign_reviewer` — assigns reviewers to MRs
  - `gitlab_add_label` — adds labels to issues
  - `write_enabled` config flag — opt-in, with token scope guidance during setup
  - Display required token permissions in `oh service setup` wizard

- **`oh conventions check`** — verifies branch naming and commit format against project wiki conventions
  - Reads patterns from `docs/wiki/technical/conventions.md`
  - Supports Conventional Commits auto-detection
  - Non-blocking warnings (medium enforcement)

- **`oh review --publish`** — creates MR and emits team notification for review
  - Detects current branch, extracts ticket reference
  - Emits `review.ready` event + Mattermost notification
  - Respects CP-2: merge is ALWAYS manual

- **`oh start --dev --ticket <id>`** — skip picker, work on specific ticket directly
  - Auto-claim in team-state on ticket selection
  - Team-state pull for claim awareness in picker

- **Beads enrichment helpers** — new functions for agent use
  - `CreateFromGitLab` — creates bead with `[TICKET-REF]` title convention
  - `CreateSubtask` — creates child bead with dependency link
  - `RememberGitLabContext` — stores GitLab context in bd memory
  - `AddNote` — agents add contextual notes during sessions
  - `ClaimTicket` / `CloseTicket` — lifecycle management

- **Team-aware skills**
  - `skills/shared/team-awareness.md` — Bucket A, all agents (when team enabled)
  - `skills/shared/team-wiki-protocol.md` — Bucket B, documentarian wiki contribution protocol
  - `skills/orchestrator/team-coordination.md` — Bucket B, conventions verification + claim checks + MR proposal

- **Multi-mode review system** — the reviewer agent now supports multiple review modes:
  - `standard` — classical 6-category checklist review (unchanged, default for tickets)
  - `adversarial` — maximum skepticism posture, min. 10 findings, dangerous assumptions analysis, confidence score
  - `edge-case` — exhaustive unhandled execution path hunting, available everywhere as an option
  - Combined modes (`standard+adversarial`, `all`) via **parallel independent sessions** with context isolation
- **`review-merge` skill** — deduplicates and unifies reports from parallel review sessions
- **`oh review --mode` flag** — CLI mode selection with shell completion
- **Reviewer self-delegation** — reviewer can invoke parallel reviewer sessions for context isolation

- **ADR-024** — Team State Repository architecture decision record (EN/FR)

### Changed

- `MCPServerConfig` extended with `WriteEnabled bool` field
- `DefaultMCPServers()` signature extended with `writeEnabled` parameter
- GitLab MCP `gitlabAPI` refactored to `gitlabRequest` supporting all HTTP methods (POST/PUT/DELETE)
- `checkMCPToken` now passes tokenless servers (e.g., team MCP)
- Deploy injects `GITLAB_WRITE_ENABLED` env var when write mode is active
- `oh start --dev` now pulls team-state before showing picker (claim awareness)

### Removed

- QA Engineer agent (`agents/quality/qa-engineer.md`) — replaced by enhanced reviewer modes

---

## [2.0.0] — 2026-07-01

### Added

- **Validation transparente des providers LLM** (`scripts/lib/provider-warnings.sh`) :
  nouveau système de diagnostic non bloquant intégré à `oc start`.
  Affiche le statut du provider (✅/⚠️) dans le bloc contextuel avec des hints actionnables.
  - **Approche A — Pre-flight check** : test de connectivité curl (3s) avant le lancement d'OpenCode,
    avec skip automatique en mode non-TTY (CI/CD).
  - **Approche C — Validation post-deploy** : détection des incohérences model ↔ bloc provider
    dans `opencode.json` (signale les modèles orphelins sans bloc provider correspondant).
  - Détection des `baseURL` malformées (suffixe `/chat/completions` en doublon).
  - Couverture de tous les chemins d'entrée via `_warn_provider_if_needed` dans `adapter_start()`
    (couvre `oc start`, `oc quick`, `oc review`, `oc audit`, `oc conventions`, `oc debug`).
  - Messages i18n FR/EN avec hints vers `/connect` OpenCode et `oc config set`.

- **Bloc `models{}` pour providers litellm** (`scripts/adapters/opencode.adapter.sh`) :
  les providers OpenAI-compatible (mammouth, ollama, github-models) déclarent désormais
  un `name` et un bloc `models` dans `opencode.json`, résolvant les erreurs
  `ProviderModelNotFoundError` au choix d'un agent dans OpenCode.

- **Support provider openrouter** (`scripts/adapters/opencode.adapter.sh`) :
  ajout du cas `openrouter` dans `_build_provider_json()` — précédemment manquant,
  ce qui laissait `provider_json` vide pour les projets configurés sur openrouter.

- **16 nouvelles clés i18n** (`scripts/lib/i18n.sh`, FR + EN) :
  `provider.status_ok`, `provider.status_unreachable`, `provider.status_no_creds`,
  `provider.status_model_orphan`, `provider.status_bad_url`, `provider.status_no_key`,
  `provider.hint_connect`, `provider.hint_hub_config`, `provider.hint_aws_creds`,
  `provider.hint_check_url`, `provider.hint_check_network`, `provider.warn_deploy_hint`.

- **Tests BATS** (`tests/test_lib_provider_warnings.bats`) :
  ~35 tests couvrant la validation de config (Approche C), le pre-flight de connectivité
  (Approche A avec mocks curl), l'affichage du statut provider, et le warning minimal.
  Tests additionnels dans `test_opencode_adapter.bats` (+6 : bloc models litellm,
  openrouter, merge opencode_cache.provider) et `test_lib_i18n.bats` (+3 : nouvelles clés).

- **Documentation** (`docs/guides/providers.fr.md`, `docs/guides/providers.en.md`) :
  nouvelle section "Diagnostic et résolution des erreurs provider" avec tableau des messages
  de statut, guide de configuration via `/connect`, erreurs courantes OpenCode,
  et cas spécifiques Bedrock et MammouthAI.

### Security

- **Fix #7 — Validation HTTPS pour `GITLAB_BASE_URL`** (`servers/gitlab-mcp/src/config.ts`) :
  le serveur MCP GitLab rejette désormais les URLs HTTP pour éviter d'envoyer le token en clair.
  Opt-out possible via `GITLAB_ALLOW_HTTP=1` pour les instances internes/dev.

- **Fix #4 — Stockage sécurisé des clés API (keychain multi-OS)** (`scripts/lib/secrets.sh`) :
  nouvelle librairie d'abstraction des secrets avec détection automatique du backend :
  macOS Keychain (`security`), Linux GNOME Keyring (`secret-tool`), fallback fichier chmod 600.
  Les clés dans `api-keys.local.md` peuvent désormais pointer vers le keychain via le marqueur `__KEYCHAIN__`.
  Override : `OC_SECRET_BACKEND=keychain|secret-tool|file`.

### Fixed

- **Fix #1 — Verrouillage de fichiers multi-OS** (`scripts/lib/filelock.sh`) :
  les écritures concurrentes dans `projects.md`, `api-keys.local.md` et `hub.json`
  sont désormais protégées par un verrou exclusif. Stratégie : `flock` (Linux),
  `/usr/bin/lockf` (macOS), `mkdir + PID` (fallback universel).
  API : `_acquire_lock`, `_release_lock`, `_with_lock`.

- **Fix #2 — Rollback transactionnel pour `oc deploy`** (`scripts/cmd-deploy.sh`) :
  un snapshot de l'état pré-déploiement est créé avant la Phase 1.
  En cas d'interruption (Ctrl+C) ou d'échec, `.opencode/agents/` et `opencode.json`
  sont restaurés automatiquement via trap `INT TERM ERR`.

- **Fix #3 — `oc sync` propage les erreurs de déploiement** (`scripts/cmd-sync.sh`) :
  les échecs de `adapter_deploy` ne sont plus masqués par `>/dev/null 2>&1`.
  Nouveau compteur `failed_count` distingué de `skipped_count`.
  Les projets en échec apparaissent dans le résumé avec la première ligne d'erreur.
  Exit code 1 si au moins un projet échoue.

### Added

- **Fix #5 — Commande `oc doctor`** (`scripts/cmd-doctor.sh`) :
  nouvelle commande de diagnostic du hub. Vérifie les outils externes (jq, git, opencode, node, perl, sqlite3, bd),
  les fichiers de configuration (hub.json, providers.json, api-keys.local.md permissions),
  et l'état de déploiement de tous les projets enregistrés.
  Exit codes : 0 (tout OK), 1 (FAIL critique), 2 (WARN uniquement).
  Supporte i18n (FR/EN).

- **Fix #6 — `oc remove --dry-run`** (`scripts/cmd-remove.sh`) :
  nouveau flag `--dry-run / -n` qui simule la suppression sans effectuer de modifications.
  Affiche les fichiers/sections qui seraient supprimés. Pas de confirmation demandée.
  Compatible avec `--clean` : affiche aussi les fichiers déployés qui seraient supprimés.




- **`oc provider` supprimé** — toute la configuration provider passe désormais par `oc config` :
  - `oc provider set-default` → `oc config set` (mode interactif sans arguments)
  - `oc provider list` → `oc config list --providers`
  - `oc provider init [--force]` → `oc config init-providers [--force]`
  - `oc provider set-key <nom> <clé>` → `oc config set --provider <nom> --api-key <clé>`
  - `oc provider set-model <nom> <modèle>` → `oc config set --provider <nom> --model <modèle>`

### Added


- **Gate de complétion — enforcement CP-feature** (`skills/orchestrator/orchestrator-protocol.md`) :
  validation bloquante avant construction du CP-feature — vérifie que le rapport final
  d'orchestrator-dev documente les 3 checks (tests passés, comportement observable conforme,
  régressions documentées ou absence justifiée). Si absent → question à l'utilisateur :
  redemander à orchestrator-dev (avec `task_id`) / accepter / stop.

- **Signal `BLOCKED_ARCHITECTURE`** (`skills/developer/dev-standards-universal.md`) :
  documentation côté émetteur — 6 conditions de déclenchement (spec contradictoire, >3 fichiers
  imprévus, contrat d'interface incompatible, pattern architectural absent, dépendance bloquante,
  estimation dépassée ×2), format du rapport de dérive avec graduation des preuves, 3 options
  proposées à l'orchestrator-dev (réviser scope / revert / bifurquer).

- **Appel explicite `skill()` pour drift detection** (`skills/orchestrator/orchestrator-dev-protocol.md`) :
  remplacement de "activer le skill" par instruction explicite `skill("developer/dev-drift-detection")`
  lors de la réception du statut `BLOCKED_ARCHITECTURE` — garantit l'appel réel à l'outil.

- **Forensic debugger** (`skills/quality/debugger-workflow.md`, implémenté en v1.5.0, documenté maintenant) :
  mode `--forensic` avec graduation de preuves Confirmed/Deduced/Hypothesized, stronghold-first,
  case file `.investigation-{slug}.md`, missing evidence = finding, délégation si >5 fichiers
  ou >10K tokens.

- **Scale-Domain-Adaptive Planning** (`skills/planning/planner-workflow.md`, implémenté en v1.5.0, documenté maintenant) :
  Phase 0.5 — scoring de complexité 4 critères × 4 pts (domaines, intégrations, sécurité, taille
  codebase), 4 tiers Small/Medium/Large/Enterprise, conditionne pathfinder obligatoire et
  audit pré-implémentation selon le tier.

- **`skills/shared/hub-workflow-reference.md`** — source de vérité canonique du hub
  (`source-of-truth: true`, bucket A). Catalogue des agents (famille, mode, quand invoquer,
  output attendu), heuristique pathfinder vs planner (keywords, complexity scoring, règle de
  doute), séquences standard par type de feature, table des handoffs (émetteur → format skill
  → récepteur). Chargé automatiquement par `orchestrator` et `planner`.

- **ADR-018** (`docs/architecture/adr/018-hub-workflow-reference.fr.md` + `.en.md`) —
  statut Proposed → Accepted, implémenté le 2026-06-29.

- **`docs/guides/authoring-skills.md`** — nouveau guide dédié à la méthodologie d'authoring skills :
  4 types (Technique/Pattern/Reference/Discipline), TDD RED/GREEN/REFACTOR, SDO checklist
  (description discriminante, keyword coverage, token efficiency, cross-refs), rationalization
  table template, 5 anti-patterns (narrative, bilingue, label générique, code flowchart,
  over-spec), checklist de validation 12 points, règle de gouvernance hub-workflow-reference.
  Complémentaire à `docs/guides/authoring.fr.md`.

- **`skills/shared/skill-authoring-protocol.md`** — skill condensé bucket B, invocable par le
  `documentarian` via `skill("shared/skill-authoring-protocol")`. Version actionnable du guide
  authoring-skills.md : TDD en 3 étapes, SDO checklist tableau, rationalization table template,
  anti-patterns en 1 ligne, checklist 12 points.
  Ajouté dans `native_skills:` du `documentarian`.


- **`oc optimize` — analyse de gaspillage de tokens** — nouvelle commande qui scanne les patterns de gaspillage sans LLM et produit un rapport avec grade global A–F :
  - 9 analyses déterministes : MCP inutilisés, sessions sans edit, ratio Read/Edit, taux d'erreurs, fichiers re-lus, délégation lourde, skills inutilisées, sessions pure conversation, cache hit rate
  - Grade global basé sur le nombre et la sévérité des findings (critique × 3 + warning)
  - Options `--period today|week|month` (défaut 30j) et `--project PROJECT_ID`
  - `scripts/cmd-optimize.sh` (nouveau)
  - `tests/test_cmd_optimize.bats` : 20 tests (nouveau)

- **`oc yield` — corrélation sessions ↔ commits git** — nouvelle commande qui mesure le rendement réel des sessions OpenCode :
  - Classifie chaque session : Productive (commit dans les 24h), Abandonnée (aucun commit), Revertée (commit suivi d'un revert)
  - Résolution automatique des worktrees git vers le dépôt principal
  - Options `--period today|week|month` (défaut 7j) et `--project PROJECT_ID`
  - `scripts/cmd-yield.sh` (nouveau)
  - `tests/test_cmd_yield.bats` : 15 tests (nouveau)

- **`oc metrics` — section Activité** — nouvelle section dans les métriques basée sur les tool-use patterns :
  - 6 catégories déterministes : Code, Planification, Exploration, Review, Debug, Conversation
  - Affiche : nombre de sessions, coût total et pourcentage par catégorie
  - `tests/test_cmd_metrics.bats` : +4 tests section Activité

- **`scripts/lib/opencode-db.sh` — nouvelles fonctions tool-use** :
  - `ocdb_tool_stats(days, limit)` — statistiques des outils par fréquence
  - `ocdb_tool_count(tool, days)` — décompte d'un outil spécifique
  - `ocdb_tool_error_rate(days)` — taux d'erreurs des tool calls
  - `ocdb_activity_breakdown(days)` — répartition sessions par catégorie d'activité
  - `ocdb_sessions_no_edit(days, min_cost)` — sessions coûteuses sans modification
  - `ocdb_avg_read_edit_ratio(days)` — ratio Read/Edit moyen
  - `ocdb_sessions_heavy_delegation(days)` — sessions à délégation lourde
  - `ocdb_repeated_reads(days, threshold)` — fichiers re-lus excessivement
  - `ocdb_unused_mcp(days)` — MCP servers déployés mais inutilisés
  - `tests/test_lib_opencode_db.bats` : table `part` ajoutée au schéma de test

- **`oc.sh`** : ajout des commandes `optimize` et `yield`
- **`docs/reference/cli.fr.md` + `cli.en.md`** : sections `oc optimize` et `oc yield` documentées


- **`oc metrics` — refonte complète** — les métriques passent d'un système dépendant de l'écriture par l'orchestrateur (`metrics.jsonl`) à une collecte passive read-only depuis la base SQLite OpenCode (`~/.local/share/opencode/opencode.db`). Aucune modification des permissions des agents orchestrateurs requise :
  - `scripts/lib/opencode-db.sh` : nouvelle bibliothèque de requêtes SQLite read-only (coûts, tokens, cache hit rate, sessions, agents, modèles)
  - `scripts/cmd-metrics.sh` : refonte — sections Vue globale (coût USD, tokens input/output, cache write/read, cache hit rate + économies estimées), Coût par projet, Top agents, Top modèles, Sessions récentes, Tickets bd et Vélocité workflow (rétrocompat.)
  - Nouvelle option `--period today|week|month` pour filtrer la période (défaut : 7 jours)
  - Fallback gracieux si `sqlite3` absent ou base inaccessible (warn + exit 0, les autres sections restent disponibles)
  - `tests/test_lib_opencode_db.bats` : 26 nouveaux tests couvrant disponibilité, chemins, requêtes, agrégation, formatage
  - `tests/test_cmd_metrics.bats` : 11 nouveaux tests (SQLite, périodes, sqlite3 absent, cache hit rate) + 12 tests rétrocompat. conservés

- **`oc dashboard` — refonte multi-projet** — le dashboard passe d'une vue mono-session (dépendant de `session-state.json` écrit par l'orchestrateur) à un dashboard multi-projet passif :
  - `scripts/cmd-dashboard.sh` : refonte — sections Projets (bd, tickets par statut pour chaque projet), Session orchestrateur active (rétrocompat.), Budget sessions (aujourd'hui / semaine / mois), Sessions récentes, Top agents
  - Sources de données exclusivement en lecture : `opencode.db` + `bd list` + `session-state.json` (rétrocompat.)
  - Fallback gracieux si `sqlite3` ou `bd` absent — sections concernées dégradées sans bloquer les autres
  - `tests/test_cmd_dashboard.bats` : 9 nouveaux tests (header, budget, sessions récentes, agents, fallbacks sqlite3/bd, rétrocompat.) + 16 tests existants conservés

- **Prérequis `sqlite3`** — ajouté dans l'installation et la documentation :
  - `install.sh` : vérification de `sqlite3` avec proposition d'installation sur Linux (`apt-get install sqlite3`) et warning informatif (non-bloquant) sur macOS
  - `scripts/cmd-install.sh` : idem pour `oc install`
  - `README.md` + `README.fr.md` : `sqlite3` ajouté dans la section Requirements/Prérequis
  - `docs/guides/getting-started.fr.md` + `.en.md` : `sqlite3` ajouté dans la table des prérequis avec note "natif macOS"
  - `docs/reference/cli.fr.md` + `cli.en.md` : nouvelles sections `oc metrics` et `oc dashboard` documentant les sources de données, sections, options et comportements de fallback


- **Enrichissement continu des documents vivants — extension à tous les agents** — le mécanisme d'amélioration continue de `ONBOARDING.md` et `CONVENTIONS.md` est étendu de 3 agents (auditor, planner, debugger) à l'ensemble du hub. Chaque agent propose désormais systématiquement la capitalisation de ses découvertes après son travail, toujours avec confirmation explicite de l'utilisateur et délégation au `documentarian` :
  - `skills/auditor/living-docs-enrichment.md` → déplacé vers `skills/shared/living-docs-enrichment.md` (nouveau path) + enrichi avec les nouvelles sources de découvertes (developer-*, reviewer, qa-engineer, pathfinder, onboarder en mode re-onboarding)
  - `agents/planning/onboarder.md` : ajout du skill `shared/living-docs-enrichment` + comportement Phase 5 adapté — si `ONBOARDING.md`/`CONVENTIONS.md` existent déjà, propose enrichissement incrémental (via `documentarian`) plutôt que réécriture silencieuse ; réécriture complète reste disponible avec avertissement explicite sur la perte des enrichissements accumulés
  - `agents/planning/pathfinder.md` : ajout du skill `shared/living-docs-enrichment` — propose la capitalisation des patterns architecturaux détectés en fin de rapport
  - `agents/quality/reviewer.md` : ajout du skill `shared/living-docs-enrichment` + permission `task.documentarian: allow` — propose la capitalisation des conventions observées dans le diff après le rapport de review
  - `agents/quality/qa-engineer.md` : ajout du skill `shared/living-docs-enrichment` + permission `task.documentarian: allow` — propose la capitalisation des conventions de test et edge cases systématiques révélés après le rapport de couverture
  - `agents/developer/*.md` (11 agents) : ajout du skill `shared/living-docs-enrichment` + permission `task.documentarian: allow` dans tous les agents developer-*
  - `skills/developer/beads-dev.md` : déclenchement du skill `shared/living-docs-enrichment` explicitement documenté après chaque `bd close`
  - `skills/planning/onboarder-workflow.md` Phase 5 : blocs "⚠️ Si ONBOARDING.md / CONVENTIONS.md existe déjà" remplacés — 3 options désormais proposées (enrichissement incrémental recommandé / réécriture complète avec avertissement / conserver l'existant)
  - `skills/auditor/audit-protocol-light.md` : référence `living-docs-enrichment` → `shared/living-docs-enrichment`
  - Agents existants (auditor, planner, debugger) : référence `auditor/living-docs-enrichment` → `shared/living-docs-enrichment` dans leur frontmatter

- **Documentation mise à jour** :
  - `docs/architecture/skills.en.md` + `.fr.md` : entrée `living-docs-enrichment` déplacée du domaine `auditor/` vers un nouveau bloc `shared/` ; agents mis à jour (onboarder, pathfinder, reviewer, qa-engineer, developer-* ajoutés) ; matrice de dépendances mise à jour pour tous les agents concernés
  - `docs/architecture/agents.en.md` + `.fr.md` : paths `auditor/living-docs-enrichment` → `shared/living-docs-enrichment` ; paragraphes "living docs enrichment" ajoutés pour onboarder, pathfinder, reviewer, qa-engineer et developer-* ; règles communes mises à jour
  - `docs/architecture/overview.en.md` + `.fr.md` : Principe 5 étendu — liste tous les agents participants à la boucle d'enrichissement continu
  - `docs/architecture/adr/010-hybrid-skills-architecture.en.md` + `.fr.md` : path `shared/living-docs-enrichment` mis à jour
  - `docs/guides/workflows.en.md` + `.fr.md` : Scénarios 4 (implémentation feature → docs vivants) et 5 (code review → docs vivants) ajoutés ; scénarios existants renumérotés (4-7 → 6-9)
  - `docs/guides/onboarding.en.md` + `.fr.md` : comportement incrémental documenté (description read-only + nouvelle section "Re-onboarding — incremental mode")



- **Suppression de la duplication des retours agents/sous-agents** — le contenu narratif et le bloc structuré de handoff réencodaient les mêmes données, produisant des retours redondants visibles par l'utilisateur. Le principe adopté : le narratif apporte le *contexte et le raisonnement* (preuves, décisions, pourquoi), le bloc structuré apporte les *métadonnées actionnables* (tableaux, statuts, routing). Les deux sont complémentaires et non redondants.
  - `skills/orchestrator/orchestrator-dev-protocol.md` : le template `## Récap implémentation` ne contient plus le tableau des tickets ni les statistiques (traités/ignorés/cycles/corrections) — ces données restent dans le bloc structuré `## Retour vers orchestrator`. Le récap narratif contient désormais uniquement les comptes rendus d'implémentation verbatim et les points d'attention agrégés.
  - `skills/orchestrator/orchestrator-handoff-format.md` : description du point 1 mise à jour pour refléter que le récap narratif ne réencode pas le tableau (qui est dans le bloc structuré).
  - `skills/planning/planner-handoff-format.md` + `skills/planning/planner-workflow.md` : le récapitulatif de planification apporte le contexte et le raisonnement, pas un ré-encodage du tableau des tickets du bloc structuré.
  - `skills/developer/developer-handoff-format.md` : le compte rendu d'implémentation = prose (décisions, contexte) sans répétition des listes techniques du bloc.
  - `skills/qa/qa-handoff-format.md` : le rapport QA = analyse narrative (raisonnement sur la couverture) sans répétition des listes de tests du bloc.
  - `skills/quality/debugger-handoff-format.md` : le rapport de diagnostic = symptôme + preuves + contexte sans répétition de la cause racine structurée du bloc.
  - `skills/auditor/audit-handoff-format.md` : le rapport d'audit = preuves et chemins d'exploitation sans répétition du tableau de synthèse du bloc.
  - `skills/planning/onboarder-handoff-format.md` : le rapport d'onboarding = récit de découverte sans répétition des listes structurées (stack, conventions, dette) du bloc.
  - Documentation mise à jour : `docs/architecture/task-delegation.fr.md`, `docs/architecture/adr/009-inter-agent-handoff-contracts.fr.md` + `.en.md`, `docs/architecture/skills.fr.md`, `docs/guides/workflows.fr.md` + `.en.md`, `docs/architecture/overview.fr.md` + `.en.md`, `docs/guides/inter-agent-interruption.fr.md`.


- **Mécanisme d'interruption de session étendu à tous les agents invocables** — `orchestrator-dev`, `onboarder`, `auditor`, `debugger`, `ux-designer` et `ui-designer` implémentent maintenant le mécanisme d'interruption de session permettant à l'utilisateur de voir les récaps intermédiaires et de répondre aux questions avant chaque checkpoint, même quand ces agents sont invoqués via `task` depuis l'orchestrateur feature :
  - `skills/orchestrator/orchestrator-dev-protocol.md` : CP-1, CP-QA (risque moyen/faible), CP-3 et branche dédiée produisent désormais un bloc `## Question pour l'orchestrator` + `## Retour vers orchestrator` (partiel) et terminent la session en mode `orchestrateur_feature` (mode `manuel` uniquement — CP-2, modes semi-auto et auto : comportement inchangé)
  - `skills/planning/onboarder-workflow.md` : Phases 0 à 4 — blocs `## Retour intermédiaire vers orchestrateur` + `## Question pour l'orchestrateur` + terminaison de session remplacent l'outil `question` en mode `orchestrateur_feature` ; retrait de la règle obsolète du "condensé dans le champ question"
  - `skills/auditor/auditor-workflow.md` : Phases 0 à 3 — même mécanisme
  - `skills/quality/debugger-workflow.md` : tous les checkpoints (fin de phase, pause artefacts, clarifications, confirmation ticket Beads, retours en arrière) — même mécanisme
  - `agents/design/ux-designer.md`, `agents/design/ui-designer.md` : section "Contexte d'invocation" ajoutée — l'outil `question` est interdit en mode `orchestrateur_feature`
  - `skills/design/design-handoff-format.md` : condition d'activation du bloc `## Retour vers orchestrator` rendue explicite via le marqueur `[CONTEXTE]` ; blocs `## Retour intermédiaire vers orchestrateur` et `## Question pour l'orchestrateur` ajoutés pour les clarifications critiques
  - `skills/designer/ui-protocol.md` : branche `orchestrateur_feature` pour la question "aucun design system détecté"
  - `skills/orchestrator/orchestrator-protocol.md` : marqueurs `[CONTEXTE]` ajoutés dans les invocations de `debugger`, `onboarder`, `auditor`, `ux-designer`, `ui-designer` ; sections "Réception d'une question montante" ajoutées pour ces agents ; templates de retranscription enrichis avec les blocs intermédiaires
  - `skills/posture/retranscription-coordinateur.md` : tableau "Règles par type de retour" étendu à tous les agents (final + question montante) ; template pour questions montantes généralisé

- **Nouveau guide** `docs/guides/inter-agent-interruption.fr.md` — documentation de référence complète sur le mécanisme d'interruption de session inter-agents : principe, format des blocs, flux de reprise avec `task_id`, guide d'implémentation pour les auteurs de skills, limites connues

- **Nouveaux tests** `tests/test_integration_inter_agent_interruption.bats` — 37 tests structurels vérifiant la présence des blocs, marqueurs et sections requis dans tous les fichiers skills/agents implémentant le mécanisme d'interruption

- **Documentation mise à jour** : `docs/architecture/task-delegation.fr.md` (tableau des agents + tableau des checkpoints étendu + note sur les deux variantes de blocs), `docs/architecture/agents.fr.md` (comportement en mode `orchestrateur_feature` ajouté pour les 6 agents concernés)

- **`oc service` — gestion générique des intégrations MCP** — nouvelle commande unifiée pour configurer, valider et gérer les services externes connectés via MCP :
  - `oc service list` : catalogue des services disponibles avec leur état (configuré / non configuré)
  - `oc service setup [nom]` : wizard interactif en N+2 étapes (credentials → validation API → sauvegarde & build MCP)
  - `oc service status [nom]` : état détaillé (credentials masqués, validité token, build MCP)
  - `oc service remove <nom>` : suppression de la configuration (avec confirmation)
  - Aliases raccourcis : `oc figma <cmd>` et `oc gitlab <cmd>` redirigent vers `oc service`
  - Catalogue extensible via `config/services.json` — ajouter un service = ajouter une entrée JSON, aucune modification de code
  - Bilingue FR/EN (détection via `OC_LANG`)
  - Compatible mode non-interactif (`OC_NON_INTERACTIVE=1` + env vars pré-définies pour CI/CD)
  - Stockage dans `~/.config/opencode/config.json` (section `env`) — compatible avec le mécanisme MCP existant
  - Services disponibles : **Figma** (`figma-mcp`) et **GitLab** (`gitlab-mcp`)
  - Nouvelle bibliothèque partagée `scripts/lib/services.sh` — 37 tests unitaires
  - 20 tests d'intégration pour `cmd-service.sh`
  - Documentation : `docs/reference/services.fr.md` + `docs/reference/services.en.md`

- **MCP Server GitLab** (`servers/gitlab-mcp/`) — nouveau serveur MCP en lecture seule pour l'intégration GitLab :
  - 5 tools MCP : `get_gitlab_issue`, `list_gitlab_issues`, `get_gitlab_merge_request`, `list_gitlab_labels`, `list_gitlab_milestones`
  - Client `GitLabClient` avec axios, retry/backoff exponentiel (429/503/504 + erreurs réseau) et `classifyGitlabError()`
  - Support instances self-hosted via `GITLAB_BASE_URL`
  - SDK `@modelcontextprotocol/sdk` upgradé vers `^1.11.0` (figma-mcp + gitlab-mcp — 27 tests existants passent)
  - 4 skills adapters : `adapters/gitlab-orchestrator-protocol`, `gitlab-pathfinder-protocol`, `gitlab-planner-protocol`, `gitlab-onboarder-protocol`
  - Agents mis à jour : `orchestrator`, `pathfinder`, `planner`, `onboarder` — `mcpServers: [gitlab]` + skill adapters ajoutés
  - Déploiement automatique via `scripts/lib/mcp-deploy.sh` (case `gitlab-mcp` ajouté)
  - Configuration via `oc service setup gitlab` / `oc gitlab setup`
  - Documentation : `docs/guides/gitlab-integration.fr.md` + `docs/guides/gitlab-integration.en.md`

- **`oc config set` unifié (hub-level)** — `oc config set` sans PROJECT_ID supporte désormais tous les flags provider :
  - Mode interactif (sans flags) : lance le wizard de sélection provider identique à l'ancien `oc provider set-default`
  - `--provider <nom>` — configure le provider par défaut du hub
  - `--api-key <clé>` — configure la clé API du provider
  - `--model <modèle>` — met à jour le modèle par défaut du hub (`.opencode.model`)
  - `--base-url <url>` — configure l'URL de base (litellm, ollama, etc.)
  - Les flags `--family-model` et `--agent-model` existants restent inchangés
  - Tous les flags peuvent être combinés en une seule commande
- **`oc config list --providers`** — affiche le catalogue des providers disponibles avec leur statut hub
- **`oc config init-providers [--force]`** — initialise les fichiers `config/providers/*.json` pour le switcher `ocp`

### Changed


- **`agents/planning/orchestrator.md`** : ajout `shared/hub-workflow-reference` dans `skills:` ;
  `## Agents disponibles` → pointeur ; heuristique pathfinder/planner (Mode E) → pointeur.
  Supprime ~80 lignes de duplication.

- **`skills/planning/planner-workflow.md`** : reformulation de "seule source de vérité"
  (décision de routing ≠ catalogue) ; `### Agents disponibles pour le routing` → pointeur.
  Supprime ~15 lignes de duplication.

- **`skills/orchestrator/orchestrator-protocol.md`** : `## Routing` — pointeur mis à jour
  de `orchestrator.md` vers `shared/hub-workflow-reference`.

- **`agents/planning/planner.md`** : ajout `shared/hub-workflow-reference` dans `skills:`.

- **Réduction de la verbosité des hand-offs et récapitulatifs agents** — suppression des duplications dans les chaînes de retour inter-agents :

  - **Récap global `orchestrator-dev` → `orchestrator`** : le `## Récap implémentation` passe d'une copie verbatim des comptes rendus narratifs developer-* à une synthèse structurée par ticket (statut, fichiers clés, critères couverts, points d'attention). Supprime N comptes rendus complets dans le fil de discussion pour N tickets traités.
    - `skills/orchestrator/orchestrator-dev-protocol.md` : section "Récap global — Fin de session" (Étape 1)
    - `skills/orchestrator/orchestrator-handoff-format.md` : règle de production et règle consommateur
    - `skills/developer/developer-handoff-format.md` : règle consommateur orchestrator-dev (point 6)

  - **Blocage 3 cycles** : le `### Contexte complet` du bloc `## Question pour l'orchestrator` passe de 3 rapports de review complets à une synthèse des problèmes persistants + résumé 1 ligne par cycle. Supprime jusqu'à 3 rapports de review complets dans le bloc de question.
    - `skills/orchestrator/orchestrator-dev-protocol.md` : section "Blocage après 3 cycles de review" (mode invoqué)
    - `skills/orchestrator/orchestrator-handoff-format.md` : tableau des CPs (ligne Blocage 3 cycles)

  - **Blocs `## Retour intermédiaire vers orchestrateur` (planner, auditor)** : le contenu des blocs passe de "récap de la phase reproduit intégralement" à "synthèse condensée (résumé 2-3 phrases + points clés)". La prose libre de la phase reste inchangée (valeur CoT). Supprime la double écriture du même contenu dans chaque réponse de phase.
    - `skills/planning/planner-workflow.md` : template générique + blocs des phases 0, 1 (×2), 1.5, 2, 3, 4, 5.5
    - `skills/auditor/auditor-workflow.md` : template générique + blocs des phases 0, 1, 2, 3

  - **CP-feature orchestrator** : suppression de la re-reproduction du récap global au CP-feature (il est déjà affiché lors de la réception du retour final Cas A).
    - `skills/orchestrator/orchestrator-protocol.md` : section "CP-feature — Récap global"

- **Documentation mise à jour** en cohérence avec les changements ci-dessus :
  - `docs/architecture/adr/009-inter-agent-handoff-contracts.fr.md` + `.en.md` : amendement "Récap d'implémentation condensé"
  - `docs/guides/workflows.fr.md` + `.en.md` : diagramme Mermaid + description étape 10
  - `docs/architecture/overview.fr.md` + `.en.md` : diagramme Mermaid
  - `docs/architecture/task-delegation.fr.md` : règle verbatim récap
  - `docs/architecture/skills.fr.md` + `.en.md` : description `orchestrator-handoff-format`
  - `docs/guides/inter-agent-interruption.fr.md` : format du bloc `## Retour intermédiaire`


- **Agent natif `scout` désactivé par défaut** — OpenCode v1.16.0 introduit un agent natif `scout` (subagent read-only pour la recherche de documentation et dépendances externes) dont l'ID entre en collision avec l'agent hub `planning/pathfinder`. Il est maintenant masqué au même titre que `build`, `plan`, `general` et `explore` :
  - `config/hub.json` : `scout` ajouté dans `opencode.disabled_native_agents`
  - `config/hub.json.example` : idem pour les nouvelles installations
  - `scripts/lib/agent-picker.sh` : `scout` ajouté dans `_pick_native_agents()` (TUI `oc init`) avec description "Recherche de documentation et dépendances externes" — `_pick_total` passe de 4 à 5
  - `scripts/lib/project.sh` : `scout` ajouté dans le squelette fallback de `hub.json`
  - `docs/reference/config.fr.md` + `docs/reference/config.en.md` : listes canoniques de `disabled_native_agents` et `Disable agents` mises à jour (L38/L142 FR, L40/L144 EN)


- **Agent `onboarder`** (`agents/planning/onboarder.md`) :
  - Description enrichie : mentionne les nouvelles capacités (contexte métier, Figma, stratégie de test)
  - Skills mis à jour : ajout de `adapters/figma-onboarder-protocol`
  - `mcpServers: [figma]` ajouté pour accès aux tools Figma
  - Workflow `planning/onboarder-workflow` enrichi avec Phases 1.4, 1.5, 1.6
- **Skill `planning/onboarder-workflow`** (`skills/planning/onboarder-workflow.md`) :
  - Ajout Phase 1.4 (Exploration contexte métier) : 7 domaines détectables, analyse sémantique codebase, extraction concepts ≥ 3 occurrences
  - Ajout Phase 1.5 (Exploration Figma, optionnelle) : recherche fichiers, analyse design system, extraction tokens
  - Ajout Phase 1.6 (Exploration stratégie de test) : frameworks, organisation, ratio test/source, philosophie TDD/BDD
  - Récap Phase 1 enrichi avec 3 nouvelles sections
  - Questions Phase 2 enrichies (métier, test, Figma)
  - Templates ONBOARDING.md et CONVENTIONS.md enrichis avec nouvelles sections
- **Skill `planning/onboarder-handoff-format`** (`skills/planning/onboarder-handoff-format.md`) :
  - 3 nouveaux champs dans le bloc `## Retour vers orchestrator` : Contexte métier, Design et maquettes, Stratégie de test
  - Checklist consommateur enrichie avec vérification des 9 champs (au lieu de 6)
- **MCP Server Figma** (`servers/figma-mcp/`) :
  - Client Figma : méthode `getDesignTokens()` ajoutée pour interroger l'API Figma Variables
  - Index : tool `extract_design_tokens` ajouté à la liste des tools disponibles
  - README : fonctionnalités et architecture mises à jour, mention des 3 agents utilisateurs (pathfinder, planner, onboarder)
- **Documentation** :
  - `README.md` / `README.fr.md` : description onboarder enrichie, section "Figma Integration" mise à jour (3 agents au lieu de 2, nouvelles capacités)
  - `docs/architecture/agents.fr.md` : section onboarder enrichie avec détail des 3 nouvelles phases, mention MCP Server figma
  - `docs/architecture/skills.fr.md` : nouvelle section "Domaine — `adapters/`" avec 3 skills Figma (pathfinder, planner, onboarder), matrice de dépendances mise à jour
- **Agent `auditor`** (`agents/auditor/auditor.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 4 enrichie — consolidation des sections `### Découvertes à documenter` et proposition d'enrichissement après la synthèse exécutive ; permission `task.documentarian = allow` ajoutée
- **Agent `planner`** (`agents/planning/planner.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 6 enrichie — identification des patterns et conventions observés, proposition d'enrichissement après validation du plan ; permission `task.documentarian = allow` ajoutée
- **Agent `debugger`** (`agents/quality/debugger.md`) : skill `auditor/living-docs-enrichment` ajouté ; Phase 5 enrichie — identification des zones d'ombre levées et patterns d'erreur, proposition d'enrichissement après le rapport ; permission `task.documentarian = allow` ajoutée
- **Agents `auditor-*`** (×7) : ajout de la section `### Découvertes à documenter` en fin de rapport — lecture seule stricte conservée (`write: deny`, aucun appel `task`)
- **Agent `planner`** (`agents/planning/planner.md`) : skills mis à jour — `planning/planner-workflow` remplace `planning/planner` + les 3 skills `analysis/*` ; `planning/planner-handoff-format` conservé
- **Agent `onboarder`** (`agents/planning/onboarder.md`) : skills mis à jour — `planning/onboarder-workflow` remplace `planning/project-discovery`, `planning/project-conventions` + les 3 skills `analysis/*` ; `planning/onboarder-handoff-format` conservé
- **Agent `debugger`** (`agents/quality/debugger.md`) : skills mis à jour — `quality/debugger-workflow` remplace `debugger/debug-protocol` ; `quality/debugger-handoff-format` conservé
- **Agent `auditor`** (`agents/auditor/auditor.md`) : skills mis à jour — `auditor/auditor-workflow` remplace `auditor/audit-protocol` + les 3 skills `analysis/*`
- **Agents `auditor-*`** (7 sous-agents) : les 3 skills `analysis/*` retirés du frontmatter — les sous-agents spécialisés n'en avaient pas besoin (workflow technique simple)
- **`oc list`** — conservé comme alias silencieux vers `oc status --short` (backward compat), retiré du `oc help`
- **`oc provider set <PROJECT_ID>`** et **`oc provider get <PROJECT_ID>`** — supprimés ; utiliser `oc config set/get <PROJECT_ID>` à la place (message d'erreur clair si l'ancienne forme est utilisée)
- **`oc config set`** — le sélecteur de provider est désormais un menu numéroté dynamique depuis `providers.json` (au lieu d'une liste statique codée en dur)
- **`oc update`** — description clarifiée : met à jour les outils installés (opencode, bd, skills externes)
- **`oc upgrade`** — description clarifiée : met à jour les sources du hub via git (git pull ou checkout tag)
- **`oc agent keytest`** — retiré du `oc help` (toujours utilisable, non documenté)
- **`lib/providers.sh`** — helpers `_build_provider_menu` et `_collect_provider_credentials` extraits et partagés (plus de duplication entre `cmd-config.sh` et `cmd-provider.sh`)
- **Agent `documentarian`** — frontmatter `skills:` enrichi avec `documentarian/doc-slides`
- **Agent `documentarian`** — section "Ce que tu fais" : ajout de la génération de présentations Marp
- **Agent `documentarian`** — table d'exemples : 2 nouveaux cas (`"Crée une présentation pour la démo v2.0"`, `"Slides de retrospective sprint 42"`)
- **Skill `documentarian/doc-protocol`** — tableau de routing : ajout de la ligne `slides, présentation, deck, pitch, diaporama, démo visuelle, retro, onboarding formation` → `doc-slides`
- **Agents producteurs** — frontmatter `skills:` mis à jour pour inclure le skill de handoff correspondant :
  - `ux-designer`, `ui-designer` → `design/design-handoff-format`
  - `auditor-security`, `auditor-performance`, `auditor-accessibility`, `auditor-ecodesign`, `auditor-architecture`, `auditor-privacy`, `auditor-observability` → `auditor/audit-handoff-format`
  - `planner` → `planning/planner-handoff-format`
  - `onboarder` → `planning/onboarder-handoff-format`
  - `debugger` → `quality/debugger-handoff-format`
  - `reviewer` → `reviewer/reviewer-handoff-format`
  - `qa-engineer` → `qa/qa-handoff-format`
  - Tous les `developer-*` (×9) → `developer/developer-handoff-format`
- **Agent `orchestrator`** — frontmatter enrichi avec les 5 skills de handoff côté consommateur : `design/design-handoff-format`, `auditor/audit-handoff-format`, `planning/planner-handoff-format`, `planning/onboarder-handoff-format`, `quality/debugger-handoff-format`
- **Agent `orchestrator-dev`** — frontmatter enrichi avec les 3 skills de handoff côté consommateur : `developer/developer-handoff-format`, `reviewer/reviewer-handoff-format`, `qa/qa-handoff-format`
- **Skill `orchestrator/orchestrator-dev-protocol`** :
  - Étape 2 (délégation developer) : détection du bloc `## Retour vers orchestrator-dev` ; routing `bloqué` vers la gestion de blocage sans soumettre à review
  - Étape 3 (QA) : invocation qa-engineer enrichie avec les critères d'acceptance déjà couverts par le developer ; détection du statut QA avant de continuer ; transmission des critères non couverts au reviewer si `couverture-partielle`
  - Étape 4 (review) : invocation reviewer enrichie avec les `### Points d'attention pour la review` du developer
  - Étape 5 (CP-2) : routing de correction basé sur le `### Routing recommandé` du reviewer ; commentaire Beads rempli avec les `### Corrections requises` verbatim (plus de résumé manuel)
  - Étape 6 (compte rendu) : compte rendu enrichi avec fichiers modifiés, couverture des critères d'acceptance, points d'attention techniques agrégés depuis les sous-agents
  - Récap global : colonne `Critères couverts` ajoutée ; `### Points d'attention` alimentés par l'agrégation des retours de toute la chaîne
- **Migration syntaxe outil `question`** — Mise à jour de 98 occurrences dans 18 fichiers skills vers la syntaxe correcte `question({ questions: [{...}] })` conforme au schéma JSON d'OpenCode
- **Refonte du skill `posture/tool-question.md`** — Documentation complète avec :
  - Schéma JSON détaillé de l'outil
  - Support multi-questions (plusieurs questions en un seul appel)
  - Support multi-sélection avec `multiple: true`
  - Documentation de l'option de saisie libre automatique ("Type your own answer")
  - Format des réponses (tableau de labels)
  - Exemples complets pour chaque cas d'usage

### Fixed


- **`oc deploy` bloqué si `jq` absent — les agents natifs OpenCode n'étaient pas désactivés** — sur une installation fraîche où `jq` était refusé lors de l'install, `get_hub_disabled_native_agents()` retournait silencieusement `""` (guard `return 0`), et `adapter_deploy_config()` ne générait aucune entrée `{"disable": true}` dans `opencode.json`. Les agents natifs OpenCode (`build`, `plan`, `general`, `explore`, `pathfinder`) restaient actifs alors qu'ils devaient être masqués :
  - `scripts/adapters/opencode.adapter.sh` — `adapter_validate()` : `jq` est désormais vérifié au même titre qu'`opencode` ; deploy bloqué avec message d'erreur si `jq` absent
  - `scripts/lib/project.sh` — `get_hub_disabled_native_agents()` : fallback bash (grep/sed) implémenté pour parser `disabled_native_agents` depuis `hub.json` sans `jq` ; la désactivation des agents natifs fonctionne même si `jq` est temporairement absent
  - `scripts/adapters/opencode.adapter.sh` — `adapter_deploy_config()` : warning explicite si `disabled_csv` est vide alors que la clé `disabled_native_agents` est absente de `hub.json`
  - `install.sh` : message de déclin de `jq` précisé — `"Sans jq, oc deploy sera bloqué"` au lieu de `"Certaines fonctionnalités seront dégradées"`
  - `tests/test_lib_project.bats` : 5 nouveaux tests pour `get_hub_disabled_native_agents()` (array non-vide, array vide, hub.json absent, fallback bash CSV, fallback bash vide)
  - `tests/test_opencode_adapter.bats` : 3 nouveaux tests pour la génération `{"disable": true}` dans `opencode.json` (agents désactivés hub, tableau vide, priorité projet > hub)
  - `tests/fixtures/configs/hub_with_disabled_agents.json` : nouveau fichier fixture avec les 5 agents natifs désactivés


- **`task_id` — nature clarifiée et garde-fou ajouté** (`task_id-delegation.fr.md`, `orchestrator-protocol.md`) :
  - Le `task_id` est un ID de session OpenCode standard (session persistée côté serveur, non un état LLM) — la reprise de session est fiable tant que la session existe ; le risque de "perte de contexte LLM" n'existe pas
  - Risque résiduel documenté : session introuvable si OpenCode redémarre pendant la fenêtre d'attente entre question montante et reprise
  - Garde-fou ajouté dans `orchestrator-protocol` : "Cas C — session introuvable" — détecter l'absence de résultat après ré-invocation avec `task_id` et proposer à l'utilisateur de relancer depuis les tickets restants ou de stopper
- **Récap partiel vs final** (`orchestrator-handoff-format`, `orchestrator-protocol`, `orchestrator-dev-protocol`) — la distinction entre récap partiel (émis avec une question montante) et récap final (émis seul en fin de session) était implicite et reposait sur un signal contextuel ; rendu explicite par l'ajout d'un champ obligatoire `**Type de récap :** partiel | final` dans le bloc `## Retour vers orchestrator` ; règles de détection et d'interdiction ajoutées dans les trois skills concernés
- **Transmission du mode de workflow** (`orchestrator-workflow-modes`, `orchestrator-protocol`, `orchestrator-dev-protocol`) — le mode (`manuel`/`semi-auto`/`auto`) était transmis via texte libre sans contrat de format ; cinq correctifs appliqués :
  - Valeurs canoniques définies (`manuel`, `semi-auto`, `auto`) et interdiction des labels bruts d'interface
  - Autocontrôle avant délégation côté orchestrator
  - Re-transmission obligatoire du mode dans chaque prompt de reprise `task_id` (correctif critique — corrige une perte silencieuse du mode à chaque CP-2)
  - Règle de parsing documentée côté orchestrator-dev avec fallback `manuel` explicite et signal d'alerte
  - Confirmation visible du mode reçu dans le message de démarrage d'orchestrator-dev
- **`orchestrator-dev` → `orchestrator` — remontée du bloc `## Retour vers orchestrator` manquante** : le bloc de retour n'était pas produit de manière fiable en fin de session d'`orchestrator-dev` quand invoqué depuis l'orchestrateur feature, empêchant la construction du CP-feature. Corrections apportées :
  - Ajout d'une règle absolue (`✅`) dans la section "Règles absolues" d'`orchestrator-dev-protocol` : le bloc est obligatoire sans exception, y compris en cas de stop, ticket bloqué ou session partielle
  - Transformation de la note conditionnelle de fin de section en une **Étape 2 numérotée et obligatoire** dans la section "Récap global", avec autocontrôle explicite avant clôture de session
  - Ajout dans la section "Ce que tu ne fais PAS" : interdiction de clore la session sans avoir produit le bloc
  - Renforcement du skill `orchestrator/orchestrator-handoff-format` côté producteur : rappel explicite que le bloc est requis même en cas de stop ou de session incomplète, avec autocontrôle
- **Skill `orchestrator/orchestrator-protocol`** :
  - Mode A : détection du bloc structuré `## Retour vers orchestrator` du planner ; présentation des hypothèses et risques au CP-0 avant de démarrer
  - Mode C : détection du bloc structuré de l'onboarder ; présentation des zones d'incertitude et dette technique au CP-onboard
  - Mode D : détection du bloc structuré du debugger ; présentation prioritaire des actions d'urgence si bug en prod
  - Tickets spec-ux/spec-ui : détection du bloc structuré des agents design ; transmission intégrale des `### Contraintes d'implémentation` à orchestrator-dev lors de l'implémentation
  - Tickets audit : détection du bloc structuré des auditors ; transmission intégrale des `### Recommandations priorisées` à orchestrator-dev

### Removed


- **`scripts/cmd-provider.sh`** — supprimé, fonctionnalités absorbées par `cmd-config.sh`
- **`oc provider`** — commande supprimée de `oc.sh`

- **Enrichissement agent `onboarder` v1.1** — 3 nouvelles phases d'exploration pour un onboarding complet :
  - **Phase 1.4 — Exploration contexte métier** : détection automatique du domaine (e-commerce, fintech, santé, RH, SaaS, éduc, immobilier), identification des utilisateurs cibles, extraction des concepts clés (≥ 3 occurrences), analyse sémantique de la codebase (classes, interfaces, services), détection du glossaire (`docs/glossary.md`), identification du pattern d'architecture (DDD, CQRS, Layered, MVC), analyse des tickets Beads pour patterns métier récurrents
  - **Phase 1.5 — Exploration Figma** (optionnelle, si frontend détecté) : recherche automatique des maquettes par nom de projet, analyse de 3 fichiers max (les plus pertinents), détection automatique du design system (DSFR, Material Design, Ant Design, Custom), extraction des design tokens depuis Figma Variables (couleurs, typographie, espacements, effets) via nouveau tool MCP `extract_design_tokens`, intégration dans `ONBOARDING.md` (section "Design et maquettes") et `CONVENTIONS.md` (section "Design tokens")
  - **Phase 1.6 — Exploration stratégie de test** : détection des frameworks (Vitest, Jest, pytest, PHPUnit, Playwright, Cypress), analyse de l'organisation (co-localisés vs dossier séparé), calcul du ratio test/source (bonne ≥ 0.8, partielle 0.4-0.8, faible < 0.4), identification de la philosophie (TDD via labels Beads, BDD via Cucumber/Behave, test-after par défaut), extraction du seuil de couverture configuré, détection des commandes de test
- **Nouveau tool MCP Figma `extract_design_tokens`** (`servers/figma-mcp/src/tools/extract-design-tokens.ts`) — extraction automatique des design tokens depuis Figma Variables : couleurs (conversion RGBA → hex), typographie (fontFamily, fontSize, fontWeight), espacements, effets (shadows). Gère les cas où les Variables ne sont pas configurées (retour vide avec message informatif). Méthode `getDesignTokens()` ajoutée au client Figma.
- **Nouveau skill `adapters/figma-onboarder-protocol`** — protocole complet d'intégration Figma dans l'onboarder : déclenchement conditionnel (si frontend détecté), workflow en 4 étapes (recherche → analyse → identification design system → récap), critères de détection DSFR/Material/Ant Design/Custom, extraction tokens, intégration dans templates de sortie, 4 exemples concrets (backend skip, frontend avec DS, frontend sans Figma, erreur accès)
- **Templates enrichis** :
  - `ONBOARDING.md` : 3 nouvelles sections — "Contexte métier" (domaine, utilisateurs, concepts, glossaire), "Design et maquettes" (fichiers Figma, design system, tokens), "Stratégie de test" (frameworks, couverture, philosophie, commandes)
  - `CONVENTIONS.md` : nouvelle section "Design tokens" (source, tokens couleurs/typo/spacing, synchronisation Figma ↔ code)
- **Handoff format enrichi** (`planning/onboarder-handoff-format`) : 3 nouveaux champs dans le bloc `## Retour vers orchestrator` — "Contexte métier" (domaine, utilisateurs, concepts, glossaire, pattern archi), "Design et maquettes" (fichiers, design system, tokens), "Stratégie de test" (frameworks, couverture, ratio, philosophie)
- **Questions Phase 2 enrichies** (workflow onboarder) : questions contextualisées sur le métier (si flou), sur la stratégie de test (si ambiguë), sur Figma (si maquettes trouvées mais statut unclear)
- **MCP Figma** : `mcpServers: [figma]` ajouté à l'agent `onboarder` ; documentation README mise à jour avec nouveau tool ; architecture actualisée
- **Parallélisme conditionnel** (`orchestrator-dev-protocol`, `orchestrator-workflow-modes`) — mode de traitement parallèle des tickets disponible exclusivement en mode `auto` lorsque 4 critères sont vérifiés simultanément : (1) pas de dépendance formelle entre tickets du lot, (2) agents distincts avec domaines disjoints (pas de `developer-fullstack`), (3) pas de fichiers transverses prévisibles, (4) maximum 3 tickets simultanés. Comportement : lancement simultané des sessions `developer-*`, CP-2 traités en séquentiel dans l'ordre d'arrivée, récap global produit uniquement quand toutes les sessions sont finales. Les modes `manuel` et `semi-auto` restent séquentiels — le bénéfice du parallélisme est réel uniquement en mode `auto` sur la phase d'implémentation.
  - Section "Évaluation du parallélisme conditionnel" ajoutée dans `orchestrator-dev-protocol` (CP-0)
  - Section "Workflow parallèle" ajoutée dans `orchestrator-dev-protocol` (entre Étape 6 et Récap global)
  - Note de parallélisme et description enrichie de l'option `Auto` dans `orchestrator-workflow-modes`
- **`docs/architecture/task-delegation.fr.md`** — nouveau document de référence sur le mécanisme de délégation inter-agents via l'outil `task` : mécanique de base (paramètres, sessions isolées, permissions par whitelist), hiérarchie des 4 niveaux d'agents, protocoles de communication (handoff contracts), reprise de session via `task_id`, marqueur de contexte d'invocation, checkpoints et compteurs anti-boucle, points d'attention et limites connues
- **Skill `auditor/living-docs-enrichment`** — nouveau skill partagé entre `auditor` (coordinateur), `planner` et `debugger` permettant d'enrichir de manière incrémentale les fichiers `ONBOARDING.md` et `CONVENTIONS.md` du projet cible :
  - **Flux en 5 étapes** : identification des découvertes → résumé affiché en texte clair → confirmation via `question` → délégation au `documentarian` via `task` → confirmation de la délégation
  - **Aucune écriture directe** : le `documentarian` est le seul agent autorisé à écrire dans ces fichiers
  - **Sources de découvertes** : auditor — sections `### Découvertes à documenter` des rapports des 7 sous-agents ; planner — patterns détectés en Phase 1 (conventions de nommage, bibliothèques non documentées) ; debugger — zones d'ombre levées par le diagnostic et patterns d'erreur récurrents
  - **Tableau de correspondance** origine × sections prioritaires pour ONBOARDING.md et CONVENTIONS.md (11 origines : audit sécurité/performance/accessibilité/éco-conception/architecture/privacy/observabilité, diagnostic bug, planification feature)
  - **Règles de qualité** : enrichissements factuels, concis, contextualisés, non redondants, actionnables
- **Section `### Découvertes à documenter`** ajoutée dans le format de rapport des 7 agents `auditor-*` — remontée des découvertes à capitaliser vers le coordinateur, lecture seule stricte conservée (aucun appel `task`)
- **Permissions `task.documentarian = allow`** ajoutées dans `opencode.json` pour `auditor`, `planner`, `debugger` et `orchestrator`
- **Workflows unifiés pour les agents coordinateurs** — 4 agents refactorisés avec workflows natifs en 5-7 phases (récaps systématiques, questions obligatoires via `question`, itérations contrôlées, phases de détection des cas particuliers, format handoff) :
  - **`planner`** : workflow unifié `planner-workflow.md` (7 phases : 0 prérequis → 1 exploration → 1.5 délégation design → 2 questions → 3 plan hiérarchique → 4 cas particuliers → 5 création Beads → 5.5 ai-delegated → 6 vérification)
  - **`onboarder`** : workflow unifié `onboarder-workflow.md` (6 phases : 0 prérequis → 1 exploration adaptative 7 profils → 2 questions → 3 rapport contexte → 4 cas particuliers → 5 production ONBOARDING.md + CONVENTIONS.md) — fusionne `project-discovery.md` et `project-conventions.md`
  - **`debugger`** : workflow unifié `debugger-workflow.md` (6 phases : 0 vérification artefacts → 1 exploration → 2 questions optionnel → 3 diagnostic 4 étapes → 4 cas particuliers → 5 rapport + ticket) — intègre la méthodologie `debug-protocol.md`
  - **`auditor`** : workflow unifié `auditor-workflow.md` (5 phases : 0 vérification prérequis → 1 chargement contexte → 2 sélection domaines avec compatibilité stack → 3 délégation sous-agents → 4 consolidation synthèse exécutive) — les 7 sous-agents `auditor-*` conservent leur workflow technique
- **Règle absolue inter-agents** : récap en texte clair dans la discussion AVANT tout appel à l'outil `question` — garantit la visibilité du contexte pour l'utilisateur et l'orchestrateur
- **Itérations contrôlées** : compteur max 3 par phase dans tous les nouveaux workflows — évite les boucles infinies, propose le passage forcé à la suite à la 3ème itération
- **Contexte d'invocation explicite** : détection du marqueur `[CONTEXTE] Invoqué depuis l'orchestrateur` dans tous les workflows — produit le bloc `## Retour vers orchestrator` en fin de workflow si détecté
- **Gouvernance des workflows** documentée (voir `CHANGELOG` ou `skills/` correspondants) : quand créer un workflow unifié (agents coordinateurs, phases itératives, validations utilisateur) vs workflow technique simple (agents spécialisés, exécution linéaire)
- **Support des providers OAuth** — github-copilot et ollama peuvent être configurés sans clé API (authentification OAuth native pour github-copilot, pas de clé requise pour ollama)
- **Fix (adapter)** — correction du bug jq `false // true` dans la lecture de `requires_api_key` depuis `providers.json` — l'opérateur `//` traitait `false` comme `null`
- **`oc debug [PROJECT_ID]`** — lance l'agent debugger sur un projet pour diagnostiquer un bug (nouveau script `scripts/cmd-debug.sh`, intégration dans `oc.sh`, aide et i18n mis à jour)
- **`oc project rename <OLD_ID> <NEW_ID>`** — renomme un projet dans tous les fichiers registre (`projects.md`, `paths.local.md`, `api-keys.local.md`) de façon atomique
- **`oc project move <PROJECT_ID> <path>`** — change le chemin local d'un projet dans `paths.local.md`
- **`oc skills validate [name]`** — valide la cohérence des skills (frontmatter `name`/`description`, correspondance nom/fichier, sources externes)
- **`oc agent deploy <agent-id> [PROJECT_ID]`** — déploie un seul agent sans tout redéployer ; respecte les cibles du projet si fourni
- **`oc status --short`** (`-s`) — vue compacte tableau id/chemin/statut (remplace `oc list`)
- **Skill `documentarian/doc-slides`** :
  - 4 templates prêts à l'emploi : `tech-demo`, `product-pitch`, `retrospective`, `onboarding`
  - Directives Marp complètes : frontmatter (`marp: true`, `theme`, `paginate`, `size`), directives locales (`_class`, `_backgroundColor`, `_paginate: false`), séparateurs `---`
  - Bonnes pratiques intégrées : 1 idée par slide, max 5 bullets, titres actionnables, taille recommandée par type de présentation
  - Exploration obligatoire avant génération : slides existants, thème custom, `.marprc`
  - Détection automatique de Marp CLI post-génération (`which marp` / `npx @marp-team/marp-cli`) — proposition de compilation HTML/PDF via `question()` si disponible
  - Fallback si Marp absent : instructions claires (npx, installation globale, extension VS Code, web.marp.app)
  - Nommage normalisé (`kebab-case` + date ISO courte) et emplacement adaptatif (`docs/presentations/`, `docs/slides/`, ou racine)
- **9 skills de contrat de communication formalisés** (voir [ADR-009](docs/architecture/adr/009-inter-agent-handoff-contracts.fr.md)) :
  - `auditor/audit-handoff-format` : bloc structuré `## Retour vers orchestrator` pour les 7 agents `auditor-*` — périmètre audité, tableau des vulnérabilités par sévérité, recommandations priorisées avec effort estimé, risque résiduel, statut (`corrections-requises` / `acceptable` / `bloquant`)
  - `design/design-handoff-format` : bloc structuré pour `ux-designer` et `ui-designer` — spec produite intégrale, contraintes d'implémentation, points ouverts, alternatives écartées, statut (`spec-complète` / `spec-partielle` / `bloqué`)
  - `developer/developer-handoff-format` : bloc structuré pour les 9 `developer-*` → `orchestrator-dev` — fichiers modifiés, tests écrits, critères d'acceptance cochés, points d'attention pour la review, blocages rencontrés, statut (`implémenté` / `partiellement-implémenté` / `bloqué`)
  - `planning/onboarder-handoff-format` : bloc structuré pour `onboarder` → `orchestrator` (Mode C) — stack technique détaillée, conventions identifiées, dette technique, zones d'incertitude, fichiers de contexte produits, statut (`contexte-établi` / `contexte-partiel` / `bloqué`)
  - `planning/planner-handoff-format` : bloc structuré pour `planner` → `orchestrator` — tableau complet des tickets créés avec agent prévu et dépendances, hypothèses et ambiguïtés, estimation, risques, statut (`planification-complète` / `planification-partielle` / `bloqué`)
  - `qa/qa-handoff-format` : bloc structuré pour `qa-engineer` → `orchestrator-dev` — tests écrits avec fichiers et cas couverts, critères d'acceptance cochés, zones non testables, statut (`couverture-complète` / `couverture-partielle` / `non-testable`)
  - `quality/debugger-handoff-format` : bloc structuré pour `debugger` → `orchestrator` (Mode D) — cause racine avec niveau de certitude + chaîne causale, hypothèses explorées, impact et régressions, tickets créés, actions d'urgence si bug en prod, statut (`diagnostiqué` / `partiellement-diagnostiqué` / `non-reproductible`)
  - `reviewer/reviewer-handoff-format` : bloc structuré pour `reviewer` → `orchestrator-dev` — verdict actionnable (`commit` / `corriger` / `corriger-sécurité`), synthèse des problèmes par sévérité, corrections requises verbatim, routing recommandé (`retour-initial` / `developer-security`), statut (`approuvé` / `corrections-requises` / `bloquant-sécurité`)
- Nouveau dossier `skills/design/` pour les skills des agents design
- Nouveau dossier `skills/quality/` pour les skills des agents qualité (hors `qa/` et `reviewer/`)


- **Skills `analysis/*` supprimés** : `skills/analysis/analysis-workflow.md` (545 L), `skills/analysis/analysis-templates.md` (510 L), `skills/analysis/analysis-questions.md` (276 L) — répertoire `skills/analysis/` supprimé. Remplacés par les 4 workflows unifiés natifs.
- **Skills archivés** (renommés `*-legacy.md`) : `planning/planner.md` → `planner-legacy.md`, `planning/project-discovery.md` → `project-discovery-legacy.md`, `planning/project-conventions.md` → `project-conventions-legacy.md`, `debugger/debug-protocol.md` → `debug-protocol-legacy.md`, `auditor/audit-protocol.md` → `audit-protocol-legacy.md`

### Documentation


- **Parité documentation EN/FR** — audit et correction de 13 divergences dans les fichiers `.en.md` :
  - `docs/architecture/agents.en.md` : skills `developer-data/mobile/platform` mis à jour (injection dynamique), skills `auditor`/`debugger`/`planner` complétés, Phase 5 debugger (living docs) et Phase 6 planner (living docs) ajoutées
  - `docs/architecture/skills.en.md` : skill `living-docs-enrichment` ajouté au domaine `auditor/`, matrice de dépendances `auditor`/`planner`/`debugger` complétée
  - `docs/architecture/overview.en.md` : diagramme orchestrateur corrigé (coordinateur `auditor` visible), principe 5 complété (mention `documentarian`, délégation `living-docs-enrichment`)
  - `docs/guides/authoring.en.md` : `audit-protocol` → `audit-protocol-light`
  - `docs/guides/contributing.en.md` : dossier `skills/posture/` ajouté au tableau
  - `docs/reference/cli.en.md` : doublons `opencode` supprimés (`oc deploy`, `oc agent create`)
  - `docs/reference/config.en.md` : sections `oc provider init/set-key/set-model` et `ocp` (interactive provider switcher) ajoutées
  - `docs/guides/workflows.en.md` : sections "Living docs enrichment" ajoutées dans les scénarios audit (Phase 4) et debug (Phase 5)
- `docs/architecture/task-delegation.fr.md` : section `### Pas de parallélisme` remplacée par `### Parallélisme conditionnel (mode auto uniquement)` — explication du choix séquentiel par défaut, tableau des 4 critères, comportement en mode parallèle, limites (CP-2 reste séquentiel, bénéfice uniquement en mode auto)
- `docs/architecture/task-delegation.fr.md` : section `### Zone d'ombre` renommée `### Le task_id est un ID de session OpenCode` — nature réelle documentée (session persistante, navigation TUI, SDK), tableau des points encore inconnus, référence au garde-fou
- `docs/architecture/task-delegation.fr.md` : section `### task_id — mécanisme opaque` renommée `### task_id — risque de session introuvable` — tableau causes/probabilité/impact, description du garde-fou
- `docs/architecture/task-delegation.fr.md` : section `### Récap partiel vs final` enrichie — tableau comparatif, arbre de détection, diagramme d'état Mermaid, tableau des erreurs possibles
- `docs/architecture/task-delegation.fr.md` : section `### Transmission du mode via prompt` enrichie — valeurs canoniques, 4 cas de défaillance avec probabilité et impact, résumé des correctifs appliqués
- `docs/architecture/skills.fr.md` : ajout de `auditor/living-docs-enrichment` dans le domaine `auditor/`, mise à jour de la matrice de dépendances (`auditor`, `planner`, `debugger`)
- `docs/architecture/agents.fr.md` : skills et descriptions mis à jour pour `auditor`, `planner`, `debugger` ; règles communes nuancées — distinction lecture seule stricte (`auditor-*`, `reviewer`) vs délégation documentaire autorisée (`auditor`, `planner`, `debugger`)
- `docs/architecture/overview.fr.md` : principe 5 ("Lecture seule pour les agents non-développeurs") mis à jour — précise que l'écriture documentaire passe toujours par le `documentarian` via délégation explicite
- `docs/guides/workflows.fr.md` : ajout d'une étape "Enrichissement des documents vivants" dans le scénario audit (Phase 4) et dans le scénario debug (Phase 5) avec exemples de blocs de proposition
- `docs/architecture/skills.fr.md` et `skills.en.md` : domaines `planning/`, `debugger/`, `auditor/`, `quality/` mis à jour — nouveaux workflows unifiés, skills archivés, matrice de dépendances agents ↔ skills mise à jour
- `docs/architecture/agents.fr.md` et `agents.en.md` : skills injectés mis à jour pour `planner`, `onboarder`, `debugger`, `auditor`
- `docs/reference/cli.fr.md` et `cli.en.md` : mise à jour complète — `oc list` → `oc status --short`, nouvelles sections `oc project`, `oc provider` (hub-level uniquement), `oc agent deploy`, `oc skills validate`, clarification `update`/`upgrade`
- `docs/architecture/skills.fr.md` et `skills.en.md` : ajout de `documentarian/doc-slides` dans le domaine `documentarian/`, mise à jour de la matrice de dépendances
- `docs/architecture/agents.fr.md` et `agents.en.md` : mise à jour des skills et de la description de l'agent `documentarian`
- `docs/architecture/skills.en.md` et `skills.fr.md` : ajout des 8 nouveaux skills de handoff dans leurs domaines respectifs, mise à jour de la matrice de dépendances agents ↔ skills
- `docs/architecture/agents.en.md` et `agents.fr.md` : mise à jour des skills injectés pour tous les agents concernés
- `docs/guides/workflows.en.md` et `workflows.fr.md` : ajout de notes sur les retours structurés dans les scénarios 1 et 3
- `docs/guides/contributing.en.md` et `contributing.fr.md` : ajout des nouveaux dossiers `skills/design/` et `skills/quality/`, règle sur les skills de handoff
- `docs/architecture/adr/009-inter-agent-handoff-contracts` (EN + FR) : décision architecturale de formaliser les contrats de communication inter-agents comme skills dédiés

---

## [1.5.0] — 2026-04-30

### Added

- **Skills spécifiques aux stacks** (`skills/developer/stacks/`) — 38 nouveaux skills atomiques organisés par catégorie :
  - **Langages** : `dev-standards-typescript`, `dev-standards-python`
  - **Frontend** : `dev-standards-react`, `dev-standards-nextjs`, `dev-standards-nuxtjs`, `dev-standards-angular`
  - **Backend** : `dev-standards-nestjs`, `dev-standards-express`, `dev-standards-django`, `dev-standards-fastapi`, `dev-standards-laravel`, `dev-standards-rails`, `dev-standards-springboot`
  - **ORMs / BDD** : `dev-standards-prisma`, `dev-standards-typeorm`, `dev-standards-sqlalchemy`, `dev-standards-mongodb`
  - **Spec API** : `dev-standards-openapi`
  - **Test** : `dev-standards-vitest`, `dev-standards-jest`, `dev-standards-playwright`, `dev-standards-cypress`
  - **Mobile** : `dev-standards-react-native`, `dev-standards-flutter`, `dev-standards-swift`, `dev-standards-kotlin`
  - **Data / ML** : `dev-standards-pandas`, `dev-standards-dbt`, `dev-standards-airflow`, `dev-standards-pyspark`
  - **DevOps / CI-CD** : `dev-standards-docker`, `dev-standards-github-actions`, `dev-standards-gitlab-ci`
  - **Platform / Infra** : `dev-standards-terraform`, `dev-standards-kubernetes`, `dev-standards-helm`, `dev-standards-argocd`

- **`config/stack-skills.json`** — table de mapping déclarative : stack détectée → skills à injecter, avec filtrage par type d'agent via `_agent_scope`

- **Détection de stack automatique à `oc deploy`** (`scripts/lib/prompt-builder.sh`) :
  - `detect_stack(project_path)` : détecte la stack depuis `package.json`, `pyproject.toml`, `requirements.txt`, `Gemfile`, `build.gradle`, `pom.xml`, `pubspec.yaml`, `Dockerfile`, `.github/workflows/`, `*.tf`, `Chart.yaml`, manifests K8s/ArgoCD
  - `resolve_stack_skills(agent_id, stacks, config)` : résout les skills à injecter par croisement stacks × `stack-skills.json` × scope d'agent — déduplique avec les skills déclarés en frontmatter
  - `build_agent_content()` : nouveau paramètre `$4 project_path` — si fourni, les stack skills sont injectés après les skills statiques

### Changed

- **Skills génériques purifiés** — suppression de toutes les références spécifiques à des outils ou frameworks dans les skills qui se veulent agnostiques :
  - `dev-standards-universal` : section TypeScript entière supprimée (extraite dans `stacks/dev-standards-typescript`)
  - `dev-standards-testing` : suppression des mentions Vitest, Jest, Playwright, Cypress, Vue, React, axios, SQLite, testcontainers — remplacées par des formulations agnostiques
  - `dev-standards-api` : section "Contrat OpenAPI" → "Contrat d'API (schema-first)", YAML OpenAPI et exemples TypeScript extraits dans `stacks/dev-standards-openapi`
  - `dev-standards-security` : `process.env.API_KEY` → `env("API_KEY")` agnostique, `npm/composer/pip audit` → "gestionnaire de paquets du projet"
  - `dev-standards-devops` : sections Docker, GitHub Actions, GitLab CI extraites dans leurs skills dédiés — garde scripts shell, secrets, registries, observabilité, IaC génériques

- **Skills multi-outils éclatés** en fichiers atomiques :
  - `dev-standards-mobile` → 4 skills (`react-native`, `flutter`, `swift`, `kotlin`) — fichier supprimé
  - `dev-standards-data` → 5 skills (`python`, `pandas`, `dbt`, `airflow`, `pyspark`) — fichier supprimé
  - `dev-standards-platform` → 4 skills (`terraform`, `kubernetes`, `helm`, `argocd`) — fichier supprimé
  - `dev-standards-vuejs` déplacé dans `stacks/`

- **Planner** (`skills/planning/planner.md`) :
  - Règle de granularité des tickets assouplie : un ticket unique est acceptable par défaut ; le découpage n'est suggéré que si **plusieurs** critères sont réunis simultanément (pas un seul)
  - PHASE 0.2 : nouvelle section "Recherche de logique existante" — le planner doit chercher dans **toutes les couches** (backend, frontend, partagé) si une logique similaire existe avant de planifier
  - PHASE 0.3 : section `### Logiques existantes réutilisables` ajoutée au template de résumé de contexte
  - Rappel final n°17 : signaler tout risque de duplication inter-couches dans le résumé de contexte

- **Agents developer** : `skills:` mis à jour pour pointer vers les nouveaux chemins `stacks/` (`developer-mobile`, `developer-data`, `developer-devops`, `developer-platform`, `developer-frontend`, `developer-fullstack`)

- **`oc deploy` / `--diff`** : passage de `deploy_dir` comme `project_path` à l'adapter OpenCode — déclenche la détection de stack automatique pour tout déploiement sur un projet enregistré

### Documentation

- `docs/architecture/skills.en.md` et `skills.fr.md` : refonte complète du domaine `developer/` — distinction skills génériques vs skills spécifiques aux stacks, tables par catégorie (langages, frontend, backend, ORMs, test, mobile, data, infra, platform), matrice de dépendances mise à jour avec mentions des catégories dynamiques
- `docs/guides/authoring.en.md` et `authoring.fr.md` : section "Skills à injecter selon le type d'agent" enrichie avec le mécanisme d'injection dynamique, scope par agent, et instructions pour ajouter une nouvelle stack
- `docs/reference/cli.en.md` et `cli.fr.md` : `oc deploy` documenté avec la détection de stack automatique et son comportement par `PROJECT_ID`

---

## [1.4.0] — 2026-04-22

### Added

- Skill `developer/dev-standards-simplicity` : KISS (solution la plus directe), YAGNI
  (n'implémenter que ce qui est dans le ticket actif), pas d'abstraction prématurée
  (3 cas concrets avant d'abstraire), limites mesurables (fonction ≤ 20 lignes,
  complexité cyclomatique ≤ 10, params ≤ 4, imbrication ≤ 3 niveaux)

### Changed

- Agent `orchestrator` : permissions techniques `bash: deny`, `edit: deny`, `write: deny`
  ajoutées dans le frontmatter — l'agent agit uniquement via `task` et `question` ;
  `task` restreint à une allowlist exhaustive (`planner`, `onboarder`, `ux-designer`,
  `ui-designer`, `auditor-*`, `orchestrator-dev`, `debugger`)
- Skill `orchestrator/orchestrator-protocol` :
  - Mode C conditionné à l'absence des fichiers `ONBOARDING.md` et `CONVENTIONS.md` sur
    disque — si l'un des deux est présent, le contexte est chargé directement sans
    proposer l'onboarder
  - Questions des sous-agents contextualisées : règle ajoutée pour qu'un sous-agent
    invoqué depuis un parent inclue toujours un bloc `[Agent — Phase | Feature]` en
    tête de son champ `question`
  - CP-0 : séparation explicite entre l'affichage du tableau des tickets (dans la
    discussion) et la demande de mode de workflow (outil `question` court, sans tableau)
  - Gestion des agents non déployés : nouvelle section avec table de substitution par
    domaine (`auditor-security → developer-security`, `auditor-accessibility →
    developer-frontend`, `auditor-architecture/performance → developer-fullstack`,
    `auditor-privacy/ecodesign/observability/ux-designer/ui-designer → aucun substitut`),
    question structurée avec option de déploiement via `!oc deploy opencode <PROJECT_ID>`
    sans quitter OpenCode
  - Annonces de délégation enrichies : chaque invocation de sous-agent (planner,
    ux-designer, ui-designer, auditor-*, orchestrator-dev) annonce explicitement que
    les questions remonteront avec leur contexte
  - Mode D — router les bugs vers `debugger` sans tentative de correction autonome
- Skill `posture/tool-question` : nouvelle section "Questions posées en tant que
  sous-agent" — format obligatoire `[Nom — Phase | Feature]` en tête du champ `question`
  quand l'agent est invoqué par un parent
- Skill `orchestrator/orchestrator-workflow-modes` : extrait en source de vérité
  autonome (précédemment intégré dans `orchestrator-dev-protocol`)
- Skill `orchestrator/orchestrator-handoff-format` : extrait en source de vérité
  autonome pour le format de retour `orchestrator-dev → orchestrator`
- `agents/planning/orchestrator.md` : skills mis à jour (`orchestrator-workflow-modes`,
  `orchestrator-handoff-format` ajoutés)
- `docs/architecture/agents.fr.md` / `agents.en.md` : section `orchestrator` enrichie
  (4 modes d'entrée D/C/A/B, permissions techniques, Mode C conditionnel, gestion des
  agents manquants)
- `docs/guides/workflows.fr.md` / `workflows.en.md` : CP-0 clarifié (tableau dans la
  discussion, question courte), notes sur les questions contextualisées des sous-agents
  et sur le comportement face aux agents manquants
- `tests/test_prompt_builder.bats` : 8 nouveaux tests d'intégrité couvrant les
  permissions du frontmatter, la table de substitution, le déploiement sans quitter
  OpenCode, la condition Mode C et la règle de contexte de `tool-question`

### Fixed

- `scripts/lib/prompt-builder.sh` : suppression de la variable `task_json` inutilisée
  (avertissement ShellCheck SC2034)
- Agent `orchestrator-dev` : délégation et outil `question` corrigés — alignement
  avec le protocole `orchestrator-dev-protocol`
- `orchestrator/orchestrator-protocol` et `orchestrator-dev-protocol` : alignement
  des deux protocoles (checkpoints, format handoff, modes de workflow)

---

## [1.3.0] — 2026-04-20

### Added

- Commande `oc review [PROJECT_ID] [--branch <branche>] [--agent <agent>]` : lance
  une review IA sur un projet en invoquant l'agent `reviewer` avec le diff injecté ;
  détecte automatiquement la branche courante si `--branch` absent ; vérifie la
  présence du reviewer dans `projects.md` ; injecte `CONVENTIONS.md` si présent
- `scripts/cmd-review.sh` : implémentation complète de la commande
- `scripts/lib/prompt-builder.sh` : `build_review_bootstrap_prompt` injecte le diff
  `git diff <branche>` et l'hint `CONVENTIONS.md` conditionnel
- `oc.sh` : case `review)` ajouté dans le dispatcher
- `docs/reference/cli.md` : section `oc review` ajoutée
- Skill `orchestrator/orchestrator-workflow-modes` : source de vérité unique pour
  les 3 modes (manuel/semi-auto/auto) — injecté dans `orchestrator` et
  `orchestrator-dev` pour garantir la cohérence
- Skill `orchestrator/orchestrator-handoff-format` : source de vérité unique pour
  le format de retour `orchestrator-dev → orchestrator`

### Changed

- Agent `orchestrator` : `onboarder` ajouté dans la table des agents disponibles,
  Mode C (projet inconnu) ajouté dans le workflow avec checkpoint `[CP-onboard]`
  optionnel et sautables — exemple d'invocation Mode C ajouté
- Skill `orchestrator/orchestrator-protocol` : Mode C documenté avec condition de
  déclenchement, proposition à l'utilisateur, format du `[CP-onboard]` et règle
  "toujours optionnel et sautables"
- Agent `planner` : invocation autonome optionnelle des agents `ux-designer` et
  `ui-designer` ajoutée (PHASE 1.5) — 3 options : invoquer directement (Option A),
  laisser l'utilisateur invoquer (Option B), continuer sans (Option C)
- Agent `orchestrator-dev` : création de branche dédiée par ticket avant implémentation —
  pause obligatoire à l'étape 1b dans tous les modes
- Agents (tous) : outil `question` OpenCode activé sur tous les agents — remplacement
  des pauses textuelles par des appels structurés à l'outil `question`
- `docs(beads)` : état review et cycle de feedback clarifiés
- `docs/architecture/agents.md` : total mis à jour, `onboarder` ajouté dans la
  famille Coordinateurs, nouvelle règle "Agents de découverte"
- `docs/architecture/skills.md` : `planning/project-discovery` ajouté, matrice
  de dépendances mise à jour pour `onboarder`
- `scripts/cmd-help.sh` : refonte avec `.cmd`/`.desc` séparés dans `i18n`,
  section `beads ui` et `tracker set-sync-mode` ajoutées

### Fixed

- `scripts/lib/prompt-builder.sh` : sauts de ligne dans les templates `bd update`
  pour le planner corrigés
- `scripts/cmd-help.sh` : commandes `agent select` et `mode` manquantes ajoutées
- Agent `planner` : sauts de ligne dans les templates `bd update` corrigés
- `fix(onboarding)` : ne pas proposer l'onboarding si `ONBOARDING.md` existe déjà
- `fix(release)` : bumper `hub.json.example` (tracké) au lieu de `hub.json` (ignoré)
- Agents `orchestrator`/`orchestrator-dev` : synchronisation de la permission
  `question` et du skill `tool-question`
- CI : avertissements ShellCheck corrigés dans `cmd-board` et `common`

---

## [1.2.0] — 2026-04-15

### Added

- Support natif AWS Bedrock (`amazon-bedrock`) : détection automatique du provider
  dans `opencode.adapter.sh`, sync `opencode.json` avec region et token
  `AWS_BEARER_TOKEN_BEDROCK` ; différencié du mode litellm
- Support région AWS pour le provider `amazon-bedrock` dans `providers.json`
- `feat(beads)` : ajout de `.beads/` au `.git/info/exclude` à l'init
- `feat(i18n)` : clés `beads.gitignore_added` et `beads.gitignore_exists` ajoutées
- `feat(beads-ui)` : intégration de `bdui` dans `oc install`, `oc update` et la
  documentation
- Import automatique des labels tracker (GitLab / Jira) à l'init Beads

### Changed

- `feat(deploy)` : utilisation de `.git/info/exclude` au lieu de `.gitignore` dans
  les projets cibles — évite de polluer le `.gitignore` versionné des projets
- `chore(config)` : `hub.json` et `opencode.json` retirés du tracking git, ajoutés
  à `.gitignore`
- `docs` : section prérequis retirée du README (EN + FR)

### Fixed

- `fix(beads)` : remplacement de `bd label add` par `bd label create` dans
  `cmd-init.sh` — alignement avec l'API Beads actuelle
- `fix(tests)` : stabilisation des tests BATS pour CI sans `hub.json`
- `test` : assertions BATS corrigées (`bd label add` → `bd label create`)

---

## [1.1.0] — 2026-04-13

### Added

- `feat(beads)` : champ `Sync mode` dans `projects.md` et commande
  `oc beads tracker set-sync-mode` pour configurer le mode de synchronisation
  du tracker
- Commande `oc init` : proposition d'ajout de `opencode.json` et `.opencode/` au
  `.gitignore` du projet à l'étape 5

### Fixed

- `fix(init)` : suppression des déclarations `local` invalides hors scope de fonction
- `fix(help)` : commandes `agent select` et `mode` manquantes ajoutées dans l'aide

---

## [1.0.0] — 2026-03-29

### Added

- Commande `oc upgrade` : met à jour les sources du hub via `git pull` (main) ou
  `git checkout <tag>` (`oc upgrade v1.1.0`). Propose `oc sync` après mise à jour réussie.
  Support du one-liner `VERSION=vX.Y.Z` dans `install.sh` pour installer une version épinglée.
- Agent `documentarian` (famille Documentation) avec 5 skills spécialisés :
  `doc-protocol`, `doc-standards`, `doc-adr`, `doc-api`, `doc-changelog`
- Skill `planning/planner.md` : Phase 0 (exploration adaptative de la codebase
  et des tickets existants, résumé de contexte), Phase 1 (questions contextualisées,
  priorités déduites et justifiées), Phase 2 (plan hiérarchique epics → tickets,
  règle >5 tickets pour création epics dans Beads), Phase 3 (`--parent`, `--deps`,
  `--estimate`), Phase 4 (`bd children`), section gestion des aléas
- `CHANGELOG.md` et `CONTRIBUTING.md` à la racine du dépôt

### Changed

- Restructuration de `agents/` en sous-dossiers par famille :
  `auditor/`, `developer/`, `documentation/`, `planning/`, `quality/`
- Migration `skills/planner.md` → `skills/planning/planner.md` — cohérence
  avec la convention de sous-dossiers par domaine
- Agent `planner` : frontmatter enrichi (skill `developer/dev-beads` ajouté),
  corps restructuré avec ce que l'agent lit, ce qu'il produit, tableau des aléas
- CI `validate-agents` : glob `agents/*.md` → `find agents/ -name "*.md"`
  pour couvrir la structure en sous-dossiers (le job était en faux positif permanent)

### Fixed

- `scripts/cmd-agent.sh` : `_find_agent_file` réécrit avec process substitution
  `< <(find ...)` — le `return 0` dans un pipe ne sortait pas de la fonction
- `scripts/cmd-skills.sh` : message d'aide corrigé (`agents/*.md` →
  `agents/<famille>/<id>.md`)
- `docs/guides/contributing.md` : chemins `agents/auditor.md`,
  `agents/developer-frontend.md` et `scripts/adapter-manager.sh` obsolètes corrigés
- `docs/architecture/skills.md` : matrice ASCII `developer-fullstack` complétée
  avec `dev-standards-frontend-a11y` et `dev-standards-vuejs`

---

## [0.1.0] — 2026-03-26

### Added

- Hub central multi-cible : OpenCode
- CLI `oc.sh` avec 13 commandes : `init`, `deploy`, `start`, `list`, `remove`,
  `agent`, `skills`, `beads`, `sync`, `update`, `install`, `version`, `help`
- 19 agents initiaux organisés en 5 familles :
  - Coordinateurs : `orchestrator`, `auditor`
  - Développeurs : `developer-frontend`, `developer-backend`, `developer-fullstack`,
    `developer-data`, `developer-devops`, `developer-mobile`, `developer-api`
  - Qualité : `reviewer`, `qa-engineer`, `debugger`
  - Audit : `auditor-security`, `auditor-performance`, `auditor-accessibility`,
    `auditor-ecodesign`, `auditor-architecture`, `auditor-privacy`
  - Planification : `planner`
- 27 skills organisés par domaine (`developer/`, `auditor/`, `orchestrator/`,
  `qa/`, `debugger/`, `reviewer/`)
- 1 adapter : `opencode.adapter.sh`
- Intégration Beads (`bd`) pour la gestion des tickets : `cmd-beads.sh`,
  workflow `bd claim → implémenter → bd close` dans tous les agents developers
- Commande `oc agent` : création interactive, édition, liste, info
- Commande `oc skills` : liste, ajout de sources externes, `used-by`
- Sélecteur de skills interactif avec navigation clavier (flèches + espace)
- Staleness detection : `oc deploy --check` pour détecter les agents obsolètes
- CI GitHub Actions : ShellCheck, validation frontmatter agents, staleness check
- Documentation complète : 5 ADR, guides (getting-started, workflows, contributing),
  référence CLI et config, architecture overview avec diagrammes Mermaid
- Support multi-projets via `projects.md` et `oc init` / `oc start`
- Config `hub.json` : targets actives, modèle IA, skills globaux VS Code
