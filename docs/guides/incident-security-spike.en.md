> [Lire en français](incident-security-spike.fr.md)

# Incident, security fix and spike

Three workflows that close the loop after a problem or before a decision: `incident` stops the impact, then learns from it; `security-fix` fixes the findings of an audit; `spike` settles a technical question. Each writes what it learns in the wiki or in Beads, never next to them. Full reference: [Shipped workflows](../reference/workflows.en.md#incident).

## `incident`: mitigate first

```bash
oh run incident -i incident="Since the 14:02 deployment, payments fail (429 from the provider). Log: ops/incident.log"
```

1. The `debugger` measures the impact, dates the start and looks for the triggering change (`git log` around the start time, diff of the suspect commit).
2. It proposes the safest measure: undo that change, switch the feature off, change a setting. If it is code, the `developer` applies it **in the work tree**, without a commit. If it happens outside the repository (environment variable, rollback to the previous version), it gives you the exact command.
3. **`cp-mitigated`**: oh runs the tests, then you choose or confirm the measure. The code fix is already in place: you may ship it yourself if the incident cannot wait.
4. The debugger writes the postmortem, `docs/wiki/postmortems/<date>-<slug>.md`, **blameless**: no person's name, even if the report names one; the timeline, the impact and the detection delay are in figures and sourced.
5. **`cp-postmortem`**: you approve or ask for a change; only then are the mitigation fix and the postmortem committed, each in its own commit.
6. **`cp-actions`**: the actions of the postmortem (prevent, detect, mitigate) become tickets if you approve them; a refusal creates none.

Three pauses in semi-auto, each mandatory: an incident is not run in automatic mode.

## `security-fix`: fix findings

```bash
oh run security-fix -i findings="$(oh session report <audit session> rp-2)"
```

1. **`cp-triage`**: the received findings are numbered (`F1`, `F2`…); you say which ones to fix now.
2. The `developer` fixes only those, at the cause and for every affected input, each with a regression test, without a commit. A false positive is reported, with no change.
3. The `auditor-subagent` runs a **targeted** re-analysis: each finding is `corrigé` (fixed), `encore présent` (still present) or `contournable` (bypassable: another path, a neighbouring prefix, a symbolic link…).
4. **`cp-rescan`** (no pause in semi-auto): oh runs the tests and refuses a change without a test; a finding still present goes back to the developer (two rounds at most).
5. **`cp-2`**: you see the fixes, the tests and the verdict; the commit only comes after.

The findings are given as input today, copied from an audit report; chaining `audit → security-fix` directly will come with workflow composition.

## `spike`: settle a question

```bash
oh run spike -i question="Can csvlite be replaced by encoding/csv? go if the real statement reads without preprocessing; kill otherwise. Time box: 30 min."
```

1. **`cp-question`**: the `pathfinder` restates the question, the criteria (what would mean go or kill) and the time box; nothing starts before you approve.
2. Sourced research, criterion by criterion: the repository first, then the official documentation; each fact cites its file or its URL.
3. If a criterion can only be settled by running code, the `developer` writes a **throwaway prototype** in `.oh-spike/`, runs it, records the output and deletes it. It is never committed.
4. The decision — **go**, **clarify** (a decision or information from you is missing) or **kill** — is written in `docs/wiki/decisions/<slug>.md`, with its sources.
5. **`cp-decision`**: oh checks that `.oh-spike/` is gone; you approve or correct the decision; only the decision is committed.

## What oh checks and locks

| Workflow | Before the checkpoint | Checked by oh |
|---|---|---|
| `incident` | no commit before `cp-postmortem`, no ticket before `cp-actions`; the debugger only writes `docs/wiki/postmortems/` | stack tests at `cp-mitigated` |
| `security-fix` | no commit before `cp-2`; the developer only starts after `cp-triage` | tests and at least one changed test file, at `cp-rescan` and `cp-2` |
| `spike` | no commit before `cp-decision`; the developer only starts after `cp-question`; the pathfinder only writes `docs/wiki/decisions/` | `.oh-spike/` deleted, at `cp-decision` |

The evaluation cases S41 to S46 and S49 to S51 (`evals/incident`, `evals/security-fix`, `evals/spike`) check these behaviours with a real model.
