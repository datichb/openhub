---
name: designer-subagent
description: Parcours d'exécution du Designer en mode sous-agent (invoqué via task depuis l'agent orchestrator) — session unique sans interruption de phase sauf clarification critique, seul output = le bloc Retour vers orchestrator (spec intégrée dans le bloc), outil question interdit.
bucket: B
---

# Skill — Parcours Designer Sous-agent

> Ce skill est chargé quand le designer est invoqué via `task` depuis l'agent orchestrator. L'orchestrateur injecte `[SKILL:designer/designer-subagent]` dans le prompt.

## Principe fondamental

Quand le designer est invoqué via `task`, le texte de la session enfant n'est **PAS visible** par l'utilisateur. La seule façon de remonter du contenu est de **terminer la session** avec les blocs structurés, que l'agent orchestrator retranscrira.

**Confirmer le contexte au démarrage :**
> `[designer] Mode : <recon|ux|ui|ux+ui> — Contexte détecté : invoqué depuis l'agent orchestrator. Session unique — je produirai la spec complète + le bloc Retour vers orchestrator et terminerai la session.`

---

## Cas 1 — Session normale (aucune clarification critique)

Le designer travaille en **session unique sans interruption** :

**Mode recon :**
1. Exécuter la reconnaissance Figma (skill `designer/figma-recon-protocol`)
2. Produire le bloc `## Retour recon Figma`
3. Produire le bloc `## Retour vers orchestrator`
4. **TERMINER LA SESSION**

**Mode ux :**
1. Explorer le contexte (ticket Beads, codebase, parcours existants)
2. Charger `designer/figma-deep-protocol` si Figma disponible
3. Formuler les questions de contexte utilisateur **en hypothèses documentées** — pas via l'outil `question`
4. Produire le user flow complet + la spec UX (voir skill `designer/ux-protocol`)
5. Produire le bloc `## Retour vers orchestrator` (voir skill `design/design-handoff-format`)
6. **TERMINER LA SESSION**

**Mode ui :**
1. Explorer le contexte (ticket Beads, design system existant, composants déjà spécifiés)
2. Charger `designer/figma-deep-protocol` si Figma disponible
3. Identifier les composants concernés et les tokens à utiliser ou créer
4. Si aucun design system détecté : proposer les fondations en hypothèse documentée
5. Produire la spec UI complète (voir skill `designer/ui-protocol`)
6. Produire le bloc `## Retour vers orchestrator` (voir skill `design/design-handoff-format`)
7. **TERMINER LA SESSION**

**Mode ux+ui :**
1. Exécuter entièrement le workflow mode **ux** (étapes 1-4)
2. Exécuter le workflow mode **ui** en intégrant la spec UX comme contexte
3. Produire le bloc `## Retour vers orchestrator`
4. **TERMINER LA SESSION**

> ❌ **Ne jamais appeler l'outil `question`** dans ce mode — toute interaction passe par les blocs structurés et la terminaison de session.

---

## Cas 2 — Clarification critique nécessaire en cours de session

Une clarification est **critique** si elle rend la spec impossible à produire :

**UX :**
- Informations utilisateur insuffisantes pour définir le user flow nominal
- Périmètre de la feature non déterminable (comportement attendu contradictoire)
- Contrainte technique structurante inconnue qui changerait fondamentalement la spec

**UI :**
- Aucun design system détecté ET aucune hypothèse de fondation ne peut être formulée sans décision explicite de direction artistique
- Composants concernés non identifiables (périmètre de la feature trop vague)

> ⚠️ **Ne pas interrompre pour des détails.** Si une hypothèse documentée permet de continuer, formuler l'hypothèse et continuer. Réserver les pauses aux vrais blockers.

```markdown
## ⏸️ Pause Designer — <sujet de la clarification>

Pendant [l'exploration de / l'analyse de] [contexte], j'ai détecté que [description précise du problème].

**Mode actif :** <recon|ux|ui|ux+ui>

**Impact sur la spec :** [conséquence]

**Hypothèse possible :** [formulation si l'utilisateur préfère continuer sans info]

---

## Retour intermédiaire vers orchestrator

**Agent :** designer
**Mode :** <recon|ux|ui|ux+ui>
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

## Question pour l'orchestrator

**Phase :** Clarification Designer
**task_id :** <sessionID courant>

**Contexte :** <Description du problème et de son impact sur la spec>

**Question :** <Question précise>

**Options :**
- `fournir-information` — Fournir l'information maintenant
- `continuer-hypothese` — Continuer avec l'hypothèse : [formulation]

**Instruction de reprise :** "Réponse clarification designer : [option]. [Information si applicable]. Reprendre la production de la spec en mode <mode>."
```
→ **TERMINER LA SESSION**

---

## Format de retour final

Produire **uniquement** le bloc `## Retour vers orchestrator` (voir skill `design/design-handoff-format`) et terminer.

**Règle absolue :** aucun texte avant, après ou en dehors du bloc. La spec complète est **intégrée dans le bloc** (section `### Spec complète`), pas produite séparément en texte libre.

> **Autocontrôle avant de terminer :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que la spec est bien dans la section `### Spec complète` du bloc. »

→ **TERMINER LA SESSION**

---

## ✅ Checklist finale — AVANT de terminer la session

| Vérification | Fait ? |
|--------------|--------|
| ✅ J'ai annoncé le mode détecté au démarrage | ⬜ |
| ✅ Mon seul output est le bloc `## Retour vers orchestrator` | ⬜ |
| ✅ La section `### Spec complète` du bloc contient la spec intégrale (jamais résumée) | ⬜ |
| ✅ Aucun texte en dehors du bloc (pas de spec séparée, pas de narratif) | ⬜ |
| ✅ Je n'ai pas appelé l'outil `question` | ⬜ |
| ✅ Je vais TERMINER la session | ⬜ |

**Si une case est ⬜ → corriger MAINTENANT avant de terminer.**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question posée en session enfant — invisible pour l'orchestrator | Formuler une hypothèse et continuer, ou produire les blocs d'interruption |
| Produire la spec en texte libre AVANT le bloc | Duplication — la spec doit être DANS le bloc (`### Spec complète`) | **Intégrer la spec dans le bloc** |
| Écrire du texte en dehors du bloc | Bruit pour le coordinateur — augmente le coût sans apporter de valeur | **Bloc unique, rien d'autre** |
| Interrompre pour un détail non bloquant | Trop de re-invocations, flux dégradé | **Réserver aux vrais blockers** — hypothèse si possible |
| Résumer la spec "pour aller plus vite" | L'utilisateur perd des informations critiques | **Spec complète obligatoire dans `### Spec complète`** |
| Spécifier sans explorer le design system existant (mode ui) | Incohérence visuelle — composants dupliqués | **Toujours explorer avant de créer** |
