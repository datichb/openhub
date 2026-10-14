> [Lire en français](sweep-mode.fr.md)

# Sweep mode — replaced in v5

The former sweep mode (`oh start --sweep`, one session and one worktree per subtask) was removed with opencode V1 (v5).

v5 equivalent: the **`sweep`** workflow: the `pathfinder` splits the goal (read-only), the `conductor` has you validate the split (`cp-plan`, in every mode), launches the subtasks to the developer agents in the same session (in parallel only for disjoint scopes), has the result verified and reviewed by the `reviewer`, then has it committed after your approval (`cp-commit`).

```bash
oh run sweep -i goal="move logs to slog" -i strategy=by-package -i verify=tests
```

`oh start --sweep <goal>` (and `--sweep-strategy`, `--sweep-tasks`, `--sweep-include`, `--sweep-exclude`, `--sweep-verify`, `--sweep-verify-cmd`, `--sweep-dry-run`) remains a deprecated alias that fills these inputs. `--sweep-branch-prefix` and `--max-sessions` no longer have any effect.

**Fan-out (experimental)**: with `[fanout] experimental = true`, `oh run sweep … -i fanout=true` brings back one session and worktree per subtask (`sweep-task` session), merged by oh into an integration branch and validated together at `cp-merge`. In both versions, oh reads and checks the breakdown presented at `cp-plan`. See the [fan-out guide](fan-out.en.md#fan-out-sweep).

See [Shipped workflows](../reference/workflows.en.md#sweep) and the [v5 migration guide](migration-v5.en.md).
