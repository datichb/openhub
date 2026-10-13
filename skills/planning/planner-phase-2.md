---
name: planner-phase-2
description: "Utiliser avant la phase 2 du planner : questions [FACT] et [DECISION], batch ou frontier, priorités, récap."
---

# Phase 2 — Questions complémentaires

## Objectif
Lever les zones d'ombre identifiées en Phase 1 : résoudre les questions factuelles par exploration, poser les questions de décision à l'utilisateur.

## Ce qu'on fait

1. **Identifier TOUTES les questions** de clarification issues de Phase 1
2. **Classifier chaque question** : `[FACT]` ou `[DECISION]` (voir section ci-dessous)
3. **Résoudre les `[FACT]`** par exploration (lecture code, doc, API, websearch) — ne pas les poser à l'utilisateur
4. **Regrouper les `[DECISION]`** en un seul envoi (ou en rounds si mode frontier)
5. **Formuler les questions en s'appuyant sur les observations de Phase 1** — pas de questions génériques
6. **Prioriser les questions par impact** — les plus bloquantes en premier

Complexité Large ou Enterprise (ou Medium si l'utilisateur le choisit) : charge `planner-phase-2-5` avant la phase 2.5 (compréhension partagée, decision map Enterprise).

---

## Classification des questions : FACT vs DECISION

### `[FACT]` — Résolvable par exploration

Une question est `[FACT]` si sa réponse peut être trouvée par lecture de code, documentation officielle, API publique, ou analyse de la codebase. L'utilisateur n'a pas besoin d'intervenir.

Exemples : « La lib X supporte-t-elle le streaming ? » → doc officielle ; « Le composant Z est-il utilisé ailleurs ? » → recherche dans `src/` ; « Quelle version est installée ? » → `package.json` / `go.mod`.

**Résolution :** Explorer immédiatement (lecture fichier, websearch, grep). Documenter la réponse et la source dans le récap Phase 2.

### `[DECISION]` — Nécessite un choix humain

Une question est `[DECISION]` si elle implique un arbitrage métier, une préférence utilisateur, un choix de périmètre, ou une priorisation. Seul l'utilisateur peut répondre.

Exemples :
- "L'optimisation performance est-elle dans le périmètre ?" → choix de scope
- "Le module partagé doit-il rester rétrocompatible ?" → arbitrage technique/métier

**En cas de doute → classifier comme `[DECISION]`.** Il vaut mieux poser une question de trop que de résoudre un fait de travers silencieusement.

---

## Questions à poser (`[DECISION]` uniquement)

Les questions doivent être **contextualisées** — s'appuyer sur ce qui a été lu, pas des questions génériques.

### Questions métier (toujours)
Objectif métier et valeur pour l'utilisateur final, utilisateurs concernés (rôles), contrainte de délai ou de périmètre, **hors périmètre** de l'itération, règles métier ou cas limites connus.

### Questions techniques contextualisées (adapter selon l'exploration)
- "J'ai vu que le module [X] n'a pas de tests. Faut-il en prévoir dans ce périmètre ?"
- "Le composant [Z] est partagé par 3 pages. La modification doit-elle rester rétrocompatible ?"

### Questions librairies externes (si comportements ⚠️ ou ❌ en étape 1.2bis)

Une question par librairie au comportement non entièrement vérifié :

- "La doc de `[lib]` indique que [comportement partiel] mais ne précise pas [point d'ambiguïté]. Faut-il traiter ce cas dans ce périmètre ?"

> **Règle** : ne poser cette question que si au moins un comportement est classé ⚠️ ou ❌ dans l'étape 1.2bis. Si tout est ✅ vérifié, skip.

### Questions impacts en cascade (si "ticket séparé nécessaire" dans l'impact map de l'étape 1.2ter)

Une question par fichier partagé dont l'impact n'est pas couvert :

- "La modification de `[fichier partagé]` impacte [N] consommateurs : [liste courte — ex : AuthController, ProfileUseCase, AdminController]. Tous doivent-ils être mis à jour dans ce périmètre, ou certains sont hors scope de cette itération ?"

### Questions de design / UX (pour les features avec une interface)
Maquettes ou specs UX disponibles, composants du design system attendus, contraintes d'accessibilité (RGAA, WCAG).

---

## Mode de questionnement : batch ou frontier

| Complexité (Phase 0) | Mode |
|-----------|------|
| Small (4-6 pts), Medium (7-10 pts) | Batch : toutes les questions `[DECISION]` en un seul envoi |
| Large (11-13 pts), Enterprise (14-16 pts) | Frontier : rounds par prérequis |

### Mode frontier — Rounds par prérequis

En mode frontier, les questions `[DECISION]` ne sont PAS toutes posées d'un coup. Elles sont organisées en **rounds** basés sur un arbre de dépendances entre décisions.

1. Pour chaque question, noter si sa pertinence dépend de la réponse à une autre (« Faut-il un thème sombre ? » dépend de « Quel design system ? »).
2. Round 1 : toutes les questions sans prérequis.
3. Après les réponses, réévaluer : certaines questions sont débloquées, d'autres invalidées (« hors périmètre » rend les détails inutiles).
4. Round suivant : les questions débloquées ; au plus 3 rounds (limite d'itération).

> **Skip global** : si l'utilisateur choisit "Skip toutes" à n'importe quel round, les rounds suivants sont annulés. Les questions restantes sont marquées "non répondu" dans le récap.

> **Frontier vide avant le max** : si les réponses du Round 1 résolvent tout, passer directement au récap. Ne pas forcer 3 rounds.

---

## Format de la question
<!-- oh:when role=entry -->

Affiche d'abord le contexte en texte (`## [Phase 2] Questions complémentaires`, ou `## [Phase 2 — Round N/M estimé]` en frontier), questions groupées par catégorie (métier, techniques, design), puis appelle l'outil `question` avec **une question par clarification**, chacune préfixée `[Planner — Phase 2 | Feature : <nom>]` (la première reprend le condensé de la phase 1 : architecture, zones d'ombre, points d'attention), et une dernière question « Skip » :

```
question({
  questions: [
    {
      header: "Hors périmètre",
      question: "[Planner — Phase 2 | Feature : <nom>]\nQu'est-ce qui ne fait PAS partie de cette itération ?",
      options: [
        { label: "Rien de spécifique", description: "Tout ce qui est décrit est dans le scope" },
        { label: "Optimisations", description: "Les optimisations de performance sont hors scope" },
        { label: "Edge cases rares", description: "Les cas limites peu fréquents sont reportés" }
      ]
    },
    // … une entrée par question [DECISION] contextualisée …
    {
      header: "Skip questions",
      question: "[Planner — Phase 2 | Feature : <nom>]\nSi vous préférez ne pas répondre aux questions ci-dessus, vous pouvez passer cette étape.",
      options: [
        { label: "J'ai répondu", description: "Continuer avec mes réponses" },
        { label: "Skip toutes", description: "Passer les clarifications — l'analyse restera partielle" }
      ]
    }
  ]
})
```

> **Note :** L'option "Type your own answer" est ajoutée automatiquement par OpenCode à chaque question — ne pas la dupliquer. L'utilisateur peut toujours saisir une réponse libre si aucune option ne convient.

Les réponses reviennent dans l'ordre des questions : label prédéfini → repris dans le récap ; réponse libre → intégrée telle quelle ; « Skip toutes » → toutes les questions « non répondu », et « Questions de clarification non traitées » dans les zones d'ombre persistantes.
<!-- /oh:when -->
<!-- oh:when role=subagent -->

⚠️ **En mode sous-agent** : **NE PAS appeler l'outil `question`**. Mets ces questions, avec leurs options, dans les `### Questions bloquantes` de ton enveloppe (statut `en-attente`), précédées dans ton rapport du condensé de la phase 1. Les questions de la section « Questions à poser » servent de **modèle de contenu**.

En frontier, chaque round est une enveloppe `en-attente` (numéro du round dans le résumé) ; relancé avec les réponses, tu passes au round suivant.
<!-- /oh:when -->

## Déduction des priorités

Ne pas imposer un cadre (pas de MoSCoW explicite). Déduire depuis le contexte et justifier :

| Niveau | Critères de déduction |
|--------|----------------------|
| **P0** | Bloquant pour d'autres tickets, critique pour la prod, dépendance de tout le reste |
| **P1** | Valeur métier principale, chemin critique de la feature, dépendance de P0 |
| **P2** | Enrichissement fonctionnel, confort utilisateur, testabilité |
| **P3** | Nice-to-have explicitement identifié comme tel par l'utilisateur |

Toujours expliquer le raisonnement :
> "Je mets ce ticket en P1 car il bloque les tickets d'authentification."

## Récap de fin de Phase 2

```markdown
## [Phase 2] Questions complémentaires traitées

**Questions classifiées :** X au total — Y résolues par exploration [FACT], Z posées à l'utilisateur [DECISION]

### Questions résolues par exploration [FACT]
| Question | Réponse | Source |
|----------|---------|--------|
| <question FACT> | <réponse trouvée> | <doc officielle v3.2 / recherche dans src/ / package.json / ...> |
(section omise sans question FACT)

### Questions résolues par l'utilisateur [DECISION]
| Question | Réponse |
|----------|---------|
| <question> | <label sélectionné ou texte libre> |

**Zones d'ombre levées :** <zones devenues claires>
**Zones d'ombre persistantes :** <zones encore floues — impact sur l'analyse>
**Priorités déduites :** P0 <bloquants> · P1 <chemin critique> · P2 <enrichissement> · P3 <nice-to-have>
```

> **Validation des FACT :** Les questions résolues par exploration sont listées dans le récap pour que l'utilisateur puisse les contester. Si une réponse FACT est incorrecte, l'utilisateur la corrige au récap ou à Phase 2.5 (understanding gate).

---

## Question de validation obligatoire (sans understanding gate)

> Cette section s'applique quand le gate est omis (complexité Small) ou optionnel et non choisi (complexité Medium).
<!-- oh:when role=entry -->

```
question({
  questions: [{
    header: "Plan hiérarchique",
    question: "[Planner — Phase 2 complétée | Feature : <nom>]\nQuestions traitées. Passer à l'analyse approfondie (Phase 3 — Plan hiérarchique) ?",
    options: [
      { label: "Passer à Phase 3 (Recommandé)", description: "Démarrer la décomposition en epics et tickets" },
      { label: "Vérifier la compréhension d'abord", description: "Afficher un résumé de ma compréhension avant de continuer (Phase 2.5)" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations reçues" }
    ]
  }]
})
```

**Selon la réponse :** Phase 3 ; vérifier la compréhension → Phase 2.5 ; d'autres questions → rester en Phase 2 et refaire le récap ; revenir à Phase 1 → Phase 1 (les réponses modifient le périmètre d'exploration).
<!-- /oh:when -->
<!-- oh:when role=subagent -->

Pas de validation de phase : une fois les réponses intégrées, enchaîne sur la phase 3 (protocole d'exécution) ; ta compréhension partagée va dans le rapport, en hypothèses.

> ❌ Ne jamais appeler l'outil `question` en mode subagent.
<!-- /oh:when -->
