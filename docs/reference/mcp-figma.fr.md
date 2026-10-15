> [Read in English](mcp-figma.en.md)

# Référence serveur MCP Figma

Le serveur MCP Figma fournit un accès en lecture aux fichiers Figma, composants et styles.

## Configuration

**Requis :** `FIGMA_TOKEN` (Token d'accès personnel Figma)

```toml
# hub.toml
[mcp.figma]
enabled = true
token_key = "openhub.mcp.figma.token"
```

## Outils

| Outil | Description | Paramètres |
|-------|-------------|------------|
| `figma_get_file` | Obtenir la structure et les métadonnées d'un fichier Figma | `file_key` (string, requis) |
| `figma_get_node` | Obtenir un noeud spécifique dans un fichier Figma | `file_key` (string, requis), `node_id` (string, requis) |
| `figma_get_styles` | Obtenir tous les styles publiés d'un fichier | `file_key` (string, requis) |

## Exemples d'utilisation

```
# Obtenir la structure du fichier
figma_get_file(file_key: "abc123XYZ")

# Obtenir un frame ou composant specifique
figma_get_node(file_key: "abc123XYZ", node_id: "1:42")

# Obtenir les design tokens (couleurs, typographie, espacement)
figma_get_styles(file_key: "abc123XYZ")
```

## Utilisé par

L'agent `designer` à l'accès MCP Figma active par défaut. Les autres agents n'y ont pas accès.

Voir le [guide d'intégration Figma](../guides/figma-integration.fr.md) pour des exemples de workflows.
