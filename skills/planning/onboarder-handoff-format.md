---
name: onboarder-handoff-format
description: Source de vérité pour le format de retour de l'onboarder vers l'orchestrator. Définit le bloc structuré unique à produire quand l'onboarder termine son exploration et est invoqué depuis l'orchestrator (Mode C). Le rapport d'onboarding est intégré dans le bloc. Injecté dans l'onboarder et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff onboarder → orchestrator

Ce skill est la **source de vérité** pour le format de retour de l'`onboarder` vers l'orchestrator.
Il est injecté dans l'`onboarder` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

---

## Format du bloc `## Retour vers orchestrator`

> **Template :** format défini dans `templates/onboarder-handoff-block.md` — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `contexte-établi` | Exploration complète, fichiers produits, contexte suffisant pour démarrer la feature |
| `contexte-partiel` | Exploration réalisée mais avec des zones d'incertitude significatives — feature démarrable avec précautions |
| `bloqué` | Exploration impossible ou contexte insuffisant pour démarrer — intervention manuelle requise |

---

## Règles pour le producteur (onboarder)

- **`### Rapport d'onboarding`** doit capturer les observations qualitatives — minimum 3-5 phrases
- **Renseigner toutes les sections** — même si vides, utiliser la mention explicite correspondante
- **Ne pas inventer** de conventions ou de stack — uniquement ce qui a été effectivement observé dans la codebase
- **Signaler honnêtement les zones d'incertitude** — l'orchestrator en a besoin pour informer l'utilisateur avant de démarrer
- Ce bloc est produit **après** l'écriture des fichiers (ou après refus explicite de les écrire)
- Ne jamais omettre `### Rapport d'onboarding` — les listes structurées seules ne suffisent pas

---

## Règles pour le consommateur (orchestrator)

**Spécificités onboarder à vérifier :**

- **Champs obligatoires** : `Rapport d'onboarding`, `Stack technique`, `Contexte métier`, `Design et maquettes`, `Stratégie de test`, `Conventions identifiées`, `Dette technique détectée`, `Zones d'incertitude`, `Fichiers de contexte produits`, `Statut`. Si l'un est absent → demander à l'onboarder de compléter.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport d'onboarding` est affiché en premier.
- **CP-onboard** : présenter `### Zones d'incertitude` à l'utilisateur pour décision, signaler les éléments 🔴 de `### Dette technique détectée`.
- **Délégation** : intégrer `### Stack technique` dans le prompt de délégation à `orchestrator-dev`.
- **Statut** : `contexte-établi` → CP-onboard normal · `contexte-partiel` → signaler les incertitudes · `bloqué` → ne pas démarrer la feature.
