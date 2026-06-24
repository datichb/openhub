---
name: elicitation-techniques
description: "Utiliser quand une ambiguité est détectée dans les requirements, quand le périmètre est flou, quand plusieurs parties prenantes ont des visions divergentes, ou quand une décision de conception est difficile à trancher. Référence de 25 techniques d'élicitation structurées avec critères de sélection contextuelle. Injecté dans ux-designer et planner quand des clarifications sont nécessaires. Couvre : convergence/divergence, exploration des contraintes, gestion des parties prenantes, prise de décision sous incertitude. Mots-clés : elicitation, requirements, ambiguity, stakeholders, design thinking, brainstorming, decision making, clarification techniques."
bucket: B
---

# Skill — Techniques d'Élicitation

## Rôle

Fournir une palette de techniques structurées pour extraire, clarifier et aligner les besoins quand les requirements sont ambigus, incomplets ou conflictuels. Choisir la technique adaptée au contexte — pas d'application mécanique.

---

## Sélection contextuelle

Avant de choisir une technique, évaluer :

| Signal | Technique(s) adaptée(s) |
|---|---|
| Idée floue, sans forme | Five Whys, Brainwriting, Inversion |
| Parties prenantes en désaccord | Six Thinking Hats, Delphi, Steelmanning |
| Scope difficile à délimiter | Boundary Sweep, MSCW, Negative Brainstorming |
| Risques non anticipés | Pre-mortem, Cascading Failure, Inversion |
| Complexité cachée | Abstraction Laddering, Morphological Analysis |
| Décision difficile à trancher | Steelmanning, Six Thinking Hats, Impact/Effort |
| Manque de détails concrets | Jobs-to-be-Done, User Journey Mapping, Storyboarding |
| Hypothèses non explicitées | Assumption Mapping, Five Whys |

---

## Catalogue des techniques

### Divergence — Explorer l'espace du problème

---

#### 1. Five Whys
**Objectif :** Remonter à la cause racine d'un problème ou d'un besoin.
**Quand :** Quand le "pourquoi" d'une feature n'est pas clair.
**Comment :**
1. Poser la question "Pourquoi [besoin exprimé] ?" → réponse A
2. "Pourquoi A ?" → réponse B
3. Répéter 3 à 5 fois jusqu'à atteindre une cause racine

**Prompt IA :**
> "Pourquoi avez-vous besoin de [feature] ? Qu'est-ce que ça résout vraiment ?"

---

#### 2. Brainwriting (Silent Brainstorming)
**Objectif :** Générer de nombreuses idées sans influence mutuelle.
**Quand :** Phase d'exploration initiale, pas de direction encore.
**Comment :**
1. Définir le problème en une phrase
2. Générer 5–10 approches sans les filtrer
3. Catégoriser, puis évaluer

**Prompt IA :**
> "Sans contrainte de faisabilité pour l'instant, quelles seraient 5 façons différentes de résoudre [problème] ?"

---

#### 3. Inversion Analysis
**Objectif :** Trouver ce qu'il ne faut PAS faire pour identifier ce qu'il faut faire.
**Quand :** Quand les objectifs positifs sont flous mais qu'on sait ce qu'on veut éviter.
**Comment :**
1. "Comment pourrait-on complètement rater l'objectif [X] ?"
2. Lister toutes les façons d'échouer
3. Inverser chaque point → contraintes et requirements implicites

**Prompt IA :**
> "Si on voulait s'assurer que cette feature échoue complètement, qu'est-ce qu'on ferait ?"

---

#### 4. Morphological Analysis
**Objectif :** Explorer toutes les combinaisons possibles d'un espace de solutions.
**Quand :** Choix technique avec plusieurs dimensions indépendantes.
**Comment :**
1. Identifier les dimensions du problème (ex : stockage, transport, UI, auth)
2. Pour chaque dimension, lister 3–4 options
3. Explorer les combinaisons non-conventionnelles

**Exemple :**
```
Dimension 1 (stockage) : DB relationnelle | NoSQL | fichiers plats | in-memory
Dimension 2 (transport) : REST | GraphQL | WebSocket | events
Dimension 3 (UI) : SPA | SSR | PWA | CLI
→ Combiner pour trouver des solutions non-évidentes
```

---

#### 5. SCAMPER
**Objectif :** Générer des variations créatives sur une solution existante.
**Quand :** Itérer sur un concept initial.
**Dimensions :** Substitute, Combine, Adapt, Modify, Put to other uses, Eliminate, Reverse

**Prompt IA :**
> "Si on éliminait [composant X], que se passerait-il ? Si on le remplaçait par [Y] ?"

---

### Convergence — Clarifier et prioriser

---

