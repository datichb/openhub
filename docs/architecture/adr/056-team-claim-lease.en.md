> [Lire en français](056-team-claim-lease.fr.md)

# ADR-056 — Team claims: lease, heartbeat, end of session and durable queue

## Status

Accepted — evolves [ADR-024](024-team-state-repository.en.md) (team-state claims); complements [ADR-028](028-tracker-sync-bidirectional.en.md) (tracker sync) and [ADR-047](047-session-interaction-daemon.en.md) (daemon)

## Date

2026-11-02

## Context

[ADR-024](024-team-state-repository.en.md) put **claims** (ticket reservations) in the team-state: one TOML file per ticket, with an owner and an activity date. A claim was "stale" (`IsStale`) after `stale_days` days (3 by default) without activity. Three flaws showed up when teams used the mode:

- **Activity was never written**: `last_activity` was not persisted (go-toml drops a `time.Time` tagged `omitempty`), so staleness was always computed from `claimed_at`. A session that worked on one ticket for several days looked abandoned.
- **The end of a session said nothing about the claim**: the claim either stayed forever or was released while the work was unfinished or waiting for a review. The only thing that happened was a silent transfer when a stale claim was taken over.
- **Offline, the effect of the end of a session was lost**: the release or the status change was only pushed if the team repository answered at that moment.

It also had to say what "in review" means: a ticket exists in Beads (or the tracker) and in a claim, which do not have the same statuses, and the REVIEW column of the team board was not computed the same way on both sides. Beads 1.3 has no `review` status; the fake `bd` of a remote job ([ADR-046](046-beads-gateways.en.md)) replayed a status the project does not know when it came back.

## Decision

