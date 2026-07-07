---
name: dev-standards-testing
description: Stratégie de tests — unitaires, intégration, E2E. Couverture obligatoire des critères d'acceptance, TDD, mocking, checklist systématique, gate de complétion et règles de non-régression.
---

# Skill — Standards de Tests

## Rôle

Tu es un assistant de développement qui applique une stratégie de tests rigoureuse.
Ce skill définit les standards de tests à respecter sur tous les projets :
couverture minimale, organisation, nomenclature et règles de non-régression.

---

## 🔒 Règles absolues

❌ Tu ne livres JAMAIS une fonctionnalité sans tests unitaires sur la logique métier
❌ Tu ne livres JAMAIS une implémentation sans couvrir les critères d'acceptance du ticket par au moins un test chacun
❌ Tu ne supprimes JAMAIS un test existant sans justification explicite de l'utilisateur
❌ Tu n'utilises JAMAIS le typage dynamique non contrôlé dans les types de test pour contourner des erreurs
❌ Tu ne testes JAMAIS l'implémentation interne (détails d'implémentation, appels de méthodes privées) — teste le **comportement observable** : entrées → sorties, états, effets de bord publics
✅ Si une fonctionnalité n'est pas testable telle qu'elle est conçue, tu le signales avant d'implémenter
✅ Chaque test doit échouer pour la bonne raison avant d'être vert (red → green vérifiable)

---

## Pyramide de tests

```
         /──────────\
        /   E2E      \         ← peu, lents, fragiles — réservés aux parcours critiques
       /──────────────\
      /  Intégration   \       ← interactions entre modules, appels API, DB (in-memory)
     /──────────────────\
    /     Unitaires       \    ← logique métier isolée — rapides, nombreux, déterministes
   /────────────────────────\
```

**Répartition cible :**
- 70 % tests unitaires
- 20 % tests d'intégration
- 10 % tests E2E

---

## Tests unitaires

### Quand écrire un test unitaire

- Toute fonction avec logique conditionnelle (if, switch, ternaire)
- Toute transformation de données (mapping, calcul, formatage)
- Toute validation ou parsing d'entrée
- Tout comportement qui doit rester stable à l'avenir

### Structure AAA (Arrange / Act / Assert)

```
// Arrange
[préparer les données et dépendances nécessaires]

// Act
[appeler la fonction ou déclencher le comportement]

// Assert
[vérifier le résultat attendu]
```

### Nommage

- Format : `doit <comportement attendu> quand <condition>`
- En français, en minuscules, descriptif
- Le nom du test est la documentation du comportement — il doit être lisible seul

### Couverture minimale

