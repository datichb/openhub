> [Lire en français](045-execution-environments.fr.md)

# ADR-045 — Execution Environments: Local, Container, Remote (GitLab CI)

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, a session always ran on the machine, under the user's account, with their tools and configuration. Three needs appeared:

- **a reproducible environment per project**, the one of the development Dockerfile, where the agent's shell has access neither to the rest of the machine nor to the daemon;
- **running an autonomous workflow without keeping the machine on** (ticket, review, audit), on the team's infrastructure;
- **keeping secrets and Beads on the machine** (D9, D10).

Decisions D11 (local or remote, chosen at each launch, remote = GitLab CI runners), D12 (OCI containers through Colima or Podman, one image per project, bundle mounted read-only) and O16 (container and remote on macOS and Linux; Windows local only). The team had no runners: phase 5 sets them up.

## Decision

### 1. Common interface (`internal/runtime`)

- `ohruntime.Runtime`: `Kind`, `Available` (translated reasons), `Prepare` (image, mounts, network, then `Prepared` with the machine ↔ environment path translation), `Command` (machine command not started: the adapter keeps control of startup), `HostAddress`, `Teardown`. Optional interface: `Estimator` (estimated preparation without building anything).
- The runtime is part of the group key.
- Chosen at each launch (`oh run --runtime local|container|remote`, launch form option), in this order: option > project Execution config > Settings > workflow default. The choice is bounded by `runtime.allowed`.

### 2. Local

`opencode serve` runs on the machine. The useful machine variables (`PATH`, `HOME`, tools, never a secret) are set again in the session environment, because opencode 2.0.20 replaces the shell's environment.

### 3. Container (`internal/runtime/container`)

**Engine.** OCI CLI only, no Docker API. `auto` tries Colima, then Podman, then Docker (`[execution] engine` setting).

**Two-stage image**, tagged by the hash of its content:

- `oh-base/<project>`: the project's development Dockerfile (detected or configured, with its build args). Without a Dockerfile, a default oh base (`debian:bookworm-slim` + git, ca-certificates, ripgrep).
- `oh-dev/<project>`: a thin layer that adds opencode for Linux at the adapter's version (npm package verified by its sha512 integrity) and the fake `bd`.

Two images are kept per project and per stage. The build duration is estimated beforehand, and progress is shown in the launch form.

**Mounts** at fixed paths, translated both ways:

- bundle on `/opt/oh/bundle`, read-only;
- group data on `/opt/oh/data`;
- project and worktrees on `/work/<name>`;
- `HOME` on a project-specific volume: the user's config is never visible.

A worktree's common git directory is also mounted at its machine path, and git is configured with `safe.directory=*` and `gc.worktreePruneExpire=never`. A mount outside the directories shared with the VM is refused (otherwise Colima would silently mount an empty directory).

**User, environment, network, process.**

- User: `--userns=keep-id:uid=…,gid=…` (rootless Podman), otherwise `--user`.
- Environment: passed through `--env-file` (0600) and minimal, without any machine variable. The machine's git identity is passed in the session environment.
- Network: port published on `127.0.0.1` only. The machine is reached through `host.docker.internal`, `host.lima.internal` or `host.containers.internal`; on Linux, through a second proxy listener.
- Process: `run --rm --init`. `Teardown` removes the container, also when the daemon puts the group to sleep. The daemon stays active during a long image build.

**Busy group.** A busy group that does not see a new directory moves to a neighbor group (`-s1`, `-s2`…).

**Execution configuration** of the project (migration v41: Dockerfile, build args, volumes, default workflow and runtime) and `[execution]` Settings (runtime, engine, kept images, pinned opencode version, strict isolation). Doctor checks the engine, virtiofs, keep-id, shared directories, opencode in the image, and whether the proxy and gateways can be reached from a container.

### 4. Remote (GitLab CI)

