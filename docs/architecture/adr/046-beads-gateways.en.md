> [Lire en français](046-beads-gateways.fr.md)

# ADR-046 — Beads on the Machine and Gateways

## Status

Accepted

## Date

2026-10-05

## Context

Beads (`bd`) is the source of the project's tickets, and its database lives on the machine. oh's MCP servers (gitlab, figma, jira, gslides, linear, github, team, workflow) read their tokens from the machine keychain. Locally, the agent calls them directly.

In a container and remotely, this no longer works:

- the Beads database is not in the image;
- copying the tokens into the container would break the rule "secrets stay on the machine" (D10);
- a remote job cannot reach the machine.

An agent also had to be limited, in Beads, to what its workflow allows (`beads.allow`, [ADR-039](./039-declarative-workflows-oh-v1.en.md)).

Decision D9: Beads always stays on the machine.

- **local**: `bd` directly;
- **container**: a fake `bd` that goes through an oh gateway;
- **remote**: snapshot on the way in, journal on the way out, replayed locally with conflict detection.

## Decision

### 1. Beads gateway (container)

- **Fake `bd`** (`cmd/oh-bd`, Go, built for `linux/{amd64,arm64}`, embedded in the project image). It sends `argv`, the current directory and, only for `--stdin` or `-`, standard input, as `POST $OH_GATEWAY_URL/beads/v1/exec` with `Authorization: Bearer $OH_GATEWAY_TOKEN`. It relays stdout, stderr and the exit code.
- **Gateway** (`internal/gateway`, in the daemon, on the proxy listeners, `/oh-gateway` prefix):
  - **allowlist** = the workflow's `beads.allow`. Without a `beads:` block, read-only (`show`, `list`, `ready`, `search`, `children`, `comments`, `count`, `status`, `graph`, `history`); an empty list refuses everything;
  - global options that change the database or directory are refused anywhere in the command (`--db`, `-C`, `--global`…);
  - current directory and file paths translated to the machine and limited to the session's directories, links resolved; bundle and group data excluded;
  - the real `bd` runs without a shell, in the session's directory (2 min timeout, 8 MiB per stream).
- **`ohg_…` tokens**:
  - one per session and per sub-session, passed through the session environment;
  - the daemon only keeps their fingerprint (`~/.oh/run/gateway.json`, 0600);
  - valid while the session is open and awake, revoked with the group;
  - opencode 2.0.20 does not pass the session environment to sub-agents: the daemon applies it to each sub-session it attaches, with a token of its own.
- Locally, `bd` stays direct.

### 2. MCP gateway

- For a runtime outside the machine, the adapter declares each bundle server of the form `oh mcp serve <name>` as `type: remote`:
  - URL `…/oh/v1/hooks/mcp/<name>`;
  - header `Authorization: Bearer {env:<proxy token variable>}`, expanded by opencode: the token is never written into the config;
  - the bundle and its hash do not change.
- The daemon runs the server command **on the machine**, read again from the immutable bundle, with the current oh executable. That process reads its token from the keychain, as it does locally.
- **Generic stdio ↔ HTTP bridge**:
  - "streamable HTTP" transport with JSON responses;
  - request IDs rewritten, for several clients per process;
  - one process per group and per server, restarted if it dies and stopped with the group.
- A call whose `_meta` names a session of another group is refused (403). The `workflow` server ([ADR-042](./042-checkpoints-headless-decisions.en.md)) thus works the same way in a container.
- Other local MCP servers stay inside the container.

### 3. Remote: snapshot and journal

- **Before sending**:
  - reservation (`bd update --claim`, then team-state claim; undone if sending fails);
  - snapshot of the tickets involved, their dependencies and their children (`bd show --json`), with each ticket's **revision**;
  - the snapshot travels in the session envelope ([ADR-045](./045-execution-environments.en.md)).
- **In the job**, the fake `bd` in journal mode (`OH_BD_MODE=journal`) sits behind the gateway of the job's daemon: `beads.allow` and the refused options apply unchanged.
  - Reads are served from the snapshot.
  - Writes are recorded in `journal.jsonl` (`beadswire.JournalEntry`: order, argv, directory, standard input) and applied to the copy.
  - `create` returns a temporary `pending-<n>` ID.
- **On return**, `oh session resolve <id>` replays the journal:
  - each entry is checked again on the machine (allowlist, options, session IDs);
  - **conflict** = a ticket written by the session whose current revision differs from the snapshot's;
  - per ticket: keep the local version, apply the remote one, or merge the notes;
  - nothing is applied without a resolution and a confirmation;
  - temporary IDs are replaced; the replay can be restarted (`replay.json`).
- **During the pipeline**, the daemon keeps the Beads lease (`bd heartbeat`) of the reserved tickets alive.

## Consequences

### Positive

- A single Beads database, on the machine, whatever the execution environment.
- The workflow's allowlist applies in a container and remotely, through the same code.
- No token in the container: `env` only shows `OH_GATEWAY_URL` and `ohg_…`. oh's MCP servers work without changes.
- Remotely, no Beads change is lost or silently applied.

### Negative / Trade-offs

- Interactive `bd` (`edit`, `create-form`) is unavailable in a container and remotely (no terminal).
- MCP gateway: no server → client notifications, request body limited to 1 MiB, server stderr only kept when the process dies.
- The Linux second listener, which also serves the gateways, is only unit-tested. The gitlab/team MCP servers with real tokens were not tested in a container.
- Remotely, MCP servers that need the machine keychain are unavailable (BL-18); only `workflow` works.
- The replay relies on the revision given by `bd show` (verified on bd 1.3.1): a change in Beads' format would break it.
- Attaching sub-sessions leaves a possible race: a first shell started before the daemon sees the sub-session would not have the environment. It has not been observed.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Mount the Beads database into the container | Full access, without an allowlist; impossible remotely. |
| Copy the MCP tokens into the container | Contrary to D10: secrets would be readable by the agent. |
| Rewrite each MCP server over HTTP | A generic bridge covers every oh server without changing them. |
| Pass the gateway token in the server environment | The session shell does not inherit the server environment (F30); it goes through the session environment. |
| Apply the remote job's writes directly to the database | The job cannot reach the machine, and a concurrent write would be overwritten unknowingly. |
