> [Read in English](cli-mcp.en.md)

# Reference CLI — MCP & Plugins

## Gestion MCP

Serveurs MCP integres a oh : `figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, plus les serveurs personnalises declares dans `~/.oh/mcp/<nom>/manifest.json`. Une session recoit les serveurs MCP du projet (ou seulement ceux listes par le champ `mcp:` du workflow) dans son paquet de session. Pages par serveur : [Figma](mcp-figma.fr.md), [GitHub](mcp-github.fr.md), [GitLab](mcp-gitlab.fr.md), [Google Slides](mcp-gslides.fr.md), [Jira](mcp-jira.fr.md), [Linear](mcp-linear.fr.md), [Team](mcp-team.fr.md).

Toutes les commandes ci-dessous (sauf `list` et `serve`) acceptent `-p, --project <nom ou ID>`.

### oh mcp enable

Active un service MCP au niveau hub ou pour un projet. Avec `--project` et sans token, propose d'heriter le token du hub ou d'en configurer un nouveau.

```
oh mcp enable <service> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp enable figma
oh mcp enable gitlab --project mon-projet
```

---

### oh mcp disable

Desactive un service MCP au niveau hub ou pour un projet. Avec `--project`, le service est desactive pour ce projet quelle que soit la config du hub.

```
oh mcp disable <service> [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp disable figma
oh mcp disable gitlab --project mon-projet
```

---

### oh mcp reset

Supprime l'override projet d'un service (retour a la config du hub). Exige `--project`.

```
oh mcp reset <service> --project <name>
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | **(requis)** Nom ou ID du projet |

**Exemple :**

```bash
oh mcp reset figma --project mon-projet
```

---

### oh mcp setup

Configure un service MCP (assistant interactif : token, options). Services : `figma`, `gitlab`, `gslides`, `jira` ; sans argument, le service est choisi dans une liste. Le mode ecriture est propose pour les services qui ont des outils d'ecriture (`gitlab`, `jira`). Avec `--project`, le token est stocke dans le trousseau sous une cle propre au projet.

```
oh mcp setup [service] [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp setup
oh mcp setup jira --project mon-projet
```

---

### oh mcp status

Affiche le statut de tous les services MCP. Avec `--project`, affiche la config effective (overrides du projet compris).

```
oh mcp status [options]
```

| Flag | Court | Type | Description |
|------|-------|------|-------------|
| `--project` | `-p` | string | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp status
oh mcp status --project mon-projet
```

---

### oh mcp serve

Lance un serveur MCP integre sur stdin/stdout (JSON-RPC). Utilise par opencode, via la declaration du serveur dans le paquet de session ; rarement lance a la main.

```
oh mcp serve <name> [--token-key <cle>]
```

| Flag | Type | Description |
|------|------|-------------|
| `--token-key` | string | Cle du trousseau d'ou lire le token du service (pose `FIGMA_TOKEN`, `GITLAB_TOKEN` ou `GOOGLE_ACCESS_TOKEN` s'ils ne sont pas deja definis) |

`oh mcp serve workflow` est interne : serveur MCP `workflow` (`workflow_status`, `workflow_checkpoint`, `workflow_outputs`) injecte dans chaque paquet de session, absent de `list`, `setup` et `enable`.

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma --token-key openhub.mcp.figma.token
```

---

### oh mcp list

Liste tous les serveurs MCP d'oh (`figma`, `github`, `gitlab`, `gslides`, `jira`, `linear`, `team`, puis les serveurs personnalises de `~/.oh/mcp/`) et leur commande.

**Alias :** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Sortie au format JSON |

**Exemple :**

```bash
oh mcp list
oh mcp ls --json
```

---

### oh service (deprecie)

`oh service`, `oh service setup` et `oh service remove` existent encore mais sont deprecies et masques de l'aide : utilisez `oh mcp status`, `oh mcp setup` et `oh mcp disable`.

---

## Gestion des plugins

`oh plugin` (plugins globaux opencode V1, dont RTK) a été supprimé en v5 : les plugins se déclarent par workflow (`plugins:`), voir [Workflows livrés](workflows.fr.md#plugins-et-code-mode).
