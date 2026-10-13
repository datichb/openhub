---
name: audit-handoff-format
description: "Utiliser en fin d'audit de domaine pour produire le bloc « Retour vers auditor » (sous-agent) ou le lire (auditor)."
when: role=subagent | calls=auditor-subagent
---

# Skill — Format de handoff d'audit

Ce skill est la **source de vérité** pour le format de retour d'audit, utilisé sur deux étapes :

| Producteur | Consommateur | Titre du bloc |
|------------|--------------|---------------|
| `auditor-subagent` (un domaine) | `auditor` | `## Retour vers auditor` |
| `auditor` (synthèse multi-domaines, invoqué via `task` par l'orchestrator) | `orchestrator` | `## Retour vers orchestrator` |

Il est injecté dans `auditor-subagent` et dans `auditor` : producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

> **Contrat de handoff :** voir skill `shared/handoff-bloc-unique-rule` pour les règles universelles producer/consumer.
>
> **Rapport déposé :** si l'outil `workflow_report` est disponible, dépose ton rapport complet avec lui **avant** le bloc, puis remplace la section du rapport par `Rapport : rp-N` (identifiant reçu) ; sinon (outil absent, réponse « non enregistré », refus), inclus le rapport complet. Côté coordinateur, `Rapport : rp-N` vaut rapport présent (oh le joint à la carte du checkpoint) : cite l'identifiant au lieu de recopier le rapport.

---

## Format du bloc

Structure commune ; l'exemple montre le bloc du sous-agent. L'`auditor` produit la même
structure sous le titre `## Retour vers orchestrator`, avec `**Agent :** auditor (<domaines>)`
et un rapport complet multi-domaines.

```
---

## Retour vers auditor

**Agent :** auditor-subagent (<domaine>)
**Ticket :** #<ID> — <titre>

### Périmètre audité
**Couvert :** <liste des fichiers, répertoires, composants, endpoints analysés>
**Non couvert :** <ce qui n'a pas été analysé et pourquoi — "Périmètre complet couvert" si rien n'a été exclu>

### Synthèse des problèmes identifiés

| Sévérité | Problème | Localisation |
|----------|---------|--------------|
| 🔴 Critique | <titre court> | <fichier:ligne ou composant> |
| 🟠 Majeur | <titre court> | <fichier:ligne ou composant> |
| 🟡 Mineur | <titre court> | <fichier:ligne ou composant> |

<"Aucun problème identifié" si le périmètre est conforme>

### Recommandations priorisées

1. 🔴 **[Critique]** <recommandation 1 — action concrète à réaliser>
   *Effort estimé : <faible | moyen | élevé>*
2. 🟠 **[Majeur]** <recommandation 2>
   *Effort estimé : <faible | moyen | élevé>*
3. 🟡 **[Mineur]** <recommandation 3>
   *Effort estimé : <faible | moyen | élevé>*

<"Aucune recommandation" si aucun problème identifié>

### Risque résiduel si non corrigé
<description du risque global si les corrections identifiées ne sont pas appliquées>
<"Risque résiduel nul — périmètre conforme" si aucun problème>

### Rapport d'audit complet

## Audit <domaine> — <périmètre>

### Contexte et périmètre
<description du périmètre audité, méthodologie, outils utilisés>

### Observations détaillées

#### 🔴 <Problème critique 1 — titre>
**Localisation :** `<fichier:ligne>`
**Description :** <description détaillée du problème>
**Preuve / chemin d'exploitation :** <comment reproduire ou exploiter>
**Impact :** <conséquence si non corrigé>
**Recommandation :** <correction concrète>

#### 🟠 <Problème majeur 1 — titre>
**Localisation :** `<fichier:ligne>`
**Description :** <description détaillée>
**Preuve :** <élément probant>
**Impact :** <conséquence>
**Recommandation :** <correction>

#### 🟡 <Problème mineur 1 — titre>
**Localisation :** `<fichier:ligne>`
**Description :** <description>
**Recommandation :** <correction>

<Répéter pour chaque problème identifié, classé par sévérité décroissante>
<"Aucun problème identifié — le périmètre audité est conforme aux standards" si clean>

### Points positifs
<bonnes pratiques observées — toujours inclure si pertinent>

### Statut
`corrections-requises` | `acceptable` | `bloquant`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `corrections-requises` | Au moins un problème Critique ou Majeur identifié — des corrections sont nécessaires avant mise en production |
| `acceptable` | Uniquement des problèmes Mineurs ou aucun — le code peut être mis en production avec les corrections en backlog |
| `bloquant` | Un problème Critique bloquant identifié qui empêche tout déploiement immédiat |

---

<!-- oh:when agent=auditor-subagent -->
## Règles pour le producteur (auditor-subagent)

- **Toujours inclure `### Rapport d'audit complet`** même si aucun problème n'est identifié — le rapport documente le périmètre audité et les bonnes pratiques observées
- **Toujours renseigner le `### Périmètre audité`** — même si le périmètre est complet, l'indiquer explicitement
- **Toujours renseigner le `### Risque résiduel`** — même si nul, l'indiquer explicitement
- Si aucun problème n'est identifié, renseigner chaque section avec la mention explicite correspondante
- Ne jamais minimiser les preuves ou chemins d'exploitation dans le rapport
<!-- /oh:when -->

---

<!-- oh:when calls=auditor-subagent -->
## Règles pour l'auditor (consommateur du sous-agent)

- **Champs obligatoires** : `Périmètre audité`, `Synthèse des problèmes identifiés`, `Recommandations priorisées`, `Risque résiduel si non corrigé`, `Rapport d'audit complet`, `Statut`. Si l'un est absent → ré-invoquer le sous-agent (même `task_id`) pour qu'il complète.
- **Statut `bloquant`** → pause de Phase 3 (audit bloquant) avant de lancer les domaines suivants.
- **Consolidation** : reprendre score, compteurs par sévérité et recommandations de chaque bloc dans la synthèse de Phase 4 ; signaler le périmètre non couvert de chaque domaine.
<!-- /oh:when -->
