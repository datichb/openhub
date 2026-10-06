> [Lire en français](044-credential-proxy-session-limits.fr.md)

# ADR-044 — LLM Credential Proxy, Session Tokens and I6 Restrictions

## Status

Accepted

## Date

2026-10-05

## Context

Up to v4, oh passed the LLM provider key to opencode, in the environment or in `opencode.json` ([ADR-021](./021-provider-transparent-validation.en.md)). An `env` run by an agent therefore showed the real key.

v5 adds further constraints:

- the per-group isolated data directory hides the credentials stored by opencode (F14);
- Bedrock requires a region (F15);
- an unavailable model made **opencode fall back to its free hosted models**: the prompt went to another provider, without warning (found in phase 0);
- containers and remote jobs must not receive any machine secret (D10);
- usage restrictions (I6: active sessions, budgets, memory) needed a counting point.

Decision O4: a credential proxy built into `oh` from phase 0.

## Decision

### 1. Proxy and tokens

- `internal/credproxy`, hosted by the `ohd` daemon: one listener per machine on `127.0.0.1`, stable port recorded in `ohd.json`. On Linux, a second listener on the address seen by containers.
- The provider URL points to the proxy (`…/<provider>/…`). opencode only receives an `ohs_…` token, in the provider's variable (for example `AWS_BEARER_TOKEN_BEDROCK`).
- **One token per server group**, not per session, because the environment of the `opencode serve` process is shared by the group's sessions. It is revoked on sleep, stop and when the group is abandoned; without a valid token, the proxy answers 401.

### 2. Credential source

- Cascade read from the keychain: project key, provider key for the project, team key, hub key, then, for Bedrock, an AWS profile. With a profile, the proxy signs with SigV4, with a cache per profile and region.
- A keychain error stops the cascade: only "not found" moves to the next level.
- Region: oh config, then `AWS_REGION` / `AWS_DEFAULT_REGION`, then profile, then `us-east-1` with a warning.
- Provider, region, source and secret fingerprint are part of the group key: a change starts a new server.

### 3. Proxy behavior

- The credential is applied in the transport, on the final request. Depending on the provider, the header is replaced (Bedrock Bearer, Anthropic `x-api-key`, OpenAI and OpenRouter Bearer) or the request is signed with SigV4. A failure returns 502 without sending anything.
- Unbuffered streaming.
- **Allowlist of inference paths** per provider, otherwise 404. Each segment is decoded only once; `.` and `..` are refused.
- Body limited to 64 MiB (413).
- With an active model list, the JSON `model` key must be exact (duplicate or case variant: 400).
- Usage is counted from the responses, streams included: Bedrock `converse-stream` and `invoke-with-response-stream` (base64), Anthropic, OpenAI with `include_usage` added.

### 4. Provider policy

The rendered config refuses `provider.use` for any provider other than the session's (`experimental.policies`): no more silent fallback to opencode-hosted models.

### 5. Storage and daemon security (M12)

- Tokens known **by their SHA-256 fingerprint** only (`proxy_grants`, `servers`). Older values are converted at startup.
- Tokens are restored after the socket is opened; orphan tokens are revoked.
- If the proxy port changes, the affected groups are put to sleep and a ✗ decision is raised for sessions that were working.
- Unix socket in 0600, **peer UID checked** (`LOCAL_PEERCRED`, `SO_PEERCRED`).
- Routes that issue tokens or stop the daemon are reserved to the CLI by a **capability** (`X-Oh-Capability`, in the keychain, or failing that in a 0600 file), never put into an environment.
- Plugin and gateway hooks (`/oh/v1/hooks/*`) are authenticated by the group token.

### 6. I6 restrictions (`internal/limits`), disabled by default

- Available restrictions: maximum active sessions, per-session and daily budget in USD, memory ceiling, model list.
- **Cascade**: workflow (`limits.budget_usd`, `limits.models`) > project > hub (`[limits]`) > team recommended. Team enforced values are a ceiling.
- **Usage registry** (migration v40): `usage_sessions` (per day and per session, with the root session, to count sub-agent cost), `usage_proxy`, `budget_extra` (raises).
- **Overrun**, checked by the daemon at the end of a step:
  - a `$` decision is raised: raise, stop or dismiss;
  - a step that starts while a `$` decision is open is interrupted.
- Beyond the number of active sessions or the memory ceiling, the first prompt is queued. Daily budget spent: new session refused.
- Commands: `oh budget show|set|unset|raise`, Restrictions section of the TUI settings, Doctor check.

### 7. Windows

The daemon runs inside the oh process: the proxy stops when oh exits, and groups are put to sleep.

## Consequences

### Positive

- No secret in the tool's environment: `env` in an agent's shell only shows `ohs_…` (verified in e2e locally, for real in a container, and in a local trial of the remote job).
- Per-group revocation, counting in a single place, and no more silent switch to another provider.
- The same mechanism serves the plugin hooks and the gateways ([ADR-046](./046-beads-gateways.en.md)).
- Restrictions are available without imposing anything by default.

### Negative / Trade-offs

- Locally, the agent runs under the user's account: it can read the keychain or run `oh` (documented in `SECURITY`). The servers' password stays in clear text in `oh.db`.
- One token per group: per-session cost comes from what opencode reports, not from the proxy.
- Budgets are **soft ceilings**:
  - checked at the end of a step, a step can overrun;
  - a fork counts the cost of the copied history again;
  - the memory ceiling does not count containers;
  - two simultaneous launches can exceed the maximum number of active sessions.
- AWS profiles (SigV4) do not work in CI: API key only remotely, and no usage registry in the job (BL-21).
- The Linux second listener is only unit-tested, and the budget e2e test was not done.

## Alternatives Considered

| Alternative | Rejected because |
|---|---|
| Pass the key to opencode (environment or config) | Visible to the agent's shell and passed to containers. |
| Let opencode store the key (`auth.json` per group) | Secret written to disk, readable by the agent; tricky import (F14). |
| An external proxy (LiteLLM…) | Extra infrastructure, secret elsewhere, and no link with oh's groups or budgets. |
| One token per session | The server environment is shared by every session of the group. |
| Token budgets at the proxy | The first choice; replaced by USD computed from the cost reported by opencode, more meaningful and common to all providers. |
| Cut a stream on overrun | Breaks a response in progress; we prefer to finish the step and then decide. |
