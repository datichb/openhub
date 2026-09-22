---
name: debugger-handoff-format
description: Source de vérité pour le format de retour du debugger vers l'orchestrator. Définit le bloc structuré unique à produire quand le debugger termine son diagnostic et est invoqué depuis l'orchestrator (Mode D). Le rapport de diagnostic complet est intégré dans le bloc. Injecté dans le debugger et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff debugger → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `debugger` vers l'orchestrator.
Il est injecté dans le `debugger` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis l'`orchestrator` (Mode D — bug signalé par l'utilisateur),
ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Format de sortie :** aucun texte avant, après ou en dehors de ce bloc. Le rapport de diagnostic complet (preuves, analyse, raisonnement) est **intégré dans le bloc** (section `### Rapport de diagnostic complet`), pas produit séparément en texte libre.

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

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **Le rapport de diagnostic complet est DANS le bloc** (section `### Rapport de diagnostic complet`) — ne pas le produire séparément en texte libre
- **Toujours inclure `### Rapport de diagnostic complet`** même si le diagnostic est `non-reproductible` — le rapport documente ce qui a été tenté
- **Ne jamais affirmer une cause racine sans éléments probants** — utiliser "confirmé / probable / incertain" dans `Niveau de certitude`
- **Renseigner toutes les sections** — même si vides, utiliser la mention explicite correspondante
- **Signaler honnêtement les hypothèses insuffisamment documentées** — l'orchestrator a besoin de cette information
- Ce bloc est produit **après** la création du ticket (ou après refus explicite de l'utilisateur)

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire le rapport comme texte libre avant le bloc — il est DANS le bloc
> ❌ Ne jamais minimiser l'impact si des régressions sont possibles

---

## Règles pour le consommateur (orchestrator)

**Spécificités debugger à vérifier :**

- **Champs obligatoires** : `Cause racine`, `Impact et régressions potentielles`, `Tickets de correction créés`, `Rapport de diagnostic complet`, `Statut`. Si l'un est absent → demander au debugger de compléter avant de continuer.
- **Priorité absolue** : présenter `### Actions d'urgence si bug en prod` en premier si renseignées — elles priment sur toute autre décision.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport de diagnostic complet` est affiché intégralement.
- **Suite** : si des tickets ont été créés → proposer à l'utilisateur de les intégrer dans le workflow (Mode A ou B). Si aucun ticket (cause non déterminée) → informer et proposer les options.
- **Statut** : `diagnostiqué` → cause établie · `partiellement-diagnostiqué` → signaler l'incertitude · `non-reproductible` → ne pas créer de ticket sans plus d'information.
- **Transmission** : ne jamais passer les tickets créés directement à `orchestrator-dev` sans les présenter à l'utilisateur d'abord.
