---
name: orchestrator-dev-payload
description: "Utiliser en fin de boucle de tickets lancée par un agent : statuts de ton retour et sections de ton rapport."
statuses: [succès, partiel, bloqué]
when: role=subagent
---

# Payload de l'orchestrator-dev

## Statuts

| Statut | Condition |
|---|---|
| `succès` | Tous les tickets traités ont été commités, sans blocage persistant |
| `partiel` | Au moins un ticket ignoré, ou passé au checkpoint de commit avec des points non résolus (plafond des tours de correction atteint) |
| `bloqué` | Au moins un ticket reste bloqué et demande une intervention manuelle |

Clarification hors checkpoint (dépendance non résolue, ticket bloqué, mêmes findings sur deux tours, agent requis absent du workflow) : statut `en-attente`, la question et ses options dans `### Questions bloquantes`, l'état de la boucle (traités, en cours, restants) dans le résumé. Tes checkpoints, eux, passent toujours par `workflow_checkpoint`.

## Rapport

Titre : `Implémentation — <feature ou tickets>`. Sections :

- `### Détail par ticket` : ID, titre, agent, verdict de la review, commit, statut.
- `### Contexte et décisions par ticket` : les décisions des retours du developer et du reviewer, reprises telles quelles.
- `### Points d'attention globaux`, `### Données techniques brutes` (si utiles).
- `### Rapports` : les identifiants `rp-…` reçus des agents lancés, par ticket.
