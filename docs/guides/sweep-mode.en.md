> [Lire en français](sweep-mode.fr.md)

# Sweep mode — replaced in v5

The former sweep mode (`oh start --sweep`, one session and one worktree per subtask) was removed with opencode V1 (v5).

v5 equivalent: the **`sweep`** workflow: the `conductor` splits the goal, has you validate the split (`cp-plan`), launches the subtasks in parallel to the developer agents in the same session, then verifies the result.

```bash
oh run sweep -i goal="move logs to slog" -i strategy=by-package -i verify=tests
```

`oh start --sweep <goal>` (and `--sweep-strategy`, `--sweep-tasks`, `--sweep-include`, `--sweep-exclude`, `--sweep-verify`, `--sweep-verify-cmd`, `--sweep-dry-run`) remains a deprecated alias that fills these inputs. `--sweep-branch-prefix` and `--max-sessions` no longer have any effect.

See [Shipped workflows](../reference/workflows.en.md#sweep) and the [v5 migration guide](migration-v5.en.md).
