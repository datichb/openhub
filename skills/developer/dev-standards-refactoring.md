---
name: dev-standards-refactoring
description: Standards de refactoring — patterns (Extract, Rename, Move), analyse d'impact, stratégie par petits pas, workflow sûr avec filet de tests. Le comportement observable ne change jamais.
---

# Skill — Standards de Refactoring

## Rôle

Tu es un assistant de développement qui applique les standards de refactoring.
Ce skill définit les patterns, l'analyse d'impact, les stratégies et le workflow
pour refactorer du code de manière sûre et incrémentale.

---

## 🔒 Règle absolue — Invariant comportemental

**Le comportement observable du code ne doit JAMAIS changer après un refactoring.**

Un refactoring modifie la structure interne sans altérer :
- Les entrées/sorties des fonctions publiques
- Les effets de bord visibles (appels API, écritures fichier, logs métier)
- Les contrats d'interface existants

❌ Tu ne refactores JAMAIS du code sans couverture de tests — demander les tests d'abord
❌ Tu ne changes JAMAIS une signature d'API publique dans un refactoring
❌ Tu ne fais JAMAIS de "big bang refactoring" — toujours par petits pas testables
✅ Chaque micro-refactoring doit laisser les tests verts

---

## Patterns de refactoring

### Extract Function / Method

Extraire un bloc de code en une fonction nommée quand :
- Le bloc a une responsabilité distincte et identifiable
- Le nom de la fonction apportera de la clarté
- Le bloc est réutilisable ou testable indépendamment

```
// Avant
function processOrder(order) {
  // validation
  if (!order.items || order.items.length === 0) {
    throw new Error('Order must have items')
  }
  if (!order.customerId) {
    throw new Error('Order must have a customer')
  }
  // ... suite du traitement
}

// Après — Extract Function
function validateOrder(order) {
  if (!order.items || order.items.length === 0) {
    throw new Error('Order must have items')
  }
  if (!order.customerId) {
    throw new Error('Order must have a customer')
  }
}

function processOrder(order) {
  validateOrder(order)
  // ... suite du traitement
}
```

### Extract Class

Extraire une classe quand :
- Un groupe de méthodes et données forment une responsabilité cohérente
- Une classe a plus de 5 responsabilités distinctes
- Des méthodes partagent systématiquement les mêmes paramètres

### Rename

Renommer quand le nom actuel :
- Ne révèle pas l'intention
- Est trompeur ou ambigu
- Utilise des abréviations non évidentes

**Règles de renommage :**
- Utiliser la fonction "Rename Symbol" via LSP (VS Code, Neovim, JetBrains, etc.) pour propager automatiquement
- Vérifier tous les usages avant de renommer manuellement
- Un renommage = un commit atomique

```
// Avant — nom ambigu
const d = new Date()
const calc = (a, b) => a * b * 0.2

// Après — intention révélée
const orderDate = new Date()
const calculateDiscount = (price, quantity) => price * quantity * 0.2
```

### Move

Déplacer une fonction, classe ou module quand :
- Elle est plus proche de ses dépendances ailleurs
- Elle est utilisée uniquement par un autre module
- Le regroupement par cohésion fonctionnelle l'exige

**Processus de Move :**
1. Créer le fichier destination
2. Copier le code (pas couper)
3. Mettre à jour les imports dans le fichier destination
4. Exporter depuis la nouvelle localisation
5. Mettre à jour tous les imports des appelants
6. Vérifier que les tests passent
7. Supprimer l'ancien fichier

### Inline

Inliner une fonction ou variable quand :
- L'abstraction n'apporte pas de clarté
- Le nom n'est pas plus expressif que le corps
- La fonction n'a qu'un seul appelant et n'est pas testée séparément

```
// Avant — abstraction inutile
function isNotEmpty(arr) {
  return arr.length > 0
}
if (isNotEmpty(items)) { ... }

// Après — inline
if (items.length > 0) { ... }
```

### Replace Conditional with Polymorphism

Remplacer des switch/if-else répétés par du polymorphisme quand :
- Le même switch apparaît à plusieurs endroits
- Les cas sont stables et ne changeront pas fréquemment
- Chaque cas a une logique suffisamment complexe

**Attention :** ne pas appliquer ce pattern pour 2-3 cas simples — un if/else reste plus lisible.

### Simplify Conditional

Simplifier les conditions complexes :

```
// Avant — condition imbriquée
if (user) {
  if (user.isActive) {
    if (user.hasPermission('admin')) {
      // action
    }
  }
}

// Après — guard clauses + early return
if (!user) return
if (!user.isActive) return
if (!user.hasPermission('admin')) return
// action
```

### Replace Magic Number/String with Constant

```
// Avant
if (status === 3) { ... }
setTimeout(fn, 86400000)

// Après
const STATUS_COMPLETED = 3
const ONE_DAY_MS = 24 * 60 * 60 * 1000

if (status === STATUS_COMPLETED) { ... }
setTimeout(fn, ONE_DAY_MS)
```

### Replace Loop with Pipeline

Remplacer une boucle impérative par une chaîne fonctionnelle quand :
- La boucle filtre, transforme ou agrège des éléments
- Le code devient plus déclaratif et lisible
- Les opérations sont indépendantes et composables

```javascript
// Avant — boucle impérative
const results = []
for (const item of items) {
  if (item.active) {
    results.push(item.name.toUpperCase())
  }
}

// Après — pipeline fonctionnel
const results = items
  .filter(item => item.active)
  .map(item => item.name.toUpperCase())
```

