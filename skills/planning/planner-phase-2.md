---
name: planner-phase-2
description: Phase 2 du workflow planner — questions complémentaires contextualisées (métier, technique, librairies, impacts, design).
---

# Phase 2 — Questions complémentaires

## Objectif
Poser les questions de clarification identifiées en Phase 1 pour lever les zones d'ombre.

## Ce qu'on fait

1. **Regrouper TOUTES les questions** de clarification en un seul appel `question`
2. **Formuler les questions en s'appuyant sur les observations de Phase 1** — pas de questions génériques
3. **Prioriser les questions par impact** — les plus bloquantes en premier

## Questions à poser

Les questions doivent être **contextualisées** — s'appuyer sur ce qui a été lu, pas des questions génériques.

### Questions métier (toujours)
- Quel est l'objectif métier de cette feature ? Quelle valeur apporte-t-elle à l'utilisateur final ?
- Qui sont les utilisateurs concernés ? (rôles, personas)
- Y a-t-il une contrainte de délai ou de périmètre à respecter ?
- Qu'est-ce qui est **hors périmètre** pour cette itération ?
- Y a-t-il des règles métier spécifiques ou des cas limites connus ?

### Questions techniques contextualisées (adapter selon l'exploration)
Exemples :
- "J'ai vu que le module [X] n'a pas de tests. Faut-il en prévoir dans ce périmètre ?"
- "La migration [Y] est ouverte. Cette feature en dépend-elle ?"
- "Le composant [Z] est partagé par 3 pages. La modification doit-elle rester rétrocompatible ?"
- "Le pattern [use case / aggregate / etc.] est utilisé sur des features similaires. Faut-il s'y conformer ?"

### Questions librairies externes (si comportements ⚠️ ou ❌ en étape 1.2bis)

Poser une question par librairie avec comportement non entièrement vérifié :

- "J'ai supposé que `[lib.méthode()]` [description du comportement supposé — ex : déclenche un re-render synchrone]. Des contraintes de version ou des comportements spécifiques à l'environnement à clarifier ?"
- "La doc de `[lib]` indique que [comportement partiel] mais ne précise pas [point d'ambiguïté]. Faut-il traiter ce cas dans ce périmètre ?"

> **Règle** : ne poser cette question que si au moins un comportement est classé ⚠️ ou ❌ dans l'étape 1.2bis. Si tout est ✅ vérifié, skip.

### Questions impacts en cascade (si "ticket séparé nécessaire" dans l'impact map de l'étape 1.2ter)

Poser une question par fichier partagé avec impact non couvert :

- "La modification de `[fichier partagé]` impacte [N] consommateurs : [liste courte — ex : AuthController, ProfileUseCase, AdminController]. Tous doivent-ils être mis à jour dans ce périmètre, ou certains sont hors scope de cette itération ?"
- "Le type `[NomType]` est utilisé dans [N] endroits. La modification de sa structure impacte [liste de consommateurs]. Traite-t-on tous ces impacts maintenant ou seulement [partie prioritaire] ?"

> **Règle** : ne poser cette question que si au moins un consommateur est classé "ticket séparé nécessaire" dans l'impact map 1.2ter. Si tous les impacts sont neutres ou couverts, skip.

### Questions de design / UX (pour les features avec une interface)
- Y a-t-il des maquettes ou des specs UX disponibles ?
- Quels composants du design system (DSFR ou autre) sont attendus ?
- Y a-t-il des contraintes d'accessibilité spécifiques (RGAA, WCAG) ?

## Format de la question

Afficher d'abord le contexte en texte :

```markdown
## [Phase 2] Questions complémentaires

Quelques questions issues de l'exploration pour affiner la planification :

### Questions métier
1. **Objectif métier** : Quelle est la valeur apportée à l'utilisateur final ?
2. **Périmètre** : [Question contextualisée issue de Phase 1]
3. **Hors périmètre** : Qu'est-ce qui ne fait pas partie de cette itération ?

### Questions techniques
1. **[Sujet 1]** : [Question contextualisée issue de Phase 1]
2. **[Sujet 2]** : [Question contextualisée issue de Phase 1]

### Questions design (si applicable)
1. **Maquettes** : Y a-t-il des maquettes disponibles ?
2. **Design system** : Quels composants DSFR sont attendus ?
```

Puis appeler l'outil `question` avec **une question par clarification** :

