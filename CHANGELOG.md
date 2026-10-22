# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Format : [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/)
Versioning : [Semantic Versioning](https://semver.org/lang/fr/)

---

## [Unreleased]

## [5.0.0] — 2026-10-22

Version majeure : oh passe sur un runtime de sessions complet (opencode V2, workflows déclaratifs, démon, conteneur) et cesse de déployer des fichiers dans les projets. **Si vous venez de la 4.2.0, lisez d'abord « Ruptures et actions requises »**, puis [MIGRATION.md](MIGRATION.md) (marche à suivre) et le [guide de migration](docs/guides/migration-v5.fr.md). Le détail historique des lots de la v5 est conservé dans [docs/dev/changelog-v5-lots.md](docs/dev/changelog-v5-lots.md) ; certaines de ses entrées ont été dépassées par des lots ultérieurs : ce qui suit fait foi.

### En bref

- **Runtime v5 et opencode V2** : chaque session part d'un paquet compilé hors du projet, servi par un `opencode serve` dédié à son groupe, en « monde fermé » (seuls les agents et skills du paquet sont visibles). opencode V2 est obligatoire (2.0.0 à 2.99.99, **2.0.25 recommandée**). Démon `ohd` (proxy d'identifiants, suivi en direct, veille, reprise), `oh session`. Plus rien n'est déployé dans les projets.
- **Workflows déclaratifs `oh/v1`** : **24 workflows** livrés (`oh run`), **18 agents** en 9 familles, **180 skills**. Trois couches (hub, équipe, projet) qui ne peuvent que durcir les règles de sécurité ; `oh workflow validate|list|show`.
- **Workflows d'équipe et briques** : workflows et briques (agents, skills) publiés dans le team-state, avec brouillons, versions, historique, impact, verrous (`workflows.lock`, `catalog.lock`) et espace solo.
- **Checkpoints contrôlés par oh** : oh lance lui-même les vérifications, garde les preuves, verrouille les opérations sensibles (`commit`, `push`, `close`, `tag`, `mr`…) jusqu'à la décision, y compris pour les sous-agents ; décisions humaines tracées, validation par lot.
- **Conteneur** : Colima, Podman ou Docker, image par projet, aucun secret dans le conteneur, passerelles Beads et MCP, vérifications lancées dans le conteneur (macOS et Linux).
- **Mode distant (expérimental)** : sessions sur GitLab CI (`oh remote setup`, `oh run --runtime remote`), désactivé par défaut, jamais validé sur un vrai runner.
- **Fan-out (expérimental)** : une session enfant par ticket ou sous-tâche, fusion par oh, `cp-merge` obligatoire, désactivé par défaut.
- **Wiki vivant** : `docs/wiki/` tenu à jour par les agents (serveur MCP `wiki`, propositions validées au checkpoint, `oh wiki`, workflow `doc-sync`).
- **Sécurité** : proxy d'identifiants et environnement filtré par liste blanche, garde global des agents, passerelles Beads et MCP fermées par défaut, URL MCP d'équipe approuvées, `oh serve` protégé, empreintes d'opencode, signature des releases. Garde-fous, pas barrières : voir [SECURITY.md](SECURITY.md).
- **TUI refondue** : Démarrer et fiche de lancement, vue Sessions (« À traiter », fan-out, onglets), éditeur de workflows, vue Nettoyage, convention de touches unique, petits terminaux.
- **Mode équipe en bêta** : publié avec avertissement et [limites connues](docs/guides/team-setup.fr.md#limites-connues) ; correction complète prévue en 5.1.
- **CLI plus courte et cohérente** : `oh --help` tient en une page, codes de sortie 0/1/2/3, options courtes à sens unique, confirmation `--yes`, interface fr/en complète, `oh run --dry-run`, `oh budget report`.

### ⚠ Ruptures et actions requises

Décrites pour un utilisateur de la **4.2.0**, dans leur état final. Marche à suivre complète : [MIGRATION.md](MIGRATION.md). Ce qui touche le plus de monde : opencode V2, `execution.env_allow` (`SSH_AUTH_SOCK`, `GH_TOKEN`), codes de sortie et `--yes`, `hub.toml` strict, touches de la TUI, migration de la base.

#### opencode et installation

- **opencode V2 obligatoire** : 2.0.0 à 2.99.99, 2.0.25 recommandée. Sans lui, `oh run` et les anciennes commandes de lancement refusent de démarrer, sans repli sur l'ancien lancement ; opencode V1 n'est plus pris en charge. → Installer opencode V2 avec son propre outil, vérifier la ligne « opencode V2 » de `oh doctor`. ([MIGRATION.md](MIGRATION.md))
- **oh n'installe plus opencode** : `oh upgrade opencode`, l'installation gérée dans `~/.oh/bin` et les clés `[opencode] version|channel|auto_update|install_dir` sont supprimés (clés ignorées, puis perdues au prochain enregistrement de `hub.toml`) ; `oh upgrade` ne met à jour qu'`oh`. → Mettre opencode à jour avec son outil. ([MIGRATION.md](MIGRATION.md))
- **Plugins** : `oh plugin` (`install|list|remove|status`), la vue Plugins et les plugins globaux de V1 (dont RTK) sont supprimés. → Déclarer les plugins voulus dans le workflow (`plugins:`). ([MIGRATION.md](MIGRATION.md))
- **Empreintes d'opencode** : pour les images de conteneur, oh vérifie le paquet Linux d'opencode contre une empreinte épinglée (versions 2.0.20 et 2.0.25) ; une autre version donne un avertissement (aussi dans `oh doctor`), un refus avec `execution.strict_isolation = true`. → Utiliser une version épinglée, ou accepter l'avertissement. ([MIGRATION.md](MIGRATION.md))
- **Installation et mise à jour d'oh plus strictes** : `oh upgrade oh` ne propose jamais une version plus ancienne sans demande explicite ; la signature Cosign est vérifiée si `cosign` est installé (`OH_REQUIRE_SIGNATURE=1` pour l'exiger) ; `install.sh` refuse d'installer sans pouvoir vérifier la somme de contrôle. → Rien à faire, sauf installer un outil de somme de contrôle si `install.sh` refuse. ([MIGRATION.md](MIGRATION.md))

