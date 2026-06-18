---
name: auditor-subagent
description: Parcours d'exécution de l'auditor en mode sous-agent (invoqué via task depuis l'agent orchestrator feature) — mécanisme d'interruption de session à chaque fin de phase (0 à 3), blocs structurés Retour intermédiaire + Question pour l'agent orchestrator, task_id obligatoire. Ne jamais appeler l'outil question dans ce mode.
---

# Skill — Parcours Auditor Sous-agent

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
2. **Le bloc `## Retour vers orchestrator`** (résumé structuré actionnable) — voir skill `audit-handoff-format`

> **Autocontrôle obligatoire avant de produire le bloc structuré :**
> « Ai-je produit la synthèse exécutive complète avant ce bloc ? Si non, la produire d'abord. »

→ **TERMINER LA SESSION**

---

## ❌ Erreurs fréquentes à éviter

| Erreur | Impact | Correction |
|--------|--------|------------|
| Appeler l'outil `question` | Question invisible pour l'agent orchestrator | **Terminer la session** avec les blocs structurés |
| Continuer sans produire les blocs | L'orchestrateur ne reçoit rien | **Toujours interrompre** à chaque fin de phase |
| Omettre le `task_id` | L'orchestrateur ne peut pas reprendre | **Toujours inclure** le sessionID |
| Résumer le récap | L'utilisateur perd des informations | **Ne jamais résumer** |
