---
name: designer-payload
description: "Utiliser en fin de mission design : statuts de ton retour (spec, variations, prototype) et sections de ton rapport."
statuses: [spec-complète, spec-partielle, choix-requis, prototype-livré, bloqué]
when: role=subagent
---

# Payload du designer

## Statuts

| Statut | Condition |
|---|---|
| `spec-complète` | Tout ce qu'il faut pour implémenter est dans la spec |
| `spec-partielle` | Spec exploitable, avec des points ouverts à trancher (listés) |
| `choix-requis` | Design It Twice : 2 ou 3 variations proposées, le choix revient à l'utilisateur (dans `### Questions bloquantes`) |
| `prototype-livré` | Mode prototype : un artefact répond à une question précise, ce n'est pas une spec à implémenter |
| `bloqué` | Spec impossible à produire : la raison est dans `### Questions bloquantes` |

## Rapport

Titre : `Design <mode> — <feature>`. La spec n'est jamais résumée : c'est le livrable. Sections :

- `### Spec complète` : user flows, états, wireframes textuels, tokens, composants, critères d'acceptance.
- `### Contraintes d'implémentation`, `### Points ouverts`, `### Alternatives écartées`.
- Lancé par le `planner` : aussi `### Composants à créer`, `### Tokens design requis`, `### Dépendances design` et `### Questions pour l'utilisateur` (qu'il pose en phase 2).
- Mode prototype : `### Artefact`, `### Réponse à la question`, `### Recommandation` (spec complète nécessaire, aucun besoin, ou clarification).
- Mode recon : le retour de reconnaissance Figma de `figma-recon-protocol`.
