---
name: design-handoff-format
description: Source de vérité pour le format de retour des agents ux-designer et ui-designer vers l'orchestrator. Définit le bloc structuré à produire quand un agent design termine sa spec et est invoqué depuis l'orchestrator. Injecté dans ux-designer, ui-designer et orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff design → orchestrator

Ce skill est la **source de vérité** pour le format de retour des agents design vers l'orchestrator.
Il est injecté dans `ux-designer`, `ui-designer` et `orchestrator` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis l'`orchestrator` (et non en standalone ou depuis le `planner`),
tu **dois** conclure ta session avec le bloc `## Retour vers orchestrator` défini ci-dessous,
**après** la validation explicite de la spec par l'utilisateur.

En standalone ou quand invoqué depuis le `planner`, tu produis ton format habituel — ce bloc n'est pas requis.

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** ux-designer | ui-designer
**Ticket :** #<ID> — <titre>

### Spec produite
<spec complète — user flow intégral avec tous les états, wireframes textuels, tokens, composants,
critères d'acceptance UX/UI — jamais résumée ni abrégée>

### Contraintes d'implémentation
- <contrainte 1 — ex : responsive mobile-first obligatoire, ratio de contraste WCAG AA minimum, etc.>
- <contrainte 2>
<"Aucune" si pas de contrainte spécifique identifiée>

### Points ouverts
- <question en suspens 1 — ce qui n'a pas été tranché et nécessite une décision avant ou pendant l'implémentation>
- <question en suspens 2>
<"Aucun" si tous les points ont été tranchés>

### Alternatives écartées
- `<alternative 1>` : <pourquoi écartée>
- `<alternative 2>` : <pourquoi écartée>
<"Aucune" si aucune alternative notable n'a été explorée>

### Statut
`spec-complète` | `spec-partielle` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `spec-complète` | Spec validée par l'utilisateur, tous les éléments nécessaires à l'implémentation sont présents |
| `spec-partielle` | Spec validée mais avec des points ouverts qui devront être résolus pendant l'implémentation |
| `bloqué` | Spec non finalisée — un blocage empêche de produire une spec exploitable |

---

## Règles pour le producteur (ux-designer / ui-designer)

- **Ne jamais résumer la spec** dans le bloc `### Spec produite` — reproduire intégralement le contenu de la spec validée
- Le bloc est produit **après** la validation explicite de l'utilisateur, pas avant
- Si invoqué depuis l'orchestrator via `Task`, utiliser ce format à la place du `bd close` habituel
- Le `task_id` n'est pas requis dans ce format (contrairement au format `orchestrator-dev`) — l'orchestrator reprend naturellement après réception

---

## Règles pour le consommateur (orchestrator)

### À la réception du bloc `## Retour vers orchestrator` d'un agent design

1. **Afficher la `### Spec produite` intégralement** dans la discussion avant de poser le CP-spec — ne jamais résumer.
2. **Vérifier la présence de tous les champs obligatoires** : `Spec produite`, `Contraintes d'implémentation`, `Points ouverts`, `Statut`.
   - Si l'un de ces champs est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander explicitement à l'agent design de compléter avant de continuer.
3. **Intégrer les `### Contraintes d'implémentation`** dans le prompt de délégation à `orchestrator-dev` lors de la phase d'implémentation.
4. **Signaler les `### Points ouverts`** à l'utilisateur lors du CP-spec pour décision avant implémentation.
5. **Utiliser le `### Statut`** pour conditionner la suite :
   - `spec-complète` ou `spec-partielle` → continuer vers CP-spec normalement
   - `bloqué` → ne pas router vers orchestrator-dev — demander à l'utilisateur comment débloquer

> ❌ Ne jamais construire le CP-spec à partir d'un retour incomplet ou sans ce bloc structuré.
> ❌ Ne jamais résumer la spec avant de la présenter à l'utilisateur au CP-spec.
