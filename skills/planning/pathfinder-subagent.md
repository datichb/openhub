---
name: pathfinder-subagent
description: Parcours d'exécution du pathfinder en mode sous-agent (invoqué via task depuis l'agent orchestrator feature) — session unique sans interruption si aucune clarification critique, ou mécanisme d'interruption si clarification critique détectée. Bloc Retour vers orchestrator obligatoire en fin de session.
---

# Skill — Parcours Pathfinder Sous-agent

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
