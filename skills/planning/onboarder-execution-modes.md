---
name: onboarder-execution-modes
description: Parcours d'exécution de l'onboarder — mode standalone (invoqué directement par l'utilisateur, récaps en texte clair avant chaque appel question, validation via outil question, rapport final sans bloc handoff orchestrateur) et mode sous-agent (invoqué via task depuis l'agent orchestrator feature, mécanisme d'interruption de session à chaque fin de phase, blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire, ne jamais appeler l'outil question).
bucket: B
---

# Modes d'exécution — Onboarder

## Détection du mode d'invocation

SI invoqué via `task` depuis un agent parent (présence d'un contexte d'invocation structuré ou d'un task_id) → **MODE SUBAGENT**
SI invoqué directement par l'utilisateur → **MODE STANDALONE**

---

## Mode standalone

> Ce skill est chargé automatiquement quand l'onboarder est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le texte de chaque phase est **directement visible** par l'utilisateur dans la discussion. La communication se fait via :
1. Le texte de réponse (récap complet de la phase)
2. L'outil `question` pour les validations et décisions

---

## Règle absolue — récap avant question

**À CHAQUE fin de phase :**

1. **TOUJOURS produire le récap en texte clair AVANT d'appeler l'outil `question`**
   - Le récap doit être affiché comme texte de réponse dans la discussion
   - Jamais intégré dans le champ `question` de l'outil
   - Jamais omis

2. **PUIS appeler l'outil `question` pour la validation**

**Séquence obligatoire :**
```
[Texte de réponse]
## [Phase X] <titre du récap>
<contenu complet du récap>

[Puis appel outil question]
question({
  questions: [{
    header: "...",
    question: "[Onboarder — Phase X | Projet : <nom>]\n<question de validation>",
    options: [...]
  }]
})
```

> ❌ **JAMAIS** : appeler `question` sans avoir d'abord affiché le récap
> ✅ **TOUJOURS** : afficher le récap en texte → puis appeler `question`

---

## Autocontrôle avant chaque appel `question`

> « Ai-je produit le récap en texte clair dans la discussion avant cet appel ? »
> - **Non** → produire le récap maintenant, puis appeler `question`
> - **Oui** → appeler `question`

---

## ✅ Checklist visuelle — AVANT CHAQUE CHECKPOINT

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai affiché le récap complet de la phase actuelle en texte dans la discussion | ⬜ |
| ✅ Le récap contient toutes les observations, découvertes et décisions de cette phase | ⬜ |
| ✅ Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| ✅ Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |

---

## Format des questions de validation (standalone)

### Phase 0 — Prérequis vérifiés
```
question({
  questions: [{
    header: "Démarrer l'exploration",
    question: "[Onboarder — Phase 0 complétée | Projet : <nom>]\nPrérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?",
    options: [
      { label: "Démarrer (Recommandé)", description: "Passer à la Phase 1 — Exploration contextuelle" },
      { label: "Préciser le contexte", description: "Ajouter des informations avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'onboarding" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Onboarder — Phase 1 complétée | Projet : <nom>]\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```

### Phase 2 — Questions complémentaires traitées
```
question({
  questions: [{
    header: "Rapport de contexte",
    question: "[Onboarder — Phase 2 complétée | Projet : <nom>]\nQuestions traitées. Passer à l'analyse approfondie (Phase 3 — Rapport de contexte) ?",
    options: [
      { label: "Passer à Phase 3 (Recommandé)", description: "Produire le rapport de contexte structuré" },
      { label: "Poser d'autres questions", description: "Rester en Phase 2 pour préciser d'autres points" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les nouvelles informations reçues" }
    ]
  }]
})
```

### Phase 3 — Rapport de contexte
```
question({
  questions: [{
    header: "Vérification des incohérences",
    question: "[Onboarder — Phase 3 complétée | Projet : <nom>]\nRapport de contexte produit. Passer à la vérification des incohérences (Phase 4) ?",
    options: [
      { label: "Passer à Phase 4 (Recommandé)", description: "Vérifier les incohérences et compléter le rapport" },
      { label: "Ajuster le rapport", description: "Modifier des éléments avant de continuer" }
    ]
  }]
})
```

### Phase 4 — Vérification des incohérences
```
question({
  questions: [{
    header: "Production du wiki",
    question: "[Onboarder — Phase 4 complétée | Projet : <nom>]\nVérifications terminées. Passer à la production du wiki (Phase 5) ?",
    options: [
      { label: "Passer à Phase 5 (Recommandé)", description: "Générer le wiki docs/wiki/ + ONBOARDING.md" },
      { label: "Revoir le rapport", description: "Ajuster le rapport avant de produire le wiki" }
    ]
  }]
})
```

---

## Format final (standalone)

Produire uniquement le rapport d'onboarding complet (voir skill `onboarder-handoff-format`), **sans** le bloc `## Retour vers orchestrator`.

---

## Mode subagent

> Ce skill est chargé quand l'onboarder est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:planning/onboarder-subagent]` dans le prompt.

## Principe fondamental

Quand l'onboarder est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés.

**Confirmer le contexte au démarrage :**
> `[onboarder] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session à chaque fin de phase pour remonter le récap et la question à l'agent orchestrator.`

---

## Mécanisme d'interruption — RÈGLE ABSOLUE

**À CHAQUE fin de phase :**

1. Produire le récap de la phase en texte
2. Produire le bloc `## Retour intermédiaire vers orchestrator`
3. Produire le bloc `## Question pour l'orchestrator`
4. **TERMINER LA SESSION**

---

## Autocontrôle avant chaque fin de session

> « Ai-je produit (1) le récap de la phase, (2) le bloc `## Retour intermédiaire vers orchestrator`, ET (3) le bloc `## Question pour l'orchestrator` ? »
> - **Non** → produire les blocs manquants MAINTENANT
> - **Oui** → terminer la session

---

## ✅ Checklist visuelle — AVANT CHAQUE FIN DE SESSION

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai produit le récap complet de la phase en texte | ⬜ |
| ✅ J'ai produit le bloc `## Retour intermédiaire vers orchestrator` avec le récap intégral | ⬜ |
| ✅ J'ai produit le bloc `## Question pour l'orchestrator` avec question + options + instruction de reprise | ⬜ |
| ✅ Le `task_id` est renseigné dans les deux blocs | ⬜ |
| ✅ Je vais TERMINER la session — pas appeler l'outil `question` | ⬜ |

---

## Format des blocs structurés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** X — <titre>
**task_id :** <sessionID courant>

<Reproduire ici le récap complet de la phase — jamais résumé>

---

## Question pour l'orchestrator

**Phase :** X
**task_id :** <sessionID courant>

**Contexte :** <pourquoi cette question — ce qui a été découvert>

**Question :** <texte exact de la question>

**Options :**
- `<label-option-a>` — <description>
- `<label-option-b>` — <description>

**Instruction de reprise :** "Réponse Phase X onboarder : [option]. Reprendre depuis Phase X+1."
```
→ **TERMINER LA SESSION**

---

## Questions par phase

### Phase 0 — Prérequis vérifiés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** 0 — Prérequis vérifiés
**task_id :** <sessionID courant>

<récap Phase 0 complet — projet identifié, fichiers structurants, prérequis manquants>

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Les prérequis pour l'onboarding ont été vérifiés.

**Question :** Démarrer l'exploration contextuelle (Phase 1) ?

**Options :**
- `demarrer` — Passer à la Phase 1 — Exploration contextuelle
- `preciser` — Ajouter des informations avant de démarrer
- `arreter` — Annuler l'onboarding

**Instruction de reprise :** "Réponse Phase 0 onboarder : [option]. Reprendre depuis Phase 1 (exploration)."
```
→ **TERMINER LA SESSION**

### Phase 1 — Exploration contextuelle

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** 1 — Exploration contextuelle
**task_id :** <sessionID courant>

<récap Phase 1 complet — stack, profil applicatif, architecture, tickets Beads, contexte métier, Figma, stratégie de test, points d'attention, zones d'ombre>

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** L'exploration contextuelle est terminée. Stack et architecture identifiées, zones d'ombre répertoriées.

**Question :** Passer aux questions complémentaires (Phase 2) ?

**Options :**
- `passer-phase-2` — Poser les questions de clarification identifiées
- `explorer-davantage` — Lire d'autres fichiers avant de poser des questions

**Instruction de reprise :** "Réponse Phase 1 onboarder : [option]. Reprendre depuis Phase 2 (questions complémentaires)."
```
→ **TERMINER LA SESSION**

### Phase 2 — Questions complémentaires

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** 2 — Questions complémentaires traitées
**task_id :** <sessionID courant>

<récap Phase 2 complet — questions posées, réponses reçues, zones d'ombre levées/persistantes>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Les questions de clarification ont été traitées. Zones d'ombre levées et persistantes identifiées.

**Question :** Passer à l'analyse approfondie (Phase 3 — Rapport de contexte) ?

**Options :**
- `passer-phase-3` — Produire le rapport de contexte structuré
- `poser-autres-questions` — Rester en Phase 2 pour préciser d'autres points
- `revenir-phase-1` — Explorer à nouveau avec les nouvelles informations reçues

**Instruction de reprise :** "Réponse Phase 2 onboarder : [option]. Reprendre depuis Phase 3 (rapport de contexte)."
```
→ **TERMINER LA SESSION**

### Phase 3 — Rapport de contexte

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** 3 — Rapport de contexte produit
**task_id :** <sessionID courant>

<récap Phase 3 complet — stack, architecture, patterns, points d'attention, agents recommandés>

---

## Question pour l'orchestrator

**Phase :** 3
**task_id :** <sessionID courant>

**Contexte :** Le rapport de contexte structuré a été produit.

**Question :** Passer à la vérification des incohérences (Phase 4) ?

**Options :**
- `passer-phase-4` — Vérifier les incohérences et compléter le rapport
- `ajuster-rapport` — Modifier des éléments avant de continuer

**Instruction de reprise :** "Réponse Phase 3 onboarder : [option]. Reprendre depuis Phase 4 (vérification incohérences)."
```
→ **TERMINER LA SESSION**

### Phase 4 — Vérification des incohérences

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** onboarder
**Phase :** 4 — Vérification des incohérences terminée
**task_id :** <sessionID courant>

<récap Phase 4 — incohérences détectées ou "aucune incohérence détectée">

---

## Question pour l'orchestrator

**Phase :** 4
**task_id :** <sessionID courant>

**Contexte :** Vérification des incohérences terminée. <N> incohérences détectées / aucune incohérence.

**Question :** Passer à la production du wiki (Phase 5) ?

**Options :**
- `passer-phase-5` — Générer le wiki docs/wiki/ + ONBOARDING.md (recommandé)
- `revoir-rapport` — Ajuster le rapport avant de produire le wiki

**Instruction de reprise :** "Réponse Phase 4 onboarder : [option]. Reprendre depuis Phase 5 (production wiki)."
```
→ **TERMINER LA SESSION**

---

## Format final (Phase 5)

Phase 5 est le **retour final**. Produire dans cet ordre :

Produire **uniquement** le bloc `## Retour vers orchestrator` (voir skill `onboarder-handoff-format`). Le rapport d'onboarding est intégré dans le bloc (section `### Rapport d'onboarding`), pas produit séparément.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer — le rapport est DANS le bloc. »

→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer sans produire les blocs | L'orchestrateur ne reçoit rien | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` | L'orchestrateur ne peut pas reprendre | **Toujours inclure** le sessionID |
| Résumer le récap | L'utilisateur perd des informations | **Ne jamais résumer** — afficher le récap complet |
