> 🇫🇷 [Lire en français](017-auditor-consolidation.fr.md)

# ADR-017 — Consolidation of auditor-* agents into a single generic auditor-subagent

## Status

Accepted

## Context

The hub had 7 `mode: subagent` agents specialized by audit domain:
`auditor-security`, `auditor-performance`, `auditor-accessibility`, `auditor-architecture`,
`auditor-privacy`, `auditor-ecodesign`, `auditor-observability`.

This architecture had several problems:

**Structural duplication (80% identical code):**
- Identical frontmatter across all 7 agents (permissions, skills, except `id`, `label`, `description`, `native_skills`)
- ~80% identical body: same preamble, same "What you do / do NOT do" rules, same 10-step workflow, same handoff pattern
- The actual difference was limited to the domain, referentials, and the specific `native_skill`

**Reliability issues detected (2026-06-18 analysis):**
- **C-2**: conflict between `posture/expert-posture` (prescribing to call `question` for critical risks) and `posture/subagent-concision-posture` (forbidding `question` in subagent mode). No priority rule. Present in all 7 agents.
- **C-5**: permission `task: { "auditor-*": allow }` in `auditor.md` uses a wildcard whose semantics are undocumented in OpenCode. If unsupported, no subagent can be invoked.
- **m-3**: `subagent-concision-posture` listed all 7 agents in its scope — manually maintained list on every add/remove.

**Asymmetry with the developer model:**
ADR-013 already consolidated specialized developer-* agents into a single generic `developer` agent. The auditors remained on the old architecture, creating an inconsistency in the hub's mental model.

## Decision

Reproduce the ADR-013 model exactly for auditors:

1. **Create a single `auditor-subagent` agent** (mode: subagent) replacing the 7 specialized agents
2. **The `auditor` coordinator injects the domain and `native_skill`** in the invocation prompt — the agent specializes based on what it receives, not by ID change
3. **The 7 specialized agents are deleted**
4. **The `task: { "auditor-*": allow }` permission is replaced** by `task: { "auditor-subagent": allow }` — explicit ID, no wildcard (resolves C-5)

### Coordinator invocation format toward `auditor-subagent`

```
task({
  subagent_type: "auditor-subagent",
  prompt: "
    [transmitted project context]
    ...
    Tu agis en tant que sous-agent d'audit [DOMAIN].
    Charge et applique le skill : auditor/audit-[DOMAIN]
  "
})
```

### Domain → native_skill table

| Domain | Native skill |
|--------|-------------|
| `security` | `auditor/audit-security` |
| `performance` | `auditor/audit-performance` |
| `accessibility` | `auditor/audit-accessibility` |
| `ecodesign` | `auditor/audit-ecodesign` |
| `architecture` | `auditor/audit-architecture` |
| `privacy` | `auditor/audit-privacy` |
| `observability` | `auditor/audit-observability` |

### C-2 resolution — `expert-posture` vs `subagent-concision-posture` priority rule

The rule is written in the body of `auditor-subagent.md`:
> In subagent mode, critical risks are surfaced via the `risks` field of the handoff block. Never call `question` — the tool is not available in this context.

### `cmd-audit.sh` evolution

The `--type` flag is preserved for CLI compatibility but its role changes:
- **Before**: agent selector (`REQUIRED_AGENTS=("auditor" "auditor-security")`)
- **After**: prompt parameter passed to the coordinator (`REQUIRED_AGENTS=("auditor")` in all cases)

## Consequences

### Positive

- **-7 agent files**: the 7 specialized agents are deleted. A single `auditor-subagent.md` to maintain.
- **Zero duplication**: common frontmatter and body are no longer duplicated 7 times.
- **C-2 resolved**: explicit priority rule in the body of the single agent.
- **C-5 resolved**: explicit `auditor-subagent` permission, no wildcard.
- **m-3 resolved**: `subagent-concision-posture` now lists only one ID in its scope.
- **Consistency with ADR-013**: same architecture as the generic `developer`.
- **Easier maintenance**: any common rule (handoff format, read-only rules, critical risks) is changed in one place.

### Negative / trade-offs

- **Loss of rich per-domain descriptions** in the OpenCode picker. The 7 agents had targeted descriptions (`"analyzes OWASP Top 10, CVE..."`, `"analyzes WCAG 2.1 AA..."`). `auditor-subagent` has a generic description. Impacts `@` picker readability — but `mode: subagent` agents don't appear in the user picker, only in coordinator `task` invocations. No practical risk.
- **`--type` CLI semantics change**: it no longer selects a deployed agent, it parameterizes a prompt. Users who manually listed deployed agents (`auditor-security.md` in `.opencode/agents/`) must deploy `auditor-subagent.md` instead.

## Rejected Alternatives

**Keep the 7 agents with added priority rules**: resolves C-2 but requires modifying 7 identical files. Does not resolve duplication or C-5. Rejected.

**Single multi-level skill** (same approach as `lite`/`subagent` levels in ADR-015): would reduce file count but doesn't eliminate agent duplication. Rejected.

## Impact

| File | Action |
|------|--------|
| `agents/auditor/auditor-subagent.md` | Created |
| `agents/auditor/auditor.md` | Modified — domain→native_skill table, `task: auditor-subagent` permission, invocation prompt |
| `agents/auditor/auditor-security.md` | Deleted |
| `agents/auditor/auditor-performance.md` | Deleted |
| `agents/auditor/auditor-accessibility.md` | Deleted |
| `agents/auditor/auditor-architecture.md` | Deleted |
| `agents/auditor/auditor-privacy.md` | Deleted |
| `agents/auditor/auditor-ecodesign.md` | Deleted |
| `agents/auditor/auditor-observability.md` | Deleted |
| `skills/auditor/auditor-workflow.md` | Modified — subagent table, Phase 3 invocation prompt |
| `skills/posture/subagent-concision-posture.md` | Modified — scope: 7 auditors → `auditor-subagent` |
| `scripts/cmd-audit.sh` | Modified — `REQUIRED_AGENTS` always `("auditor")`, scan pattern `auditor\|auditor-subagent` |
| `scripts/lib/agent-discovery.sh` | Modified — `hub_ids`: 7 auditors → `auditor-subagent` |
| `scripts/lib/prompt-builder.sh` | Modified — example comment |
| `tests/test_cmd_audit.bats` | Modified — remove `auditor-*` fixtures, add B-bis section |
| `tests/test_lib_agent_discovery.bats` | Modified — add `auditor-subagent` match tests, 7 auditors no-match |
| `tests/test_lib_prompt_builder.bats` | Modified — add `build_audit_bootstrap_prompt` tests |
| `docs/architecture/agents.fr.md` + `.en.md` | Modified |
| `docs/architecture/skills.fr.md` + `.en.md` | Modified |
| `docs/architecture/task-delegation.fr.md` | Modified |
| `docs/guides/workflows.fr.md` + `.en.md` | Modified |
| `docs/dev/fiabilisation-agents-en-cours.md` | Modified — C-2, C-5, m-3 marked resolved |
