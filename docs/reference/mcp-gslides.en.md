> [Lire en francais](mcp-gslides.fr.md)

# Google Slides MCP Server Reference

The Google Slides MCP server provides read access to Google Slides presentations for analysis.

## Configuration

**Required:** Google OAuth credentials (`GOOGLE_ACCESS_TOKEN`)

```toml
# hub.toml
[mcp.gslides]
enabled = true
token_key = "openhub.mcp.gslides.token"
```

## Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `gslides_get_presentation` | Get presentation metadata and slide list | `presentation_id` (string, required) |
| `gslides_get_slide` | Get content of a specific slide | `presentation_id` (string, required), `slide_id` (string, required) |

## Usage Examples

```
# Get presentation overview
gslides_get_presentation(presentation_id: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtNs")

# Get a specific slide's content
gslides_get_slide(presentation_id: "1Bxi...", slide_id: "p")
```

## Used By

Available to agents when the `gslides` MCP server is enabled. Used primarily by the `documentarian` agent for presentation analysis (see [doc-slides skill](../architecture/skills.en.md)).