> **Si CONTEXTE = orchestrator_feature** : enrichir le champ `question` de la **première question** avec un condensé des observations Phase 1 (architecture, zones d'ombre, signaux détectés) — c'est la seule information visible dans la session parent.

```
question({
  questions: [
    // Question métier — Objectif (avec condensé Phase 1 si orchestrateur)
    {
      header: "Objectif métier",
      question: "[Planner — Phase 2 | Feature : <nom>]\n\n**Contexte de l'exploration (Phase 1) :**\n- Architecture : <pattern détecté>\n- Zones d'ombre identifiées : <liste courte ou 'Aucune'>\n- Points d'attention : <liste courte ou 'Aucun'>\n\nQuelle est la valeur apportée à l'utilisateur final ?",
      options: [
        { label: "Gain de temps", description: "Automatisation ou simplification d'un processus existant" },
        { label: "Nouvelle capacité", description: "Fonction qui n'existait pas auparavant" },
        { label: "Conformité", description: "Mise en conformité réglementaire ou technique" }
      ]
    },
    // Question métier — Périmètre
    {
      header: "Hors périmètre",
      question: "[Planner — Phase 2 | Feature : <nom>]\nQu'est-ce qui ne fait PAS partie de cette itération ?",
      options: [
        { label: "Rien de spécifique", description: "Tout ce qui est décrit est dans le scope" },
        { label: "Optimisations", description: "Les optimisations de performance sont hors scope" },
        { label: "Edge cases rares", description: "Les cas limites peu fréquents sont reportés" }
      ]
    },
    // Questions techniques contextualisées (adapter selon Phase 1)
    {
      header: "[Sujet technique 1]",
      question: "[Planner — Phase 2 | Feature : <nom>]\n[Question contextualisée issue de Phase 1 — ex: 'J'ai vu que le module X n'a pas de tests. Faut-il en prévoir dans ce périmètre ?']",
      options: [
        { label: "Oui", description: "À inclure dans le périmètre" },
        { label: "Non", description: "Hors périmètre pour cette itération" },
        { label: "À voir selon effort", description: "Inclure si l'effort reste raisonnable" }
      ]
    },
    // Questions design (si applicable)
    {
      header: "Maquettes UX",
      question: "[Planner — Phase 2 | Feature : <nom>]\nY a-t-il des maquettes ou specs UX disponibles ?",
      options: [
        { label: "Oui — disponibles", description: "Maquettes fournies, à suivre" },
        { label: "Non — liberté", description: "Pas de maquettes, liberté d'implémentation" },
        { label: "À produire", description: "Maquettes à créer avant implémentation" }
      ]
    },
    // Option Skip globale en dernière position
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

## Traitement des réponses

Les réponses sont retournées dans l'ordre des questions posées, sous forme de tableau de labels :
```
["Gain de temps", "Rien de spécifique", "Oui", "Non — liberté", "J'ai répondu"]
```

**Règles de traitement :**

| Réponse | Action |
|---------|--------|
| Label prédéfini | Utiliser directement dans le récap de fin de Phase 2 |
| Réponse libre (texte saisi) | Intégrer le texte complet dans le récap |
| "Skip toutes" (dernière question) | Marquer toutes les questions précédentes comme "non répondu" — l'analyse restera partielle |

**Mapping réponses → récap :**

```typescript
// Pseudo-code de traitement
const [objectif, horsPerimetre, sujetTech1, maquettes, skipStatus] = reponses;

// Si l'utilisateur a choisi "Skip toutes"
if (skipStatus === "Skip toutes") {
  // Marquer toutes les questions comme "non répondu"
  recapPhase2.questions.forEach(q => q.reponse = "non répondu");
  recapPhase2.zonesOmbrePersistantes.push("Questions de clarification non traitées");
} else {
  // Mapper chaque réponse
  recapPhase2.questions = [
    { question: "Objectif métier", reponse: objectif },
    { question: "Hors périmètre", reponse: horsPerimetre },
    { question: "[Sujet technique 1]", reponse: sujetTech1 },
    { question: "Maquettes UX", reponse: maquettes }
  ];
}
```

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
> "Ce ticket est P3 — vous l'avez mentionné comme optionnel pour cette itération."

## Récap de fin de Phase 2

```markdown
## [Phase 2] Questions complémentaires traitées

**Questions posées :** X questions (via outil question multi-questions)

**Réponses reçues :**
| Question | Réponse |
|----------|---------|
| Objectif métier | <label sélectionné ou texte libre> |
| Hors périmètre | <label sélectionné ou texte libre> |
| [Sujet technique 1] | <label sélectionné ou texte libre> |
| Maquettes UX | <label sélectionné ou texte libre> |

**Zones d'ombre levées :**
- <zone 1 qui était floue et qui est maintenant claire grâce à la réponse>
- <Exemple : "L'objectif métier est maintenant clair : gain de temps sur le processus de validation">

**Zones d'ombre persistantes :**
- <zone 1 qui reste floue — impact sur l'analyse>
- <Si "Skip toutes" : "Questions de clarification non traitées — l'analyse restera partielle sur les points suivants : [liste]">

**Priorités déduites :**
- P0 : <tickets identifiés comme bloquants>
- P1 : <tickets identifiés comme chemin critique>
- P2 : <tickets identifiés comme enrichissement>
- P3 : <tickets identifiés comme nice-to-have>
```

> **Traitement des réponses libres :** Si l'utilisateur a saisi une réponse libre (texte personnalisé), l'intégrer telle quelle dans le tableau. Ces réponses libres sont souvent plus précises que les labels prédéfinis.

## Question de validation obligatoire

**Si CONTEXTE = standalone :**
```
question({
  questions: [{
    header: "Plan hiérarchique",
    question: "[Planner — Phase 2 complétée | Feature : <nom>]\nQuestions traitées. Passer à l'analyse approfondie (Phase 3 — Plan hiérarchique) ?",
    options: [
      { label: "Passer à Phase 3 (Recommandé)", description: "Démarrer la décomposition en epics et tickets" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations reçues" }
    ]
  }]
})
```

**Selon la réponse (dans tous les contextes) :**
- **Passer à Phase 3** → Phase 3
- **Poser d'autres questions** → rester en Phase 2, poser de nouvelles questions, re-produire le récap
- **Revenir à Phase 1** → Phase 1 (les réponses reçues modifient le périmètre d'exploration)
