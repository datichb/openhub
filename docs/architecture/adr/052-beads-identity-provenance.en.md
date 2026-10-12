> [Lire en français](052-beads-identity-provenance.fr.md)

# ADR-052 — Beads Identity and Provenance: `--readonly`, Actor, Provenance Events

## Status

Accepted

## Date

2026-10-12

## Context

The Beads gateway ([ADR-046](046-beads-gateways.en.md)) checks each `bd` command against the workflow allow-list (`beads.allow`), with the command and flag table of bd 1.3.1 (`bdargs`). A review of bd 1.3.1 against what oh uses found four gaps:

- **One layer only.** If the table classified a command wrongly, or a newer bd changed a command, nothing would stop a write in a read-only session.
- **"Read-only" was not read-only.** The default list (no `beads:` block) holds `comments`, and a one-word entry grants all its subcommands: `bd comments add` passed. `ready` let `ready --claim` through the same way. The validator of `risk: read` used a blacklist (`BeadsWriteCommands`) that missed `note`, `assign`, `tag`, `priority`, `defer`, `todo`, `batch`, `import`, `kv`, `remember`, `set-state`, `heartbeat`, `unclaim`, `rename`, `link`, `config`, `provenance`…
- **No identity.** Every write was recorded under bd's default actor (git `user.name`): the history of a ticket could not tell an agent from the person, nor which agent.
- **No trace of the work.** Nothing bound a ticket to the session that claimed it, the commit it was closed on, or the merge request of a remote job; only notes and `external_ref`.

bd 1.3.1 offers what is missing: a global `--readonly` flag (bd refuses writes), `--actor` (recorded in `bd history --events`), and `bd provenance record`, an append-only log of events bound to a ticket (`claim`, `commit`, `handoff`… with a `git-sha`, `pr`, `work-id` reference), idempotent on `source:issue:kind:ref`.

## Decision

### 1. One list of read commands

`bdargs` holds a closed list of the bd commands that never write (`show`, `list`, `ready`, `search`, `query`, `count`, `children`, `comments`, `dep tree`, `label list-all`, `provenance log`, `history`…; `ready --claim` is a write). A command missing from the list counts as a write.

- **Read-only session**: a session whose allow-list names only read commands (the default list included) only runs commands of that list. `comments add` and `ready --claim` are refused (`cmd.gateway.beads.read_only`), on the machine and when a remote journal is replayed.
- **`risk: read`**: every `beads.allow` entry must name a read command (the blacklist is gone). `risk: plan` uses the same rule to tell whether the workflow writes.
- The default list gains `query`.

### 2. `--readonly` on every read

The gateway adds `--readonly` to every read command it runs (and to its own `bd statuses`), whatever the workflow. Writes are not changed. In a remote job, the journal mode of the fake bd refuses a write that carries `--readonly`, and a read it cannot answer from the snapshot (`query`…) fails instead of being recorded as a write.

`--readonly` is a **second layer**: bd 1.3.1 still lets `config set`, `hooks install`, `backup` and `import` through in that mode (tested). The allow-list and the table stay the first layer.

### 3. Actor set by oh

Every write run by the gateway carries `--actor=<person> via oh/<agent>`:

- **person**: git `user.name` of the session location (else the login name), fixed when the grant is issued (`Grant.Human`, signed with the grant file);
- **agent**: the entry agent of the session, or the subagent of a sub-session (`subagent` while its name is not known yet).

**Claims** (`update --claim`, `--status in_progress`, `ready --claim`) keep the person alone: bd makes the actor of a claim the assignee, and the board, `ready --assignee` and the tracker sync rely on it. The agent of a claim is in its provenance event.

Agents cannot choose the actor: `--actor` joins the refused flags. On replay, the machine rewrites the actor of each entry: its own person, the agent recorded by the job, marked `(remote)`; a replayed claim keeps bd's own actor.

### 4. Provenance recorded by oh only

oh records events with `bd provenance record --source=oh`. Agents never do: `provenance record` is a write that no oh workflow allows.

| oh event | `--kind` | `--ref-kind` / `--ref` | payload |
|---|---|---|---|
| Claim through the gateway | `claim` | `work-id` / session id | workflow |
| Close through the gateway (after the lock) | `commit` | `git-sha` / HEAD | session, branch |
| Remote session sent | `claim` | `work-id` / session id | runtime, workflow, pipeline |
| Remote session fetched, commit pushed | `commit` | `git-sha` / pushed commit | session, branch |
| Remote session fetched, draft MR | `handoff` | `pr` / MR URL | session |

A failure is only logged: the operation it describes already ran. The job daemon records nothing (its bd is a journal); the machine records the job's events when it fetches the session.

### 5. Leases kept by the daemon

A bd 1.3.1 claim carries a 5-minute lease, refreshed by `bd heartbeat`; once it expires, `bd reclaim` (run by anyone) gives the ticket back to the pool. The daemon refreshes every minute, with the person of the grant (owner of the lease), the leases of the tickets each open session claimed through the gateway or lists as claimed and not closed in its workflow state. It stops when the session ends or bd refuses (ticket closed, reclaimed, claimed by someone else).

### 6. No outside model

`bd find-duplicates --method ai` sends the tickets to an outside model (Anthropic-compatible API): the gateway refuses any method other than the default mechanical one, as for any data leaving the closed world.

## Consequences

### Positive

- A read-only session cannot write to Beads, even through a command the table does not know well: bd refuses it too.
- The history of a ticket tells who changed it: the person and the oh agent, remote runs marked.
- A ticket is bound to its session, its commit and its merge request (`bd provenance log <id>`), without parsing notes.
- One read/write classification for the gateway, the validator and the replay.

### Negative / Trade-offs

- **Behaviour change**: a workflow without `beads:` block can no longer add comments; `risk: read` workflows that allowed a write command other than the old blacklist become invalid.
- The read list is maintained by hand next to the generated table; a new read command of a newer bd is a write until it is added.
- `bd provenance record` costs one more `bd` call per claim and close.
- `git-sha` references must be 40 hex characters: a SHA-256 repository records no `commit` event (logged).

## Alternatives considered

| Alternative | Rejected because |
|-------------|------------------|
| `--readonly` on every command of a read-only session | Equivalent once a read-only session only runs read commands; `--readonly` on reads alone also covers sessions that write |
| Actor `<agent>` alone | Loses the person; a claim would assign the ticket to the agent |
| Same actor for claims | Changes the assignee of claimed tickets (board, `ready --assignee`, tracker sync) |
| Let agents record provenance | An agent could bind a ticket to any commit or MR; the facts are known by oh |
| `bd serve` (HTTP API) instead of the CLI | Needs a Dolt server (oh projects use embedded Dolt), no auth by default on loopback, refuses `--readonly`; studied for later |
