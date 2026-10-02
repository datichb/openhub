> [Read in English](cli.en.md)

# Reference CLI

## Flags globaux

| Flag | Court | Description |
|------|-------|-------------|
| `--verbose` | `-v` | Active la sortie verbeuse (logging debug) |
| `--log-format` | | Format de sortie des logs : `pretty` (defaut) ou `json` |
| `--no-tui` | | Desactive le TUI riche (utilise les prompts inline) |

---

## Table des matieres

| Section | Fichier | Commandes cles |
|---------|---------|---------------|
| [Sessions](cli-sessions.fr.md) | `cli-sessions.fr.md` | `oh start`, `oh review`, `oh audit`, `oh debug` |
| [Projets](cli-projects.fr.md) | `cli-projects.fr.md` | `oh project list\|add\|remove\|rename\|move\|configure` |
| [Deploiement](cli-deploy.fr.md) | `cli-deploy.fr.md` | `oh deploy`, `oh sync` |
| [Configuration](cli-config.fr.md) | `cli-config.fr.md` | `oh config *`, `oh provider setup`, `oh config model` |
| [Infrastructure](cli-infra.fr.md) | `cli-infra.fr.md` | `oh init`, `oh doctor`, `oh repair`, `oh upgrade`, `oh purge`, `oh serve` |
| [MCP & Plugins](cli-mcp.fr.md) | `cli-mcp.fr.md` | `oh mcp *`, `oh plugin *` |
| [Equipe](cli-team.fr.md) | `cli-team.fr.md` | `oh team *`, `oh teams *`, `oh conventions`, `oh policies` |
| [Outils](cli-tools.fr.md) | `cli-tools.fr.md` | `oh skill *`, `oh worktree *`, `oh secrets *`, `oh metrics` |

---

## Codes de sortie

| Code | Signification |
|------|--------------|
| `0` | Succes |
| `1` | Erreur |
| `2` | Avertissement |
