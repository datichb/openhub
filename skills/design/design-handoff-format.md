---
name: design-handoff-format
description: Source de vérité pour le format de retour de l'agent designer vers l'orchestrator. Définit le bloc structuré unique à produire quand le designer termine sa spec et est invoqué depuis l'orchestrator. La spec complète est intégrée dans le bloc. Injecté dans designer et orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff design → orchestrator

Ce skill est la **source de vérité** pour le format de retour de l'agent designer vers l'orchestrator.
Il est injecté dans `designer` et `orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

### Détection du contexte d'invocation

Au démarrage, charger le skill de parcours selon le contexte :

- Si le prompt contient `[SKILL:designer/designer-subagent]` → charger le skill correspondant via l'outil `skill`
  - Mémoriser **CONTEXTE = orchestrator_feature** pour toute la session
  - Ne jamais utiliser l'outil `question` — toute interaction passe par les blocs structurés
  - En fin de session : produire **uniquement** le bloc `## Retour vers orchestrator`
  - En cas de clarification critique nécessaire en cours de session : produire `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` et **terminer la session**
- Sinon (standalone ou depuis `planner`) :
  - Utiliser l'outil `question` normalement
  - Produire la spec sans le bloc `## Retour vers orchestrator`

---

Quand CONTEXTE = orchestrator_feature, ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Format de sortie :** aucun texte avant, après ou en dehors de ce bloc. La spec complète (user flows, wireframes textuels, tokens, composants, critères UX/UI) est **intégrée dans le bloc** (section `### Spec complète`), pas produite séparément en texte libre.

---

## Format du bloc `## Retour vers orchestrator`

> **Template :** format défini dans `templates/designer-handoff-blocks.md` (section « Bloc Retour vers orchestrator ») — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `spec-complète` | Spec validée par l'utilisateur, tous les éléments nécessaires à l'implémentation sont présents |
| `spec-partielle` | Spec validée mais avec des points ouverts qui devront être résolus pendant l'implémentation |
| `bloqué` | Spec non finalisée — un blocage empêche de produire une spec exploitable |

---

## Règles pour le producteur (designer)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **La spec complète est DANS le bloc** (section `### Spec complète`) — ne pas la produire séparément en texte libre
- **`### Spec complète`** ne doit JAMAIS être résumée ou abrégée, même si longue — c'est le livrable principal
- Le bloc est produit **après** la validation explicite de l'utilisateur, pas avant
- Si invoqué depuis l'orchestrator via `Task`, utiliser ce format à la place du `bd close` habituel
- Le `task_id` n'est pas requis dans ce format (contrairement au format `orchestrator-dev`) — l'orchestrator reprend naturellement après réception

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire la spec comme texte libre avant le bloc — elle est DANS le bloc
> ❌ Ne jamais résumer la spec dans `### Spec complète` — elle doit être exhaustive et exploitable

---

## Bloc `## Retour intermédiaire vers orchestrator` (clarification en cours de session)

Produit quand une **clarification critique** est nécessaire en cours de session (CONTEXTE = orchestrator_feature uniquement) — ex : aucun design system détecté, informations utilisateur insuffisantes, décision de direction artistique bloquante.

> ⚠️ Réserver aux vrais blockers. Formuler une hypothèse documentée et continuer si possible.

> **Template :** format défini dans `templates/designer-handoff-blocks.md` (section « Bloc Retour intermédiaire vers orchestrator ») — charger via `read` quand tu produis ce bloc.

---

## Bloc `## Question pour l'orchestrator` (clarification en cours de session)

Accompagne toujours un `## Retour intermédiaire vers orchestrator`.

> **Template :** format défini dans `templates/designer-handoff-blocks.md` (section « Bloc Question pour l'orchestrator ») — charger via `read` quand tu produis ce bloc.

---

## Règles pour le consommateur (orchestrator)

**Spécificités design à vérifier :**

- **Champs obligatoires** : `Spec complète`, `Contraintes d'implémentation`, `Points ouverts`, `Statut`. Si l'un est absent ou vide sans mention explicite (`"Aucun"` / `"Aucune"`) → demander à l'agent design de compléter avant de continuer.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). La `### Spec complète` est affichée intégralement.
- **Délégation** : intégrer `### Contraintes d'implémentation` dans le prompt de délégation à `orchestrator-dev`.
- **CP-spec** : signaler `### Points ouverts` à l'utilisateur pour décision avant implémentation.
- **Statut** : `spec-complète` ou `spec-partielle` → CP-spec normal · `bloqué` → ne pas router vers `orchestrator-dev`.
