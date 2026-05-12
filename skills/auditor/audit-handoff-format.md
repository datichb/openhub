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
tu **dois** conclure ta session avec le bloc `## Retour vers orchestrator` défini ci-dessous,
après avoir produit ton rapport complet.

En standalone, tu produis ton rapport habituel — ce bloc structuré vient s'y ajouter en conclusion.

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

### Vulnérabilités / Problèmes identifiés

| Sévérité | Problème | Localisation | Description |
|----------|---------|--------------|-------------|
| 🔴 Critique | <titre court> | <fichier:ligne ou composant> | <description précise> |
| 🟠 Majeur | <titre court> | <fichier:ligne ou composant> | <description précise> |
| 🟡 Mineur | <titre court> | <fichier:ligne ou composant> | <description précise> |

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

- **Reproduire le rapport intégral** dans le bloc `### Vulnérabilités / Problèmes identifiés` — ne jamais résumer
- **Toujours renseigner le `### Périmètre audité`** — même si le périmètre est complet, l'indiquer explicitement
- **Toujours renseigner le `### Risque résiduel`** — même si nul, l'indiquer explicitement
- Si aucun problème n'est identifié, renseigner chaque section avec la mention explicite correspondante

---

## Règles pour le consommateur (orchestrator)

### À la réception du bloc `## Retour vers orchestrator` d'un agent auditor

1. **Afficher l'intégralité du bloc** dans la discussion avant de poser le CP-audit — ne jamais résumer.
2. **Vérifier la présence de tous les champs obligatoires** : `Périmètre audité`, `Vulnérabilités / Problèmes identifiés`, `Recommandations priorisées`, `Risque résiduel si non corrigé`, `Statut`.
   - Si l'un de ces champs est absent → demander explicitement à l'agent auditor de compléter avant de continuer.
3. **Utiliser le `### Statut`** pour informer la question au CP-audit :
   - `bloquant` → le mentionner explicitement dans la question CP-audit
   - `acceptable` → le mentionner pour aider l'utilisateur à choisir "Accepter"
4. **Transmettre les `### Recommandations priorisées` intégralement** à `orchestrator-dev` si l'utilisateur choisit "Corriger" au CP-audit.
5. **Signaler le `### Périmètre non couvert`** dans le récap CP-feature si des zones n'ont pas été auditées.

> ❌ Ne jamais construire le CP-audit à partir d'un retour incomplet ou sans ce bloc structuré.
> ❌ Ne jamais résumer ou filtrer les recommandations avant de les transmettre à orchestrator-dev.