**Setup.** One central **`oh-runner`** project per GitLab group, with no change to the projects' CI. `oh remote setup|status` checks access, creates the project, protects its default branch, writes the generated `.gitlab-ci.yml` (versioned schema), creates the trigger and the masked variables (LLM key, token per target project, team-state token), and checks the runners. Targets are declared in `[[remote.targets]]`.

**Sending** (`oh run … --runtime remote`):

1. Validation: `remote` is in `runtime.allowed`, and no checkpoint waiting for the user in the mode has `remote: forbid`.
2. Checks: branch pushed, Dockerfile committed, oh binary available.
3. Reservation: `bd update --claim` and team-state claim.
4. Beads snapshot.
5. Bundle (`oh-bundle/<hash>`) and session envelope (`oh-session/<sha256>`: manifest, inputs, prompt, checkpoint policy, snapshot) in the package registry. Inputs do not appear on the pipeline page.
6. Trigger, and reference recorded in `sessions.remote_ref` (migration v39).

**Job** (`oh runner run`):

- three jobs: `oh-cli`, `oh-image` (Kaniko by default, or Docker-in-Docker) and `oh-run`;
- oh's daemon, proxy and gateways run inside the job process, and the LLM key comes from a CI variable;
- secret variables are removed from the environment before startup, and opencode runs under an `oh` account (uid 10001);
- **policy responder**, never `--auto`: an `auto` checkpoint is approved; `defer`, an unknown checkpoint or a question cleanly stop the work; any other permission is refused;
- the branch is pushed with a draft MR (`git push -o merge_request.create`);
- artifacts: `journal.jsonl`, `summary.json`, `session.export` (session and sub-sessions).

**Return**:

- pipeline tracking, also by the daemon when oh is closed; "To fetch" notification;
- `oh session fetch`: artifacts, then import of the session into a local server, on the worktree of the pushed branch; the session can be resumed;
- `oh session resolve`: replay of the Beads journal ([ADR-046](./046-beads-gateways.en.md));
- TUI: ☁ option in the launch form, "To fetch" section, conflict window.

### 5. Windows

Local only, with the daemon inside the oh process. Container and remote are not supported.

## Consequences

### Positive

- A single launch path for the three environments: same bundle, same closed world, same checkpoints.
- In a container, the agent's shell has access neither to the daemon socket nor to `oh.db` (verified), and no secret is present; the environment is the project's, reproducible by hash.
- Remotely, the machine can be turned off, no request is blindly approved, and the session can be resumed locally.

### Negative / Trade-offs

- **The remote mode has not been validated for real** (no runner during the build): downloading GitLab artifacts, importing a real session, Kaniko on a private project, musl base.
- Remote job limits:
  - MCP servers that read a token from the machine keychain (gitlab, jira, figma…) are not available there (BL-18);
  - build args are not passed (BL-19);
  - no I6 budgets (BL-21) and no live follow (BL-9);
  - API keys only;
  - a deferred checkpoint is not recreated as a decision after import: the agent asks for it again on resume.
- Container:
  - validated for real on macOS (Colima and Podman); Linux in unit tests only;
  - iTerm2, and the gitlab/team MCP servers with real tokens, not tested;
  - a relative volume creates an empty directory in the project (issue Q3-4);
  - changing engines keeps the same group key;
  - interactive `bd` is unavailable;
  - the first image build is long.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Docker API (SDK) instead of the CLI | Heavy dependency, and Colima, Podman and Docker can all be driven through the same CLI. |
| devcontainer CLI | Extra dependency and less control over mounts, network and user. |
| Mount `~/.oh` into the container | Would expose the daemon socket, `oh.db` and the tokens to the agent. |
| One image per session | Repeated builds; the per-project image, tagged by content, is reused. |
| Change each project's CI | Intrusive; one central `oh-runner` project per group is enough. |
| Remote machines through SSH | Infrastructure to maintain; the team relies on GitLab CI. |
| `opencode --auto` in the job | Approves every request (F25); the responder applies the workflow's policy. |
| Inputs as pipeline variables | Visible on the pipeline page; they go through the session envelope. |
