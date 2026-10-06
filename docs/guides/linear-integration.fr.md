# Intégration Linear - Guide de démarrage

> 🇬🇧 [Read in English](linear-integration.en.md)

## Vue d'ensemble

L'intégration Linear connecte les agents à ton workspace Linear via l'**API GraphQL Linear**, permettant des workflows modernes de suivi d'issues. Linear est conçu pour les équipes d'ingénierie agiles avec un modèle de données épuré et des filtres puissants.

### Fonctionnalités

- **Lecture des issues** : description complète, état, priorité, assignee, commentaires
- **Création d'issues** (mode écriture) : créer des issues avec titre, description, équipe, priorité
- **Mise à jour d'issues** (mode écriture) : changer l'état ou l'assignee
- **Filtrage flexible** : par équipe, état ou assignee

---

## Prérequis

Tu as besoin d'une **Personal API Key Linear** :

1. Va dans Linear Paramètres > API > Personal API Keys
2. Clique sur **"Create new API key"**
3. Donne-lui un label (ex. `openhub`)
4. Copie la clé générée (format : `lin_api_xxxxxxxxxxxxxxxxxxxx`)

Stocke-la dans `LINEAR_API_KEY`.

---

## Setup

### 1. Configurer via `oh mcp setup`

```bash
oh mcp setup linear
```

Le wizard va :
1. Demander ta **Personal API Key Linear**
2. Valider la connexion à l'API GraphQL Linear
3. Stocker la clé de façon sécurisée dans le keychain système
4. Mettre à jour `hub.toml` avec le bloc `[mcp.linear]`

Vérifier le statut :
```bash
oh mcp status linear
```

### 2. Configuration manuelle (alternative)

```bash
export LINEAR_API_KEY=lin_api_xxxxxxxxxxxxxxxxxxxx
```

Ou dans `hub.toml` :

```toml
[mcp.linear]
enabled = true
# api_key peut aussi être défini via la variable LINEAR_API_KEY (recommandé)
write_enabled = false
```

---

## Configuration dans hub.toml

```toml
[mcp.linear]
enabled = true
write_enabled = false  # Mettre à true pour activer la création et mise à jour d'issues
```

Aucun redéploiement nécessaire (`oh deploy` supprimé en v5) : le changement est pris en compte au prochain lancement de session, quand le paquet de session est reconstruit :

```bash
oh run <workflow>
```

---

## Outils disponibles

| Outil | Description | Utilisé par |
|-------|-------------|-------------|
| `linear_list_issues` | Lister les issues avec filtres (équipe, état, assignee) | Planner, Pathfinder |
| `linear_get_issue` | Détails complets d'une issue (description, commentaires) | Planner, Pathfinder |
| `linear_create_issue` | Créer une nouvelle issue (mode écriture uniquement) | Planner |
| `linear_update_issue` | Mettre à jour l'état ou l'assignee (mode écriture uniquement) | Planner |

---

## Exemples de filtrage

Filtre les issues par équipe, état ou assignee dans les prompts agents :

```
"Liste toutes les issues In Progress dans l'équipe BACKEND"
"Montre-moi les issues assignées à alice dans l'équipe FRONTEND"
```

Ou avec des paramètres de filtre explicites passés via l'outil MCP :

```json
{
  "team_key": "BACKEND",
  "state": "In Progress",
  "assignee": "alice@macompagnie.com"
}
```

```json
{
  "team_key": "FRONTEND",
  "state": "Todo",
  "first": 25
}
```

---

## Mode écriture

Par défaut, le serveur MCP Linear est en lecture seule. Pour activer la création et la mise à jour d'issues :

```bash
export LINEAR_WRITE_ENABLED=true
```

Ou dans `hub.toml` :

```toml
[mcp.linear]
write_enabled = true
```

Cela déverrouille `linear_create_issue` et `linear_update_issue`.

> **Note :** La clé API doit appartenir à un membre du workspace ayant la permission de créer/modifier des issues dans l'équipe cible.

---

## Exemples d'utilisation

### Lister les tickets en cours

```
"Sur quoi l'équipe BACKEND travaille-t-elle actuellement ?"
"Liste toutes les issues In Progress dans l'équipe PLATFORM assignées à moi"
```

L'agent appelle `linear_list_issues` avec `team_key=BACKEND, state=In Progress` et présente un tableau récapitulatif avec les IDs, titres, assignees et priorités des issues.

### Mettre à jour l'état d'une issue

Avec le mode écriture activé :

```
"Marque LINEAR-42 comme Done"
"Passe FRONTEND-15 en In Review"
```

Le planner appelle `linear_update_issue` pour faire évoluer l'état de l'issue, en ajoutant un commentaire avec un résumé du travail effectué.

---

## Dépannage

### Clé API invalide

```
Error: Authentication failed — invalid Linear API key
```

Régénère la clé API dans Linear Paramètres > API > Personal API Keys, puis reconfigure :
```bash
oh mcp setup linear
```

### Équipe introuvable

```
Error: Team "XYZ" not found in your workspace
```

Vérifie la clé d'équipe dans les paramètres de ton workspace Linear. Les clés d'équipe sont sensibles à la casse (ex. `BACKEND`, pas `backend`).

### Erreurs GraphQL

L'API GraphQL de Linear retourne des messages d'erreur détaillés. Causes fréquentes :
- Paramètres de filtre malformés (vérifie les types : `first` est un entier)
- Tentative d'utiliser les outils d'écriture sans `write_enabled = true`

---

## Ressources

- [Documentation API Linear](https://developers.linear.app/docs/graphql/working-with-the-graphql-api)
- [Clés API Personal Linear](https://linear.app/settings/api)
- [Référence CLI `oh mcp`](../reference/mcp.fr.md)

---

## Support

- `oh mcp status linear` — vérifier la configuration
- `oh mcp setup linear` — reconfigurer le service
- Problème persistant → signaler sur [GitHub Issues](https://github.com/anomalyco/opencode)
