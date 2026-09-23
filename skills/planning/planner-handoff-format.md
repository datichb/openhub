---
name: planner-handoff-format
description: Source de vérité pour le format de retour du planner vers l'orchestrator. Définit le bloc structuré unique à produire quand le planner termine sa session de planification et est invoqué depuis l'orchestrator. Le récapitulatif de planification est intégré dans le bloc. Injecté dans le planner et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff planner → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `planner` vers l'orchestrator.
Il est injecté dans le `planner` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

En standalone, le bloc est également le seul output après la Phase 4 (vérification + validation finale).

---

## Format du bloc `## Retour vers orchestrator`

> **Template :** format défini dans `templates/planner-handoff-block.md` — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `planification-complète` | Tous les tickets sont créés, validés et prêts à être routés |
| `planification-partielle` | Des tickets ont été créés mais certains sont incomplets ou des points restent à préciser |
| `bloqué` | La planification ne peut pas être finalisée — un blocage empêche la création des tickets |

---

## Règles pour le producteur (planner)

- **`### Récapitulatif de planification`** doit capturer le raisonnement et les décisions — minimum 3-5 phrases
- **Lister tous les tickets créés** dans le tableau — ne pas en omettre, même les tickets mineurs
- **Renseigner la colonne `Agent prévu`** pour chaque ticket — cette colonne est la **source de vérité pour le routing**, l'orchestrator ne doit pas avoir à deviner l'agent depuis les labels ou le contenu du ticket
- **Renseigner obligatoirement la section `### Ordre de traitement`** — cette section définit la séquence exacte d'exécution, l'orchestrator la suivra sans recalculer l'ordre depuis les dépendances
- **Signaler toute hypothèse** faite lors de la planification — l'orchestrator doit pouvoir la valider avec l'utilisateur
- Ce bloc est produit **après** la validation explicite du plan par l'utilisateur (après Phase 4)
- Ne jamais omettre `### Récapitulatif de planification` — le tableau seul ne suffit pas, le "pourquoi" est nécessaire
- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.

---

## Règles pour le consommateur (orchestrator)

**Spécificités planner à vérifier :**

- **Champs obligatoires** : `Récapitulatif de planification`, `Tickets créés`, `Dépendances`, `Ordre de traitement`, `Hypothèses et ambiguïtés`, `Risques identifiés`, `Questions bloquantes`, `Statut`. Si l'un est absent → demander au planner de compléter avant de continuer.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Récapitulatif de planification` est affiché en premier pour donner le contexte.
- **Routing** : utiliser la colonne `Agent prévu` du tableau comme source de vérité — ne jamais analyser les labels ou le contenu du ticket pour deviner l'agent.
- **Séquençage** : suivre `### Ordre de traitement` tel quel — ne jamais recalculer depuis les dépendances.
- **CP-0** : présenter `### Hypothèses et ambiguïtés` à l'utilisateur pour validation, signaler `### Risques identifiés`.
- **Statut** : `planification-complète` → CP-0 normal · `planification-partielle` → signaler les incomplétudes · `bloqué` → ne pas continuer.
- **Optimisation** : ne pas relire les tickets un par un avec `bd show` si le tableau `### Tickets créés` est complet.