#### 6. MSCW (MoSCoW)
**Objectif :** Prioriser les exigences par nécessité.
**Quand :** Trop de features, pas assez de temps.
**Catégories :**
- **Must have** — sans ça, la solution ne fonctionne pas
- **Should have** — important mais pas bloquant pour un MVP
- **Could have** — nice-to-have si le temps le permet
- **Won't have (this time)** — explicitement hors scope pour cette itération

**Prompt IA :**
> "Parmi ces [N] features, lesquelles sont absolument indispensables pour que la solution soit utilisable ?"

---

#### 7. Impact / Effort Matrix
**Objectif :** Identifier les quick wins et les projets à fort ROI.
**Quand :** Décision de priorisation avec des ressources limitées.
**Comment :** Placer chaque option dans un quadrant Impact × Effort :
```
Haut impact / Faible effort  → Faire en premier
Haut impact / Fort effort   → Planifier
Faible impact / Faible effort → Si le temps le permet
Faible impact / Fort effort  → Éviter
```

---

#### 8. Dot Voting (Fist to Five)
**Objectif :** Atteindre un consensus rapidement sur plusieurs options.
**Quand :** Plusieurs solutions viables, décision collective.
**Comment :**
1. Lister les options
2. Chaque participant distribue N votes sur les options
3. L'option avec le plus de votes est choisie (sous réserve de blocages explicites)

---

#### 9. Boundary Sweep
**Objectif :** Identifier les cas limites du scope.
**Quand :** Définir précisément ce qui est IN et ce qui est OUT.
**Questions :**
- "Quel est le plus petit exemple de [cas X] qui est IN scope ?"
- "Quel est le plus grand exemple de [cas Y] qui est OUT scope ?"
- "Qu'est-ce qui est ambigu et a besoin d'une décision explicite ?"

---

#### 10. Assumption Mapping
**Objectif :** Rendre explicites les hypothèses cachées dans un plan ou une spec.
**Quand :** Avant de commencer un développement complexe.
**Comment :**
1. Lister toutes les choses qu'on suppose vraies sans les avoir vérifiées
2. Pour chaque hypothèse : quelle est la conséquence si elle est fausse ?
3. Prioriser les hypothèses à valider d'abord (les plus risquées)

**Prompt IA :**
> "Quelles sont les 3 choses que vous supposez vraies pour que cette solution fonctionne, sans les avoir explicitement validées ?"

---

### Parties prenantes — Aligner les visions

---

#### 11. Six Thinking Hats (De Bono)
**Objectif :** Explorer un problème depuis 6 angles pour éviter les biais de confirmation.
**Quand :** Décision difficile avec des parties prenantes ayant des visions différentes.

| Chapeau | Angle |
|---|---|
| ⬜ Blanc | Faits et données uniquement |
| 🔴 Rouge | Émotions, intuitions, ressentis |
| ⬩ Noir | Risques, problèmes, ce qui peut mal tourner |
| 🟡 Jaune | Optimisme, bénéfices, ce qui peut bien se passer |
| 🟢 Vert | Créativité, alternatives, nouvelles idées |
| 🔵 Bleu | Processus, organisation de la réflexion |

**Usage IA :** Analyser une décision successivement avec chacun des 6 angles.

---

#### 12. Steelmanning
**Objectif :** Comprendre et renforcer la position adverse avant de la challenger.
**Quand :** Désaccord technique ou de conception entre parties prenantes.
**Comment :**
1. Formuler la position adverse dans sa version la plus forte (pas la caricature)
2. Identifier les mérites réels de cette position
3. Seulement ensuite : construire son contre-argument

**Prompt IA :**
> "Quel est le meilleur argument possible en faveur de [approche X que je pense mauvaise] ?"

---

#### 13. Méthode Delphi
**Objectif :** Construire un consensus itératif entre experts ayant des avis divergents.
**Quand :** Décision technique complexe, plusieurs experts en désaccord.
**Comment :**
1. Chaque expert donne son avis indépendamment (sans influence mutuelle)
2. Les avis sont agrégés et résumés anonymement
3. Chaque expert révise sa position en ayant connaissance du consensus
4. Répéter jusqu'à convergence (2–3 rounds suffisent généralement)

---

#### 14. Jobs-to-be-Done (JTBD)
**Objectif :** Comprendre le vrai besoin derrière une feature demandée.
**Quand :** La feature demandée semble être une solution, pas un besoin.
**Format :** "Quand [situation], je veux [motivation], pour que [résultat attendu]."

**Exemple :**
> Feature demandée : "Exporter les données en CSV"
> JTBD : "Quand je fais mon reporting mensuel, je veux récupérer mes données facilement, pour que je puisse les analyser dans Excel sans avoir besoin d'un développeur."
> → Peut révéler que l'export CSV n'est pas la seule solution (dashboard intégré ?)

---

#### 15. Empathy Map
**Objectif :** Comprendre le contexte utilisateur au-delà des requirements fonctionnels.
**Dimensions :** Ce qu'il dit / Ce qu'il pense / Ce qu'il fait / Ce qu'il ressent

