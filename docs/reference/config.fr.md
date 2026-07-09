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
token_key = "figma-token"          # nom de la cle dans le trousseau (PAS le token)

[mcp.gitlab]
enabled = true
token_key = "gitlab-token"

[mcp.gslides]
enabled = false
token_key = "gslides-token"

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
| `<projet>/opencode.json` | Config opencode du projet (generee par deploy) |
| `<projet>/.opencode/agents/` | Definitions d'agents deployees |
| `<projet>/.opencode/skills/` | Protocoles de skills deployes |
