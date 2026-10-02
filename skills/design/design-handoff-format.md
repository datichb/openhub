---
name: design-handoff-format
description: Source de vérité pour le format de retour de l'agent designer vers l'orchestrator. Définit le bloc structuré unique à produire quand le designer termine sa spec et est invoqué depuis l'orchestrator. La spec complète est intégrée dans le bloc. Injecté dans designer et orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff design → orchestrator

Ce skill est la **source de vérité** pour le format de retour de l'agent designer vers l'orchestrator.
Il est injecté dans `designer` et `orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

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

- **`### Spec complète`** ne doit JAMAIS être résumée ou abrégée, même si longue — c'est le livrable principal
- Le bloc est produit **après** la validation explicite de l'utilisateur, pas avant
- Si invoqué depuis l'orchestrator via `Task`, utiliser ce format à la place du `bd close` habituel
- Le `task_id` n'est pas requis dans ce format (contrairement au format `orchestrator-dev`) — l'orchestrator reprend naturellement après réception

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
- **Statut** : `spec-complète` ou `spec-partielle` → CP-spec normal · `bloqué` → ne pas router vers `orchestrator-dev` · `prototype-livré` → la réponse est dans le bloc, pas de spec à implémenter · `choix-requis` → le designer propose 2-3 variations UI et attend un choix (voir Design It Twice dans `ui-protocol`) — l'orchestrateur relaie le choix à l'utilisateur et ré-invoque le designer avec la réponse.

---

## Bloc `## Retour prototype vers orchestrator` (mode prototype uniquement)

Produit quand le skill `prototype-protocol` est chargé. Ce bloc remplace le bloc standard `## Retour vers orchestrator` — il ne contient pas de spec complète mais un artefact ciblé répondant à une question précise.

> **Template :** format défini dans `templates/designer-handoff-blocks.md` (section « Bloc Prototype ») — charger via `read` quand tu produis ce bloc.

**Règles pour le consommateur (orchestrator) :**
- Le prototype n'est PAS une spec à implémenter — c'est une réponse à une question
- La `### Recommandation` indique la suite : spec complète nécessaire, aucun besoin, ou clarification supplémentaire
- Si recommandation = "spec complète" → proposer une délégation design classique (ux/ui/ux+ui)
