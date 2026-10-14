> [Read in English](sweep-mode.en.md)

# Mode sweep — remplacé en v5

L'ancien mode sweep (`oh start --sweep`, une session et un worktree par sous-tâche) a été retiré avec opencode V1 (v5).

Équivalent v5 : le workflow **`sweep`** : le `pathfinder` découpe l'objectif (lecture seule), le `conductor` valide le découpage avec toi (`cp-plan`, dans tous les modes), lance les sous-tâches vers les agents développeurs dans la même session (en parallèle seulement pour des périmètres disjoints), fait vérifier et relire le résultat par le `reviewer`, puis fait commiter après ta validation (`cp-commit`).

```bash
oh run sweep -i goal="migrer les logs vers slog" -i strategy=by-package -i verify=tests
```

`oh start --sweep <objectif>` (et `--sweep-strategy`, `--sweep-tasks`, `--sweep-include`, `--sweep-exclude`, `--sweep-verify`, `--sweep-verify-cmd`, `--sweep-dry-run`) reste un alias déprécié qui remplit ces entrées. `--sweep-branch-prefix` et `--max-sessions` n'ont plus d'effet.

**Fan-out (expérimental)** : avec `[fanout] experimental = true`, `oh run sweep … -i fanout=true` retrouve une session et un worktree par sous-tâche (session `sweep-task`), mergées par oh dans une branche d'intégration et validées ensemble à `cp-merge`. Dans les deux versions, oh relève et vérifie le découpage présenté à `cp-plan`. Voir le [guide du fan-out](fan-out.fr.md#sweep-en-fan-out).

Voir [Workflows livrés](../reference/workflows.fr.md#sweep) et le [guide de migration v5](migration-v5.fr.md).