**Attention :** ne pas appliquer si la boucle a des effets de bord complexes ou si la performance est critique (une seule itération vs plusieurs passes).

---

## Analyse d'impact

### Avant tout refactoring

1. **Identifier le scope** — quels fichiers, fonctions, classes sont concernés ?
2. **Mapper les dépendances** — qui appelle ce code ? qui est appelé par ce code ?
3. **Vérifier la couverture de tests** — le code concerné est-il testé ?
4. **Identifier les points de rupture potentiels** — API publiques, contrats d'interface

### Outils d'analyse

```bash
# Trouver tous les usages d'un symbole (ripgrep — plus rapide que grep)
rg "nomDuSymbole" src/
# Alternative avec grep
grep -r "nomDuSymbole" src/

# Vérifier les imports
rg "from.*fichier" src/

# Afficher les dépendances d'un fichier
madge src/fichier.ts --image deps.png

# Détecter le code mort (TypeScript)
npx knip
```

### Matrice de risque

| Élément modifié | Risque | Vérification requise |
|-----------------|--------|---------------------|
| Fonction privée | Faible | Tests unitaires locaux |
| Fonction exportée | Moyen | Tests unitaires + grep des imports |
| Interface/Type | Élevé | Tous les implémenteurs + appelants |
| API publique | Très élevé | **Hors scope refactoring** — ticket dédié |

---

## Stratégies de refactoring

### Par petits pas (Mikado Method)

1. Identifier l'objectif final
2. Tenter la modification minimale
3. Si les tests cassent → annuler, noter la dépendance, traiter d'abord la dépendance
4. Répéter jusqu'à ce que l'objectif soit atteint

```
Objectif : extraire UserValidator de UserService

1. Créer UserValidator vide → tests verts ✅
2. Copier validateEmail dans UserValidator → tests verts ✅
3. Faire appeler UserService.validateEmail → UserValidator.validateEmail → tests verts ✅
4. Supprimer UserService.validateEmail → tests verts ✅
5. Répéter pour validatePassword, validateUsername...
```

### Avec filet de tests (Characterization Tests)

Si le code n'est pas testé :
1. **Écrire des tests de caractérisation** qui capturent le comportement actuel
2. Ces tests ne définissent pas le comportement correct — ils documentent l'existant
3. Refactorer sous protection de ces tests
4. Les tests de caractérisation peuvent être supprimés après si des tests métier existent

```typescript
// Test de caractérisation — capture l'existant
it('devrait retourner le comportement actuel (caractérisation)', () => {
  // Ce test documente ce que fait le code, pas ce qu'il devrait faire
  expect(legacyFunction(input)).toEqual(outputActuel)
})
```

### Strangler Fig Pattern (pour les gros refactorings)

Pour remplacer un module entier progressivement :

1. Créer le nouveau module à côté de l'ancien
2. Router progressivement les nouveaux appels vers le nouveau module
3. Migrer les anciens appels un par un
4. Supprimer l'ancien module quand il n'est plus appelé

---

## Workflow de refactoring sûr

### Pré-conditions obligatoires

```
☐ Les tests existants passent (baseline verte)
☐ Le code à refactorer est couvert par des tests
☐ L'analyse d'impact est faite (dépendances identifiées)
☐ Le scope est clairement délimité (pas de scope creep)
```

### Cycle de refactoring

```
1. VERT    — S'assurer que tous les tests passent
2. CHANGE  — Appliquer UN micro-refactoring
3. VERT    — Relancer les tests immédiatement
4. COMMIT  — Committer si les tests passent
5. REPEAT  — Retour à l'étape 2 jusqu'à objectif atteint
```

**Règle des 2 minutes :** si un micro-refactoring prend plus de 2 minutes sans tests verts,
c'est qu'il n'est pas assez micro. Annuler et découper plus finement.

### Post-conditions

```
☐ Tous les tests passent (même résultat qu'avant)
☐ Aucune signature publique n'a changé
☐ Le code est plus lisible / maintenable qu'avant
☐ Chaque étape a été commitée séparément
```

---

## Ce que tu NE fais PAS

### Anti-patterns de refactoring

| Anti-pattern | Pourquoi c'est problématique |
|--------------|------------------------------|
| Big bang refactoring | Impossible à debugger si quelque chose casse |
| Refactoring sans tests | Aucune garantie de non-régression |
| Refactoring + feature | Mélange deux types de changements — impossible à reviewer |
| Refactoring "au passage" | Scope creep — pollue l'historique git |
| Over-refactoring | Abstractions prématurées déguisées en "nettoyage" |

### Signaux d'alerte — STOP

- Les tests ne passent plus depuis plus de 5 minutes → **annuler, découper**
- Le refactoring nécessite de changer une API publique → **ticket séparé**
- Le scope grossit au fil du refactoring → **noter pour plus tard, terminer le scope initial**
- Pas de tests sur le code concerné → **écrire les tests d'abord ou abandonner**

---

## 🔎 Mode Auditeur Refactoring

Déclenchement : `@dev-standards audit refactoring` ou demande d'audit de code

Quand ce mode est actif :
1. Identifier les code smells (fonctions longues, classes god, duplication)
2. Proposer les patterns de refactoring applicables
3. Évaluer le risque de chaque refactoring proposé
4. Prioriser par ratio bénéfice/risque
5. Ne jamais appliquer sans validation explicite
