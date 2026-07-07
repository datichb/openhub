---
name: auditor-execution-modes
description: Parcours d'exécution de l'auditor — mode standalone (invoqué directement par l'utilisateur, récaps en texte clair avant chaque appel question, validation via outil question, synthèse finale sans bloc handoff orchestrateur) et mode sous-agent (invoqué via task depuis l'agent orchestrator feature, mécanisme d'interruption de session à chaque fin de phase 0 à 3, blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire, ne jamais appeler l'outil question).
bucket: B
---

# Modes d'exécution — Auditor

## Détection du mode d'invocation

SI invoqué via `task` depuis un agent parent (présence d'un contexte d'invocation structuré ou d'un task_id) → **MODE SUBAGENT**
SI invoqué directement par l'utilisateur → **MODE STANDALONE**

---

## Mode standalone

> Ce skill est chargé automatiquement quand l'auditor est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le texte de chaque phase est **directement visible** par l'utilisateur. La communication se fait via :
1. Le texte de réponse (récap complet de la phase)
2. L'outil `question` pour les validations et décisions

---

## Règle absolue — récap avant question

**À CHAQUE fin de phase :**

1. **TOUJOURS produire le récap en texte clair AVANT d'appeler l'outil `question`**
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
    question: "[Auditeur — Phase X | Projet : <nom>]\n<question de validation>",
    options: [...]
  }]
})
```

> ❌ **JAMAIS** : appeler `question` sans avoir d'abord affiché le récap
> ✅ **TOUJOURS** : afficher le récap en texte → puis appeler `question`

---

## Format des questions de validation (standalone)

### Phase 0 — Prérequis vérifiés
```
question({
  questions: [{
    header: "Charger le contexte",
    question: "[Auditeur — Phase 0 complétée | Projet : <nom>]\nPrérequis vérifiés. Charger le contexte projet (Phase 1) ?",
    options: [
      { label: "Charger le contexte (Recommandé)", description: "Passer à la Phase 1 — Chargement contexte projet" },
      { label: "Préciser le périmètre", description: "Ajuster le périmètre avant de continuer" },
      { label: "Arrêter", description: "Annuler l'audit" }
    ]
  }]
})
```

### Phase 1 — Contexte chargé
```
question({
  questions: [{
    header: "Sélectionner les domaines",
    question: "[Auditeur — Phase 1 complétée | Projet : <nom>]\nContexte chargé. Passer à la sélection des domaines à auditer (Phase 2) ?",
    options: [
      { label: "Sélectionner les domaines (Recommandé)", description: "Passer à la Phase 2 — Sélection des domaines" },
      { label: "Recharger le contexte", description: "Relire ONBOARDING.md ou refaire la reconnaissance rapide" },
      { label: "Arrêter", description: "Annuler l'audit" }
    ]
  }]
})
```

### Phase 2 — Domaines sélectionnés
```
question({
  questions: [{
    header: "Démarrer les audits",
    question: "[Auditeur — Phase 2 complétée | Projet : <nom>]\nDomaines sélectionnés. Démarrer les audits (Phase 3) ?",
    options: [
      { label: "Démarrer les audits (Recommandé)", description: "Passer à la Phase 3 — Délégation aux sous-agents" },
      { label: "Ajuster les domaines", description: "Ajouter ou retirer des domaines avant de démarrer" },
      { label: "Arrêter", description: "Annuler l'audit" }
    ]
  }]
})
```

### Phase 3 — Audits réalisés
```
question({
  questions: [{
    header: "Consolider les rapports",
    question: "[Auditeur — Phase 3 complétée | Projet : <nom>]\nAudits réalisés. Passer à la consolidation (Phase 4) ?",
    options: [
      { label: "Consolider (Recommandé)", description: "Passer à la Phase 4 — Consolidation et synthèse exécutive" },
      { label: "Relancer un audit", description: "Relancer un sous-agent pour affiner son rapport" },
      { label: "Arrêter", description: "Stopper avant la consolidation — rapports disponibles individuellement" }
    ]
  }]
})
```

### Phase 4 — Synthèse produite
```
question({
  questions: [{
    header: "Audit terminé",
    question: "[Auditeur — Phase 4 complétée | Projet : <nom>]\nSynthèse exécutive produite. Besoin d'ajustements ?",
    options: [
      { label: "Terminer", description: "Audit complet terminé" },
      { label: "Relancer un audit", description: "Relancer un sous-agent pour affiner son rapport" },
      { label: "Revoir la consolidation", description: "Ajuster la synthèse exécutive" }
    ]
  }]
})
```

---

## Format final (standalone)

Produire uniquement la synthèse exécutive multi-domaines, **sans** le bloc `## Retour vers orchestrator`.

---

## Mode subagent

> Ce skill est chargé quand l'auditor est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:auditor/auditor-subagent]` dans le prompt.

## Principe fondamental

Quand l'auditor est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur dans la session parent. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés.

**Confirmer le contexte au démarrage :**
> `[auditor] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session à chaque fin de phase pour remonter le récap et la question à l'agent orchestrator.`

---

## Mécanisme d'interruption — RÈGLE ABSOLUE

**À CHAQUE fin de phase (0 à 3) :**

1. Produire le récap de la phase en texte
2. Produire le bloc `## Retour intermédiaire vers orchestrator`
3. Produire le bloc `## Question pour l'orchestrator`
4. **TERMINER LA SESSION**

---

## Autocontrôle avant chaque fin de session

> « Ai-je produit (1) le récap, (2) le bloc `## Retour intermédiaire vers orchestrator`, ET (3) le bloc `## Question pour l'orchestrator` ? »
> - **Non** → produire les blocs manquants MAINTENANT
> - **Oui** → terminer la session

