---
name: audit-handoff-format
description: Source de vérité pour le format de retour des agents auditor-* vers l'orchestrator. Définit le bloc structuré unique à produire quand un agent auditeur termine son rapport et est invoqué depuis l'orchestrator. Le rapport d'audit complet est intégré dans le bloc. Injecté dans tous les auditor-* et dans l'orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff auditor → orchestrator

Ce skill est la **source de vérité** pour le format de retour des agents `auditor-*` vers l'orchestrator.
Il est injecté dans chaque `auditor-*` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Principe fondamental — bloc unique

Quand tu es invoqué depuis l'`orchestrator` (et non en standalone),
ton **seul output** est le bloc `## Retour vers orchestrator` défini ci-dessous.

**Règle absolue :** aucun texte avant, après ou en dehors de ce bloc. Le rapport d'audit complet (preuves, contexte, chemins d'exploitation) est **intégré dans le bloc** (section `### Rapport d'audit complet`), pas produit séparément en texte libre.

> **Autocontrôle obligatoire avant de terminer la session :**
> « Mon output contient-il du texte en dehors du bloc `## Retour vers orchestrator` ? Si oui, le supprimer et vérifier que le rapport d'audit est bien dans la section `### Rapport d'audit complet` du bloc. »

---

## Format du bloc `## Retour vers orchestrator`

```
---

## Retour vers orchestrator

**Agent :** auditor-<domaine>
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

## Règles pour le producteur (auditor-*)

- **Produire UNIQUEMENT le bloc `## Retour vers orchestrator`** — aucun texte avant ou après
- **Le rapport d'audit complet est DANS le bloc** (section `### Rapport d'audit complet`) — ne pas le produire séparément en texte libre
- **Toujours inclure `### Rapport d'audit complet`** même si aucun problème n'est identifié — le rapport documente le périmètre audité et les bonnes pratiques observées
- **Toujours renseigner le `### Périmètre audité`** — même si le périmètre est complet, l'indiquer explicitement
- **Toujours renseigner le `### Risque résiduel`** — même si nul, l'indiquer explicitement
- Si aucun problème n'est identifié, renseigner chaque section avec la mention explicite correspondante

> ❌ Ne jamais écrire de texte en dehors du bloc de handoff
> ❌ Ne jamais produire le rapport comme texte libre avant le bloc — il est DANS le bloc
> ❌ Ne jamais minimiser les preuves ou chemins d'exploitation dans le rapport

---

## Règles pour le consommateur (orchestrator)

**Spécificités auditor à vérifier :**

- **Champs obligatoires** : `Périmètre audité`, `Synthèse des problèmes identifiés`, `Recommandations priorisées`, `Risque résiduel si non corrigé`, `Rapport d'audit complet`, `Statut`. Si l'un est absent → demander à l'agent auditor de compléter avant de continuer.
- **Retranscription** : afficher les champs du bloc de manière formatée dans la discussion (voir skill `retranscription-coordinateur`). Le `### Rapport d'audit complet` est affiché intégralement à l'utilisateur.
- **Statut** : `bloquant` → le mentionner explicitement dans la question CP-audit · `acceptable` → le mentionner pour aider l'utilisateur à choisir "Accepter".
- **Corrections** : transmettre les `### Recommandations priorisées` **intégralement** à `orchestrator-dev` si l'utilisateur choisit "Corriger" au CP-audit — ne jamais les résumer.
- **Périmètre non couvert** : signaler dans le récap CP-feature si des zones n'ont pas été auditées.
