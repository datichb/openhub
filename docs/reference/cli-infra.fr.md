> [Read in English](cli-infra.en.md)

# Référence CLI — Infrastructure

## Infrastructure

### oh init

Initialise oh pour la première fois. Assistant interactif, sans flag.

```
oh init
```

Configure : langue de l'interface, fournisseur IA et identifiants, premier projet (facultatif), équipe à rejoindre ou à créer (facultatif) et intégrations MCP (Figma, GitLab, Google Slides, facultatif). Sans équipe, les workflows du projet vivent dans un espace solo. La version d'opencode se vérifie avec `oh doctor`.

**Exemple :**

```bash
oh init
```

---

### oh doctor

Vérifie l'état du système et les dépendances. Sort avec le code 1 si un contrôle obligatoire échoue (✗) ; les avertissements (⚠) ne comptent pas, et les contrôles optionnels (moteur de conteneurs arrêté ou absent, `bd` absent, passerelles vues d'un conteneur) en font partie, sauf si `execution.runtime = container` rend le conteneur obligatoire. La vue Doctor de la TUI exécute exactement les mêmes contrôles.

`oh doctor` fonctionne même quand `hub.toml` ne se charge pas ou que la base ne s'ouvre pas (démarrage dégradé) : le contrôle en échec le dit (erreur TOML avec ligne et colonne, base illisible), les autres contrôles s'exécutent. La base est ouverte **sans être migrée** : version du schéma et version connue d'oh, migrations en attente (⚠), dernière migration en échec, schéma plus récent que cette version d'oh, contrôle rapide d'intégrité, taille du WAL.

```
oh doctor [--fix [--yes]] [--offline]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--offline` | | bool | Aucun appel réseau : ni dernière version publiée, ni validation de la clé du provider (aussi `OH_DOCTOR_OFFLINE=1`) |
| `--fix` | | bool | Répare d'abord ce qui peut l'être, après un résumé et une confirmation : un **`hub.toml` qui ne se charge plus** (remplacé par la dernière sauvegarde valide `hub.toml.bak-<date>`, le fichier cassé gardé en `hub.toml.broken-<date>`), **Beads zéro impact** (contenu écrit par `bd init` dans les projets enregistrés, commité compris ; le commit de `bd init` lui-même est annulé une fois listé et confirmé dans un terminal, ou avec `--yes`, après un `git fetch`, s'il n'est pas poussé ni le premier du dépôt) |
| `--yes` | `-y` | bool | Avec `--fix` : appliquer sans confirmation (obligatoire sans terminal) |

