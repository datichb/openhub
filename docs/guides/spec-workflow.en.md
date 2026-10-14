> [Lire en français](spec-workflow.fr.md)

# Writing a spec with the `spec` workflow

The `spec` workflow turns a request into a spec written in the project wiki (`docs/wiki/specs/<topic>.md`), then into Beads tickets, without implementing it. oh checks the spec before you approve it: a glossary, a test point for each criterion, a breakdown that covers every criterion and nothing else.

```bash
oh run spec -i request="Limit to 3 the current reservations per member"
```

In short:

1. the planner explores the code and asks you its questions (one per decision);
2. **`cp-understanding`**: you approve its restatement and its vocabulary (each word of the request against the names of the code);
3. it writes the spec: goal, scope, glossary, acceptance criteria with their test point, breakdown into slices;
4. **`cp-converge`**, without a pause: oh checks the spec and sends what is missing back to the planner;
5. **`cp-spec`**: you approve the spec, the planner commits it (that file alone);
6. **`cp-tickets`**: you approve the breakdown, the planner creates one ticket per slice.

In semi-auto, three pauses: the understanding, the spec, the breakdown.

## What the spec holds

```markdown
## Glossaire
- **Réservation** : waiting list on a book already lent — code: `Reservation`; not to be confused with `Reserve`, which creates a loan

## Critères d'acceptation
- **CA-1** — a 4th current loan is refused with a clear message · Test : `TestReserve_LimitReached`

## Découpage
- **T1** — cap of the loans per member · Couvre : CA-1 · Agent prévu : developer (backend)
```

Each criterion carries an id (`CA-1`) and, on its line, the test that will check it (its name, or the command). Each slice cites the criteria it covers. This is what oh checks at `cp-converge`, `cp-spec` and `cp-tickets` ([`verify.spec`](../reference/workflow-schema.en.md#verify--verification-run-by-oh)): a criterion without a test point, a criterion no slice covers or a slice outside the spec refuse the checkpoint, and the planner fixes it before the spec is shown to you. Headings in English (`Glossary`, `Acceptance criteria`, `Breakdown`) are read too.

## What the planner may write

The planner stays read-only; the workflow only opens `docs/wiki/specs/**` to it (field [`writes`](../reference/workflow-schema.en.md#writes-documentation-paths)): it creates and edits its spec, and commits it alone (`git commit -- docs/wiki/specs/<topic>.md`) once `cp-spec` is passed. Any other file, your work in progress included, is neither changed nor committed. A team may narrow these paths (`docs/wiki/specs/team/**`), never widen them.

## Changing the spec on the way

Reject `cp-spec` or `cp-tickets` with a note (“add a rule: …”, “add a slice for the CI”): the planner edits the file, which goes again through `cp-converge`, `cp-spec` (new commit) and `cp-tickets`. A requested slice that covers no criterion is refused by oh until the spec has the matching criterion.

## Next: `implement-spec`

oh records the checked spec and the created tickets itself (the agents have nothing to declare). When you close the client of the session, oh shows these outputs and the next step it proposes, without launching it:

```
Session outputs:
  spec : docs/wiki/specs/reservations.md
  tickets : bd-12,bd-13
→ Proposed next: implement-spec — Implement a spec
    oh run implement-spec --parent ses_… --tickets bd-12,bd-13 -i spec=docs/wiki/specs/reservations.md
```

`oh session next <session>` shows this proposal again; in the TUI, it comes first in “Chain with…” (key `e` of the Sessions view). `oh run implement-spec` without tickets also takes the tickets and the spec of the last `spec` session of the project. `implement-spec` processes the tickets one after the other (`cp-plan`, one `cp-2` per ticket, review of the whole at `cp-final`): see [Shipped workflows](../reference/workflows.en.md#implement-spec).

## What about `cadrage`?

`cadrage` is now an alias of `spec`: `oh run cadrage` launches `spec`. When the request compares designs or asks for an analysis of the code, the pathfinder explores it first, as in the former `cadrage`. Details: [Shipped workflows](../reference/workflows.en.md#spec).
