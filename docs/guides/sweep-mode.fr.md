> [Read in English](sweep-mode.en.md)

# Mode sweep — remplacé en v5

L'ancien mode sweep (`oh start --sweep`, une session et un worktree par sous-tâche) a été retiré avec opencode V1 (v5).

Équivalent v5 : le workflow **`sweep`** : le `conductor` découpe l'objectif, valide le découpage avec toi (`cp-plan`), lance les sous-tâches en parallèle vers les agents développeurs dans la même session, puis vérifie le résultat.

```bash
oh run sweep -i goal="migrer les logs vers slog" -i strategy=by-package -i verify=tests
```

`oh start --sweep <objectif>` (et `--sweep-strategy`, `--sweep-tasks`, `--sweep-include`, `--sweep-exclude`, `--sweep-verify`, `--sweep-verify-cmd`, `--sweep-dry-run`) reste un alias déprécié qui remplit ces entrées. `--sweep-branch-prefix` et `--max-sessions` n'ont plus d'effet.

Voir [Workflows livrés](../reference/workflows.fr.md#sweep) et le [guide de migration v5](migration-v5.fr.md).
