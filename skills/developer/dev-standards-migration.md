---
name: dev-standards-migration
description: Standards de migration — frameworks, versions, dépendances, DB/ORM, build tools. Stratégies (Strangler Fig, Bridge Pattern), analyse pré-migration, workflow incrémental avec rollback toujours possible.
---

# Skill — Standards de Migration

## Rôle

Tu es un assistant de développement qui applique les standards de migration.
Ce skill définit les types de migrations, les stratégies, l'analyse pré-migration
et le workflow pour migrer du code de manière sûre et incrémentale.

---

## 🔒 Règle absolue — Réversibilité

**Une migration réussie est une migration réversible.**

Chaque étape de migration doit :
- Permettre un rollback rapide si problème détecté
- Être testable indépendamment
- Ne pas bloquer le développement des autres features
- Documenter explicitement les changements de comportement inévitables

❌ Tu ne migres JAMAIS sans plan de rollback documenté
❌ Tu ne migres JAMAIS plusieurs composants majeurs en même temps
❌ Tu ne supprimes JAMAIS le code legacy avant validation complète
❌ Tu ne fais JAMAIS de "big bang migration" — toujours incrémental
✅ Chaque étape doit laisser l'application fonctionnelle

---

## 🛑 Protocole pré-migration destructive

Une migration est **destructive** si elle contient l'un des patterns suivants :
- `DROP TABLE` / `DROP COLUMN` / `DROP INDEX`
- `TRUNCATE`
- `DELETE` sans clause `WHERE` (suppression masse)
- `ALTER TABLE ... DROP ...`
- Renommage de colonne référencée par d'autres services ou migrations

### Workflow OBLIGATOIRE si migration destructive détectée

**Étape 1 — Dry-run** : exécuter la commande de preview de la migration

| ORM / Framework | Commande de preview |
|----------------|---------------------|
| Prisma | `npx prisma migrate diff` |
| Alembic | `alembic history --verbose` + lire le fichier de migration |
| TypeORM | `npx typeorm migration:show -d <datasource>` |
| Django | `python manage.py migrate --plan` |
| Rails | `rails db:migrate:status` + lire le fichier de migration |

**Étape 2 — Documenter** dans le bloc de handoff :

```
⚠️ MIGRATION DESTRUCTIVE DÉTECTÉE
Type : [DROP COLUMN / TRUNCATE / DELETE masse / ...]
Table(s) impactée(s) : [liste]
Données perdues si exécutée : [estimation / "irréversible"]
Réversibilité : [rollback possible via <commande> / non-réversible]
Dry-run output : [résultat de la commande de preview ci-dessus]
```

**Étape 3 — STOP** : marquer le statut `bloqué` et retourner à orchestrator-dev.

```bash
bd update <ID> -s blocked
bd comments add <ID> "Migration destructive détectée — escalade requise avant exécution. Voir handoff."
```

> Le developer-migrator NE PEUT PAS exécuter une migration destructive de manière autonome.
> Seul l'utilisateur peut autoriser l'exécution via un checkpoint explicite (CP-2).

---

## Types de migrations

### Frameworks frontend

| Migration | Outils / Guides |
|-----------|-----------------|
| Vue 2 → Vue 3 | `vue-codemod`, migration guide officiel, `@vue/compat` |
| React 17 → React 18 | Migration guide officiel, `react-codemod` |
| Angular upgrades | `ng update`, Angular Update Guide |
| jQuery → framework moderne | Migration manuelle progressive |

**Stratégie recommandée :** Bridge Pattern avec coexistence temporaire.

### Frameworks backend

| Migration | Points d'attention |
|-----------|-------------------|
| Express → Fastify | Middleware compatibility, plugins |
| Django upgrades | Deprecation warnings, database migrations |
| Rails upgrades | `rails app:update`, gem compatibility |
| Spring Boot upgrades | Configuration changes, dependencies |

**Stratégie recommandée :** Feature flags pour basculer progressivement.

### Versions majeures de runtime

| Migration | Checklist |
|-----------|-----------|
| Node.js (ex: 18 → 20) | Changelog LTS, native modules, npm compatibility |
| Python (ex: 3.9 → 3.12) | `pyupgrade`, syntax changes, typing updates |
| Java (ex: 11 → 17 → 21) | Preview features, removed APIs, module system |

**Stratégie recommandée :** CI parallèle sur les deux versions pendant la transition.

### Dépendances

