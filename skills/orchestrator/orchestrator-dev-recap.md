---
name: orchestrator-dev-recap
description: Récap global et retour vers orchestrator — format du récap d'implémentation, bloc retour structuré.
---

## Récap global — Fin de session

**Deux étapes obligatoires dans cet ordre — ne jamais les inverser, ne jamais en omettre une.**

### Étape 1 — Récap global complet (texte)

Afficher en fin de workflow (tous les tickets traités ou suite à un **stop**).
Construire ce récap en agrégeant les données structurées collectées à chaque étape 6 :

```
## Récap implémentation — <nom de la feature ou session>

### Synthèse par ticket

<Pour chaque ticket traité, inclure la synthèse structurée issue du bloc handoff developer et du compte rendu d'étape (étape 6).>
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

> ❌ Ne jamais omettre ce récap — il contient la synthèse par ticket et les points d'attention. Le tableau de synthèse des tickets et les statistiques sont dans le bloc structuré `## Retour vers orchestrator` qui suit.

### Étape 2 — Bloc de retour structuré (obligatoire si invoqué depuis l'agent orchestrator feature)

> ⚠️ Ce bloc est **requis sans exception** — y compris en cas de stop, de ticket bloqué ou de session incomplète.
> **Champ `Type de récap` obligatoire :** renseigner `**Type de récap :** final` — ce bloc est émis seul en fin de session, tous les tickets ont été traités ou stop demandé.
> Il vient **après** le récap global complet — il en est le résumé structuré, il ne le remplace pas.
> Ne jamais clore la session sans avoir produit les deux.

Ajouter immédiatement après le récap global le bloc `## Retour vers orchestrator` :

```
---

## Retour vers orchestrator

**Tickets traités :** [bd-XX ✅, bd-YY ✅, ...]
**Tickets ignorés :** [bd-ZZ ⏭️, ...]

### Détail par ticket
| ID | Agent (domaine) | Cycles review | Critères couverts | Statut |
|----|----------------|---------------|-------------------|--------|
| bd-XX | developer (frontend) | 1 | tous | ✅ Terminé |
| bd-YY | developer (backend)  | 2 | partielle | ✅ Terminé |
| bd-ZZ | developer (api)      | — | — | ⏭️ Ignoré  |

**Points d'attention :**
- <agrégation des points d'attention techniques collectés à chaque étape 6>
**Statut global :** succès | partiel | bloqué
```

Le format exact, les champs obligatoires et les définitions des statuts (`succès`, `partiel`, `bloqué`) sont définis dans le skill `orchestrator-handoff-format` — s'y référer comme source de vérité unique.

> Les `### Points d'attention` doivent reprendre l'agrégation ci-dessus — jamais une liste vide si des points ont été signalés en cours de session.

**Vérification obligatoire avant de clore la session :**
> « Suis-je invoqué depuis l'agent orchestrator feature ? Si oui, ai-je produit le bloc `## Retour vers orchestrator` avec tous les champs requis (dont `### Contexte et décisions par ticket` et `### Points d'attention globaux`) ? Si non, le produire maintenant. Aucun texte libre avant ou après le bloc. »
