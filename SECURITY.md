> [Lire en français](SECURITY.fr.md)

# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 5.x     | Yes       |
| 4.x     | No (opencode V1, end of life) |
| < 4.0   | No        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

### Preferred method

Use [GitHub Security Advisories](https://github.com/datichb/openhub/security/advisories/new) to report a vulnerability privately. This allows us to assess the impact, prepare a fix, and coordinate disclosure before any public announcement.

### If you cannot use GitHub Security Advisories

Open a public issue that only asks for a private contact (no details of the vulnerability). A report should contain:

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Potential impact assessment
- Any suggested fix (if applicable)

### What to expect

- **Acknowledgement** within 48 hours
- **Status update** within 7 days
- **Fix timeline** depending on severity (critical: 72h, high: 7 days, medium: 30 days)

If the vulnerability is accepted, we will:
1. Develop and test a fix
2. Publish a security advisory
3. Release a patched version
4. Credit the reporter (unless anonymity is preferred)

If declined, we will provide a clear explanation.

## Security Scope

### Overview (oh v5)

- **Secrets stay on the machine.** opencode never receives an LLM key or an integration token: it gets a group token, exchanged by the credential proxy of the oh daemon. Containers and remote jobs receive no secret from the machine.
- **Closed world.** A session only sees the agents and skills of its session bundle, and this is checked at each server start.
- **Beads stays on the machine.** Outside the machine, `bd` goes through a gateway restricted by the workflow's allow-list.
- **Local mode is not a sandbox.** In local mode the agent's shell runs as your user (see [Known limitations of the local mode](#known-limitations-of-the-local-mode)). Use container sessions for real isolation.
- **Guard rails, not barriers.** oh's shell rules (patterns) stop the mistakes of a model and cheap workarounds; they do not confine an agent that tries to get out (see [What the shell rules do not protect](#what-the-shell-rules-do-not-protect)). The locks that do not depend on patterns are: the checkpoint validation seen by oh, the gateways (Beads, MCP), the credential proxy and the container.

Design decisions: [ADR-041](docs/architecture/adr/041-closed-world-isolation.en.md) (closed world), [ADR-044](docs/architecture/adr/044-credential-proxy-session-limits.en.md) (credential proxy, limits), [ADR-045](docs/architecture/adr/045-execution-environments.en.md) (execution environments), [ADR-046](docs/architecture/adr/046-beads-gateways.en.md) (gateways), [ADR-052](docs/architecture/adr/052-beads-identity-provenance.en.md) (Beads read-only, actor, provenance).

### Credential storage

openhub stores API tokens and secrets using two mechanisms:

- **OS keychain** (`go-keyring`) — preferred method, uses the native secure storage (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Encrypted file fallback** — when the OS keychain is unavailable, secrets are encrypted at rest using **AES-256-GCM** with **Argon2id** key derivation (OWASP minimum parameters: t=3, memory=64MB, threads=4). The passphrase is read from the `OH_PASSPHRASE` environment variable or prompted interactively.

Secrets are never stored in plaintext on disk. `~/.oh` is 0700; `oh.db` and its WAL files are 0600.

The passwords of the opencode servers (needed to attach to a session and for the daemon to drive the server) are not in `oh.db`: one 0600 file per group under `~/.oh/run/server-passwords/` (0700 folder), `oh.db` keeps only their hash. A database of an older version is converted when the daemon starts (then compacted); `oh doctor` reports a password left in clear. These files stay readable by your user: the shell of the agents only reaches them by mistake (shell refusal of `~/.oh/run`), not through a barrier.

### LLM credential proxy

The oh daemon (`ohd`) runs a credential proxy on `127.0.0.1` (one listener per machine; on Linux, a second listener on the address seen from containers, accepted only when it is an address of a container engine bridge of the machine — `docker0`, `br-…`, `podman…`, `cni…`, `virbr…` — and checked again when the daemon restarts: never an address of the local network or of a VPN, even when the project image gives it).

- **Group tokens.** Each server group (bundle version, project, runtime) gets a random 256-bit token `ohs_…`, given to opencode in the provider variable. It is revoked when the group sleeps, stops or is abandoned; without a valid token the proxy answers 401.
- **The key stays in the keychain.** The proxy reads the credential from the keychain (project key, provider key for the project, team key, hub key, then an AWS profile for Bedrock) and applies it in the transport, on the final request: header replaced, or AWS SigV4 signature. If the credential cannot be applied (for example expired AWS credentials), the request is refused (502) and nothing is sent to the provider.
- **Path allow-list.** Only the inference paths of each provider are forwarded; any other path gets 404. Each segment is decoded once; empty segments, `.`, `..` and double escaping are refused.
- **Provider host checked.** The Bedrock region must be an AWS region name (`eu-west-1`…): checked when configured (`oh config set provider.bedrock.aws_region`, `oh provider`), at launch (`AWS_REGION` and the profile included), when the token is issued and on every request; a value such as `x.evil.com#` is refused before anything is sent. A well-formed region where Bedrock is not known gives a warning. Each request only goes over HTTPS to the provider's host (`bedrock-runtime.<region>.amazonaws.com`, `api.anthropic.com`, `openrouter.ai`, `api.openai.com`).
- **Model allow-list.** When a model list is active (session limits), the JSON `model` key must match exactly; a duplicate, a case variant or data after the JSON object is refused (400). With OpenRouter, `route`, `preset` and a preset in the model id (`…@preset/x`) are refused, and every fallback model of `models` must be in the list (403). A model list that cannot be read from the database is never read as "any model": the token is not restored.
- **Size limit.** Request bodies are limited to 64 MiB (413 beyond).
- **No silent fallback.** The rendered configuration forbids any provider other than the session's one: opencode can no longer switch to its hosted models when a model is unavailable.
- **Hashes only.** The database stores only the SHA-256 hash of each token and a reference to the credential, never the token nor the secret. Tokens stored in clear by older versions are hashed when the daemon starts. A hash read from `oh.db` is not accepted by the proxy.

### The oh daemon

- **Socket.** `~/.oh/run/ohd.sock`, mode 0600. The peer identity is checked by the kernel on each connection (UID, `LOCAL_PEERCRED` on macOS, `SO_PEERCRED` on Linux): only processes of your user are accepted. On other systems, the 0600 mode alone applies.
- **Issuing capability.** The routes that hand out access (proxy and gateway tokens, credentials, extra proxy listeners) or stop the daemon also require the `X-Oh-Capability` header. It is a random secret kept in the OS keychain (`openhub.daemon.capability`), or, without a usable keychain, in `~/.oh/run/capability` (0600, reported by `oh doctor`). It is read by the oh CLI and the daemon only, compared in constant time, and never put in any environment.
- **Hooks.** The oh plugin and gateway routes (`/oh/v1/hooks/*`) are authenticated by the group token.
- **Workflow routes.** `/v1/workflow/status`, `/checkpoint` and `/outputs` (backend of the `workflow` MCP server) require the token of the session's server group, which the MCP server reads from the tool server environment (never from the shell of a session); the session must belong to that group (401/403 otherwise). The routes only oh calls (`/v1/workflow/rules`, quit policies `/groups/{g}/policy`, attached clients `/v1/clients`) require the capability. A heartbeat keeps a client alive for 5 minutes at most.
- **Checkpoints bound to the user's validation.** A checkpoint that pauses in the session mode passes only with a validation seen by oh: given in oh (TUI, CLI), or given in the tool interface (oh reads the answer to the permission request in the event stream). Any other call (socket, forged MCP request) is refused and unlocks nothing; the agent is told to wait for the validation. An "always" answer saved by the tool does not let later checkpoints through: oh asks again.
- **Checks.** `oh doctor` reports the capability storage, the mode and owner of the socket, and tokens left in clear in `oh.db`.
- **Windows.** The daemon runs inside the oh process (no background daemon).

### Closed world

A session sees only the agents and skills of its bundle. This is mandatory and not configurable.

- opencode's native agents are disabled; built-in skills are refused; for each agent, delegation is allowed only towards the targets of the workflow graph.
- The project's opencode configuration and data are excluded (`OPENCODE_DISABLE_PROJECT_CONFIG=1`, data folder per group). This variable also leaves out the `AGENTS.md` of the project root and of its parents. Two sources stay outside this control, reported by `oh doctor`: a subfolder `AGENTS.md`, added to the prompt by the opencode read tool when an agent reads a file of that folder, and `~/.config/opencode/AGENTS.md` in local mode without `[execution] strict_isolation`.
- The oh plugin removes any agent or skill outside the bundle (a second barrier).
- **`Attest`** runs after each server start and for each new folder: it compares the agents, the skills visible to each agent (on the effective rules reported by the tool) and the MCP servers with the bundle. Anything unexpected makes the launch fail; the server is stopped and its token revoked.
- `[execution] strict_isolation` also hides your opencode user configuration in local mode. In a container or a remote job, it is never visible.

- **Session bundle integrity**: a bundle is shared by every session of the same hash. Its files are read-only once built (0444, folders 0555); the agents' edit tool refuses it; its hash is recomputed when it is reused (a modified bundle is rebuilt) and when oh loads it (a modified bundle is never used). A bundle downloaded from a remote job is refused when it does not match its hash.

The closed world is about what the model sees, not what the shell can do.

### Gateways

**Beads gateway** (local, container and remote job). The fake `bd` sends the command to the daemon (`/oh-gateway/beads/v1/exec`), which runs the real `bd` on the machine. Locally, the fake `bd` (`~/.oh/run/bin/bd`) comes first on the `PATH` of the session and its subagents; the shell rules of every agent refuse a `bd` called by a path (`/opt/homebrew/bin/bd …`, `./bd`), which would bypass the gateway:

- **`ohg_…` tokens**, one per session and sub-session, passed through the session environment. The daemon keeps only their hash (`~/.oh/run/gateway.json`, 0600). They are valid while the session is open and awake, and revoked with the group.
- **Allow-list** = `beads.allow` of the workflow. Without a `beads:` block, read only; an empty list refuses everything. A list that names only read commands (the default list included) is read-only: the write forms of its commands are refused too (`comments add`, `ready --claim`).
- **`--readonly` on reads**: every read command (closed list of oh, `show`, `list`, `query`, `comments`…) runs with `--readonly`, so that bd itself refuses a write behind the allow-list. Second layer only: bd 1.3.1 still lets `config set`, `hooks install`, `backup` and `import` through in that mode.
- **Actor set by oh**: every write carries `--actor=<person> via oh/<agent>` (git `user.name` of the location, fixed when the token is issued; entry agent or subagent), recorded in `bd history --events`. A claim keeps the person alone (bd makes the actor the assignee). Agents cannot choose the actor (`--actor` refused). oh binds claimed and closed tickets to the session and the commit (`bd provenance record --source=oh`); agents cannot record provenance. The daemon refreshes the leases (`bd heartbeat`) of the tickets claimed by open sessions. `find-duplicates --method ai`, which sends the tickets to an outside model, is refused. Project memories (`bd remember`) reach the session prompts inside data tags only.
- **Arguments read as `bd` reads them**: the gateway parses the command with the command and flag table of `bd` (generated from `bd` 1.3.1): attached forms (`-sclosed`, `-s=closed`, `--status=closed`), combined short flags (`-qC/other`), flags before or after the subcommand, aliases (`done`). A flag or command unknown to the table is refused (the real `bd` would refuse it too; one of a newer version cannot be checked). The real `bd` then receives the canonical form of what was checked (`--name=value`, then `--` and the arguments).
- **Refused options**: options that change the database or the folder (`--db`, `--database`, `-C`/`--directory`, `--global`, `--repo`…) or the actor (`--actor`), in every form.
- **Fixed database**: the Beads workspace of the session is resolved by `bd where` when the token is issued; the real `bd` then always runs with `BEADS_DIR` set to it (a `.beads` created in a sub-folder is ignored), and is refused when `redirect`, `metadata.json` or `config.yaml` of that workspace changed since. Agents cannot write to `.beads/` (edit tool, and shell rules on `redirect`, `metadata.json`, `config.yaml` and redirections into `.beads/`).
- **Closings recognized**: `close` (`done`), `update --status` to `closed` or to a status of category `done` in the project configuration (asked to `bd statuses`), `duplicate`, `supersede`, `import`, `batch`, `sql`, `todo done`, `gate resolve`, `mol squash|burn` go through the closing lock of the workflow.
- **Paths** are translated to the machine and limited to the session locations (links resolved), attached forms included (`-o/etc/x`); the bundle and the group data are excluded.
- The real `bd` runs without a shell, in the session folder (2 min timeout, 8 MiB per stream).
- **Shell start-up**: the shell of a local session reads oh's start-up files (`ZDOTDIR` for zsh, `BASH_ENV` for bash, `~/.oh/run/shell/`): they load the user's own files (`~/.zshenv`…), then put the fake `bd` back first, so that a real `bd` installed by npm, volta or in `~/.local/bin` cannot come first. The daemon checks it once per session (alert ✗ in "To handle" otherwise) and so does `oh doctor`.
- **Git hooks of Beads** (`bd hooks run <hook>`, run by git with `BD_GIT_HOOK=1`): they go through the gateway whatever `beads.allow` says, so that a commit is never blocked by the allow-list. `BD_GIT_HOOK` alone opens nothing: the fake `bd` of a local session asks the daemon, on its socket, for a **one-time hook token** (30 s, bound to the session and the hook); the daemon gives it only when the calling process is run by git from the hook of the session location (process tree read on the machine, macOS and Linux; elsewhere only the shape of the command is checked). Only `hooks run <bd hook>` with the arguments git passes to that hook (number and form) are accepted. Outside the machine (container, remote job), they are not run on the machine (the hooks bd chains would run outside the container).
- **Git hooks** (guard rail): every agent is refused `--no-verify` and its abbreviations, `-n` alone or combined with the short options of a commit (`-an`, `-qn`, `-aqn`…), `-c core.hooksPath=…` and any `git -c` option, `git config core.hooksPath`, `GIT_CONFIG_*`, config includes, writing to `.git/`, `.beads/hooks/`, `.husky/`, `.githooks/` (shell), editing `.git/`, the `.git` file of a worktree and `.beads/hooks/`; editing the hook files of the project (`.husky/*`, `.githooks/*`, `lefthook*.yml`, `.pre-commit-config.yaml`) asks.
- **Commit and closing locked by a checkpoint** (`unlocks:`, e.g. `cp-2` of `ticket`): until it is approved, `git commit` (and the commands that create a commit: `git revert`, `pull`, `merge`, `cherry-pick`, `am`, `rebase`, `stash pop`), `git push` and closing a ticket are refused to every agent; then the gateway closes a ticket only once the work is committed: HEAD moved from the commit of the checkpoint (on top of it), at least one commit changes tracked files outside `.beads/` (an empty commit does not count) and nothing is left to commit. A git or lock error refuses the closing.
- **Environment of `bd`**: the real `bd` gets a minimal environment (`PATH`, `HOME`, user, locale, temporary folder, XDG folders, `BEADS_*`), never the other variables of the daemon. The token of a subagent is revoked when its run ends and given again when it resumes. The grants kept in `~/.oh/run/gateway.json` are signed by the daemon (key derived from its capability): a file that was changed, is malformed or is readable by others is ignored at restart.
- **Subagent sessions closed by default**: a session oh launches gets its session rules (checkpoint locks) when it is created; a subagent session, created by the tool's delegation, gets them only once the daemon links it to its oh session. Meanwhile, the oh plugin lets none of the operations these rules may lock through (commit and the commands that create one, push, merge, tag, creating and closing tickets, merge request, `apply`, writing the `writes:` paths held by `writes_after`, delegating to a locked agent) without oh's answer: it relays them, oh links the session on the spot (a few seconds at most), then answers from the rules it sets (refused with the reason while the checkpoint is not passed). Without an answer (daemon stopped or unreachable, session oh links to none of its own, timeout), they are refused with a message that says so; other actions are unchanged. An imported session (resumed from a remote job, transferred) gets its rules at import, its subagents too. Without an active oh plugin, oh refuses to start a workflow that delegates to subagents (instead of the fallback mode).
- **Global guard** (last of every agent, whatever its permission base, also without a locking workflow; applied by the oh plugin, rendered in every agent without it): git commands that throw work away anywhere in the line (after `env`, `X=y`, `sh -c`, `git -C`); git options and variables that run a program or write a file (`--output`, `--upload-pack`, `--receive-pack`, `--ext-diff`, `--textconv`, `fsmonitor`, `GIT_DIR`, `GIT_SSH*`, `GIT_EXTERNAL_DIFF`, `GIT_PAGER`…); the oh socket and data (`ohd.sock`, `--unix-socket`, `~/.oh/run`, `~/.oh/bundles`, `~/.oh/sessions`, `oh.db`, `oh mcp serve`); the secrets and API of the opencode server; `find -exec|-delete|-ok…`; shell start-up files, `~/.ssh`, `~/.aws`, `~/.kube`, `~/.gitconfig`, `~/.netrc`, `~/.npmrc`, `crontab`, `LaunchAgents`; system installations (`npm -g` in any order, `--location=global`, `yarn global`, `pnpm -g`, `uv --system`, `--break-system-packages`, `sudo`); docker/podman options that get out of the container (`--privileged`, machine mounts, `--network host`, docker socket…). The context-mode tools that run code (`ctx_execute`…) never get more than the agent's shell.
- **Session scratch folders** (`agents.<id>.scratch`, [schema](docs/reference/workflow-schema.en.md#scratch-session-scratch-folders)): `~/.oh/sessions/<session>/scratch/<agent>/`, outside the repository, deleted with the session. oh guards after the rules of each agent, in this order: (0) all of `~/.oh/sessions` refused (reports, transcripts, session environment), for read, write, search and access outside the project; (1) `*.oh/sessions/*/scratch/*` refused to every agent, for read, write, search (`glob`, `grep`, `list`), access outside the project and shell, the owner included for the shell; (2) the owner reads and writes its folder, **in this session**; (3) the declared readers read it, in this session. The tool asks two permissions for a path outside the project: the directory outside the project, then the action on the path. The agent rules (shared by the sessions of one bundle) check the agent on the action; the session rules, last, check the session on the outside directory: an agent of another session, reader or owner included, is refused. A search only names a pattern: the oh plugin refuses it to the agents that are neither owner nor reader. Limits: without the oh plugin (fallback mode), another agent of the same session may search the folder (never an agent of another session); a reader with a broad shell may enter it with `cd` and act by relative path (the shell is not a barrier, see below); a project opened in the home directory itself (`~`) contains `~/.oh`: the session check, which goes through the outside directory, does not apply.

**MCP gateway** (outside the machine). oh MCP servers of the bundle run on the machine, started by the daemon from the immutable bundle; they read their own tokens from the keychain. The container reaches them over HTTP with the group token (`{env:…}`, never written in the configuration). A call whose `_meta` designates a session of another group is refused (403).

### MCP token handling

MCP server tokens (GitLab, Figma, Jira, Linear, GitHub, Google) are:

- stored in the OS keychain or the encrypted file (never in plaintext config files);
- read by the `oh mcp serve` process itself, on the machine: the bundle only holds the name of the keychain key (`--token-key`);
- never written to the bundle, the opencode configuration or session state, never logged, never sent to AI providers, never put in a container.

### Container sessions

- **No secret in the container's environment.** The environment is passed with `--env-file` (0600) and is minimal, with no variable from the machine. The only credentials the shell sees are the `ohs_…` token and an `ohg_…` token (with `OH_GATEWAY_URL`).
- **Read-only bundle** on `/opt/oh/bundle`. `HOME` is a volume of the server group, emptied at each container start: your configuration is never visible and nothing an agent leaves there (`.zshenv`…) reaches another session. `~/.oh` is not mounted: neither the daemon socket nor `oh.db` are reachable.
- **Git files that run programs are read-only**: `.git/config`, `.git/hooks`, `info/attributes`, and for a worktree its `.git` file and the `commondir`/`gitdir` links, in every view of the repository inside the container. The git of the machine (oh, then you after the session) runs nothing an agent planted there. Consequence: `git push -u` and `git branch --set-upstream-to` (which write the configuration) fail inside the container; branches are created without tracking configuration (`branch.autoSetupMerge=false`).
- **Hardened container**: `--cap-drop=ALL`, `--security-opt=no-new-privileges`, `--pids-limit=4096`; cache volumes refuse commas, quotes, `=`, `..` and targets under `/opt/oh`. The environment file (session tokens) is removed shortly after the container starts and when it stops.
- The opencode port is published on `127.0.0.1` only. A mount outside the folders shared with the VM is refused.
- The container is removed when the session stops or sleeps (`run --rm --init`).

### Remote sessions (GitLab CI)

> **Experimental, disabled by default in 5.0.** The remote mode has never been validated on a real runner (its protections, described below, are tested off-runner). oh refuses it until `[remote] experimental = true` is set (`oh config set remote.experimental true`); only enable it for trials, with test projects and tokens.

- **Masked and protected CI variables.** The job only receives the CI variables of the `oh-runner` project: LLM key of the jobs, an access token per target project, a team-state write token. They are masked (absent from logs) and protected (default branch of `oh-runner`, which must be protected). `oh remote setup` reads secrets from hidden input or an environment variable, never from arguments; `[[remote.targets]]` in `hub.toml` holds no secret. The machine keeps only your GitLab API token and the trigger token, in the keychain.
- **Image build without secrets.** Secret variables are limited to the environment of the `oh-run` job (`oh-run/*`, and `oh-run/<project id>` for a project token): the jobs that run the `RUN` steps of a project Dockerfile only have their `CI_JOB_TOKEN`, and read the build context sent by the machine (never a token in a URL). This token can write to the container registry of `oh-runner`: the `oh-run` job therefore never takes its image by tag, only by digest (the one the build job of the same pipeline pushed, or the one the machine recorded at the first successful build and whose tag did not change). Residual risk: tags and packages deleted or replaced (denial of service, no execution); only group projects of the same trust in a target.
- **No secret in pipeline variables or artifacts.** Inputs and prompt travel in the session envelope (package registry), not in pipeline variables. Artifacts (`journal.jsonl`, `summary.json`, `session.export`) and error messages are scrubbed of secret values.
- **Inside the job.** `ScrubEnv` removes every variable whose name contains `TOKEN`, `PASSWORD`, `SECRET`, `_KEY`, `PASSPHRASE`, `CREDENTIAL`, `JWT` or `AUTH` before the daemon starts. The LLM key is held by the proxy of the job only. opencode runs under the `oh` account (uid 10001), which cannot read the job's processes, `oh.db` or the secrets. The project token is only handed to git by a one-shot `GIT_ASKPASS` script (0600 file of root, removed after the call), never in an argument, a URL, an environment variable or the git configuration.
- **Final git steps without anything of the agent.** Before the final commit and push, oh kills every process of the `oh` account. It never runs git in the agent's repository: it copies the objects, the refs and `HEAD` into a fresh repository of its own (no configuration, hooks, index or `alternates` of the agent), and reads the work tree through it, with `core.hooksPath=/dev/null`, `core.fsmonitor=false`, `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`. Nested repositories and files not owned by the `oh` account (hard links to files of root) are not committed. The branch is pushed to the project URL chosen by the machine, never to a remote of the agent.
- **Authenticated and bounded Beads replay.** The job journal is sealed (HMAC) with a key of the session, kept in the machine keychain and handed to the job **encrypted** for the public key of `oh-runner` (X25519 + AES-256-GCM; private key in a masked, protected CI variable of the `oh-run` job only): members who read the package registry cannot forge a journal; before any replay, oh checks this seal and the one of its local copy of the envelope (`beads.allow` list, snapshot). Every write is checked again, read by the bd argument analyser of the Beads gateway, then run in its canonical form: a short list of options, tickets of the session only (including in `--parent` and `--deps`), any close (status `closed`/`done`, a project status of category `done`, an unknown status, aliases) handled as `close`. Control characters of the journal are neutralized on display.
- **No blind approval.** A policy responder answers decisions, never `--auto`: `auto` checkpoints are approved, any other permission is refused, a deferred checkpoint or a question stops the work. A checkpoint whose verification run by oh failed is never overridden in the job (the agent fixes the work); when its verification cannot run, it is deferred to the machine.
- **Checkpoint verification in the job, as the account of the agents.** oh runs the verification (`verify:`) in the clone of the job, as the `oh` account (the one of the tool server), never as root: the tests of the project, which the agent may have written, get no more rights than the agent (without an `oh` account in the image, the server and the verification both run as the user of the job, with a warning). Same filtered environment, timeout, bounded output and process group as on the machine; the git reads of the verification (state of the work, changed paths) run as that account too.
- **No git read as root in the agent's clone.** git refuses a repository of another owner because its configuration can make it run code (`core.fsmonitor`, filters named by an attribute…); oh never lifts that refusal (never `safe.directory`, neither `*` nor an exact path). Every git read of oh in the clone (`commit` window, closing guard, configuration check, start ref, provenance, Beads author, hook token) runs as the `oh` account registered by the job (root only switches to that account, and only for a repository it owns), with its folders, without the credential helper nor the SSH command of root, on top of the usual overrides: what the repository makes git run has the rights of the agent only. A read that fails keeps the protection closed (`commit` window closed, `bd close` refused, verification impossible, `paths` condition true). The configuration check only reads regular files and never blocks on a FIFO planted by the agent. The job waits for the running verifications before it kills the processes of the `oh` account and pushes; their result travels in `summary.json`, imported on the machine cleaned and bounded again, and never counts as a verification of the work on the machine.


### Agent reports (`workflow_report`)

- **Author set by oh.** The daemon takes the author of a report from the calling tool session (the subagent it sees), never from the request; the routes are those of the `workflow` MCP server, with the group token of the session.
- **Neither read nor forged by agents.** Bodies are in `~/.oh/sessions/<session>/reports/` (files 0600, folder 0700), refused to every agent by the oh guards (shell, read, write, search; layer 0, before the scratch exceptions); each body is checked against its SHA-256 digest before it is shown. Agents read the reports of their session through `workflow_report_get` only.
- **Bounded and cleaned.** 64 KiB per report, 100 reports and 4 MiB per session; terminal sequences and control characters removed; the TUI escapes the text. The reports of a remote session (`reports.json`, pipeline schema 5) are cleaned and bounded again on import.

### Project wiki (`wiki` MCP server, LD-J)

- **Proposing is not writing.** Agents propose with `wiki_propose`; only the proposals the user accepts on the card of the commit checkpoint are written in `docs/wiki/`, by oh, when the checkpoint is passed (traced: status, `applied_by`, date, timeline entry). The others stay queued.
- **Author set by oh.** The daemon takes the author from the calling tool session, never from the request; the tag line (tag, author, date, source) is written by oh, those of the content are removed. Routes authenticated like those of the `workflow` server (group token of the session).
- **`wiki_apply` reserved to the documentarian.** Generated permission rule (refused to every agent, `ask` for the `documentarian`) and daemon check on the agent it sees calling (403 otherwise).
- **Untrusted content.** Cleaned (control characters, terminal sequences), neutralised (headings, frontmatter markers, HTML and comments, `oh:when` among them, an unclosed code fence), bounded (4 KiB, 60 lines; 50 open proposals per session, 500 per project) and escaped in the TUI. A proposal citing the oh session data (`~/.oh/sessions`, the scratch folders among them, or the sessions folder actually in use) is refused: a scratch file never reaches the wiki.
- **No symbolic link.** oh never reads nor writes through a symbolic link in `docs/` or `docs/wiki/`; page paths are relative, without `..` nor hidden components; writes are atomic. The pages and the files they cite are read without ever leaving the project, even when a link is swapped in during the read (`os.Root`).
- **Remote session.** The daemon of the job (root) serves the wiki of the clone, which belongs to the agents' `oh` account: reads confined to the clone, git reads as the `oh` account. No proposal is applied in the job (`wiki_apply` refused to all, nothing applied at the commit checkpoint); they come back in `wiki.json`, are cleaned, bounded and queued **pending** again on the machine, where the user validates them.

### Session limits (I6)

Optional and disabled by default (`oh budget show|set|unset|raise`): maximum active sessions, budget per session and daily budget in USD, memory cap, list of allowed models (enforced by the proxy on the group token). Cascade: workflow (`limits:`) > project > hub > team recommended; a team's enforced value is a ceiling. Budgets are soft ceilings: they are checked at the end of each step, which then raises a `$` decision.

### Known limitations of the local mode

- The agent's shell runs as your user. It can still reach the daemon socket and `oh.db`, but the socket no longer passes a checkpoint (see above).
- **Environment of other processes**: a process of your user can read the environment of another one (`ps eww` on macOS, `/proc/<pid>/environ` on Linux), including the opencode server (password, tokens); these commands are refused to agents as a guard rail, not a barrier.
- **oh's git on the machine**: oh runs git in your repositories (state before a checkpoint, worktrees, diffs, Beads clean-up) with what the repository configuration could run neutralized (`core.fsmonitor`, hooks, `diff.external`, textconv, `core.sshCommand`, `uploadpack`/`receivepack`, credential helpers, `ext::` URLs), and refuses to pass a checkpoint that unlocks an operation when the git configuration or hooks changed since the session started; an alert ✗ in “To handle” (`oh session inbox`, Sessions view, notification) names the changed file and says what to do (check, restore the file, or start the session again).
- Without the capability it cannot obtain new tokens, but a process of your user can read the keychain item or the capability file, or run `oh` itself. These protections raise the bar; they do not isolate the agent.
- The LLM key is never in the agent's environment or in the opencode configuration, but your keychain is readable by your user.
- The shell of a session never gets the opencode server password, its inline configuration, the group token nor the provider token: the oh plugin removes them from every shell, also without a session environment (a subagent not yet adopted, a session opened in the tool interface). Without an active oh plugin (fallback mode, reported in the log), a shell without a session environment still inherits them. Commands aiming at these variables or at the server API (permission and question answers, session environment) are refused to every agent, as a guard rail.
- Without strict isolation, your opencode user configuration is loaded; `Attest` refuses what adds visible agents, skills or MCP servers, not other settings.
- The shipped development agents (`developer-rw` base) install dependencies only in the project: `pip` only inside the project's virtual environment (`.venv/bin/…`, `uv`, `poetry`), no `npm -g`, `yarn global`, `pnpm add -g`, `gem install` nor `--break-system-packages`. Their allow-list is a guard rail, not a sandbox: test runners and build tools run the project's code.

- **Infrastructure**: the `operator` (workflow `infra-change`) reads freely (`terraform plan`, `kubectl get|describe|diff`, `helm template`, `helm diff`, `aws … describe-*|list-*`…), asks before reading secrets (`terraform output`, `kubectl get secret`, `vault kv get`, `aws … get-*`) and tunnels; any change (`terraform apply`, `kubectl apply|delete|exec…`, `helm install|upgrade`, `aws … create-*|delete-*|put-*…`, `pulumi up`, `vault write`…) is refused until a checkpoint `unlocks: [apply]` is passed (always refused in a workflow that declares none), then asked one by one. This guard applies to an agent that runs a tool as a whole (`terraform*`); the `auditor-subagent` (audit domain `infra`) only names static analysis commands (`terraform validate`, `terraform plan`, `tflint`, `checkov`, `helm lint`…), each one asked, and every other command is refused to it. Credential extraction (`get-secret-value`, `--with-decryption`, `get-login-password`, `assume-role`, `print-access-token`, `--show-secrets`, `kubectl config view --raw`…) is refused to every agent. Limit: `terraform plan` and `pulumi preview` run code of the repository (providers, programs); the real protection is a read-only credential (`ReadOnlyAccess` role, `view` kubeconfig) or a container session without `~/.aws` and `~/.kube`.
- **Reviewer**: read-only shell (read-only git, `bd show`) plus the build, test, lint and type check commands of the detected stack (`go test`, `npm test`, `npm run build`, `pytest`, `cargo test`, `make test`…; `npx --no`, `uv run --no-sync`: never an install), allowed in the workflows on the team's own code and asked in `review` (`risk: read`: running the tests of someone else's merge request runs its code). Options that change the project (`--fix`, `--write`, snapshot and golden updates) are refused. Like any test runner, these commands run the project's code.
- **Debugger and benchmarker**: the `debugger` no longer has a free shell (reads, read-only git, Beads, test runners on confirmation); the `benchmarker` no longer reads the environment (`env`, `printenv`) and asks before the tools that run an arbitrary command (`hyperfine`, `perf`, `wrk`, `py-spy`).

Treat the local mode as "the agent acts with your user rights". Real isolation comes with container sessions.

### What the shell rules do not protect

opencode compares the shell rules with the raw text of each command of a line (verified on opencode 2.0.20):

- **Quotes and spaces** are not normalized: `git pu''sh` or `git  push` (two spaces) do not match `git push*`.
- **Declarations**: `export X=…`, `declare`, `local`… are not checked; a pattern on a variable name (`*GIT_DIR*`) is bypassed with `export GIT_DIR=… && git …`.
- **Interpreters and project tools**: any agent that can edit a file and run a build tool, a test runner or an interpreter runs whatever it wants (`npm run`, `make`, `pytest` with a `conftest.py`, `.venv/bin/python`, `go run`…).
- **Outside the project**: opencode's "external directory" permission covers the working directory and `cd` only; a redirection (`> ~/file`) or an argument writes outside the project without asking. Only the patterns of the guard cover sensitive files.

What does not depend on patterns: a paused checkpoint passes only with your validation seen by oh; Beads and the oh MCP servers go through gateways that apply the workflow list; the proxy keeps the keys; the container isolates the file system. For work that writes, prefer the container, and give infrastructure tools read-only credentials.

### Known limitations of the remote mode

- MCP servers that read a token from the machine's keychain (gitlab, jira, figma…) are not available in the job; only `workflow` works.
- API key only (no AWS profile), and no I6 limits in the job.
- The parent GitLab shell keeps its own environment; only the `oh` account (server, agents) is protected from it.
- Maintainers of `oh-runner` can read its CI variables (`OH_JOURNAL_KEY` included): restrict them.
- The rest of the session envelope (prompt, inputs, ticket snapshot) is readable by the members of `oh-runner` and by the image build jobs; only the journal key is encrypted in it.
- A target project can delete or replace the tags and packages of `oh-runner`: the jobs detect it (digests) and fail or rebuild (denial of service only). The image by digest after a build relies on GitLab Runner reading the dotenv in `image:` (to check during the acceptance run).
- The remote mode has not yet been validated on a real runner: it is **experimental and disabled by default** (`[remote] experimental = true` to try it).

### Self-update

The `oh upgrade oh` command downloads binaries exclusively from GitHub Releases with the following protections (opencode is installed and updated with its own tool):

- **URL allowlist** — only `github.com` and `objects.githubusercontent.com` are accepted
- **HTTPS-only** — plaintext HTTP is rejected
- **SHA256 checksums** — the downloaded binary is verified against the published checksum file; installation is refused if checksums do not match
- **Cosign signing** — release checksums are signed with [Sigstore](https://www.sigstore.dev/) (keyless OIDC, identity: the `release.yml` workflow of this repository on a `v*` tag). When the `cosign` CLI is installed, `oh upgrade oh` and `install.sh` verify the signature and **refuse** an invalid one. Without `cosign` they only verify the checksum and print a warning; set `OH_REQUIRE_SIGNATURE=1` to refuse any update whose signature cannot be verified
- **Atomic replacement** — the binary is replaced atomically with automatic rollback on failure

### Supply chain

- Releases are built via [GoReleaser](https://goreleaser.com/) in GitHub Actions with `CGO_ENABLED=0` (static binaries, no C dependencies)
- All release artifacts are signed with [Cosign/Sigstore](https://www.sigstore.dev/) (keyless, OIDC-based)
- Dependency updates are managed by Dependabot (weekly cadence)
- Security-critical code paths are protected by [CODEOWNERS](CODEOWNERS) requiring maintainer review
- The opencode layer of container images is installed from the npm package checked by its sha512 integrity

## Best Practices for Users

1. **Use the OS keychain** — it is the most secure storage. Avoid the file fallback unless necessary.
2. **Set `OH_PASSPHRASE`** — if using the encrypted file fallback, set this environment variable securely (e.g., via a secrets manager or shell profile, not in a committed file).
3. **Do not commit `hub.toml` with tokens** — the `hub.toml` configuration file should never contain plaintext tokens. Use `oh init`, `oh provider setup`, `oh mcp setup` or `oh secrets set` to store them securely.
4. **Prefer container sessions for untrusted work** — in local mode the agent acts with your user rights.
5. **Keep read-only workflows read-only** — `risk: read` and a narrow `beads.allow` limit what an agent may change; team and project layers can only harden security.
6. **Protect `oh-runner`** — protect its default branch and restrict its Maintainers.
7. **Run `oh doctor`** — it checks the opencode version, the daemon capability, the socket and tokens left in clear.
8. **Verify binary signatures** — after downloading, verify the Cosign signature:
   ```bash
   cosign verify-blob --bundle checksums.txt.sigstore.json \
     --certificate-oidc-issuer https://token.actions.githubusercontent.com \
     --certificate-identity-regexp '^https://github\.com/datichb/openhub/\.github/workflows/release\.yml@refs/tags/v.*$' \
     checksums.txt
   ```
9. **Keep openhub updated** — run `oh upgrade oh` (or `brew upgrade openhub`) regularly to receive security patches.

## Code Review Requirements

Security-critical paths require maintainer review via CODEOWNERS:

| Path | Scope |
|------|-------|
| `.github/` | CI/CD workflows and release configuration |
| `cli/.goreleaser.yml` | Release and signing configuration |
| `cli/internal/selfupdate/` | Binary self-update logic |
| `cli/internal/hubcontent/` | Embedded content integrity |
| `cli/internal/storage/keychain/` | OS keychain operations |
| `cli/internal/storage/filecrypt/` | Encryption at rest |
| `permissions/` | Agent permission definitions |
| `install.sh` | Curl-pipe installation script |
