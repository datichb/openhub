---
name: debugger-handoff-format
description: Source de vérité pour le format de retour du debugger vers l'orchestrator. Définit le bloc structuré unique à produire quand le debugger termine son diagnostic et est invoqué depuis l'orchestrator (Mode D). Le rapport de diagnostic complet est intégré dans le bloc. Injecté dans le debugger et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
annexes: [templates/debugger-handoff-block.md]
---

# Skill — Format de handoff debugger → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `debugger` vers l'orchestrator.
Il est injecté dans le `debugger` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.

---

## Format du bloc `## Retour vers orchestrator`

> **Template :** format défini dans `templates/debugger-handoff-block.md` — charger via `read` quand tu produis ce bloc.

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `diagnostiqué` | Cause racine identifiée avec certitude suffisante, ticket créé |
| `partiellement-diagnostiqué` | Hypothèse probable mais sans certitude — ticket créé avec les informations disponibles |
| `non-reproductible` | Bug non reproductible depuis la codebase — artefacts insuffisants ou bug intermittent |

---

## Règles pour le producteur (debugger)

- **Toujours inclure `### Rapport de diagnostic complet`** même si le diagnostic est `non-reproductible` — le rapport documente ce qui a été tenté
- **Ne jamais affirmer une cause racine sans éléments probants** — utiliser "confirmé / probable / incertain" dans `Niveau de certitude`
- **Renseigner toutes les sections** — même si vides, utiliser la mention explicite correspondante
- **Signaler honnêtement les hypothèses insuffisamment documentées** — l'orchestrator a besoin de cette information
- Ce bloc est produit **après** la création du ticket (ou après refus explicite de l'utilisateur)
- Ne jamais minimiser l'impact si des régressions sont possibles
- **`### Questions bloquantes`** (obligatoire) — liste numérotée des questions nécessitant une réponse avant de poursuivre. Si aucune : écrire `Aucune.`. Le coordinateur **doit** résoudre ces questions avant de continuer le workflow.

---

## Règles pour le consommateur (orchestrator)

**Spécificités debugger à vérifier :**

- **Champs obligatoires** : `Cause racine`, `Impact et régressions potentielles`, `Tickets de correction créés`, `Rapport de diagnostic complet`, `Questions bloquantes`, `Statut`. Si l'un est absent → demander au debugger de compléter avant de continuer.
- **Priorité absolue** : présenter `### Actions d'urgence si bug en prod` en premier si renseignées — elles priment sur toute autre décision.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport de diagnostic complet` est affiché intégralement.
- **Suite** : si des tickets ont été créés → proposer à l'utilisateur de les intégrer dans le workflow (Mode A ou B). Si aucun ticket (cause non déterminée) → informer et proposer les options.
- **Statut** : `diagnostiqué` → cause établie · `partiellement-diagnostiqué` → signaler l'incertitude · `non-reproductible` → ne pas créer de ticket sans plus d'information.
- **Transmission** : ne jamais passer les tickets créés directement à `orchestrator-dev` sans les présenter à l'utilisateur d'abord.