- Logique métier : **100 %** des branches (y compris les cas d'erreur)
- Composants UI : les comportements visibles (rendu conditionnel, émission d'événements)
- Utilitaires : **100 %**
- Controllers/Routes : couverture par tests d'intégration, pas unitaires

---

## Tests d'intégration

- Tester les interactions réelles entre modules (service + repository, controller + service)
- Utiliser des bases de données in-memory ou des mocks de couche IO
- Chaque appel API exposé doit avoir au moins un test d'intégration couvrant :
  - Le cas nominal (200 OK)
  - Un cas d'erreur métier (400 / 422)
  - Le cas non authentifié si la route est protégée (401 / 403)

---

## Tests E2E

- Réservés aux parcours utilisateurs critiques : inscription, connexion, achat, etc.
- Utiliser l'outil E2E adapté à la stack du projet
- Chaque test E2E doit être **idempotent** : nettoyage en beforeEach/afterEach
- Pas d'attentes temporelles fixes — utiliser les waits sémantiques (attente d'un élément, d'un état)

---

## Mocking

### Ce qu'on mocke

- Les appels réseau (HTTP, bibliothèque cliente)
- Les accès fichier système
- Les dépendances externes (email, SMS, paiement)
- Le temps (`Date.now()`, `new Date()`) quand il influence la logique

### Ce qu'on ne mocke pas

- La logique métier que le test est censé vérifier
- Les transformations de données pures (pas de dépendance externe)
- Les composants UI quand on teste le comportement de rendu

### Syntaxe de mocking

Utiliser l'API de mocking fournie par le framework de test du projet. Le principe général :

```
// Remplacer une dépendance par une implémentation contrôlée
mock('<chemin/dependance>', retourner: { fonction: stub })

// Vérifier les interactions
vérifier que fonction a été appelée avec les bons arguments
```

Les spécificités syntaxiques (vi.mock, jest.mock, unittest.mock, etc.) sont définies dans le skill dédié au framework de test du projet.

---

## Exécution Optimisée des Tests (RTK)

**Pour économiser 60-80% de tokens sur les sorties de tests**, utilise TOUJOURS RTK pour exécuter les commandes de test :

### JavaScript/TypeScript

```bash
rtk jest --coverage          # Au lieu de: jest --coverage
rtk vitest run               # Au lieu de: vitest run
rtk playwright test          # Au lieu de: playwright test
```

### Python

```bash
rtk pytest -v                # Au lieu de: pytest -v
rtk pytest --cov             # Au lieu de: pytest --cov
```

### Go

```bash
rtk go test ./...            # Au lieu de: go test ./...
```

### Ruby/Rails

```bash
rtk rspec spec/              # Au lieu de: rspec spec/
rtk rake test                # Au lieu de: rake test
```

**Pourquoi RTK ?**
- Filtre automatiquement la sortie verbeuse des tests
- Conserve uniquement les erreurs et le résumé
- Économise 60-80% de tokens sur les outputs longs
- Le plugin OpenCode réécrit automatiquement les commandes

**Note :** Tu n'as pas besoin de préfixer manuellement avec `rtk` dans tes commandes — le plugin OpenCode le fait automatiquement. Mais connaître ces commandes aide à déboguer et à comprendre le fonctionnement.

Voir `skills/shared/rtk-usage.md` pour plus de détails.

---

## TDD — Développement piloté par les tests

### Quand appliquer le TDD

- Logique métier complexe (calculs, règles, validations)
- Correction de bugs (le test reproduit le bug avant le fix)
- API publiques (contrat défini avant l'implémentation)
- Tout ticket Beads portant le label **`tdd`**

### Processus Red / Green / Refactor

```
1. Red    — Écrire le(s) test(s) qui échoue(nt) (l'implémentation n'existe pas encore)
2. Green  — Écrire le minimum de code pour faire passer le(s) test(s)
3. Refactor — Améliorer le code sans casser les tests
```

### Workflow TDD en contexte Beads

Quand le ticket porte le label `tdd`, respecter impérativement cet ordre :

```
1. bd show <ID>                   → lire les critères d'acceptance — ils définissent les tests à écrire
2. bd update <ID> --claim         → clamer le ticket
3. [RED]    Écrire les tests qui couvrent les critères d'acceptance → vérifier qu'ils échouent
4. [GREEN]  Implémenter le minimum de code pour faire passer les tests
5. [REFACTOR] Nettoyer le code sans casser les tests
6. bd update <ID> -s review       → passer en review
```

❌ Ne jamais écrire l'implémentation avant que les tests rouges existent
❌ Ne jamais modifier un test pour le faire passer — modifier l'implémentation
❌ Ne jamais supprimer un test rouge "gênant" — s'il échoue, l'implémentation est incomplète
✅ Les tests rouges sont le contrat — l'implémentation les satisfait, pas l'inverse

### Critère de "done" en TDD

- Tous les tests écrits en phase Red sont verts
- Aucun test existant n'a été supprimé ou modifié pour forcer le green
- Le refactor n'a pas introduit de régression (tous les tests passent après refactor)
- Les critères d'acceptance du ticket sont couverts chacun par au moins un test

### Exemple — cycle Red / Green / Refactor

**Red — test écrit en premier (échoue) :**

```typescript
// Red : la fonction calculerRemise n'existe pas encore
it('doit appliquer 10% de remise quand le montant dépasse 100€', () => {
  expect(calculerRemise(150)).toBe(135)
})

it('doit retourner le montant sans remise quand il est inférieur à 100€', () => {
  expect(calculerRemise(80)).toBe(80)
})
```

**Green — implémentation minimale :**

```typescript
// Green : minimum pour faire passer les tests
export function calculerRemise(montant: number): number {
  return montant > 100 ? montant * 0.9 : montant
}
```

**Refactor — amélioration sans casser les tests :**

```typescript
// Refactor : constante nommée, seuil et taux extraits
const SEUIL_REMISE = 100
const TAUX_REMISE = 0.10

export function calculerRemise(montant: number): number {
  return montant > SEUIL_REMISE ? montant * (1 - TAUX_REMISE) : montant
}
// Les tests passent toujours — rien n'a changé du point de vue du comportement
```

### Impact sur la review

Quand le ticket est en TDD, le reviewer vérifie que le TDD a été correctement appliqué :
- Couverture >= 80% et tous les critères d'acceptance couverts
- Les tests ont été écrits avant l'implémentation (cohérence visible dans les commits)
- Si le TDD est incomplet ou mal appliqué → finding de sévérité 🟠 Majeur demandant au developer de compléter les tests

---

## Organisation des fichiers

```
src/
├── services/
│   ├── paiement.service.ts
│   └── __tests__/
│       └── paiement.service.test.ts   ← co-localisé avec la source
├── components/
│   ├── PaiementForm.ts
│   └── __tests__/
│       └── PaiementForm.test.ts
tests/
├── integration/                        ← tests d'intégration multi-modules
│   └── checkout.integration.test.ts
└── e2e/                               ← tests E2E
    └── checkout.e2e.test.ts
```

---

## Tests incrémentaux pendant l'implémentation

### Principe

**Lancer les tests après chaque bloc logique de modifications.**

Ne pas attendre d'avoir terminé toute l'implémentation pour lancer les tests.
Un "bloc logique" correspond à :
- Une fonction ajoutée ou modifiée
- Un cas de bord traité
- Un refactor local terminé

Cette pratique permet de :
- Détecter les régressions immédiatement, quand le contexte est frais
- Éviter l'accumulation d'erreurs difficiles à démêler
- Maintenir une confiance continue dans le code

### Commandes par framework

Les commandes de test (watch mode, couverture, exécution ciblée) sont définies dans le skill
dédié au framework de test du projet :

- **Vitest** : `dev-standards-vitest.md`
- **Jest** : `dev-standards-jest.md`

Principes généraux applicables à tous les frameworks :

| Mode | Usage |
|------|-------|
| **Watch mode** | Relance automatique des tests modifiés — garder actif en arrière-plan |
| **Run ciblé** | Tester un seul fichier après modification — feedback rapide |
| **Related / Changed** | Tester les fichiers impactés par les dernières modifications |
| **Coverage** | Vérifier que les nouvelles fonctions sont couvertes — avant commit |

### Bonnes pratiques

| Situation | Action |
|-----------|--------|
| Début de session de développement | Lancer le watch mode en arrière-plan |
| Ajout d'une fonction | Lancer les tests du fichier concerné |
| Refactor d'un module | Lancer les tests liés (`related` ou `--onlyChanged`) |
| Avant de commit | Lancer la suite complète |

### Intégration avec le workflow TDD

En mode TDD, le watch mode est particulièrement utile :
1. Écrire le test (Red) → le watch détecte et montre l'échec
2. Implémenter (Green) → le watch détecte et montre le succès
3. Refactorer → le watch confirme que rien n'est cassé

Le feedback immédiat du watch mode renforce la boucle Red/Green/Refactor.

---

## Non-régression

- Tout bug corrigé **doit** avoir un test qui le reproduit avant le fix
- Format du test de non-régression :
  ```typescript
  it('doit [comportement] — non-régression #<ID-ticket>', () => { ... })
  ```
- Ce test reste dans le code définitivement

---

## 🔎 Mode Auditeur

Quand l'utilisateur demande un audit, une review ou utilise le mot-clé **"audit tests"** :

1. Lister les fonctions/modules sans tests ou avec couverture insuffisante
2. Identifier les tests qui testent l'implémentation plutôt que le comportement
3. Signaler les mocks qui masquent du code jamais exécuté
4. Vérifier la nomenclature et la structure AAA
5. Proposer un plan de correction priorisé

---

## Checklist systématique de couverture

Pour chaque unité de code implémentée ou modifiée, vérifier dans l'ordre :

### 1. Cas nominal
- [ ] Le chemin principal fonctionne avec des données valides
- [ ] Les valeurs de retour sont correctes

### 2. Cas d'erreur
- [ ] Entrée invalide ou manquante → comportement attendu (exception, valeur par défaut, message d'erreur)
- [ ] Dépendance externe en erreur → comportement dégradé géré

### 3. Edge cases
- [ ] Valeurs limites (0, null, undefined, chaîne vide, tableau vide)
- [ ] Valeurs aux bornes des intervalles
- [ ] Concurrence ou appels multiples si pertinent

### 4. Couverture des critères d'acceptance
- [ ] Chaque critère d'acceptance du ticket a au moins un test associé
- [ ] Les critères négatifs ("ne doit pas faire X") sont couverts

### 5. Qualité des tests
- [ ] Nommage expressif : `doit <faire quoi> quand <contexte>`
- [ ] Structure AAA respectée (Arrange / Act / Assert)
- [ ] Pas d'assertion multiple non liée dans un même test
- [ ] Pas de logique conditionnelle dans les tests

---

## Gate de complétion — Avant de déclarer l'implémentation terminée

Avant de produire le compte rendu ou le bloc de handoff, passer les 3 checks suivants **dans l'ordre** :

### Check 1 — Tests passent

✅ Tous les tests écrits dans cette session passent (green)
✅ Les tests existants du projet ne sont pas cassés (aucune régression introduite)
❌ Si un test est rouge : corriger l'implémentation ou documenter pourquoi dans le compte rendu

### Check 2 — Comportement observable conforme à la spec

✅ Chaque critère d'acceptance du ticket a au moins un test associé (ou une justification documentée)
✅ Les critères négatifs ("ne doit pas faire X") sont couverts
❌ Critère non couvert → signaler dans le compte rendu comme point d'attention

### Check 3 — Aucune régression connue non documentée

✅ Aucun test existant n'a été supprimé ou contourné sans justification
✅ Les tests écrits testent bien le comportement observable, pas les détails d'implémentation
❌ Si une zone non testable est identifiée : signaler dans le compte rendu

**Règle absolue :** les 3 checks doivent être passés ou leur impossibilité explicitement documentée dans le compte rendu.

---

## Outils par type de test

| Type | Outils |
|------|--------|
| Unitaires (TS/JS) | Vitest (préféré), Jest |
| Unitaires (Python) | pytest |
| Unitaires (PHP) | PHPUnit |
| Intégration API (Node.js) | Supertest |
| Intégration API (Python) | pytest + httpx |
| Intégration API (PHP) | Symfony BrowserKit |
| Intégration DB | Base in-memory (SQLite, testcontainers) ou transactions rollbackées |
| Composants Vue.js | Vitest + Vue Test Utils |
| Composants React | Vitest + React Testing Library (ou Jest + RTL) |
| E2E Web | Playwright (préféré), Cypress |
| E2E Mobile | Detox (React Native), XCTest (iOS), Espresso (Android) |
