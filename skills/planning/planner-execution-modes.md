---
name: planner-execution-modes
description: Parcours d'exécution du planner — mode standalone (invoqué directement par l'utilisateur, récaps en texte clair avant chaque appel question, validation via outil question) et mode sous-agent (invoqué via task depuis l'agent orchestrator feature, mécanisme d'interruption de session, blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire, ne jamais appeler l'outil question).
bucket: B
---

# Modes d'exécution — Planner

## Détection du mode d'invocation

SI invoqué via `task` depuis un agent parent (présence d'un contexte d'invocation structuré ou d'un task_id) → **MODE SUBAGENT**
SI invoqué directement par l'utilisateur → **MODE STANDALONE**

---

## Mode standalone

> Ce skill est chargé automatiquement quand le planner est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

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
<contenu complet du récap — observations, découvertes, décisions>

[Puis appel outil question]
question({
  questions: [{
    header: "...",
    question: "[Planner — Phase X | Feature : <nom>]\n<question de validation>",
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

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai affiché le récap complet de la phase actuelle en texte dans la discussion | ⬜ |
| ✅ Le récap contient toutes les observations, découvertes et décisions de cette phase | ⬜ |
| ✅ Le récap n'est PAS résumé — il est complet et détaillé | ⬜ |
| ✅ Le récap est affiché AVANT cet appel à `question`, PAS après | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et produire le contenu manquant MAINTENANT.**

---

## Format des questions de validation (standalone)

### Phase 0 — Prérequis vérifiés
```
question({
  questions: [{
    header: "Démarrer l'exploration",
    question: "[Planner — Phase 0 complétée | Feature : <nom>]\nPrérequis vérifiés. Démarrer l'exploration contextuelle (Phase 1) ?",
    options: [
      { label: "Démarrer (Recommandé)", description: "Passer à la Phase 1 — Exploration contextuelle" },
      { label: "Préciser le contexte", description: "Ajouter des informations avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'analyse" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle (signal design détecté)
```
question({
  questions: [{
    header: "Délégation design",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté>\n- Tests existants : <état>\n- Signal <UX/UI> détecté : <raison concrète>\n- Zones d'ombre : <liste courte>\n\nComment procéder ?",
    options: [
      { label: "Phase 1.5 — Délégation design (Recommandé)", description: "Invoquer <designer/designer> avant de planifier" },
      { label: "Skip design — Phase 2", description: "Passer aux questions complémentaires sans spec design" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de décider" }
    ]
  }]
})
```

### Phase 1 — Exploration contextuelle (aucun signal design)
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1 complétée | Feature : <nom>]\n\n**Résumé de l'exploration (X fichiers lus) :**\n- Architecture : <pattern détecté>\n- Tests existants : <état>\n- Aucun signal design détecté\n- Zones d'ombre : <liste courte ou 'Aucune'>\n\nPasser aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Explorer davantage", description: "Lire d'autres fichiers avant de poser des questions" }
    ]
  }]
})
```

### Phase 1.5 — Délégation design terminée
```
question({
  questions: [{
    header: "Questions complémentaires",
    question: "[Planner — Phase 1.5 complétée | Feature : <nom>]\nSpecs design intégrées. Passer aux questions complémentaires (Phase 2) ?",
    options: [
      { label: "Passer à Phase 2 (Recommandé)", description: "Poser les questions de clarification identifiées" },
      { label: "Revenir à Phase 1", description: "Explorer à nouveau avec les specs design reçues" }
    ]
  }]
})
```

### Phase 2 — Questions complémentaires traitées
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

### Phase 3 — Plan hiérarchique
```
question({
  questions: [{
    header: "Validation du plan",
    question: "[Planner — Phase 3 complétée | Feature : <nom>]\nEst-ce que ce découpage vous convient ? Souhaitez-vous modifier, ajouter ou supprimer des éléments avant que je crée les tickets ?",
    options: [
      { label: "Valider le plan (Recommandé)", description: "Passer à la détection des cas particuliers (Phase 4)" },
      { label: "Modifier le plan", description: "Apporter des modifications au découpage" },
      { label: "Revenir à Phase 2", description: "Reposer des questions avant de finaliser le plan" }
    ]
  }]
})
```

### Phase 4 — Cas particuliers
```
question({
  questions: [{
    header: "Création des tickets",
    question: "[Planner — Phase 4 complétée | Feature : <nom>]\nDétection des cas particuliers terminée. Passer à la création des tickets dans Beads (Phase 5) ?",
    options: [
      { label: "Créer les tickets (Recommandé)", description: "Passer à la Phase 5 — Création dans Beads" },
      { label: "Vérifier d'autres cas", description: "Rester en Phase 4 pour vérifier d'autres cas particuliers" },
      { label: "Revenir à Phase 3", description: "Revoir le plan après détection de cas particuliers critiques" }
    ]
  }]
})
```

### Phase 5.5 — Délégation ai-delegated
```
question({
  questions: [{
    header: "Délégation ai-delegated",
    question: "[Planner — Phase 5.5 | Feature : <nom>]\nSouhaitez-vous déléguer certains tickets à l'agent IA (label ai-delegated) ?",
    options: [
      { label: "Non", description: "Aucun ticket délégué à l'IA" },
      { label: "Oui — certains tickets", description: "Indiquer les IDs dans la réponse libre" },
      { label: "Oui — tous les tickets éligibles", description: "Déléguer tous les tickets sans dépendance bloquante" }
    ]
  }]
})
```

### Phase 6 — Vérification finale
```
question({
  questions: [{
    header: "Validation finale",
    question: "[Planner — Phase 6 complétée | Feature : <nom>]\nLes tickets correspondent-ils à vos attentes ? Souhaitez-vous des ajustements ?",
    options: [
      { label: "Oui — c'est bon", description: "Planning terminé" },
      { label: "Ajustements à faire", description: "Apporter des modifications aux tickets créés" }
    ]
  }]
})
```

### Retour en arrière
```
question({
  questions: [{
    header: "Retour à Phase X",
    question: "[Planner — Retour en arrière | Feature : <nom>]\n<raison du retour>. Revenir à la Phase X pour <action> ?",
    options: [
      { label: "Oui, revenir à Phase X", description: "<ce qui sera fait en Phase X>" },
      { label: "Non, continuer", description: "Poursuivre avec l'information disponible" }
    ]
  }]
})
```

---

## Format final (Phase 6)

Produire uniquement le récapitulatif de planification complet (voir section Phase 6 du skill `planner-workflow`), **sans** le bloc `## Retour vers orchestrator`.

**Selon la réponse à la validation finale :**
- **C'est bon** → Fin de session
- **Ajustements** → rester en Phase 6, appliquer les ajustements via `bd update`, re-produire le récap

---

## Mode subagent

> Ce skill est chargé quand le planner est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:planning/planner-subagent]` dans le prompt.

## Principe fondamental

Quand le planner est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés, que l'agent orchestrator retranscrira.

**Confirmer le contexte au démarrage :**
> `[planner] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session à chaque checkpoint pour remonter le récap et la question à l'agent orchestrator.`

---

## Mécanisme d'interruption — RÈGLE ABSOLUE

**À CHAQUE fin de phase ET à chaque pause ad hoc :**

1. Produire le récap de la phase en texte
2. Produire le bloc `## Retour intermédiaire vers orchestrator`
3. Produire le bloc `## Question pour l'orchestrator`
4. **TERMINER LA SESSION** — ne pas appeler l'outil `question`, ne pas continuer

L'orchestrateur :
- Affiche le `## Retour intermédiaire` en texte dans la discussion
- Lit la `## Question pour l'orchestrator`
- Pose la question à l'utilisateur via l'outil `question`
- Re-invoque le planner avec `task_id` + la réponse → le planner recharge l'historique et continue

---

## Autocontrôle avant chaque fin de session

> « Ai-je produit (1) le récap de la phase, (2) le bloc `## Retour intermédiaire vers orchestrator`, ET (3) le bloc `## Question pour l'orchestrator` ? »
> - **Non** → produire les blocs manquants MAINTENANT
> - **Oui** → terminer la session

> ⚠️ **RAPPEL CRITIQUE** : Le récap Phase 6 (contexte = orchestrator_feature) doit contenir le **contexte et le raisonnement** derrière les décisions de planification — pourquoi ces tickets, pourquoi cet ordre, quelles hypothèses, quels risques. Il n'a **pas** à reproduire le tableau des tickets ni les listes formelles — ceux-ci sont dans le bloc structuré `## Retour vers orchestrator`. L'orchestrateur retransmettra ce récap narratif intégralement à l'utilisateur pour le CP-0.

---

## ✅ Checklist visuelle — AVANT CHAQUE FIN DE SESSION

**STOP — Vérifier MAINTENANT :**

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai produit le récap complet de la phase en texte | ⬜ |
| ✅ J'ai produit le bloc `## Retour intermédiaire vers orchestrator` avec la synthèse condensée (résumé + points clés) | ⬜ |
| ✅ J'ai produit le bloc `## Question pour l'orchestrator` avec question + options + instruction de reprise | ⬜ |
| ✅ Le `task_id` est renseigné dans les deux blocs | ⬜ |
| ✅ Je vais TERMINER la session — pas appeler l'outil `question` | ⬜ |

**Si une seule case est ⬜ (non cochée) → ARRÊTER et produire le contenu manquant MAINTENANT.**

---

## Format des blocs structurés

### Bloc standard (fin de phase)

```markdown
## [Phase X] <titre du récap>

<contenu complet du récap — observations, découvertes, décisions — JAMAIS résumé>

---

## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** X — <titre>
**task_id :** <sessionID courant>

**Résumé :** <2-3 phrases décrivant ce qui a été fait dans cette phase>
**Points clés :** <liste courte — découvertes importantes, décisions prises, hypothèses formulées>
**Zones d'ombre / Blocages :** <si applicable, sinon omettre>

---

## Question pour l'orchestrator

**Phase :** X
**task_id :** <sessionID courant>

**Contexte :** <pourquoi cette question — ce qui a été découvert, ce qui bloque ou nécessite validation>

**Question :** <texte exact de la question à poser à l'utilisateur>

**Options :**
- `<label-option-a>` — <description de l'option>
- `<label-option-b>` — <description de l'option>
- `<label-option-c>` — <description si applicable>

**Instruction de reprise :** "Réponse au checkpoint Phase X : [option choisie]. Reprendre depuis <contexte précis>."
```
→ **TERMINER LA SESSION**

---

### Bloc pause ad hoc (information manquante critique)

> ⚠️ Réserver aux vrais blockers — pas aux détails. Si une hypothèse documentée permet de continuer, continuer.

```markdown
## ⏸️ Pause — Phase X — <sujet de la pause>

Pendant l'exploration de [fichier/module/contexte], j'ai détecté que [description précise du problème].

**Impact :** Sans cette information, [conséquence concrète sur la planification].

**Hypothèse possible :** [formulation de l'hypothèse si l'utilisateur souhaite continuer sans info]

---

## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** X — Pause (information manquante critique)
**task_id :** <sessionID courant>

**Résumé :** <description en 1-2 phrases du problème détecté>
**Impact :** <conséquence concrète sur la planification si non résolu>

---

## Question pour l'orchestrator

**Phase :** X — Pause
**task_id :** <sessionID courant>

**Contexte :** <description du problème détecté et de son impact>

**Question :** <question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse à la pause Phase X : [option]. [Information fournie si applicable]. Reprendre depuis le point d'interruption."
```
→ **TERMINER LA SESSION**

---

## Questions par phase

### Phase 0 — Prérequis vérifiés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 0 — Prérequis vérifiés
**task_id :** <sessionID courant>

**Résumé :** Prérequis vérifiés — feature identifiée, board Beads localisé.
**Points clés :** <prérequis manquants et hypothèses formulées, ou "Aucun prérequis manquant">

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Les prérequis pour la planification ont été vérifiés.

**Question :** Démarrer l'exploration contextuelle (Phase 1) ?

**Options :**
- `demarrer` — Démarrer la Phase 1 — Exploration contextuelle
- `preciser` — Préciser le contexte avant de démarrer
- `arreter` — Annuler l'analyse

**Instruction de reprise :** "Réponse Phase 0 : [option]. Reprendre depuis Phase 1 (exploration)."
```
→ **TERMINER LA SESSION**

### Phase 1 — Signal design détecté

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1 — Exploration contextuelle (signal design détecté)
**task_id :** <sessionID courant>

**Résumé :** Exploration contextuelle terminée — signal <UX/UI> détecté : <raison concrète>.
**Points clés :** architecture <pattern détecté>, tests <état>, signal design <description>, zones d'ombre <liste courte ou "aucune">

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** L'exploration a détecté un signal <UX/UI> : <raison concrète>. Une délégation design avant la planification est recommandée.

**Question :** Comment procéder après l'exploration Phase 1 ?

**Options :**
- `phase-1-5-design` — Phase 1.5 : déléguer au designer avant de planifier (recommandé)
- `skip-design-phase-2` — Passer directement aux questions (Phase 2) sans spec design
- `explorer-davantage` — Explorer d'autres fichiers avant de décider

**Instruction de reprise :** "Réponse Phase 1 : [option]. Reprendre depuis <Phase 1.5 / Phase 2 / exploration complémentaire>."
```
→ **TERMINER LA SESSION**

### Phase 1 — Aucun signal design

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1 — Exploration contextuelle (terminée)
**task_id :** <sessionID courant>

**Résumé :** Exploration contextuelle terminée — aucun signal design détecté.
**Points clés :** architecture <pattern détecté>, tests <état>, zones d'ombre <liste courte ou "aucune">, tickets existants liés <IDs ou "aucun">

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** L'exploration est terminée. Aucun signal design détecté. Zones d'ombre : <liste courte ou 'aucune'>.

**Question :** Passer aux questions complémentaires (Phase 2) ?

**Options :**
- `phase-2` — Passer à Phase 2 (recommandé)
- `explorer-davantage` — Explorer d'autres fichiers avant de poser des questions

**Instruction de reprise :** "Réponse Phase 1 : [option]. Reprendre depuis Phase 2 / exploration complémentaire."
```
→ **TERMINER LA SESSION**

### Phase 1.5 — Délégation design terminée

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 1.5 — Délégation design (terminée)
**task_id :** <sessionID courant>

**Résumé :** Délégation design terminée — specs <UX/UI> <reçues et intégrées | skippées>.
**Points clés :** <specs reçues (composants/parcours concernés) ou "aucune spec — tracé via bd comments add en Phase 5">

---

## Question pour l'orchestrator

**Phase :** 1.5
**task_id :** <sessionID courant>

**Contexte :** La phase de délégation design est terminée. Specs intégrées / skippées.

**Question :** Passer aux questions complémentaires (Phase 2) ?

**Options :**
- `phase-2` — Passer à Phase 2 (recommandé)
- `retour-phase-1` — Revenir à Phase 1 pour re-explorer avec les specs design

**Instruction de reprise :** "Réponse Phase 1.5 : [option]. Reprendre depuis Phase 2 / Phase 1."
```
→ **TERMINER LA SESSION**

### Phase 2 — Questions complémentaires traitées

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 2 — Questions complémentaires (traitées)
**task_id :** <sessionID courant>

**Résumé :** Questions complémentaires traitées — <N> questions posées, réponses intégrées.
**Points clés :** <décisions clés issues des réponses, hypothèses confirmées ou levées, zones d'ombre restantes>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Les questions complémentaires ont été traitées. Zones d'ombre levées : <liste>. Persistantes : <liste ou aucune>.

**Question :** Passer à l'analyse approfondie (Phase 3 — Plan hiérarchique) ?

**Options :**
- `phase-3` — Passer à Phase 3 (recommandé)
- `autres-questions` — Poser d'autres questions de clarification
- `retour-phase-1` — Revenir à Phase 1 avec les nouvelles informations

**Instruction de reprise :** "Réponse Phase 2 : [option]. Reprendre depuis Phase 3 / Phase 2 / Phase 1."
```
→ **TERMINER LA SESSION**

### Phase 3 — Plan hiérarchique

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 3 — Plan hiérarchique
**task_id :** <sessionID courant>

**Résumé :** Plan hiérarchique produit — <N> epics, <M> tickets créés, ordre de traitement défini.
**Points clés :** <dépendances critiques, risques identifiés, hypothèses structurantes>

---

## Question pour l'orchestrator

**Phase :** 3
**task_id :** <sessionID courant>

**Contexte :** Le plan hiérarchique est prêt. N epics, M tickets. Estimation totale : ~Xh.

**Question :** Ce découpage convient-il ? Souhaitez-vous modifier des éléments avant la création des tickets ?

**Options :**
- `valider-plan` — Valider et passer à Phase 4 (détection cas particuliers) (recommandé)
- `modifier-plan` — Modifier le découpage avant de continuer
- `retour-phase-2` — Revenir aux questions complémentaires

**Instruction de reprise :** "Réponse Phase 3 : [option]. [Modifications souhaitées si applicable]. Reprendre depuis Phase 4 / Phase 3 / Phase 2."
```
→ **TERMINER LA SESSION**

### Phase 4 — Cas particuliers

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 4 — Détection des cas particuliers (terminée)
**task_id :** <sessionID courant>

**Résumé :** Détection des cas particuliers terminée — <N> cas détectés, <M> écartés.
**Points clés :** <cas retenus et leur impact sur le plan, ou "Aucun cas particulier détecté">

---

## Question pour l'orchestrator

**Phase :** 4
**task_id :** <sessionID courant>

**Contexte :** Détection des cas particuliers terminée. Cas détectés : <liste ou aucun>. Ajustements au plan : <liste ou aucun>.

**Question :** Passer à la création des tickets dans Beads (Phase 5) ?

**Options :**
- `phase-5` — Créer les tickets (recommandé)
- `verifier-autres-cas` — Vérifier d'autres cas particuliers
- `retour-phase-3` — Revoir le plan

**Instruction de reprise :** "Réponse Phase 4 : [option]. Reprendre depuis Phase 5 / Phase 4 / Phase 3."
```
→ **TERMINER LA SESSION**

### Phase 5.5 — Délégation ai-delegated

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** planner
**Phase :** 5.5 — Délégation ai-delegated
**task_id :** <sessionID courant>

**Résumé :** <N> tickets créés — <Y> tickets éligibles à délégation ai-delegated.
**Points clés :** <IDs des tickets éligibles et pourquoi, ou "Aucun ticket éligible">

---

## Question pour l'orchestrator

**Phase :** 5.5
**task_id :** <sessionID courant>

**Contexte :** N tickets créés. Y tickets sont éligibles à la délégation ai-delegated.

**Question :** Souhaitez-vous déléguer certains tickets à l'agent IA ?

**Options :**
- `non` — Aucun ticket délégué
- `certains` — Indiquer les IDs dans la réponse
- `tous-eligibles` — Déléguer tous les tickets éligibles

**Instruction de reprise :** "Réponse Phase 5.5 : [option]. [IDs si applicable]. Reprendre depuis Phase 6."
```
→ **TERMINER LA SESSION**

### Phase 6 — Retour final

Phase 6 est le **retour final** — pas de question intermédiaire. Produire dans cet ordre et terminer :

1. Le récapitulatif de planification complet (liste narrative détaillée de tous les tickets avec descriptions + acceptance + notes + dépendances + risques + hypothèses)
2. Le bloc `## Retour vers orchestrator` (voir skill `planner-handoff-format`)

```markdown
---

## Retour vers orchestrator

**Agent :** planner
**Feature :** <nom>

### Tickets créés
<tableau structuré tel que défini dans planner-handoff-format>

### Dépendances
<dépendances structurées>

### Ordre de traitement
<séquence d'exécution — l'agent orchestrator la suit sans interprétation>

### Hypothèses et ambiguïtés
<hypothèses structurées>

### Risques identifiés
<risques structurés>

### Statut
`planification-complète` | `planification-partielle` | `bloqué`
```

> **Autocontrôle avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer — le récapitulatif de planification est DANS le bloc (section `### Récapitulatif de planification`). »
> « La section `### Tickets créés` contient-elle TOUS les tickets (descriptions + acceptance + notes) ? Si non → la compléter. »

→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question posée en session enfant — invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer vers la phase suivante sans produire les blocs | L'orchestrateur ne reçoit rien avant la fin complète | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` dans les blocs | L'orchestrateur ne peut pas re-invoquer pour reprendre | **Toujours inclure** le sessionID |
| Résumer le récap dans le bloc intermédiaire | L'utilisateur perd des informations critiques | **Ne jamais résumer** — copier intégralement |
| Pause ad hoc pour des détails mineurs | Trop de re-invocations, flux dégradé | **Réserver aux vrais blockers** |
