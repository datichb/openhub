---
name: planner-phase-2
description: Phase 2 du workflow planner — questions complémentaires contextualisées (métier, technique, librairies, impacts, design).
---

# Phase 2 — Questions complémentaires

## Objectif
Lever les zones d'ombre identifiées en Phase 1 : résoudre les questions factuelles par exploration, poser les questions de décision à l'utilisateur.

## Ce qu'on fait

1. **Identifier TOUTES les questions** de clarification issues de Phase 1
2. **Classifier chaque question** : `[FACT]` ou `[DECISION]` (voir section ci-dessous)
3. **Résoudre les `[FACT]`** par exploration (lecture code, doc, API, websearch) — ne pas les poser à l'utilisateur
4. **Regrouper les `[DECISION]`** en un seul appel `question` (ou en rounds si mode frontier, voir R3)
5. **Formuler les questions en s'appuyant sur les observations de Phase 1** — pas de questions génériques
6. **Prioriser les questions par impact** — les plus bloquantes en premier

---

## Classification des questions : FACT vs DECISION

Avant de formuler les questions, classifier chaque zone d'ombre :

### `[FACT]` — Résolvable par exploration

Une question est `[FACT]` si sa réponse peut être trouvée par lecture de code, documentation officielle, API publique, ou analyse de la codebase. L'utilisateur n'a pas besoin d'intervenir.

**Exemples de FACT :**
- "La lib X supporte-t-elle le mode streaming ?" → lire la doc officielle
- "Le module Y a-t-il des tests ?" → `grep -rn "test" src/Y/`
- "Le composant Z est-il utilisé ailleurs ?" → `grep -rn "Z" src/`
- "Quelle version de la lib est installée ?" → lire `package.json` / `go.mod`

**Résolution :** Explorer immédiatement (lecture fichier, websearch, grep). Documenter la réponse et la source dans le récap Phase 2.

### `[DECISION]` — Nécessite un choix humain

Une question est `[DECISION]` si elle implique un arbitrage métier, une préférence utilisateur, un choix de périmètre, ou une priorisation. Seul l'utilisateur peut répondre.

**Exemples de DECISION :**
- "L'optimisation performance est-elle dans le périmètre ?" → choix de scope
- "Quel design system utiliser ?" → préférence
- "Le module partagé doit-il rester rétrocompatible ?" → arbitrage technique/métier
- "Quelle priorité pour les edge cases ?" → priorisation

**En cas de doute → classifier comme `[DECISION]`.** Il vaut mieux poser une question de trop que de résoudre un fait de travers silencieusement.

---

## Questions à poser (`[DECISION]` uniquement)

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

> ⚠️ **Si CONTEXTE = orchestrator_feature** : **NE PAS appeler l'outil `question`**. Utiliser le format `## Question batch pour l'orchestrator` défini dans `planner-execution-modes` (section "Phase 2 — Questions complémentaires (questions à poser)"). Les questions ci-dessous servent de **modèle de contenu** — le format de sortie change selon le contexte d'invocation.

**Si CONTEXTE = standalone :**

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

**Questions classifiées :** X au total — Y résolues par exploration [FACT], Z posées à l'utilisateur [DECISION]

### Questions résolues par exploration [FACT]
| Question | Réponse | Source |
|----------|---------|--------|
| <question FACT 1> | <réponse trouvée> | <doc officielle v3.2 / grep src/ / package.json / ...> |
| <question FACT 2> | <réponse trouvée> | <source> |

> Si aucune question FACT : omettre cette section.

### Questions résolues par l'utilisateur [DECISION]
| Question | Réponse |
|----------|---------|
| Objectif métier | <label sélectionné ou texte libre> |
| Hors périmètre | <label sélectionné ou texte libre> |
| [Sujet technique 1] | <label sélectionné ou texte libre> |
| Maquettes UX | <label sélectionné ou texte libre> |

**Zones d'ombre levées :**
- <zone 1 qui était floue et qui est maintenant claire grâce à la réponse ou l'exploration>

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
> **Validation des FACT :** Les questions résolues par exploration sont listées dans le récap pour que l'utilisateur puisse les contester. Si une réponse FACT est incorrecte, l'utilisateur la corrige au récap ou à Phase 2.5 (understanding gate).

---

## Phase 2.5 — Compréhension partagée (Understanding Gate)

### Déclenchement

- **Complexité >= Large (11+ pts)** : gate **obligatoire** — toujours produire le résumé de compréhension et demander validation avant Phase 3
- **Complexité Medium (7-10 pts)** : gate **optionnel** — proposer comme option dans la question de validation ("Vérifier la compréhension avant de continuer")
- **Complexité Small (4-6 pts)** : gate **omis** — passer directement à la question de validation standard

### Objectif

Présenter un résumé structuré de la compréhension acquise en Phase 1 + Phase 2, pour vérifier l'alignement avec l'utilisateur **avant** de décomposer en tickets. Un malentendu détecté ici coûte une question. Un malentendu détecté en Phase 5 coûte une re-planification.

### Format du résumé de compréhension

Afficher avant la question de validation :

```markdown
## [Phase 2.5] Compréhension partagée

