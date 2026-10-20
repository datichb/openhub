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
| [Projets](cli-projects.fr.md) | `cli-projects.fr.md` | `oh project list\|add\|remove\|rename\|move\|configure` ; `oh mr create` ; `oh wiki check\|pending\|stats` |
| [Déploiement (alias de migration)](cli-deploy.fr.md) | `cli-deploy.fr.md` | `oh deploy`, `oh sync` (supprimées en v5 : message de migration) |
| [Configuration](cli-config.fr.md) | `cli-config.fr.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.fr.md) | `cli-infra.fr.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.fr.md) | `cli-mcp.fr.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Équipe](cli-team.fr.md) | `cli-team.fr.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Outils](cli-tools.fr.md) | `cli-tools.fr.md` | `oh skill check`, `oh eval`, `oh worktree *`, `oh secrets *`, `oh metrics`, `oh dashboard`, `oh board`, `oh version`, `oh completion` |

Commandes internes (masquées de l'aide, non documentées en détail) : `oh daemon run` (démon `ohd`, lancé automatiquement par oh), `oh runner install|run` (côté CI de l'exécution distante, utilisé par le pipeline `oh-runner`), `oh mcp serve workflow` (serveur MCP `workflow` injecté dans chaque paquet de session), `oh mcp serve wiki` (serveur MCP `wiki` du projet).

---

## Codes de sortie

Les codes sont définis en un seul endroit (`cli/cmd/exitcodes.go`). Un script peut les tester.

| Code | Constante | Signification |
|------|-----------|---------------|
| `0` | `ExitOK` | Succès. Pour `oh doctor`, aussi quand seuls des avertissements (⚠) sont affichés : les contrôles optionnels (moteur de conteneurs, `bd`, accès aux passerelles depuis un conteneur) n'échouent jamais la commande, sauf si `execution.runtime = container` rend le conteneur obligatoire |
| `1` | `ExitFailure` | Échec : le message est écrit sur la sortie d'erreur. Par exemple : `oh workflow validate` ou `oh skill check` qui trouvent des erreurs, `oh run --headless` dont une session échoue ou dépasse `--timeout`, `oh doctor` quand un contrôle obligatoire échoue (✗), erreur interne (panic : oh invite à signaler le bogue) |
| `2` | `ExitUsage` | Mauvais usage : commande ou option inconnue, ou commande supprimée (`oh deploy`, `oh sync` : le message de migration est affiché) |
| `3` | `ExitPending` | En attente : `oh run --headless` dont une session attend une décision (checkpoint) et que personne ne peut prendre sans interface. La session reste en cours : `oh session approve <id>` ou `oh session answer <id>`. Si une autre session a échoué, le code est `1` |
| code de `bd` | | `oh beads` transmet le code de sortie de `bd` |

Contrôles de `oh doctor` : un contrôle obligatoire (système, `git`, configuration, base de données, outil des sessions, intégrité des workflows…) qui échoue donne `1`. Un contrôle optionnel qui échoue est affiché comme un avertissement et ne change pas le code.

## Options courtes

Une lettre a un seul sens dans toutes les commandes. Les anciennes formes restent acceptées pendant une version, avec un avertissement, et n'apparaissent plus dans l'aide.

| Lettre | Sens | Commandes | Ancienne forme (dépréciée) |
|--------|------|-----------|-----------------------------|
| `-a` | `--agent` | `oh run`, `oh start` | |
| `-A` | `--assignee` | `oh start` | |
| `-b` | `--base` (branche de base) | `oh worktree cleanup` | `oh review -b` (maintenant `--branch`) |
| `-d` | `--path` (dossier) | `oh project add` | `oh metrics -d` (maintenant `--period`) |
| `-f` | `--force` | `oh project remove`, `oh teams remove`, `oh worktree remove`, `oh worktree cleanup`, `oh service remove` | |
| `-h` | `--help` | toutes | |
| `-i` | `--input` (entrée du workflow) | `oh run` | `oh debug -i` (maintenant `--issue`) |
| `-l` | `--language` | `oh project add`, `oh project configure` | `oh start -l` (maintenant `--label`) |
| `-m` | `--message` : un texte (la demande de `oh run`, la note de publication, le message transmis à l'agent). `--prompt` de `oh start` est la demande | `oh run`, `oh brick publish`, `oh workflow publish`, `oh workflow archive`, `oh session approve`, `oh team notify test`, `oh start` | `oh review -m` (maintenant `--mode`), `oh project configure -m` (maintenant `--model`) |
| `-n` | `--name` | `oh project add` | |
| `-o` | `--output` | `oh export`, `oh session trace` | |
| `-p` | `--project` | toutes les commandes qui visent un projet, `oh` | `oh serve -p` (maintenant `--port`) |
| `-P` | `--provider` | `oh run`, `oh bundle build\|show`, `oh project configure`, `oh start` | |
| `-r` | `--resume` | `oh start` | |
| `-s` | `--status` | `oh project list` | |
| `-t` | `--ticket` | `oh start` | `oh audit -t` (maintenant `--type`) |
| `-v` | `--verbose` | toutes | |
| `-w` | `--workflow` | `oh config model show` | `oh start -w` (maintenant `--worktree`) |
| `-y` | `--yes` (sans confirmation) | `oh doctor`, `oh gc`, `oh mcp trust`, `oh migrate deploy-cleanup`, `oh project add`, `oh session delete`, `oh session resolve`, `oh brick publish`, `oh workflow publish\|archive\|restore` | |

Une ancienne forme reste acceptée pendant une version : elle fait la même chose que la nouvelle, affiche un avertissement (« L'option -p de oh serve est dépréciée, utilisez --port. ») et disparaît de l'aide. Les formes courtes de `oh start`, `oh audit`, `oh review` et `oh debug` (alias dépréciés de `oh run`) suivent la même règle.

**Ouvrir une fenêtre.** Une seule option, `--attach` (`auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, et `none` pour `oh run`), dans `oh run` comme dans `oh session attach`. L'ancien nom `--how` de `oh session attach` reste accepté (avertissement), de même que la valeur `here`, ancien synonyme de `suspend`.
