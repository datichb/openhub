---
name: developer-payload
description: "Utiliser en fin d'implémentation : statuts de ton retour et sections de ton rapport (diff, changements, critères)."
statuses: [implémenté, partiellement-implémenté, bloqué, BLOCKED_ARCHITECTURE]
when: role=subagent
---

# Payload du developer

## Statuts

| Statut | Condition |
|---|---|
| `implémenté` | Tous les critères d'acceptance couverts ; label `ready-for-review` ajouté |
| `partiellement-implémenté` | Une partie des critères couverts ; `ready-for-review` ajouté, manques listés dans le rapport |
| `bloqué` | Impossible sans déblocage externe ou sans décision (dans `### Questions bloquantes`) : `bd update <ID> -s blocked` et un commentaire `Bloqué par : <raison>` avant le retour |
| `BLOCKED_ARCHITECTURE` | Dérive architecturale détectée : implémentation arrêtée, options dans `### Questions bloquantes` |

## Rapport

Titre : `Implémentation #<ID> — <titre>` (ou la tâche reçue, sans ticket). Sections, dans l'ordre :

- `### Contexte et décisions` : au moins 2 entrées (choix et raison) pour un travail non trivial.
- `### Changements par fichier` : d'abord la sortie condensée de `git diff --stat` et `git status --porcelain` (le retour précède le commit), puis, par fichier, les symboles `+` ajouté, `~` modifié, `-` supprimé ; tests : les cas ajoutés ; configuration ou migration : une ligne de prose.
- `### Critères d'acceptance couverts` : chaque critère de `bd show <ID>`, coché ou non couvert avec sa raison. Sans ticket : chaque élément numéroté de la tâche (`F1`…), avec son test et ses fichiers.
- `### Points d'attention pour la review` : zones fragiles, compromis, fichiers supprimés.
- `### Migration destructive` (si présente), `### Blocages rencontrés`, `### Enrichissements wiki proposés` : les identifiants `wp-…` de tes `wiki_propose` (sans l'outil, les découvertes elles-mêmes).
