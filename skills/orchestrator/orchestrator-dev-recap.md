---
name: orchestrator-dev-recap
description: "Utiliser après le dernier ticket : récap d'implémentation (texte ou rapport) et enveloppe de retour."
---

## Récap global — Fin de session

**Agent d'entrée : le récap en texte. Sous-agent : le même récap est ton rapport, suivi de ton enveloppe.**

### Étape 1 — Récap global complet (texte)

Afficher en fin de workflow (tous les tickets traités ou suite à un **stop**).
Construire ce récap en agrégeant les données structurées collectées à chaque étape 6 :

```
## Récap implémentation — <nom de la feature ou session>

### Synthèse par ticket

<Pour chaque ticket traité, la synthèse issue des retours du developer (enveloppe et rapport) et du compte rendu d'étape (étape 6).>
<"Aucun ticket traité" si la session est interrompue avant toute implémentation>

#### Ticket #bd-XX — <titre>
**Statut :** `implémenté` | `partiellement-implémenté` | `bloqué`
**Agent :** developer (<domaine>)
**Fichiers clés :** <1-3 fichiers les plus significatifs modifiés>
**Critères couverts :** tous | partielle — <critères non couverts>
**Points d'attention :** <liste issue du ### Points d'attention pour la review du developer — ou "Aucun">

#### Ticket #bd-YY — <titre>
**Statut :** ...
**Agent :** ...
**Fichiers clés :** ...
**Critères couverts :** ...
**Points d'attention :** ...

### Points d'attention globaux
<Agrégation des points d'attention techniques collectés à chaque étape 6 :
 - Points signalés par les developer (décisions techniques, compromis, dette)
 - Points récurrents signalés par le reviewer sur plusieurs tickets>
<"Aucun point d'attention" si aucun point n'a été signalé en cours de session>
```

> ❌ Ne jamais omettre ce récap — il contient la synthèse par ticket et les points d'attention.

### Étape 2 — Sous-agent : rapport et enveloppe

> ⚠️ **Requis sans exception** — y compris en cas de stop, de ticket bloqué ou de session incomplète.

Sous-agent : ce récap est ton rapport (sections et statuts `succès`, `partiel`, `bloqué` de ton payload) ; dépose-le, puis rends ton enveloppe.

> Les `### Points d'attention` doivent reprendre l'agrégation ci-dessus — jamais une liste vide si des points ont été signalés en cours de session.

**Vérification obligatoire avant de clore la session :**
> « Suis-je lancé par un agent ? Si oui, ai-je déposé mon rapport (dont `### Contexte et décisions par ticket` et `### Points d'attention globaux`) et rendu mon enveloppe ? Si non, le faire maintenant. »
