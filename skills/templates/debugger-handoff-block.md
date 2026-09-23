# Template — Handoff debugger → orchestrator

## Bloc `## Retour vers orchestrator`

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

<!-- Obligatoire — voir shared/handoff-bloc-unique-rule -->
### Questions bloquantes

Aucune.

### Statut
`diagnostiqué` | `partiellement-diagnostiqué` | `non-reproductible`
```
