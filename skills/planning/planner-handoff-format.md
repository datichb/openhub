---
name: planner-handoff-format
description: Source de vérité pour le format de retour du planner vers l'orchestrator. Définit le bloc structuré à produire quand le planner termine sa session de planification et est invoqué depuis l'orchestrator. Injecté dans le planner et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff planner → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `planner` vers l'orchestrator.
Il est injecté dans le `planner` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis l'`orchestrator`, tu **dois** produire dans cet ordre :

1. **Le récapitulatif de planification complet** — présentation narrative du contexte et du raisonnement ayant mené aux décisions de décomposition : pourquoi ces tickets, pourquoi cet ordre, quelles hypothèses, quels risques identifiés. **Ce récapitulatif doit être produit même si la planification est partielle ou bloquée.** Il n'a pas à reproduire le tableau des tickets ni les listes formelles — ceux-ci sont dans le bloc structuré qui suit.
2. **Le bloc `## Retour vers orchestrator`** défini ci-dessous, après avoir terminé la Phase 4 (vérification + validation finale).

En standalone, le récapitulatif de planification précède également ce bloc.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le récapitulatif de planification complet avant ce bloc ? Si non, le produire d'abord. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** planner
**Feature :** <nom de la feature planifiée>

### Tickets créés

| ID | Titre | Type | Priorité | Labels | Agent prévu | TDD | Dépend de |
|----|-------|------|----------|--------|-------------|-----|-----------|
| bd-XX | <titre> | feature | P1 | <labels> | developer-backend | — | — |
| bd-YY | <titre> | task | P1 | <labels> | developer-frontend | ✅ | bd-XX |
| bd-ZZ | <titre> | feature | P2 | audit-security | auditor | — | — |

**Total :** X tickets créés (Y epics + Z tickets fils)

### Dépendances
- `bd-YY` dépend de `bd-XX` : <raison — ex : le composant frontend consomme l'API créée par bd-XX>
- `bd-ZZ` peut être traité en parallèle de `bd-XX` et `bd-YY`
<"Aucune dépendance entre les tickets" si tous sont indépendants>

### Ordre de traitement
1. bd-XX — <raison : bloquant pour bd-YY / ticket fondation>
2. bd-YY, bd-ZZ — <parallélisables après bd-XX>
<Séquence exacte d'exécution que l'agent orchestrator doit suivre sans interprétation>

### Hypothèses et ambiguïtés
- <hypothèse 1 — ce qui n'était pas explicite dans la demande et a été inféré>
- <ambiguïté 1 — point non tranché qui pourrait influencer l'implémentation>
<"Aucune" si la demande était complète et sans ambiguïté>

### Estimation globale
**Tickets :** X | **Complexité estimée :** <faible | moyenne | élevée>
<estimation en jours ou sprints si des signaux suffisants sont disponibles, sinon omettre>

### Risques identifiés
- <risque 1 — ex : dépendance externe non maîtrisée, ticket avec critères d'acceptance flous>
- <risque 2>
<"Aucun risque identifié" si le plan est clair et sans risque notable>

### Statut
`planification-complète` | `planification-partielle` | `bloqué`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `planification-complète` | Tous les tickets sont créés, validés et prêts à être routés |
| `planification-partielle` | Des tickets ont été créés mais certains sont incomplets ou des points restent à préciser |
| `bloqué` | La planification ne peut pas être finalisée — un blocage empêche la création des tickets |

---

## Règles pour le producteur (planner)

- **Toujours produire le récapitulatif de planification complet** avant ce bloc — même si la planification est partielle ou bloquée. Le récapitulatif est obligatoire dans tous les cas. Il apporte le **contexte et le raisonnement** (pourquoi ces tickets, pourquoi cet ordre, quelles hypothèses) — pas un ré-encodage du tableau du bloc structuré.
- **Toujours produire ce bloc** à la suite du récapitulatif, même si le statut est `bloqué`
- **Lister tous les tickets créés** dans le tableau — ne pas en omettre, même les tickets mineurs
- **Renseigner la colonne `Dépend de`** pour chaque ticket — mettre `—` si aucune dépendance
- **Renseigner obligatoirement la colonne `Agent prévu`** pour chaque ticket — cette colonne est la **source de vérité pour le routing**, l'orchestrator ne doit pas avoir à deviner l'agent depuis les labels ou le contenu du ticket
- **Renseigner obligatoirement la section `### Ordre de traitement`** — cette section définit la séquence exacte d'exécution, l'orchestrator la suivra sans recalculer l'ordre depuis les dépendances
- **Signaler toute hypothèse** faite lors de la planification — l'orchestrator doit pouvoir la valider avec l'utilisateur
- Ce bloc est produit **après** la validation explicite du plan par l'utilisateur (après Phase 4)

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit le récapitulatif de planification complet.
> ❌ Ne jamais résumer le récapitulatif — le bloc est un résumé structuré, pas un substitut.

---

## Règles pour le consommateur (orchestrator)

> Protocole de retranscription complet (séquence obligatoire, templates, checklist, exemples) → skill `posture/retranscription-coordinateur`.

**Spécificités planner à vérifier :**

- **Champs obligatoires** : `Tickets créés`, `Dépendances`, `Ordre de traitement`, `Hypothèses et ambiguïtés`, `Risques identifiés`, `Statut`. Si l'un est absent → demander au planner de compléter avant de continuer.
- **Routing** : utiliser la colonne `Agent prévu` du tableau comme source de vérité — ne jamais analyser les labels ou le contenu du ticket pour deviner l'agent.
- **Séquençage** : suivre `### Ordre de traitement` tel quel — ne jamais recalculer depuis les dépendances.
- **CP-0** : présenter `### Hypothèses et ambiguïtés` à l'utilisateur pour validation, signaler `### Risques identifiés`.
- **Statut** : `planification-complète` → CP-0 normal · `planification-partielle` → signaler les incomplétudes · `bloqué` → ne pas continuer.
- **Optimisation** : ne pas relire les tickets un par un avec `bd show` si le tableau `### Tickets créés` est complet.
