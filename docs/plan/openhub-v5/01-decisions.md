# 01 — Décisions validées

Toutes les décisions ci-dessous ont été validées par l'utilisateur. Toute remise en cause doit être explicite.

## D — Décisions structurantes

| # | Décision |
|---|---|
| D1 | **Trois couches** : briques (agents/skills) · workflows (enchaînement) · exécution (local/conteneur/distant). |
| D2 | **La logique reste dans `oh`** ; un **adaptateur** par outil (prolonge `platform.SessionPlatform`, ADR-036). |
| D3 | **opencode V2 = cible** ; ~~V1 reste compatible~~ → **révisé le 06/10/2026 : V1 n'est plus pris en charge en v5** (sans opencode V2, oh refuse de lancer avec un message clair, Doctor et guide de migration). Le modèle neutre `SessionSpec` et l'interface d'adaptateur restent (autres outils, BL-6). L'ancien lancement (`internal/opencode`, plateforme V1, repli des alias) est supprimé avec le lot 3.E. |
| D4 | **Paquet par session construit hors du projet** (`~/.oh/sessions/<id>/`, `~/.oh/bundles/<hash>/`). Plus aucun déploiement dans les projets. |
| D5 | **Workflows en YAML stockés dans `team-state`**. Un projet peut créer ses propres workflows. Projet sans équipe → **team-state solo** (dépôt git local sans remote, promouvable). |
| D6 | **Orchestration hybride** : agent `conductor` générique par défaut, agent d'entrée dédié possible (`entry.agent`). |
| D7 | **Gouvernance dans le hub, sans MR** : brouillon → publié ; **tout membre peut publier** ; validation obligatoire ; `workflows.lock` (hash) ; historique + restauration ; événements team-state. |
| D8 | **Checkpoints sur 3 niveaux** : (1) prompt généré ; (2) outil MCP `workflow_checkpoint` en `ask` + validation via l'API permissions (S3) ; (3) plugin mince `permission.hook("evaluate")`. Machine à états dans `oh` (`CheckpointService`). |
| D9 | **Beads reste toujours sur la machine** : local = `bd` direct ; conteneur = faux `bd` → passerelle `oh` ; distant = instantané en entrée / journal en sortie, rejoué localement avec détection de conflits. |
| D10 | **Les secrets restent sur la machine** : trousseau ; passerelle MCP HTTP pour gitlab/figma/jira… ; **proxy d'identifiants LLM** (O4). |
| D11 | **Local ou distant au choix** à chaque lancement. Distant = **runners GitLab CI** (à mettre en place : objectif de la phase 5). |
| D12 | **Conteneurs OCI** via Colima ou Podman (Docker CLI si présent). **Une image par projet** (Dockerfile de dev du projet + couche fine), **paquet monté en lecture seule**. |
| D13 | **« Monde fermé » obligatoire et non configurable** : seuls agents/skills du paquet visibles ; agents natifs de l'outil désactivés d'office ; mis en œuvre et **vérifié** (`Attest`) par chaque adaptateur ; échec = la session ne démarre pas. `[deploy].disable_native_agents` supprimé. |
| D14 | **Suppression du déploiement par projet** : `oh deploy`, `oh sync`, UI de déploiement, étapes « Agents » des assistants, `Project.Agents`, vue `project.agents`. Remplacés par `oh bundle build/show`. Nettoyage via `oh migrate deploy-cleanup`. |
| D15 | **Couche de services commune** (CLI, TUI, futur `oh serve`). Zéro logique métier dans `cmd/tui_*.go`. |
| D16 | **Pages d'accueil** (hub, projet, équipe) : on garde la structure, on remplace seulement le bloc de lancement rapide par « Démarrer » et on ajoute « Sessions » (phase 3). |
| D17 | **`oh serve` (dashboard web oh) : plus tard** (BL-3), il exposera les services. |
| D18 | **Version majeure v5** (O15). |
| D19 | **Indépendance vis-à-vis de l'outil** (07/10/2026) : toute spécificité d'un outil (noms, identifiants de fournisseurs, variables d'environnement, API, chemins, formats, versions, binaire, noms affichés) vit **uniquement** dans `internal/adapters/<outil>`. Hors adaptateur : interfaces et types neutres (`adapters`, `sessionspec`, `Capabilities`) ; une seule **racine de composition** choisit l'implémentation (registre d'adaptateurs). Le nom de l'outil affiché vient de l'adaptateur. Garde-fou : **test d'architecture** qui échoue sur tout import d'un adaptateur ou identifiant/littéral propre à l'outil hors de la liste autorisée. Toute nouvelle fonction d'un outil passe par une **capacité** de l'adaptateur, avec repli quand elle est absente (ex. S8). |

## O — Points d'ombre tranchés

| # | Décision |
|---|---|
| O1 | `experimental.subagent_depth` **calculé** = profondeur max du graphe du workflow (+ test de contrat à chaque version d'opencode). La clé V1 `subagent_depth` est ignorée par V2. |
| O2 | V2 : **ne pas définir `system`** sur nos agents ; le **plugin** ajoute le corps de l'agent via `session.hook("context")` → le prompt de base natif d'opencode est conservé. Skill **« posture outillage »** minimale comme socle pour les autres adaptateurs. Étude complémentaire : `instructions` par session (S8). |
| CM | **Code Mode** : champ `code_mode: on\|off` par workflow, **off par défaut** ; appliqué par l'adaptateur via la permission `execute`. |
| O3 | Lancement : **`serve` → session créée par l'API (id choisi par oh, agent, permissions de session) → prompt initial → interface attachée `--server <url> -s <id>`**. `--standalone` = secours. |
| O4 | **Proxy d'identifiants LLM intégré à `oh` dès la phase 0** : jeton par session (révoqué en fin de session), liste blanche modèles/chemins, signature SigV4 côté machine (cas `AWS_PROFILE`), comptage coût/budget. Région du fournisseur dans `SessionSpec`. `XDG_DATA_HOME` par groupe de serveur. |
| O5 | Plugins déclarés **par workflow** (`plugins:`) ; `context-mode` devient optionnel. Télémétrie lue depuis le **flux d'événements SSE**. Plugin `rtk` existant (V1, global) à inventorier. |
| O6 | Plugin oh en **TypeScript embarqué dans le binaire** (`go:embed`), écrit dans le paquet sous forme de **dossier + `package.json`**, objet exporté `{ id, setup }` (sans dépendance npm) ; export dual V1 (`server()`) / V2 (`setup()`) ; V1 ≥ 1.18.29 pour le niveau 3. |
| O7 | **Sorties typées** + suggestion **« Enchaîner avec… »** en fin de session (fiche suivante préremplie ; multi-sélection possible → parallèle). |
| O8 | Correspondance des fonctions existantes : `--parallel` → multi-sélection dans le lanceur · `--sweep` → workflow `sweep` · `review --publish` → entrée `publish` + sortie `merge_request` · `review feedback` → workflow `review-feedback` · `--worktree` → option « Emplacement » / `--location` · enrichissement des briefs → workflow headless `brief-enrich` · `oh skill budget` → `oh bundle show --budget` · trackers Jira/Linear inchangés. |
| O9 | **Niveau workflow** dans la cascade des modèles : workflow·agent > workflow > projet·agent > projet·famille > projet > hub·agent > hub·famille > hub > frontmatter. |
| O10 | **Worktree automatique** si une autre session qui écrit est active sur le même chemin. |
| O11 | Entrées **délimitées** (balises de données) et **tronquées** ; rappel dans le `conductor`. **Révisé le 08/10/2026 (A27, piste T)** : seules les entrées **calculées depuis une source externe** (`from:` : discussions de MR, brief de ticket…) sont délimitées ; les entrées **saisies par l'utilisateur** au lancement sont sa demande et sont écrites telles quelles (toujours tronquées, balises neutralisées). Le contenu lu par les agents (tickets, fichiers, web) reste de la donnée. |
| O12 | **Résumé d'impact** à la publication couvrant aussi le catalogue d'équipe + badge « nouvelle brique ». |
| O13 | Multi-dépôts → **backlog BL-7**. |
| O14 | Publication : **pull + rebase → revalidation → version +1 → push**, retry ; hors ligne = file d'attente. |
| O15 | **v5** + alias temporaires des anciennes commandes + `migrate deploy-cleanup` + guide. |
| O16 | Conteneur/distant : **macOS + Linux**. Windows : local uniquement (avertissement Doctor). |
| O17 | Tests : **golden** paquets + **contrat** contre vrai serveur sans LLM + **e2e nocturne Haiku**. |
| O18 | Docs mises à jour **à chaque phase** ; ADR ensuite. |
| O19 | Mesure du gain : `oh bundle show --budget` vs déploiement complet actuel + **mesure réelle** du prompt système via plugin de sonde. |

## S — Usages du mode serve retenus

| # | Usage | Phase |
|---|---|---|
| S1 | Lancement unifié serve → API → `--server` | 0 |
| S2 | Suivi temps réel via SSE (`/api/event`) + resynchronisation par endpoints de liste | 0 (socle) / 3 |
| S3 | Checkpoints validés via l'API permissions (outil MCP `workflow_checkpoint` en `ask`) depuis la TUI oh sans attacher | 3 |
| S4 | Réponses aux questions des agents (formulaires) depuis oh | 3 |
| S5 | Répondeur automatique en CI selon la politique du workflow (au lieu de `--auto`) | 5 |
| S6 | Messages `synthetic` de oh dans la session | 3 |
| S7 | `session.environment` pour variables propres à la session (jeton passerelle Beads, etc.) | 3–4 |
| S8 | `instructions` par session (expérimental) — étude | 1 |
| S9 | `diff`/`vcs` pour récap et MR ; `export`/`import` (distant) ; `fork` (variantes) | 3 / 5 |
| S10 | `interrupt`, `switchModel`, `compact` | 3 |
| S11 | Ouvrir une session dans le navigateur / mobile (`pair`) | 3 |
| S12 | Vérification du monde fermé (`agent.list`, `skill.list`, `mcp.list`) | 0 |
| S13 | IDE via ACP | backlog |

## I — Interaction avec les sessions

| # | Décision |
|---|---|
| I1 | **oh = tour de contrôle, opencode = cabine.** Fermer la fenêtre opencode **ne coupe pas** la session ; seul « Arrêter » l'interrompt. |
| I2 | Ouverture des sessions, mode `auto` : **iTerm2 (onglet)** → **Terminal.app (nouvelle fenêtre)** → tmux (si `$TMUX`) → navigateur (`pair`) → **suspension en tout dernier recours**. Réglable : `auto\|iterm\|terminal\|tmux\|browser\|suspend` ; style iTerm `tab\|split\|window`. Réutiliser `cli/internal/termlaunch`. |
| I3 | **Boîte « À traiter »** multi-sessions (⏸ checkpoint, ? question, ! permission, $ budget, ✗ erreur) + fiches de décision + **flux en direct** (lecture seule) dans la vue Sessions. |
| I4 | Notifications système (osascript / `terminal-notifier`) + bip ; CLI `oh session list\|attach\|follow\|approve\|answer\|send\|stop\|resume`. |
| I5 | **Un serveur par groupe (version du workflow, projet, environnement)** ; sessions du groupe = locations/worktrees distincts ; environnement/permissions/instructions propres à chaque session. |
| I6 | Restrictions **optionnelles, désactivées par défaut**, configurables en cascade (hub → équipe recommandé/imposé → projet → workflow) : sessions actives max, budget par session, budget journalier, plafond mémoire. |
| I7 | Mise en veille d'un serveur inactif sans décision en attente après **5 min** (réglable). **En quittant oh** : sessions inactives/en attente → veille immédiate ; sessions qui travaillent → **« finir l'étape puis mettre en veille » (défaut)**, ou « continuer en arrière-plan », ou « arrêter maintenant », session par session. Au redémarrage de oh : récap de ce qui a tourné + coût. |

## T — TUI

| # | Décision |
|---|---|
| T1 | « Démarrer » = **★ épinglés (max 5) + récents (max 3, sans doublon)** ; catégories repliées en mode projet/équipe ; épinglage `*` global ou par projet, stocké dans `oh.db`. Suggestions par défaut si vide. |
| T2 | **Sélecteur de tickets Beads réécrit en tview** (plus de huh pendant la TUI). |
| T3 | Éditeur de prompt intégré → **backlog BL-1** ; en attendant `$EDITOR` (suspension). |
| T4 | **Suppression** de l'UI de déploiement, des étapes « Agents » des assistants, de la config des agents par projet ; la vue `project.agents` devient le **Catalogue des briques** (lecture seule). |
| T5 | Vue `workflow` actuelle (embryon) **remplacée** par Catalogue + Éditeur + Publication + Historique. |
| T6 | Vue `parallel` (vide) **remplacée** par la vue **Sessions**. |
