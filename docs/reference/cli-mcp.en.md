> [Lire en francais](cli-mcp.fr.md)

# CLI Reference — MCP & Plugins

## MCP Management

### oh mcp enable

Enable an MCP service at hub level or for a specific project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp enable figma
oh mcp enable gitlab --project my-project
```

---

### oh mcp disable

Disable an MCP service at hub level or for a specific project.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp disable figma
oh mcp disable gitlab --project my-project
```

---

### oh mcp reset

Remove the project-level override for an MCP service (revert to hub config).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | **(required)** Project name or ID |

```bash
oh mcp reset figma --project my-project
```

---

### oh mcp setup

Interactive wizard to configure an MCP service (token, options).

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID |

```bash
oh mcp setup
oh mcp setup --project my-project
```

---

### oh mcp status

Display the status of all MCP services.

| Flag | Short | Description |
|------|-------|-------------|
| `--project` | `-p` | Project name or ID (shows effective config) |

```bash
oh mcp status
oh mcp status --project my-project
```

---

### oh mcp serve

Serve a built-in MCP server via stdio.

```bash
oh mcp serve figma
oh mcp serve gitlab
oh mcp serve gslides
oh mcp serve team
oh mcp serve github
oh mcp serve jira
oh mcp serve linear
```

---

### oh mcp list

List available MCP servers. Aliases: `ls`

| Flag | Type | Description |
|------|------|-------------|
| `--json` | bool | Output in JSON format |

```bash
oh mcp list
oh mcp ls --json
```

---

## Plugin Management

### oh plugin install

Install a plugin by name.

```bash
oh plugin install my-plugin
```

---

### oh plugin remove

Remove an installed plugin. Aliases: `rm`, `uninstall`

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` | `-f` | bool | Skip confirmation |

```bash
oh plugin remove my-plugin
oh plugin rm my-plugin -f
```

---

### oh plugin list

List installed plugins. Aliases: `ls`

```bash
oh plugin list
oh plugin ls
```

---

### oh plugin status

Show status of all installed plugins.

```bash
oh plugin status
```

---

