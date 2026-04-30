---
name: dev-standards-typescript
description: Standards TypeScript — typage strict, inférence, types utilitaires, gestion des erreurs typées, patterns avancés.
---

# Skill — Standards TypeScript

## Rôle

Ce skill définit les conventions TypeScript à respecter sur les projets qui utilisent
ce langage, que ce soit côté frontend, backend ou en code partagé.
Il complète `dev-standards-universal.md` et s'applique dès que TypeScript est détecté
dans la stack du projet.

---

## Configuration de base

- `strict: true` activé dans `tsconfig.json` — obligatoire
- `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes` inclus dans `strict`
- `noUncheckedIndexedAccess: true` recommandé pour les accès tableaux/objets
- `paths` configurés pour les alias d'import — éviter les chemins relatifs profonds (`../../../`)
- Un seul `tsconfig.json` de base étendu par environnement si nécessaire

---

## Typage

### Règles fondamentales

- Pas de `any` — utiliser `unknown` avec narrowing explicite si le type est réellement indéterminé
- Pas de cast forcé (`as Type`) sans vérification préalable — préférer les type guards
- Pas de `!` (non-null assertion) sauf si la nullité est garantie par le contexte et documentée
- L'inférence de type est utilisée quand elle est évidente et non ambiguë — ne pas re-typer inutilement

### Interfaces vs types

- **Interfaces** pour les contrats publics et les formes d'objets extensibles
- **Types** pour les unions, intersections, types utilitaires et aliases
- Ne pas mélanger les deux pour la même entité

```typescript
// ✅ Interface pour un contrat public
interface UserRepository {
  findById(id: string): Promise<User | null>
  save(user: User): Promise<void>
}

// ✅ Type pour une union
type PaymentStatus = 'pending' | 'completed' | 'failed' | 'refunded'

// ✅ Type pour un utilitaire
type CreateUserDto = Omit<User, 'id' | 'createdAt'>
```

### Enums

- Préférer les `const enum` ou les unions de littéraux pour les valeurs constantes
- Les `enum` classiques sont évités (génèrent du JS à l'exécution, difficiles à tree-shaker)

```typescript
// ✅ Union de littéraux — léger, inférable
type Direction = 'north' | 'south' | 'east' | 'west'

// ✅ Const enum si valeur numérique nécessaire
const enum HttpStatus {
  OK = 200,
  Created = 201,
  NotFound = 404,
}

// ❌ Enum classique — éviter
enum Color { Red, Green, Blue }
```

---

## Types partagés

- Les DTOs, interfaces de domaine et types partagés entre couches sont centralisés
  dans un dossier dédié (`types/`, `shared/`, `domain/`) — pas de duplication entre couches
- Les types de réponse API sont définis une seule fois et importés par les deux couches
- Les types générés (OpenAPI codegen, Prisma, GraphQL) sont importés depuis leur source — jamais recopiés

---

## Gestion des erreurs

- Les erreurs métier sont des classes typées qui étendent `Error`
- Le pattern `Result<T, E>` est recommandé pour les fonctions qui peuvent échouer sans lever d'exception

```typescript
// ✅ Erreur métier typée
class InsufficientStockError extends Error {
  constructor(
    public readonly productId: string,
    public readonly requested: number,
    public readonly available: number,
  ) {
    super(`Stock insuffisant pour le produit ${productId}`)
    this.name = 'InsufficientStockError'
  }
}

// ✅ Pattern Result (optionnel selon la convention projet)
type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E }
```

- Pas de `catch (e: any)` — typer l'erreur ou utiliser `instanceof` pour la narrower
- Les `Promise` non gérées sont interdites — toujours `.catch()` ou `await` dans un `try/catch`

---

## Patterns avancés

### Type guards

```typescript
// ✅ Type guard nommé
function isUser(value: unknown): value is User {
  return (
    typeof value === 'object' &&
    value !== null &&
    'id' in value &&
    'email' in value
  )
}
```

### Types utilitaires courants

| Utilitaire | Usage |
|---|---|
| `Partial<T>` | Tous les champs optionnels (ex: DTO de mise à jour) |
| `Required<T>` | Tous les champs obligatoires |
| `Pick<T, K>` | Sous-ensemble de champs |
| `Omit<T, K>` | Exclure des champs |
| `Readonly<T>` | Immuabilité (value objects, configs) |
| `Record<K, V>` | Dictionnaire typé |
| `ReturnType<T>` | Inférer le type de retour d'une fonction |

### Generics

- Nommer les paramètres génériques de façon significative : `TEntity`, `TResponse`, `TError`
  plutôt que `T`, `U`, `V` seuls dès que le contexte n'est pas trivial
- Contraindre les génériques quand la borne est connue : `<T extends BaseEntity>`

---

## Ce que tu ne fais PAS

- Utiliser `any` pour "faire passer le compilateur"
- Caster avec `as` pour contourner une erreur de type plutôt que de la corriger
- Dupliquer des interfaces entre le frontend et le backend
- Utiliser `@ts-ignore` ou `@ts-expect-error` sans commentaire explicatif
- Ignorer les erreurs TypeScript en mode `skipLibCheck: true` global sans justification
