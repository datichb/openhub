> 🇬🇧 [Read in English](015-concision-posture.en.md)

# ADR-015 — Skills de posture de concision pour les agents internes

## Statut

Accepté

## Contexte

Les agents internes du hub (orchestrator, orchestrator-dev, planner, pathfinder, developer, reviewer) produisent des outputs verbeux qui ne sont pas des livrables formels destinés à l'utilisateur final, mais des échanges de coordination. Ces outputs contiennent systématiquement :

- **Formules d'introduction sans valeur** : "Bien sûr !", "Je vais maintenant...", "Voici ce que j'ai trouvé :"
- **Reformulations du contexte connu** : répétition de ce que l'utilisateur vient de dire ou de ce qui est déjà établi dans la session
- **Transitions redondantes entre sections titrées** : "Passons maintenant à la section suivante :" avant un titre `##`
- **Formules de clôture** : "N'hésite pas à me poser d'autres questions."

Ces patterns ne portent aucune information et allongent inutilement les réponses. Sur des sessions longues avec plusieurs agents chaînés, cela représente 30-40% du volume de tokens de réponse.

Le projet caveman (JuliusBrussee/caveman, 71k stars) valide cette approche à grande échelle : moyenne de 65% de réduction des output tokens sur 10 benchmarks (22-87% selon le type de tâche) avec 100% de précision technique maintenue. La recherche "Brevity Constraints Reverse Performance Hierarchies in Language Models" (arxiv, mars 2026) confirme que contraindre à la brièveté améliore la précision de 26 points sur certains benchmarks.

