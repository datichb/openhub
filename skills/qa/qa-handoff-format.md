---
name: qa-handoff-format
description: Source de vérité pour le format de retour du qa-engineer vers orchestrator-dev. Définit le bloc structuré à produire en fin de session QA quand invoqué depuis orchestrator-dev. Injecté dans qa-engineer et orchestrator-dev pour garantir que producteur et consommateur partagent le même contrat.
---

# Skill — Format de handoff qa-engineer → orchestrator-dev

Ce skill est la **source de vérité** pour le format de retour du `qa-engineer` vers `orchestrator-dev`.
Il est injecté dans le `qa-engineer` et dans `orchestrator-dev` — producteur et consommateur partagent le même contrat.

---

## Quand produire ce bloc

Quand tu es invoqué depuis `orchestrator-dev` (via l'outil `Task`),
tu **dois** conclure ta session avec le bloc `## Retour vers orchestrator-dev` défini ci-dessous,
après avoir produit ton rapport de couverture complet et écrit les tests.

Ce bloc vient **après** ton rapport QA habituel — il en est le résumé actionnable structuré.

---

## Format du bloc `## Retour vers orchestrator-dev`

```
---

## Retour vers orchestrator-dev

**Agent :** qa-engineer
**Ticket :** #<ID> — <titre>
**Branche :** <nom de la branche analysée>

### Tests écrits
**Nombre de tests ajoutés :** X (Y unitaires, Z intégration, W E2E)
**Fichiers créés ou modifiés :**
- `<chemin/vers/fichier.test.ts>` — <type> — <cas couverts en résumé>
- `<chemin/vers/fichier.test.ts>` — <type> — <cas couverts en résumé>
<"Aucun test écrit" si aucun test n'a pu être ajouté>

### Couverture des critères d'acceptance
- [x] <critère 1 du ticket> — couvert par `<fichier:test>`
- [x] <critère 2> — couvert
- [ ] <critère 3> — **non couvert** — <raison>
<"Tous les critères d'acceptance sont couverts" si applicable>

### Zones non testables identifiées
- <module ou fonction non testable 1 — raison : couplage fort / absence d'injection / etc.>
<"Aucune zone non testable identifiée" si tout est testable>

### Statut
`couverture-complète` | `couverture-partielle` | `non-testable`
```

**Définitions du statut :**

| Statut | Condition |
|--------|-----------|
| `couverture-complète` | Tous les critères d'acceptance ont au moins un test, aucune zone bloquante |
| `couverture-partielle` | Certains critères non couverts ou zones non testables identifiées |
| `non-testable` | L'implémentation reçue ne peut pas être testée sans refactoring préalable |

---

## Règles pour le producteur (qa-engineer)

- **Toujours produire ce bloc**, même si aucun test n'a pu être écrit
- **La `### Couverture des critères d'acceptance`** doit être basée sur `bd show <ID>` — ne pas supposer les critères
- **Signaler honnêtement les zones non testables** — `orchestrator-dev` en a besoin pour informer sur la qualité globale
- Si aucun test n'a été écrit (statut `non-testable`), expliquer clairement pourquoi dans les zones non testables

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du qa-engineer

1. **Lire le `### Statut`** pour évaluer si la couverture est suffisante avant la review :
   - `couverture-complète` → continuer vers l'étape 4 (review) normalement
   - `couverture-partielle` → inclure les critères non couverts dans le prompt envoyé au reviewer (pour qu'il en tienne compte dans sa review)
   - `non-testable` → signaler dans le compte rendu d'étape (étape 6) comme point d'attention

2. **Intégrer les `### Tests écrits`** dans le prompt envoyé au reviewer à l'étape 4 :
   > Fournir au reviewer : le diff de la branche **incluant les tests QA ajoutés** + la liste des fichiers de test créés

3. **Intégrer les `### Zones non testables identifiées`** dans le compte rendu d'étape (étape 6) comme point d'attention technique.

4. **Si le bloc est absent** → demander explicitement au qa-engineer de le produire avant de continuer.

> ❌ Ne jamais passer à la review sans avoir vérifié le statut QA — une couverture `non-testable` doit être signalée.
> ❌ Ne jamais ignorer les critères d'acceptance non couverts — les transmettre au reviewer.
