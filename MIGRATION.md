# Passer de oh 4.x à oh 5.0

oh 5.0 est une version majeure : opencode V2 devient obligatoire, chaque session part d'un workflow (`oh run`), oh ne déploie plus rien dans les projets, `hub.toml` est lu strictement, l'environnement des agents est filtré et la base de données change de schéma. Cette partie s'adresse à un utilisateur de **oh 4.2.0** : les prérequis, les étapes dans l'ordre, puis chaque rupture avec l'action à faire. La présentation par sujet, avec les explications, est dans le guide [Migrer vers oh v5](docs/guides/migration-v5.fr.md) ([English](docs/guides/migration-v5.en.md)). La migration depuis `oc` (oh v2) est tout en bas : [historique](#historique--migration-depuis-oc-oh-v2).

**À retenir avant de commencer**

- Installez **opencode V2** (2.0.0 au minimum, **2.0.25** recommandée) : sans lui, `oh run` et `oh start` refusent de démarrer.
- Faites `oh export` avant la mise à jour, puis lancez `oh doctor`.
- **`SSH_AUTH_SOCK` et `GH_TOKEN` ne passent plus aux agents** : autorisez-les avec `execution.env_allow` si vos agents en ont besoin.
- `hub.toml` est lu **strictement** (`"true"` n'est plus un booléen) et `./hub.toml` n'est plus lu.
- Dans un script ou en CI : une confirmation sans terminal est **refusée** (code 2) sauf `--yes` ; une commande ou une option inconnue sort en **2** (et non plus 1) ; `oh deploy` et `oh sync` n'existent plus.
- La base de données passe de la version 27 à la 49 au premier lancement, avec une copie automatique ; le retour en arrière se fait en restaurant cette copie.

## Prérequis

| Prérequis | Valeur | Vérification |
|---|---|---|
| oh de départ | 4.2.0 (les autres 4.x se migrent de la même façon) | `oh version` |
| opencode | **V2** : minimum **2.0.0**, maximum 2.99.99, **recommandée 2.0.25** (la version testée par la CI) | `opencode --version`, puis `oh doctor` (ligne « opencode ») |
| Plateforme | macOS ou Linux. Windows : sessions locales seulement (pas de conteneur, pas de mode distant, pas d'arrière-plan pour le démon, `oh upgrade oh` non pris en charge) | |

opencode V1 n'est plus pris en charge, sans repli. Avec une version hors de la plage (trop ancienne, ou au-delà de 2.99.99), `oh run`, `oh start` et les alias refusent de démarrer et affichent :

```text
opencode 1.18.29 n'est pas pris en charge : oh v5 demande la version 2.0.0 ou plus récente.
Installez une version prise en charge de opencode, puis lancez oh doctor. Guide : docs/guides/migration-v5.fr.md
```

Sans opencode du tout : `opencode est introuvable : oh v5 en demande une version prise en charge.` (suivi du même conseil). `oh doctor` affiche la ligne **opencode** en ✗ avec le même texte.

Une version de la plage mais **non épinglée** dans oh (autre que 2.0.20 et 2.0.25) fonctionne, mais `oh doctor` l'avertit : les images de conteneur téléchargeraient opencode sans empreinte vérifiée, et `execution.strict_isolation = true` la refuse. Voir [SECURITY.fr.md](SECURITY.fr.md#chaîne-dapprovisionnement).

## Les étapes, dans l'ordre

1. **Sauvegardez.** `oh export -o ~/oh-backup-4.x.tar.gz` emporte la base, `hub.toml` et les secrets chiffrés. Gardez aussi le binaire 4.2.0 (voir [Retour arrière](#retour-arrière)).
2. **Installez opencode V2** avec son propre outil (`brew install anomalyco/tap/opencode`, ou <https://opencode.ai>). oh n'installe plus ni ne met plus à jour opencode (`oh upgrade opencode` n'existe plus). Vérifiez : `opencode --version`.
3. **Mettez oh à jour** : `brew upgrade openhub`, ou `oh upgrade oh` (`oh upgrade oh --check` pour seulement vérifier).
4. **Lancez `oh doctor` en premier**, avant toute autre commande qui écrit (`oh config set`, la TUI…). Le premier lancement fait, tout seul :
   - la migration de la base de la version 27 à la 49, après une copie dans `~/.oh/backups/` ([Base de données](#base-de-données)) ;
   - la migration des anciennes surcharges de workflow (`[workflow]` de `hub.toml`, du `config.toml` d'équipe ou d'un projet) vers des workflows d'équipe ou de projet ; les originaux sont archivés dans `~/.oh/migrated/`. **Il n'y a pas de commande à lancer** : elle tourne au démarrage de toute commande, **sauf `oh init` et `oh purge`**. Tant qu'elle n'a pas tourné, ne modifiez pas `hub.toml` à la main : une section `[workflow]` écrasée avant la migration serait perdue ;
   - la proposition, une seule fois, de retirer les anciens déploiements (étape 6).
5. **Corrigez ce que `oh doctor` signale** (code de sortie 1 seulement pour un contrôle obligatoire ; les avertissements ⚠ ne comptent pas) :
   - un `hub.toml` refusé : le message donne le fichier, la ligne, la colonne et la clé ([tableau](#hubtoml-lu-strictement)) ; corrigez-le à la main, ou `oh doctor --fix` restaure la dernière sauvegarde `hub.toml.bak-*` qui se charge (vous perdez les modifications faites depuis) ;
   - les clés inconnues ou héritées (liste dans `oh doctor`), les anciens déploiements, `~/.oh/skills`, les variables d'environnement retenues, la version d'opencode.

   `oh doctor --fix` applique les réparations après un récapitulatif et une confirmation ; sans terminal, il demande `--yes`.
6. **Retirez les anciens déploiements** : `oh migrate deploy-cleanup --dry-run` (ce qui serait retiré), puis `oh migrate deploy-cleanup`. Voir [le guide](docs/guides/migration-v5.fr.md#8-nettoyer-les-anciens-déploiements--oh-migrate-deploy-cleanup).
7. **Autorisez les variables d'environnement dont vos agents ont besoin** (git par ssh, `gh`…) : `oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN"` ([détail](#environnement-des-agents)).
8. **Si vous êtes dans une équipe**, approuvez chaque URL de service imposée par l'équipe : `oh mcp trust <service>` ([détail](#sécurité-visible)), et lisez [Mode équipe (bêta)](#mode-équipe-bêta).
9. **Mettez à jour vos scripts et votre CI** : `oh deploy`/`oh sync`, `oh start` → `oh run <workflow>`, `--yes` partout où une confirmation est demandée, test du code de sortie 1, `-j`, `--include-opencode`, analyse de `oh --help`, `oh status --json` ([Commandes et options](#commandes-et-options)).
10. **Vérifiez vos workflows d'équipe et de projet** : `oh workflow validate --all`. Les agents retirés, les couches qui étendent `feature` ou `cadrage`, les ajouts d'agent ou de skills par une couche de projet sont refusés ([Workflows et couches](#workflows-et-couches)).
11. **Touches de la TUI** : au premier lancement, oh affiche un message « Nouvelles touches » (rappel : `?` ou `F1`). Voir [Touches de la TUI](#touches-de-la-tui).

## Ruptures par thème

Chaque ligne donne ce qui change pour un utilisateur de la 4.2.0 et ce qu'il faut faire. Quand aucune action n'est nécessaire, c'est écrit.

### opencode et installation

| Changement | À faire |
|---|---|
| **opencode V2 obligatoire** (2.0.0 à 2.99.99, recommandée 2.0.25) ; en 4.2.0 la plage était 1.15.0 à 1.99.99. Plus de repli sur V1 | Installer opencode V2 (étape 2) |
| `oh upgrade opencode`, l'installation gérée dans `~/.oh/bin` et les clés `[opencode] version`, `channel`, `auto_update`, `install_dir` de `hub.toml` sont **supprimés** (`oh upgrade` n'a plus que `oh`) | Mettre opencode à jour avec son outil ; ces clés sont ignorées, puis perdues au prochain enregistrement de `hub.toml` |
| `oh plugin` (`install`, `list`, `remove`, `status`), la vue Plugins et les plugins globaux d'opencode V1 (dont RTK) sont **supprimés** : « unknown command », code 2 | Déclarer les plugins dans le workflow (`plugins:`), voir [Workflows livrés](docs/reference/workflows.fr.md#plugins-et-code-mode) |
| Empreinte des paquets Linux d'opencode **épinglée** pour 2.0.20 et 2.0.25 ; autre version : avertissement (`oh doctor`), refus avec `execution.strict_isolation` (images de conteneur) | Préférer une version épinglée, ou accepter l'avertissement |
| `oh upgrade oh` ne rétrograde plus sans demande explicite ; la signature Cosign est vérifiée si `cosign` est installé (`OH_REQUIRE_SIGNATURE=1` pour l'exiger) ; `install.sh` refuse sans outil de somme de contrôle | Rien (information) |

### Commandes et options

**Commandes retirées ou dépréciées**

| Avant (4.2.0) | Maintenant | À faire |
|---|---|---|
| `oh deploy`, `oh sync` | **Masquées**, n'affichent que le message de migration, **code de sortie 2**. oh n'écrit plus rien dans `.opencode` : chaque session part d'un paquet construit hors du projet (`oh bundle show <workflow>`). Les étapes Agents et Déploiement de `oh init`, `oh project add` et `oh project configure` sont retirées (colonne `projects.agents` supprimée, migrations 36 et 37) | Retirer ces commandes des scripts ; `oh migrate deploy-cleanup` ([guide §8](docs/guides/migration-v5.fr.md#8-nettoyer-les-anciens-déploiements--oh-migrate-deploy-cleanup)) |
| `oh start` | **Alias déprécié** (avertissement qui donne la commande) → `oh run feature`, lui-même alias de `spec` (ou de `implement-spec` avec `--tickets`) | `oh run spec`, `oh run implement-spec --tickets …` |
| `oh start --dev [-t <id>]`, `--parallel --tickets a,b` | → `oh run ticket --tickets <id>` | idem |
| `oh start --onboard [--refresh]`, `--sweep <objectif>` | → `oh run onboarding [-i refresh=true]`, `oh run sweep -i goal=<objectif>` | idem |
| `oh start --agent <id>` | → `oh run libre --agent <id>` | idem |
| `oh start --resume <id>` | → `oh session attach <id> --attach suspend` | idem |
| `oh audit`, `oh review`, `oh debug` | Alias dépréciés → `oh run audit`, `oh run review`, `oh run debug` | idem |
| `oh review feedback <mr>`, `oh review --publish` | Dépréciés → `oh run review-feedback -i mr=<mr>` ; `oh mr create` | idem |
| `oh takeover-brief enrich` | Déprécié → `oh run brief-enrich --headless` | idem |
| `oh service` (`setup`, `remove`) | **Dépréciée** (non annoncée par l'ancien CHANGELOG) → `oh mcp status`, `oh mcp setup`, `oh mcp disable` | idem |
| `oh skill add\|list\|remove\|search` et `oh skill budget` | **Supprimées** (`oh skill check` reste) ; `~/.oh/skills` n'est plus lu | Copier les skills voulues dans le catalogue d'équipe ; budget : `oh bundle show <workflow> --budget` ([guide §10](docs/guides/migration-v5.fr.md#10-skills-communautaires-et-oh-skill-budget-déconnectés)) |

Ces alias marchent pendant toute la 5.x. Restent refusés ou sans effet : `oh start --parallel` **sans** `--tickets` ; `--agent` combiné à `--dev`, `--onboard`, `--parallel` ou `--sweep` ; `--max-sessions`, `--priority`, `--sweep-branch-prefix` (sans effet) ; `oh start --yes` (déprécié).

**Options**

| Changement | À faire |
|---|---|
| **Options courtes : une lettre, un sens** (`-p` = `--project`, `-m` = `--message`, `-y` = `--yes`…, [tableau](docs/reference/cli.fr.md#options-courtes)). Anciennes formes acceptées **une version** avec avertissement : `oh serve -p` (→ `--port`), `oh project configure -m` (→ `--model`), `oh metrics -d` (→ `--period`), `oh review -m` / `-b` (→ `--mode` / `--branch`), `oh debug -i` (→ `--issue`), `oh audit -t` (→ `--type`), `oh start -w` / `-l` (→ `--worktree` / `--label`), `oh session attach --how` (→ `--attach`, `here` devient `suspend`) | Utiliser les formes longues ou les nouvelles lettres |
| **Supprimées sans alias** (code 2) : `oh config model … -j` (→ `-p`) et `oh purge --include-opencode` (→ `--include-tool-data`) | Remplacer dans les scripts |
| `--force` / `-f` ne sert plus à sauter une confirmation : c'est `--yes` / `-y`. Sur `oh purge`, `oh project remove`, `oh teams remove`, `oh service remove`, `--force` reste accepté **une version** (même effet, avertissement, absent de l'aide). `--force` subsiste là où il outrepasse une sécurité : `oh daemon stop`, `oh remote setup`, `oh worktree remove`, `oh worktree cleanup` | `--force` → `--yes` |
| `oh status --json` : `tool` et `tool_version` remplacent `opencode_version` | Adapter les scripts qui lisent ce champ |
| **Projet implicite** : hors du dossier d'un projet, oh ne prend plus en silence le seul projet enregistré. Une commande qui écrit demande confirmation, et **refuse sans terminal** (« lancez la commande depuis le dossier du projet ou utilisez -p ») ; une commande qui lit annonce le projet utilisé | Se placer dans le projet, ou passer `-p <projet>` |
| `oh --help` n'affiche plus qu'**une ligne par commande** (≈ 70 lignes au lieu de 573) ; `oh help --all` (ou `oh --help --all`) donne l'affichage complet d'avant ; `oh <commande> --help` est inchangé. `oh --all` seul affiche l'aide complète (il n'ouvre plus la TUI) | Un script qui analyse `oh --help` doit lire `oh help --all` |
| `oh init` : **3 écrans** en solo (accueil avec langue et type d'installation, fournisseur, premier projet facultatif) au lieu de 8. Les intégrations MCP ne sont plus demandées | `oh mcp setup` après `oh init` |
| `oh teams` (équipes de cette machine) et `oh team` (équipe active) sont distinguées dans l'aide | Rien |

### Codes de sortie

| Code | Sens |
|---|---|
| 0 | Succès. `oh doctor` : aussi quand il n'y a que des avertissements |
| 1 | Échec : `workflow validate` ou `skill check` en erreur, `run --headless` dont une session échoue, `doctor` sur un contrôle obligatoire, erreur ordinaire |
| 2 | **Mauvais usage** : commande ou option inconnue (**c'était 1 en 4.2.0**), commande supprimée (`oh deploy`, `oh sync`), **confirmation demandée sans terminal et sans `--yes`** |
| 3 | `oh run --headless` dont une session attend une décision, sans qu'aucune n'ait échoué |

**À faire** : un script qui testait `$? -eq 1` pour détecter « commande inconnue » doit tester 2. `oh beads` renvoie le code de `bd`. Référence : [Codes de sortie](docs/reference/cli.fr.md#codes-de-sortie).

### Confirmation sans terminal

**Rupture.** Une action qui demande confirmation est **refusée** quand oh n'a pas de terminal (script, CI, entrée redirigée), sauf `--yes` (`-y`) : message « confirmation requise : relancez avec --yes », **code de sortie 2**. En 4.2.0, certaines commandes répondaient « oui » seules, d'autres refusaient en silence ou échouaient avec « open /dev/tty ».

Commandes concernées : `workflow publish|archive|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`, `session delete|resolve`, `migrate deploy-cleanup`, `run --recap` (et `start --recap`). `--yes` est ajouté à `run`, `purge`, `project remove`, `teams remove`, `service remove`, `worktree cleanup`. Avec `purge`, `--yes` ne propose pas la sauvegarde. Deux exceptions : `oh repair --restore` demande `--auto` sans terminal, et `oh doctor --fix` et `oh gc` refusent aussi sans terminal et sans `--yes`, mais sortent en code 1 (et non 2).

**À faire** : ajouter `--yes` aux scripts et à la CI qui lancent ces commandes. Un agent ne peut pas les lancer à votre place (voir [Sécurité visible](#sécurité-visible)).

### Configuration

#### `hub.toml` lu strictement

La 4.2.0 lisait `hub.toml` avec un lecteur faiblement typé qui convertissait les valeurs sans rien dire. La 5.0 le lit avec un décodeur TOML strict : **chaque valeur doit avoir le type de son réglage**. Un `hub.toml` écrit par oh (`oh init`, `oh config set`, la TUI) reste valide ; seuls les fichiers modifiés à la main sont concernés. Refusé désormais :

| Écrit dans `hub.toml` | Avant | Maintenant | À faire |
|---|---|---|---|
| `setup_done = "true"`, `enabled = "false"`, `"1"`, ou `setup_done = 1` (booléen entre guillemets, ou nombre pour un booléen) | lu comme un booléen | **erreur** | `true` / `false` sans guillemets |
| `keep_images = "3"`, `runner_project_id = "42"`, `budget_usd = "12.5"` (nombre entre guillemets) | lu comme un nombre | **erreur** | `3`, `42`, `12.5` sans guillemets |
| `keep_images = 2.7` (décimal pour un entier) | tronqué à `2` | **erreur** | un entier |
| `member_id = 123`, `language = true` (nombre ou booléen pour un texte) | converti en texte | **erreur** | `"123"` |
| `env_allow = "A,B"`, `instruction_files = "X.md"`, `models = "a,b"` (texte pour une liste) | coupé aux virgules | **erreur** | `["A", "B"]` |
| une clé ou une table qu'aucun réglage n'utilise (faute de frappe, clé d'un oh plus récent) | ignorée sans un mot | **avertissement**, ignorée ; `oh doctor` la liste ; **perdue à la prochaine écriture** de `hub.toml` | la corriger ou la supprimer |

L'erreur arrête oh et donne le fichier, la ligne, la colonne et la clé de la première valeur fautive. `oh doctor` affiche le même message ; `oh doctor --fix` restaure la dernière sauvegarde qui se charge. `hub.toml` est écrit de façon atomique et durable (`hub.toml.bak-*`, 3 copies gardées). Aucune variable d'environnement ne règle `hub.toml` (inchangé). Référence : [Lecture stricte de `hub.toml`](docs/reference/config.fr.md#lecture-stricte-de-hubtoml).

#### Autres changements de configuration

| Changement | À faire |
|---|---|
| **`./hub.toml` n'est plus lu.** En 4.2.0, sans `~/.oh/hub.toml`, oh lisait un `hub.toml` du dossier courant (un dépôt cloné pouvait injecter une configuration). Seul le dossier du hub compte : `~/.oh`, ou `OH_HOME` | Placer la configuration dans `~/.oh/hub.toml` (ou `$OH_HOME/hub.toml`) |
| **Casse des clés** : les clés des tables `remote.projects` et `remote.targets.binaries` **gardent leur casse** (elles étaient passées en minuscules). Les autres noms restent comparés sans tenir compte de la casse | Écrire ces clés comme l'identifiant du projet |
| `[opencode] default_provider` → **`[llm] default_provider`**. L'ancienne clé est encore lue (et `oh config set opencode.default_provider` accepté), réécrite sous le nouveau nom au prochain enregistrement | Utiliser `oh config set llm.default_provider <fournisseur>` |
| `[execution] opencode_version` → **`[execution] tool_version`**. L'ancienne clé est lue en repli seulement ; `oh config set execution.opencode_version` est **refusé** (« unknown config key ») | `oh config set execution.tool_version <version>` |
| Anciens noms de trousseau des jetons MCP `<service>-token` et `<service>-token-<id-projet>` → `openhub.mcp.<service>.token[.<id-projet>]` : lus en repli et copiés sous le nouveau nom au premier accès (les anciens ne sont pas supprimés) | Rien. Ces anciennes clés et noms ne seront **plus lus en v6** ; `oh doctor` les liste |
| Sections supprimées, ignorées puis perdues à l'enregistrement : `[opencode]` (sauf `default_provider`), `[deploy] disable_native_agents` (le monde fermé masque toujours les agents natifs ; `[deploy] instruction_files` reste), `[workflow]` / `[workflow.overrides]`, `[parallel]` (sans effet) | Rien (le contenu de `[workflow]` est migré, étape 4) |
| **`policies.toml` est réécrit en entier** à chaque modification (`oh policies add`, vue Politiques de la TUI), par un encodeur TOML, sous le verrou du team-state : les commentaires et la mise en forme faits à la main sont **perdus** | Ne pas compter sur les commentaires du fichier |
| **`auth_mode` de Bedrock respecté** : `bearer` n'utilise que la clé API enregistrée ; `profile` et `env` signent en SigV4 et **ignorent toute clé enregistrée** ; une clé refusée par AWS déclenche une alerte | Vérifier `provider.bedrock.auth_mode` ; `oh provider setup bedrock --from-env` |
| Réglages nouveaux, à connaître : `remote.experimental` et `fanout.experimental` (défaut `false`), `execution.strict_isolation`, `execution.env_allow`, `[limits]`, `[gc]`, `[cli] compact`. Page « Où se règle quoi » | [Référence de configuration](docs/reference/config.fr.md) |

### Environnement des agents

**Rupture de sécurité, la plus visible.** En 4.2.0, le serveur opencode et les shells des agents recevaient **tout** votre environnement. En 5.0, en session locale, seule une **liste blanche** passe : `PATH`, `HOME`, `USER`, `SHELL`, `TERM`, `LANG`/`LC_*`, `TZ`, `TMPDIR`, proxys et certificats, `XDG_*`, emplacements des chaînes d'outils, et les `OH_*` utiles. Ne passent donc plus : **`SSH_AUTH_SOCK`**, **`GH_TOKEN`**, `GEMINI_API_KEY`, `NPM_TOKEN`, les URL de base de données, etc. Conséquence typique : `git push` par ssh, `gh`, ou un outil qui lit un jeton dans l'environnement échouent dans la session.

**À faire** : nommer ce que vos agents doivent recevoir, par noms ou préfixes `PREFIXE_*` :

```bash
oh config set execution.env_allow "SSH_AUTH_SOCK, GH_TOKEN, MYCO_*"
```

- `oh doctor` liste, par nom seulement, les variables retenues, et **avertit** quand un nom ressemblant à un secret est autorisé (`GH_TOKEN` par exemple : l'agent peut le lire et l'envoyer partout où ses outils portent).
- Restent refusés quoi qu'on écrive : les identifiants que oh gère lui-même (`AWS_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GITLAB_TOKEN`, `GITHUB_TOKEN`, `OH_PASSPHRASE`…). `ZDOTDIR`, `BASH_ENV` et `ENV` ne sont pas dans la liste de base.
- Les conteneurs et les jobs distants n'héritent jamais de votre environnement.

Référence : [L'environnement des agents](docs/reference/config.fr.md#lenvironnement-des-agents).

### Workflows et couches

| Changement | À faire |
|---|---|
| **`feature` et `cadrage` sont des alias** (plus de `feature.yaml` ni de `cadrage.yaml`) : `feature` et `cadrage` → `spec` ; `feature` avec `--tickets` → `implement-spec` (`-m` y est refusé : pas d'entrée `request`, passer `-i instructions=…`). Checkpoints : `cp-plan` remplace `cp-0`, `cp-final` remplace `cp-feature`. Une couche qui étend `hub:feature` ou `hub:cadrage` est **refusée** (`extends_alias`) ; `oh doctor` liste les « Workflows retirés » | Réécrire ces couches avec `extends: hub:spec` ou `hub:implement-spec` |
| **Une couche de projet ne peut plus ajouter d'agent ni de `skills.extra`** à un workflow hérité (erreurs `patch_agent_refused`, `patch_skills_extra_refused`, l'ajout est ignoré). La couche d'équipe garde un avertissement (`patch_agent_added`, `patch_skills_extra_added`) | Déclarer l'agent ou la skill dans le workflow d'équipe, puis l'étendre dans le projet |
| **Les couches ne peuvent que durcir** : `skills.deny`, préconditions des workflows `risk: publish`, `mcp`, `plugins`, `after:`, `calls`, `circuit_breaker`, `writes`, `code_mode`, `modes.allowed` ; `checkpoints.<id>.approver` (couche d'équipe seulement). Un assouplissement donne l'erreur `loosening` | `oh workflow validate --all`, corriger les couches signalées |
| **5 agents retirés, 4 ajoutés** (`conductor`, `merger`, `operator`, `release-manager`) : table ci-dessous. Un workflow qui nomme un agent retiré est refusé (`agent_removed`) avec le remplaçant | Retirer ces membres de vos workflows |
| Les anciennes **surcharges du workflow** (`[workflow]`) sont migrées vers des workflows `implement-spec` du team-state (migration v38) ; originaux archivés | Vérifier `~/.oh/migrated/` et `oh workflow list` ([guide §9](docs/guides/migration-v5.fr.md#9-anciennes-surcharges-du-workflow-migration-v38)) |
| Alias lus dans les YAML : `secu` (→ `security-fix`), `perf` (→ `benchmark`), `dbg` (→ `debug`), `feedback` et `retours` (→ `review-feedback`), `free` (→ `libre`). `manual` est lu comme `manuel` partout où l'on saisit un mode. Les termes `dev`, `start`, `fast`, `rev`, `orchestrator` ne sont **plus reconnus** dans la palette ni dans `oh run` | Utiliser l'identifiant du workflow ou ces alias. Dans la TUI, la commande `coder` s'appelle `libre` (ancien nom en alias) |
| La session libre « par agent » est le workflow **`libre`** : pas de checkpoint, donc **pas de verrou de commit** (voulu) ; plus d'auto-délégation déduite de la permission `task`, profondeur 3 → 2 | `oh run libre --agent <id>` |
| **Sous-agents fermés par défaut** : le plugin oh relaie les opérations verrouillables ; **démon arrêté, ces opérations sont refusées** ; sans plugin oh actif, un workflow qui délègue à des sous-agents ne démarre plus | Garder le démon actif (`oh daemon status`) |
| **Checkpoints contrôlés par oh** : `verify:` lancé par oh avant déverrouillage (dans un conteneur si le runtime est `container`), preuves, limites de tours de correction, verrous `unlocks: [commit, push, close, create, update, apply, tag, mr]` appliqués à tous les agents | Rien (comportement) |
| **Passerelle Beads** : `risk: read` est vraiment en lecture seule (`bd comments add`, `bd ready --claim` refusés) ; `--actor="<personne> via oh/<agent>"` imposé ; un chemin absolu vers `bd` est refusé aux agents | Rien |
| **Briques d'équipe** (`oh brick`) : avec `catalog.lock`, une brique modifiée à la main ou commitée sans oh est ignorée ; liens symboliques ignorés | Publier avec `oh brick publish` (`--adopt` pour un catalogue jamais scellé) |
| **Budgets** : la décision `$` est levée dès que le dépassement est vu ; chaque enfant d'un fan-out reçoit une **part** du budget restant ; `limits.models` vérifiée au lancement | Rien |
| **Métriques** : `oh metrics`, le tableau de bord et la vue Métriques lisent le registre des sessions d'oh ; les sessions lancées hors d'oh ou avant la 5.0 ne sont plus comptées ([guide §5](docs/guides/migration-v5.fr.md#5-métriques)) | Rien |
| `spec` : la demande (`request`) est **facultative** ; sans demande, la session la demande | Rien |

#### Agents supprimés (consolidation C1)

Ces agents ne sont plus livrés. Un workflow d'équipe ou de projet qui les nomme (`agents:`, `entry.agent`, `oh run libre --agent`) est refusé par `oh workflow validate` avec le remplaçant (diagnostic `agent_removed`).

| Agent supprimé | Remplacement |
|----------------|--------------|
| `developer-refactor` | `developer`, domaine `refactor` (skill `developer/dev-standards-refactoring`) |
| `developer-migrator` | `developer`, domaine `migration` (skill `developer/dev-standards-migration`) ; les commandes de migration de base (`alembic`, `python manage.py migrate`/`makemigrations`, `rails db:migrate`/`db:rollback`, `flask db`) sont dans les permissions du `developer` |
| `test-generator` | `developer`, domaine `tests` (skill `developer/test-generation`) — aussi dans `sweep` pour un objectif d'ajout de tests |
| `infra` | agent `operator`, workflow `infra-change`, domaine d'audit `infra` |
| `database` | domaine d'audit `database` ; pour implémenter, `developer` avec la skill `developer/dev-standards-database` |

À faire dans vos workflows : retirer ces membres (`developer` suffit, le domaine passe dans le prompt d'invocation) ; dans les tickets planifiés, la colonne « Agent prévu » s'écrit `developer (refactor)`, `developer (migration)` ou `developer (tests)`.

### Touches de la TUI

**Rupture.** La TUI suit un contrat de touches unique ([Référence TUI](docs/reference/tui.fr.md#convention-des-touches)). Ce qui change pour tous :

- **Taper une lettre n'ouvre plus la palette** : `Ctrl+P` (ou `/` hors filtre). `Entrée` et les lettres ne sont plus pris à la liste par la palette.
- Aide : `?` ou `F1` (`F1` marche partout, formulaires compris).
- `d` ne ferme plus les toasts : c'est `Échap`. Dans un assistant, `Échap` demande « Quitter l'assistant ? ». Fiche de lancement : `Ctrl+S` va au récapitulatif puis lance.
- Convention : `n` crée partout, `d` supprime (toujours confirmé), `x` archive (jamais destructif), `R` reprend ou restaure, `c` compare ; décision : `y` valider, `n` refuser, `e` modifier, `s` laisser en attente, `A` tout accepter.
- Une seule vue **Nettoyage** (anciens déploiements et `oh gc`), commandes de palette `cleanup`, `gc`, `nettoyage`, `deploy-cleanup`.

Les **23 changements de touches** (`keys.Changes()`). La colonne « 4.2.0 » dit si la vue existait déjà : sinon, le changement ne concerne que les builds de développement.

| Vue | Avant → maintenant | 4.2.0 |
|---|---|---|
| Sessions | `D` → `d` supprimer · `c` → `R` reprendre · `g` → `R` récupérer une session distante · nouveau `B` valider en lot | non |
| Récap d'une session existante | `r` → `R` reprendre · `t` → `T` transférer | non |
| Propositions wiki d'équipe | `a` → `y` accepter · `x` → `n` refuser | **oui** |
| Workflows | `D` → `c` comparer · `x` → `d` supprimer un brouillon (`x` n'archive que les publiés) | non |
| Historique d'un workflow | `r` → `R` restaurer | non |
| Éditeur de workflow | `x` → `d` retirer · `y` → `Y` YAML brut | non |
| Patterns | `a` → `n` créer · `v` → `y` valider | **oui** |
| Projets | `a` → `n` créer · `n` → `e` renommer | **oui** |
| Board d'équipe | `f` → `/` filtrer · `x` → `L` libérer | **oui** |
| Worktrees | `a` → `n` créer · `x` → `M` nettoyer les fusionnés | **oui** |
| Détail d'équipe | `y` → `T` découvrir le tracker | **oui** |
| Autres listes (modèles, policies, secrets, équipes…) | `a` → `n` créer | **oui** |

**À faire** : rien à configurer. Le message « Nouvelles touches » s'affiche une fois au premier lancement après la mise à jour, puis reste accessible depuis l'aide (`?`). Petits terminaux : mode compact sous 32 lignes (`[cli] compact = auto|always|never`) ; `NO_COLOR` est respecté.

### Mode équipe (bêta)

| Changement | À faire |
|---|---|
| **Le mode équipe est publié en bêta** : avertissement unique par processus sur stderr (commandes `oh team …`, premier claim d'un lancement), avertissement dans `oh doctor` (sans effet sur le code de sortie), ligne « Bêta » en tête du board et de la configuration d'équipe. Limites connues : la réservation (claim) est libérée à l'arrêt de la session même avec `cp-2` en attente ; **pas d'expiration** (reprise manuelle après `takeover.stale_days`, 3 par défaut, `oh team claim transfer`) ; une libération peut se perdre hors ligne ou si le démon s'arrête avant le push (`oh team release <ticket> -p <projet>`) ; `oh doctor` signale les orphelins. Correction complète prévue en 5.1 | Lire les [limites connues](docs/guides/team-setup.fr.md#limites-connues) avant de passer une équipe sur la 5.0 |
| **Jeton GitLab unifié** : un seul ordre de recherche pour la synchronisation du tracker, `oh mr create` / `oh review --publish`, `oh review feedback`, la vérification d'identité de `oh team init\|rejoin` et `oh mcp enable --project` : `GITLAB_TOKEN`, `tracker_token_key`, jeton du projet, jeton de l'équipe, jeton MCP du hub (`[mcp.gitlab] token`, puis `openhub.mcp.gitlab.token`, puis `gitlab-token`), ancienne clé `openhub.tracker.gitlab.token`. Pour `oh mr create`, le jeton de l'équipe du projet passe avant le jeton MCP du hub | Vérifier quel jeton est trouvé en premier si vous en avez plusieurs ; sinon rien |
| **La TUI ne synchronise plus un tracker désactivé** (`[tracker] enabled = false`) ; avant, elle le synchronisait quand même | Rien (réactiver avec `enabled = true` si vous comptiez sur la synchronisation de la TUI) |
| `oh mr create` lit le projet GitLab et le relecteur dans l'équipe du projet (repli sur l'équipe du hub) | Rien |
| Colonne REVIEW de `oh board` et du Board TUI : un ticket Beads `in_progress` portant le label `ready-for-review` y est affiché (elle n'était jamais remplie). Vote de review MCP : une approbation depuis `in_progress` passe par `review` puis `done` | Rien |

### Mode distant et fan-out

| Changement | À faire |
|---|---|
| **Le mode distant (GitLab CI) est expérimental et désactivé par défaut** (jamais validé sur un vrai runner). Sans `remote.experimental = true`, `oh remote setup`, `oh run --runtime remote`, l'option ☁ de la fiche de lancement, `oh session fetch` et `oh session resolve` sont refusés, et le runtime distant n'est plus choisi par défaut | `oh config set remote.experimental true`, avec des projets et des jetons de test seulement. Une 4.2.0 n'a pas `oh remote` : **rien à relancer** (`oh remote setup` ne se relance qu'après la mise à jour d'un build intermédiaire de la 5.0) |
| **Le fan-out est expérimental et désactivé par défaut** : sans `fanout.experimental = true`, `oh run implement-spec -i fanout=true` (et `sweep`) est refusé ; refusé aussi en session distante ; budget global 10 $ | `oh config set fanout.experimental true` pour l'essayer |

### Base de données

| Changement | À faire |
|---|---|
| La base passe de la version **27 (4.2.0) à la 49** au premier lancement (21 migrations, 28 à 46, 48 et 49). Un **verrou** entre processus évite les migrations concurrentes. Une copie `~/.oh/backups/oh.db.v27-<date>.bak` est écrite **avant** (3 gardées). `OH_SKIP_DB_BACKUP=1` saute la copie (la migration a lieu quand même : à éviter) | Garder la copie jusqu'à ce que la 5.0 vous convienne |
| Une base écrite par un oh **plus récent** est refusée (`OH_ALLOW_NEWER_SCHEMA=1` pour outrepasser) | Mettre oh à jour plutôt que forcer |
| `~/.oh` est en 0700 et `oh.db` en 0600. Les mots de passe des serveurs opencode ne sont plus dans `oh.db` (un fichier 0600 par groupe sous `~/.oh/run/server-passwords/`) : `oh export` n'en emporte plus | Rien |
| **Pas de retour arrière simple vers la 4.2.0** : voir [Retour arrière](#retour-arrière). `oh repair` contrôle la base sans la migrer (`--migrate` applique les migrations restantes après une copie, `--restore` remet une copie) | |

### Sécurité visible

| Changement | À faire |
|---|---|
| **`oh mcp trust <service>`** : une URL de service MCP **imposée par l'équipe** (`url_enforced`) n'est utilisée qu'une fois approuvée (épinglée dans `mcp.<service>.trusted_url` de **votre** `hub.toml`) ; `https` obligatoire hors loopback. Sans approbation, le jeton n'est **pas envoyé** à cette URL et oh avertit (`oh doctor`, `oh mcp status`, `oh team config`, vue MCP de la TUI, touche `T`). `oh mcp untrust <service>` retire l'approbation | Chaque membre lance `oh mcp trust <service>` après avoir vérifié l'URL et l'hôte affichés |
| **Garde global des agents** : un agent ne peut plus lancer, de son shell, `oh mcp trust\|untrust\|setup\|enable\|disable\|reset\|serve`, `oh config set\|unset\|language\|websearch`, `oh config model default\|family\|agent\|unset`, `oh service setup\|remove`, `oh secrets set\|delete\|get\|cleanup`, `oh budget set\|unset\|raise`, ni ce qui publie ou supprime : `oh workflow publish\|archive\|restore`, `oh brick publish`, `oh purge`, `oh project remove` (alias `rm`, `p`), `oh teams remove\|archive\|restore` — `--yes` compris. Les lectures (`list`, `show`, `diff`, `config get`, `mcp status`) restent permises. C'est un garde-fou, pas une barrière | Lancer ces commandes vous-même |
| **`oh serve`** : jeton aléatoire par lancement (URL `?token=…`, puis cookie `HttpOnly` `SameSite=Strict`) ; 401 sans jeton, 403 si `Host` n'est pas `127.0.0.1`, `localhost` ou `[::1]`. En 4.2.0 : aucune garde | Ouvrir l'URL affichée (ne pas la partager) |
| Le proxy d'identifiants ne relaie plus les en-têtes et la query de l'agent tels quels (liste blanche par fournisseur) ; un en-tête personnalisé ajouté aux options du fournisseur d'opencode est retiré | Rien (cas rare) |
| La passerelle MCP des conteneurs est fermée par défaut ; les écritures de secrets, d'index du trousseau, de rapports, de wiki, de transcripts et de configuration sont atomiques et durables ; les serveurs MCP Figma et Google Slides refusent un identifiant mal formé | Rien |
| Isolation : les plugins de votre configuration personnelle d'opencode sont signalés au lancement et par `oh doctor` ; `execution.strict_isolation = true` les masque et refuse un paquet d'opencode sans empreinte épinglée (défaut inchangé : `false`) | Activer `strict_isolation` si vous voulez la refuser |

Limites assumées, maturité des fonctions (bêta, expérimental) et ce que les garde-fous ne couvrent pas : [SECURITY.fr.md](SECURITY.fr.md).

## Retour arrière

Il n'y a pas de retour arrière simple : la base est migrée sur place. Le chemin sûr :

1. Arrêtez le démon : `oh daemon stop` (refusé tant que des sessions tournent, sauf `--force`).
2. Remettez la copie faite avant la migration : `oh repair --restore ~/.oh/backups/oh.db.v27-<date>.bak` (avec `--auto` sans terminal). L'ancienne base est mise de côté, pas supprimée.
3. Réinstallez le binaire 4.2.0 (gardé à l'étape 1) **avant** de relancer une commande : la 5.0 migrerait de nouveau la base restaurée.
4. Au besoin, restaurez aussi `hub.toml` (`hub.toml.bak-*` du dossier du hub, ou l'archive de `oh export` avec `oh import`). Les modifications faites avec la 5.0 (sessions, projets, configuration) sont perdues.

Non testé : l'ouverture d'une base 5.0 (version 49) par un binaire 4.2.0. Ne tentez pas de lire la base actuelle avec la 4.2.0.

## Ce qui ne demande aucune action

Les mécanismes suivants s'appliquent seuls : migration de la base, migration des surcharges `[workflow]`, `[team]` → `[[teams]]` (si elle reste), copie des anciennes clés de trousseau MCP, avertissements sur les anciennes clés de `hub.toml`, proposition de nettoyage des anciens déploiements, message « Nouvelles touches ».

---

# Historique : migration depuis `oc` (oh v2)

> **Historique — ne pas suivre pour oh 5.** Ce guide date de la v2.0.0 (passage de la CLI bash `oc` à la CLI Go `oh`). Les commandes citées en dessous **n'existent plus en v5** : `oh deploy`, `oh sync`, `oh plugin`, `oh upgrade opencode`, la clé `[opencode] version`, `oh start --dev` et les autres alias de lancement (dépréciés, remplacés par `oh run`), `oh service`, `oh skill add`… Les chiffres (taille du binaire, nombre de tests, de clés i18n) sont ceux de la v2.0.0. Pour oh 5, utilisez la partie « Passer de oh 4.x à oh 5.0 » ci-dessus.

Guide de migration de la CLI bash (`oc`) vers la CLI Go (`oh`) v2.0.0.

---

## Pourquoi migrer

| | `oc` (bash) | `oh` (Go) |
|---|---|---|
| Runtime | bash + jq + node + bun + sqlite3 | Binaire Go unique (5.5 MB) |
| Dependencies | 6+ outils externes | Zero (git + opencode seuls requis) |
| Securite | Secrets en clair dans api-keys.local.md | Keychain OS + fallback AES-256-GCM |
| Deploy | Script sequentiel sans rollback | Transactionnel avec snapshot + rollback |
| MCP Servers | TypeScript (npx/bun) | Go natif (stdio, no build step) |
| TUI | Bash ANSI brut | BubbleTea (interactif, souris, scroll) |
| Tests | 0 | 213 tests, 24 packages |
| i18n | Partiel | 481 cles fr/en, parité verifiee |
| Shell completion | Aucune | Dynamique (bash, zsh, fish, powershell) |

---

## Installation de `oh`

```bash
brew install datichb/tap/openhub
```

Verifier :

```bash
oh version
```

---

## Migrer vos projets

### Premier setup

```bash
oh init
```

Le wizard interactif remplace `oc install` + `oc init`. Il :
1. Choisit la langue (fr/en)
2. Telecharge opencode si absent
3. Enregistre votre premier projet
4. Configure les services MCP
5. Initialise le tracker Beads (si disponible)
6. Deploie agents/skills/config dans le projet

### Projets supplementaires

```bash
oh project add --name mon-projet --path ~/workspace/mon-projet
oh deploy -p mon-projet
```

---

## Equivalence des commandes

| `oc` (bash) | `oh` (Go) | Notes |
|---|---|---|
| `oc install` | `oh init` | Wizard complet |
| `oc init PROJECT PATH` | `oh project add -n PROJECT -d PATH` | |
| `oc start PROJECT` | `oh start -p PROJECT` | Detection auto si dans le dossier |
| `oc start --dev` | `oh start --dev` | Picker interactif epics/tickets |
| `oc start --onboard` | `oh start --onboard` | Cree le wiki docs/wiki/ |
| `oc start --parallel` | `oh start --dev` (multi-tickets) | Orchestrator gere le parallele |
| `oc start --resume` | `oh start --resume ID` | |
| `oc start --worktree BRANCH` | `oh start --worktree BRANCH` | |
| `oc quick PROJECT "prompt"` | `oh start` | Quick launch is now the default |
| `oc deploy opencode PROJECT` | `oh deploy -p PROJECT` | |
| `oc deploy --check` | `oh deploy --check` | |
| `oc deploy --diff` | `oh deploy --diff` | |
| `oc sync` | `oh sync --all` | |
| `oc sync --dry-run` | `oh sync --dry-run` | |
| `oc status` | `oh status` | |
| `oc list` | `oh project list` | |
| `oc remove PROJECT` | `oh project remove PROJECT` | |
| `oc project rename` | `oh project rename` | |
| `oc project move` | `oh project move` | |
| `oc project configure` | `oh project configure` | |
| `oc config set` | `oh config set KEY VALUE` | |
| `oc config get` | `oh config get KEY` | |
| `oc config list` | `oh config list` | Support `--json` |
| `oc config unset` | `oh config unset KEY` | |
| `oc config language` | `oh config language [fr\|en]` | |
| `oc config websearch` | `oh config websearch [enable\|disable\|status]` | |
| `oc audit --type TYPE` | `oh audit --type TYPE` | |
| `oc review` | `oh review` | |
| `oc debug` | `oh debug` | Support `--issue` |
| `oc conventions` | *Supprimé* | Conventions gérées via skills |
| `oc doctor` | `oh doctor` | |
| `oc version` | `oh version` | |
| `oc metrics --period` | `oh metrics --period` | 7d/30d/all |
| `oc dashboard` | `oh dashboard` | TUI interactif |
| `oc optimize` | `oh optimize` | |
| `oc yield` | `oh yield` | |
| `oc worktree list` | `oh worktree list` | Support `--json` |
| `oc worktree create` | `oh worktree add` | |
| `oc worktree remove` | `oh worktree remove` | |
| `oc worktree cleanup` | `oh worktree cleanup` | |
| `oc plugin install rtk` | `oh plugin install rtk` | |
| `oc plugin remove rtk` | `oh plugin remove rtk` | |
| `oc plugin status` | `oh plugin status` | |
| `oc service setup` | `oh service setup` | Wizard interactif |
| `oc service status` | `oh service` | RunE par defaut |
| `oc service remove` | `oh service remove` | |
| `oc beads *` | `oh beads *` | Proxy transparent vers bd |
| `oc board` | `oh board` | TUI + `--watch` |
| `ocp` | `oh start --provider NAME` | Integre dans start |
| `oc upgrade opencode` | `oh upgrade opencode` | |
| `oc update` | `oh upgrade opencode` | bd se gere seul |

---

## Configuration

### hub.json -> hub.toml

L'ancien `config/hub.json` est remplace par `~/.oh/hub.toml` :

```toml
# ~/.oh/hub.toml
[cli]
language = "fr"

[opencode]
version = ""
default_provider = "bedrock"

[websearch]
enabled = true

[mcp.figma]
enabled = true

[mcp.gitlab]
enabled = true

[worktree]
auto_cleanup = true
base_branch = "main"
```

Gerer via `oh config set/get/list/unset`.

### api-keys.local.md -> Keychain

Les tokens ne sont plus stockes en fichier texte. Ils sont dans le keychain OS :

```bash
oh service setup    # wizard interactif, stocke dans le keychain
```

Fallback : si le keychain OS n'est pas disponible, `oh` utilise un fichier chiffre
(`~/.oh/secrets.enc`, AES-256-GCM + Argon2id).

### providers/*.json -> hub.toml

Les templates de providers sont remplaces par une simple cle :

```bash
oh config set llm.default_provider bedrock
```

Le provider peut aussi etre specifie par commande :

```bash
oh start --provider anthropic
```

---

## Variables d'environnement

| Ancien (`oc`) | Nouveau (`oh`) | Usage |
|---|---|---|
| `OC_NON_INTERACTIVE` | `OH_NON_INTERACTIVE` | Desactiver les prompts interactifs |
| `OC_LOCK_PROJECTS` | `OH_LOCK_PROJECTS` | Verrouiller la liste des projets |
| `OH_PASSPHRASE` | `OH_PASSPHRASE` | Passphrase pour le fallback chiffre |

---

## Breaking changes

### Commandes supprimees

| Commande | Raison |
|----------|--------|
| `oc uninstall` | `brew uninstall openhub` |
| `oc upgrade` (hub via git pull) | `brew upgrade openhub` |
| `oc update` (adapter + bd + skills) | `oh upgrade opencode` pour opencode ; bd se gere seul |
| `oc agent create/edit` | Gestion manuelle des fichiers .md |
| `oc agent select/mode/validate/discover` | Report post-release |
| `oc skills install/remove/update/search/info/add/sync` | Gestion manuelle des fichiers skill |
| `oc config init-providers` | Remplace par `oh config set llm.default_provider` |
| `ocp` (binary separee) | Integre dans `oh start --provider` |

### Flags renommes

| Ancien | Nouveau | Commande |
|--------|---------|----------|
| `--project/-p PROJECT` | `--project/-p PROJECT` | `oh start`, `oh deploy`, etc. |
| `--dev --label` | `--dev --label` (meme) | `oh start` |
| N/A (nouveau) | `--onboard` | `oh start` |
| N/A (nouveau) | `--refresh` | `oh start` |
| N/A (nouveau) | `--json` | 7 commandes list |

> Note : `-p` est uniformément réservé à `--project` sur toutes les commandes.
> `--prompt` utilise `-m`, `--path` utilise `-d`.

---

## Desinstallation de `oc`

```bash
# Supprimer le hub bash
rm -rf ~/.openhub

# Retirer l'alias du rc file
# Editez ~/.zshrc ou ~/.bashrc et supprimez la ligne:
# alias oc="~/.openhub/oc.sh"

# Recharger le shell
source ~/.zshrc
```

---

## FAQ

### Mes projets existants sont-ils perdus ?

Non. `oh init` vous propose d'enregistrer vos projets. Les agents deployes dans
vos projets (`.opencode/agents/`) restent en place et sont compatibles.

### Puis-je utiliser `oc` et `oh` en parallele ?

Oui, temporairement. Les deux CLI lisent les memes fichiers agents/skills dans
les projets. Cependant la configuration (hub.json vs hub.toml) et les secrets
(api-keys vs keychain) sont separes.

### Comment migrer mes secrets ?

```bash
oh service setup    # re-configurez chaque service interactivement
```

Les tokens seront stockes dans le keychain OS de maniere securisee.

### Le binaire `oh` est-il compatible avec toutes les versions d'opencode ?

`oh doctor` verifie la compatibilite. En cas d'incompatibilite, un warning est
affiche au `oh start` avec la commande de mise a jour.

---

## Migration single-team → multi-team

A partir de la version qui introduit le multi-team (ADR-029), le format de
configuration equipe dans `hub.toml` evolue :

### Avant (single-team)

```toml
[team]
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
state_path = "~/.oh/team-state"
member_id = "alice"
```

### Apres (multi-team)

```toml
[[teams]]
id = "team-state"
enabled = true
state_repo = "git@gitlab.com:acme/team-state.git"
state_path = "~/.oh/team-states/team-state"
member_id = "alice"
```

### Migration automatique

La migration est **automatique** au premier lancement :

1. `oh` detecte la section `[team]` legacy
2. Cree un backup `hub.toml.<timestamp>.bak`
3. Convertit en entree `[[teams]]` avec un ID derive du repo URL
4. Met a jour les projets : `Mode: "inherit"` → `TeamID = "<id-derive>"`
5. Supprime la section `[team]` legacy

### Aucune action requise

- Les utilisateurs mono-equipe ne voient aucune difference
- Les projets continuent de fonctionner normalement
- Le backup permet de revenir en arriere si necessaire

### Nouvelles commandes

```bash
oh teams list           # voir les equipes configurees
oh teams add --repo <url> --member-id <id>   # ajouter une equipe
oh teams remove <id>    # retirer une equipe
```

### Projets et equipes

Les projets declarent leur equipe via `TeamID` (au lieu de `ProjectTeamConfig.Mode`) :

```bash
oh project configure    # inclut maintenant le choix d'equipe
```
