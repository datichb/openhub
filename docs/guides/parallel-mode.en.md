> [Lire en français](parallel-mode.fr.md)

# Parallel mode — replaced in v5

The former parallel mode (`oh start --parallel`, one opencode server per ticket, monitor and merge view) was removed with opencode V1 (v5).

v5 equivalent: **one session per ticket, in a single server group**, each in its own worktree when it writes:

```bash
oh run ticket --tickets BD-42,BD-43,BD-44
```

- `oh start --parallel --tickets a,b` remains a deprecated alias of this command; without `--tickets`, it is refused.
- Follow the sessions in the TUI **Sessions** view or with `oh session list`; branches are merged through your merge requests.
- `--max-sessions` and `--priority` no longer have any effect.

See [v5 sessions](sessions-v5.en.md) and the [v5 migration guide](migration-v5.en.md).
