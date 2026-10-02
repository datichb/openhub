> [Lire en francais](gslides-integration.fr.md)

# Google Slides Integration — Getting Started Guide

## Overview

The Google Slides MCP server provides read-only access to Google Slides presentations for AI agents. It enables agents to extract content, analyze slide structure, and understand presentation layouts.

### Features

- **Presentation metadata**: retrieve title, slide count, and layout information
- **Slide content extraction**: read individual slide content (text, shapes, images metadata)

---

## Prerequisites

- A Google OAuth 2.0 access token with `https://www.googleapis.com/auth/presentations.readonly` scope
- The token must be refreshed periodically (Google OAuth tokens expire after ~1 hour)

> **Note:** This integration requires an OAuth Bearer token, not a Google API key. See [Google OAuth 2.0 documentation](https://developers.google.com/identity/protocols/oauth2) for token generation.

---

## Setup

### 1. Configure via `oh mcp setup gslides`

```bash
oh mcp setup gslides
```

The interactive wizard will prompt for your Google access token.

### 2. Manual configuration (alternative)

Set the environment variable:

```bash
export GOOGLE_ACCESS_TOKEN="ya29.a0..."
```

Or configure in `hub.toml`:

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

Then deploy:

```bash
oh deploy
```

---

## Configuration in hub.toml

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

> **Security:** Store the token in the OS keychain via `oh secrets set GOOGLE_ACCESS_TOKEN` instead of writing it in `hub.toml`. The MCP server reads the environment variable at startup.

---

## Available Tools

| Tool | Description | Used by |
|------|-------------|---------|
| `gslides_get_presentation` | Get presentation metadata and slide list | Documentarian |
| `gslides_get_slide` | Get a specific slide's content | Documentarian |

### `gslides_get_presentation`

**Parameters:**
- `presentation_id` (string, **required**) — The Google Slides presentation ID (from the URL: `docs.google.com/presentation/d/{ID}/edit`)

**Returns:** JSON with presentation metadata including title, locale, slide dimensions, and a list of all slides with their IDs.

### `gslides_get_slide`

**Parameters:**
- `presentation_id` (string, **required**) — The presentation ID
- `slide_id` (string, **required**) — The slide page ID (obtained from `gslides_get_presentation`)

**Returns:** JSON with the full slide page element tree (text runs, shapes, images, tables).

---

## Usage Examples

**Extract content from a presentation:**
```
"Analyze the slides in presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 and summarize the key points"
```

**Get a specific slide:**
```
"Get slide p3 from presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 and describe its layout"
```

---

## Troubleshooting

### Token expired

```
Error: 401 Unauthorized
```

Google OAuth tokens expire after approximately 1 hour. Refresh your token and update the environment variable or keychain entry.

### Presentation not found

```
Error: 404 Not Found
```

Verify the presentation ID is correct and that the token has access to the presentation. The presentation must be shared with the Google account that generated the token.

### Insufficient permissions

```
Error: 403 Forbidden
```

Ensure the OAuth token was generated with the `presentations.readonly` scope. Tokens with only `drive.readonly` scope cannot access Slides API endpoints.

---

## Current Limitations

- **Read-only** — No creation or editing of presentations
- **No export** — Cannot export slides as PDF or images
- **Response size cap** — Responses are limited to 50 MB
- **Token management** — OAuth token refresh is manual; the MCP server does not auto-refresh tokens

---

## Resources

- [Google Slides API Reference](https://developers.google.com/slides/api/reference/rest)
- [Google OAuth 2.0 Documentation](https://developers.google.com/identity/protocols/oauth2)
- [MCP Google Slides Reference](../reference/mcp-gslides.en.md)
- [CLI Reference](../reference/cli.en.md)

---

## Support

```bash
oh mcp status gslides    # Check server status
oh mcp setup gslides     # Reconfigure
```

For issues: [GitHub Issues](https://github.com/datichb/openhub/issues)
