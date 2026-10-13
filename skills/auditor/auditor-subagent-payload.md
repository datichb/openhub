---
name: auditor-subagent-payload
description: "Utiliser en fin d'audit d'un domaine : statuts de ton retour et sections de ton rapport (périmètre, problèmes, risque)."
statuses: [corrections-requises, acceptable, bloquant]
when: role=subagent
---

# Payload de l'auditor-subagent

## Statuts

| Statut | Condition |
|---|---|
| `corrections-requises` | Au moins un problème 🔴 ou 🟠 : corrections nécessaires avant la mise en production |
| `acceptable` | Seulement des problèmes 🟡, ou aucun : corrections en backlog |
| `bloquant` | Un problème 🔴 empêche tout déploiement immédiat |

## Rapport

Titre : `Audit <domaine> — <périmètre>`, au format de `audit-protocol-light` (score, criticités, findings), toujours produit, même sans problème. Chaque section vide porte une mention explicite. Sections :

- `### Périmètre audité` : couvert et non couvert (avec la raison).
- `### Synthèse des problèmes identifiés` : sévérité, problème, localisation.
- `### Recommandations priorisées` : action concrète et effort estimé.
- `### Risque résiduel si non corrigé`.
- `### Observations détaillées` : par problème, localisation, description, preuve ou chemin d'exploitation (jamais minimisé), impact, recommandation.
- `### Points positifs`, `### Enrichissements wiki proposés`.