---

### Risques — Anticiper ce qui peut mal tourner

---

#### 16. Pre-mortem
**Objectif :** Identifier les risques d'échec avant qu'ils se produisent.
**Quand :** Avant de commencer un développement important.
**Comment :**
1. "Imaginons que dans 6 mois, ce projet a complètement échoué. Qu'est-ce qui s'est passé ?"
2. Lister toutes les raisons d'échec imaginables
3. Pour chaque raison : probabilité × impact → mitigation préventive

**Prompt IA :**
> "Si ce développement échoue complètement dans 3 mois, quelle sera la cause principale ?"

---

#### 17. Cascading Failure Simulation
**Objectif :** Identifier les effets domino d'une défaillance dans un système distribué.
**Quand :** Architecture microservices ou à fort couplage.
**Comment :**
1. "Si [composant X] tombe en panne, qu'est-ce qui se passe ?"
2. "Si la réponse à ça tombe en panne, qu'est-ce qui se passe ?"
3. Continuer jusqu'à identifier les points de défaillance critique

---

#### 18. Negative Brainstorming
**Objectif :** Identifier les risques en listant tout ce qu'on veut ÉVITER.
**Quand :** Complement à un brainstorming positif.
**Comment :**
1. "Qu'est-ce qu'on veut absolument éviter avec cette solution ?"
2. "Quels sont les anti-patterns pour ce type de problème ?"
3. Convertir chaque contrainte négative en exigence positive

---

### Profondeur — Explorer la complexité cachée

---

#### 19. Abstraction Laddering
**Objectif :** Explorer un problème à différents niveaux d'abstraction.
**Quand :** La solution proposée est peut-être trop spécifique ou trop générique.
**Comment :**
- **Monter** (pourquoi ?) : "Pourquoi avons-nous besoin de [X] ?" → version plus abstraite du problème
- **Descendre** (comment ?) : "Comment [X] serait-il implémenté ?" → version plus concrète

**Usage :** Trouver le bon niveau d'abstraction pour la solution.

---

#### 20. User Journey Mapping
**Objectif :** Visualiser l'expérience complète d'un utilisateur, étape par étape.
**Quand :** Feature qui couvre plusieurs actions séquentielles.
**Format :**
```
Étape 1 → Action → Émotion → Point de friction potentiel
Étape 2 → ...
```

---

#### 21. Storyboarding
**Objectif :** Visualiser un scénario d'usage concret, panel par panel.
**Quand :** Valider qu'une UX flow a du sens avant de la coder.
**Format :** Décrire en 5–7 "panels" textuels le parcours d'un utilisateur type.

---

#### 22. Chain-of-Thought Scaffolding
**Objectif :** Décomposer un problème complexe en étapes de raisonnement intermédiaires.
**Quand :** Estimation complexe, décision d'architecture avec beaucoup d'inconnues.
**Comment :**
1. "Pour répondre à cette question, j'ai besoin de savoir [A], [B], [C]"
2. Répondre à chaque sous-question
3. Synthétiser la réponse à la question initiale

---

#### 23. Kano Model
**Objectif :** Classifier les features selon leur impact sur la satisfaction utilisateur.
**Catégories :**
- **Must-be** (hygiene) — si absent, insatisfaction. Si présent, neutre.
- **Performance** — plus c'est bien fait, plus c'est satisfaisant.
- **Delighters** — surprise positive, non attendu.
- **Indifferent** — peu importe.

---

#### 24. How Might We (HMW)
**Objectif :** Reformuler les problèmes en opportunités de conception.
**Quand :** Transition entre exploration du problème et idéation de solutions.
**Format :** "Comment pourrions-nous [résoudre X] en tenant compte de [contrainte Y] ?"

---

#### 25. Definition of Done collaborative
**Objectif :** Aligner toutes les parties prenantes sur ce que "terminé" signifie.
**Quand :** Avant de planifier un ticket ou une feature.
**Questions :**
- "Qu'est-ce qui devra être vrai pour que vous considériez cette feature terminée ?"
- "Qui doit approuver pour que ce soit considéré comme terminé ?"
- "Quels tests ou critères de vérification sont nécessaires ?"

---

## Combinaisons recommandées par contexte

| Contexte | Séquence |
|---|---|
| Nouvelle feature floue | JTBD → Five Whys → Brainwriting → MSCW |
| Décision d'architecture difficile | Morphological Analysis → Steelmanning → Six Thinking Hats |
| Feature à risque élevé | Assumption Mapping → Pre-mortem → Cascading Failure |
| Désaccord entre parties prenantes | Steelmanning → Delphi → Dot Voting |
| Scope difficile à délimiter | Boundary Sweep → MSCW → Definition of Done |
| Optimisation UX | User Journey Mapping → Empathy Map → HMW → Kano |
