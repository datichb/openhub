---
name: orchestrator-workflow-modes
description: Source de vérité unique pour les trois modes de workflow (manuel/semi-auto/auto) — tableau des comportements par checkpoint, règles absolues, et blocs question canoniques pour le choix du mode et du QA global. Injecté dans orchestrator et orchestrator-dev pour garantir la cohérence des descriptions et éviter toute désynchro.
---

# Skill — Modes de workflow

Ce skill est la **source de vérité unique** pour les trois modes de workflow.
Il est injecté dans `orchestrator` et `orchestrator-dev` — toute modification ici s'applique aux deux agents.

---

## Les trois modes

| Mode | CP-0 | CP-1 | CP-QA | CP-2 | CP-3 |
|------|------|------|-------|------|------|
| `manuel` _(défaut)_ | ⏸️ pause | ⏸️ pause | ⏸️ pause | ⏸️ pause | ⏸️ pause |
| `semi-auto` | ⏸️ pause | ▶️ auto | ⏸️ pause | ⏸️ **pause** | ▶️ auto |
| `auto` | ⏸️ pause (+ choix QA global) | ▶️ auto | ▶️ valeur fixée en CP-0 | ⏸️ **pause** | ▶️ auto |

> Quand un CP est `▶️ auto`, l'orchestrateur affiche quand même l'information mais enchaîne sans attendre de confirmation.

---

## Règles absolues sur les modes

- **Le mode par défaut est `manuel`** si rien n'est précisé.
- **CP-2 (commit ou corriger ?) est une pause dans TOUS les modes sans exception** — cette règle ne peut pas être outrepassée, même en mode `auto`.
- Ne jamais passer en mode `semi-auto` ou `auto` sans que ce mode ait été choisi explicitement par l'utilisateur.
- L'utilisateur peut taper "stop" à n'importe quel moment — tous les modes honorent cette commande.

---

## Bloc question — Choix du mode

Utiliser ce bloc exact via l'outil `question` pour demander le mode :

```
question({
  header: "Mode de workflow",
  question: "Quel mode de workflow pour les phases d'implémentation ?",
  options: [
    { label: "Manuel (Recommandé)", description: "Chaque étape attend ta confirmation — CP-1, CP-QA, CP-2, CP-3 tous en pause" },
    { label: "Semi-auto", description: "CP-1 et CP-3 automatiques, CP-QA et CP-2 (commit) restent manuels" },
    { label: "Auto", description: "Workflow entièrement automatique sauf CP-2 (commit) — QA configurable au démarrage" }
  ]
})
```

Enregistrer le mode pour toute la session.

---

## Bloc question — QA global (mode `auto` uniquement)

En mode `auto`, poser également via l'outil `question` :

```
question({
  header: "QA global",
  question: "QA activé pour tous les tickets d'implémentation ?",
  options: [
    { label: "Non (Recommandé)", description: "QA skippé pour tous les tickets — review directe après implémentation" },
    { label: "Oui", description: "QA activé pour tous les tickets — qa-engineer invoqué avant chaque review" }
  ]
})
```

La valeur choisie est fixée pour toute la session et appliquée automatiquement à chaque CP-QA.

---

## Comportement selon le contexte d'invocation

- **Invoqué standalone** : demander le mode via le bloc question ci-dessus au CP-0.
- **Invoqué depuis l'orchestrateur feature** : le mode est transmis dans le texte du prompt — ne pas redemander le mode, démarrer directement avec la valeur reçue.

  **Format de transmission requis :** le mode doit figurer dans le prompt sous l'une des trois valeurs canoniques exactes suivantes :
  - `manuel`
  - `semi-auto`
  - `auto`

  Ne jamais transmettre le label brut de l'option d'interface (ex : `"Manuel (Recommandé)"`, `"Semi-auto"`) — normaliser en minuscule avant transmission. Exemple de formulation correcte dans le prompt : `Mode de workflow : semi-auto`.
