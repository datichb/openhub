> [Lire en français](055-checkpoint-approver.fr.md)

# ADR-055 — Human approver of a checkpoint (`approver:`, CP-R9), step 1

## Status

Accepted (step 1; step 2 to come)

## Date

2026-10-15

## Context

`by:` says which **agent** passes a checkpoint (CP-R11). Nothing said which **human** must validate it: in a team, any member who has the session at hand validates `cp-apply` or `cp-publish`, and the history does not say who did. The modernisation report (CP-R9) asks for an approver per checkpoint (for instance a lead for `cp-apply` and `cp-publish`), declared by the team.

Two limits frame the decision:

- **On the machine, oh cannot enforce an approver.** The person at the machine is the one running oh: oh only knows what git and the system tell it (`user.name`, `user.email`, login name), which that person can change. Refusing a validation would be a sham protection.
- **The identity of a person already exists**: [ADR-052](052-beads-identity-provenance.en.md) sets the author of Beads writes to the git `user.name` of the session location (else the login name).

## Decision

### 1. Field `checkpoints.<id>.approver`

```yaml
checkpoints:
  cp-apply:
    approver: [Grace Hopper, lead@example.com]   # or one identity: approver: Grace Hopper
```

- **Format**: one identity or a list of identities (at most 20). An identity is a name (git `user.name`, the identity of ADR-052) or an e-mail address (`user.email`, recognised by the `@`), on one line, at most 100 characters. Compared ignoring case and extra spaces; an entry with `@` is compared with the e-mail, the others with the name.
- **No role**: team-state has no directory of roles; a role ("lead") is written as the list of its members. A named role will come with a directory, if there is one.
- **Team layer only**: the field is accepted only from a document of the `team` layer (published or team draft). The hub (shipped workflows), a project layer and the launch options do not write it (`checkpoint_approver_layer`, error).
- **Only hardens**: a project layer neither removes nor changes it; it does not remove the checkpoint (`approver_checkpoint_removed`) nor make it pause less in a mode (`loosening`), as for a mandatory checkpoint. A team layer extending another team document may change it. Removing an approver is a widening in the impact summary (`checkpoint_approver_removed`): a draft that does it is refused at launch.
- Warnings: `checkpoint_approver_no_pause` (the checkpoint pauses in no mode: nobody will be asked), `checkpoint_approver_remote_auto` (in a remote session, the `remote: auto` policy passes it on its own), `checkpoint_approver_duplicate`.

### 2. Effect on the machine: show, warn, trace

- **Card** (TUI and `oh session approve`): "Expected approver: …". When the person who validates (identity of ADR-052, `user.name <user.email>` of the session location) is none of them: a clear warning, **no blocking**. In the TUI, `y` asks for a confirmation; the decision form repeats it; validation by batch (`oh session approve --batch`, TUI `B`) shows it for each item.
- **Tracing, always** (with or without an approver): every validation and every refusal by a human records in the checkpoint events who (`identity`), when (`at`), the expected approvers (`approver`) and, when it applies, `approver_mismatch: true`; `oh session trace` (text and JSONL) and its digest show them. A pass by the mode (`auto`) or by delegation (CP-R3) has no identity. In a remote job, the `remote: auto` policy validates under the identity `remote`: traced as out of approver.
- No migration: the checkpoint state is a JSON document.

### 3. Step 2 (not done): real blocking in a remote session

What will complete it:

- an **attested**, not declared, identity: the forge (GitLab) account answering a remote session, or a signature of the validation;
- a checkpoint with an approver that, in a remote session, passes only on the answer of an approver: `remote: auto` refused (a warning today), `remote: defer` waiting for the validation of an approver, on their machine or from the forge;
- reading the `approver_mismatch` already traced to report, when a session is fetched, a validation made out of approver.

## Consequences

### Positive

- In a team, every sensitive checkpoint says who must validate it, and the history says who validated it, when, and whether it was out of approver.
- The layer rule is the one of the team governance ([ADR-040](040-workflows-team-state-governance.en.md)): a project cannot remove an approver set by the team.
- The data of step 2 is already recorded.

### Negative / trade-offs

- **No guarantee on the machine**: anyone can change `user.name` or validate anyway; oh says it and traces it, nothing more.
- The identity is read with git at each validation (cached one minute per location).
- An identity written by hand (name or e-mail) must match the git configuration of the approver.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Refuse on the machine a validation out of approver | The identity is declared by the person themselves: a sham protection, and a team blocked when its approver is away |
| Named roles (`approver: lead`) | No directory of roles in team-state; a list of identities is enough and stays checkable |
| Approver declarable by the project | A project could remove it or replace it with itself: the rule would no longer be the team's |
| Approver in the hub | The shipped workflows do not know the people of a team |
