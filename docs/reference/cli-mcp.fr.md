> [Read in English](cli-mcp.en.md)

# Reference CLI — MCP & Plugins

## Gestion MCP

### oh mcp enable

Active un service MCP au niveau hub ou pour un projet.

```
oh mcp enable <service> [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp enable figma
oh mcp enable gitlab --project mon-projet
```

---

### oh mcp disable

Desactive un service MCP au niveau hub ou pour un projet.

```
oh mcp disable <service> [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp disable figma
oh mcp disable gitlab --project mon-projet
```

---

### oh mcp reset

Supprime l'override projet pour un service (retour a la config hub).

```
oh mcp reset <service> --project <name>
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | **(requis)** Nom ou ID du projet |

**Exemple :**

```bash
oh mcp reset figma --project mon-projet
```

---

### oh mcp setup

Configure un service MCP (wizard interactif : token, options).

```
oh mcp setup [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet |

**Exemple :**

```bash
oh mcp setup
oh mcp setup --project mon-projet
```

---

### oh mcp status

Affiche le statut de tous les services MCP.

```
oh mcp status [options]
```

| Flag | Court | Description |
|------|-------|-------------|
| `--project` | `-p` | Nom ou ID du projet (affiche la config effective) |

**Exemple :**

```bash
oh mcp status
oh mcp status --project mon-projet
```

---

### oh mcp serve

Lance un serveur MCP integre via stdio.

```
oh mcp serve <name>
```

Sert un serveur MCP natif (figma, gitlab, gslides, team, github, jira, linear).

**Exemple :**

```bash
oh mcp serve gitlab
oh mcp serve figma
oh mcp serve gslides
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
```

---

### oh mcp list

Liste les serveurs MCP disponibles.

**Alias :** `oh mcp ls`

```
oh mcp list [options]
```

| Flag | Description |
|------|-------------|
| `--json` | Sortie au format JSON |

**Exemple :**

```bash
oh mcp list
oh mcp ls --json
```

---


## Gestion des plugins

`oh plugin` (plugins globaux opencode V1, dont RTK) a été supprimé en v5 : les plugins se déclarent par workflow (`plugins:`), voir [Workflows livrés](workflows.fr.md#plugins-et-code-mode).
