---
name: github-planner-protocol
description: Protocole d'intégration GitHub pour l'agent Planner — lecture du ticket source, exploitation des labels et milestones pour contextualiser la décomposition en sous-tickets
---

# Skill — GitHub Planner Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 (Exploration contextuelle) du Planner avec les données GitHub pour ancrer la décomposition dans le contexte réel du projet : ticket source, priorité, contraintes de sprint.

## Phase 1.2bis — Exploration GitHub (optionnelle)

Cette phase se place **après Phase 1.2 (Codebase)** et **avant Phase 2 (Questions)**.

### Déclencheur

Lancer Phase 1.2bis si **au moins un** de ces critères :
- L'utilisateur a fourni un numéro de ticket GitHub (ex : `#42`, `#15`)
- L'utilisateur a mentionné un dépôt GitHub (`owner/repo`)
- La feature est décrite comme "ticket X" ou "issue X"

### Workflow

> **Protocole d'intégration tracker :** voir skill `shared/tracker-integration-protocol` pour le workflow, la table d'impact, et la gestion d'erreurs.
>
> Paramètres pour cet adapter : `{platform}` = GitHub, `{platform_emoji}` = 🐙, `{tool_read_issue}` = github_get_issue, `{tool_list_issues}` = github_list_issues.

#### Étape 2 : Comprendre le contexte projet (si première utilisation)

Si les labels ou milestones ne sont pas encore connus :

```
Utiliser l'outil : github_list_issues
Arguments : owner, repo, state: "open", per_page: 1
→ Obtenir : aperçu des labels et milestones utilisés dans le projet

Utiliser l'outil : github_get_repo
Arguments : owner, repo
→ Obtenir : informations générales sur le dépôt (langue principale, topics, description)
```

**Exploiter pour :**
- Comprendre la nomenclature de priorité du projet (`priority: high`, `P0`, etc.)
- Identifier le milestone actuel et sa date de fin
- Évaluer l'urgence de la feature

#### Étape 4 : Enrichissement du récap Phase 1

Ajouter cette section dans le récap si données GitHub disponibles :

```markdown
## 🐙 Contexte GitHub

**Ticket source :** #<number> — <titre>
**URL :** <html_url>
**Labels :** <labels>
**Milestone :** <titre> (échéance : <date>)
**Priorité détectée :** <haute/moyenne/normale — déduite des labels>

**Critères d'acceptation extraits :**
<liste extraite de la description ou des commentaires>

**Tickets liés détectés :**
- #<number> — <titre> [<état>]
```

**Si aucune donnée GitHub :** ne pas inclure cette section.
