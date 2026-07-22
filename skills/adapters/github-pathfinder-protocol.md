---
name: github-pathfinder-protocol
description: Protocole d'intégration GitHub pour l'agent Pathfinder — lecture d'un ticket pour affiner l'estimation de complexité, détection de PRs existantes sur le même périmètre
---

# Skill — GitHub Pathfinder Protocol (v1)

## Rôle

Ce skill enrichit le workflow du Pathfinder avec les données GitHub pour améliorer la précision des estimations et détecter les travaux déjà en cours sur le même périmètre.

## Étape 3bis — Vérification GitHub (optionnelle, après exploration codebase)

### Déclencheur

Activer si **au moins un** de ces critères :
- L'utilisateur a fourni un numéro de ticket (`#42`) ou de PR (`#15`)
- L'utilisateur a mentionné un dépôt GitHub
- La feature est décrite comme "ticket X" ou issue référencée

### Workflow

#### Cas A — Un ticket est fourni

```
Utiliser l'outil : github_get_issue
Arguments : owner, repo, issue_number
→ Obtenir : titre, description, labels, milestone, commentaires
```

**Exploiter pour affiner l'estimation :**

| Donnée GitHub | Impact sur l'estimation |
|---|---|
| Description longue avec ACs détaillés | +1 niveau de complexité si > 5 ACs |
| Labels `type: feature` + `area: frontend` + `area: backend` | Full-stack → +1 ticket minimum |
| Commentaires avec questions ouvertes | Signaler incertitudes dans le rapport |
| Milestone < 7 jours | Mentionner contrainte temporelle forte |
| Assigné à quelqu'un d'autre | Mentionner dans le rapport |

#### Cas B — Une PR est fournie

```
Utiliser l'outil : github_get_pr
Arguments : owner, repo, pull_number
→ Obtenir : titre, description, branches, état, labels, changements
```

**Exploiter pour :**
- Comprendre le périmètre si la PR est déjà en cours
- Estimer le delta restant si la PR est `open` et partiellement implémentée
- Signaler si la PR a des conflits (`mergeable: false`)

#### Cas C — Recherche de travaux similaires (optionnel)

Si la feature semble liée à des travaux existants :

```
Utiliser l'outil : github_list_issues
Arguments : owner, repo, state: "open", labels: <labels pertinents>
→ Vérifier : tickets déjà ouverts sur le même périmètre

Utiliser l'outil : github_list_prs
Arguments : owner, repo, state: "open"
→ Vérifier : PRs en cours sur le même périmètre
```

**Si ticket ou PR similaire trouvé :**
- Mentionner dans le rapport Pathfinder : "Ticket similaire détecté : #N"
- Recommander au planner de vérifier si c'est un doublon

### Format de sortie enrichi

Ajouter cette section dans le rapport Pathfinder si données GitHub disponibles :

```markdown
## 🐙 Contexte GitHub

**Ticket source :** #<number> — <titre>
**Labels :** <labels>
**Milestone :** <titre> (échéance : <date ou "aucune">)
**Complexité ajustée par GitHub :** <XS/S/M/L/XL> (depuis <estimation initiale>)

**Facteurs d'ajustement :**
- <raison 1>
- <raison 2>

**Points d'attention :**
- <blockers / questions ouvertes / dépendances détectées>
```

**Si aucune donnée GitHub :** ne pas inclure cette section.

### Gestion des erreurs

| Erreur | Comportement |
|---|---|
| Token invalide / expiré | Afficher : `⚠️ Token GitHub invalide — vérifier : GITHUB_TOKEN` |
| Ticket non trouvé (404) | Mentionner dans le rapport, continuer sans données GitHub |
| Pas de credentials configurés | Skiper silencieusement, continuer l'estimation sans GitHub |
