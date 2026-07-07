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

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le rapport de diagnostic complet (preuves, analyse, raisonnement) est **intégré dans le bloc** (section `### Rapport de diagnostic complet`), pas produit séparément en texte libre.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que le rapport de diagnostic est bien dans la section `### Rapport de diagnostic complet` du bloc. »

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

### Rapport de diagnostic complet

## Diagnostic — <titre du bug>

### Symptôme
<comportement attendu vs. réel, conditions de déclenchement, fréquence, environnement>

### Périmètre analysé
<artefacts fournis et exploités : stacktraces, logs, code, config — mentionner ce qui manquait>

### Localisation probable
<fichier:ligne — point d'origine identifié>

### Analyse détaillée

#### Hypothèse principale — <niveau de probabilité>
<description de la cause>

**Éléments qui l'étayent :**
- <preuve 1 — stacktrace, log, code source>
- <preuve 2>

**Pour confirmer :**
- <action de vérification>

#### Hypothèse secondaire — <niveau de probabilité>
<description>

**Éléments qui l'étayent :**
- <preuve>

**Pour confirmer :**
- <action>

<Répéter pour chaque hypothèse explorée>

### Fichiers impliqués
| Fichier | Rôle dans le bug |
|---------|-----------------|
| `<fichier:ligne>` | <rôle — point d'origine, propagation, etc.> |

### ⚠️ Informations manquantes
<ce qui manquait pour un diagnostic complet — ou "Aucune — tous les artefacts nécessaires étaient disponibles">

### Ticket de correction suggéré
**Titre :** <titre>
**Type :** bug
**Priorité :** P<X>
**Description :** <description du fix attendu>
**Acceptance criteria :**
- <critère 1>
- <critère 2>
**Notes techniques :** <indication de fix si évidente>

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
