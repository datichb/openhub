---
name: reviewer-subagent
description: Parcours d'exécution du reviewer en mode sous-agent (invoqué via task depuis orchestrator-dev ou orchestrator feature) — chargement wiki obligatoire, préparation du contexte pour sous-sessions, rapport de review complet obligatoire suivi du bloc Retour vers orchestrator-dev. Supporte les modes standard (ticket), adversarial (CP-feature), et combiné adversarial + edge-case (CP-feature avec option).
---

# Skill — Parcours Reviewer Sous-agent

> Ce skill est chargé quand le reviewer est invoqué via `task` depuis orchestrator-dev ou orchestrator feature. L'invocateur injecte `[SKILL:reviewer/reviewer-subagent]` dans le prompt.

## Principe fondamental

Quand le reviewer est invoqué via `task`, son **seul output** est le bloc `## Retour vers orchestrator-dev`.

**Format de sortie :** aucun texte avant, après ou en dehors du bloc. Le rapport de review complet est **intégré dans le bloc** (section `### Rapport complet`), pas produit séparément en texte libre.

---

## Détection du mode d'invocation

Le mode est déterminé par les tags présents dans le prompt :

| Tag présent | Mode | Contexte |
|-------------|------|----------|
| `[MODE:standard]` ou aucun tag MODE | Standard | Review de ticket (orchestrator-dev, Étape 4) |
| `[MODE:adversarial]` | Adversarial | CP-feature (orchestrator) |
| `[MODE:adversarial+edge-case]` | Adversarial + Edge-case combiné | CP-feature avec option edge-case |

---

## Prérequis commun à tous les modes — Contexte et périmètre

Avant toute analyse, quel que soit le mode :

1. **Charger le contexte wiki** (étapes 0 du workflow `reviewer.md`) :
   - Lire `docs/wiki/index.md`, `conventions.md`, `architecture.md`, `review-rules.md`
   - Mémoriser les god nodes
2. **Acquérir le diff et cadrer le périmètre** (étape 1 du workflow) :
   - Résoudre branche (`[BRANCH:]`) et base (`[BASE:]`)
   - Lister les fichiers modifiés → périmètre de review
   - Filtrer les fichiers exclus (path filters)
   - Croiser périmètre × god nodes
3. **Charger les standards ciblés** (étape 1.5 du workflow) :
   - Selon les types de fichiers dans le périmètre
4. **Produire le walkthrough** (étape 2 du workflow)
5. **Exploration contextuelle ciblée** (étape 2.5 du workflow) — max 10 fichiers

---

## Comportement par mode

### Mode Standard (review de ticket — par défaut)

1. Exécuter le prérequis commun ci-dessus
2. Exécuter le workflow de review complet (checklist, vérification de scope, auto-vérification)
3. Produire le rapport structuré complet au format défini dans `review-protocol` (avec walkthrough, périmètre, scores de confiance, séparation corrections/suggestions)
4. Conclure avec le bloc `## Retour vers orchestrator-dev` (voir skill `reviewer-handoff-format`)

### Mode Adversarial (CP-feature)

1. Exécuter le prérequis commun ci-dessus
2. Charger le skill `reviewer-adversarial` via l'outil `skill`
3. Exécuter la revue adversariale sur le diff complet feature (`git diff <base>..<feature-branch>`)
4. Produire le rapport au format `## Revue Adversariale — <périmètre>` avec scores de confiance
5. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur les findings adversariaux

### Mode Adversarial + Edge-case combiné (CP-feature avec option)

Pour garantir l'isolation contextuelle, orchestrer des sessions parallèles **avec injection du contexte** :

1. Exécuter le prérequis commun ci-dessus
2. **Préparer le contexte à injecter** — même protocole que le mode combiné standalone :
   a. Extraire la synthèse compacte (conventions, architecture, review-rules, god nodes, périmètre)
   b. Formater `[WIKI-CONTEXT:]`, `[DIFF-SCOPE:]`, `[STANDARDS:]`
3. **Lancer les sessions en parallèle** via l'outil `task` :
   ```
   // Session 1 — Adversarial (contexte wiki injecté)
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] Revue adversariale de la feature <branche>. git diff <base>..<branche>")

   // Session 2 — Edge-case (contexte wiki injecté)
   task(subagent_type: "reviewer", prompt: "[MODE:edge-case] [REVIEW:single] [WIKI-CONTEXT:<synthèse>] [DIFF-SCOPE:<liste fichiers>] [STANDARDS:<liste>] Analyse edge-case de la feature <branche>. git diff <base>..<branche>")
   ```
4. **Récupérer les rapports bruts** de chaque session
5. **Fusionner** en chargeant le skill `review-merge` et en lui fournissant les rapports
6. Produire le rapport unifié final
7. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur le rapport unifié post-fusion

---

## Format obligatoire

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire le rapport comme texte libre avant le bloc — il est DANS le bloc (section `### Rapport complet`)
> ✅ Bloc unique contenant le rapport complet intégré (incluant walkthrough, périmètre, scores de confiance)

---

## Format final (mode standard)

```markdown
## Retour vers orchestrator-dev

**Agent :** reviewer
**Ticket :** #<ID> — <titre>
**Branche :** <branche>

### Verdict
...

### Synthèse des problèmes
...

### Corrections requises
<Findings 🔴 + 🟠 uniquement — actionnables, copiés VERBATIM dans Beads>

### Suggestions (non-bloquant)
<Findings 🟡 + 💡 — pour information>

### Routing recommandé
...

### Rapport complet

## Review — <nom de la branche ou titre de la PR>

### Walkthrough
| Fichier | Changement | God node | Domaine |
|---------|-----------|----------|---------|

### Résumé
<évaluation globale>

### Périmètre et contexte
...

### 🔴 Critique — bloquant
<si applicable>

### 🟠 Majeur — à corriger
<si applicable>

### 🟡 Mineur — amélioration recommandée
<si applicable>

### 💡 Suggestions
<si applicable>

### ✅ Points positifs
<toujours inclure si pertinent>

### 🔍 Hors scope
<si applicable>

### Statut
...
```

## Format final (mode adversarial ou combiné)

```markdown
## Retour vers orchestrator-dev

**Agent :** reviewer
**Ticket :** #<ID> — <titre>
**Branche :** <feature-branch>

### Verdict
...

### Synthèse des problèmes
...

### Corrections requises
<Findings 🔴 + 🟠 uniquement>

### Suggestions (non-bloquant)
<Findings 🟡 + 💡>

### Routing recommandé
...

### Rapport complet

## Revue Adversariale — <feature-branch>
<ou ## Review unifiée — <feature-branch> si mode combiné>

<rapport selon le format du mode activé, incluant walkthrough et scores de confiance>

### Statut
...
```

> Un rapport sans problèmes comporte au minimum `### Walkthrough`, `### Résumé`, `### Périmètre et contexte`, `### Couverture des critères d'acceptance` (si ticket Beads avec critères disponible) et `### ✅ Points positifs` dans la section `### Rapport complet` du bloc.
