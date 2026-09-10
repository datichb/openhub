> [Read in English](mcp-figma.en.md)

# Reference serveur MCP Figma

Le serveur MCP Figma fournit un acces en lecture aux fichiers Figma, composants et styles.

## Configuration

**Requis :** `FIGMA_TOKEN` (Token d'acces personnel Figma)

```toml
# hub.toml
[mcp.figma]
enabled = true
token_key = "openhub.mcp.figma.token"
```

## Outils

| Outil | Description | Parametres |
|-------|-------------|------------|
| `figma_get_file` | Obtenir la structure et les metadonnees d'un fichier Figma | `file_key` (string, requis) |
| `figma_get_node` | Obtenir un noeud specifique dans un fichier Figma | `file_key` (string, requis), `node_id` (string, requis) |
| `figma_get_styles` | Obtenir tous les styles publies d'un fichier | `file_key` (string, requis) |

## Exemples d'utilisation

```
# Obtenir la structure du fichier
figma_get_file(file_key: "abc123XYZ")

# Obtenir un frame ou composant specifique
figma_get_node(file_key: "abc123XYZ", node_id: "1:42")

# Obtenir les design tokens (couleurs, typographie, espacement)
figma_get_styles(file_key: "abc123XYZ")
```

## Utilise par

L'agent `designer` a l'acces MCP Figma active par defaut. Les autres agents n'y ont pas acces.

Voir le [guide d'integration Figma](../guides/figma-integration.fr.md) pour des exemples de workflows.