| Type | Exemples | Approche |
|------|----------|----------|
| Date/time | moment → date-fns, dayjs | Wrapper d'abstraction puis remplacement |
| Utilities | lodash → native ES | Remplacement fonction par fonction |
| HTTP client | axios → fetch, got | Adapter pattern |
| State management | Vuex → Pinia, Redux → Zustand | Coexistence puis migration store par store |

### Bases de données / ORM

| Migration | Complexité | Stratégie |
|-----------|------------|-----------|
| Changement d'ORM (Sequelize → Prisma) | Élevée | Repository pattern, migration progressive |
| Changement de DB (MySQL → PostgreSQL) | Très élevée | Dual-write puis switch |
| Upgrade majeur ORM | Moyenne | Suivre le guide officiel, tester les requêtes |

**Attention :** Ne pas confondre avec les SQL migrations de schéma (scope du domaine backend).

### Build tools

| Migration | Outils |
|-----------|--------|
| Webpack → Vite | `vite-plugin-legacy` pour compatibilité |
| CRA → Next.js / Vite | Migration manuelle, restructuration routes |
| Jest → Vitest | Compatibilité API quasi-totale |
| Babel → SWC / esbuild | Configuration transpilation |

---

## Stratégies de migration

### Strangler Fig Pattern

Remplacer progressivement l'ancien système par le nouveau sans interruption.

```
┌─────────────────────────────────────┐
│           Load Balancer             │
└─────────────┬───────────────────────┘
              │
    ┌─────────┴─────────┐
    │                   │
    ▼                   ▼
┌───────┐         ┌───────────┐
│ Legacy │ ◄────── │  Nouveau  │
│ System │  proxy  │  Système  │
└───────┘         └───────────┘
    │                   │
    └───────┬───────────┘
            │
            ▼
      Migration progressive
      route par route
```

**Quand l'utiliser :**
- Migration de frameworks backend complets
- Remplacement de services entiers
- Changement d'architecture (monolithe → microservices)

### Bridge Pattern

Créer une couche d'abstraction qui permet aux deux implémentations de coexister.

```typescript
// Bridge — interface commune
interface DateFormatter {
  format(date: Date, pattern: string): string
  parse(str: string, pattern: string): Date
}

// Implémentation legacy (moment)
class MomentFormatter implements DateFormatter {
  format(date: Date, pattern: string): string {
    return moment(date).format(pattern)
  }
  // ...
}

// Nouvelle implémentation (date-fns)
class DateFnsFormatter implements DateFormatter {
  format(date: Date, pattern: string): string {
    return formatDateFns(date, pattern)
  }
  // ...
}

// Utilisation — switch via config/feature flag
const formatter: DateFormatter = config.useDateFns
  ? new DateFnsFormatter()
  : new MomentFormatter()
```

**Quand l'utiliser :**
- Migration de dépendances utilitaires
- Changement de librairies avec API différentes
- Besoin de rollback instantané

### Feature Flags

Activer/désactiver la nouvelle implémentation sans déploiement.

```typescript
// Feature flag pour migration
if (featureFlags.isEnabled('use-new-payment-service')) {
  return newPaymentService.process(order)
} else {
  return legacyPaymentService.process(order)
}
```

**Quand l'utiliser :**
- Migration de services critiques (paiement, auth)
- Besoin de rollback sans déploiement
- A/B testing de la nouvelle implémentation

### Migration incrémentale (fichier par fichier)

Migrer progressivement sans couche d'abstraction.

```
Semaine 1: Migrer les utilitaires (helpers, utils)
Semaine 2: Migrer les services non critiques
Semaine 3: Migrer les composants UI simples
Semaine 4: Migrer les composants complexes
Semaine 5: Migrer les services critiques
Semaine 6: Supprimer le code legacy
```

**Quand l'utiliser :**
- Upgrade de version de framework (Vue 2.x → 2.y)
- Migration de syntaxe (Options API → Composition API)
- Quand les deux versions sont compatibles

---

## Analyse pré-migration

### Checklist obligatoire

```
☐ Version source et version cible identifiées
☐ Changelog / migration guide officiel lu intégralement
☐ Breaking changes listés et évalués
☐ Dépendances tierces compatibles avec la cible vérifiées
☐ Couverture de tests suffisante (ou plan pour l'augmenter)
☐ Plan de rollback documenté
☐ Estimation de l'effort (heures/jours)
☐ Risques identifiés et mitigations prévues
```

### Évaluation des risques

| Risque | Impact | Mitigation |
|--------|--------|------------|
| Breaking changes non documentés | Élevé | Tests E2E extensifs, canary deployment |
| Dépendance tierce incompatible | Moyen | Trouver alternative ou forker temporairement |
| Performance dégradée | Moyen | Benchmarks avant/après, monitoring |
| Régression fonctionnelle | Élevé | Tests de caractérisation, feature flags |

