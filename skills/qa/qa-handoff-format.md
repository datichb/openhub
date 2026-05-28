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
tu **dois** produire dans cet ordre :

1. **Le rapport QA complet** — liste détaillée des tests écrits, analyse de la couverture par critère d'acceptance, justification des zones non testables. **Ce rapport doit être produit même si aucun test n'a pu être écrit (statut `non-testable`).**
2. **Le bloc `## Retour vers orchestrator-dev`** défini ci-dessous — résumé structuré actionnable.

Ce bloc vient **après** le rapport QA — il en est le résumé structuré. Il ne le remplace pas.

> **Autocontrôle obligatoire avant de produire ce bloc :**
> « Ai-je produit le rapport QA complet avant ce bloc ? Si non, le produire d'abord. »

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

### Points d'attention pour la review
<Liste des éléments que le reviewer devrait vérifier en priorité :>
- ⚠️ <Zone de code non testable et pourquoi (couplage fort, dépendances externes hard-codées, etc.)>
- ⚠️ <Edge cases volontairement non couverts et justification>
- ⚠️ <Hypothèses faites sur le comportement (ex: "j'ai supposé que userId est toujours défini")>
- 💡 <Suggestion optionnelle : refactoring pour améliorer la testabilité>
<"Aucun point d'attention particulier" si tout est couvert et testable>

**Exemple :**
```
### Points d'attention pour la review

- ⚠️ La méthode `AuthService.validateToken()` n'est pas testable en l'état car elle appelle directement `jwt.verify()` sans injection de dépendance. Le reviewer devrait vérifier manuellement la logique de gestion des tokens expirés.
- ⚠️ Le edge case "userId null" n'est pas couvert car le type ne le permet pas, mais si l'API change, ce cas pourrait survenir.
- 💡 Suggestion : extraire la logique de calcul de `getTotalPrice()` dans une fonction pure pour faciliter les tests unitaires.
```

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

- **Toujours produire le rapport QA complet** avant ce bloc — même si aucun test n'a pu être écrit. Le rapport est obligatoire dans tous les cas.
- **Toujours produire ce bloc** à la suite du rapport, même si le statut est `non-testable`
- **La `### Couverture des critères d'acceptance`** doit être basée sur `bd show <ID>` — ne pas supposer les critères
- **Signaler honnêtement les zones non testables** — `orchestrator-dev` en a besoin pour informer sur la qualité globale
- **Remplir la section `### Points d'attention pour la review`** avec les éléments que le reviewer devrait vérifier en priorité — c'est ta valeur ajoutée pour faciliter la review
- Si aucun test n'a été écrit (statut `non-testable`), expliquer clairement pourquoi dans les zones non testables

> ❌ Ne jamais produire le bloc handoff sans avoir d'abord produit le rapport QA complet.
> ❌ Ne jamais résumer le rapport — le bloc est un résumé structuré, pas un substitut.

---

## Règles pour le consommateur (orchestrator-dev)

### À la réception du bloc `## Retour vers orchestrator-dev` du qa-engineer

1. **Lire le `### Statut`** pour évaluer si la couverture est suffisante avant la review :
   - `couverture-complète` → continuer vers l'étape 4 (review) normalement
   - `couverture-partielle` → inclure les critères non couverts dans le prompt envoyé au reviewer (pour qu'il en tienne compte dans sa review)
   - `non-testable` → signaler dans le compte rendu d'étape (étape 6) comme point d'attention

2. **Intégrer les `### Tests écrits`** dans le prompt envoyé au reviewer à l'étape 4 :
   > Fournir au reviewer : le diff de la branche **incluant les tests QA ajoutés** + la liste des fichiers de test créés

3. **Intégrer les `### Points d'attention pour la review`** dans le prompt envoyé au reviewer à l'étape 4 :
   > Fournir au reviewer : les points d'attention signalés par le qa-engineer (zones non testables, edge cases non couverts, hypothèses, suggestions)

4. **Intégrer les `### Zones non testables identifiées`** dans le compte rendu d'étape (étape 6) comme point d'attention technique.

5. **Si le bloc est absent** → demander explicitement au qa-engineer de le produire avant de continuer.

6. **Si le rapport QA complet est absent** (le bloc handoff est présent sans rapport préalable) → demander explicitement au qa-engineer de produire le rapport complet avant de continuer.

> ❌ Ne jamais passer à la review sans avoir vérifié le statut QA — une couverture `non-testable` doit être signalée.
> ❌ Ne jamais ignorer les critères d'acceptance non couverts — les transmettre au reviewer.
> ❌ Ne jamais accepter un bloc handoff sans rapport QA préalable — les deux sont obligatoires.
