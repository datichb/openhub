> [Read in English](cli.en.md)

# Référence CLI

`oh` sans commande ouvre la TUI (dans un terminal interactif ; sinon l'aide). `oh -p <projet>` ouvre la TUI directement sur ce projet. `oh help` (ou `oh --help`) affiche la vue d'ensemble des commandes : une ligne par commande de premier niveau, par section, les commandes avancées (`bundle`, `fanout`, `migrate`, `takeover-brief`, `completion`) à part (une soixantaine de lignes ; construite à partir des commandes elles-mêmes). `oh help --all` (ou `oh --help --all`) affiche la liste complète : toutes les sous-commandes visibles avec leurs options. `oh <commande> --help` (ou `oh help <commande>`) détaille une commande (flags, exemples). Toute l'aide suit la langue de l'interface (`oh config language`).

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
| [Sessions](cli-sessions.fr.md) | `cli-sessions.fr.md` | [Gestion des sessions (v5)](cli-sessions.fr.md#gestion-des-sessions-v5) : `oh session list\|inbox\|attach\|follow\|approve\|answer\|dismiss\|send\|interrupt\|compact\|model\|fork\|results\|trace\|resume\|stop\|delete\|open\|fetch\|resolve` ; `oh budget show\|set\|unset\|raise\|report` ; `oh history` ; `oh beads`  |
| [Workflows](cli-workflows.fr.md) | `cli-workflows.fr.md` | `oh run` ; `oh workflow list\|show\|validate\|new\|edit\|diff\|publish\|history\|restore\|archive` ; `oh bundle build\|show` |
| [Briques](cli-bricks.fr.md) | `cli-bricks.fr.md` | `oh brick list\|show\|new\|edit\|publish\|history` (catalogue de briques d'équipe) |
| [Projets](cli-projects.fr.md) | `cli-projects.fr.md` | `oh project list\|add\|remove\|rename\|move\|configure` ; `oh mr create` ; `oh wiki check\|pending\|stats` |
| [Déploiement (alias de migration)](cli-deploy.fr.md) | `cli-deploy.fr.md` | `oh deploy`, `oh sync` (supprimées en v5 : message de migration) |
| [Configuration](cli-config.fr.md) | `cli-config.fr.md` | `oh config list\|get\|set\|unset\|path\|language\|websearch`, `oh config model *`, `oh provider setup` |
| [Infrastructure](cli-infra.fr.md) | `cli-infra.fr.md` | `oh init`, `oh doctor`, `oh status`, `oh repair`, `oh export`, `oh import`, `oh purge`, `oh upgrade oh`, `oh migrate deploy-cleanup`, `oh daemon status\|stop`, `oh remote setup\|status`, `oh serve` |
| [MCP](cli-mcp.fr.md) | `cli-mcp.fr.md` | `oh mcp list\|status\|enable\|disable\|reset\|setup\|serve` |
| [Équipe](cli-team.fr.md) | `cli-team.fr.md` | `oh team *`, `oh teams *`, `oh conventions check`, `oh patterns *`, `oh policies *`, `oh takeover-brief *` |
| [Outils](cli-tools.fr.md) | `cli-tools.fr.md` | `oh skill check`, `oh eval`, `oh worktree *`, `oh secrets *`, `oh metrics` (déprécié), `oh dashboard` (déprécié), `oh board`, `oh version`, `oh completion` |

Commandes internes (masquées de l'aide, non documentées en détail) : `oh daemon run` (démon `ohd`, lancé automatiquement par oh), `oh runner install|run` (côté CI de l'exécution distante, utilisé par le pipeline `oh-runner`), `oh mcp serve workflow` (serveur MCP `workflow` injecté dans chaque paquet de session), `oh mcp serve wiki` (serveur MCP `wiki` du projet).

---

## Codes de sortie

Les codes sont définis en un seul endroit (`cli/cmd/exitcodes.go`). Un script peut les tester.

| Code | Constante | Signification |
|------|-----------|---------------|
| `0` | `ExitOK` | Succès. Pour `oh doctor`, aussi quand seuls des avertissements (⚠) sont affichés : les contrôles optionnels (moteur de conteneurs, `bd`, accès aux passerelles depuis un conteneur) n'échouent jamais la commande, sauf si `execution.runtime = container` rend le conteneur obligatoire |
| `1` | `ExitFailure` | Échec : le message est écrit sur la sortie d'erreur. Par exemple : `oh workflow validate` ou `oh skill check` qui trouvent des erreurs, `oh run --headless` dont une session échoue ou dépasse `--timeout`, `oh doctor` quand un contrôle obligatoire échoue (✗), erreur interne (panic : oh invite à signaler le bogue) |
| `2` | `ExitUsage` | Mauvais usage : commande ou option inconnue, ou commande supprimée (`oh deploy`, `oh sync`, et en 5.1 `oh start`, `oh audit`, `oh review`, `oh debug`, `oh service`, `oh takeover-brief enrich` : le message de migration est affiché). Aussi une confirmation demandée sans terminal et sans `--yes` : l'action est refusée (`workflow publish\|archive\|restore`, `brick publish`, `purge`, `project remove`, `teams remove`, `worktree cleanup`, `session delete\|resolve`, `migrate deploy-cleanup`, `run --recap`) |
| `3` | `ExitPending` | En attente : `oh run --headless` dont une session attend une décision (checkpoint) et que personne ne peut prendre sans interface. La session reste en cours : `oh session approve <id>` ou `oh session answer <id>`. Si une autre session a échoué, le code est `1` |
| code de `bd` | | `oh beads` transmet le code de sortie de `bd` |

Contrôles de `oh doctor` : un contrôle obligatoire (système, `git`, configuration, base de données, outil des sessions, intégrité des workflows…) qui échoue donne `1`. Un contrôle optionnel qui échoue est affiché comme un avertissement et ne change pas le code.

## Options courtes

Une lettre a un seul sens dans toutes les commandes. Les anciennes formes dépréciées en 5.0 ont été retirées en 5.1 : une option retirée donne l'erreur d'option inconnue (`unknown shorthand flag: 'p' in -p`, code de sortie 2).

| Lettre | Sens | Commandes |
|--------|------|-----------|
| `-a` | `--agent` | `oh run` |
| `-b` | `--base` (branche de base) | `oh worktree cleanup` |
| `-d` | `--path` (dossier) | `oh project add` |
| `-f` | `--force` (outrepasser une sécurité) | `oh worktree remove`, `oh worktree cleanup` |
| `-h` | `--help` | toutes |
| `-i` | `--input` (entrée du workflow) | `oh run` |
| `-l` | `--language` | `oh project add`, `oh project configure` |
| `-m` | `--message` : un texte (la demande de `oh run`, la note de publication, le message transmis à l'agent) | `oh run`, `oh brick publish`, `oh workflow publish`, `oh workflow archive`, `oh session approve`, `oh team notify test` |
| `-n` | `--name` | `oh project add` |
| `-o` | `--output` | `oh export`, `oh session trace` |
| `-p` | `--project` | toutes les commandes qui visent un projet, `oh` |
| `-P` | `--provider` | `oh run`, `oh bundle build\|show`, `oh project configure` |
| `-s` | `--status` | `oh project list` |
| `-v` | `--verbose` | toutes |
| `-w` | `--workflow` | `oh config model show` |
| `-y` | `--yes` (sans confirmation) | `oh doctor`, `oh gc`, `oh mcp trust`, `oh migrate deploy-cleanup`, `oh project add`, `oh session delete`, `oh session resolve`, `oh brick publish`, `oh workflow publish\|archive\|restore`, `oh run`, `oh purge`, `oh project remove`, `oh teams remove`, `oh worktree cleanup` |

Les formes courtes dépréciées en 5.0 n'existent plus : `oh serve -p` (maintenant `--port`), `oh metrics -d` (`--period`), `oh project configure -m` (`--model`), `oh session attach --how` (`--attach`). Les noms longs ne changent pas. Les options courtes de `oh start`, `oh audit`, `oh review` et `oh debug` sont parties avec ces commandes (voir [Migration depuis la 5.0](../guides/migration-v5.fr.md#depuis-la-50-vers-la-51)).

`--force` ne sert plus à sauter une confirmation : c'est `--yes`. Sur `oh purge`, `oh project remove` et `oh teams remove`, `--force` et `-f` ont été retirés en 5.1 (ils étaient des alias dépréciés de `--yes`). `--force` ne subsiste que là où il outrepasse une sécurité : `oh daemon stop` (arrêter malgré des sessions en cours), `oh remote setup` (remplacer un `.gitlab-ci.yml` écrit à la main), `oh worktree remove` et `oh worktree cleanup` (supprimer aussi les worktrees qui ont des modifications non validées). De même, `oh repair --auto` a été retiré : `--yes` confirme la restauration.

**Déprécié en 5.1 (retiré en v6).** `oh dashboard` et `oh metrics` fonctionnent encore, masqués de l'aide, et avertissent sur la sortie d'erreur (`« oh dashboard » est déprécié et sera retiré en v6 : utilisez la vue « Métriques » du TUI (oh, puis Ctrl+P, Métriques)`) dans la langue du hub. `oh dashboard` ouvre la TUI sur cette vue ; `oh metrics` garde sa sortie texte sur la sortie standard jusqu'à la v6.

**Ouvrir une fenêtre.** Une seule option, `--attach` (`auto`, `iterm`, `terminal`, `tmux`, `browser`, `suspend`, et `none` pour `oh run`), dans `oh run` comme dans `oh session attach`. L'ancien nom `--how` de `oh session attach` et la valeur `here` (ancien synonyme de `suspend`) ont été retirés en 5.1.

**Voir sans lancer.** `oh run --dry-run` prépare le lancement (workflow résolu, paquet construit, sessions planifiées), affiche le récapitulatif, les pauses et les avertissements, puis s'arrête (« dry-run : rien n'a été lancé », code de sortie 0) : aucune session, aucun worktree, aucune réservation de ticket, aucune question. Voir [`oh run`](cli-workflows.fr.md#oh-run).

**Annuler une session.** Il n'y a pas de commande « annuler » : [`oh session stop`](cli-sessions.fr.md#oh-session-stop) l'arrête en gardant son travail, [`oh session delete`](cli-sessions.fr.md#oh-session-delete) la supprime (worktree, branche locale, fichiers).

## Symboles et couleurs

Un symbole désigne une seule chose, dans la CLI (`oh session list`, `oh session inbox`) comme dans la TUI (vue Sessions, badge de la barre de mode, sections des accueils). Les deux lisent la même table (`cli/internal/glyph`).

**État d'une session**

| Symbole | État | Sens |
|---------|------|------|
| `◓` | `preparing` | le paquet de session se prépare |
| `…` | `queued` | en file d'attente (limite de sessions) |
| `●` | `active` | l'agent travaille |
| `◐` | `waiting` | attend une décision de votre part |
| `○` | `idle` | tour terminé, en attente de consigne |
| `◌` | `sleeping` | en veille (le serveur est arrêté, la session se reprend) |
| `✔` | `completed` | terminée |
| `⊗` | `failed` | en échec |
| `■` | `stopped` | arrêtée par l'utilisateur |

**Décision en attente**

| Symbole | Type | Sens |
|---------|------|------|
| `⏸` | checkpoint | point de passage d'un workflow à valider |
| `?` | question | l'agent pose une question |
| `!` | permission | une action demande votre accord |
| `$` | budget | une restriction est atteinte |
| `✗` | erreur | l'agent a rencontré une erreur à traiter |
| `⊘` | coupe-circuit | trop d'appels de sous-agents sans vous : la délégation est refusée jusqu'à ce que vous l'acquittiez |


**Événement d'équipe** (`oh team activity`, vues Activité et Statut d'équipe : `cli/internal/glyph`, `Events`)

| Symbole | Événements |
|---------|------------|
| `●` | `claim.taken` |
| `○` | `claim.released`, `workflow.archived` |
| `→` | `claim.transferred` |
| `✗` | `claim.conflict`, `review.rejected`, `wiki.rejected` |
| `✔` | `session.complete`, `review.approved`, `wiki.accepted`, `workflow.published` |
| `◐` | `review.ready` (attend une personne) |
| `⚠` | `audit.finding` |
| `✎` | `wiki.proposal` |
| `↺` | `workflow.restored` |

La phrase après l'acteur est la même dans la CLI et la TUI (`Event.Describe`).

Le badge de la barre de mode, `● N  ◐ M`, compte les sessions vivantes (`●`) et les décisions en attente (`◐`). Ailleurs, `✓` marque un retour de commande réussi, `✗` un retour en erreur, `⚠` ou `!` un avertissement.

**Couleurs.** `NO_COLOR` (toute valeur non vide, voir [no-color.org](https://no-color.org)) supprime les couleurs de la CLI (aide, tableaux, formulaires, journaux) et de la TUI ; les symboles ci-dessus suffisent à lire l'état. Sans `NO_COLOR`, le terminal décide (sortie redirigée = sans couleur). Il n'y a pas d'option `--no-color`.
