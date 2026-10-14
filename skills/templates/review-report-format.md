# Template — Format du rapport de review

```
## Review — <nom de la branche ou titre de la PR>

### Walkthrough
| Fichier | Changement | God node | Domaine |
|---------|-----------|----------|---------|
| <fichier modifié> | <résumé 1 ligne> | <god node ou —> | <domaine ou —> |

### Résumé
<1-3 phrases : ce que fait la PR, ton évaluation globale>

### Périmètre et contexte
- **Conventions chargées :** [conventions.md ✓/✗, architecture.md ✓/✗, review-rules.md ✓/✗]
- **Standards appliqués :** [liste des dev-standards-* chargés]
- **God nodes touchés :** [liste ou "aucun"]
> ⚠️ Review sans contexte wiki — findings basés sur standards génériques uniquement. (si applicable)

### Vérification
*(Omettre dans une sous-session `[REVIEW:single]` : la session parente l'a faite.)*

| Commande | Résultat |
|----------|----------|
| <commande lancée> | ✅ passe / ❌ échec (finding 🔴) / ⚠️ non vérifiable : <raison> (finding 🟡) |

- **Arbre de travail après exécution :** propre / fichiers produits non ignorés : <liste> (finding 🟡)

--- BLOC 1 : CORRECTIONS (workflow orchestrator → developer), en deux axes ---

### Axe 1 — Conformité à la demande
**Demande :** <ticket <ID> (`bd show`) | spec | description de la PR | conformité non vérifiable : <raison>>

| Critère | Implémenté | Testé | Fichier(s) |
|---------|-----------|-------|------------|
| <critère 1 de la demande> | ✅/⚠️/❌ | ✅/❌ | `fichier:ligne` ou — |

> Légende : ✅ = vérifié dans le diff, ⚠️ = partiel ou à côté, ❌ = non identifié dans le diff. Sans critères : une ligne par exigence de la demande.

**Ce qui manque :** <findings des ⚠️ et ❌, au format ci-dessous ; ou « rien »>
**Ce qui dépasse :** <findings des ajouts non demandés ; ou « rien hors demande »>

### Axe 2 — Qualité du code

#### 🔴 Critique — bloquant
<Problèmes qui doivent être résolus avant merge>

#### 🟠 Majeur — à corriger
<Problèmes importants mais non-bloquants pour le merge immédiat>

#### 🟡 Mineur — amélioration recommandée
<Petits écarts aux standards, nommage, lisibilité>

--- BLOC 2 : OBSERVATIONS (pour le reviewer humain, non-bloquant) ---

### 💡 Suggestions
<Idées d'amélioration, alternatives, pistes futures — sans pression>

### ✅ Points positifs
<Ce qui est bien fait — toujours inclure si pertinent>

### 🔍 Hors scope
<Problèmes existants détectés mais qui ne concernent pas cette PR>
```

## Exemples de findings

```
**[🟠 Majeur] [5/5]** `src/services/user.service.ts:47` — Gestion d'erreur absente

La méthode `findById` ne gère pas le cas où l'utilisateur n'existe pas.
Si `user` est null, la ligne 52 lancera une erreur non catchée.

Suggestion : ajouter un guard `if (!user) throw new NotFoundException(...)` avant la ligne 52.

> Confiance 5/5 : conventions.md §Error-handling impose un guard sur tout accès nullable.
```

```
**[🟡 Mineur] [3/5]** `src/utils/format.ts:23` — Nommage peu expressif

La variable `d` ne communique pas sa signification. Un nommage comme `formattedDate` serait plus lisible.

> Confiance 3/5 : standard dev-standards-universal §Clean-Code, pas de convention projet spécifique au nommage.
```

```
**[💡 Suggestion] [2/5]** `src/api/routes.ts:89` — Extraction possible en middleware

La logique de validation des headers (lignes 89-102) pourrait être extraite en middleware réutilisable.

> Confiance 2/5 : jugement d'expert, pas de convention sur les middlewares dans ce projet.
```
