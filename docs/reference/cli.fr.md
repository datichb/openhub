> [Read in English](cli.en.md)

# Référence CLI

`oh` sans commande ouvre la TUI (dans un terminal interactif ; sinon l'aide). `oh -p <projet>` ouvre la TUI directement sur ce projet. `oh help` (ou `oh --help`) affiche la vue d'ensemble des commandes (toutes les commandes visibles, par section, avec leurs options ; construite à partir des commandes elles-mêmes) ; `oh <commande> --help` détaille une commande (flags, exemples). Toute l'aide suit la langue de l'interface (`oh config language`).

## Flags globaux

Acceptés par toutes les commandes.

| Flag | Court | Défaut | Description |
|------|-------|--------|-------------|
| `--verbose` | `-v` | `false` | Active la sortie verbeuse (logs de niveau debug ; sinon : avertissements seulement) |
| `--log-format` | | `pretty` | Format des logs sur la sortie d'erreur : `pretty` ou `json` |
| `--no-tui` | | `false` | Désactive la TUI riche (invites en ligne uniquement) |
| `--help` | `-h` | | Aide de la commande |

---

## Table des matières

| Section | Fichier | Commandes clés |
|---------|---------|---------------|
| [Sessions](cli-sessions.fr.md) | `cli-sessions.fr.md` | [Gestion des sessions (v5)](cli-sessions.fr.md#gestion-des-sessions-v5) : `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|trace\|resume\|stop\|delete\|open\|fetch\|resolve` ; `oh budget show\|set\|unset\|raise` ; `oh history` ; `oh beads` ; alias dépréciés `oh start`, `oh audit`, `oh review`, `oh debug` |
| [Workflows](cli-workflows.fr.md) | `cli-workflows.fr.md` | `oh run` ; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive` ; `oh bundle build\|show` |
| [Briques](cli-bricks.fr.md) | `cli-bricks.fr.md` | `oh brick list\|show\|new\|edit\|publish\|history` (catalogue de briques d'équipe) |
| [Projets](cli-projects.fr.md) | `cli-projects.fr.md` | `oh project list\|add\|remove\|rename\|move\|configure` ; `oh wiki check\|pending\|stats` |
| [Déploiement (alias de migration)](cli-deploy.fr.md) | `cli-deploy.fr.md` | `oh deploy`, `oh sync` (supprimées en v5 : message de migration) |
| [Configuration](cli-config.fr.md) | `cli-config.fr.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.fr.md) | `cli-infra.fr.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.fr.md) | `cli-mcp.fr.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Équipe](cli-team.fr.md) | `cli-team.fr.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Outils](cli-tools.fr.md) | `cli-tools.fr.md` | `oh skill check`, `oh eval`, `oh worktree *`, `oh secrets *`, `oh metrics`, `oh dashboard`, `oh board`, `oh version`, `oh completion` |

Commandes internes (masquées de l'aide, non documentées en détail) : `oh daemon run` (démon `ohd`, lancé automatiquement par oh), `oh runner install|run` (côté CI de l'exécution distante, utilisé par le pipeline `oh-runner`), `oh mcp serve workflow` (serveur MCP `workflow` injecté dans chaque paquet de session), `oh mcp serve wiki` (serveur MCP `wiki` du projet).

---

## Codes de sortie

| Code | Signification |
|------|--------------|
| `0` | Succès. Aussi pour `oh deploy` / `oh sync` (message de migration), et pour `oh doctor` quand aucun contrôle n'échoue (les avertissements ne comptent pas) |
| `1` | Erreur : le message est écrit sur la sortie d'erreur. Par exemple : `oh workflow validate` ou `oh skill check` qui trouvent des erreurs, `oh run --headless` dont une session attend une décision ou dépasse `--timeout`, `oh doctor` quand un contrôle échoue (✗) |
| code de `bd` | `oh beads` transmet le code de sortie de `bd` |
| `2` | Erreur interne (panic) : oh affiche un message invitant à signaler le bogue |