---

## Format des blocs structurés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** auditor
**Phase :** X — <titre>
**task_id :** <sessionID courant>

**Résumé :** <2-3 phrases décrivant ce qui a été fait dans cette phase>
**Points clés :** <liste courte — découvertes importantes, domaines identifiés, blocages>

---

## Question pour l'orchestrator

**Phase :** X
**task_id :** <sessionID courant>

**Contexte :** <résumé de ce qui a été fait et pourquoi la question>

**Question :** <question exacte>

**Options :**
- `<label-a>` — <description>
- `<label-b>` — <description>

**Instruction de reprise :** "Réponse Phase X auditor : [option]. Reprendre depuis Phase X+1 / <contexte>."
```
→ **TERMINER LA SESSION**

---

## Questions par phase

### Phase 0 — Prérequis vérifiés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** auditor
**Phase :** 0 — Vérification des prérequis
**task_id :** <sessionID courant>

**Résumé :** Prérequis vérifiés — périmètre, stack et accès aux fichiers analysés.
**Points clés :** <domaines à auditer, contraintes légales, limites d'accès identifiées>

---

## Question pour l'orchestrator

**Phase :** 0
**task_id :** <sessionID courant>

**Contexte :** Prérequis vérifiés. Périmètre, stack et accès aux fichiers ont été analysés.

**Question :** Charger le contexte projet (Phase 1) ?

**Options :**
- `charger-contexte` — Passer à la Phase 1 — Chargement contexte projet
- `preciser-perimetre` — Ajuster le périmètre avant de continuer
- `arreter` — Annuler l'audit

**Instruction de reprise :** "Réponse Phase 0 auditor : [option]. Reprendre depuis Phase 1 / chargement contexte."
```
→ **TERMINER LA SESSION**

### Phase 1 — Contexte chargé

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** auditor
**Phase :** 1 — Chargement du contexte projet
**task_id :** <sessionID courant>

**Résumé :** Contexte projet chargé — stack et architecture identifiées via <ONBOARDING.md | reconnaissance rapide>.
**Points clés :** <langages/frameworks clés, pattern architectural, points d'attention identifiés>

---

## Question pour l'orchestrator

**Phase :** 1
**task_id :** <sessionID courant>

**Contexte :** Contexte projet chargé (ONBOARDING.md ou reconnaissance rapide). Stack et architecture identifiées.

**Question :** Passer à la sélection des domaines à auditer (Phase 2) ?

**Options :**
- `selectionner-domaines` — Passer à la Phase 2 — Sélection des domaines
- `recharger-contexte` — Relire ONBOARDING.md ou refaire la reconnaissance rapide
- `arreter` — Annuler l'audit

**Instruction de reprise :** "Réponse Phase 1 auditor : [option]. Reprendre depuis Phase 2 / sélection des domaines."
```
→ **TERMINER LA SESSION**

### Phase 2 — Domaines sélectionnés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** auditor
**Phase :** 2 — Sélection des domaines à auditer
**task_id :** <sessionID courant>

**Résumé :** <N> domaines sélectionnés pour audit, <M> écartés.
**Points clés :** <domaines retenus et leur ordre de délégation, domaines écartés et raison>

---

## Question pour l'orchestrator

**Phase :** 2
**task_id :** <sessionID courant>

**Contexte :** Domaines à auditer sélectionnés en fonction de la demande et de la stack projet.

**Question :** Démarrer les audits (Phase 3) ?

**Options :**
- `demarrer-audits` — Passer à la Phase 3 — Délégation aux sous-agents
- `ajuster-domaines` — Ajouter ou retirer des domaines avant de démarrer
- `arreter` — Annuler l'audit

**Instruction de reprise :** "Réponse Phase 2 auditor : [option]. Reprendre depuis Phase 3 / délégation aux sous-agents."
```
→ **TERMINER LA SESSION**

### Phase 3 — Audits réalisés

```markdown
## Retour intermédiaire vers orchestrator

**Agent :** auditor
**Phase :** 3 — Délégation aux sous-agents spécialisés
**task_id :** <sessionID courant>

**Résumé :** <N> sous-agents invoqués, rapports reçus pour tous les domaines.
**Points clés :** <synthèse par domaine : nombre de critiques/majeurs/mineurs, statut global par domaine>
**Problèmes critiques détectés :** <liste courte des critiques bloquants, ou "Aucun problème critique">

---

## Question pour l'orchestrator

**Phase :** 3
**task_id :** <sessionID courant>

**Contexte :** Les sous-agents spécialisés ont été invoqués et ont retourné leurs rapports d'audit.

**Question :** Passer à la consolidation (Phase 4) ?

**Options :**
- `consolider` — Passer à la Phase 4 — Consolidation et synthèse exécutive
- `relancer-audit` — Relancer un sous-agent pour affiner son rapport
- `arreter` — Stopper avant la consolidation — rapports disponibles individuellement

**Instruction de reprise :** "Réponse Phase 3 auditor : [option]. Reprendre depuis Phase 4 / consolidation."
```
→ **TERMINER LA SESSION**

---

## Format final (Phase 4)

Phase 4 est le **retour final**. Produire dans cet ordre :

1. **La synthèse exécutive multi-domaines** (texte narratif)
2. **Le bloc `## Retour vers orchestrator`** (bloc unique autosuffisant — la synthèse est intégrée dans la section `### Rapport d'audit complet`) — voir skill `audit-handoff-format`

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer — la synthèse est DANS le bloc. »

→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer sans produire les blocs | L'orchestrateur ne reçoit rien | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` | L'orchestrateur ne peut pas reprendre | **Toujours inclure** le sessionID |
| Résumer le récap | L'utilisateur perd des informations | **Ne jamais résumer** |
