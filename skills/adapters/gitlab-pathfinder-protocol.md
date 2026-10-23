---
name: gitlab-pathfinder-protocol
description: "Utiliser si le MCP gitlab est présent et qu'un ticket est cité : lire le ticket, repérer les MR du même périmètre."
mcp: gitlab
---

# Skill — GitLab Pathfinder Protocol (v1)

## Rôle

Ce skill enrichit le workflow du Pathfinder avec les données GitLab pour améliorer la précision des estimations et détecter les travaux déjà en cours sur le même périmètre.

## Étape 3bis — Vérification GitLab (optionnelle, après exploration codebase)

### Déclencheur

Activer si **au moins un** de ces critères :
- L'utilisateur a fourni un numéro de ticket (`#42`) ou de MR (`!15`)
- L'utilisateur a mentionné un projet GitLab
- La feature est décrite comme "ticket X" ou issue référencée

### Workflow

> **Protocole d'intégration tracker :** voir skill `shared/tracker-integration-protocol` pour le workflow, la table d'impact, et la gestion d'erreurs.
>
> Paramètres pour cet adapter : `{platform}` = GitLab, `{platform_emoji}` = 🦊, `{tool_read_issue}` = gitlab_get_issue, `{tool_list_issues}` = gitlab_list_issues.

#### Cas B — Une MR est fournie

```
Utiliser l'outil : gitlab_get_mr
Arguments : project_id, mr_iid
→ Obtenir : titre, description, branches, état, labels, statut de fusion, tickets fermés

IID inconnu (seule la branche est connue) : gitlab_list_mrs
Arguments : project_id, source_branch (ou iids, search, state: "opened")
```

**Exploiter pour :**
- Comprendre le périmètre si la MR est déjà en cours
- Estimer le delta restant si la MR est `opened` et partiellement implémentée
- Signaler si la MR a des conflits (`has_conflicts: true`)