### Ce que je comprends de la feature
<2-4 phrases : l'objectif, la valeur métier, le périmètre technique, les utilisateurs concernés>

### Décisions prises (issues des réponses Phase 2)
- <décision 1 — ex : "L'optimisation performance est hors scope pour cette itération">
- <décision 2 — ex : "Le design system DSFR sera utilisé">

### Hypothèses retenues (non confirmées mais intégrées au plan)
- <hypothèse 1 — ex : "La lib X supporte le mode streaming (basé sur doc v3.2, non testé)">

### Hors périmètre confirmé
- <élément 1 explicitement exclu par l'utilisateur>

### Points d'attention pour la décomposition
- <risque ou contrainte qui influencera Phase 3 — ex : "Le module partagé Y devra rester rétrocompatible">
```

> Ce résumé devient le **brief de décomposition** utilisé en Phase 3. Toute correction apportée ici est intégrée avant de décomposer.

### Question de validation (avec understanding gate)

**Si CONTEXTE = standalone et complexité >= Large :**
```
question({
  questions: [{
    header: "Compréhension partagée",
    question: "[Planner — Phase 2.5 | Feature : <nom>]\nVoici ma compréhension de la feature (résumé ci-dessus). Est-ce correct et complet ?",
    options: [
      { label: "Correct — passer à Phase 3 (Recommandé)", description: "Compréhension validée, démarrer la décomposition" },
      { label: "Corriger", description: "Des points à ajuster dans la compréhension" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations" }
    ]
  }]
})
```

**Selon la réponse :**
- **Correct** → Phase 3 (avec le résumé de compréhension comme brief)
- **Corriger** → l'utilisateur précise ce qui est incorrect, le planner met à jour le résumé et le re-présente (max 3 itérations)
- **Poser d'autres questions** → retour en Phase 2
- **Revenir à Phase 1** → Phase 1

---

## Question de validation obligatoire (sans understanding gate)

> Cette section s'applique quand le gate est omis (complexité Small) ou optionnel et non choisi (complexité Medium).

**Si CONTEXTE = standalone :**
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

**Si CONTEXTE = orchestrator_feature :**

La question de validation (avec ou sans understanding gate) est incluse dans le bloc `## Question pour l'orchestrator` standard (Phase 2 — réponses traitées) défini dans `planner-execution-modes`. Ce bloc est produit **après** la ré-invocation avec les réponses de l'utilisateur — voir le format "Phase 2 — Questions complémentaires (réponses traitées)" dans `planner-execution-modes`.

> ❌ Ne jamais appeler l'outil `question` en mode subagent — toujours terminer la session avec les blocs structurés.

**Selon la réponse (dans tous les contextes) :**
- **Passer à Phase 3** → Phase 3
- **Vérifier la compréhension** → Phase 2.5 (produire le résumé de compréhension)
- **Poser d'autres questions** → rester en Phase 2, poser de nouvelles questions, re-produire le récap
- **Revenir à Phase 1** → Phase 1 (les réponses reçues modifient le périmètre d'exploration)
