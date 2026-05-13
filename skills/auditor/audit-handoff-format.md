---
name: audit-handoff-format
description: Source de vérité pour le format de retour des agents auditor-* vers l'orchestrator. Définit le bloc structuré à produire quand un agent auditeur termine son rapport et est invoqué depuis l'orchestrator. Injecté dans tous les auditor-* et dans l'orchestrator pour garantir que le producteur et le consommateur partagent le même contrat.
---

# Skill — Format de handoff auditor → orchestrator

Ce skill est la **source de vérité** pour le format de retour des agents `auditor-*` vers l'orchestrator.
Il est injecté dans chaque `auditor-*` et dans l'`orchestrator` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis l'`orchestrator` (et non en standalone),
tu **dois** produire dans cet ordre :

1. **Le rapport d'audit complet** — analyse narrative et détaillée du périmètre : observations item par item, contexte de chaque problème identifié, preuves, chemins d'exploitation si applicable. **Ce rapport doit être produit même si aucun problème n'est identifié.**
2. **Le bloc `## Retour vers orchestrator`** défini ci-dessous — synthèse structurée et actionnelle du rapport.

En standalone, le rapport d'audit complet précède également ce bloc.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le rapport d'audit complet avant ce bloc ? Si non, le produire d'abord. »

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
<Le détail complet de chaque problème est dans le rapport d'audit ci-dessus>

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

- **Toujours produire le rapport d'audit complet** avant ce bloc — même si aucun problème n'est identifié. Le rapport est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite du rapport, même si le statut est `acceptable`
- **Le tableau `### Synthèse des problèmes identifiés`** est une synthèse — le détail complet est dans le rapport narratif qui précède
- **Toujours renseigner le `### Périmètre audité`** — même si le périmètre est complet, l'indiquer explicitement
- **Toujours renseigner le `### Risque résiduel`** — même si nul, l'indiquer explicitement
- Si aucun problème n'est identifié, renseigner chaque section avec la mention explicite correspondante

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit le rapport d'audit complet.
> ❌ Ne jamais résumer le rapport dans le bloc — le bloc est une synthèse structurée, pas un substitut.

---

## Règles pour le consommateur (orchestrator)

### À la réception du retour d'un agent auditor

1. **Afficher le rapport d'audit complet dans le texte de la discussion** (ne pas inclure dans l'outil `question`) avant de poser le CP-audit — ne jamais résumer ni filtrer.
2. **Afficher l'intégralité du bloc dans le texte de la discussion** (ne pas inclure dans l'outil `question`).
3. **Vérifier la présence de tous les champs obligatoires** : `Périmètre audité`, `Synthèse des problèmes identifiés`, `Recommandations priorisées`, `Risque résiduel si non corrigé`, `Statut`.
   - Si l'un de ces champs est absent → demander explicitement à l'agent auditor de compléter avant de continuer.
4. **Si le rapport d'audit complet est absent** (le bloc handoff est présent sans rapport préalable) → demander explicitement à l'agent auditor de produire le rapport complet avant de continuer.
5. **Utiliser le `### Statut`** pour informer la question au CP-audit :
   - `bloquant` → le mentionner explicitement dans la question CP-audit
   - `acceptable` → le mentionner pour aider l'utilisateur à choisir "Accepter"
6. **Transmettre les `### Recommandations priorisées` intégralement** à `orchestrator-dev` si l'utilisateur choisit "Corriger" au CP-audit.
7. **Signaler le `### Périmètre non couvert`** dans le récap CP-feature si des zones n'ont pas été auditées.

> ❌ Ne jamais construire le CP-audit à partir d'un retour incomplet ou sans ce bloc structuré.
> ❌ Ne jamais résumer ou filtrer les recommandations avant de les transmettre à orchestrator-dev.
> ❌ Ne jamais accepter un bloc handoff sans rapport d'audit préalable — les deux sont obligatoires.