Cependant, caveman en mode `full` ou `ultra` est trop agressif pour un hub dont certains agents produisent des livrables formels (rapports d'audit, specs UX, rapports de diagnostic). Un niveau `lite` — suppression du filler uniquement — est le bon compromis pour les agents primaires.

Pour les subagents, la situation est fondamentalement différente : leur output est consommé par un agent coordinateur, pas par un humain. Cela justifie un skill dédié, plus agressif : `posture/subagent-concision-posture`.

La décision est de créer deux skills séparés plutôt qu'un skill multi-niveaux unique, pour éviter toute ambiguïté dans le contexte des agents et minimiser la taille du contexte injecté.

1. **Contrôle par agent** : les skills s'injectent sélectivement dans les agents concernés. Le plugin caveman est global.
2. **Formalisme préservé** : le niveau `lite` est défini avec précision pour ne pas toucher aux livrables formels. caveman mode `full` ne fait pas cette distinction.
3. **Séparation claire des préoccupations** : les agents primaires reçoivent `concision-posture` (lite), les subagents reçoivent `subagent-concision-posture` (compact). Zéro ambiguïté de niveau.
4. **Pas de dépendance externe** : des skills Markdown n'ont pas de prérequis npm/binaire.

## Décision

Créer deux skills :

### `skills/posture/concision-posture.md` — niveau `lite` pour les agents primaires

**Niveau `lite` — supprime uniquement :**
- Formules d'introduction sans valeur ("Bien sûr !", "Je vais...", "Voici...")
- Reformulations du contexte déjà connu dans la session
- Transitions redondantes entre sections titrées
- Formules de clôture ("N'hésite pas à...", "J'espère que...")

**N'affecte pas :**
- Les blocs de handoff (contrats fonctionnels)
- Les récapitulatifs narratifs obligatoires (planner, onboarder, designers)
- Les rapports de review, rapports QA, rapports de diagnostic
- Les justifications techniques, avertissements, hypothèses

**Agents concernés :** orchestrator, orchestrator-dev, planner, pathfinder, reviewer

---

### `skills/posture/subagent-concision-posture.md` — niveau `compact` pour les subagents

**Principe :** l'output d'un subagent est consommé par un agent coordinateur, pas par un humain. Le contenu attendu est : (1) le bloc de handoff structuré, (2) les données techniques brutes non encodables dans ce bloc.

**Supprime (en plus de tout ce que `lite` supprime) :**
- Explications de méthode ("J'ai exploré les fichiers X, Y, Z en commençant par...")
- Justifications de décision en prose libre — elles vont dans les champs dédiés du bloc de handoff (`risques`, `recommandations`)
- Warnings non-critiques hors du bloc de handoff
- Récapitulatif pré-handoff de ce qui a été fait (le bloc de handoff le contient déjà)

**Ne supprime jamais :**
- Le bloc de handoff complet (contrat fonctionnel non négociable)
- Les données techniques brutes que le coordinateur doit recevoir : stacktraces, diffs, extraits de code avec numéros de ligne

**Règle de décision :** "Ce contenu est-il dans le bloc de handoff ? → OUI : ne pas le répéter en prose. NON : est-ce une donnée technique brute que le coordinateur doit recevoir ? → NON : ne pas l'écrire."

**Agents concernés :** developer, developer-refactor, developer-migrator, auditor-architecture, auditor-security, auditor-observability, auditor-ecodesign, auditor-accessibility, auditor-performance, auditor-privacy

**Note sur les auditor-* :** précédemment exclus de tout skill de concision (leurs rapports étaient considérés comme des livrables formels). Cette décision est révisée : les rapports d'audit sont consommés par le coordinateur `auditor` qui retranscrit à l'utilisateur. Le skill `subagent-concision-posture` ne supprime pas le contenu du bloc de handoff — il élimine uniquement la prose d'encadrement.

**Configuration** : `token_optimization.output_verbosity: "lite"` (agents primaires) et `token_optimization.subagent_verbosity: "subagent"` (subagents) dans `config/hub.json`.

## Conséquences

### Positives

- **-30-40% output tokens sur les agents primaires** (niveau `lite`, échanges de coordination).
- **-40-60% output tokens sur les subagents** (niveau `compact`, échanges inter-agents).
- **Aucune perte d'information** : `lite` ne supprime que le bruit syntaxique ; `compact` supprime la prose d'encadrement tout en préservant l'intégrité du bloc de handoff et des données techniques brutes.
- **Zéro ambiguïté** : deux skills distincts avec des portées explicites et non chevauchantes. Chaque agent ne charge que ce qui s'applique à son mode de communication.
- **Contexte allégé par agent** : chaque skill est plus petit qu'un skill multi-niveaux combiné ne le serait.
- **Configurable** : les clés de verbosité dans `hub.json` documentent les niveaux actifs.
- **Sans dépendance** : des fichiers Markdown dans `skills/posture/`, zéro setup.

### Négatives / compromis

- **Deux skills à maintenir** : toute modification des principes communs (ex: ce qu'est le filler à mesure que les modèles évoluent) doit être appliquée aux deux fichiers.
- **Risque de sur-concision sur les subagents** : le niveau `compact` est plus agressif. La règle de décision ("est-ce dans le handoff ou une donnée technique brute ?") est le garde-fou contre la perte d'information.

## Alternatives rejetées

**Plugin caveman tel quel** : caveman en mode `full` ne distingue pas les échanges de coordination des livrables formels. Pas de contrôle par agent. Dépendance npm supplémentaire.

**Skill unique avec plusieurs niveaux (`lite`, `subagent`)** : réduit le nombre de fichiers mais augmente la taille du contexte pour chaque agent (ils chargent des règles qui ne les concernent pas) et introduit une ambiguïté de sélection de niveau. Deux skills ciblés sont plus propres.

**Règles de concision dans chaque agent séparément** : duplication de contenu, maintenance distribuée, risque d'incohérence entre agents.

**Ne rien faire** : les output tokens représentent 40-60% du coût total sur les sessions longues multi-agents. Le filler est un pattern observable et mesurable.

## Impact

| Fichier | Action |
|---------|--------|
| `skills/posture/concision-posture.md` | Modifié — portée mise à jour aux agents primaires uniquement, références aux subagents supprimées |
| `skills/posture/subagent-concision-posture.md` | Créé — niveau compact pour tous les agents mode:subagent |
| `config/hub.json` | Modifié — ajout `token_optimization.subagent_verbosity: "subagent"` |
| `agents/developer/developer.md` | Modifié — `posture/concision-posture` remplacé par `posture/subagent-concision-posture` |
| `agents/developer/developer-refactor.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/developer/developer-migrator.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/quality/debugger.md` | Modifié — `posture/subagent-concision-posture` retiré (fix C-3 : passage à `mode: primary`, pattern double-rôle via `debugger-subagent`) |
| `agents/auditor/auditor-architecture.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-security.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-observability.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-ecodesign.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-accessibility.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-performance.md` | Modifié — `posture/subagent-concision-posture` ajouté |
| `agents/auditor/auditor-privacy.md` | Modifié — `posture/subagent-concision-posture` ajouté |
