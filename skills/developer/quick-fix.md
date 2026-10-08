---
name: quick-fix
description: Corrections auto-applicables sans review — lint fix, import manquant, typo évidente, formatage. Ces corrections déterministes ne modifient pas la logique métier et peuvent être appliquées immédiatement.
---

# Skill — Corrections Auto-Applicables (Quick Fix)

## Rôle

Ce skill définit les corrections triviales qu'un agent developer peut appliquer
**immédiatement** sans passer par le cycle complet de review.

Ces corrections sont **déterministes** : elles n'impliquent aucun jugement,
aucune décision d'architecture, et aucune modification de la logique métier.

---

## ✅ Corrections éligibles (auto-applicables)

### Exemples représentatifs

```typescript
// Lint fix — let value = 42  →  const value = 42
// Import manquant — ajout de `import { ref } from 'vue'` quand ref() est utilisé
// Typo — 'Invalide user ID' → 'Invalid user ID'
```

### Catégories éligibles

- **Lint fix** — corrections automatiques du linter (prefer-const, no-unused-vars, etc.)
- **Import manquant** — ajout d'un import pour un symbole utilisé mais non importé
- **Typo évidente** — faute de frappe non ambiguë dans identifiant, commentaire ou chaîne
- **Formatage** — indentation, espaces, sauts de ligne conformes au formatter du projet
- **Point-virgule** — ajout/suppression selon la convention configurée (ESLint, Prettier)
- **Trailing comma** — ajout/suppression selon la configuration du projet

---

## ❌ Corrections NON éligibles

Toute correction impliquant un **jugement**, un **choix d'architecture** ou un **changement de comportement** nécessite une review :

- **Renommage de variable** — choix de nommage subjectif, décision de design
- **Refactoring** — extraction de fonction, changement de structure de contrôle
- **Changement de signature** — ajout/suppression de paramètre, changement de type de retour
- **Modification de logique métier** — changement de condition, de valeur par défaut, de validation
- **Changement de dépendance** — remplacement, ajout ou suppression de lib
- **Suppression de code** — même apparemment mort (peut être utilisé dynamiquement)

---

## Conditions d'application

Une correction quick fix peut être appliquée **uniquement si** :

1. **Pas de changement de logique métier** — le comportement observable reste identique
2. **Correction déterministe** — une seule façon correcte de corriger (pas de choix)
3. **Changement local** — impact limité au fichier concerné, pas d'effet de bord
4. **Réversible trivialement** — l'annuler ne demande que de défaire ta propre modification (jamais celles de l'utilisateur)
5. **Conforme aux conventions du projet** — respecte le linter, le formatter configuré

---

## Workflow

Quand tu identifies une correction éligible :

1. **Vérifier l'éligibilité** — la correction entre dans les catégories ✅ ci-dessus
2. **Appliquer directement** — sans demander confirmation
3. **Mentionner dans le compte rendu** — lister les quick fixes appliqués

```markdown
### Quick fixes appliqués
- `src/utils/format.ts` : lint fix (prefer-const, 2 occurrences)
- `src/components/Button.vue` : import manquant (ref)
- `src/services/user.ts` : formatage Prettier
```

---

## Ce que ce skill ne remplace PAS

Ce skill ne dispense pas de la review pour les modifications substantielles.
En cas de doute sur l'éligibilité d'une correction, **ne pas l'appliquer**
et la signaler comme suggestion dans le compte rendu.

```markdown
### Suggestions (non appliquées — nécessitent validation)
- `src/utils/date.ts:42` : variable `d` pourrait être renommée en `formattedDate`
- `src/services/api.ts:15-20` : code dupliqué, extraction possible
```
