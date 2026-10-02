---
name: gitlab-planner-protocol
description: Protocole d'intégration GitLab pour l'agent Planner — lecture du ticket source, exploitation des labels et milestones pour contextualiser la décomposition en sous-tickets
---

# Skill — GitLab Planner Protocol (v1)

## Rôle

Ce skill enrichit la Phase 1 (Exploration contextuelle) du Planner avec les données GitLab pour ancrer la décomposition dans le contexte réel du projet : ticket source, priorité, contraintes de sprint.

## Phase 1.2bis — Exploration GitLab (optionnelle)

Cette phase se place **après Phase 1.2 (Codebase)** et **avant Phase 2 (Questions)**.

### Déclencheur

Lancer Phase 1.2bis si **au moins un** de ces critères :
- L'utilisateur a fourni un numéro de ticket GitLab (ex : `#42`, `!15`)
- L'utilisateur a mentionné un projet GitLab (`mon-groupe/mon-projet`)
- La feature est décrite comme "ticket X" ou "issue X"

### Workflow

> **Protocole d'intégration tracker :** voir skill `shared/tracker-integration-protocol` pour le workflow, la table d'impact, et la gestion d'erreurs.
>
> Paramètres pour cet adapter : `{platform}` = GitLab, `{platform_emoji}` = 🦊, `{tool_read_issue}` = gitlab_list_issues, `{tool_list_issues}` = gitlab_list_issues.

#### Étape 2 : Comprendre le contexte projet (si première utilisation)

Si les labels ou milestones ne sont pas encore connus, les extraire depuis les issues existantes :

```
Utiliser l'outil : gitlab_list_issues
Arguments : project_id, state: "opened"
→ Obtenir : les issues avec leurs labels, assignees et état
→ En déduire : taxonomie des labels (types, priorités, domaines)
```

> **Note :** Les outils `list_gitlab_labels` et `list_gitlab_milestones` ne sont plus disponibles
> dans le MCP GitLab v2. Extraire les labels depuis les issues retournées par `gitlab_list_issues`.

**Exploiter pour :**
- Comprendre la nomenclature de priorité du projet (`priority::high`, `P0`, etc.)
- Identifier le sprint actuel et sa date de fin
- Évaluer l'urgence de la feature

#### Étape 4 : Enrichissement du récap Phase 1

Ajouter cette section dans le récap si données GitLab disponibles :

```markdown
## 🦊 Contexte GitLab

**Ticket source :** #<iid> — <titre>
**URL :** <web_url>
**Labels :** <labels>
**Milestone :** <titre> (échéance : <date>)
**Priorité détectée :** <haute/moyenne/normale — déduite des labels>

**Critères d'acceptation extraits :**
<liste extraite de la description ou des commentaires>

**Tickets liés détectés :**
- #<iid> — <titre> [<état>]
```

**Si aucune donnée GitLab :** ne pas inclure cette section.
