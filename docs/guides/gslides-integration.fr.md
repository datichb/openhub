> [Read in English](gslides-integration.en.md)

# Integration Google Slides — Guide de demarrage

## Presentation

Le serveur MCP Google Slides fournit un acces en lecture seule aux presentations Google Slides pour les agents IA. Il permet aux agents d'extraire le contenu, d'analyser la structure des diapositives et de comprendre la mise en page des presentations.

### Fonctionnalites

- **Metadonnees de presentation** : recuperer le titre, le nombre de diapositives et les informations de mise en page
- **Extraction du contenu des diapositives** : lire le contenu de chaque diapositive (texte, formes, metadonnees des images)

---

## Prerequis

- Un jeton d'acces Google OAuth 2.0 avec le scope `https://www.googleapis.com/auth/presentations.readonly`
- Le jeton doit etre renouvele periodiquement (les jetons Google OAuth expirent apres environ 1 heure)

> **Note :** Cette integration necessite un jeton Bearer OAuth, et non une cle API Google. Consultez la [documentation Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2) pour la generation de jetons.

---

## Installation

### 1. Configuration via `oh mcp setup gslides`

```bash
oh mcp setup gslides
```

L'assistant interactif vous demandera votre jeton d'acces Google.

### 2. Configuration manuelle (alternative)

Definissez la variable d'environnement :

```bash
export GOOGLE_ACCESS_TOKEN="ya29.a0..."
```

Ou configurez dans `hub.toml` :

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

Puis deployez :

```bash
oh deploy
```

---

## Configuration dans hub.toml

```toml
[mcp.gslides]
enabled = true
env = { GOOGLE_ACCESS_TOKEN = "ya29.a0..." }
```

> **Securite :** Stockez le jeton dans le trousseau du systeme via `oh secrets set GOOGLE_ACCESS_TOKEN` au lieu de l'ecrire dans `hub.toml`. Le serveur MCP lit la variable d'environnement au demarrage.

---

## Outils disponibles

| Outil | Description | Utilise par |
|-------|-------------|-------------|
| `gslides_get_presentation` | Recuperer les metadonnees et la liste des diapositives | Documentarian |
| `gslides_get_slide` | Recuperer le contenu d'une diapositive specifique | Documentarian |

### `gslides_get_presentation`

**Parametres :**
- `presentation_id` (string, **requis**) — L'identifiant de la presentation Google Slides (depuis l'URL : `docs.google.com/presentation/d/{ID}/edit`)

**Retourne :** JSON avec les metadonnees de la presentation incluant le titre, la langue, les dimensions des diapositives et la liste de toutes les diapositives avec leurs identifiants.

### `gslides_get_slide`

**Parametres :**
- `presentation_id` (string, **requis**) — L'identifiant de la presentation
- `slide_id` (string, **requis**) — L'identifiant de page de la diapositive (obtenu via `gslides_get_presentation`)

**Retourne :** JSON avec l'arbre complet des elements de la diapositive (blocs de texte, formes, images, tableaux).

---

## Exemples d'utilisation

**Extraire le contenu d'une presentation :**
```
"Analyse les diapositives de la presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 et resume les points cles"
```

**Recuperer une diapositive specifique :**
```
"Recupere la diapositive p3 de la presentation 1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgVE2xtG0 et decris sa mise en page"
```

---

## Depannage

### Jeton expire

```
Error: 401 Unauthorized
```

Les jetons Google OAuth expirent apres environ 1 heure. Renouvelez votre jeton et mettez a jour la variable d'environnement ou l'entree du trousseau.

### Presentation introuvable

```
Error: 404 Not Found
```

Verifiez que l'identifiant de la presentation est correct et que le jeton a acces a la presentation. La presentation doit etre partagee avec le compte Google qui a genere le jeton.

### Permissions insuffisantes

```
Error: 403 Forbidden
```

Assurez-vous que le jeton OAuth a ete genere avec le scope `presentations.readonly`. Les jetons avec uniquement le scope `drive.readonly` ne peuvent pas acceder aux endpoints de l'API Slides.

---

## Limitations actuelles

- **Lecture seule** — Pas de creation ni de modification de presentations
- **Pas d'export** — Impossible d'exporter les diapositives en PDF ou en images
- **Taille de reponse limitee** — Les reponses sont limitees a 50 Mo
- **Gestion des jetons** — Le renouvellement du jeton OAuth est manuel ; le serveur MCP ne renouvelle pas automatiquement les jetons

---

## Ressources

- [Reference de l'API Google Slides](https://developers.google.com/slides/api/reference/rest)
- [Documentation Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)
- [Reference MCP Google Slides](../reference/mcp-gslides.fr.md)
- [Reference CLI](../reference/cli.fr.md)

---

## Support

```bash
oh mcp status gslides    # Verifier le statut du serveur
oh mcp setup gslides     # Reconfigurer
```

Pour signaler un probleme : [GitHub Issues](https://github.com/datichb/openhub/issues)
