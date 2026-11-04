> [Read in English](migration-v5.en.md)

# Migrer vers oh v5

oh v5 lance chaque session à partir d'un **workflow** (`oh run <workflow>`), dans un **paquet de session** construit hors du projet, sur un serveur **opencode V2**. Ce guide liste ce qui change pour un utilisateur de la v4 (4.2.0). Les mêmes ruptures, classées par thème avec l'action à faire, sont dans [MIGRATION.md](../../MIGRATION.md). **Vous venez de la 5.0 ?** Allez à [Depuis la 5.0 vers la 5.1](#depuis-la-50-vers-la-51). **Vous venez de la 5.1 ?** Allez à [Depuis la 5.1 vers la 5.2](#depuis-la-51-vers-la-52).

## Mise à jour pas à pas

1. **Sauvegardez** : `oh export -o ~/oh-backup-4.x.tar.gz` (base, `hub.toml`, secrets chiffrés), et gardez le binaire 4.2.0 (§19).
2. **Installez opencode V2** (§1), puis **mettez oh à jour** : `brew upgrade openhub` ou `oh upgrade oh`.
3. **Lancez `oh doctor` en premier.** Le premier lancement migre la base (27 → 49, avec copie, §19) et les anciennes surcharges `[workflow]` (§9) ; il n'y a pas de commande de migration à lancer : elle tourne au démarrage de toute commande **sauf `oh init` et `oh purge`**. Ne modifiez pas `hub.toml` à la main avant ce premier lancement.
4. **Corrigez ce que `oh doctor` signale** : `hub.toml` refusé (§13), clés héritées, anciens déploiements (§8), `~/.oh/skills` (§10), version d'opencode. `oh doctor --fix` répare après un récapitulatif et une confirmation (`--yes` sans terminal).
5. **Retirez les anciens déploiements** : `oh migrate deploy-cleanup` (§8).
6. **Autorisez les variables d'environnement** dont vos agents ont besoin : `oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN"` (§14).
7. **En équipe** : `oh mcp trust <service>` pour chaque URL imposée (§20), et lisez le mode équipe (§17).
8. **Mettez à jour scripts et CI** : `oh deploy`/`oh sync`, `oh start` → `oh run`, `--yes` (§12), code de sortie 2 (§11).
9. **Vérifiez vos workflows** : `oh workflow validate --all` (§15).

## 1. opencode V2 obligatoire

opencode V1 n'est plus pris en charge, sans repli. oh v5 demande une version d'opencode comprise entre **2.0.0** et **2.99.99** ; la version **recommandée** est **2.2.5** (celle de la CI). Hors de cette plage, `oh run` refuse de démarrer et l'explique :

```text
opencode 1.18.29 n'est pas pris en charge : oh v5 demande la version 2.0.0 ou plus récente.
Installez une version prise en charge de opencode, puis lancez oh doctor. Guide : docs/guides/migration-v5.fr.md
```

Sans opencode : « opencode est introuvable : oh v5 en demande une version prise en charge ».

1. Installez opencode V2 avec son propre outil (`brew install opencode`, ou https://opencode.ai).
2. Vérifiez avec `oh doctor` : la ligne **opencode** doit être verte (✓).

L'empreinte des paquets Linux d'opencode est épinglée dans oh pour les versions **2.0.20**, **2.0.25** et **2.2.5**. Une autre version de la plage fonctionne, mais `oh doctor` l'avertit : les images de conteneur la téléchargeraient sans empreinte vérifiée, et `execution.strict_isolation = true` la refuse ([SECURITY.fr.md](../../SECURITY.fr.md#chaîne-dapprovisionnement)).

`oh upgrade opencode`, l'installation gérée dans `~/.oh/bin` et les clés `[opencode] version`, `channel`, `auto_update`, `install_dir` de `hub.toml` sont supprimées (ignorées si elles restent). `[llm] default_provider` reste.

## 2. Anciennes commandes : remplacées par `oh run`

Ces commandes étaient des alias dépréciés pendant la 5.0 ; **la 5.1 les a retirées** ([Depuis la 5.0 vers la 5.1](#depuis-la-50-vers-la-51)). Le tableau donne la commande à utiliser :

| Ancienne commande | Équivalent v5 |
|---|---|
| `oh start` | `oh run feature` (alias de `spec`, ou de `implement-spec` avec `--tickets`, §15) |
| `oh start --agent <id>` | `oh run libre --agent <id>` |
| `oh start --dev [-t <id>]` | `oh run ticket --tickets <id>` |
| `oh start --onboard [--refresh]` | `oh run onboarding [-i refresh=true]` |
| `oh start --parallel --tickets a,b` | `oh run ticket --tickets a,b` |
| `oh start --sweep <objectif>` | `oh run sweep -i goal=<objectif>` |
| `oh start --resume <id>` | `oh session attach <id> --attach suspend` |
| `oh audit`, `oh review`, `oh debug` | `oh run audit`, `oh run review`, `oh run debug` |
| `oh review feedback <mr>` | `oh run review-feedback -i mr=<mr>` (le workflow lit les discussions GitLab lui-même) ; `oh review --publish` : `oh mr create` |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |
| `oh service` (`setup`, `remove`) | `oh mcp status`, `oh mcp setup`, `oh mcp disable` |

Ne fonctionnent plus (sans équivalent) : `--max-sessions`, `--priority`, `--sweep-branch-prefix`.

- Le moniteur et la vue de fusion du mode parallèle : suivez les sessions dans la vue **Sessions** ou `oh session list`.

## 3. Session libre : workflow `libre`

L'ancien lancement « par agent » devient le workflow livré **`libre`** : l'agent de ton choix (par défaut `orchestrator`) et les agents qu'il peut appeler, sans checkpoint, dans un monde fermé.

```bash
oh run libre --agent debugger -m "le test X échoue depuis hier"
```

Dans la TUI : commande `libre` (ancien nom `coder`, encore accepté), ou ouverture d'une session depuis la vue Worktrees. `free` est un alias de `libre`.

## 4. Plugins

`oh plugin` et la vue Plugins (plugins globaux opencode V1, dont RTK) sont supprimés. Les plugins se déclarent par workflow (`plugins:`) ; voir [Workflows livrés](../reference/workflows.fr.md#plugins-et-code-mode).

## 5. Métriques

`oh metrics`, le tableau de bord et la vue Métriques lisent désormais le registre des sessions d'oh (`~/.oh/oh.db`), alimenté par le démon pour chaque session v5. Les sessions lancées hors d'oh, ou avant v5, ne sont plus comptées.

## 6. Plus de déploiement dans les projets

`oh deploy` et `oh sync` sont supprimés : chaque session part d'un paquet construit au lancement, hors du projet (`oh bundle show <workflow>` pour le voir). Les deux commandes sont masquées de l'aide et n'affichent plus qu'un message de migration, avec le **code de sortie 2**. Sont aussi retirés :

- les étapes **Agents** et **Déploiement** de l'assistant d'initialisation, de `oh project add` et de `oh project configure` (le paquet contient les agents du workflow ; la sélection d'agents par projet, colonne `projects.agents`, est supprimée par les migrations v36 et v37) ;
- la section Déploiement du mode projet, les commandes `deploy`/`sync` de l'omnibar et le toast d'écart ;
- `[deploy] disable_native_agents` (le monde fermé du paquet masque toujours les agents natifs) ; `[deploy] instruction_files` reste ;
- les liens `.opencode` des worktrees (et la resynchronisation `s` de la vue Worktrees) ;
- l'ancienne vue Workflow (remplacée par le catalogue et l'éditeur de workflows).

Le serveur MCP `team` lit l'équipe de la session dans son environnement (`OH_TEAM_ID`) au lieu de `.opencode/team.json`.

## 7. Catalogue des briques

L'ancienne vue « Agents » du projet (sélection des agents déployés) devient le **Catalogue des briques**, en lecture seule : agents et skills, origine (hub ou catalogue d'équipe), coût estimé, skills chargées, dépendances (`requires`) et workflows qui les livrent. Accès : Config projet › « Briques », ou la commande `bricks` de l'omnibar. Pour changer les agents d'une session, on modifie (ou on étend) son workflow.

## 8. Nettoyer les anciens déploiements : `oh migrate deploy-cleanup`

Les fichiers écrits par `oh deploy` restent dans les projets tant qu'on ne les retire pas. Après la mise à jour, oh le propose **une fois** : message au démarrage d'une commande (`oh: anciens déploiements trouvés dans N projet(s)…`) ou écran « Nettoyage des anciens déploiements » dans la TUI (Nettoyer / Voir le diff / Plus tard ; commande `cleanup` de l'omnibar pour le rouvrir). `oh doctor` les signale tant qu'il en reste.

```bash
oh migrate deploy-cleanup --dry-run   # ce qui serait retiré, avec le diff d'opencode.json
oh migrate deploy-cleanup             # récapitulatif puis confirmation
oh migrate deploy-cleanup -p web --yes
```

Pour chaque projet enregistré, sont retirés : `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` (le dossier `.opencode` aussi s'il est vide ; les autres fichiers, comme `package.json`, restent).

Dans `opencode.json`, seules les clés **écrites par oh et inchangées depuis le dernier déploiement** sont retirées : blocs `agent.<id>` des agents du hub et des agents natifs désactivés, serveurs `mcp.<nom>` lancés par `oh mcp serve`, `context-mode` dans `plugin`, fichiers d'instructions ajoutés par oh, `compaction`, `subagent_depth`, `enabled_providers`, `model`, le bloc du fournisseur configuré, `permission.websearch`/`webfetch`. La référence est la copie d'`opencode.json` gardée dans `.deploy-state` (`config_snapshot`) : une clé que vous avez modifiée depuis est **conservée** (et signalée), vos propres clés ne sont jamais touchées, l'ordre du fichier est gardé. Sans `.deploy-state`, `opencode.json` n'est pas modifié (à vérifier à la main). Un `opencode.json` qui ne contenait que des clés d'oh est supprimé.

## 9. Anciennes surcharges du workflow (migration v38)

Les surcharges de l'ancien workflow unique (`hub.toml [workflow]`, `[workflow]` du `config.toml` d'équipe, configuration de workflow des projets) sont migrées **automatiquement** au démarrage vers des workflows `implement-spec` du team-state (équipe, projet ; un espace solo est créé pour un projet sans équipe). Rien n'est perdu : la configuration d'origine est archivée à côté (`workflows/migrated/…`, `~/.oh/migrated/` pour `hub.toml`). Une configuration de projet illisible ou au format inconnu n'est pas migrée : elle est archivée dans `~/.oh/migrated/project-<id>-workflow-config.json`, avec un message au démarrage. Détails : [Workflows d'équipe › Migration des anciennes surcharges](team-workflows.fr.md). Il n'existe pas de commande dédiée : la migration tourne au démarrage de toute commande, sauf `oh init` et `oh purge`.

## 10. Skills communautaires et `oh skill budget` déconnectés

L'ancien registre de skills communautaires (`oh skill add`, `list`, `remove`, `search`) et `oh skill budget` sont **supprimés** ([ADR-051](../architecture/adr/051-community-skills-disconnection.fr.md)) ; `oh skill check` reste.

- `~/.oh/skills` n'est plus lu : ces skills ne sont plus livrées dans les sessions, et un workflow qui en cite une dans `skills.extra` devient invalide (« skill inconnue »).
- Rien n'est supprimé : `oh doctor` liste les paquets restés dans `~/.oh/skills`. Pour garder une skill, copiez son `SKILL.md` dans le catalogue d'équipe (`catalog/skills/<catégorie>/<nom>.md` du team-state, ou de votre espace solo `oh team init --solo`), puis citez-la par sa référence (`<catégorie>/<nom>`) dans `skills.extra`. Le dossier de `~/.oh/skills` peut ensuite être supprimé.
- Budget d'une session : `oh bundle show <workflow> --budget`.

## 11. Commandes, options et codes de sortie

**Commandes retirées**

- `oh deploy` et `oh sync` : voir §6 (masquées, code de sortie 2).
- `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich` : retirés en 5.1 (§2) ; **`oh service`** (`setup`, `remove`) aussi : utilisez `oh mcp status`, `oh mcp setup`, `oh mcp disable`. Le nom répond par le remplaçant, code 2.
- `oh plugin` (§4), `oh skill add|list|remove|search|budget` (§10) et `oh upgrade opencode` (§1) n'existent plus : « unknown command », code 2.

**Options**

- **Une lettre, un sens** : `-p` est toujours `--project`, `-m` toujours `--message`, `-y` toujours `--yes`… ([tableau des options courtes](../reference/cli.fr.md#options-courtes)). Les formes courtes dépréciées en 5.0 (`oh serve -p`, `oh project configure -m`, `oh metrics -d`, `oh session attach --how` et la valeur `here`…) ont été **retirées en 5.1** : utilisez `--port`, `--model`, `--period`, `--attach` (valeur `suspend`).
- **Supprimées sans alias** (code 2) : `oh config model … -j` (utilisez `-p`) et `oh purge --include-opencode` (utilisez `--include-tool-data`).
- `--force` / `-f` ne sert plus à sauter une confirmation : c'est `--yes` / `-y`. Sur `oh purge`, `oh project remove` et `oh teams remove`, `--force` a été retiré en 5.1. `--force` subsiste là où il outrepasse une sécurité : `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` (ce dernier supprime aussi les worktrees modifiés).
- `oh status --json` : `tool` et `tool_version` remplacent `opencode_version`.
- **Projet implicite** : hors du dossier d'un projet, oh ne prend plus en silence le seul projet enregistré. Une commande qui écrit demande confirmation et refuse sans terminal (« lancez la commande depuis le dossier du projet ou utilisez -p <projet> ») ; une commande qui lit annonce le projet utilisé.
- **Aide** : `oh --help` donne une ligne par commande de premier niveau (environ 70 lignes au lieu de 573), avec une section « Avancé » (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`). `oh help --all` (ou `oh --help --all`) donne l'affichage complet d'avant ; `oh <commande> --help` est inchangé. Un script qui analysait `oh --help` doit lire `oh help --all`. `oh --all` seul affiche l'aide complète au lieu d'ouvrir la TUI.
- **`oh init`** : trois écrans en solo (accueil avec la langue et le type d'installation, fournisseur, premier projet facultatif) au lieu de huit. Les intégrations MCP ne sont plus demandées : lancez `oh mcp setup` ensuite.
- `oh teams` (équipes de cette machine) et `oh team` (équipe active) sont deux commandes distinctes.

**Codes de sortie** ([référence](../reference/cli.fr.md#codes-de-sortie))

| Code | Sens |
|---|---|
| 0 | Succès (`oh doctor` aussi quand il n'y a que des avertissements) |
| 1 | Échec : `workflow validate` ou `skill check` en erreur, `run --headless` dont une session échoue, `doctor` sur un contrôle obligatoire, erreur ordinaire |
| 2 | Mauvais usage : commande ou option inconnue (**c'était 1 en 4.2.0**), commande supprimée (`oh deploy`, `oh sync`), confirmation demandée sans terminal et sans `--yes` |
| 3 | `oh run --headless` dont une session attend une décision, sans qu'aucune n'ait échoué |

Un script qui testait `$? -eq 1` pour détecter une commande inconnue doit tester 2. `oh beads` renvoie le code de `bd`.

## 12. Confirmation sans terminal : `--yes`

Une action qui demande confirmation est **refusée** quand oh n'a pas de terminal (script, CI, entrée redirigée), sauf `--yes` (`-y`) : « confirmation requise : relancez avec --yes », code de sortie **2**. En 4.2.0, `oh workflow publish|archive|restore` répondaient « oui » seuls et plusieurs commandes refusaient en silence ou échouaient avec « open /dev/tty ».

Commandes concernées : `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete|resolve`, `migrate deploy-cleanup`, `run --recap` (et `start --recap`). `--yes` est ajouté à `run`, `purge`, `project remove`, `teams remove`, `service remove` et `worktree cleanup` ; avec `purge`, il ne propose pas la sauvegarde. Deux exceptions : `oh repair --restore` demande `--auto` sans terminal ; `oh doctor --fix` et `oh gc` refusent aussi sans terminal et sans `--yes`, mais sortent en code 1.

À faire : ajouter `--yes` aux scripts et à la CI qui lancent ces commandes. Un agent ne peut pas les lancer à votre place (§20).

## 13. Configuration

### `hub.toml` lu strictement

La 4.2.0 lisait `hub.toml` avec un lecteur faiblement typé qui convertissait les valeurs sans rien dire. La 5.0 le lit avec un décodeur TOML strict : **chaque valeur doit avoir le type de son réglage**. Un `hub.toml` écrit par oh (`oh init`, `oh config set`, la TUI) reste valide ; seuls les fichiers modifiés à la main sont concernés. Refusé désormais :

| Écrit dans `hub.toml` | Avant | Maintenant | À faire |
|---|---|---|---|
| `setup_done = "true"`, `enabled = "false"`, `"1"`, ou `setup_done = 1` (booléen entre guillemets, ou nombre pour un booléen) | lu comme un booléen | **erreur** | `true` / `false` sans guillemets |
| `keep_images = "3"`, `runner_project_id = "42"`, `budget_usd = "12.5"` (nombre entre guillemets) | lu comme un nombre | **erreur** | `3`, `42`, `12.5` sans guillemets |
| `keep_images = 2.7` (décimal pour un entier) | tronqué à `2` | **erreur** | un entier |
| `member_id = 123`, `language = true` (nombre ou booléen pour un texte) | converti en texte | **erreur** | `"123"` |
| `env_allow = "A,B"`, `instruction_files = "X.md"`, `models = "a,b"` (texte pour une liste) | coupé aux virgules | **erreur** | `["A", "B"]` |
| une clé ou une table qu'aucun réglage n'utilise (faute de frappe, clé d'un oh plus récent) | ignorée sans un mot | **avertissement**, ignorée, **perdue à la prochaine écriture** de `hub.toml` ; `oh doctor` la liste | la corriger ou la supprimer |

L'erreur arrête oh et donne le fichier, la ligne, la colonne et la clé de la première valeur fautive. `oh doctor` affiche le même message ; `oh doctor --fix` restaure la dernière sauvegarde `hub.toml.bak-*` qui se charge (trois copies gardées). `hub.toml` est écrit de façon atomique et durable. Aucune variable d'environnement ne règle `hub.toml`. Référence : [Lecture stricte de `hub.toml`](../reference/config.fr.md#lecture-stricte-de-hubtoml).

### `./hub.toml` n'est plus lu

En 4.2.0, sans `~/.oh/hub.toml`, oh lisait un `hub.toml` du dossier courant : un dépôt cloné pouvait injecter une configuration. Seul le dossier du hub compte désormais : `~/.oh`, ou `OH_HOME`. À faire : placer la configuration dans `~/.oh/hub.toml`.

### Clés renommées, supprimées, casse

- **Casse** : les clés des tables `remote.projects` et `remote.targets.binaries` gardent leur casse (elles étaient passées en minuscules). Écrivez-les comme l'identifiant du projet.
- `[opencode] default_provider` → **`[llm] default_provider`** : l'ancienne clé est encore lue, `oh config set opencode.default_provider` est accepté, et `hub.toml` est réécrit avec la clé actuelle au prochain enregistrement.
- `[execution] opencode_version` → **`[execution] tool_version`** : lue en repli seulement ; `oh config set execution.opencode_version` est **refusé** (« unknown config key »). Utilisez `oh config set execution.tool_version <version>`.
- Anciens noms de trousseau des jetons MCP (`<service>-token`, `<service>-token-<id-projet>`) → `openhub.mcp.<service>.token[.<id-projet>]` : lus en repli et copiés sous le nouveau nom au premier accès. Les anciennes clés et les anciens noms ne seront **plus lus en v6** ; `oh doctor` les liste.
- Supprimés, ignorés puis perdus à l'enregistrement : `[opencode]` (§1), `[deploy] disable_native_agents` (§6), `[workflow]` et `[workflow.overrides]` (§9), `[parallel]` (sans effet).
- **`policies.toml` est réécrit en entier** à chaque modification (`oh policies add`, vue Politiques de la TUI), sous le verrou du team-state : les commentaires et la mise en forme faits à la main sont perdus.
- **`auth_mode` de Bedrock** est respecté : `bearer` n'utilise que la clé API enregistrée ; `profile` et `env` signent en SigV4 et ignorent toute clé enregistrée. Vérifiez `provider.bedrock.auth_mode` (`oh provider setup bedrock --from-env`).

## 14. Environnement des agents : `execution.env_allow`

En 4.2.0, le serveur opencode et les shells des agents recevaient **tout** votre environnement. En 5.0, en session locale, seule une **liste blanche** passe : `PATH`, `HOME`, `USER`, `SHELL`, `TERM`, `LANG`/`LC_*`, `TZ`, `TMPDIR`, proxys et certificats, `XDG_*`, emplacements des chaînes d'outils, et les `OH_*` utiles. Ne passent donc plus **`SSH_AUTH_SOCK`**, **`GH_TOKEN`**, `GEMINI_API_KEY`, `NPM_TOKEN`, les URL de base de données… Conséquence typique : `git push` par ssh, `gh` ou un outil qui lit un jeton dans l'environnement échouent dans la session.

Pour autoriser ce dont vos agents ont besoin (par noms ou préfixes `PREFIXE_*`) :

```bash
oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN, MYCO_*"
```

- `oh doctor` liste, par nom seulement, les variables retenues, et avertit quand un nom qui ressemble à un secret est autorisé (l'agent peut le lire et l'envoyer partout où ses outils portent).
- Restent refusés quoi qu'on écrive : les identifiants que oh gère lui-même (`AWS_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `OH_PASSPHRASE`…). `ZDOTDIR`, `BASH_ENV` et `ENV` ne sont pas dans la liste de base.
- Les conteneurs et les jobs distants n'héritent jamais de votre environnement.

Référence : [L'environnement des agents](../reference/config.fr.md#lenvironnement-des-agents).

## 15. Workflows, agents et couches

- **`feature` et `cadrage` sont des alias** (plus de `feature.yaml` ni de `cadrage.yaml`) : `feature` et `cadrage` mènent à `spec` ; `feature` avec `--tickets` mène à `implement-spec` (`-m` y est refusé : pas d'entrée `request`, passez `-i instructions=…`). Les checkpoints `cp-plan` remplacent `cp-0` et `cp-final` remplace `cp-feature`. Une couche qui étend `hub:feature` ou `hub:cadrage` est **refusée** (`extends_alias`) : écrivez `extends: hub:spec` ou `extends: hub:implement-spec`. `oh doctor` liste les « Workflows retirés ».
- Autres alias lus dans les workflows : `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback` et `retours` (→ `review-feedback`), `free` (→ `libre`) ; `manual` est lu comme `manuel`. Les termes `dev`, `start`, `fast`, `rev` et `orchestrator` ne sont plus reconnus dans la palette ni dans `oh run`.
- **Une couche de projet ne peut plus ajouter d'agent ni de `skills.extra`** à un workflow hérité (erreurs `patch_agent_refused`, `patch_skills_extra_refused`, l'ajout est ignoré). La couche d'équipe garde un avertissement (`patch_agent_added`, `patch_skills_extra_added`). À faire : déclarer l'agent ou la skill dans le workflow d'équipe, puis l'étendre dans le projet.
- **Les couches ne peuvent que durcir** : `skills.deny`, préconditions des workflows `risk: publish`, `mcp`, `plugins`, `after:`, `calls`, `circuit_breaker`, `writes`, `code_mode`, `modes.allowed` ; `checkpoints.<id>.approver` (couche d'équipe seulement). Un assouplissement donne l'erreur `loosening`. Vérifiez avec `oh workflow validate --all`.
- **5 agents retirés** : `developer-refactor`, `developer-migrator`, `test-generator`, `infra`, `database` ; **4 ajoutés** : `conductor`, `merger`, `operator`, `release-manager`. Un workflow d'équipe ou de projet qui nomme un agent retiré est refusé (`agent_removed`) avec le remplaçant ; la table des remplacements est dans [MIGRATION.md](../../MIGRATION.md#agents-supprimés-consolidation-c1).
- **Comportements de session qui changent sans action de votre part** : sous-agents fermés par défaut (démon arrêté, les opérations verrouillables sont refusées ; sans plugin oh actif, un workflow qui délègue ne démarre plus : gardez le démon actif, `oh daemon status`) ; checkpoints contrôlés par oh (`verify:` lancé par oh, verrous `unlocks:` appliqués à tous les agents) ; `risk: read` vraiment en lecture seule côté Beads ; avec `catalog.lock`, une brique modifiée à la main ou commitée sans oh est ignorée (publiez avec `oh brick publish`) ; budgets levés dès que le dépassement est vu, chaque enfant d'un fan-out recevant une part du budget restant ; `spec` : la demande est facultative.

## 16. Touches de la TUI

La TUI suit un contrat de touches unique ([Référence TUI](../reference/tui.fr.md#convention-des-touches)).

- **Taper une lettre n'ouvre plus la palette** : `Ctrl+P` (ou `/` hors filtre). `Entrée` et les lettres ne sont plus pris à la liste.
- Aide : `?` ou `F1` (`F1` marche partout, formulaires compris). `d` ne ferme plus les toasts : c'est `Échap`. Dans un assistant, `Échap` demande « Quitter l'assistant ? ». Fiche de lancement : `Ctrl+S` va au récapitulatif puis lance.
- Convention : `n` crée partout, `d` supprime (toujours confirmé), `x` archive (jamais destructif), `R` reprend ou restaure, `c` compare ; décision : `y` valider, `n` refuser, `e` modifier, `s` laisser en attente, `A` tout accepter.
- Une seule vue **Nettoyage** (anciens déploiements et `oh gc`) ; commandes de palette `cleanup`, `gc`, `nettoyage`, `deploy-cleanup`. La commande `coder` s'appelle `libre`.

Les **23 changements de touches**. La colonne « 4.2.0 » dit si la vue existait déjà ; sinon le changement ne concerne que les builds de développement.

| Vue | Avant → maintenant | 4.2.0 |
|---|---|---|
| Sessions | `D` → `d` supprimer · `c` → `R` reprendre · `g` → `R` récupérer une session distante · nouveau `B` valider en lot | non |
| Récap d'une session existante | `r` → `R` reprendre · `t` → `T` transférer | non |
| Propositions wiki d'équipe | `a` → `y` accepter · `x` → `n` refuser | oui |
| Workflows | `D` → `c` comparer · `x` → `d` supprimer un brouillon (`x` n'archive que les publiés) | non |
| Historique d'un workflow | `r` → `R` restaurer | non |
| Éditeur de workflow | `x` → `d` retirer · `y` → `Y` YAML brut | non |
| Patterns | `a` → `n` créer · `v` → `y` valider | oui |
| Projets | `a` → `n` créer · `n` → `e` renommer | oui |
| Board d'équipe | `f` → `/` filtrer · `x` → `L` libérer | oui |
| Worktrees | `a` → `n` créer · `x` → `M` nettoyer les fusionnés | oui |
| Détail d'équipe | `y` → `T` découvrir le tracker | oui |
| Autres listes (modèles, policies, secrets, équipes…) | `a` → `n` créer | oui |

Rien à configurer : un message « Nouvelles touches » s'affiche une fois au premier lancement après la mise à jour, puis reste accessible depuis l'aide (`?`).

## 17. Mode équipe

- **Plus de bêta (5.1).** Le mode équipe n'a plus d'avertissement sur stderr, de contrôle « mode bêta » dans `oh doctor` ni de ligne « Bêta » en tête du board et de la configuration d'équipe. Ce que la 5.0 annonçait comme limites est corrigé : une réservation (claim) en revue (label `ready-for-review` ou statut `review`) est **gardée** à la fin de la session, de même qu'une réservation dont le checkpoint (`cp-2`) est en attente ou reporté avec du travail laissé (label `session-ended`) ; elle a un **bail** (`lease_hours`, 24 h par défaut) que le démon renouvelle tant qu'une session ouverte travaille sur le ticket, après quoi un autre membre peut la reprendre avec `oh team claim <ticket>` (confirmation commune ; la reprise n'est plus un transfert silencieux) ; ce que la fin d'une session décide passe par une file locale (`.git/oh-claims-queue.json`) appliquée dès que le dépôt d'équipe répond, donc une libération ne se perd plus hors ligne. Détails : [Bail et fin de session](team-setup.fr.md#bail-et-fin-de-session) et [Modèle de statut](team-setup.fr.md#modèle-de-statut). Le mode distant reste **expérimental**.
- **Jeton GitLab unifié.** Un seul ordre de recherche pour la synchronisation du tracker, `oh mr create`, les entrées de `review-feedback`, la vérification d'identité de `oh team init|rejoin` et `oh mcp enable --project` : `GITLAB_TOKEN`, `tracker_token_key`, jeton du projet, jeton de l'équipe, jeton MCP du hub (clé de `[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token`, puis `gitlab-token`), puis l'ancienne clé `openhub.tracker.gitlab.token`. Pour `oh mr create`, le jeton de l'équipe du projet passe avant le jeton MCP du hub.
- **Tracker désactivé.** La TUI ne synchronise plus un tracker dont `[tracker] enabled = false`. Avant, elle le synchronisait quand même.
- `oh mr create` lit le projet GitLab et le relecteur dans l'équipe du projet (repli sur l'équipe du hub).
- La colonne REVIEW de `oh board` et du Board de la TUI se remplit : un ticket Beads `in_progress` portant le label `ready-for-review` y est affiché. Le vote de review MCP fait passer un ticket approuvé par `review` puis `done`.

## 18. Mode distant et fan-out expérimentaux

- **Mode distant (GitLab CI)** : expérimental et désactivé par défaut (jamais validé sur un vrai runner). Sans `remote.experimental = true`, `oh remote setup`, `oh run --runtime remote`, l'option ☁ de la fiche de lancement, `oh session fetch` et `oh session resolve` sont refusés. Pour l'essayer, avec des projets et des jetons de test : `oh config set remote.experimental true`. La 4.2.0 n'ayant pas `oh remote`, il n'y a rien à relancer ; `oh remote setup` ne se relance qu'après la mise à jour d'un build intermédiaire de la 5.0.
- **Fan-out** : expérimental et désactivé par défaut. Sans `fanout.experimental = true`, `oh run implement-spec -i fanout=true` (ou `sweep`) est refusé ; refusé aussi en session distante ([guide](fan-out.fr.md)).

## 19. Base de données et retour arrière

La base passe de la version **27 (4.2.0) à la 49** au premier lancement (21 migrations : 28 à 46, 48 et 49). Un verrou entre processus évite les migrations concurrentes, et une copie `~/.oh/backups/oh.db.v27-<date>.bak` est écrite **avant** (trois copies gardées). `OH_SKIP_DB_BACKUP=1` saute la copie (la migration a lieu quand même : à éviter). Une base écrite par un oh plus récent est refusée (`OH_ALLOW_NEWER_SCHEMA=1` pour outrepasser). `~/.oh` est en 0700 et `oh.db` en 0600 ; les mots de passe des serveurs opencode ne sont plus dans `oh.db` (`oh export` n'en emporte plus).

**Il n'y a pas de retour arrière simple** : la base est migrée sur place. Le chemin sûr :

1. `oh daemon stop` (refusé tant que des sessions tournent, sauf `--force`).
2. `oh repair --restore ~/.oh/backups/oh.db.v27-<date>.bak` (avec `--auto` sans terminal). L'ancienne base est mise de côté.
3. Réinstallez le binaire 4.2.0 **avant** de relancer une commande : la 5.0 migrerait de nouveau la base restaurée.
4. Au besoin, restaurez aussi `hub.toml` (`hub.toml.bak-*`, ou l'archive de `oh export` avec `oh import`). Les modifications faites avec la 5.0 sont perdues.

Non testé : l'ouverture d'une base 5.0 par un binaire 4.2.0. `oh repair` (sans option) contrôle la base sans la migrer ; `--migrate` applique les migrations restantes après une copie.

## 20. Sécurité visible

- **`oh mcp trust <service>`** : une URL de service MCP **imposée par l'équipe** n'est utilisée qu'une fois approuvée (épinglée dans `mcp.<service>.trusted_url` de votre `hub.toml`) ; `https` obligatoire hors loopback. Sans approbation, le jeton n'est pas envoyé à cette URL et oh avertit (`oh doctor`, `oh mcp status`, `oh team config`, vue MCP de la TUI, touche `T`). Chaque membre lance `oh mcp trust <service>` après avoir vérifié l'URL et l'hôte affichés ; `oh mcp untrust <service>` retire l'approbation.
- **Garde global des agents** : un agent ne peut plus lancer, de son shell, `oh mcp trust|untrust|setup|enable|disable|reset|serve`, `oh config set|unset|language|websearch`, `oh config model default|family|agent|unset`, `oh secrets set|delete|get|cleanup`, `oh budget set|unset|raise`, ni ce qui publie ou supprime : `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove` (alias `rm`, `p`), `oh teams remove|archive|restore`, `--yes` compris. Les lectures (`list`, `show`, `diff`) restent permises. C'est un garde-fou, pas une barrière : lancez ces commandes vous-même.
- **`oh serve`** : jeton aléatoire par lancement (URL `?token=…`, puis cookie), 401 sans jeton, 403 si `Host` n'est pas `127.0.0.1`, `localhost` ou `[::1]`. Ouvrez l'URL affichée et ne la partagez pas.
- **Isolation** : les plugins de votre configuration personnelle d'opencode sont signalés au lancement et par `oh doctor` ; `execution.strict_isolation = true` les masque et refuse un paquet d'opencode sans empreinte épinglée (défaut : `false`).
- Sans action de votre part : le proxy d'identifiants ne relaie plus les en-têtes de l'agent tels quels, la passerelle MCP des conteneurs est fermée par défaut, les écritures de secrets, de configuration, de rapports et de wiki sont atomiques.

Limites assumées et maturité des fonctions : [SECURITY.fr.md](../../SECURITY.fr.md).

## Depuis la 5.0 vers la 5.1

La 5.1 **retire** ce que la 5.0 avait déprécié (rupture annoncée dans le [CHANGELOG](../../CHANGELOG.md)). Un nom de commande retiré ne disparaît pas sans explication : `oh` affiche le remplaçant sur la sortie d'erreur et sort avec le code **2** (un script le voit). Une option retirée donne l'erreur d'option inconnue de la ligne de commande (code 2).

**Commandes**

| Retirée en 5.1 | Remplacement |
|---|---|
| `oh start` | `oh run <workflow>` : `oh run libre --agent <agent>` (lancement libre), `oh run ticket --tickets <ids>`, `oh run onboarding`, `oh run sweep`, `oh run feature` ; `oh session attach <id>` pour reprendre une session |
| `oh audit` | `oh run audit -i type=<type>` |
| `oh review` | `oh run review -i review_mode=<mode> -i branch=<branche>` ; `oh review --publish` → `oh mr create` |
| `oh review feedback <réf>` | `oh run review-feedback -i mr=<réf>` |
| `oh debug` | `oh run debug -i issue=<description>` |
| `oh service`, `oh service setup`, `oh service remove` | `oh mcp status`, `oh mcp setup`, `oh mcp disable` (le jeton se supprime avec `oh secrets delete <clé>` : `oh mcp disable` ne touche pas au trousseau) |
| `oh takeover-brief enrich <ticket>` | `oh run brief-enrich --headless -i ticket=<ticket>`, ou la touche `e` de la vue Reprise de la TUI (qui enregistre le brief enrichi) |

**Options**

| Retirée en 5.1 | Remplacement |
|---|---|
| `oh serve -p`, `oh metrics -d`, `oh project configure -m` | `--port`, `--period`, `--model` |
| `oh session attach --how`, valeur `--attach here` | `--attach`, valeur `suspend` |
| `--force` / `-f` de `oh purge`, `oh project remove`, `oh teams remove` | `--yes` / `-y` |
| `oh repair --auto` | `--yes` (pour `--restore` sans terminal) |
| `-w`, `-l`, `-t`, `-A`, `-r` d'`oh start`, `-m`, `-b` d'`oh review`, `-t` d'`oh audit`, `-i` d'`oh debug` | partis avec leurs commandes |

`--force` reste sur `oh daemon stop`, `oh remote setup`, `oh worktree remove` et `oh worktree cleanup`, où il outrepasse une sécurité.

**À faire** : chercher ces noms dans vos scripts, alias de shell et CI (`grep -rnE 'oh (start|audit|review|debug|service)\b|takeover-brief enrich|--force|repair --auto'`), puis remplacer selon les tableaux. Détail des commandes : [Commandes retirées en 5.1](../reference/cli-sessions.fr.md#commandes-retirées-en-51).

**Déprécié en 5.1 (retiré en v6)**

| Déprécié | Remplaçant |
|---|---|
| `oh dashboard` | La vue **Métriques** de la TUI (`oh`, puis `Ctrl+P` › Métriques). La commande l'ouvre encore, avec un avertissement |
| `oh metrics` | La même vue. La commande garde son texte (`--period`) sur la sortie standard jusqu'à la v6, et avertit sur la sortie d'erreur |

Les deux sont masquées de l'aide. À faire : les remplacer dans vos habitudes ; un script qui lit `oh metrics` a jusqu'à la v6 (`oh budget report --json` donne les dépenses).

**Changé en 5.1** : `oh board`, `oh team board` et `oh dashboard` ne dessinent plus de vue plein écran à part : elles ouvrent la TUI sur le Board, le Board d'équipe et la vue Métriques, et `Esc` revient à l'accueil du mode. Sans terminal (script, CI), elles sortent avec le code **2**. `oh board` affiche le projet du dossier courant (ou `--project <nom>`, ou l'unique projet enregistré) au lieu des tickets du dossier courant ; `--watch` est accepté et sans effet (le board se rafraîchit toutes les 5 s).

## Depuis la 5.1 vers la 5.2

La 5.2 change le sens d'un réglage du mode équipe et ajoute un refus (deux ruptures annoncées dans le [CHANGELOG](../../CHANGELOG.md)). La base de données ne change pas. La 5.2.1 est une version corrective : aucune rupture, aucune migration. Sans mode équipe, rien à faire.

**1. `max_auto_plan_per_member` devient `max_in_progress_per_member`**

| En 5.1 | Depuis la 5.2 |
|---|---|
| `max_auto_plan_per_member` bornait les réservations que la synchro du tracker créait pour un membre | `max_in_progress_per_member` borne le nombre de tickets **en cours** d'un membre (5 par défaut, `0` = sans limite) |
| Les issues au-delà du plafond restaient « retenues », hors du board | La synchro met **tous** les tickets assignés sur le board, sans plafond, et ne refuse jamais rien ; un ticket déjà en cours sur le tracker arrive en cours, même au-delà de la limite |

Les tickets en revue (statut `review`, ou label `review` / `ready-for-review`) et en validation ne comptent pas ; sur un board personnalisé, ce sont les colonnes de rôle `active`. L'ancienne clé est **dépréciée** : encore lue quand la nouvelle est absente (une valeur à 0 de `hub.toml` valait « défaut », elle est ignorée), annoncée une fois par processus, signalée par [`oh doctor`](../reference/cli-infra.fr.md#oh-doctor) avec la commande à lancer, et retirée à la prochaine écriture du fichier. `oh config set|get|unset tracker.max_auto_plan_per_member` reste accepté comme alias de la nouvelle clé. Fin de la prise en charge en v6.

**À faire** : écrire la nouvelle clé, `oh config set tracker.max_in_progress_per_member <n>` pour votre machine (`hub.toml`) ou `oh team config` pour l'équipe (`config.toml`). Le champ de la TUI s'appelle maintenant « Tickets en cours par membre (max) ». Le mécanisme des issues « retenues » disparaît (avertissement de synchro, ligne « N issue(s) hors du board » du board d'équipe) ; un état de synchro de la 5.1.0 qui en contient fait refaire un balayage complet au prochain run, qui met ces issues sur le board.

**2. Au-delà de la limite, oh refuse de prendre ou de démarrer un ticket**

Un membre qui a atteint sa limite ne peut plus prendre ni démarrer un ticket de plus ; oh refuse en donnant le compte et la clé :

- `oh team claim`, aussi la reprise d'une réservation expirée et `oh team claim transfer` vers soi ;
- la touche de prise (`c`) du board d'équipe, et une carte passée en cours ou transférée à soi sur le board ;
- le lancement d'une session sur des tickets d'équipe, locale ou distante, avant que rien ne soit créé.

**À faire** : un script ou un membre qui avait plus de 5 tickets en cours est refusé au prochain ticket. Relever la limite (`oh config set tracker.max_in_progress_per_member <n>` pour soi, `oh team config` pour l'équipe, `0` pour la supprimer) ou passer des tickets en revue. Détail : [mode équipe](team-setup.fr.md) et [`[tracker]`](../reference/config.fr.md).

## Depuis la 5.2 vers la 5.3

La 5.3 change quatre comportements du mode équipe, dont deux ruptures annoncées dans le [CHANGELOG](../../CHANGELOG.md) (`unassigned_labels`, menu des cartes). La base de données ne change pas. Elle recommande opencode **2.2.5** (la plage supportée ne bouge pas : 2.0.0 à 2.99.99) et attend bd **1.3.3** : [`oh doctor`](../reference/cli-infra.fr.md#oh-doctor) signale une autre version de bd. Sans mode équipe, rien d'autre à faire.

**1. `unassigned_labels` devient un OU, et la réserve est balayée en entier**

| En 5.2 | Depuis la 5.3 |
|---|---|
| Une issue n'entrait dans la réserve que si elle portait **tous** les labels de `unassigned_labels` : avec plusieurs labels, la réserve était presque toujours vide | Une issue qui porte **l'un** des labels entre dans la réserve (une requête par label, chaque issue comptée une fois) ; Jira l'était déjà |
| Seules les issues mises à jour depuis la dernière synchro étaient lues : les vieilles issues jamais retouchées n'arrivaient jamais | La réserve est d'abord balayée en entier, en autant de passages qu'il faut (au plus `max_unassigned_issues` claims par passage, 20 par défaut), puis en incrémental ; changer `unassigned_labels` relance ce balayage |

Une réserve de 70 issues demande donc quatre passages de synchro. Un claim libéré à la main n'est pas recréé par le balayage.

**À faire** : si vous comptiez sur le ET pour restreindre la réserve, gardez un seul label ou créez un label dédié. Augmentez `max_unassigned_issues` pour aller plus vite.

**2. `'todo'` n'est pas un statut de claim**

L'assistant de découverte de la 5.2 écrivait `'todo'` comme cible de `label_status_mapping`. La synchro le refusait en silence et mettait ces issues en IN PROGRESS. La 5.3 le lit comme `planned` (statut de la colonne initiale) et le signale ; une valeur qui ne désigne aucune colonne est ignorée et la synchro le dit. L'**enregistrement** de la config d'équipe est refusé, avec la valeur en cause et les statuts valides, tant qu'une telle valeur s'y trouve.

**À faire** : lancez `oh doctor` (contrôle « Correspondances du tracker »), puis `oh doctor --fix`, qui réécrit `config.toml` en un commit dans le dépôt d'équipe. Une valeur qui ne désigne rien se corrige à la main. Au premier passage, un claim dont le seul label de statut est un label « à faire » passe en TODO.

**3. Plusieurs labels rattachés : bloqué d'abord, puis le plus avancé**

Quand une issue porte plusieurs labels rattachés à des statuts, une colonne **bloquée** (rôle `blocked`, ou `blocked` sur le board par défaut) l'emporte toujours ; sinon la colonne **la plus avancée** (par défaut DONE > VALIDATION > REVIEW > IN PROGRESS > TODO) ; à égalité, l'ordre alphabétique des labels. Avant, c'était le premier label de l'issue dans l'ordre du tracker. Une issue « Bloqué » + « READY TO DEV » est donc bloquée, et un vieux « READY TO DEV » resté à côté de « DEV DOING » ne la fait plus reculer.

**À faire** : rien, sauf si une colonne doit passer avant les autres : donnez-lui le rôle `blocked`, ou placez-la plus loin dans le board. Détail : [`[tracker]`](../reference/config.fr.md#section-tracker).

**4. Le menu `a` d'une carte suit sa colonne**

| Colonne | Actions |
|---|---|
| todo (rôle `initial`) | *Cadrer et écrire la spec* (`spec`), *Implémenter le ticket* |
| en cours (première colonne `active`) | *Implémenter le ticket*, *Donner une consigne à la session en cours* |
| review (rôle `review`) | *Review de code* (branche du claim), *Traiter les retours de review* (si le claim a une MR) |
| validation, bloquée, terminée… | détail, statut, transfert |

Sur une issue GitLab, *Implémenter le ticket* importe l'issue en ticket Beads **quand vous confirmez le lancement** (si le projet a Beads), puis lance `ticket` ; sinon `quick`. Le claim de la carte passe en cours au lancement, dans la limite de travail en cours, et il est réglé à la fin de la session comme ceux des tickets. Un board dont les colonnes n'ont aucun rôle garde l'ancien menu.

Le nouveau rôle de colonne `review` est facultatif : sans lui, la colonne d'identifiant `review` en tient lieu. Ses claims ne comptent pas dans la limite de travail en cours et n'expirent pas, quel que soit l'ID de la colonne.

**À faire** : pour lancer un autre workflow sur une carte, utilisez la barre de commandes ou l'accueil. Détail : [le menu `a` d'une carte](../reference/tui.fr.md#le-menu-a-dune-carte) et [`[board]`](../reference/config.fr.md#section-board).

## Depuis la 5.3 vers la 5.3.2

La 5.3.2 reprend la 5.3.1, jamais publiée. Elle corrige surtout la création et l'adhésion à une équipe, avec deux ruptures légères. La base de données ne change pas. Sans mode équipe, rien à faire.

**1. Accès au dépôt d'équipe avant le clone**

| En 5.3.0 | Depuis la 5.3.2 |
|---|---|
| Le clone partait juste après la saisie de l'URL, avec les seuls accès de git sur le poste ; un dépôt privé en HTTPS échouait sans qu'on puisse saisir de token | Dépôt, sonde d'accès, étape **Accès** si git est refusé (token HTTPS, ou rappel `ssh-add` en SSH), identité, puis clone. L'écran d'erreur propose Réessayer, Configurer l'accès, Modifier l'URL, Passer l'équipe |
| Le token saisi n'était confié qu'à l'assistant d'identifiants du poste (`git credential approve`), et rien n'était gardé sans lui | Le token est vérifié, gardé dans le trousseau de `oh` sous `openhub.git.<hôte>/<dépôt>.token` (`.user` : `oauth2` par défaut) et donné à git par un `GIT_ASKPASS` à usage unique pour le clone, le pull, le push et le démon |

**À faire** : rien si l'accès marche déjà (SSH, assistant d'identifiants du poste). Sinon, « Rejoindre une équipe » dans la palette refait l'étape Accès, ou `oh secrets set --global openhub.git.<hôte>/<dépôt>.token`. Détail : [mode équipe](team-setup.fr.md) et [SECURITY](../../SECURITY.fr.md).

**2. Un seul emplacement pour le clone**

« Créer une équipe » et `oh team init` clonaient dans `~/.oh/team-state/`, « Rejoindre » dans `~/.oh/team-states/<hôte>/<dépôt>/`. Tous les parcours utilisent maintenant le second. Une équipe déjà configurée garde son `state_path`, ancien chemin compris : rien n'est déplacé. Un dossier qui contient le clone d'un autre dépôt est refusé au lieu d'être réutilisé.

**À faire** : rien, sauf si oh signale qu'un dossier contient le clone d'un autre dépôt : supprimez-le, ou choisissez un autre dépôt.

**3. Identité du membre**

Le formulaire est le même dans tous les parcours : ID, nom affiché, handle GitLab (facultatif, unique dans l'équipe), rôle (`lead` pour qui crée l'équipe, `dev` pour qui la rejoint), mode `semi-auto`. Un identifiant ou un handle déjà porté par un collègue est **refusé** ; avant, « Créer » écrasait sa fiche et `oh team init` l'ignorait. Si c'est bien votre fiche, « C'est moi » la rattache sans rien écraser (seuls les champs vides sont complétés). « Rejoindre une équipe » dans la palette permet maintenant de créer sa fiche, et `n` dans la vue Équipes lance cet assistant. Un dépôt d'équipe vide (projet GitLab neuf) est accepté par « Créer ».

**À faire** : rien, sauf pour un script qui réutilisait l'identifiant d'un autre membre : il échoue désormais.

## Depuis la 5.3.2 vers la 5.3.3

La 5.3.3 corrige des défauts du mode équipe qui pouvaient perdre ou abîmer des données. La base de données ne change pas. Sans mode équipe, rien à faire.

**1. La synchro du tracker suit les changements de l'issue, pas son état à chaque passage**

| En 5.3.2 | Depuis la 5.3.3 |
|---|---|
| Une issue ouverte sans label de statut donnait `in_progress` à chaque passage, chez n'importe quel membre : une réservation passée en review, validation ou done dans le hub y revenait | Le statut n'est réécrit que si l'état de l'issue, son statut ou ses labels rattachés à une colonne ont changé depuis le passage précédent (mémorisés sur la réservation : `tracker_state`, `tracker_labels`). Une issue fermée passe en terminal, une issue rouverte repasse en travail |
| Les labels posés par le hub (`session-ended`, `review`…) étaient retirés à chaque synchro | Seuls les labels retirés côté tracker sont retirés côté hub |
| Rien n'était repoussé vers le tracker | Avec `push_labels`, le label de la colonne du statut du hub est posé sur l'issue et les autres labels de statut retirés |

Au premier passage après la mise à jour, une réservation existante n'est pas déplacée : oh mémorise l'issue. Une synchro dont la poussée vers le team-state échoue est maintenant une erreur, et `sync-state.json` n'est pas écrit.

**À faire** : rien.

**2. Token git et token d'API GitLab séparés**

Le token git saisi à l'étape Accès (`read_repository`, `write_repository`) n'est plus copié vers `openhub.team.<équipe>.gitlab.token`. « Rejoindre » propose de nouveau l'étape « Token GitLab » (on peut la passer), et une identité que GitLab ne confirme pas (`GET /user` refusé ou injoignable) devient un avertissement au lieu de bloquer. Une copie faite par une 5.3.x reste en place.

**À faire** : pour le tracker et la vérification d'identité, un token d'API `api` (ou `read_api` + `read_user`).

**3. `config.toml` de l'équipe et `oh team config`**

Une modification de `config.toml` (TUI, assistants, `oh team config`) est appliquée sur la dernière version du fichier : si un collègue a changé le même réglage entre-temps, rien n'est écrasé et oh propose de recharger. Un `config.toml` illisible n'est plus affiché comme vide : l'enregistrement est refusé. `oh team config` garde la valeur actuelle sur Entrée, propose « hériter » pour retirer un réglage local, et refuse sans terminal. `oh team config status` n'écrit plus rien : il ne supprime plus les collègues de `members.toml`.

**À faire** : un script qui pilotait `oh team config` par l'entrée standard échoue ; réglez les clés dans `config.toml` (équipe) ou avec `oh config set` (machine).

**4. Chemins et identifiants d'équipe**

Une équipe sans `state_path` prend maintenant le chemin de son clone à la lecture de `hub.toml` (l'ancien clone du bon dépôt s'il existe) : plus aucune commande git n'est lancée dans le dossier courant. Les nouveaux clones vont dans `~/.oh/team-states/<hôte>/<groupe>/<dépôt>`. Deux dépôts de même nom (`acme/team-state`, `beta/team-state`) coexistent : le second reçoit l'identifiant `beta-team-state`. Un identifiant choisi avec `--id` n'est plus réécrit, et une équipe n'en remplace jamais une autre. Aucun clone n'est déplacé ni cloné de nouveau.

**À faire** : rien ; si un script supposait que l'identifiant d'une équipe est le nom du dépôt, lire `oh teams list`.
