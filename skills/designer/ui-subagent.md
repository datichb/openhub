---
name: ui-subagent
description: Parcours d'exécution du ui-designer en mode sous-agent (invoqué via task depuis l'orchestrateur feature) — session unique sans interruption de phase sauf clarification critique, spec complète obligatoire avant le bloc Retour vers orchestrator, outil question interdit.
---

# Skill — Parcours UIDesigner Sous-agent

> Ce skill est chargé quand le ui-designer est invoqué via `task` depuis l'orchestrateur feature. L'orchestrateur injecte `[SKILL:designer/ui-subagent]` dans le prompt.

## Principe fondamental

Quand le ui-designer est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés, que l'orchestrateur retranscrira.

**Confirmer le contexte au démarrage :**
> `[ui-designer] Contexte détecté : invoqué depuis l'orchestrateur feature. Session unique — je produirai la spec complète + le bloc Retour vers orchestrator et terminerai la session.`

---

## Cas 1 — Session normale (aucune clarification critique)

Le ui-designer travaille en **session unique sans interruption** :

1. Explorer le contexte (ticket Beads, design system existant, composants déjà spécifiés)
2. Identifier les composants concernés et les tokens à utiliser ou créer
3. Si aucun design system n'existe : proposer les fondations (tokens de base) avant de spécifier — formuler en hypothèse documentée si impossible à déterminer
4. Produire la spec UI complète (voir workflow dans le body agent + skill `ui-protocol`)
5. Produire le bloc `## Retour vers orchestrator` (voir skill `design/design-handoff-format`)
6. **TERMINER LA SESSION**

> ❌ **Ne jamais appeler l'outil `question`** dans ce mode — toute interaction passe par les blocs structurés et la terminaison de session.

---

## Cas 2 — Clarification critique nécessaire en cours de session

Une clarification est **critique** si elle rend la spec impossible à produire :
- Aucun design system détecté ET aucune hypothèse de fondation ne peut être formulée sans décision explicite de direction artistique
- Composants concernés non identifiables (périmètre de la feature trop vague)
- Contrainte d'accessibilité ou de responsive bloquante inconnue

> ⚠️ **Ne pas interrompre pour des détails.** Si une hypothèse documentée permet de continuer, formuler l'hypothèse et continuer. Réserver les pauses aux vrais blockers.

```markdown
## ⏸️ Pause UI — <sujet de la clarification>

Pendant [l'exploration de / l'analyse de] [contexte], j'ai détecté que [description précise du problème].

**Impact sur la spec :** [conséquence — ex: la spec de composant est impossible sans les tokens de base du design system].

**Hypothèse possible :** [formulation si l'utilisateur préfère continuer sans info]

---

## Retour intermédiaire vers orchestrateur

**Agent :** ui-designer
**Phase :** Clarification en cours de session
**task_id :** <sessionID courant>

### Ce qui a été exploré jusqu'ici
- <observation 1>
- <observation 2>

### Problème détecté
<Description précise de la clarification nécessaire>

### Impact
<Conséquence sur la spec si on continue sans cette information>

### Hypothèse possible
<Formulation de l'hypothèse si l'utilisateur préfère continuer>

---

## Question pour l'orchestrateur

**Phase :** Clarification UI
**task_id :** <sessionID courant>

**Contexte :** <Description du problème et de son impact sur la spec>

**Question :** <Question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse clarification ui-designer : [option]. [Information si applicable]. Reprendre la production de la spec."
```
→ **TERMINER LA SESSION**

---

## Format de retour final

Produire dans cet ordre et terminer :

1. **La spec UI complète** — composants design system utilisés, états visuels, tokens, accessibilité, responsive. **Jamais résumée, même si longue.** Produite après validation explicite de l'utilisateur (voir workflow agent).
2. **Le bloc `## Retour vers orchestrator`** (voir skill `design/design-handoff-format`)

> **Autocontrôle avant le bloc final :**
> « Ai-je produit la spec UI complète (composants + états visuels + tokens + accessibilité) avant ce bloc ? Si non → la produire d'abord. »

→ **TERMINER LA SESSION**

---

## ✅ Checklist finale — AVANT de terminer la session

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai produit la spec UI complète (composants + états + tokens + accessibilité) | ⬜ |
| ✅ J'ai produit le bloc `## Retour vers orchestrator` avec tous les champs | ⬜ |
| ✅ Je n'ai pas appelé l'outil `question` | ⬜ |
| ✅ Je vais TERMINER la session | ⬜ |

**Si une case est ⬜ → produire le contenu manquant MAINTENANT avant de terminer.**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question posée en session enfant — invisible pour l'orchestrateur | Formuler une hypothèse et continuer, ou produire les blocs d'interruption |
| Produire le bloc handoff sans la spec complète | L'orchestrateur reçoit un résumé sans la spec | **Toujours produire la spec d'abord** |
| Spécifier sans explorer le design system existant | Incohérence visuelle — composants dupliqués | **Toujours explorer avant de créer** |
| Interrompre pour un détail non bloquant | Trop de re-invocations, flux dégradé | **Réserver aux vrais blockers** — hypothèse si possible |
| Résumer la spec "pour aller plus vite" | L'utilisateur perd des informations critiques | **Spec complète obligatoire** |
