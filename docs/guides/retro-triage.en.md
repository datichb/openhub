> [Lire en français](retro-triage.fr.md)

# Retrospective and triage

Two workflows to learn from what happened and to put the work to do back in order: `retro` reads the trace of past sessions and draws actions from it, `triage` sorts a batch of tickets or findings without writing anything before you agree. They chain: the actions of a retrospective get triaged. Full reference: [Shipped workflows](../reference/workflows.en.md#retro).

## `retro`: learn from past sessions

```bash
oh run retro                                   # the last finished session of the project
oh run retro --parent ses_ab12                 # one session (the one the launch is chained from)
oh run retro -i sessions="ticket 7d"           # the ticket sessions of the last 7 days
oh run retro -i sessions="ses_ab12 ses_cd34" -i focus="Why was cp-2 refused so often?"
```

oh builds the summary of the trace of each session itself (6 at most, the most recent first): duration, total cost and cost per agent, checkpoints asked several times, refused decisions with your instruction, refused commands, delegations. The `debugger` only reads these summaries and, to shed light on a cause, the repository.

1. **Facts**: each fact cites its source (`ses_ab12 · Appels refusés · bash docker compose up -d db`). What the summary does not say does not exist.
2. **Findings**: the facts grouped by cause, counted and costed (retries, duration, dollars), with a cause in the system (an instruction, a permission, a step of the workflow, a convention of the project), **blameless**: no person named, even when your question names one. An unproven cause is marked as a hypothesis.
3. **Actions**: each says what changes and the sign that will show, in a later trace, that it worked ("no more refused `docker compose` in the `ticket` sessions").
4. The page is written in `docs/wiki/retros/<date>-<slug>.md` (sessions looked at, what went well, findings, actions).
5. **`cp-retro`**: you approve or ask for a correction; only then is the page committed, on its own.
6. **`cp-actions`**: the actions become tickets if you approve them; a refusal creates none.

Two pauses in semi-auto. With no session to look at, the debugger says so and gives the command to run again, without writing anything. At the end, oh shows the page and the created tickets, and proposes (without launching it) `triage` on those tickets.

To replay a retrospective on a saved summary: `oh run retro -i trace="$(cat summary.txt)"`.

## `triage`: sort without surprises

```bash
oh run triage --tickets ev-1,ev-2,ev-3          # a given batch
oh run triage -i query="status=open AND label=bug"
oh run triage -i findings="$(cat review-findings.md)"  # findings to attach or to turn into tickets
oh run triage                                   # the open tickets of the project (30 at most)
```

1. The `planner` reads the tickets, the labels of the project and the duplicate candidates of `bd find-duplicates`.
2. It triages: a priority changed only when the ticket justifies it, a duplicate only when both tickets describe the same problem at the same place, the project's labels only, a ticket that mixes several deliverables split into child tickets, a finding already covered attached by a comment. It touches neither the status, the assignee nor the description.
3. **`cp-triage`**: the table of every change (ticket, field, before → after, reason), the tickets to create and the unchanged ones. **Before you approve, oh refuses every write to Beads** (creation, priority, label, link, comment, duplicate), in the shell and in the Beads gateway. A refusal: nothing is written.
4. Once approved, exactly the changes of the table, and only those.

One pause in semi-auto. oh records the changed, created and duplicate-closed tickets itself (outputs `updated`, `created`, `duplicates`), and proposes `ticket` on the created tickets.

## For a team layer

- The `update` lock (changing tickets) is new: a team workflow may declare it on a checkpoint (`unlocks: [update]`) to show ticket changes before making them. An older oh ignores it (warning `unlock_unknown`): update oh on every machine.
- `session.traces(<input>)` computes the summaries of several sessions for a `text` input of a team workflow; see the [schema](../reference/workflow-schema.en.md).
