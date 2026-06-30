---
name: pathfinder-execution-modes
description: Parcours d'exécution du pathfinder — mode standalone (invoqué directement par l'utilisateur, récap en texte clair avant question, outil question pour les pauses, rapport final sans bloc handoff orchestrateur) et mode sous-agent (invoqué via task depuis l'agent orchestrator feature, session unique sans interruption si aucune clarification critique, ou mécanisme d'interruption si clarification critique détectée, bloc Retour vers orchestrator obligatoire en fin de session, ne jamais appeler l'outil question).
bucket: B
---

# Modes d'exécution — Pathfinder

## Détection du mode d'invocation

SI invoqué via `task` depuis un agent parent (présence d'un contexte d'invocation structuré ou d'un task_id) → **MODE SUBAGENT**
SI invoqué directement par l'utilisateur → **MODE STANDALONE**

---

## Mode standalone

> Ce skill est chargé automatiquement quand le pathfinder est invoqué directement par l'utilisateur (aucun `[SKILL:...]` injecté dans le prompt).

## Principe fondamental

En mode standalone, le texte est **directement visible** par l'utilisateur. La communication se fait via :
1. Le texte de réponse (rapport, contexte, récap)
2. L'outil `question` pour les clarifications et pauses

---

## Règle absolue — récap avant question

**Avant tout appel à l'outil `question` :**

1. **TOUJOURS afficher le contexte en texte clair** dans la discussion avant d'appeler `question`
2. **PUIS** appeler l'outil `question`

> ❌ **JAMAIS** : appeler `question` sans avoir d'abord affiché le contexte
> ✅ **TOUJOURS** : afficher le contexte → puis appeler `question`

### Format standard pour une pause avec question

```markdown
## ⏸️ Pause — <sujet>

<Contexte de la pause : ce qui a été observé, ce qui pose question, impact sur la suite>

**Options disponibles :**
- <Option A> → <conséquence>
- <Option B> → <conséquence>
```

Puis appeler l'outil `question`.

---

## Format final (standalone)

Produire le rapport pathfinder complet (voir skill `pathfinder-handoff-format` pour le format exact), **sans** le bloc `## Retour vers orchestrator`.

Après le rapport, si des tickets doivent être créés, demander la confirmation via l'outil `question` :

```
question({
  questions: [{
    header: "Créer les tickets",
    question: "Créer les tickets suggérés dans Beads ?",
    options: [
      { label: "Oui — créer (Recommandé)", description: "Créer les tickets du draft de plan" },
      { label: "Non", description: "Laisser la création au planner (si escalade recommandée)" }
    ]
  }]
})
```

---

## Mode subagent

> Ce skill est chargé quand le pathfinder est invoqué via `task` depuis l'agent orchestrator feature. L'orchestrateur injecte `[SKILL:planning/pathfinder-subagent]` dans le prompt.

## Principe fondamental

Quand le pathfinder est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés.

**Confirmer le contexte au démarrage :**
> `[pathfinder] Contexte détecté : invoqué depuis l'agent orchestrator feature. Mode interruption actif — je terminerai ma session pour remonter le rapport et les éventuelles clarifications à l'agent orchestrator.`

---

## Cas 1 — Session normale (aucune clarification critique)

Le pathfinder travaille en **session unique** sans interruption :
1. Exploration → estimation → rapport complet
2. Produire le rapport pathfinder (voir skill `pathfinder-handoff-format`)
3. Produire le bloc `## Retour vers orchestrator` (voir skill `pathfinder-handoff-format`)
4. **TERMINER LA SESSION**

```markdown
---

## Retour vers orchestrator

**Agent :** pathfinder
**Feature :** <nom>
**Complexité :** <XS|S|M|L|XL>

### Recommandation
`direct` | `escalade-planner`

### Handoff planner
[Présent si escalade — pointer vers la section `## 📦 Handoff vers planner` du rapport]
[Absent si traitement direct]

### Statut
`reconnaissance-complète` | `reconnaissance-partielle`
```

→ **TERMINER LA SESSION**

---

## Cas 2 — Clarification critique nécessaire en cours de session

Une clarification est **critique** si elle change fondamentalement :
- La complexité estimée (XS/S vs L/XL)
- La recommandation (direct vs escalade)
- Le périmètre de la feature

> ⚠️ Ne pas interrompre pour des détails — formuler une hypothèse documentée et continuer si possible.

```markdown
## ⏸️ Pause pathfinder — <sujet de la clarification>

Pendant l'exploration de [contexte], j'ai détecté que [description précise du problème].

**Impact sur le rapport :** [conséquence — ex: l'estimation passe de S à L si le module X est concerné].

**Hypothèse possible :** [formulation si l'utilisateur préfère continuer sans info]

---

## Retour intermédiaire vers orchestrator

**Agent :** pathfinder
**Phase :** Clarification en cours d'exploration
**task_id :** <sessionID courant>

<Reproduire le contenu de la pause ci-dessus>
Ce qui a été exploré jusqu'ici : <résumé rapide des observations>

---

## Question pour l'orchestrator

**Phase :** Clarification
**task_id :** <sessionID courant>

**Contexte :** <description précise du problème et de son impact sur le rapport>

**Question :** <question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse à la clarification pathfinder : [option]. [Information si applicable]. Reprendre l'exploration depuis le point d'interruption."
```
→ **TERMINER LA SESSION**

---

## Autocontrôle final

- [ ] Ai-je produit le rapport pathfinder complet ?
- [ ] Ai-je produit le bloc `## Retour vers orchestrator` ?
- [ ] En cas de clarification interrompue : ai-je produit `## Retour intermédiaire vers orchestrator` + `## Question pour l'orchestrator` avec le `task_id` ?
- [ ] Ai-je terminé la session sans appeler l'outil `question` ?

> ❌ Ne JAMAIS appeler l'outil `question` dans ce mode
> ✅ Toujours produire le rapport complet AVANT le bloc `## Retour vers orchestrator`
