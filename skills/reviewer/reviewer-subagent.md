---
name: reviewer-subagent
description: Parcours d'exécution du reviewer en mode sous-agent (invoqué via task depuis orchestrator-dev ou orchestrator feature) — rapport de review complet obligatoire suivi du bloc Retour vers orchestrator-dev. Supporte les modes standard (ticket), adversarial (CP-feature), et combiné adversarial + edge-case (CP-feature avec option).
---

# Skill — Parcours Reviewer Sous-agent

> Ce skill est chargé quand le reviewer est invoqué via `task` depuis orchestrator-dev ou orchestrator feature. L'invocateur injecte `[SKILL:reviewer/reviewer-subagent]` dans le prompt.

## Principe fondamental

Quand le reviewer est invoqué via `task`, son **seul output** est le bloc `## Retour vers orchestrator-dev`.

**Règle absolue :** aucun texte avant, après ou en dehors du bloc. Le rapport de review complet est **intégré dans le bloc** (section `### Rapport complet`), pas produit séparément en texte libre.

---

## Détection du mode d'invocation

Le mode est déterminé par les tags présents dans le prompt :

| Tag présent | Mode | Contexte |
|-------------|------|----------|
| `[MODE:standard]` ou aucun tag MODE | Standard | Review de ticket (orchestrator-dev, Étape 4) |
| `[MODE:adversarial]` | Adversarial | CP-feature (orchestrator) |
| `[MODE:adversarial+edge-case]` | Adversarial + Edge-case combiné | CP-feature avec option edge-case |

---

## Comportement par mode

### Mode Standard (review de ticket — par défaut)

1. Exécuter le workflow de review complet (voir skill `review-protocol`)
2. Produire le rapport structuré complet au format défini dans `review-protocol`
3. Conclure avec le bloc `## Retour vers orchestrator-dev` (voir skill `reviewer-handoff-format`)

### Mode Adversarial (CP-feature)

1. Charger le skill `reviewer-adversarial` via l'outil `skill`
2. Exécuter la revue adversariale sur le diff complet feature (`git diff main..<feature-branch>`)
3. Produire le rapport au format `## Revue Adversariale — <périmètre>`
4. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur les findings adversariaux

### Mode Adversarial + Edge-case combiné (CP-feature avec option)

Pour garantir l'isolation contextuelle, orchestrer des sessions parallèles :

1. **Lancer les sessions en parallèle** via l'outil `task` :
   ```
   // Session 1 — Adversarial (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:adversarial] [SKILL:reviewer/reviewer-standalone-single] Revue adversariale de la feature <branche>. git diff main..<branche>")

   // Session 2 — Edge-case (contexte vierge)
   task(subagent_type: "reviewer", prompt: "[MODE:edge-case] [SKILL:reviewer/reviewer-standalone-single] Analyse edge-case de la feature <branche>. git diff main..<branche>")
   ```

2. **Récupérer les rapports bruts** de chaque session
3. **Fusionner** en chargeant le skill `review-merge` et en lui fournissant les rapports
4. Produire le rapport unifié final
5. Conclure avec le bloc `## Retour vers orchestrator-dev` — le verdict se base sur le rapport unifié post-fusion

---

## Règle absolue

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire le rapport comme texte libre avant le bloc — il est DANS le bloc (section `### Rapport complet`)
> ✅ Bloc unique contenant le rapport complet intégré

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
...

### Routing recommandé
...

### Rapport complet

## Review — <nom de la branche ou titre de la PR>

### Résumé
<évaluation globale>

### 🔴 Critique — bloquant
<si applicable>

### 🟠 Majeur — à corriger
<si applicable>

### 🟡 Mineur — amélioration recommandée
<si applicable>

### 💡 Suggestion — optionnel
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
...

### Routing recommandé
...

### Rapport complet

## Revue Adversariale — <feature-branch>
<ou ## Review unifiée — <feature-branch> si mode combiné>

<rapport selon le format du mode activé>

### Statut
...
```

> Un rapport sans problèmes comporte au minimum `### Résumé` et `### ✅ Points positifs` dans la section `### Rapport complet` du bloc.
