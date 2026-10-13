---
name: documentarian-payload
description: "Utiliser en fin de session de documentation : statuts de ton retour et sections de ton rapport (fichiers, résumé)."
statuses: [documenté, partiellement-documenté, bloqué]
when: role=subagent
---

# Payload du documentarian

## Statuts

| Statut | Condition |
|---|---|
| `documenté` | Documentation produite, fichiers mis à jour sans blocage |
| `partiellement-documenté` | Documentation incomplète : sections manquantes ou en attente de validation, listées dans le rapport |
| `bloqué` | Impossible de produire la documentation (structure introuvable, format inconnu, confirmation attendue) ; la raison est dans le résumé |

## Rapport

Titre : `Documentation — <sujet>` (avec `#<ID>` s'il y a un ticket). Le contenu documenté est écrit dans les fichiers, jamais recopié dans le rapport. Sections :

- `### Documentation produite` : type (`changelog`, `adr`, `api`, `readme`, `guide`, `slides`, `autre`) et fichiers modifiés, chacun avec son action (créé, mis à jour, section ajoutée).
- `### Résumé de l'entrée` : 1 à 3 phrases assez précises pour que ton appelant les reprenne dans son récap sans relire le fichier.
- `### Contexte et décisions` : les choix de format ou de structure et leur raison.
