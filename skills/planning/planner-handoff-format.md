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

1. **Le récapitulatif de planification complet** — présentation narrative des tickets créés, des dépendances identifiées, des hypothèses faites, des risques. **Ce récapitulatif doit être produit même si la planification est partielle ou bloquée.**
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
| bd-ZZ | <titre> | feature | P2 | audit-security | auditor-security | — | — |

**Total :** X tickets créés (Y epics + Z tickets fils)

### Dépendances
- `bd-YY` dépend de `bd-XX` : <raison — ex : le composant frontend consomme l'API créée par bd-XX>
- `bd-ZZ` peut être traité en parallèle de `bd-XX` et `bd-YY`
<"Aucune dépendance entre les tickets" si tous sont indépendants>

### Ordre de traitement
1. bd-XX — <raison : bloquant pour bd-YY / ticket fondation>
2. bd-YY, bd-ZZ — <parallélisables après bd-XX>
<Séquence exacte d'exécution que l'orchestrateur doit suivre sans interprétation>

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

- **Toujours produire le récapitulatif de planification complet** avant ce bloc — même si la planification est partielle ou bloquée. Le récapitulatif est obligatoire dans tous les cas.
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

### À la réception du bloc `## Retour vers orchestrator` du planner

1. **Afficher le récapitulatif de planification complet dans le texte de la discussion** (ne pas inclure dans l'outil `question`) avant de poser le CP-0 — ne jamais résumer.
2. **Utiliser le tableau `### Tickets créés`** comme source de vérité pour le CP-0 — ne pas relire les tickets un par un avec `bd show` si le tableau est complet.
3. **Vérifier la présence de tous les champs obligatoires** : `Tickets créés`, `Dépendances`, `Ordre de traitement`, `Hypothèses et ambiguïtés`, `Risques identifiés`, `Statut`.
   - Si l'un de ces champs est absent → demander explicitement au planner de compléter avant de continuer.
4. **Si le récapitulatif de planification complet est absent** (le bloc handoff est présent sans récapitulatif préalable) → demander explicitement au planner de produire le récapitulatif complet avant de continuer.
5. **Utiliser `### Ordre de traitement` comme instruction directe de séquençage** — ne pas recalculer l'ordre depuis les dépendances, suivre la séquence fournie sans interprétation.
6. **Utiliser la colonne `Agent prévu` pour router directement** — ne pas analyser les labels, le titre ou la description du ticket pour deviner l'agent approprié.
7. **Présenter les `### Hypothèses et ambiguïtés`** à l'utilisateur au CP-0 pour validation, si elles existent.
8. **Signaler les `### Risques identifiés`** dans le CP-0 pour que l'utilisateur en ait connaissance avant de démarrer.
9. **Utiliser le `### Statut`** pour conditionner la suite :
   - `planification-complète` → continuer vers CP-0 normalement
   - `planification-partielle` → signaler les points incomplets à l'utilisateur au CP-0
   - `bloqué` → ne pas continuer — demander à l'utilisateur comment débloquer

> ❌ Ne jamais construire le CP-0 à partir d'un retour sans ce bloc structuré.
> ❌ Ne jamais ignorer les hypothèses ou ambiguïtés — les présenter à l'utilisateur.
> ❌ Ne jamais accepter un bloc handoff sans récapitulatif de planification préalable — les deux sont obligatoires.
> ❌ Ne jamais recalculer l'ordre de traitement — utiliser `### Ordre de traitement` tel quel.
> ❌ Ne jamais deviner l'agent — utiliser la colonne `Agent prévu` du tableau.
