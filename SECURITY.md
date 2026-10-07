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

### Alternative

Send an email to **security@example.com** with the following information:

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

Design decisions: [ADR-041](docs/architecture/adr/041-closed-world-isolation.en.md) (closed world), [ADR-044](docs/architecture/adr/044-credential-proxy-session-limits.en.md) (credential proxy, limits), [ADR-045](docs/architecture/adr/045-execution-environments.en.md) (execution environments), [ADR-046](docs/architecture/adr/046-beads-gateways.en.md) (gateways).

### Credential storage

openhub stores API tokens and secrets using two mechanisms:

- **OS keychain** (`go-keyring`) — preferred method, uses the native secure storage (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Encrypted file fallback** — when the OS keychain is unavailable, secrets are encrypted at rest using **AES-256-GCM** with **Argon2id** key derivation (OWASP minimum parameters: t=3, memory=64MB, threads=4). The passphrase is read from the `OH_PASSPHRASE` environment variable or prompted interactively.

Secrets are never stored in plaintext on disk. `~/.oh` is 0700; `oh.db` and its WAL files are 0600.

### LLM credential proxy

The oh daemon (`ohd`) runs a credential proxy on `127.0.0.1` (one listener per machine; on Linux, a second listener on the address seen by containers).

- **Group tokens.** Each server group (bundle version, project, runtime) gets a random 256-bit token `ohs_…`, given to opencode in the provider variable. It is revoked when the group sleeps, stops or is abandoned; without a valid token the proxy answers 401.
- **The key stays in the keychain.** The proxy reads the credential from the keychain (project key, provider key for the project, team key, hub key, then an AWS profile for Bedrock) and applies it in the transport, on the final request: header replaced, or AWS SigV4 signature. If the credential cannot be applied (for example expired AWS credentials), the request is refused (502) and nothing is sent to the provider.
- **Path allow-list.** Only the inference paths of each provider are forwarded; any other path gets 404. Each segment is decoded once; empty segments, `.`, `..` and double escaping are refused.
- **Model allow-list.** When a model list is active (session limits), the JSON `model` key must match exactly; a duplicate or a case variant is refused (400).
- **Size limit.** Request bodies are limited to 64 MiB (413 beyond).
- **No silent fallback.** The rendered configuration forbids any provider other than the session's one: opencode can no longer switch to its hosted models when a model is unavailable.
- **Hashes only.** The database stores only the SHA-256 hash of each token and a reference to the credential, never the token nor the secret. Tokens stored in clear by older versions are hashed when the daemon starts. A hash read from `oh.db` is not accepted by the proxy.

### The oh daemon

- **Socket.** `~/.oh/run/ohd.sock`, mode 0600. The peer identity is checked by the kernel on each connection (UID, `LOCAL_PEERCRED` on macOS, `SO_PEERCRED` on Linux): only processes of your user are accepted. On other systems, the 0600 mode alone applies.
- **Issuing capability.** The routes that hand out access (proxy and gateway tokens, credentials, extra proxy listeners) or stop the daemon also require the `X-Oh-Capability` header. It is a random secret kept in the OS keychain (`openhub.daemon.capability`), or, without a usable keychain, in `~/.oh/run/capability` (0600, reported by `oh doctor`). It is read by the oh CLI and the daemon only, compared in constant time, and never put in any environment.
- **Hooks.** The oh plugin and gateway routes (`/oh/v1/hooks/*`) are authenticated by the group token.
- **Checks.** `oh doctor` reports the capability storage, the mode and owner of the socket, and tokens left in clear in `oh.db`.
- **Windows.** The daemon runs inside the oh process (no background daemon).

### Closed world

A session sees only the agents and skills of its bundle. This is mandatory and not configurable.

- opencode's native agents are disabled; built-in skills are refused; for each agent, delegation is allowed only towards the targets of the workflow graph.
- The project's opencode configuration and data are excluded (`OPENCODE_DISABLE_PROJECT_CONFIG=1`, data folder per group).
- The oh plugin removes any agent or skill outside the bundle (a second barrier).
- **`Attest`** runs after each server start and for each new folder: it compares the agents, the skills visible to each agent (on the effective rules reported by the tool) and the MCP servers with the bundle. Anything unexpected makes the launch fail; the server is stopped and its token revoked.
- `[execution] strict_isolation` also hides your opencode user configuration in local mode. In a container or a remote job, it is never visible.

The closed world is about what the model sees, not what the shell can do.

### Gateways

**Beads gateway** (container and remote job). The fake `bd` sends the command to the daemon (`/oh-gateway/beads/v1/exec`), which runs the real `bd` on the machine:

- **`ohg_…` tokens**, one per session and sub-session, passed through the session environment. The daemon keeps only their hash (`~/.oh/run/gateway.json`, 0600). They are valid while the session is open and awake, and revoked with the group.
- **Allow-list** = `beads.allow` of the workflow. Without a `beads:` block, read only; an empty list refuses everything.
- **Refused options**: global options that change the database or the folder (`--db`, `-C`, `--global`…), anywhere in the command.
- **Paths** are translated to the machine and limited to the session locations (links resolved); the bundle and the group data are excluded.
- The real `bd` runs without a shell, in the session folder (2 min timeout, 8 MiB per stream).

**MCP gateway** (outside the machine). oh MCP servers of the bundle run on the machine, started by the daemon from the immutable bundle; they read their own tokens from the keychain. The container reaches them over HTTP with the group token (`{env:…}`, never written in the configuration). A call whose `_meta` designates a session of another group is refused (403).

### MCP token handling

MCP server tokens (GitLab, Figma, Jira, Linear, GitHub, Google) are:

- stored in the OS keychain or the encrypted file (never in plaintext config files);
- read by the `oh mcp serve` process itself, on the machine: the bundle only holds the name of the keychain key (`--token-key`);
- never written to the bundle, the opencode configuration or session state, never logged, never sent to AI providers, never put in a container.

### Container sessions

- **No secret in the container's environment.** The environment is passed with `--env-file` (0600) and is minimal, with no variable from the machine. The only credentials the shell sees are the `ohs_…` token and an `ohg_…` token (with `OH_GATEWAY_URL`).
- **Read-only bundle** on `/opt/oh/bundle`. `HOME` is a volume of the project: your configuration is never visible. `~/.oh` is not mounted: neither the daemon socket nor `oh.db` are reachable.
- The opencode port is published on `127.0.0.1` only. A mount outside the folders shared with the VM is refused.
- The container is removed when the session stops or sleeps (`run --rm --init`).

### Remote sessions (GitLab CI)

- **Masked and protected CI variables.** The job only receives the CI variables of the `oh-runner` project: LLM key of the jobs, an access token per target project, a team-state write token. They are masked (absent from logs) and protected (default branch of `oh-runner`, which must be protected). `oh remote setup` reads secrets from hidden input or an environment variable, never from arguments; `[[remote.targets]]` in `hub.toml` holds no secret. The machine keeps only your GitLab API token and the trigger token, in the keychain.
- **No secret in pipeline variables or artifacts.** Inputs and prompt travel in the session envelope (package registry), not in pipeline variables. Artifacts (`journal.jsonl`, `summary.json`, `session.export`) and error messages are scrubbed of secret values.
- **Inside the job.** `ScrubEnv` removes every variable whose name contains `TOKEN`, `PASSWORD`, `SECRET`, `_KEY`, `PASSPHRASE`, `CREDENTIAL`, `JWT` or `AUTH` before the daemon starts. The LLM key is held by the proxy of the job only. opencode runs under the `oh` account (uid 10001), which cannot read the job's processes, `oh.db` or the secrets. The project token is given to git through `GIT_CONFIG_*` in the git process environment, never in an argument, URL or file.
- **No blind approval.** A policy responder answers decisions, never `--auto`: `auto` checkpoints are approved, any other permission is refused, a deferred checkpoint or a question stops the work.

### Session limits (I6)

Optional and disabled by default (`oh budget show|set|unset|raise`): maximum active sessions, budget per session and daily budget in USD, memory cap, list of allowed models (enforced by the proxy on the group token). Cascade: workflow (`limits:`) > project > hub > team recommended; a team's enforced value is a ceiling. Budgets are soft ceilings: they are checked at the end of each step, which then raises a `$` decision.

### Known limitations of the local mode

- The agent's shell runs as your user. It may see its server's own proxy token, and it can still reach the daemon socket and `oh.db`.
- Without the capability it cannot obtain new tokens, but a process of your user can read the keychain item or the capability file, or run `oh` itself. These protections raise the bar; they do not isolate the agent.
- The LLM key is never in the agent's environment or in the opencode configuration, but your keychain is readable by your user.
- The password of the opencode servers stays in clear in `oh.db` (needed to attach to a session).
- Without strict isolation, your opencode user configuration is loaded; `Attest` refuses what adds visible agents, skills or MCP servers, not other settings.

Treat the local mode as "the agent acts with your user rights". Real isolation comes with container sessions.

### Known limitations of the remote mode

- MCP servers that read a token from the machine's keychain (gitlab, jira, figma…) are not available in the job; only `workflow` works.
- API key only (no AWS profile), and no I6 limits in the job.
- The parent GitLab shell keeps its own environment; only the `oh` account (server, agents) is protected from it.
- Maintainers of `oh-runner` can read its CI variables: restrict them.
- The remote mode has not yet been validated on a real runner.

### Self-update

The `oh upgrade oh` command downloads binaries exclusively from GitHub Releases with the following protections (opencode is installed and updated with its own tool):

- **URL allowlist** — only `github.com` and `objects.githubusercontent.com` are accepted
- **HTTPS-only** — plaintext HTTP is rejected
- **SHA256 checksums** — the downloaded binary is verified against the published checksum file; installation is refused if checksums do not match
- **Cosign signing** — release checksums are signed with [Sigstore](https://www.sigstore.dev/) (keyless OIDC). Client-side verification is currently advisory (logged warning); enforcement is planned for a future version
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
   cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
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
