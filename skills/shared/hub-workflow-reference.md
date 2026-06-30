---
name: hub-workflow-reference
description: Source de vérité canonique du hub — catalogue des agents, heuristique pathfinder vs planner, séquences standard par type de feature, table des handoffs, intégration du complexity scoring. Chargé automatiquement (bucket A) par l'orchestrator et le planner.
bucket: A
source-of-truth: true
---

# Skill — Hub Workflow Reference

> ⚠️ Ce skill est la **source de vérité unique** pour le catalogue agents, l'heuristique de routage et les séquences standard du hub. Toute modification d'un agent doit inclure une mise à jour de ce skill.

---

## Catalogue des agents

| Agent | Famille | Mode | Quand invoquer | Output attendu |
|-------|---------|------|---------------|---------------|
| `onboarder` | planning | primary | Projet inconnu — aucun contexte disponible en session | `ONBOARDING.md`, `CONVENTIONS.md`, carte des agents recommandés |
| `pathfinder` | planning | primary | Feature simple ou exploratoire — score complexity ≤ Medium (≤ 10 pts) | Rapport reconnaissance + estimation XS/S/M/L/XL + recommandation `direct` ou `escalade-planner` |
| `planner` | planning | primary | Feature complexe — signal UX/audit détecté ou score ≥ Large (≥ 11 pts) | Tickets Beads enrichis + champ `Agent prévu` par ticket + `Ordre de traitement` |
| `designer` | design | primary | Spec UX (flows, friction, états) et/ou spec UI (tokens, composants, accessibilité). Seul agent avec accès Figma. Modes : recon, ux, ui, ux+ui. | Spec UX/UI complète selon le mode — user flows, tokens, critères d'acceptance, handoff vers orchestrator |
| `auditor` | auditor | primary | Sécurité, performance, accessibilité, RGPD, éco-conception, architecture, observabilité | Rapport d'audit structuré + recommandations priorisées + statut `corrections-requises`/`acceptable`/`bloquant` |
| `orchestrator-dev` | planning | primary | Implémentation de tickets Beads prêts (statut `ready`) | Récap implémentation par ticket — statut, fichiers clés, critères couverts, points d'attention |
| `debugger` | quality | primary | Bug signalé avec artefacts (stacktrace, logs, description reproductible) | Rapport de diagnostic avec hypothèses graduées + ticket de correction Beads |

> **Note :** `developer-*`, `reviewer`, `qa-engineer`, `documentarian` sont invoqués par `orchestrator-dev` lors de la phase d'implémentation — jamais directement par l'agent orchestrator feature.

---

## Heuristique de routage — Pathfinder vs Planner

### Invoquer `pathfinder` (reconnaissance rapide) si

- **Mots-clés de simplicité** : "simple", "petit", "rapide", "ajouter un champ", "modifier le style"
- **Phase exploratoire** : "explorer", "voir si", "tester l'idée", "POC", "prototype"
- **Demande explicite** : "quick scan", "pathfinder", "regarde rapidement", "estimation rapide"
- **Feature apparemment simple** sans signal complexe évident
- **Score complexity (Phase 0.5)** : Small (4–6 pts) ou Medium (7–10 pts)

### Invoquer directement `planner` (analyse complète) si

- **Mots-clés de complexité** : "refonte", "nouveau système", "architecture", "migration", "refactorisation majeure"
- **Signaux spéciaux** : "UX", "design", "sécurité", "performance", "RGPD", "accessibilité", "audit"
- **Feature clairement complexe** : multi-composants, impact large, plusieurs modules
- **Demande explicite** : "planifie complètement", "structure détaillée", "analyse approfondie"
- **Score complexity (Phase 0.5)** : Large (11–13 pts) ou Enterprise (14–16 pts)

### En cas de doute (critères mixtes)

Poser la question via l'outil `question` :

```
question({
  questions: [{
    header: "Mode de planification",
    question: "Cette feature peut être traitée de deux façons :\n\n- **Pathfinder** (reconnaissance rapide, estimation + recommandation)\n- **Planner** (analyse complète 7 phases, tickets Beads enrichis)\n\nQuel mode préférez-vous ?",
    options: [
      { label: "Pathfinder (Recommandé)", description: "Reconnaissance rapide — peut escalader si nécessaire" },
      { label: "Planner direct", description: "Analyse complète 7 phases dès le départ" }
    ]
  }]
})
```

### Par défaut (aucun signal clair)

→ Commencer par `pathfinder` — peut escalader vers `planner` si nécessaire.

### Exemples

| Demande | Routing | Justification |
|---------|---------|---------------|
| "Ajoute un champ email au profil" | **Pathfinder** | Simplicité évidente |
| "Refonte complète du système d'auth" | **Planner** | Mot-clé "refonte" + complexité évidente |
| "Dashboard analytics avec UX optimisée" | **Planner** | Signal UX détecté |
| "Voir si on peut intégrer Stripe" | **Pathfinder** | Phase exploratoire |
| "Système de notifications temps réel" | **Doute** → Question | Peut être simple ou complexe |

---

## Séquences standard

L'`### Ordre de traitement` fourni par le planner **prime toujours** sur ces variantes.
Ces séquences sont des références — pas des contraintes rigides.

| Variante | Séquence |
|----------|---------|
| **Solo simple** | `pathfinder` → `orchestrator-dev` |
| **Solo complet** | `planner` → `orchestrator-dev` |
| **Avec conception UX** | `planner` → `designer` (Mode: ux) → `orchestrator-dev` |
| **Avec conception complète** | `planner` → `designer` (Mode: ux+ui) → `orchestrator-dev` |
| **Avec audit** | `planner` → `auditor` → `orchestrator-dev` |
| **Complète** | `planner` → `designer` (Mode: ux+ui) → `auditor` → `orchestrator-dev` |
| **Bug isolé** | `debugger` → `orchestrator-dev` (si ticket de correction créé) |
| **Projet inconnu** | `onboarder` → Mode A ou B |

---

## Table des handoffs

| Émetteur | Skill de format | Récepteur | Bucket |
|----------|----------------|-----------|--------|
| `planner` | `planning/planner-handoff-format` | `orchestrator` | A |
| `pathfinder` | `planning/pathfinder-handoff-format` | `orchestrator` | B |
| `onboarder` | `planning/onboarder-handoff-format` | `orchestrator` | B |
| `designer` | `design/design-handoff-format` | `orchestrator` | B |
| `auditor` | `auditor/audit-handoff-format` | `orchestrator` | B |
| `debugger` | `quality/debugger-handoff-format` | `orchestrator` | B |
| `orchestrator-dev` | `orchestrator/orchestrator-handoff-format` | `orchestrator` | A |
| `developer-*` | `developer/developer-handoff-format` | `orchestrator-dev` | A |
| `reviewer` | `reviewer/reviewer-handoff-format` | `orchestrator-dev` | A |
| `qa-engineer` | `qa/qa-handoff-format` | `orchestrator-dev` | A |
| `documentarian` | `documentarian/documentarian-handoff-format` | `orchestrator-dev` | A |

> Les skills de format Bucket B doivent être chargés via l'outil `skill` **avant** d'invoquer l'agent correspondant — ils définissent le contrat de réception.

---

## Règle de gouvernance

> Tout nouvel agent ajouté au hub **doit** inclure une mise à jour de ce skill (`hub-workflow-reference.md`).
> Documenter dans `docs/guides/authoring.fr.md` à la section "Ajouter un agent".
