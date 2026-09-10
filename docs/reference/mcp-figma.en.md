> [Lire en francais](mcp-figma.fr.md)

# Figma MCP Server Reference

The Figma MCP server provides read access to Figma design files, components, and styles.

## Configuration

**Required:** `FIGMA_TOKEN` (Figma Personal Access Token)

```toml
# hub.toml
[mcp.figma]
enabled = true
token_key = "openhub.mcp.figma.token"
```

## Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `figma_get_file` | Get a Figma file's structure and metadata | `file_key` (string, required) |
| `figma_get_node` | Get a specific node within a Figma file | `file_key` (string, required), `node_id` (string, required) |
| `figma_get_styles` | Get all published styles in a file | `file_key` (string, required) |

## Usage Examples

```
# Get file structure
figma_get_file(file_key: "abc123XYZ")

# Get a specific frame or component
figma_get_node(file_key: "abc123XYZ", node_id: "1:42")

# Get design tokens (colors, typography, spacing)
figma_get_styles(file_key: "abc123XYZ")
```

## Used By

The `designer` agent has Figma MCP access enabled by default. Other agents do not have access.

See [Figma integration guide](../guides/figma-integration.en.md) for workflow examples.