### Plan de rollback

Documenter explicitement pour chaque migration :

```markdown
## Plan de rollback — Migration Vue 2 → Vue 3

### Déclencheurs de rollback
- Erreur critique en production non résolue en 30 min
- Régression fonctionnelle bloquante
- Performance dégradée > 20%

### Procédure
1. Désactiver le feature flag `vue3-enabled`
2. Reverter le commit de migration (git revert <hash>)
3. Redéployer la version précédente
4. Notifier l'équipe

### Temps estimé
- Feature flag : 1 min
- Revert + redeploy : 15 min
```

---

## Workflow de migration sûr

### Étape 1 — Préparer

```bash
# Créer une branche dédiée
git checkout -b migrate/vue-2-to-3

# Augmenter la couverture si nécessaire
npm run test:coverage
# Si couverture < 70%, écrire des tests de caractérisation d'abord
```

### Étape 2 — Analyser

```bash
# Lire le guide officiel
# Lister les breaking changes applicables
# Auditer les dépendances

npm outdated
npx npm-check-updates

# Pour Vue spécifiquement
npx vue-migration-helper
```

### Étape 3 — Migrer (incrémental)

```
Pour chaque fichier/module :
  1. Appliquer les codemods automatiques si disponibles
  2. Corriger les erreurs de compilation
  3. Lancer les tests du module
  4. Vérifier manuellement si UI concernée
  5. Committer avec message explicite
  6. Passer au fichier suivant
```

### Étape 4 — Valider

```bash
# Tests complets
npm run test

# Build de production
npm run build

# Tests E2E si disponibles
npm run test:e2e

# Vérifier les performances
npm run bench # si applicable
```

### Étape 5 — Déployer

```
1. Déployer en staging
2. Tests de fumée manuels
3. Monitoring des erreurs (Sentry, etc.)
4. Déployer en production (canary si possible)
5. Surveiller les métriques 24-48h
6. Supprimer le code legacy (après validation)
```

---

## Outils de migration

### Codemods

| Outil | Usage |
|-------|-------|
| `jscodeshift` | Transformations AST JavaScript génériques |
| `vue-codemod` | Migrations Vue.js automatisées |
| `react-codemod` | Migrations React automatisées |
| `ts-migrate` | Migration JavaScript → TypeScript |
| `pyupgrade` | Upgrade syntaxe Python |
| `2to3` | Migration Python 2 → 3 |

### Exemple jscodeshift

```javascript
// Codemod : remplacer moment par date-fns
export default function transformer(file, api) {
  const j = api.jscodeshift
  const root = j(file.source)

  // Remplacer les imports
  root
    .find(j.ImportDeclaration, { source: { value: 'moment' } })
    .replaceWith(
      j.importDeclaration(
        [j.importSpecifier(j.identifier('format'))],
        j.literal('date-fns')
      )
    )

  return root.toSource()
}
```

### Guides de migration officiels

Toujours consulter en premier :
- Framework : documentation officielle + changelog
- Runtime : release notes LTS
- Dépendances : CHANGELOG.md + migration guide si existant

---

## Ce que tu NE fais PAS

### Anti-patterns de migration

| Anti-pattern | Pourquoi c'est problématique |
|--------------|------------------------------|
| Big bang migration | Impossible à debugger, rollback complexe |
| Migration sans tests | Aucune garantie de non-régression |
| Migration + features | Mélange deux types de changements |
| Suppression immédiate du legacy | Pas de fallback possible |
| Ignorer les deprecation warnings | Dette technique accumulée |

### Signaux d'alerte — STOP

- Les tests ne passent plus depuis plus de 1 heure → **rollback, découper**
- Dépendance tierce incompatible non anticipée → **évaluer alternatives ou reporter**
- La migration nécessite des changements de logique métier → **ticket séparé**
- Le scope grossit au fil de la migration → **terminer le scope initial, noter le reste**

---

## 🔎 Mode Auditeur Migration

Déclenchement : `@dev-standards audit migration` ou demande d'audit de dépendances

Quand ce mode est actif :
1. Auditer les dépendances obsolètes (`npm outdated`, `pip list --outdated`)
2. Identifier les deprecation warnings dans le code
3. Lister les migrations prioritaires (sécurité, EOL)
4. Évaluer l'effort de chaque migration
5. Proposer un plan de migration priorisé
6. Ne jamais migrer sans validation explicite
