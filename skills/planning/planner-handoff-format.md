---
name: planner-handoff-format
description: Source de vérité pour le format de retour du planner vers l'orchestrator. Définit le bloc structuré unique à produire quand le planner termine sa session de planification et est invoqué depuis l'orchestrator. Le récapitulatif de planification est intégré dans le bloc. Injecté dans le planner et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff planner → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `planner` vers l'orchestrator.
Il est injecté dans le `planner` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis l'`orchestrator`, ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le récapitulatif de planification (contexte, raisonnement, justification des choix) est **intégré dans le bloc** (section `### Récapitulatif de planification`), pas produit séparément en texte libre.

En standalone, le bloc est également le seul output après la Phase 4 (vérification + validation finale).

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que le récapitulatif est bien dans la section `### Récapitulatif de planification` du bloc. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** planner
**Feature :** <nom de la feature planifiée>

### Récapitulatif de planification

<Contexte narratif de la décomposition : pourquoi ces tickets, pourquoi cet ordre, quels compromis, quelles hypothèses faites. Ce texte apporte le "pourquoi" qui ne peut pas être encodé dans le tableau ci-dessous. Minimum 3-5 phrases pour toute planification non triviale.>

<Ex : "La feature a été décomposée en 3 tickets séquentiels car le middleware JWT dépend du service d'authentification. L'endpoint login a été priorisé car il est bloquant pour bd-43 et bd-44. Le stockage en localStorage a été choisi comme hypothèse par défaut faute de précision dans la demande — les cookies httpOnly seraient une alternative plus sécurisée.">

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

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **Le récapitulatif est DANS le bloc** (section `### Récapitulatif de planification`) — ne pas le produire séparément en texte libre
- **`### Récapitulatif de planification`** doit capturer le raisonnement et les décisions — minimum 3-5 phrases
- **Lister tous les tickets créés** dans le tableau — ne pas en omettre, même les tickets mineurs
- **Renseigner la colonne `Agent prévu`** pour chaque ticket — cette colonne est la **source de vérité pour le routing**, l'orchestrator ne doit pas avoir à deviner l'agent depuis les labels ou le contenu du ticket
- **Renseigner obligatoirement la section `### Ordre de traitement`** — cette section définit la séquence exacte d'exécution, l'orchestrator la suivra sans recalculer l'ordre depuis les dépendances
- **Signaler toute hypothèse** faite lors de la planification — l'orchestrator doit pouvoir la valider avec l'utilisateur
- Ce bloc est produit **après** la validation explicite du plan par l'utilisateur (après Phase 4)

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire de récapitulatif narratif séparé avant le bloc — il est DANS le bloc
> ❌ Ne jamais omettre `### Récapitulatif de planification` — le tableau seul ne suffit pas, le "pourquoi" est nécessaire

---

## Règles pour le consommateur (orchestrator)

**Spécificités planner à vérifier :**

- **Champs obligatoires** : `Récapitulatif de planification`, `Tickets créés`, `Dépendances`, `Ordre de traitement`, `Hypothèses et ambiguïtés`, `Risques identifiés`, `Statut`. Si l'un est absent → demander au planner de compléter avant de continuer.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Récapitulatif de planification` est affiché en premier pour donner le contexte.
- **Routing** : utiliser la colonne `Agent prévu` du tableau comme source de vérité — ne jamais analyser les labels ou le contenu du ticket pour deviner l'agent.
- **Séquençage** : suivre `### Ordre de traitement` tel quel — ne jamais recalculer depuis les dépendances.
- **CP-0** : présenter `### Hypothèses et ambiguïtés` à l'utilisateur pour validation, signaler `### Risques identifiés`.
- **Statut** : `planification-complète` → CP-0 normal · `planification-partielle` → signaler les incomplétudes · `bloqué` → ne pas continuer.
- **Optimisation** : ne pas relire les tickets un par un avec `bd show` si le tableau `### Tickets créés` est complet.
