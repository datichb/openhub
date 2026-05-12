---
name: debugger-handoff-format
description: Source de vérité pour le format de retour du debugger vers l'orchestrator. Définit le bloc structuré à produire quand le debugger termine son diagnostic et est invoqué depuis l'orchestrator (Mode D). Injecté dans le debugger et dans l'orchestrator pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff debugger → orchestrator

Ce skill est la **source de vérité** pour le format de retour du `debugger` vers l'orchestrator.
Il est injecté dans le `debugger` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis l'`orchestrator` (Mode D — bug signalé par l'utilisateur),
tu **dois** conclure ta session avec le bloc `## Retour vers orchestrator` défini ci-dessous,
après avoir terminé ton diagnostic et (si confirmé) créé le ou les tickets de correction.

En standalone (invocation directe), tu produis ton rapport habituel — ce bloc structuré vient s'y ajouter en conclusion.

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** debugger
**Problème :** <description courte du bug tel que signalé — verbatim si possible>

### Cause racine
**Hypothèse retenue :** <cause racine identifiée — formulée en hypothèse si incertitude>
**Niveau de certitude :** <confirmé | probable | incertain>
**Chaîne causale :**
1. <étape 1 — événement déclencheur>
2. <étape 2 — propagation>
3. <étape 3 — symptôme observable>
<"Cause racine non déterminée" si le diagnostic n'a pas pu identifier la cause>

### Hypothèses explorées
- `<hypothèse 1>` : **écartée** — <raison>
- `<hypothèse 2>` : **confirmée** — <raison>
- `<hypothèse 3>` : **insuffisamment documentée** — <ce qui manque pour la confirmer ou l'écarter>
<"Aucune hypothèse alternative explorée" si la cause était évidente>

### Impact et régressions potentielles
- <composant ou feature impacté 1 — ex : authentification compromise si le bug est en prod>
- <régression possible 1 — ex : tout le flux de paiement est potentiellement affecté>
- <utilisateurs touchés si estimable — ex : tous les utilisateurs sur mobile>
<"Impact limité au composant isolé, aucune régression identifiée" si l'impact est contenu>

### Tickets de correction créés

| ID | Titre | Priorité | Labels |
|----|-------|----------|--------|
| bd-XX | <titre du ticket de correction> | P<X> | <labels> |

<"Aucun ticket créé — refus de l'utilisateur" si l'utilisateur a répondu Non à la création>
<"Aucun ticket créé — cause non déterminée, correction impossible à planifier" si diagnostic incomplet>

### Actions d'urgence si bug en prod
<steps immédiats à réaliser si le bug est actif en production>
<ex : "Désactiver le feature flag X", "Rollback vers la version Y", "Bloquer les requêtes vers /endpoint">
<"N/A — bug non critique en production" si le bug n'est pas en prod ou n'est pas urgent>

### Statut
`diagnostiqué` | `partiellement-diagnostiqué` | `non-reproductible`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `diagnostiqué` | Cause racine identifiée avec certitude suffisante, ticket créé |
| `partiellement-diagnostiqué` | Hypothèse probable mais sans certitude — ticket créé avec les informations disponibles |
| `non-reproductible` | Bug non reproductible depuis la codebase — artefacts insuffisants ou bug intermittent |

---

## Règles pour le producteur (debugger)

- **Ne jamais affirmer une cause racine sans éléments probants** — utiliser "confirmé / probable / incertain" dans `Niveau de certitude`
- **Renseigner toutes les sections** — même si vides, utiliser la mention explicite correspondante
- **Signaler honnêtement les `### Hypothèses explorées`** — y compris celles insuffisamment documentées
- **Signaler l'impact honnêtement** — ne pas minimiser si des régressions sont possibles
- Ce bloc est produit **après** la création du ticket (ou après refus explicite de l'utilisateur)

---

## Règles pour le consommateur (orchestrator)

### À la réception du bloc `## Retour vers orchestrator` du debugger

1. **Afficher l'intégralité du bloc** dans la discussion — ne jamais résumer ni filtrer.
2. **Vérifier la présence de tous les champs obligatoires** : `Cause racine`, `Impact et régressions potentielles`, `Tickets de correction créés`, `Statut`.
   - Si l'un de ces champs est absent → demander explicitement au debugger de compléter avant de continuer.
3. **Présenter les `### Actions d'urgence si bug en prod`** en premier si renseignées — elles sont prioritaires sur toute autre décision.
4. **Utiliser les tickets créés** comme point d'entrée pour la suite :
   - Si des tickets ont été créés → proposer à l'utilisateur de les intégrer dans le workflow feature (Mode A ou B)
   - Si aucun ticket créé (cause non déterminée) → informer l'utilisateur et proposer les options disponibles
5. **Utiliser le `### Statut`** pour informer l'utilisateur du niveau de confiance du diagnostic :
   - `diagnostiqué` → présenter la cause racine comme établie
   - `partiellement-diagnostiqué` → signaler l'incertitude explicitement
   - `non-reproductible` → ne pas créer de ticket de correction sans plus d'information

> ❌ Ne jamais minimiser ou ignorer la section `### Impact et régressions potentielles`.
> ❌ Ne jamais passer les tickets créés directement à orchestrator-dev sans les présenter à l'utilisateur.
