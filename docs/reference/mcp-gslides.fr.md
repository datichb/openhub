> [Read in English](mcp-gslides.en.md)

# Référence serveur MCP Google Slides

Le serveur MCP Google Slides fournit un accès en lecture aux présentations Google Slides pour l'analyse.

## Configuration

**Requis :** Credentials OAuth Google (`GOOGLE_ACCESS_TOKEN`)

```toml
# hub.toml
[mcp.gslides]
enabled = true
token_key = "openhub.mcp.gslides.token"
```

## Outils

| Outil | Description | Paramètres |
|-------|-------------|------------|
| `gslides_get_presentation` | Obtenir les métadonnées de la présentation et la liste des slides | `presentation_id` (string, requis) |
| `gslides_get_slide` | Obtenir le contenu d'un slide spécifique | `presentation_id` (string, requis), `slide_id` (string, requis) |

## Exemples d'utilisation

```
# Obtenir la vue d'ensemble de la presentation
gslides_get_presentation(presentation_id: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtNs")

# Obtenir le contenu d'un slide specifique
gslides_get_slide(presentation_id: "1Bxi...", slide_id: "p")
```

## Utilisé par

Disponible pour les agents quand le serveur MCP `gslides` est activé. Utilisé principalement par l'agent `documentarian` pour l'analyse de présentations (voir [skill doc-slides](../architecture/skills.fr.md)).