#### Commandes et options

- **`oh deploy` et `oh sync`** : masquées, elles n'affichent plus qu'un message de migration et sortent en code 2. Plus aucune écriture dans `.opencode` ; étapes Agents et Déploiement retirées de `oh init` et `oh project add|configure` ; sélection d'agents par projet supprimée. → Retirer ces commandes des scripts ; lancer `oh migrate deploy-cleanup` pour nettoyer les projets déjà déployés (proposé une fois). ([MIGRATION.md](MIGRATION.md))
- **Anciennes commandes de lancement** : `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich` et `oh service` (`setup`, `remove`) sont des alias dépréciés (absents de `oh --help`, avertissement). `oh start` lance `oh run feature`, c'est-à-dire `spec` (ou `implement-spec` avec des tickets). → Passer à `oh run <workflow>` ; `oh service` est remplacé par `oh mcp`. ([MIGRATION.md](MIGRATION.md))
- **Options de `oh start`** : `--parallel` sans `--tickets` refusé ; `--agent` combiné à un mode refusé (→ `oh run libre --agent`) ; `--max-sessions`, `--priority`, `--sweep-branch-prefix` sans effet ; `--resume <id>` remplacé par `oh session attach <id> --attach suspend`. → Adapter les scripts. ([MIGRATION.md](MIGRATION.md))
- **Skills communautaires** : `oh skill add|list|remove|search` et `oh skill budget` sont supprimés (`oh skill check` reste ; budget d'un paquet : `oh bundle show <workflow> --budget`) ; `~/.oh/skills` n'est plus lu. → Copier les skills voulues dans le catalogue d'équipe ([ADR-051](docs/architecture/adr/051-community-skills-disconnection.fr.md)). ([MIGRATION.md](MIGRATION.md))
- **Codes de sortie** : 0 succès, 1 échec, 2 mauvais usage, 3 décision en attente (`oh run --headless`). Une commande ou une option inconnue sort en **2** (1 en 4.2.0) ; `oh doctor` ne sort en 1 que sur un échec obligatoire, les avertissements donnent 0. → Revoir les scripts qui testent `$? == 1` ([référence](docs/reference/cli.fr.md)). ([MIGRATION.md](MIGRATION.md))
- **Options courtes : une lettre, un sens** (`-p` projet, `-m` message, `-y` oui, `-i` entrée, `-d` chemin, `-b` base, `-t` ticket…). Anciennes formes acceptées pendant une version, avec avertissement : `oh serve -p`, `project configure -m`, `metrics -d`, `review -m|-b`, `debug -i`, `audit -t`, `start -w|-l`, `session attach --how`. → Utiliser les nouvelles formes ([tableau](docs/reference/cli.fr.md)). ([MIGRATION.md](MIGRATION.md))
- **Formes supprimées sans alias** : `oh config model … -j` (→ `-p`) et `oh purge --include-opencode` (→ `--include-tool-data`) ; code 2. → Remplacer dans les scripts. ([MIGRATION.md](MIGRATION.md))
- **`oh status --json`** donne `tool` et `tool_version` au lieu de `opencode_version`. → Adapter les scripts. ([MIGRATION.md](MIGRATION.md))
- **Projet implicite** : hors du dossier d'un projet, oh ne prend plus en silence le seul projet enregistré ; une commande qui écrit demande confirmation et refuse sans terminal, une commande qui lit annonce le projet. → Se placer dans le projet ou passer `-p`. ([MIGRATION.md](MIGRATION.md))
- **Confirmation sans terminal** : une action qui demande confirmation est **refusée** quand oh n'a pas de terminal (script, CI, entrée redirigée), sauf `--yes` (`-y`) ; message « confirmation requise : relancez avec --yes », code **2**. Concerne `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete|resolve`, `migrate deploy-cleanup` et `run --recap` ; `--yes` est ajouté à `run`, `purge`, `project remove`, `teams remove`, `service remove` et `worktree cleanup`. `oh gc`, `oh doctor --fix` et quelques autres refusaient déjà sans `--yes` (code 1, inchangé). Avant, `workflow publish|archive|restore` et `brick publish` répondaient « oui » seuls. → Ajouter `--yes` aux scripts et à la CI. ([MIGRATION.md](MIGRATION.md))
- **`--force` ne saute plus une confirmation** : sur `purge`, `project remove`, `teams remove` et `service remove`, `--force` et `-f` restent acceptés une version (même effet, avertissement, absents de l'aide) et disparaissent en v6 ; `--yes` les remplace. `--force` garde son sens là où il outrepasse une sécurité : `daemon stop`, `remote setup`, `worktree remove`, `worktree cleanup` (supprime aussi les worktrees modifiés). → Remplacer par `--yes`. ([MIGRATION.md](MIGRATION.md))
- **`oh --help` compact** : une ligne par commande de premier niveau (≈ 70 lignes au lieu de 573), section « Avancé » ; l'ancien affichage complet est `oh help --all` (ou `oh --all`) ; `oh <commande> --help` est inchangé. → Un script qui analysait `oh --help` passe à `oh help --all`. ([MIGRATION.md](MIGRATION.md))
- **`oh init` en 3 écrans** en solo (accueil avec langue et type d'installation, fournisseur, premier projet facultatif). Le type par défaut est « solo » (« complet » si une équipe est configurée) et les intégrations MCP ne sont plus demandées. → Lancer `oh mcp setup` ; le récapitulatif le suggère. ([MIGRATION.md](MIGRATION.md))

#### Workflows et couches

- **`feature` et `cadrage` sont des alias** (pas de workflow de ce nom) : `feature` et `cadrage` lancent `spec`, `feature` avec `--tickets` lance `implement-spec` (qui refuse `-m` : consigne par `-i instructions=…`). Checkpoints renommés (`cp-plan` pour `cp-0`, `cp-final` pour `cp-feature`). Une couche qui étend `hub:feature` ou `hub:cadrage` est refusée (`extends_alias`) ; `oh doctor` liste les « Workflows retirés ». → Réécrire avec `extends: hub:spec` ou `hub:implement-spec`. ([MIGRATION.md](MIGRATION.md))
- **Alias lus dans les workflows** : `secu` (`security-fix`), `perf` (`benchmark`), `dbg` (`debug`), `feedback` et `retours` (`review-feedback`), `free` (`libre`). Les termes en dur `dev`, `start`, `fast`, `rev` et `orchestrator` ne sont plus reconnus dans la palette ni par `oh run`. → Utiliser les identifiants ou alias. ([MIGRATION.md](MIGRATION.md))
- **`libre`** remplace `oh start --agent` et la session libre de la TUI : l'auto-délégation n'est plus déduite de la permission `task`, profondeur de délégation 2, agent d'entrée `orchestrator` par défaut, pas de verrou de commit (voulu). → `oh run libre --agent <id>`. ([MIGRATION.md](MIGRATION.md))
- **Agents** : 18 au lieu de 19. Retirés : `developer-refactor`, `developer-migrator`, `test-generator`, `infra`, `database` ; ajoutés : `conductor`, `merger`, `operator`, `release-manager`. Un workflow d'équipe qui nomme un agent retiré est refusé (`agent_removed`, avec le remplaçant). → Retirer ces membres de vos workflows. ([MIGRATION.md](MIGRATION.md))
- **Une couche de projet ne peut plus ajouter d'agent ni de `skills.extra`** à un workflow hérité (erreurs `patch_agent_refused`, `patch_skills_extra_refused`) : le contenu du dépôt ne choisit pas les agents. Une couche d'équipe garde l'avertissement. → Déclarer l'agent ou la skill dans le workflow d'équipe, puis l'étendre. ([MIGRATION.md](MIGRATION.md))
- **Les couches ne peuvent que durcir** : `skills.deny`, préconditions des workflows `risk: publish`, `mcp`, `plugins`, `after:`, `calls`, `circuit_breaker`, `writes`, `code_mode`, `modes.allowed`, `checkpoints.<id>.approver` (couche d'équipe seulement) ; un assouplissement est refusé (`loosening`). → `oh workflow validate` sur vos couches. ([MIGRATION.md](MIGRATION.md))
- **Anciennes surcharges du workflow** (`[workflow]` du hub, de l'équipe ou du projet) : migrées automatiquement (migration v38) vers des workflows d'équipe ou de projet qui étendent `implement-spec` ; originaux archivés dans `~/.oh/migrated/`. → Vérifier ces workflows. ([MIGRATION.md](MIGRATION.md))
- **Sessions contrôlées par oh** : les verrous des checkpoints (`unlocks`) s'appliquent à tous les agents ; les sous-agents sont fermés par défaut : le plugin oh relaie les opérations verrouillables, **démon arrêté elles sont refusées**, et sans plugin oh actif un workflow qui délègue à des sous-agents ne démarre pas ; une vérification (`verify:`) d'un checkpoint tourne dans le conteneur du projet quand la session y est ; `expect: fail` exige build, typecheck et lint verts. → Garder le démon (`oh daemon status`). ([MIGRATION.md](MIGRATION.md))
- **Passerelle Beads** : `risk: read` est vraiment en lecture seule (`bd comments add`, `bd ready --claim` refusés), `--actor="<personne> via oh/<agent>"` est imposé, `beads.allow` est appliqué aussi en local par un faux `bd` en tête du `PATH` ; un chemin absolu vers `bd` est refusé aux agents. → Aucune action, sauf scripts d'agents qui appellent `bd` par son chemin. ([MIGRATION.md](MIGRATION.md))
- **Briques et budgets** : avec `catalog.lock`, une brique modifiée à la main ou commitée sans oh est ignorée (catalogue sans lock : `oh brick publish --adopt`) ; une décision `$` est levée dès que le dépassement est vu ; chaque enfant de fan-out reçoit une part du budget restant ; `limits.models` est vérifiée au lancement. → Publier avec `oh brick publish`. ([MIGRATION.md](MIGRATION.md))
- **Identifiants Bedrock** : `provider.bedrock.auth_mode` est respecté : `bearer` n'utilise que la clé API enregistrée, `profile` et `env` signent en SigV4 et ignorent toute clé enregistrée ; une clé refusée par AWS déclenche une alerte. → Vérifier `auth_mode` ; `oh provider setup bedrock --from-env`. ([MIGRATION.md](MIGRATION.md))

#### Configuration

- **`hub.toml` lu strictement** : chaque valeur doit avoir le type de son réglage. Sont refusés, avec fichier, ligne, colonne et clé : `"true"`, `"1"` ou `1` pour un booléen ; un nombre entre guillemets ; un décimal pour un entier ; un nombre ou un booléen pour un texte ; un texte pour une liste (`"A,B"`). Un fichier écrit par oh reste valide. Une clé inconnue est ignorée avec un avertissement (perdue à la prochaine écriture) ; les clés des tables `remote.projects` et `binaries` gardent leur casse. → Corriger les types ; `oh doctor` affiche le même message, `oh doctor --fix` restaure la dernière sauvegarde qui se charge ([détail](docs/reference/config.fr.md#lecture-stricte-de-hubtoml)). ([MIGRATION.md](MIGRATION.md))
- **`./hub.toml` n'est plus lu** : sans `~/.oh/hub.toml`, la 4.2.0 lisait un `hub.toml` du dossier courant (un dépôt cloné pouvait injecter une configuration). Seul le dossier du hub compte (`~/.oh` ou `OH_HOME`). → Placer la configuration dans `~/.oh/hub.toml`. ([MIGRATION.md](MIGRATION.md))
- **Clés renommées** : `[llm] default_provider` (ex `[opencode] default_provider`), `[execution] tool_version` (ex `opencode_version`), clés du trousseau MCP `openhub.mcp.<service>.token[.<projet>]` (ex `<service>-token`) ; `oh purge --include-tool-data` ; `OH_TOOL_VERSION` côté runner. Les anciens noms sont encore lus, signalés une fois par commande (et par `oh doctor`) et **ne seront plus lus en v6** ; `oh config set` accepte encore `opencode.default_provider` mais refuse `execution.opencode_version`. → Renommer à la main. ([MIGRATION.md](MIGRATION.md))
- **Clés supprimées** : `[opencode]` (voir plus haut), `[deploy] disable_native_agents` (le monde fermé masque toujours les agents natifs), `[workflow.overrides]`, `[parallel]` de l'équipe (sans effet) : ignorées, puis perdues à l'enregistrement. → Rien à faire. ([MIGRATION.md](MIGRATION.md))
- **`policies.toml` réécrit en entier** à chaque modification (`oh policies add`, vue Politiques) : les commentaires et la mise en forme d'un fichier écrit à la main sont perdus. → Ne pas compter sur les commentaires du fichier. ([MIGRATION.md](MIGRATION.md))

#### Sécurité visible

- **Environnement filtré par liste blanche** : le serveur opencode local et les shells des agents ne reçoivent plus tout l'environnement ; `SSH_AUTH_SOCK`, `GH_TOKEN`, `GEMINI_API_KEY`, `NPM_TOKEN`… ne passent plus. Les identifiants gérés par oh (`AWS_*`, `ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GITLAB_TOKEN`…) sont refusés quoi qu'on configure ; conteneurs et jobs distants n'héritent de rien. → Pour git par ssh, `gh` ou un outil qui a besoin d'une variable : `oh config set execution.env_allow "SSH_AUTH_SOCK, MYCO_*"` ; `oh doctor` liste les variables retenues. ([MIGRATION.md](MIGRATION.md))
- **Garde global des agents** : ils ne peuvent plus lancer `oh mcp trust|untrust|setup|enable|disable|reset|serve`, `oh config set|unset|language|websearch` et `oh config model default|family|agent|unset`, `oh secrets set|delete|get|cleanup`, `oh budget set|unset|raise`, `oh service setup|remove`, `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove`, `oh teams remove|archive|restore` (`--yes` compris) ; les lectures restent permises. → Lancer ces commandes soi-même. ([MIGRATION.md](MIGRATION.md))
- **`oh mcp trust <service>`** : une URL MCP imposée par l'équipe n'est utilisée qu'une fois approuvée (épinglée dans `[mcp.<service>] trusted_url` de votre `hub.toml`) ; https obligatoire hors loopback ; sans approbation, le jeton n'est pas envoyé et `oh doctor`, `oh mcp status` et la vue MCP avertissent. → Chaque membre lance `oh mcp trust <service>`. ([MIGRATION.md](MIGRATION.md))
- **`oh serve`** : jeton aléatoire par lancement (URL `?token=…`, puis cookie), 401 sans jeton, 403 si `Host` n'est pas `127.0.0.1`, `localhost` ou `[::1]`. → Ouvrir l'URL affichée ; `curl` avec `?token=…`. ([MIGRATION.md](MIGRATION.md))
- **Proxy d'identifiants** : il ne relaie plus les en-têtes ni la query de l'agent tels quels (liste blanche par fournisseur) ; un en-tête personnalisé ajouté aux options du fournisseur d'opencode est retiré. → Cas rare, rien à faire. ([MIGRATION.md](MIGRATION.md))

#### Touches de la TUI

- **Contrat des touches** : taper une lettre n'ouvre plus la palette (`Ctrl+P` ou `/`) ; `Entrée` et les lettres ne sont plus pris à la liste ; `d` ne ferme plus les toasts (`Échap`) ; dans un assistant, `Échap` demande « Quitter l'assistant ? » ; l'aide est sur `?` ou `F1` (aussi dans les onglets d'une session). → Apprendre `Ctrl+P` et `?`. ([MIGRATION.md](MIGRATION.md))
- **Convention unique** : `n` crée partout, `d` supprime (toujours confirmé), `x` archive, `R` reprend ou restaure, `c` compare, `y` `n` `e` `s` `A` décident. Changent pour vous : wiki d'équipe `a`/`x` → `y`/`n`, patterns `a`/`v` → `n`/`y`, projets `a`/`n` → `n`/`e`, board d'équipe `f`/`x` → `/`/`L`, worktrees `a`/`x` → `n`/`M`, détail d'équipe `y` → `T`, autres listes `a` → `n`. → Un message « Nouvelles touches » s'affiche au premier lancement, puis dans l'aide ([référence](docs/reference/tui.fr.md)). ([MIGRATION.md](MIGRATION.md))

#### Mode distant et fan-out

- **Mode distant et fan-out expérimentaux, désactivés par défaut** : sans `remote.experimental`, `oh remote setup`, `oh run --runtime remote`, l'option ☁, `oh session fetch|resolve` sont refusés ; sans `fanout.experimental`, `-i fanout=true` est refusé. Rien n'est à relancer depuis la 4.2.0 (ces fonctions n'y existaient pas) ; les testeurs de builds intermédiaires relancent `oh remote setup` (pipeline de schéma 5). → `oh config set remote.experimental true` (ou `fanout.experimental`) pour essayer, avec des projets et jetons de test. ([MIGRATION.md](MIGRATION.md))

#### Base de données

- **Base migrée de la version 27 à 49** au premier lancement : verrou entre processus, copie `~/.oh/backups/oh.db.v27-<date>.bak` avant migration (3 copies gardées), base plus récente refusée (`OH_ALLOW_NEWER_SCHEMA=1`). `~/.oh` passe en 0700 et `oh.db` en 0600 ; les mots de passe des serveurs sortent de `oh.db` (`oh export` n'en emporte plus). **Pas de retour arrière simple vers la 4.2.0.** → Garder la copie ; revenir en arrière = restaurer une sauvegarde (`oh repair --restore <fichier>`). ([MIGRATION.md](MIGRATION.md))

#### Mode équipe en bêta et jeton GitLab

- **Mode équipe en bêta** : avertissement une fois par processus sur stderr (`oh team …`, premier claim d'un lancement) et dans `oh doctor` (sans effet sur le code de sortie). Limites : claim libéré à l'arrêt de la session même avec `cp-2` en attente, pas d'expiration (reprise manuelle après `takeover.stale_days`), libération perdue hors ligne ou si le démon s'arrête (`oh team release`). → Lire les [limites connues](docs/guides/team-setup.fr.md#limites-connues). ([MIGRATION.md](MIGRATION.md))
- **Un seul ordre de recherche du jeton GitLab** (sync du tracker, `oh mr create`, `oh review --publish`, `oh review feedback`, `oh team init|rejoin`, `oh mcp enable --project`) : `GITLAB_TOKEN`, `tracker_token_key`, jeton du projet, jeton de l'équipe, jeton MCP du hub, ancienne clé `openhub.tracker.gitlab.token`. Le jeton de l'équipe du projet passe avant le jeton MCP du hub. → Vérifier quel jeton est utilisé avec `oh doctor`. ([MIGRATION.md](MIGRATION.md))
- **La TUI ne synchronise plus un tracker désactivé** (`[tracker] enabled = false`). → Rien, sauf si vous comptiez sur la synchronisation. ([MIGRATION.md](MIGRATION.md))

### Added

**Runtime et sessions**
- Sessions sur le runtime v5 : paquet compilé (`~/.oh/bundles/<hash>`, fichiers en lecture seule, empreinte vérifiée), serveur dédié par groupe, plugin oh embarqué, `OH_HOME` pour relocaliser `~/.oh`.
- Démon `ohd` (`oh daemon status|stop`) : proxy d'identifiants (Bedrock par jeton, profil ou SigV4, Anthropic, OpenRouter), suivi en direct (état, coût, tokens), veille des serveurs inactifs, reprise.
- `oh session attach|list|stop|inbox|approve|answer|dismiss|send|follow|interrupt|compact|model|fork|results|resume|trace|report|fetch|resolve|delete` ; ouverture en onglet ou fenêtre (iTerm2, Terminal.app, tmux, navigateur) ; décisions en attente (permissions, questions, budget) avec « première réponse gagne » ; notifications système ; clés LLM au niveau équipe ; réglages `[session]`.
- Restrictions des sessions (désactivées par défaut) : sessions actives maximum, budget par session et par jour, mémoire, modèles autorisés (`oh budget show|set|unset|raise`).
- `oh run --dry-run` : prépare le lancement et affiche récapitulatif, pauses et avertissements sans démarrer (ni session, ni worktree, ni ticket réservé, ni question) ; refusé en distant.
- `oh budget report [--by project|workflow] [--since 30d] [--json]` : dépenses par projet ou workflow ; les sessions supprimées par `oh gc` comptent sous « inconnu ».
- Autres commandes : `oh gc` et `[gc]` (nettoyage automatique), `oh repair --migrate|--restore`, `oh doctor --fix|--offline`, `oh config list --all|--keys`, `oh config model show --workflow`, `oh provider setup --from-env`, `oh mr create`, `oh migrate deploy-cleanup`.
- Windows : sessions locales (démon dans le processus d'oh, sans arrière-plan) ; ni conteneur ni mode distant, pas d'`oh upgrade oh` ; plateforme moins vérifiée (27 paquets exclus de la CI).

**Workflows**
- Schéma `oh/v1` lu strictement (erreurs avec ligne et colonne), `extends`, validation complète (agents, graphe de délégation, gabarits, skills, risques) ; `oh workflow list|show|validate`, `oh bundle build|show`, `oh run` (entrées `-i`, mode, runtime, emplacement, `--tickets`, `--headless`, `--draft`).
- **24 workflows** : `ticket`, `quick`, `libre`, `spec`, `implement-spec`, `sweep`, `sweep-task`, `merge-conflicts`, `review`, `review-feedback`, `audit`, `security-fix`, `debug`, `incident`, `spike`, `benchmark`, `infra-change`, `onboarding`, `doc-sync`, `skill-refresh`, `release`, `retro`, `triage`, `brief-enrich`.
- Composition : suites `next:` entre workflows, sorties relevées par oh, sources d'entrée `session.output(…)`, alias (`aliases:`), préconditions, plugins et Code Mode déclarés par workflow, `risk: plan|read|publish`, mode TDD (`cp-seams`, `cp-red`), tâches de `sweep` relevées par oh.
- Checkpoints : `verify:` lancé par oh, preuves, limite de tours de correction, fenêtres bornées, `unlocks:`, valideur humain (`approver`), coupe-circuit, budget de pauses, rapports stockés par oh (`report`, `report_get`, `oh session report`), brouillons de session (`scratch`).
- Évals : `oh eval` (vrai modèle, hors CI) avec 71 cas, référence de 35 cas, `--tiered`, `--plan`, `--variant`, `--parallel`, `--budget-total`.

**Équipe et briques**
- Workflows d'équipe et de projet dans le team-state (`workflows.lock`, verrous `enforce`, gouvernance `any_member`), brouillons, publication (file hors ligne, reprise après publication concurrente), historique, restauration, archivage, résumé d'impact ; `oh workflow new|edit|diff|publish|history|restore|archive`.
- Espace solo (`oh team init --solo`, `oh team promote --remote`) ; catalogue de briques d'équipe (`oh brick`, [ADR-054](docs/architecture/adr/054-team-brick-catalogue.fr.md)) ; service équipe commun à la CLI, la TUI et la synchronisation automatique.

**Conteneur, distant, fan-out**
- Conteneur : image par projet, passerelles Beads et MCP, config projet › Exécution, `oh doctor` conteneur, `make test-container`.
- Distant : projet `oh-runner`, pipeline généré par oh, `oh remote setup|status`, `oh runner`, répondeur de politique, retour par « À récupérer » et `oh session fetch|resolve`.
- Fan-out : sessions enfants, merge par oh, agent `merger`, workflow `merge-conflicts`, budget global (`[fanout] budget_usd`, 10 $), validation par lot (`oh session approve --batch`), tour de correction.

**Wiki, skills, agents**
- Wiki vivant : serveur MCP `wiki`, `oh wiki check|pending|decide|stats`, motif de refus d'une proposition, `doc-sync`, wiki en session distante.
- Skills : `requires:`, `annexes:`, `when:`, fraîcheur des skills de stack (`oh skill check --stale|--strict|--json`), catalogue de skills propre à chaque agent ; contrat de session et protocole de retour uniques ; détection Django, FastAPI, Flask, Rails, Java.

**TUI et documentation**
- TUI : Démarrer, fiche de lancement, vue Sessions, validation par lot, catalogue et éditeur de workflows, Nettoyage, Doctor (`f`), petits terminaux (`[cli] compact`), `NO_COLOR`, symboles communs à la CLI et à la TUI.
- Guides et références : migration v5, sessions v5, conteneur, exécution distante, workflows d'équipe, fan-out, MCP GitLab, page « Où se règle quoi » ; référence CLI découpée par thème ; rédaction des skills et cascade des modèles mises à jour.

### Changed

- **CLI** : `oh --help` construit depuis l'arbre des commandes (une ligne par commande, `oh help --all` pour tout) ; codes de sortie 0/1/2/3 ; options courtes à sens unique ; erreurs, messages et boutons traduits (fr/en) ; `oh init` en 3 écrans ; « mode » ne désigne plus que le mode de workflow (type d'agent, vue Hub / Projet / Équipe, type d'installation).
- **Workflows** : `manual` accepté partout où l'on saisit un mode (lu comme `manuel`) ; commandes TUI `coder` renommées `libre` (anciens noms en alias) ; `oh metrics`, tableau de bord et API tirent leurs statistiques du registre des sessions d'oh (`oh.db`).
- **Configuration** : `hub.toml` lu par un décodeur TOML strict (viper retiré) et écrit de façon atomique et durable ; anciennes clés signalées une fois et listées par `oh doctor` ; `oh config set|unset` couvre `session.*`, `execution.*`, `limits.*`, `remote.*` ; clones du team-state sous `OH_HOME`.
- **Écritures** : écritures atomiques durables (`internal/fsutil` : temporaire unique, fsync, rename) pour secrets, trousseau, mots de passe des serveurs, passerelle, rapports, wiki, transcript, config d'équipe, import d'une sauvegarde et historique des notifications.
- **Base de données** : `oh export` archive une copie cohérente de la base ; chaque migration SQLite de v24 à v34 a un retour arrière ou est déclarée irréversible (test d'aller-retour) ; schéma de la base 49.
- **Contenu** : 18 agents (19 en 4.2.0), 180 skills ; prompts plus courts (catalogue par agent, découpage par phase, variantes par rôle) ; orchestrateur recentré sur `libre` (les modes A à E sont devenus des workflows).
- **TUI** : convention de touches unique et vérifiée ; vue Sessions : aide en 8 touches essentielles et `? aide` (`?` liste tout) ; chargements asynchrones gardés (Historique, Métriques, Doctor, Nettoyage) ; confirmations avant toute suppression.
- **Mode équipe** : colonne REVIEW du board (ticket `in_progress` portant le label `ready-for-review`), vote de review MCP par `review` puis `done` ; politiques écrites par le service `policy` (un commit, `policies: init|update`).
- **Publication et CI** : un tag `v5.0.0-rc.N` est une pré-version GitHub (jamais « latest », Homebrew non mis à jour) ; notes de version tirées de cette section (`.github/scripts/release-notes.sh`) ; opencode 2.0.25 épinglé en CI.
- **Suite de tests** : environ 460 tests de premier niveau en moins (2 963 → 2 505), outils communs (`sqlitetest`, `gittest`, `configtest`, `testutil/golden`, `i18ntest`, `bdtest`), un seul passage `-race` par système en CI ; durée `-short` 261 s → 107 s (−59 %), complète `-race` 974 s → 384 s (−61 %), sans baisse de couverture de la suite complète ; tests isolés du vrai hub et du vrai `HOME`.
- **Architecture** : outil des sessions isolé derrière un adaptateur ([ADR-049](docs/architecture/adr/049-tool-independence-architecture-guard.fr.md)) ; règles de couches vérifiées par `archtest` avec liste figée d'exceptions ; `golangci-lint` sans signalement.

### Deprecated

- Alias toujours fonctionnels (avertissement, absents de `oh --help`) : `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich`, `oh service` (`setup`, `remove`), options `--max-sessions`, `--priority`, `--sweep-branch-prefix`, `--yes` de `oh start`.
- Anciennes formes d'options courtes (`oh serve -p`, `project configure -m`, `metrics -d`, `review -m|-b`, `debug -i`, `audit -t`, `start -w|-l`, `session attach --how`), acceptées une version avec avertissement ; `--force`/`-f` comme équivalent de `--yes` (`purge`, `project remove`, `teams remove`, `service remove`), retiré en v6.
- Anciennes clés lues en repli, plus lues en v6 : `[opencode] default_provider`, `[execution] opencode_version`, noms de trousseau MCP `<service>-token`.

### Removed

- **opencode V1** et tout l'ancien lancement (interactif, sans interface, parallèle et sweep V1), `oh upgrade opencode`, l'installation gérée dans `~/.oh/bin`, `oh plugin`, la vue Plugins et les plugins globaux de V1.
- **Déploiement par projet** : `oh deploy` et `oh sync` (messages de migration seulement), étapes Agents et Déploiement de `oh init` et `oh project add|configure`, liens `.opencode` des worktrees, sélection d'agents par projet (`projects.agents`).
- Registre de skills communautaires : `oh skill add|list|remove|search|budget`, lecture de `~/.oh/skills` ([ADR-051](docs/architecture/adr/051-community-skills-disconnection.fr.md)).
- Ancienne vue Workflow et ancien modèle de workflow codé en dur ; sections `[opencode]`, `[deploy] disable_native_agents`, `[workflow.overrides]` de `hub.toml`.
- Agents `developer-refactor`, `developer-migrator`, `test-generator`, `infra`, `database` ; skill `orchestrator-dev-parallel` (remplacée par le fan-out) ; modes d'entrée A à E de l'orchestrateur.
- `oh config model … -j` (→ `-p`), `oh purge --include-opencode` (→ `--include-tool-data`), `opencode_version` de `oh status --json` (→ `tool_version`).

### Fixed

- **Sessions et lancement** : prompt initial qui se perdait au lancement de plusieurs tickets ; shell local qui perdait le `PATH` ; bascule silencieuse vers des modèles hébergés par opencode ; modèles Bedrock sans profil régional ; lancement dans un dossier de base modifié ; coût total des sessions, bilan de `oh session results`, état après `oh budget raise`, réservations d'équipe à l'arrêt d'une session.
- **Workflows** : commit et fermeture du ticket garantis par oh dans `ticket` ; hooks git de Beads dans les sessions ; `oh run review-feedback -i mr=…` lit la MR sur GitLab ; `oh workflow show|validate` suivent la chaîne complète des couches ; migration v38 qui archive au lieu de vider une configuration illisible.
- **Budgets** : décompte complet du proxy (cache en lecture et écriture), réservation des requêtes concurrentes, sous-agents relus après un redémarrage du démon, modèles vérifiés au lancement.
- **Permissions (F-12)** : jokers des règles évalués par caractère (`?` ne correspondait pas à un caractère accentué) via un paquet commun `internal/wildcard` (limites, proxy, permissions).
- **Mode équipe et politiques** : la colonne REVIEW du board n'était jamais remplie ; l'approbation MCP d'un review passe par `review` puis `done` et remonte un échec de transition ; ajout, édition et suppression d'une politique ne bloquent plus la TUI et n'écrivent plus de fichier TOML invalide ; boards d'équipe dont les étiquettes disparaissaient.
- **TUI** : une réponse obsolète n'est plus appliquée (Historique, Métriques, Doctor, Nettoyage) ; Réglages et MCP ne gèlent plus l'interface au montage ; fiche de décision fermée quand la décision est réglée ailleurs ; course de données sur la configuration dans la vue MCP ; fenêtres de sessions qui se multipliaient.
- **Configuration et CLI** : `oh config model unset family|agent`, `oh import --merge`, `oh mcp setup <service>`, GitHub et Linear (`[mcp.github]`, `[mcp.linear]`), `oh <commande> --help` qui affichait l'aide générale, `oh mcp setup` sans terminal (message clair), `review-feedback` sur un projet GitHub (message explicite au lieu d'une erreur de jeton), `oh init` qui écrasait la première équipe.
- **Fan-out et wiki** : arrêt juste après une fusion, enfant orphelin, enfant fusionné deux fois, `oh wiki check --fix` sur les god nodes.
- **Publication** : `release.yml` ne partait jamais sur un tag (déclencheur filtré sur la branche `main`) ; les liens relatifs du CHANGELOG sont réécrits en URL absolues vers le tag dans les notes de la release GitHub.
- **`oh purge`** n'arrête plus que les processus de son propre hub (le démon par sa socket, les serveurs d'outil enregistrés dans la base, après vérification de leurs identifiants) : il ne lance plus `pkill -f "oh serve"` ni `pkill -f "oh mcp"`, qui pouvaient tuer des processus d'autres hubs ou sans rapport avec oh. Ce qu'oh ne suit pas (`oh serve`, serveurs MCP lancés à la main, conteneurs) est listé « à arrêter à la main ».

### Security

- **Proxy d'identifiants et démon** : jetons de session dont seule l'empreinte est gardée, révocations fiables, client qui vérifie le démon avant d'envoyer, démon qui survit à une panique, région Bedrock et modèles validés, seconde écoute limitée aux ponts des moteurs de conteneurs, délai de 5 minutes avant 504.
- **Environnement et secrets** : liste blanche (`execution.env_allow`), secrets du serveur cachés au shell des agents, mots de passe des serveurs hors de `oh.db`, `~/.oh` en 0700 et `oh.db` en 0600, démarrage du shell contrôlé (`ZDOTDIR`, `BASH_ENV`, `ENV`, fish).
- **Garde des agents** : garde global (git destructif, commandes `oh` de configuration, de secrets, de publication et de suppression, y compris avec `--yes`), permissions des agents restreintes (outils d'infrastructure, `debugger`, `benchmarker`), frontière de confiance « contenu lu = donnée » dans chaque agent, balises de données dans les prompts.
- **Checkpoints et couches** : un agent ne peut plus passer lui-même un checkpoint ; sous-agents fermés par défaut ; une couche de projet ne peut plus ajouter d'agent ni de skills ; les plugins de la configuration personnelle d'opencode sont signalés au lancement et par `oh doctor` (`strict_isolation` les masque).
- **Beads et MCP** : passerelle Beads qui lit les arguments comme `bd`, hooks git vérifiés, lecture seule réelle, auteur et provenance imposés ; passerelle MCP fermée par défaut, URL d'équipe approuvées (`oh mcp trust`), chemins d'API échappés, identifiants Figma et Google Slides validés (`file_key`, `presentation_id`, `slide_id`), identifiant de session avec `../` refusé.
- **Intégrité et chaîne d'approvisionnement** : paquets de session en lecture seule avec empreinte recalculée, empreintes des paquets Linux d'opencode 2.0.20 et 2.0.25 (`.github/scripts/opencode-pin.sh`), écritures atomiques, signature Cosign des releases vérifiée, redirections de mise à jour limitées à GitHub, git d'oh neutralisé contre la configuration du dépôt, `./hub.toml` ignoré, `oh serve` protégé par jeton, dépendances à jour.
- Limites assumées : [SECURITY.md](SECURITY.md) (isolation stricte non activée par défaut, `make`/`npm run`, jeton MCP par groupe, bash interactif, couche d'équipe en avertissement, Cosign facultatif) ; suivi des constats : [audit v5](docs/dev/audit-v5.md).

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
