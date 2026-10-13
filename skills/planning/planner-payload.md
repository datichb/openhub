---
name: planner-payload
description: "Utiliser en fin de planification : statuts de ton retour et sections de ton rapport (tickets, ordre, hypothèses)."
statuses: [découpage-à-valider, planification-complète, planification-partielle, bloqué]
when: role=subagent
---

# Payload du planner

## Statuts

| Statut | Condition |
|---|---|
| `découpage-à-valider` | Découpage prêt, aucun ticket créé : tu attends sa validation (création verrouillée jusqu'au checkpoint de ton appelant) |
| `planification-complète` | Tickets créés, validés et prêts à être routés |
| `planification-partielle` | Tickets créés, mais certains incomplets ou des points restent à préciser (listés) |
| `bloqué` | Planification impossible à finaliser : la raison et les options sont dans `### Questions bloquantes` |

## Rapport

Titre : `Planification — <feature>`. Sections :

- `### Récapitulatif de planification` : le raisonnement et les décisions, en 3 à 5 phrases au moins.
- `### Tickets créés` (ou `### Tickets proposés` avant validation) : tableau ID, titre, type, priorité, **Agent prévu** (`developer (<domaine>)` : frontend, backend, fullstack, api, mobile, data, devops, platform, security, refactor, migration, tests) ; c'est la source du routing.
- `### Dépendances`.
- `### Ordre de traitement` : la séquence exacte, suivie telle quelle par ton appelant.
- `### Hypothèses et ambiguïtés` : chaque hypothèse faite, à valider avec l'utilisateur.
- `### Estimation globale`, `### Risques identifiés`.
