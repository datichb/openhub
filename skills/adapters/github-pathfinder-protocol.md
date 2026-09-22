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

> **Protocole d'intégration tracker :** voir skill `shared/tracker-integration-protocol` pour le workflow, la table d'impact, et la gestion d'erreurs.
>
> Paramètres pour cet adapter : `{platform}` = GitHub, `{platform_emoji}` = 🐙, `{tool_read_issue}` = github_get_issue, `{tool_list_issues}` = github_list_issues.

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
Utiliser l'outil : github_list_prs
Arguments : owner, repo, state: "open"
→ Vérifier : PRs en cours sur le même périmètre
```

**Si ticket ou PR similaire trouvé :**
- Mentionner dans le rapport Pathfinder : "Ticket similaire détecté : #N"
- Recommander au planner de vérifier si c'est un doublon
