# Référence CLI — commandes `oh service` et `oh mcp`

> [Read in English](services.en.md)

Gestion des services externes et serveurs MCP (Model Context Protocol) intégrés au binaire `oh`.

---

## Architecture

Les serveurs MCP sont **intégrés au binaire Go** — il n'y a pas de répertoire `servers/` séparé ni d'étape de build Node.js. Chaque serveur est implémenté nativement dans `cli/internal/mcp/` et servi via stdio JSON-RPC.

Serveurs disponibles :
- **figma** — Intégration API Figma (fichiers, composants, signaux UI)
- **gitlab** — Intégration API GitLab (issues, MRs, labels, milestones)
- **gslides** — Intégration Google Slides

---

## `oh mcp` — Gestion des serveurs MCP

### `oh mcp list [--json]`

Liste tous les serveurs MCP disponibles et leur état.

```bash
oh mcp list
oh mcp list --json
```

### `oh mcp serve <name>`

Démarre un serveur MCP via stdio JSON-RPC. C'est la commande injectée dans `opencode.json` lors du déploiement.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
```

---

## `oh service` — Configuration des services

### Synopsis

```bash
oh service <sous-commande> [service] [options]
```

| Sous-commande | Description |
|---|---|
| `setup [nom]` | Configure un service interactivement (token, validation) |
| `remove <nom>` | Supprime la configuration d'un service |

**Aliases :** `oh figma setup` · `oh gitlab setup` · `oh gslides setup`

### `oh service setup`

Configure interactivement un service (credentials stockés dans le Keychain macOS).

```bash
oh service setup [nom-du-service]
```

**Options :**
- `--project, -p <id>` — Configure le service pour un projet specifique (surcharge le hub)

En mode projet, le token est stocke avec une cle specifique (`<service>-token-<project-id>`) et le projet utilise ce token au lieu du token hub.

**Comportement :**
- Si `nom-du-service` est omis, affiche un menu de sélection.
- Guide l'utilisateur pour chaque credential requis.
- Valide le format du token et la connectivité API.
- Stocke les credentials de manière sécurisée dans le keychain système.
- Active le service dans `hub.toml`.

**Exemples :**

```bash
# Mode interactif — menu de sélection
oh service setup

# Configurer Figma directement
oh service setup figma

# Configurer GitLab (alias)
oh gitlab setup
```

### `oh service remove`

Supprime la configuration d'un service (retire les credentials du keychain).

```bash
oh service remove <nom-du-service>
```

---

## Déploiement — `mcpServers` dans `opencode.json`

Lors de `oh deploy`, le CLI injecte un bloc `mcpServers` dans le `opencode.json` du projet pour chaque service activé :

```json
{
  "mcpServers": {
    "figma": {
      "command": "oh",
      "args": ["mcp", "serve", "figma"]
    },
    "gitlab": {
      "command": "oh",
      "args": ["mcp", "serve", "gitlab"]
    }
  }
}
```

### Sélection des MCP par projet

Les services sont sélectionnés par projet lors de `oh init` ou via `oh project configure` :

```bash
# Lors de l'initialisation du projet
oh init
# → Étape propose : "Activer des intégrations MCP pour ce projet ?"

# Changer la sélection MCP pour un projet existant
oh project configure --services figma,gitlab
```

### Cascade MCP par projet

Quand un projet a une `MCPConfig` non-vide, elle **remplace** la liste MCP du hub pour ce projet. Les credentials (token_key) et options (write_enabled) peuvent etre surcharges par service.

Si `MCPConfig` est vide → le projet herite du hub.

---

## Variables d'environnement au runtime

Au runtime, les serveurs MCP lisent les credentials depuis l'environnement. Le binaire `oh` les injecte automatiquement depuis le keychain.

| Service | Variables requises |
|---------|-------------------|
| figma | `FIGMA_TOKEN` |
| gitlab | `GITLAB_TOKEN`, `GITLAB_URL` |
| gslides | `GOOGLE_ACCESS_TOKEN` |

---

## Stockage de la configuration (`hub.toml`)

L'activation des services est stockée dans `~/.oh/hub.toml` :

```toml
[mcp.figma]
enabled = true
token_key = "figma-token"

[mcp.gitlab]
enabled = true
token_key = "gitlab-token"
write_enabled = true
```

Les credentials sont stockés dans le keychain système (macOS Keychain / Linux secret-service), pas en clair dans des fichiers.

---

## Voir aussi

- [Guide d'intégration Figma](../guides/figma-integration.fr.md)
- [Guide d'intégration GitLab](../guides/gitlab-integration.fr.md)
- [Référence CLI complète](cli.fr.md)