The claim gets a **lease** renewed by the daemon, and the end of a session follows a single rule, applied through a **durable queue**. The user-facing detail is in the [`team-setup`](../../guides/team-setup.en.md#lease-and-end-of-session) guide.

### 1. Lease

- Two optional fields on the claim: `lease_until` (deadline, an RFC 3339 string to the second, the `Instant` type: go-toml would drop a `time.Time` tagged `omitempty`) and `session` (the session working on it). A claim without `lease_until` (written by oh 5.0) is judged by the former inactivity rule (`IsStale`, `[takeover] stale_days`): no migration.
- `Claim.IsExpired(now, cfg)`: the lease is past if there is one, otherwise it falls back on inactivity.
- **Only a claim with an owner and work started has a lease.** A pool claim (no owner), `planned`, in the initial column or finished has none: it never expires.
- **A claim in review never expires** (amended on 2026-11-02, before 5.1): status `review`, or the `review` / `ready-for-review` label on the claim (`Claim.InReview`). It waits for a person, not for a session, so no lease can run out on it (`IsExpired` and `HasLease` answer no, whatever `lease_until` says): it stays reserved until the merge or until its owner releases it. `oh doctor` reports it when it has been there for longer than `stale_days` (`claims_review`, a warning), without calling it expired. When the review ends (status back to `in_progress`, label removed) and the lease it held is past, it gets a new lease from that moment, otherwise it would be taken over at once.
- `last_activity` is written as a native TOML date-time, not as a string: otherwise an oh 5.0 refuses the whole claim ("cannot decode TOML string"). A test reads the claim back with a 5.0 struct.
- Settings (`[claim]` of the team-state `config.toml`, shared by the team): `lease_hours` (24, 1 to 168) and `heartbeat_minutes` (60, 0 = off, 5 to 1440, at most half the lease).

### 2. Heartbeat and caps

- The daemon (`ohd`, or the `oh` process on Windows) renews every `heartbeat_minutes` the lease of the claims of **open sessions** (a sleeping session is open), in **a single commit** per team repository, and **none** while more than half the lease is left: commit noise stays low.
- **Renewal is capped**, otherwise a claim would outlive for ever a session nobody resumes: `sleeping_renewal_days` (3 by default) after the session went to sleep, `remote_renewal_days` (7) after a remote session was launched. After that the claim expires normally.
- **Hand-taken claim** (no session: `oh team claim`, the board, the tracker plan): a longer lease, `lease_hours_manual` (72, from `lease_hours` to 720), that nothing renews by itself. `oh team claim <ticket>` on one's own claim renews it (`Repo.RenewClaim`); starting a session replaces it with the session's lease (`AdoptClaim`).

### 3. End of session: `DecideClaimEnd`

A pure function (`teamstate.DecideClaimEnd`), with no clock, git or file: the caller gathers the facts (ticket, claim, checkpoints, work left) and turns the decision into queue operations. The first rule that applies:

1. claim already terminal: kept (the retention cleans it up);
2. ticket closed: claim moved to the terminal status;
3. ticket or claim in review (label `review` or `ready-for-review`, or status `review`): **kept**;
4. checkpoint waiting for a decision: kept, label `session-ended`;
5. commit checkpoint put off while the session leaves work (or that cannot be told): kept, `session-ended`;
6. checkpoint state unreadable: kept, `session-ended` (a claim released by mistake is lost, a claim kept by mistake expires with its lease);
7. otherwise: released.

A kept claim is no longer renewed by the session that ended: it expires with its lease if nobody takes it up. The exception is the claim in review (rule 3): it does not expire at all, see section 1.

### 4. Durable queue

What the end of a session decides (release, complete, mark) and the renewals that could not be pushed are first written to `.git/oh-claims-queue.json` in the team-state clone (local, never committed). Each operation carries its conditions (same member, same session); if the claim changed in the meantime the operation is dropped and logged, never applied. The queue is emptied in one commit (one pull, one push) at the end of the session, at each daemon beat and by the daemon's `claims-queue` task (at start-up, then every 15 minutes). Offline, nothing is lost. A session that starts offline on a claim already held takes it up when the network is back through an `adopt` operation, which **cancels** the queued operations of the other sessions on that claim (otherwise the old release would apply to a claim being worked on).

### 5. Expiry and takeover

An expired claim is not deleted by itself. `oh doctor` reports it (`claims_expired`, a warning) with the command to keep or release it, and reports a waiting queue (`claims_queue`, `--fix` applies it). Another member takes it over with `oh team claim <ticket>`: a **real confirmation** (`--yes` without a terminal, otherwise refused, code 2), a takeover brief generated, a new lease; the silent transfer of a stale claim is gone. `[claim] release_expired_on_sync` (off) makes the tracker sync release expired claims; a claim in review is never one of them. `oh team claim` does not take over a claim in review of another member, whatever its lease. A claim released on purpose is not recreated by the sync (`Repo.ClaimRemovedOnPurpose`, read from the git history).

### 6. Status model: REVIEW

- **In Beads, REVIEW is a label**: `in_progress` + `ready-for-review` (no `review` status in Beads 1.3, unless the project declares a custom one). **In the claim, REVIEW is the `review` status**, set by publishing a review or from the board.
- **The two are not copied onto each other.** The only equivalence is the one of `DecideClaimEnd`: at the end of a session the label (on the ticket or the claim) is worth the `review` status; either way the claim survives.
- **The fake `bd` of a remote job refuses a status the project does not configure** (code 1, before any write, exact case; the statuses travel in the snapshot, `Snapshot.Statuses`): the agent uses the label instead of failing on replay.

## Consequences

### Positive

- A claim no longer lives for ever, no longer vanishes by accident, and says so: `oh doctor` gives the command. A claim in review keeps its reservation while the review lasts, and the doctor tells when the wait is long.
- The end of a session is testable (pure function) and loses nothing offline.
- Compatible with 5.0: fields ignored by the older version, `last_activity` readable, fallback on `IsStale`.
- The team mode is no longer flagged as beta: its known limits are documented rules.

### Negative / trade-offs

- **Commit noise** in the team-state (one commit per repository for each useful renewal), bounded by the period, half the lease and the `heartbeat_minutes = 0` setting.
- **An older oh that rewrites a claim** loses `lease_until`: the claim falls back on the inactivity rule (no breakage).
- A hand-taken claim expires if it is not renewed by hand; a `sleeping` or remote session stops renewing at the cap, even if it is resumed later (`oh team claim` or a new session renews).
- A shallow clone is treated as a lost claim; the window between emptying the queue and an offline adoption stays narrow.
- A claim in review that nobody merges or releases stays reserved for ever: only `oh doctor` (`claims_review`) points it out, its owner or a `oh team release` frees it.
- Three mechanisms to understand (lease, queue, end rule) instead of one activity field.

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| Keep inactivity (`stale_days`) as the only rule | Unobservable (activity was not persisted) and too slow or too harsh depending on the team; does not tell a live session from a forgotten claim |
| A lock or central service for claims | The team-state is a git repository with no infrastructure ([ADR-024](024-team-state-repository.en.md)); a service would add a server to run |
| Always release the claim at the end of the session | Loses the claim of a ticket in review or whose work is not committed |
| Never release, let the lease expire | A ticket finished cleanly would stay reserved until expiry |
| Renew without a cap | A forgotten sleeping session or a dead remote job would hold the claim for ever |
| Push the end of the session immediately, without a queue | Lost offline or if oh is killed; the end of a session must not depend on the network |
| Copy the Beads label onto the claim status (and back) | Two sources written by two paths: divergences and conflicts on replay; the equivalence stays in one place |
