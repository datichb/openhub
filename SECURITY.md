# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 4.x     | Yes       |
| 3.x     | No (end of life) |
| < 3.0   | No        |

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

### Credential storage

openhub stores API tokens and secrets using two mechanisms:

- **OS keychain** (`go-keyring`) — preferred method, uses the native secure storage (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- **Encrypted file fallback** — when the OS keychain is unavailable, secrets are encrypted at rest using **AES-256-GCM** with **Argon2id** key derivation (OWASP minimum parameters: t=3, memory=64MB, threads=4). The passphrase is read from the `OH_PASSPHRASE` environment variable or prompted interactively.

Secrets are never stored in plaintext on disk.

### Self-update

The `oh upgrade` command downloads binaries exclusively from GitHub Releases with the following protections:

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

### MCP token handling

MCP server tokens (GitLab, Figma, Jira, Linear, Google) are:
- Stored in the OS keychain or encrypted file (never in plaintext config files)
- Passed to MCP server subprocesses via environment variables
- Never logged, never written to session state, never transmitted to AI providers

## Best Practices for Users

1. **Use the OS keychain** — it is the most secure storage. Avoid the file fallback unless necessary.
2. **Set `OH_PASSPHRASE`** — if using the encrypted file fallback, set this environment variable securely (e.g., via a secrets manager or shell profile, not in a committed file).
3. **Do not commit `hub.toml` with tokens** — the `hub.toml` configuration file should never contain plaintext tokens. Use `oh init` to store them securely.
4. **Verify binary signatures** — after downloading, verify the Cosign signature:
   ```bash
   cosign verify-blob --bundle checksums.txt.sigstore.json checksums.txt
   ```
5. **Keep openhub updated** — run `oh upgrade` regularly to receive security patches.

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
