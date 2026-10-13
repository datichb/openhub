---
name: debugger-payload
description: "Utiliser en fin de diagnostic : statuts de ton retour et sections de ton rapport (cause racine, impact, tickets)."
statuses: [diagnostiqué, partiellement-diagnostiqué, non-reproductible]
when: role=subagent
---

# Payload du debugger

## Statuts

| Statut | Condition |
|---|---|
| `diagnostiqué` | Cause racine établie avec des éléments probants ; ticket de correction créé (ou refusé) |
| `partiellement-diagnostiqué` | Hypothèse probable, sans certitude ; ticket créé avec les informations disponibles |
| `non-reproductible` | Bug non reproductible : artefacts insuffisants ou bug intermittent |

## Rapport

Titre : `Diagnostic — <bug>`. Toujours produit, même `non-reproductible` (il dit ce qui a été tenté). Aucune cause racine affirmée sans preuve ; l'impact n'est jamais minimisé. Sections :

- `### Actions d'urgence si bug en prod` : en premier (ou « N/A ») ; elles priment sur toute autre décision.
- `### Cause racine` : hypothèse retenue, niveau de certitude (confirmé, probable, incertain), chaîne causale.
- `### Hypothèses explorées` : écartée, confirmée ou insuffisamment documentée, avec la raison.
- `### Impact et régressions potentielles`.
- `### Tickets de correction créés` : ID, titre, priorité, labels (ou pourquoi aucun).
- `### Diagnostic détaillé` : symptôme, périmètre analysé, localisation, preuves par hypothèse, fichiers impliqués, informations manquantes.
