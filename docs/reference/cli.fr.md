> [Read in English](cli.en.md)

# Reference CLI

`oh` sans commande ouvre la TUI (dans un terminal interactif ; sinon l'aide). `oh -p <projet>` ouvre la TUI directement sur ce projet. `oh help` (ou `oh --help`) affiche la vue d'ensemble des commandes ; `oh <commande> --help` detaille une commande (flags, exemples).

## Flags globaux

Acceptes par toutes les commandes.

| Flag | Court | Defaut | Description |
|------|-------|--------|-------------|
| `--verbose` | `-v` | `false` | Active la sortie verbeuse (logs de niveau debug ; sinon : avertissements seulement) |
| `--log-format` | | `pretty` | Format des logs sur la sortie d'erreur : `pretty` ou `json` |
| `--no-tui` | | `false` | Desactive la TUI riche (invites en ligne uniquement) |
| `--help` | `-h` | | Aide de la commande |

---

## Table des matieres

| Section | Fichier | Commandes cles |
|---------|---------|---------------|
| [Sessions](cli-sessions.fr.md) | `cli-sessions.fr.md` | [Gestion des sessions (v5)](cli-sessions.fr.md#gestion-des-sessions-v5) : `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|resume\|stop\|open\|fetch\|resolve` ; `oh budget show\|set\|unset\|raise` ; `oh history` ; `oh beads` ; alias deprecies `oh start`, `oh audit`, `oh review`, `oh debug` |
| [Workflows](cli-workflows.fr.md) | `cli-workflows.fr.md` | `oh run` ; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive` ; `oh bundle build\|show` |
| [Projets](cli-projects.fr.md) | `cli-projects.fr.md` | `oh project list\|add\|remove\|rename\|move\|configure` |
| [Deploiement (alias de migration)](cli-deploy.fr.md) | `cli-deploy.fr.md` | `oh deploy`, `oh sync` (supprimes en v5 : message de migration) |
| [Configuration](cli-config.fr.md) | `cli-config.fr.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.fr.md) | `cli-infra.fr.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.fr.md) | `cli-mcp.fr.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Equipe](cli-team.fr.md) | `cli-team.fr.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Outils](cli-tools.fr.md) | `cli-tools.fr.md` | `oh skill add\|list\|remove\|search\|check\|budget`, `oh worktree *`, `oh secrets *`, `oh metrics`, `oh dashboard`, `oh board`, `oh version`, `oh completion` |

Commandes internes (masquees de l'aide, non documentees en detail) : `oh daemon run` (demon `ohd`, lance automatiquement par oh), `oh runner install|run` (cote CI de l'execution distante, utilise par le pipeline `oh-runner`), `oh mcp serve workflow` (serveur MCP `workflow` injecte dans chaque paquet de session).

---

## Codes de sortie

| Code | Signification |
|------|--------------|
| `0` | Succes. Aussi pour `oh deploy` / `oh sync` (message de migration) et pour `oh doctor`, meme quand des controles echouent (le resultat est affiche) |
| `1` | Erreur : le message est ecrit sur la sortie d'erreur. Par exemple : `oh workflow validate` ou `oh skill check` qui trouvent des erreurs, `oh run --headless` dont une session attend une decision ou depasse `--timeout`, `oh beads` quand `bd` echoue (le code de `bd` n'est pas transmis) |
| `2` | Erreur interne (panic) : oh affiche un message invitant a signaler le bogue |