Contrôles : OS et architecture, git, `bd` (optionnel), version d'oh (mise à jour disponible : avertissement ; une pré-version comme `5.0.0-rc.1` ou un build de développement `git describe` plus récent que la dernière version publiée est à jour), `hub.toml`, identifiants du provider (cascade d'un lancement depuis le dossier courant : clé du projet, de l'équipe, du hub, puis profil AWS pour Bedrock, selon `auth_mode` ; clé, portée et date d'enregistrement ; une clé API Bedrock est validée auprès d'AWS, ✗ si elle est refusée ; une `AWS_BEARER_TOKEN_BEDROCK` du shell inutilisée ou différente de la clé enregistrée est signalée), base de données, clés API (trousseau), Beads « zéro impact » (hooks, `.gitignore`), puis les contrôles v5 : opencode V2 (version minimale ; V1 refusé avec le [guide de migration](../guides/migration-v5.fr.md)), démon `ohd`, git, ouverture des terminaux, moteur de conteneurs et image, passerelles, restrictions et mémoire, sécurité (socket, capacité, jetons), environnement des agents (⚠ entrées de `execution.env_allow` nommées comme des secrets ; les noms, jamais les valeurs, des variables de votre environnement retenues avant les agents), shell des sessions (le `bd` d'oh en premier, aussi dans un shell interactif), restes des anciens déploiements, `AGENTS.md` hors du paquet de session (⚠ : `AGENTS.md` de sous-dossiers des projets, que l'outil de lecture d'opencode ajoute au prompt quand un agent lit un fichier de ces dossiers, et `~/.config/opencode/AGENTS.md`, ajouté aux sessions locales sans `[execution] strict_isolation` ; celui de la racine du projet n'est pas lu), intégrité des workflows du team-state, cibles distantes, réservations d'équipe orphelines (ticket fermé, ou toutes les sessions du ticket terminées ; avec la commande `oh team release` à lancer), URL MCP imposées par l'équipe que vous n'avez pas approuvées (⚠, avec la commande [`oh mcp trust`](cli-mcp.fr.md#oh-mcp-trust) à lancer), serveurs opencode orphelins (lancés par oh ou ses tests, que plus personne ne possède ; `--fix` les arrête après confirmation).

**Exemple :**

```bash
oh doctor
```

---

### oh status

Affiche l'état du hub et du projet courant.

```
oh status [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh status
oh status --json
```

---

### oh migrate deploy-cleanup

Retire des projets enregistrés ce qu'avait laissé l'ancien `oh deploy` : `.opencode/agents`, `.opencode/skills`, `.opencode/.deploy-state`, `.opencode/context-manifest.json`, `.opencode/team.json` et, dans `opencode.json`, uniquement les clés écrites par oh et inchangées depuis le dernier déploiement (référence : `config_snapshot` de `.deploy-state`). Vos clés, et les clés d'oh que vous avez modifiées, sont gardées. Sans `.deploy-state`, `opencode.json` n'est pas touché. Récapitulatif (et diff) puis confirmation. oh le propose une fois au démarrage (la TUI a son écran de nettoyage, omnibar `cleanup`).

```
oh migrate deploy-cleanup [-p <projet>] [--dry-run] [--diff] [--yes] [--json]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Un seul projet (défaut : tous les projets actifs) |
| `--dry-run` | | bool | Afficher ce qui serait retiré, avec le diff, sans rien changer |
| `--diff` | | bool | Afficher le diff d'`opencode.json` |
| `--yes` | `-y` | bool | Appliquer sans confirmation (obligatoire sans terminal) |
| `--json` | | bool | Plan au format JSON (rien n'est changé) |

Voir le [guide de migration v5](../guides/migration-v5.fr.md#8-nettoyer-les-anciens-déploiements--oh-migrate-deploy-cleanup) et [`oh deploy` / `oh sync`](cli-deploy.fr.md).

---

### oh upgrade oh

Met à jour le binaire `oh` en place (remplacement atomique), pour les installations par `install.sh` ou téléchargement direct. opencode s'installe et se met à jour avec son propre outil (`oh upgrade opencode` n'existe plus).

```
oh upgrade oh [version] [--check]
```

| Argument / flag | Type | Description |
|-----------------|------|-------------|
| `version` | argument | Version cible (défaut : la dernière publiée). Sans version, oh n'installe jamais une version plus ancienne que la sienne ; une version plus ancienne demandée explicitement est installée avec un avertissement de rétrogradation |
| `--check` | bool | Vérifier seulement si une mise à jour est disponible |

Le téléchargement ne suit une redirection que vers HTTPS sur GitHub et son CDN de releases ; la version reçue doit être celle qui est demandée, et plus récente que celle qui tourne (sauf rétrogradation demandée explicitement, voir ci-dessus). Voir [SECURITY.fr.md](../../SECURITY.fr.md#mise-à-jour-automatique).

**Exemple :**

```bash
oh upgrade oh
oh upgrade oh 5.0.1
oh upgrade oh --check
```

> **Utilisateurs Homebrew :** utilisez `brew upgrade openhub` à la place.

---

### oh export

Crée une archive de sauvegarde des données du hub.

```
oh export [--output <chemin>]
```

Archive `.tar.gz` contenant `oh.db`, `hub.toml` et `secrets.enc` (chiffré), avec un fichier de somme de contrôle SHA-256.

La base est archivée à partir d'une copie cohérente (`VACUUM INTO`, WAL inclus), et non depuis `oh.db` lui-même : les écritures pas encore reportées de `oh.db-wal` dans `oh.db` figurent dans l'archive, et une base en cours d'utilisation n'est pas perturbée. Si la copie échoue (base corrompue), le fichier est archivé tel quel, avec un avertissement. `oh import` restaure `oh.db` et supprime les fichiers `oh.db-wal` / `oh.db-shm` laissés par la base qu'il remplace.

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--output` | `-o` | string | Chemin de sortie (défaut : `./oh-backup-<date>.tar.gz`) |

**Exemple :**

```bash
oh export
oh export --output ~/sauvegardes/oh-backup.tar.gz
```

---

### oh import

Restaure depuis une archive de sauvegarde (`oh export`). Vérifie la somme de contrôle SHA-256 avant d'écrire.

```
oh import <fichier> [--overwrite | --merge]
```

| Flag | Type | Description |
|------|------|-------------|
| `--overwrite` | bool | Écraser les données existantes sans confirmation |
| `--merge` | bool | Fusionner avec les données existantes (projets uniquement) : ajoute les projets de l'archive dont ni l'identifiant, ni le nom, ni le chemin n'existent déjà ; les projets existants, `hub.toml` et les secrets restent inchangés. Incompatible avec `--overwrite` |

**Exemple :**

```bash
oh import oh-backup-2026-10-06.tar.gz
oh import ~/sauvegardes/oh-backup.tar.gz --overwrite
oh import ~/sauvegardes/oh-backup.tar.gz --merge
```

---

### oh gc

Nettoie ce qu'oh laisse derrière lui, après un résumé (tailles par catégorie) et une confirmation : paquets de session qu'aucune session ne sert plus, données des groupes de serveurs arrêtés (verrou du groupe pris, serveur arrêté ; ses jetons du proxy révoqués), dossiers des sessions **arrêtées**, brouillons de session compris (`scratch/`, [schéma](workflow-schema.fr.md#scratch-brouillons-de-session) ; `results/` gardé jusqu'à `history_days`), worktrees **créés par oh** au lancement (propres, fichiers non suivis compris, non verrouillés, sans commit absent de toute autre branche, inutilisés ; leur branche supprimée seulement si elle est fusionnée), restes des fan-outs mergés ou dont la session parente est arrêtée (worktrees et branches `oh/fanout/…`, comme [`oh fanout clean`](cli-sessions.fr.md#oh-fanout) sans `--discard`), dossiers temporaires `.build-*`/`.fetch-*` de plus d'une heure, serveurs orphelins, historique ancien de la base (par lots, puis `PRAGMA optimize`). Les sessions terminées ou en échec, qui restent reprenables, ne sont jamais nettoyées ; les worktrees des sessions antérieures à la v5.0 non plus. Retentions : [`[gc]`](config.fr.md).

```
oh gc [--dry-run] [--yes] [--json] [--only <catégories>] [--older-than <durée>]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--dry-run` | | bool | Afficher ce qui serait supprimé, sans rien supprimer |
| `--yes` | `-y` | bool | Supprimer sans confirmation (obligatoire sans terminal) |
| `--json` | | bool | Plan (ou résultat) au format JSON |
| `--only` | | liste | `bundles`, `servers`, `sessions`, `worktrees`, `fanout`, `temp`, `orphans`, `history` |
| `--older-than` | | durée | Remplace `session_days` pour cette fois (`30d`, `12h`) |

Le démon fait le même nettoyage toutes les `[gc] every_hours` (24 h) sans se garder éveillé pour autant. Son **premier passage est une simulation** : `oh doctor` affiche alors « N récupérables : lancez oh gc » ; après un premier `oh gc`, le démon nettoie tout seul. Un seul nettoyage à la fois (`~/.oh/run/gc.lock`).

**Exemple :**

```bash
oh gc --dry-run
oh gc --only bundles,servers --yes
oh gc --older-than 7d
```

---

### oh repair

Vérifie la base SQLite `~/.oh/oh.db` **sans la migrer** et sans avoir besoin d'une configuration ou d'une base qui fonctionnent : intégrité (`PRAGMA integrity_check`), version du schéma et version connue d'oh, migrations en attente, dernière migration en échec, sauvegardes faites avant les migrations (`~/.oh/backups/`). En cas de corruption, les fichiers de la base (avec `-wal` et `-shm`) sont copiés en `oh.db.corrupt-<date>`, puis les options sont proposées : restaurer une sauvegarde d'avant migration, restaurer un export, et seulement en dernier recours repartir d'une base vide.

Revenir en arrière après une migration = restaurer la sauvegarde faite juste avant (`oh repair --restore`) ; oh ne défait jamais lui-même le schéma d'une base utilisateur.

```
oh repair [--check-only] [--auto] [--migrate] [--restore <fichier>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Vérifier uniquement, sans réparer |
| `--auto` | bool | Mode non interactif (scripts) ; confirme `--restore` sans terminal |
| `--migrate` | bool | Appliquer les migrations en attente ; une sauvegarde (`VACUUM INTO`, WAL compris) est faite avant ; l'erreur d'une migration est détaillée avec la sauvegarde à restaurer |
| `--restore` | string | Remettre en place une sauvegarde (fichier `.bak`) ; le démon doit être arrêté (`oh daemon stop`) ; la base remplacée est gardée en `oh.db.replaced-<date>` |

**Exemple :**

```bash
oh repair
oh repair --check-only
oh repair --migrate
oh repair --restore ~/.oh/backups/oh.db.v41-20261009T101500.000000000Z.bak
```

---

### oh purge

Suppression totale du hub et de ses données. Inventorie les artefacts (secrets du trousseau, restes des anciens déploiements dans les projets (`.opencode/`), dossier du hub, données OpenCode, binaire) puis les supprime après confirmation.

```
oh purge [--dry-run] [--force] [--keep-binary] [--include-tool-data]
```

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Afficher ce qui serait supprimé sans rien supprimer |
| `--force` | bool | Supprimer sans confirmation |
| `--keep-binary` | bool | Conserver le binaire `oh` |
| `--include-tool-data` | bool | Supprimer aussi les données globales de l'outil des sessions (pour opencode : `~/.local/share/opencode/`, `~/.config/opencode/`). Ancien nom : `--include-opencode` |

```bash
oh purge --dry-run
oh purge --force
oh purge --keep-binary
oh purge --include-tool-data --force
```

---

### oh daemon

Le démon `ohd` héberge le proxy d'identifiants LLM (jeton par groupe, les vraies clés restent sur la machine), la supervision des sessions, le suivi en direct, les décisions, les notifications système et les passerelles. oh le lance automatiquement (`oh daemon run`, commande interne). Sous Windows, il tourne dans le processus oh : fermer oh met les sessions en veille. Voir l'[ADR-047](../architecture/adr/047-session-interaction-daemon.fr.md) et l'[ADR-044](../architecture/adr/044-credential-proxy-session-limits.fr.md).

```
oh daemon status
oh daemon stop [--force]
```

| Commande / flag | Type | Description |
|-----------------|------|-------------|
| `status` | | État du démon : version, PID, adresse du proxy, serveurs actifs, jetons, demandes en attente (ou « ne tourne pas ») |
| `stop` | | Arrête le démon ; refusé si des sessions tournent |
| `stop --force` | bool | Arrêter même si des sessions tournent |

---

### oh remote

Exécution distante sur GitLab CI : un projet `oh-runner` par groupe GitLab, dont le pipeline est généré par oh ; la CI des projets n'est jamais modifiée. Les sessions se lancent avec `oh run --runtime remote` et se récupèrent avec [`oh session fetch`](cli-sessions.fr.md#oh-session-fetch) puis [`oh session resolve`](cli-sessions.fr.md#oh-session-resolve). Guide : [Exécution distante](../guides/remote-runners.fr.md). Le côté CI (`oh runner install|run`) est une commande interne utilisée par le pipeline.

**Expérimental en 5.0, désactivé par défaut** : `oh remote setup` (comme `oh run --runtime remote`, `oh session fetch` et `oh session resolve`) est refusé tant que `oh config set remote.experimental true` n'a pas été fait ; `oh remote status` et `oh doctor` le rappellent par un avertissement.

#### oh remote setup

Vérifie l'accès à l'API GitLab, crée ou valide le projet `oh-runner` du groupe, écrit le pipeline généré par oh, crée le jeton de déclenchement (gardé dans le trousseau) et pose les variables CI masquées : clé LLM des jobs (`--llm`), jeton d'accès de chaque projet cible (`--project`, portées `write_repository` et `read_api`), jeton du team-state (`--teamstate`). Les secrets sont lus au terminal (saisie masquée) ou dans une variable d'environnement (`--*-env`), jamais en argument. Relançable : seul ce qui manque ou a changé est modifié.

```
oh remote setup [flags]
```

| Flag | Type | Défaut | Description |
|------|------|--------|-------------|
| `--url` | string | celle du dépôt courant | URL de l'instance GitLab |
| `--group` | string | celui du dépôt courant | Chemin complet du groupe servi |
| `--name` | string | dérivé du groupe | Nom de la cible |
| `--runner-project` | string | `<groupe>/oh-runner` | Chemin du projet `oh-runner` |
| `--no-create` | bool | `false` | Ne pas créer le projet `oh-runner` s'il manque |
| `--force` | bool | `false` | Remplacer un `.gitlab-ci.yml` qui n'a pas été généré par oh |
| `--token-env` | string | | Variable d'environnement du jeton GitLab (portée `api`) |
| `--token-key` | string | | Clé du trousseau d'un jeton GitLab existant (portée `api`) |
| `--llm` | bool | `false` | Poser la clé LLM des jobs (saisie masquée) |
| `--llm-key-env` | string | | Variable d'environnement de la clé LLM des jobs |
| `--llm-provider` | string | `bedrock` | Fournisseur LLM des jobs : `bedrock`, `anthropic`, `openrouter` |
| `--llm-region` | string | `eu-west-1` (Bedrock) | Région du fournisseur |
| `--project` | string (répétable) | | Projet cible dont poser le jeton d'accès (chemin complet) |
| `--project-token-env` | string | | Variable d'environnement du jeton du projet (un seul `--project`) |
| `--teamstate` | bool | `false` | Poser le jeton d'écriture du dépôt team-state (saisie masquée) |
| `--teamstate-token-env` | string | | Variable d'environnement du jeton team-state |
| `--tag` | string | `oh` | Tag des runners |
| `--arch` | string | `amd64` | Architecture des runners : `amd64` ou `arm64` |
| `--builder` | string | `kaniko` | Construction de l'image du projet : `kaniko` ou `dind` |
| `--timeout` | string | `3h` | Durée maximale d'un job |
| `--oh-binary` | string | | Binaire oh Linux à téléverser (builds de développement) |

```bash
oh remote setup --group acme/web --llm --project acme/web/api --teamstate
oh remote setup --token-env GITLAB_TOKEN --llm-key-env OH_LLM_KEY --project acme/web/api --project-token-env API_TOKEN
```

#### oh remote status

Vérifie les cibles distantes (projet `oh-runner`, pipeline, variables, runners) : toutes, ou celle nommée. Sort en erreur si un contrôle échoue.

```
oh remote status [cible]
```

---

### oh serve

Lance un tableau de bord web local.

```
oh serve [--port 8080]
```

Démarre un serveur HTTP lié à `127.0.0.1` uniquement (jamais exposé sur le réseau). oh affiche l'URL du tableau de bord avec un **jeton d'accès** aléatoire, propre à ce lancement : `http://127.0.0.1:8080/?token=…`. Ouvrez-la telle quelle : le navigateur garde le jeton dans un cookie (`HttpOnly`, `SameSite=Strict`) et la barre d'adresse n'affiche plus que `/`. Une requête sans jeton valide reçoit 401 ; une requête dont l'en-tête `Host` n'est pas `127.0.0.1:<port>`, `localhost:<port>` ou `[::1]:<port>` reçoit 403. Le tableau de bord affiche projets, sessions, métriques, télémétrie des agents, board d'équipe (kanban), chronologie et membres.

**Endpoints API :**
- `GET /api/v1/health`
- `GET /api/v1/projects`
- `GET /api/v1/sessions?project_id=<id>`
- `GET /api/v1/metrics/agents?project_id=<id>`
- `GET /api/v1/platform/stats?period=7d|30d|all`
- `GET /api/v1/platform/sessions?limit=20`
- `GET /api/v1/team/board?project=<id>`
- `GET /api/v1/team/events?limit=50&project=<id>`
- `GET /api/v1/team/members`
- `GET /api/v1/chart/costs?period=30d`
- `GET /sse` (Server-Sent Events, temps réel)

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--port` | | int | Port d'écoute (défaut : 8080). L'ancienne forme courte `-p` (le projet partout ailleurs) reste acceptée, avec un avertissement |

**Exemple :**

```bash
oh serve
oh serve --port 9090
```

Pour un appel hors navigateur, passez le jeton dans l'URL : `curl "http://127.0.0.1:8080/api/v1/health?token=<jeton>"`.

> **Sécurité :** le serveur est lié à `127.0.0.1` uniquement et n'est jamais accessible depuis le réseau. Le contrôle de l'en-tête `Host` et le jeton empêchent une page web malveillante de lire le tableau de bord (DNS rebinding) ; le jeton n'est pas écrit sur le disque et change à chaque lancement. Ne partagez pas l'URL affichée.
