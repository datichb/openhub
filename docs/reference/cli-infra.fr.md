> [Read in English](cli-infra.en.md)

# Reference CLI — Infrastructure

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

Vérifie l'état du système et les dépendances. Sans flag. Sort avec le code 0 même si des contrôles échouent (le résultat est affiché).

```
oh doctor
```

Contrôles : OS et architecture, git, `bd` et `fzf` (optionnels), version d'oh (mise à jour disponible), `hub.toml`, identifiants du provider, base de données, clés API (trousseau), Beads « zéro impact » (hooks, `.gitignore`), puis les contrôles v5 : opencode V2 (version minimale ; V1 refusé avec le [guide de migration](../guides/migration-v5.fr.md)), démon `ohd`, git, ouverture des terminaux, moteur de conteneurs et image, passerelles, restrictions et mémoire, sécurité (socket, capacité, jetons), restes des anciens déploiements, intégrité des workflows du team-state, cibles distantes.

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
| `version` | argument | Version cible (défaut : la dernière) |
| `--check` | bool | Vérifier seulement si une mise à jour est disponible |

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
| `--merge` | bool | Fusionner avec les données existantes (projets uniquement) |

**Exemple :**

```bash
oh import oh-backup-2026-10-06.tar.gz
oh import ~/sauvegardes/oh-backup.tar.gz --overwrite
oh import ~/sauvegardes/oh-backup.tar.gz --merge
```

---

### oh repair

Vérifie l'intégrité de la base SQLite (`PRAGMA integrity_check` sur `~/.oh/oh.db`). En cas de corruption, tente une récupération et propose : restaurer une sauvegarde, réinitialiser la base, ou réenregistrer les projets. Affiche aussi la version du schéma.

```
oh repair [--check-only] [--auto]
```

| Flag | Type | Description |
|------|------|-------------|
| `--check-only` | bool | Vérifier uniquement, sans réparer |
| `--auto` | bool | Mode non interactif (scripts) |

**Exemple :**

```bash
oh repair
oh repair --check-only
oh repair --auto
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

Démarre un serveur HTTP lié à `127.0.0.1` uniquement (jamais exposé sur le réseau). Le tableau de bord affiche projets, sessions, métriques, télémétrie des agents, board d'équipe (kanban), chronologie et membres.

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
| `--port` | `-p` | int | Port d'écoute (défaut : 8080) |

**Exemple :**

```bash
oh serve
oh serve --port 9090
```

> **Sécurité :** le serveur est lié à `127.0.0.1` uniquement et n'est jamais accessible depuis le réseau.
