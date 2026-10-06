> [Read in English](cli-infra.en.md)

# Reference CLI — Infrastructure

## Infrastructure

### oh init

Initialise oh pour la premiere fois. Wizard interactif.

```
oh init
```

Pas de flags. Configure : langue, opencode, projet, serveurs MCP, base de donnees et deploiement.

**Exemple :**

```bash
oh init
```

---

### oh doctor

Verifie l'etat du systeme.

```
oh doctor
```

Pas de flags. Checks : OS, git, opencode V2 (version minimale, V1 refusé), bd, fzf, config, BDD, cles API, runtime v5. Verifie egalement la disponibilite de mises a jour pour le binaire `oh`.

**Exemple :**

```bash
oh doctor
```

---

### oh status

Affiche l'etat du hub.

```
oh status [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh status
oh status --json
```

---

### oh upgrade oh

Met a jour le binaire `oh` en place (remplacement atomique). Uniquement pour les installations hors Homebrew.

```
oh upgrade oh [version] [--check]
```

| Flag | Description |
|------|-------------|
| `--check` | Verifie la version disponible sans telecharger |
| `version` | Version cible (defaut : derniere) |

**Exemple :**

```bash
oh upgrade oh
oh upgrade oh 1.3.0
oh upgrade oh --check
```

> **Utilisateurs Homebrew :** utilisez `brew upgrade openhub` a la place.

---


### oh service setup (deprecated)

> **Deprecated :** Utilisez `oh mcp setup` a la place.

Configure un service MCP. Wizard interactif. Stocke les tokens dans le keychain.

```
oh service setup
```

---

### oh service remove (deprecated)

> **Deprecated :** Utilisez `oh mcp disable` a la place.

Supprime un service.

```
oh service remove [service-name]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--force` | `-f` | Supprimer sans confirmation |

---


### oh export

Cree une archive de sauvegarde des donnees du hub.

```
oh export [--output <chemin>]
```

Cree une archive `.tar.gz` contenant : `oh.db`, `hub.toml`, `secrets.enc` (chiffre). Inclut un fichier de checksum SHA-256.

| Flag | Court | Description |
|------|-------|-------------|
| `--output` | `-o` | Chemin de sortie (defaut : `./oh-backup-YYYY-MM-DD.tar.gz`) |

**Exemple :**

```bash
oh export
oh export --output ~/sauvegardes/oh-backup.tar.gz
```

---

### oh import

Restaure depuis une archive de sauvegarde.

```
oh import <fichier> [--overwrite] [--merge]
```

Verifie le checksum SHA-256 avant d'ecrire les donnees.

| Flag | Description |
|------|-------------|
| `--overwrite` | Ecraser les donnees existantes |
| `--merge` | Fusionner les projets uniquement (non-destructif) |

**Exemple :**

```bash
oh import oh-backup-2025-07-22.tar.gz
oh import ~/sauvegardes/oh-backup.tar.gz --overwrite
oh import ~/sauvegardes/oh-backup.tar.gz --merge
```

---

### oh repair

Diagnostique et repare la base de donnees SQLite.

```
oh repair [--check-only] [--auto]
```

Execute `PRAGMA integrity_check` sur `~/.oh/oh.db`. En cas de corruption, propose des options de recuperation : restaurer depuis une sauvegarde, reinitialiser la base de donnees, ou re-enregistrer les projets manuellement. Affiche egalement la version courante du schema.

| Flag | Description |
|------|-------------|
| `--check-only` | Diagnostiquer sans effectuer de modifications |
| `--auto` | Mode non-interactif |

**Exemple :**

```bash
oh repair
oh repair --check-only
oh repair --auto
```

---

### oh serve

Lance un tableau de bord web local.

```
oh serve [--port 8080]
```

Demarre un serveur HTTP lie a `127.0.0.1` uniquement (jamais expose sur le reseau). Le tableau de bord affiche les projets, sessions, telemetrie des agents, board equipe et graphiques de couts.

**Endpoints API :**
- `GET /api/v1/health`
- `GET /api/v1/projects`
- `GET /api/v1/sessions?project_id=<id>`
- `GET /api/v1/metrics/agents?project_id=<id>`
- `GET /api/v1/opencode/stats?period=7d|30d|all`
- `GET /api/v1/opencode/sessions?limit=20`
- `GET /api/v1/team/board?project=<id>`
- `GET /api/v1/team/events?limit=50&project=<id>`
- `GET /api/v1/team/members`
- `GET /api/v1/chart/costs?period=30d`
- `GET /sse` (Server-Sent Events, temps reel)

| Flag | Court | Description |
|------|-------|-------------|
| `--port` | `-p` | Port (defaut : 8080) |

**Exemple :**

```bash
oh serve
oh serve --port 9090
```

> **Securite :** Le serveur est lie a `127.0.0.1` uniquement et n'est jamais accessible depuis le reseau.

---

---


---

### oh purge

Suppression totale du hub et de ses donnees. Inventorie tous les artefacts (secrets keychain, deploiements, repertoire hub, donnees OpenCode, binaire) puis les supprime apres confirmation.

| Flag | Type | Description |
|------|------|-------------|
| `--dry-run` | bool | Afficher ce qui serait supprime sans supprimer |
| `--force` | bool | Ignorer les confirmations |
| `--keep-binary` | bool | Conserver le binaire `oh` installe |
| `--include-opencode` | bool | Supprimer egalement les donnees globales OpenCode |

```bash
oh purge --dry-run
oh purge --force
oh purge --keep-binary
oh purge --include-opencode --force
```
