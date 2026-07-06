> 🇫🇷 [Lire en français](021-provider-transparent-validation.fr.md)

# ADR-021 — Transparent LLM Provider Validation

- **Status:** Accepted
- **Date:** 2026-07-01
- **Deciders:** OpenCode Hub team

---

## Context and Problem

When an LLM provider is misconfigured (missing or expired API key, unreachable endpoint, or
inconsistent `opencode.json`), OpenCode displays a non-blocking `ProviderInitError` notification
at the moment the user selects an agent in the TUI. The interface stays open but no conversation
starts — the user is left in a silently unusable interface with no indication of the cause or
how to resolve it.

Root causes identified:

1. **No pre-flight**: the hub launches OpenCode without verifying that the provider is reachable.
2. **Orphan models**: when an API key is missing, `_build_provider_json()` returns empty but the
   `model` field in `opencode.json` is still prefixed with the provider name
   (e.g. `amazon-bedrock/...`). OpenCode attempts to initialize a provider with no configuration.
3. **litellm providers without `models{}`**: mammouth, ollama, and github-models did not declare
   a `models` block in `opencode.json`, causing `ProviderModelNotFoundError` on agent selection.
4. **openrouter not handled**: the `openrouter` case was missing from `_build_provider_json()`,
   leaving the JSON without a provider block for projects configured on openrouter.
5. **Duplicate `/chat/completions` suffix**: the AI SDK automatically appends this suffix to
   `baseURL`. If the hub includes it too, the endpoint becomes invalid.

---

## Decision

Implement **transparent, non-blocking validation** in 4 layers, integrated into the hub without
modifying OpenCode's own behavior:

### Layer 1 — Informative messages (`provider-warnings.sh`)

New module `scripts/lib/provider-warnings.sh` displayed in the `oh start` context block.
Status is shown with ✅ (OK) or ⚠️ (problem) with an actionable hint toward `/connect`
or `oh config set`.

### Layer 2 — Pre-flight check (Approach A)

Lightweight connectivity test (`curl`, 3s timeout) toward the provider endpoint before
launching OpenCode. The test is adapted to the authentication type:
- **API Key providers**: `GET /v1/models` or HTTP header test
- **Bedrock**: presence of `AWS_BEARER_TOKEN_BEDROCK`, `AWS_ACCESS_KEY_ID`, or `~/.aws/credentials`
- **GitHub Copilot**: presence of the token in `~/.local/share/opencode/auth.json`

Automatically skipped if `curl` is absent or if the command is run without a TTY (CI/CD).

### Layer 3 — Post-deploy validation (Approach C)

After each write of `opencode.json`, verify that the prefix of the `model` field corresponds
to an existing `provider` block in the generated file. If not (orphan model), store in
`_DEPLOY_PROVIDER_WARNING` for display at the next `oh start`.

### Layer 4 — Structural fixes (Approach B)

- Add a `models` block and a `name` to litellm providers to prevent `ProviderModelNotFoundError`.
- Add the `openrouter` case to `_build_provider_json()`.
- Detect and report `baseURL` values that include `/chat/completions`.

---

## Alternatives Considered

### Alternative A — Block launch if provider is down

Refuse to launch OpenCode if the pre-flight fails. Rejected because:
- False positives on slow networks or VPN
- Prevents offline use (local Ollama, temporarily unavailable Bedrock)
- Incompatible with the transparency objective

### Alternative B — Fully delegate to `/connect`

Remove credential injection from `opencode.json` and fully delegate to OpenCode's native
mechanism (`auth.json` via `/connect`). Rejected because:
- Breaks current UX (deployed projects work without manual action)
- Loses centralized per-project credential control via `api-keys.local.md`

### Alternative C — Automatic provider fallback

Configure a fallback provider in `hub.json`. Rejected for this iteration because:
- Increased complexity (which fallback? what UX?)
- Masks problems instead of surfacing them clearly
- Can be added in a future iteration if the need is confirmed

---

## Consequences

### Positive

- Users are **always informed** of their provider status before the problem appears in OpenCode.
- The hints `→ Use /connect` and `→ oh config set` reduce time to resolution.
- Detection of the `/chat/completions` suffix prevents a recurring issue with MammouthAI.
- litellm providers work correctly without `ProviderModelNotFoundError`.
- Coverage of all entry paths (`adapter_start`) ensures the warning appears even from
  `oh quick`, `oh review`, `oh audit`, etc.

### Negative / constraints

- The pre-flight adds ~3s of delay on TTY commands if the endpoint doesn't respond.
  Acceptable: this delay only appears when the provider is actually problematic.
- `curl` is required for connectivity tests. Graceful skip if absent.
- Bedrock connectivity tests require the `aws` CLI for complete validation.
  Without it, detection is less precise (env vars check only).

---

## Files Affected

| File | Change |
|------|--------|
| `scripts/lib/provider-warnings.sh` | New file — validation + display |
| `scripts/lib/i18n.sh` | +12 i18n keys (6 FR + 6 EN) |
| `scripts/adapters/opencode.adapter.sh` | `models{}` block for litellm + openrouter + post-deploy validation + `_warn_provider_if_needed` in `adapter_start` |
| `scripts/cmd-start.sh` | Integration of `_display_provider_status` in context block |
| `tests/test_lib_provider_warnings.bats` | New file — ~35 BATS tests |
| `tests/test_opencode_adapter.bats` | +6 tests |
| `tests/test_lib_i18n.bats` | +3 tests |
| `docs/guides/providers.fr.md` | New "Diagnostic et résolution" section |
| `docs/guides/providers.en.md` | New "Diagnosing and Resolving" section |
