> [Read in English](migration-v5.en.md)

# Migrer vers oh v5

oh v5 lance chaque session à partir d'un **workflow** (`oh run <workflow>`), dans un **paquet de session** construit hors du projet, sur un serveur **opencode V2**. Ce guide liste ce qui change pour un utilisateur de la v4 (4.2.0). Les mêmes ruptures, classées par thème avec l'action à faire, sont dans [MIGRATION.md](../../MIGRATION.md).

## Mise à jour pas à pas

1. **Sauvegardez** : `oh export -o ~/oh-backup-4.x.tar.gz` (base, `hub.toml`, secrets chiffrés), et gardez le binaire 4.2.0 (§19).
2. **Installez opencode V2** (§1), puis **mettez oh à jour** : `brew upgrade openhub` ou `oh upgrade oh`.
3. **Lancez `oh doctor` en premier.** Le premier lancement migre la base (27 → 49, avec copie, §19) et les anciennes surcharges `[workflow]` (§9) ; il n'y a pas de commande de migration à lancer : elle tourne au démarrage de toute commande **sauf `oh init` et `oh purge`**. Ne modifiez pas `hub.toml` à la main avant ce premier lancement.
4. **Corrigez ce que `oh doctor` signale** : `hub.toml` refusé (§13), clés héritées, anciens déploiements (§8), `~/.oh/skills` (§10), version d'opencode. `oh doctor --fix` répare après un récapitulatif et une confirmation (`--yes` sans terminal).
5. **Retirez les anciens déploiements** : `oh migrate deploy-cleanup` (§8).
6. **Autorisez les variables d'environnement** dont vos agents ont besoin : `oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN"` (§14).
7. **En équipe** : `oh mcp trust <service>` pour chaque URL imposée (§20), et lisez le mode équipe en bêta (§17).
8. **Mettez à jour scripts et CI** : `oh deploy`/`oh sync`, `oh start` → `oh run`, `--yes` (§12), code de sortie 2 (§11).
9. **Vérifiez vos workflows** : `oh workflow validate --all` (§15).

## 1. opencode V2 obligatoire

opencode V1 n'est plus pris en charge, sans repli. oh v5 demande une version d'opencode comprise entre **2.0.0** et **2.99.99** ; la version **recommandée** est **2.0.25** (celle de la CI). Hors de cette plage, `oh run`, `oh start` et ses alias refusent de démarrer et l'expliquent :

```text
opencode 1.18.29 n'est pas pris en charge : oh v5 demande la version 2.0.0 ou plus récente.
Installez une version prise en charge de opencode, puis lancez oh doctor. Guide : docs/guides/migration-v5.fr.md
```

Sans opencode : « opencode est introuvable : oh v5 en demande une version prise en charge ».

