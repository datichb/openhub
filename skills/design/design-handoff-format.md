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
tu **dois** produire dans cet ordre :

1. **La spec complète** — user flows intégraux avec tous les états, wireframes textuels, tokens, composants, critères d'acceptance UX/UI. **Cette spec doit être produite dans sa totalité, jamais résumée, même si elle est longue.** Elle est produite après la validation explicite de l'utilisateur.
2. **Le bloc `## Retour vers orchestrator`** défini ci-dessous — synthèse structurée avec les métadonnées, contraintes et statut.

En standalone ou quand invoqué depuis le `planner`, la spec est produite sans ce bloc.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit la spec complète avant ce bloc ? Si non, la produire d'abord. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** ux-designer | ui-designer
**Ticket :** #<ID> — <titre>

### Spec produite
Voir spec complète ci-dessus — jamais résumée ni reproduite ici.

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

- **Toujours produire la spec complète** avant ce bloc — jamais résumée ni abrégée. La spec est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite de la spec, même si le statut est `bloqué`
- **Le champ `### Spec produite`** dans le bloc pointe vers la spec ci-dessus — ne pas la reproduire dans le bloc
- Le bloc est produit **après** la validation explicite de l'utilisateur, pas avant
- Si invoqué depuis l'orchestrator via `Task`, utiliser ce format à la place du `bd close` habituel
- Le `task_id` n'est pas requis dans ce format (contrairement au format `orchestrator-dev`) — l'orchestrator reprend naturellement après réception

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit la spec complète.
> ❌ Ne jamais résumer la spec — le bloc est une synthèse de métadonnées, pas un substitut à la spec.

---

## Règles pour le consommateur (orchestrator)

### À la réception du retour d'un agent design

1. **Afficher la spec complète** dans la discussion avant de poser le CP-spec — ne jamais résumer.
2. **Afficher l'intégralité du bloc** dans la discussion.
3. **Vérifier la présence de tous les champs obligatoires** : `Contraintes d'implémentation`, `Points ouverts`, `Statut`.
   - Si l'un de ces champs est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander explicitement à l'agent design de compléter avant de continuer.
4. **Si la spec complète est absente** (le bloc handoff est présent sans spec préalable) → demander explicitement à l'agent design de produire la spec complète avant de continuer.
5. **Intégrer les `### Contraintes d'implémentation`** dans le prompt de délégation à `orchestrator-dev` lors de la phase d'implémentation.
6. **Signaler les `### Points ouverts`** à l'utilisateur lors du CP-spec pour décision avant implémentation.
7. **Utiliser le `### Statut`** pour conditionner la suite :
   - `spec-complète` ou `spec-partielle` → continuer vers CP-spec normalement
   - `bloqué` → ne pas router vers orchestrator-dev — demander à l'utilisateur comment débloquer

> ❌ Ne jamais construire le CP-spec à partir d'un retour incomplet ou sans ce bloc structuré.
> ❌ Ne jamais résumer la spec avant de la présenter à l'utilisateur au CP-spec.
> ❌ Ne jamais accepter un bloc handoff sans spec préalable — les deux sont obligatoires.
