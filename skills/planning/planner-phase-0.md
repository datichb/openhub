---
name: planner-phase-0
description: Phase 0 (prérequis) et Phase 0.5 (complexity scoring) du workflow planner.
---

# Phase 0 — Vérification des prérequis

## Objectif
Vérifier que les informations minimales pour démarrer la planification sont disponibles.

## Ce qu'on vérifie
- La feature est compréhensible (titre + description ou contexte minimal)
- Le projet est accessible (répertoire courant, `.beads/` trouvé)
- Au moins un point d'entrée pour démarrer l'exploration

## Déclencheur de pause ⏸️

Si **un ou plusieurs prérequis critiques sont manquants** :

**Si CONTEXTE = standalone :**
```
[Texte de réponse]
## ⏸️ Phase 0 — Prérequis manquants

Pour démarrer la planification dans de bonnes conditions, j'ai besoin de :
1. <élément manquant 1 — ex : description de la feature>
2. <élément manquant 2 — ex : accès au projet>

**Impact :** Sans ces éléments, [conséquence].

question({
  questions: [{
    header: "Prérequis manquants",
    question: "[Planner — Phase 0 : Prérequis | Feature : <nom>]\nPour démarrer l'analyse, j'ai besoin de :\n<liste numérotée>\n\nComment procéder ?",
    options: [
      { label: "Fournir les informations", description: "Préciser les éléments manquants maintenant" },
      { label: "Continuer quand même", description: "Démarrer avec les informations disponibles — la planification sera partielle" }
    ]
  }]
})
```

**Si CONTEXTE = orchestrator_feature :** utiliser le format d'interruption de session (voir section "Cas particulier : pause ad hoc").

## Récap de fin de Phase 0

```markdown
## [Phase 0] Prérequis vérifiés

**Contexte identifié :**
- Feature : <nom de la feature pressentie>
- Projet : <nom du projet ou répertoire courant>
- Board Beads : <chemin vers .beads/>

**Prérequis manquants (si applicable) :**
- <élément manquant 1> — hypothèse formulée : <hypothèse>

**Hypothèses formulées :**
- <hypothèse 1 si un prérequis manque>
```

## Question de validation obligatoire

**Si CONTEXTE = standalone :**
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

**Selon la réponse (dans tous les contextes) :**
- **Démarrer** → Phase 1
- **Préciser** → rester en Phase 0, intégrer les nouvelles informations, re-produire le récap
- **Arrêter** → fin de session

---

# Phase 0.5 — Complexity Scoring

## Objectif

Calibrer la profondeur de planification avant l'exploration. Un projet simple mérite un plan léger ; un projet enterprise mérite toutes les phases obligatoires.

## Grille de scoring (4 critères, 1–4 pts chacun)

| Critère | 1 pt | 2 pts | 3 pts | 4 pts |
|---------|------|-------|-------|-------|
| **Domaines techniques** | 1 seul | 2 | 3 | 4+ |
| **Intégrations tierces** | 0 | 1 | 2–3 | 4+ |
| **Sensibilité sécurité** | Faible | Moyenne | Haute | Critique |
| **Taille codebase estimée** | < 500 LOC | 500–5K | 5K–50K | 50K+ |

**Score total = somme des 4 critères (4–16 pts)**

## Tiers et comportement conditionnel

| Tier | Score | Comportement |
|------|-------|-------------|
| **Small** | 4–6 | Plan léger — pathfinder optionnel, 3–5 tâches Beads attendues, Phase 4 allégée |
| **Medium** | 7–10 | Flow standard — pathfinder recommandé, 5–15 tâches Beads |
| **Large** | 11–13 | Pathfinder obligatoire + audit pré-implem recommandé, tickets structurés |
| **Enterprise** | 14–16 | Toutes phases obligatoires + onboarder pre-flight si contexte absent, architecture review |

## Calcul et annonce

Calculer le score à partir des informations disponibles (description de la feature, codebase détecté, contexte fourni). En cas d'incertitude sur un critère, prendre la valeur la plus basse (prudence sur le scope initial).

Annoncer le tier détecté :
```
[Complexity Scoring] Score : X/16 → Tier : Small / Medium / Large / Enterprise
Critères : Domaines: X pts | Intégrations: X pts | Sécurité: X pts | Codebase: X pts
Comportement : <conséquence du tier sur la planification>
```

> Ce scoring est informatif, pas bloquant. Si l'utilisateur conteste le tier, l'ajuster explicitement et continuer.

## Récap de fin de Phase 0.5

Ce scoring est intégré dans le récap Phase 0 existant — pas de récap séparé ni de checkpoint supplémentaire.
