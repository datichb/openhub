> 🇬🇧 [Read in English](config.en.md)

# Reference de configuration

---

## Fichier de configuration du Hub

**Emplacement :** `~/.oh/hub.toml`  
**Cree par :** `oh init`  
**Afficher le chemin :** `oh config path`

### Structure TOML complete

```toml
[cli]
language = "en"                    # "fr" ou "en"

[opencode]
version = "latest"                 # version fixee ou "latest"
channel = "stable"                 # canal de release
auto_update = false                # mise a jour automatique du binaire opencode
install_dir = "~/.oh/bin"          # repertoire d'installation d'opencode
default_provider = "bedrock"       # bedrock | anthropic | openai | openrouter

[provider.bedrock]
aws_profile = "default"            # profil AWS (bedrock uniquement)
aws_region = "eu-west-1"           # region AWS (bedrock uniquement)
auth_mode = "bearer"               # "bearer" | "profile" (bedrock uniquement)

[mcp.figma]
enabled = true                     # activer le serveur MCP Figma
token_key = "openhub.mcp.figma.token"          # nom de la cle dans le trousseau (PAS le token)

[mcp.gitlab]
enabled = true
token_key = "openhub.mcp.gitlab.token"

[mcp.gslides]
enabled = false
token_key = "openhub.mcp.gslides.token"

[notify]
enabled = false
type = "slack"                     # mattermost | slack | discord | teams
webhook_url = "https://hooks.slack.com/services/..."
channel = "#dev-ai"                # Mattermost uniquement
bot_name = "OpenHub"

# Multi-destination (optionnel) : notifier plusieurs canaux simultanement
# [[notify.destinations]]
# type = "slack"
# webhook_url = "https://hooks.slack.com/..."
# bot_name = "OpenHub"

# [[notify.destinations]]
# type = "discord"
# webhook_url = "https://discord.com/api/webhooks/..."

[worktree]
auto_cleanup = true                # supprimer les worktrees mergees au demarrage
base_branch = ""                   # vide = detection auto (main/master)
```

---

## Commandes de configuration

| Commande | Description |
|----------|-------------|
| `oh config list [--json]` | Afficher toutes les valeurs de configuration |
| `oh config get <key>` | Obtenir une valeur specifique (notation pointee : `opencode.version`) |
| `oh config set <key> <value>` | Definir une valeur |
| `oh config unset <key>` | Supprimer une cle |
| `oh config path` | Afficher le chemin du fichier de configuration |
| `oh config language [fr\|en]` | Obtenir ou definir la langue d'affichage |
| `oh config websearch [enable\|disable\|status]` | Gerer le WebSearch pour les agents |

---

## Stockage des projets

Les projets sont stockes dans une **base de donnees SQLite** : `~/.oh/oh.db`.

### Champs d'un projet

| Champ | Description |
|-------|-------------|
| ID | Slug auto-genere + prefixe UUID |
| Name | Nom lisible du projet |
| Path | Chemin absolu sur le systeme de fichiers |
| Language | go, typescript, python, rust, java, etc. |
| Tracker | github, gitlab, jira, linear, ou vide |
| Provider | Surcharge au niveau projet, ou vide pour le defaut du hub |
| Model | Surcharge au niveau projet, ou vide pour le defaut du hub |
| MCPConfig | Configuration MCP par projet (JSON, surcharge le hub) |
| ProviderConfig | Configuration provider par projet (JSON, surcharge le hub) |
| Status | active, archived |
| CreatedAt | Horodatage de creation |
| UpdatedAt | Horodatage de derniere modification |

### Commandes projet

| Commande | Description |
|----------|-------------|
| `oh project list` | Lister tous les projets enregistres |
| `oh project add` | Enregistrer un nouveau projet |
| `oh project remove` | Desenregistrer un projet |
| `oh project rename` | Renommer un projet |
| `oh project move` | Mettre a jour le chemin d'un projet |
| `oh project configure` | Modifier les parametres d'un projet (provider, model, tracker) |

