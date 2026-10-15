> [Read in English](gslides-integration.en.md)

# Intégration Google Slides — Guide de démarrage

## Présentation

Le serveur MCP Google Slides fournit un accès en lecture seule aux présentations Google Slides pour les agents IA. Il permet aux agents d'extraire le contenu, d'analyser la structure des diapositives et de comprendre la mise en page des présentations.

### Fonctionnalités

- **Métadonnées de présentation** : récupérer le titre, le nombre de diapositives et les informations de mise en page
- **Extraction du contenu des diapositives** : lire le contenu de chaque diapositive (texte, formes, métadonnées des images)

---

## Prérequis

- Un jeton d'accès Google OAuth 2.0 avec le scope `https://www.googleapis.com/auth/presentations.readonly`
- Le jeton doit être renouvelé periodiquement (les jetons Google OAuth expirent après environ 1 heure)

> **Note :** Cette intégration nécessite un jeton Bearer OAuth, et non une clé API Google. Consultez la [documentation Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2) pour la génération de jetons.

---

## Installation

### 1. Configuration via `oh mcp setup gslides`

```bash
oh mcp setup gslides
```

L'assistant interactif vous demandera votre jeton d'accès Google.

### 2. Configuration manuelle (alternative)

Définissez la variable d'environnement :

```bash
export GOOGLE_ACCESS_TOKEN="ya29.a0..."
```

Ou configurez dans `hub.toml` :

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

Aucun redéploiement nécessaire (`oh deploy` supprimé en v5) : le changement est pris en compte au prochain lancement de session, quand le paquet de session est reconstruit :

```bash
oh run <workflow>
```

---

## Configuration dans hub.toml

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

> **Sécurité :** Stockez le jeton dans le trousseau du système via `oh secrets set GOOGLE_ACCESS_TOKEN` au lieu de l'écrire dans `hub.toml`. Le serveur MCP lit la variable d'environnement au démarrage.

---

## Outils disponibles

| Outil | Description | Utilisé par |
|-------|-------------|-------------|
| `gslides_get_presentation` | Récupérer les métadonnées et la liste des diapositives | Documentarian |
| `gslides_get_slide` | Récupérer le contenu d'une diapositive spécifique | Documentarian |

### `gslides_get_presentation`

**Paramètres :**
- `presentation_id` (string, **requis**) — L'identifiant de la présentation Google Slides (depuis l'URL : `docs.google.com/presentation/d/{ID}/edit`)

**Retourne :** JSON avec les métadonnées de la présentation incluant le titre, la langue, les dimensions des diapositives et la liste de toutes les diapositives avec leurs identifiants.

### `gslides_get_slide`

**Paramètres :**
- `presentation_id` (string, **requis**) — L'identifiant de la présentation
- `slide_id` (string, **requis**) — L'identifiant de page de la diapositive (obtenu via `gslides_get_presentation`)

**Retourne :** JSON avec l'arbre complet des éléments de la diapositive (blocs de texte, formes, images, tableaux).

---

## Exemples d'utilisation

**Extraire le contenu d'une présentation :**
```
"Analyse les diapositives de la presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 et resume les points cles"
```

**Récupérer une diapositive spécifique :**
```
"Recupere la diapositive p3 de la presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 et decris sa mise en page"
```

---

## Dépannage

### Jeton expiré

```
Error: 401 Unauthorized
```

Les jetons Google OAuth expirent après environ 1 heure. Renouvelez votre jeton et mettez à jour la variable d'environnement ou l'entrée du trousseau.

### Présentation introuvable

```
Error: 404 Not Found
```

Vérifiez que l'identifiant de la présentation est correct et que le jeton a accès à la présentation. La présentation doit être partagée avec le compte Google qui a généré le jeton.

### Permissions insuffisantes

```
Error: 403 Forbidden
```

Assurez-vous que le jeton OAuth a été généré avec le scope `presentations.readonly`. Les jetons avec uniquement le scope `drive.readonly` ne peuvent pas accéder aux endpoints de l'API Slides.

---

## Limitations actuelles

- **Lecture seule** — Pas de création ni de modification de présentations
- **Pas d'export** — Impossible d'exporter les diapositives en PDF ou en images
- **Taille de réponse limitée** — Les réponses sont limitées à 50 Mo
- **Gestion des jetons** — Le renouvellement du jeton OAuth est manuel ; le serveur MCP ne renouvelle pas automatiquement les jetons

---

## Ressources

- [Référence de l'API Google Slides](https://developers.google.com/slides/api/reference/rest)
- [Documentation Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)
- [Référence MCP Google Slides](../reference/mcp-gslides.fr.md)
- [Référence CLI](../reference/cli.fr.md)

---

## Support

```bash
oh mcp status gslides    # Verifier le statut du serveur
oh mcp setup gslides     # Reconfigurer
```

Pour signaler un problème : [GitHub Issues](https://github.com/datichb/openhub/issues)