1. Installez opencode V2 avec son propre outil (`brew install anomalyco/tap/opencode`, ou https://opencode.ai).
2. Vérifiez avec `oh doctor` : la ligne **opencode** doit être verte (✓).

L'empreinte des paquets Linux d'opencode est épinglée dans oh pour les versions **2.0.20** et **2.0.25**. Une autre version de la plage fonctionne, mais `oh doctor` l'avertit : les images de conteneur la téléchargeraient sans empreinte vérifiée, et `execution.strict_isolation = true` la refuse ([SECURITY.fr.md](../../SECURITY.fr.md#chaîne-dapprovisionnement)).

`oh upgrade opencode`, l'installation gérée dans `~/.oh/bin` et les clés `[opencode] version`, `channel`, `auto_update`, `install_dir` de `hub.toml` sont supprimées (ignorées si elles restent). `[llm] default_provider` reste.

## 2. Anciennes commandes : alias de `oh run`

Elles marchent encore pendant v5.x, avec un avertissement qui donne la commande à utiliser :

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
| `oh review feedback <mr>` | `oh run review-feedback -i mr=<mr>` (dépréciée : `oh review feedback` lit encore les discussions GitLab, les prévisualise puis lance ce workflow) ; `oh review --publish` : `oh mr create` |
| `oh takeover-brief enrich` | `oh run brief-enrich --headless` |
| `oh service` (`setup`, `remove`) | `oh mcp status`, `oh mcp setup`, `oh mcp disable` |

Ne fonctionnent plus :

- `oh start --parallel` **sans** `--tickets` (refusé) ; `--max-sessions`, `--priority`, `--sweep-branch-prefix` sont sans effet.
- `oh start --agent` combiné à `--dev`, `--onboard`, `--parallel` ou `--sweep` (refusé).
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

**Commandes retirées ou dépréciées**

- `oh deploy` et `oh sync` : voir §6 (masquées, code de sortie 2).
- `oh start`, `oh audit`, `oh review`, `oh debug`, `oh review feedback`, `oh takeover-brief enrich` : alias dépréciés de `oh run` (§2). **`oh service`** (`setup`, `remove`) l'est aussi : utilisez `oh mcp status`, `oh mcp setup`, `oh mcp disable`.
- `oh plugin` (§4), `oh skill add|list|remove|search|budget` (§10) et `oh upgrade opencode` (§1) n'existent plus : « unknown command », code 2.

**Options**

- **Une lettre, un sens** : `-p` est toujours `--project`, `-m` toujours `--message`, `-y` toujours `--yes`… ([tableau des options courtes](../reference/cli.fr.md#options-courtes)). Les anciennes formes restent acceptées **une version**, avec un avertissement : `oh serve -p` (→ `--port`), `oh project configure -m` (→ `--model`), `oh metrics -d` (→ `--period`), `oh review -m` / `-b` (→ `--mode` / `--branch`), `oh debug -i` (→ `--issue`), `oh audit -t` (→ `--type`), `oh start -w` / `-l` (→ `--worktree` / `--label`), `oh session attach --how` (→ `--attach` ; la valeur `here` devient `suspend`).
- **Supprimées sans alias** (code 2) : `oh config model … -j` (utilisez `-p`) et `oh purge --include-opencode` (utilisez `--include-tool-data`).
- `--force` / `-f` ne sert plus à sauter une confirmation : c'est `--yes` / `-y`. Sur `oh purge`, `oh project remove`, `oh teams remove` et `oh service remove`, `--force` reste accepté une version (même effet, avertissement, absent de l'aide). `--force` subsiste là où il outrepasse une sécurité : `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` (ce dernier supprime aussi les worktrees modifiés).
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

## 17. Mode équipe (bêta)

- **Bêta.** Le mode équipe est publié avec un avertissement unique par processus sur stderr (commandes `oh team …`, premier claim d'un lancement), un avertissement dans `oh doctor` (sans effet sur le code de sortie) et une ligne « Bêta » en tête du board et de la configuration d'équipe. Limites connues : la réservation (claim) est libérée à l'arrêt de la session même avec `cp-2` en attente ; **pas d'expiration** (reprise manuelle après `takeover.stale_days`, 3 par défaut, `oh team claim transfer`) ; une libération peut se perdre hors ligne ou si le démon s'arrête avant le push (`oh team release <ticket> -p <projet>`) ; `oh doctor` signale les orphelins. Détails : [limites connues](team-setup.fr.md#limites-connues). Correction complète prévue en 5.1.
- **Jeton GitLab unifié.** Un seul ordre de recherche pour la synchronisation du tracker, `oh mr create` et `oh review --publish`, `oh review feedback`, la vérification d'identité de `oh team init|rejoin` et `oh mcp enable --project` : `GITLAB_TOKEN`, `tracker_token_key`, jeton du projet, jeton de l'équipe, jeton MCP du hub (clé de `[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token`, puis `gitlab-token`), puis l'ancienne clé `openhub.tracker.gitlab.token`. Pour `oh mr create`, le jeton de l'équipe du projet passe avant le jeton MCP du hub.
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
- **Garde global des agents** : un agent ne peut plus lancer, de son shell, `oh mcp trust|untrust|setup|enable|disable|reset|serve`, `oh config set|unset|language|websearch`, `oh config model default|family|agent|unset`, `oh service setup|remove`, `oh secrets set|delete|get|cleanup`, `oh budget set|unset|raise`, ni ce qui publie ou supprime : `oh workflow publish|archive|restore`, `oh brick publish`, `oh purge`, `oh project remove` (alias `rm`, `p`), `oh teams remove|archive|restore`, `--yes` compris. Les lectures (`list`, `show`, `diff`) restent permises. C'est un garde-fou, pas une barrière : lancez ces commandes vous-même.
- **`oh serve`** : jeton aléatoire par lancement (URL `?token=…`, puis cookie), 401 sans jeton, 403 si `Host` n'est pas `127.0.0.1`, `localhost` ou `[::1]`. Ouvrez l'URL affichée et ne la partagez pas.
- **Isolation** : les plugins de votre configuration personnelle d'opencode sont signalés au lancement et par `oh doctor` ; `execution.strict_isolation = true` les masque et refuse un paquet d'opencode sans empreinte épinglée (défaut : `false`).
- Sans action de votre part : le proxy d'identifiants ne relaie plus les en-têtes de l'agent tels quels, la passerelle MCP des conteneurs est fermée par défaut, les écritures de secrets, de configuration, de rapports et de wiki sont atomiques.

Limites assumées et maturité des fonctions : [SECURITY.fr.md](../../SECURITY.fr.md).