---

## Configuration des notifications

Configurez des webhooks pour envoyer des notifications sur les evenements de session (completion, erreurs, resultats d'audit).

```toml
[notify]
enabled = true
type = "slack"           # mattermost | slack | discord | teams
webhook_url = "https://hooks.slack.com/services/..."
channel = "#dev-ai"      # Mattermost uniquement
bot_name = "OpenHub"

# Multi-destination (optionnel) : notifier plusieurs canaux simultanement
[[notify.destinations]]
type = "slack"
webhook_url = "https://hooks.slack.com/..."
bot_name = "OpenHub"

[[notify.destinations]]
type = "discord"
webhook_url = "https://discord.com/api/webhooks/..."
```

| Champ | Type | Defaut | Description |
|-------|------|--------|-------------|
| `enabled` | bool | false | Activer les notifications |
| `type` | string | "mattermost" | Backend : mattermost, slack, discord, teams |
| `webhook_url` | string | — | URL du webhook entrant |
| `channel` | string | — | Nom du canal (Mattermost uniquement) |
| `bot_name` | string | "OpenHub" | Nom d'affichage du bot |
| `destinations` | array | — | Plusieurs destinations (remplace `type`/`webhook_url`) |

> **Note :** Quand `destinations` est defini, les champs `type`/`webhook_url` de premier niveau sont ignores.

---

## Configuration Team par Projet

Chaque projet peut surcharger indépendamment la configuration `[team]` du hub. La surcharge
est stockée dans la colonne JSON `team_config` de la table `projects` (SQLite).

### Champs de `ProjectTeamConfig`

| Champ | Type | Description |
|-------|------|-------------|
| `mode` | string | `"inherit"` (défaut) · `"custom"` · `"disabled"` |
| `state_repo` | string | URL Git remote du repo team-state _(custom uniquement)_ |
| `state_path` | string | Chemin de clone local — auto-déduit de `state_repo` si vide _(custom uniquement)_ |
| `member_id` | string | Override d'identité — fall back sur `member_id` du hub si vide _(custom uniquement)_ |

### Cascade de résolution

```
project.TeamConfig.Mode == "inherit" (ou nil)  →  config [team] du hub utilisée telle quelle
project.TeamConfig.Mode == "custom"            →  champs du projet ; member_id fall back sur le hub
project.TeamConfig.Mode == "disabled"          →  team désactivée pour ce projet
```

### Artefact deployé : `.opencode/team.json`

`oh deploy` résout la config effective et écrit `.opencode/team.json` à la racine du projet :

```json
{
  "enabled": true,
  "state_repo": "git@gitlab.com:acme/team-state.git",
  "state_path": "/Users/alice/.oh/team-states/team-state",
  "member_id": "alice"
}
```

Quand le mode est `disabled`, le fichier est supprimé (ou jamais créé) et le serveur MCP
`team` n'est pas injecté dans `opencode.json`.

### Déduction automatique du state path

Quand `state_path` est vide dans une config custom, il est déduit de `state_repo` :

```
git@gitlab.com:acme/other-team.git  →  ~/.oh/team-states/other-team
https://github.com/acme/my-team.git →  ~/.oh/team-states/my-team
```

### Définir le mode

- **À la création du projet :** le wizard `oh project add` inclut une étape Team.
- **Après création :** utiliser `team configure` dans l'omnibar TUI, puis redéployer.

---

## Configuration du Team-State (`config.toml`)

**Emplacement :** `~/.oh/team-state/config.toml`  
**Objet :** comportement à l'exécution du serveur MCP team-state (claims, intégration tracker).  
Ce fichier réside dans le dépôt git team-state — **pas** dans `hub.toml`.

> **Les credentials ne sont pas stockés ici.** Les tokens et URLs de connexion sont réutilisés
> depuis le bloc `[mcp.gitlab]` ou `[mcp.jira]` de `hub.toml` — pas de duplication de secrets.

### Section `[claim]`

Contrôle la durée de rétention des claims terminés avant nettoyage automatique.

```toml
[claim]
done_retention_days = 7
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `done_retention_days` | int | `7` | Nombre de jours qu'un claim reste en statut `done` avant d'être purgé automatiquement par `CleanupDoneClaims`. Mettre à `0` pour désactiver le nettoyage automatique. |

### Section `[tracker]`

Intègre le team-state avec un tracker d'issues externe (GitLab ou Jira).

```toml
[tracker]
enabled = true
type = "gitlab"              # "gitlab" ou "jira"
auto_sync = true             # synchroniser automatiquement à l'ouverture des vues team
sync_interval_minutes = 5    # intervalle de polling quand le board est ouvert (0 = désactivé)
auto_plan_assigned = true    # créer des claims "planned" pour les issues assignées dans le tracker
max_auto_plan_per_member = 5 # max de claims auto-planifiés par membre (évite de saturer le TODO)
push_labels = false          # synchroniser les labels hub vers le tracker (nécessite write_enabled sur le MCP)

# Mapping ID de ticket hub → IID externe via groupe de capture regex
ticket_patterns = { "T-SRU" = "SRU-(\\d+)", "T-FRONT" = "FRONT-(\\d+)" }

[tracker.projects]
"T-SRU" = "42"               # ID projet hub → ID ou chemin projet GitLab
"T-FRONT" = "group/frontend"
```

| Champ | Type | Défaut | Description |
|-------|------|--------|-------------|
| `enabled` | bool | `false` | Activer l'intégration tracker |
| `type` | string | — | Backend tracker : `"gitlab"` ou `"jira"` |
| `auto_sync` | bool | `false` | Synchroniser automatiquement à l'ouverture des vues team |
| `sync_interval_minutes` | int | `0` | Intervalle de polling en minutes quand le board est ouvert. `0` désactive le polling. |
| `auto_plan_assigned` | bool | `false` | Créer automatiquement des claims `"planned"` pour les issues assignées à chaque membre dans le tracker |
| `max_auto_plan_per_member` | int | `5` | Plafond de claims auto-planifiés par membre et par sync (évite de saturer la liste TODO) |
| `push_labels` | bool | `false` | Synchroniser les labels hub vers le tracker. Nécessite `write_enabled = true` sur le serveur MCP correspondant. |
| `ticket_patterns` | map | `{}` | Map ID projet hub → regex avec **exactement un groupe de capture** extrayant l'IID numérique (ex. `"SRU-(\\d+)"`) |
| `[tracker.projects]` | map | `{}` | Map ID projet hub → ID / chemin projet GitLab ou clé projet Jira |

**Notes :**

- **GitLab** (`type = "gitlab"`) : les credentials et l'URL de base proviennent de `[mcp.gitlab]` dans `hub.toml`.
- **Jira** (`type = "jira"`) : les credentials et l'URL de base proviennent de `[mcp.jira]` dans `hub.toml`. Les issues fermées sont détectées via `statusCategory.key == "done"` — les noms de workflow personnalisés sont ignorés.
- `push_labels` nécessite `write_enabled = true` sur le serveur MCP correspondant dans `hub.toml`.
- Chaque regex `ticket_patterns` doit contenir **exactement un groupe de capture** extrayant l'IID numérique.

---

## Vue Notifications

Le TUI capture chaque toast dans un store en mémoire et rend l'historique complet
accessible via une vue dédiée.

### Accéder à la vue

Taper l'un des termes suivants dans l'omnibar :

| Commande | Description |
|----------|-------------|
| `notifications` | Ouvrir l'historique des notifications |
| `notif` / `logs` / `messages` / `toasts` / `erreurs` | Alias |

### Ce qu'elle affiche

```
14:32:05  ✗  Erreur setup team : git clone https://gitlab.com/...: fatal: repository not found
14:31:58  ✓  Team configurée : custom — redéployez pour appliquer
14:31:52  →  Initialisation team pour ce projet...
```

Chaque entrée contient :
- **Horodatage** (`HH:MM:SS`)
- **Icône de niveau** — `✓` succès · `✗` erreur · `!` avertissement · `→` info
- **Message complet non tronqué** — les toasts à l'écran sont tronqués (80 chars pour
  les erreurs/warnings, 50 pour les autres) ; le store conserve toujours le texte complet

La vue est **scrollable** (`j` / `k` pour naviguer).

### Comportement du store

- **Capacité :** 50 entrées en mémoire par session (FIFO — la plus ancienne est evincée quand plein)
- **Persistance :** les notifications sont appendées dans `~/.oh/notifications.jsonl` après
  chaque toast. Le fichier est roté au démarrage du TUI (max 500 lignes, entrées > 7 jours
  supprimées). La vue Notifications charge les 50 dernières entrées depuis ce fichier à chaque
  Mount — l'historique est disponible entre les sessions.
- **Log stderr :** les notifications de niveau erreur sont aussi ecrites sur stderr sous la
  forme `[ERROR] <message>`, utile pour la redirection de logs en CI

---

## Sélection de Texte

Le TUI supporte la sélection de texte native à la souris partout en dehors des widgets
interactifs (omnibar, modals). Aucune dépendance externe n'est requise.

### Comment l'utiliser

1. **Clic et glisser** pour sélectionner — les cellules sélectionnées sont en reverse video
2. **Relâcher** — le texte est automatiquement copié dans le presse-papiers
3. **Double-clic** — sélectionne le mot sous le curseur
4. **Triple-clic** — sélectionne la ligne entière
5. **Esc** — efface la sélection

### Clipboard

L'écriture clipboard utilise un pipeline à deux stratégies (sans dépendance Go externe) :

| Stratégie | Plateformes |
|-----------|------------|
| Séquence OSC 52 via `/dev/tty` | Tous les terminaux modernes (iTerm2, WezTerm, Alacritty, kitty, tmux avec `set-clipboard on`), fonctionne en SSH |
| `pbcopy` | macOS fallback |
| `wl-copy` | Linux/Wayland fallback |
| `xclip` / `xsel` | Linux/X11 fallback |
| `clip.exe` | Windows fallback |

Les deux stratégies sont tentées ; l'opération réussit si au moins une fonctionne.

### Zones interactives (pass-through)

Les événements souris dans les zones suivantes sont passés à tview sans déclencher
la sélection :

- Container de l'omnibar, champ de saisie, liste de suggestions
- Toute overlay modale (prompts, listes de sélection)

### Overlays de toast

Les toasts sont **sélectionnables** — cliquer sur un toast démarre une sélection
plutôt qu'une interaction avec le widget.

---

## Secrets / Cles API

Les secrets sont stockes dans le **trousseau du systeme** (macOS Keychain, Linux secret-service, Windows Credential Manager).

### Fallback

Si le trousseau n'est pas disponible, les secrets sont stockes dans `~/.oh/secrets.enc` (fichier chiffre AES-256-GCM avec KDF Argon2id).

**Source de la passphrase (pour le fallback) :**

1. Variable d'environnement `OH_PASSPHRASE`
2. Saisie interactive dans le terminal (avec confirmation a la premiere utilisation, minimum 8 caracteres)

### Commandes de gestion des secrets

| Commande | Description |
|----------|-------------|
| `oh service setup` | Stocker un token dans le trousseau |

### Cles de secrets connues

| Cle | Usage |
|-----|-------|
| `bedrock-token-default` | Token Bearer AWS pour Bedrock |
| `bedrock-token-<project-id>` | Token Bedrock par projet |
| `anthropic-api-key-default` | Cle API Anthropic globale |
| `anthropic-api-key-<project-id>` | Cle API Anthropic par projet |
| `openrouter-api-key-default` | Cle API OpenRouter globale |
| `openrouter-api-key-<project-id>` | Cle API OpenRouter par projet |
| `figma-token` | Token API Figma |
| `gitlab-token` | Token API GitLab |
| `gslides-token` | Token OAuth Google Slides |

---

## Configuration projet (opencode.json)

Chaque projet possede un `opencode.json` a sa racine, genere par `oh deploy`.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "claude-sonnet-4-5",
  "provider": { ... },
  "agent": { ... },
  "plugin": ["context-mode"],
  "compaction": { "auto": true, "prune": true, "reserved": 10000 },
  "mcpServers": {
    "figma": { "command": "oh", "args": ["mcp", "serve", "figma"] },
    "gitlab": { "command": "oh", "args": ["mcp", "serve", "gitlab"] }
  }
}
```

> **Note :** Ce fichier est gere par `oh deploy` — ne le modifiez pas manuellement sauf si vous savez ce que vous faites.

---

## Variables d'environnement

| Variable | Usage |
|----------|-------|
| `OH_PASSPHRASE` | Passphrase pour le stockage chiffre de secrets (fallback) |
| `FIGMA_TOKEN` | Token API Figma (lu par le serveur MCP au runtime) |
| `GITLAB_TOKEN` | Token API GitLab (lu par le serveur MCP au runtime) |
| `GITLAB_URL` | URL de l'instance GitLab (defaut : `https://gitlab.com`) |
| `GOOGLE_ACCESS_TOKEN` | Token OAuth Google (lu par le serveur MCP au runtime) |
| `GITHUB_TOKEN` | Token API GitHub (lu par le serveur MCP au runtime) ; alias : `GH_TOKEN` |
| `GITHUB_WRITE_ENABLED` | Mettre a `true` pour activer l'outil `github_create_issue` |
| `JIRA_URL` | URL de l'instance Jira (ex. `https://masociete.atlassian.net`) |
| `JIRA_TOKEN` | Token API Jira ; ou utiliser `JIRA_USER` + `JIRA_API_TOKEN` |
| `JIRA_WRITE_ENABLED` | Mettre a `true` pour activer l'outil `jira_transition_issue` |
| `LINEAR_API_KEY` | Cle API Linear (lue par le serveur MCP au runtime) |
| `LINEAR_WRITE_ENABLED` | Mettre a `true` pour activer `linear_create_issue` / `linear_update_issue` |
| `PAGER` | Pager personnalise pour l'aide (defaut : `less`) |

---

## Resolution du provider

Au demarrage d'une session, le provider LLM est resolu dans cet ordre :

1. Flag `--provider` / `-P` (priorite la plus haute)
2. `project.Provider` dans la base de donnees
3. `opencode.default_provider` dans `hub.toml`
4. `"bedrock"` (fallback en dur)

---

## Recapitulatif des emplacements de fichiers

| Chemin | Usage |
|--------|-------|
| `~/.oh/` | Repertoire de configuration du hub |
| `~/.oh/hub.toml` | Fichier de configuration du hub |
| `~/.oh/hub/` | Hub content extrait (agents, skills) |
| `~/.oh/oh.db` | Base de donnees SQLite (projets, sessions) |
| `~/.oh/secrets.enc` | Fichier de secrets chiffre (fallback) |
| `~/.oh/bin/` | Binaire opencode gere |
| `~/.oh/mcp/<name>/manifest.json` | Manifeste de serveur MCP personnalise (registre dynamique) |
| `<projet>/opencode.json` | Config opencode du projet (generee par deploy) |
| `<projet>/.opencode/agents/` | Definitions d'agents deployees |
| `<projet>/.opencode/skills/` | Protocoles de skills deployes |

> **Integrite de la base de donnees :** Executez `oh repair --check-only` pour verifier la base SQLite. La version courante du schema est affichee par `oh repair`.
>
> **Tableau de bord web :** `oh serve` est lie a `127.0.0.1` uniquement et n'est jamais accessible depuis le reseau.
