---
name: auditor-standalone
description: Parcours d'exécution de l'auditor en mode standalone (invoqué directement par l'utilisateur) — récaps en texte clair avant chaque appel question, validation via outil question à chaque phase, synthèse finale sans bloc handoff orchestrateur.
---

# Skill — Parcours Auditor Standalone

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
