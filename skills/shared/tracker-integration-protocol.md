---
name: tracker-integration-protocol
description: "Protocole d'intégration tracker (GitLab/GitHub) — triggers, workflow lecture ticket, table d'impact, gestion erreurs."
---

# Protocole d'intégration tracker

Ce protocole définit le comportement commun pour l'intégration avec les trackers de tickets (GitLab, GitHub). Les adapters spécifiques (`adapters/gitlab-*`, `adapters/github-*`) fournissent les noms d'outils MCP et les mappings de champs.

## Déclencheurs

Activer si **au moins un** de ces critères :
- L'utilisateur a fourni un numéro de ticket ou de MR/PR
- L'utilisateur a mentionné un projet/dépôt spécifique
- La feature est décrite comme "ticket X" ou "issue X"

## Workflow — lecture du contexte ticket

### Cas A — Un ticket est fourni

1. Lire le ticket via l'outil MCP approprié (`{tool_read_issue}`)
2. Extraire : titre, description, labels, milestone, commentaires
3. Exploiter pour affiner l'analyse :

| Donnée | Impact |
|--------|--------|
| Description longue avec ACs détaillés | +1 niveau de complexité si > 5 ACs |
| Labels type + area (multi-domaine) | Full-stack → +1 ticket minimum |
| Commentaires avec questions ouvertes | Signaler incertitudes dans le rapport |
| Milestone < 7 jours | Mentionner contrainte temporelle forte |

### Cas B — Pas de ticket, un projet est fourni

1. Explorer les tickets ouverts via l'outil MCP de listing (`{tool_list_issues}`)
2. Identifier les tickets pertinents par titre/labels
3. Résumer le contexte projet

## Gestion des erreurs

| Erreur | Comportement |
|--------|-------------|
| Token invalide / expiré | Afficher : `⚠️ Token {platform} invalide — vérifier la configuration` |
| Ressource non trouvée (404) | Mentionner dans le rapport, continuer sans le contexte tracker |
| Pas de credentials configurés | Skipper silencieusement, continuer sans enrichissement tracker |

## Format de sortie — section enrichissement

Ajouter une section dans le rapport :

```
## {platform_emoji} Contexte {platform}

**Ticket source :** #{id} — {titre}
**Labels :** {labels}
**Milestone :** {titre} (échéance : {date ou "aucune"})
```

Les paramètres `{platform}`, `{platform_emoji}`, `{tool_read_issue}`, `{tool_list_issues}` sont fournis par l'adapter spécifique.
