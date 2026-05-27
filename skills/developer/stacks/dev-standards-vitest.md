---
name: dev-standards-vitest
description: Standards Vitest — configuration, mocking (vi.mock, vi.fn, vi.spyOn), matchers, coverage et bonnes pratiques.
---

# Skill — Standards Vitest

## Rôle

Ce skill définit les bonnes pratiques pour les tests avec Vitest.
Il complète `dev-standards-testing.md` et s'applique à tout projet utilisant Vitest
comme framework de test (Vue.js, Nuxt.js, Vite, NestJS avec adapter, etc.).

---

## Configuration

```typescript
// vitest.config.ts
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    globals: true,              // describe, it, expect sans import
    environment: 'node',        // 'jsdom' pour les tests de composants UI
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      thresholds: {
        lines: 80,
        functions: 80,
        branches: 75,
      },
      exclude: ['**/*.d.ts', '**/index.ts', 'src/migrations/**'],
    },
    setupFiles: ['./tests/setup.ts'],
  },
})
```

---

## Structure des tests

```typescript
// ✅ Structure AAA avec describe imbriqués
describe('UserService', () => {
  describe('findOneOrThrow', () => {
    it('retourne l\'utilisateur quand il existe', async () => {
      // Arrange
      const mockUser = { id: '1', email: 'alice@exemple.com', name: 'Alice' }
      vi.mocked(userRepository.findById).mockResolvedValue(mockUser)

      // Act
      const result = await userService.findOneOrThrow('1')

      // Assert
      expect(result).toEqual(mockUser)
    })

    it('lève une erreur quand l\'utilisateur n\'existe pas', async () => {
      vi.mocked(userRepository.findById).mockResolvedValue(null)
      await expect(userService.findOneOrThrow('inexistant')).rejects.toThrow('introuvable')
    })
  })
})
```

---

## Mocking

### vi.mock — module entier

```typescript
// ✅ Mock d'un module au niveau du fichier
vi.mock('../repositories/userRepository', () => ({
  UserRepository: vi.fn().mockImplementation(() => ({
    findById: vi.fn(),
    create: vi.fn(),
    findByEmail: vi.fn(),
  })),
}))
```

### vi.fn — fonction mock

```typescript
// ✅ Mock de fonction avec implémentation
const mockSendEmail = vi.fn().mockResolvedValue({ success: true })

// Vérification des appels
expect(mockSendEmail).toHaveBeenCalledOnce()
expect(mockSendEmail).toHaveBeenCalledWith({
  to: 'alice@exemple.com',
  subject: expect.stringContaining('Bienvenue'),
})

// Reset entre les tests
beforeEach(() => {
  vi.clearAllMocks()  // réinitialise les compteurs d'appels
})
```

### vi.spyOn — espionner sans remplacer

```typescript
// ✅ Espionner une méthode existante
const spy = vi.spyOn(emailService, 'send').mockResolvedValue(undefined)

// Restaurer après le test
afterEach(() => {
  spy.mockRestore()
})
```

### vi.useFakeTimers — contrôle du temps

```typescript
// ✅ Contrôler Date.now() et setTimeout
beforeEach(() => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2024-01-15'))
})

afterEach(() => {
  vi.useRealTimers()
})

it('crée un token avec l\'horodatage correct', () => {
  const token = createToken()
  expect(token.issuedAt).toEqual(new Date('2024-01-15').getTime())
})
```

---

## Matchers utiles

```typescript
// ✅ Matchers courants
expect(result).toBe(42)                           // égalité stricte (===)
expect(result).toEqual({ id: '1', name: 'Alice' }) // égalité profonde
expect(result).toMatchObject({ name: 'Alice' })    // sous-ensemble
expect(array).toContain('item')
expect(array).toHaveLength(3)
expect(fn).toThrow('message')
expect(promise).rejects.toThrow(NotFoundException)
expect(mock).toHaveBeenCalledWith(expect.objectContaining({ email: 'alice@exemple.com' }))
expect(value).toBeNull()
expect(value).toBeDefined()
expect(value).toMatchInlineSnapshot(`"expected string"`)
```

---

## Tests de composants Vue avec Vitest

```typescript
// ✅ Test de composant Vue avec @vue/test-utils
import { mount } from '@vue/test-utils'
import UserCard from './UserCard.vue'

describe('UserCard', () => {
  it('affiche le nom et l\'email', () => {
    const wrapper = mount(UserCard, {
      props: { name: 'Alice', email: 'alice@exemple.com' },
    })
    expect(wrapper.text()).toContain('Alice')
    expect(wrapper.text()).toContain('alice@exemple.com')
  })

  it('émet edit-click au clic du bouton', async () => {
    const wrapper = mount(UserCard, {
      props: { name: 'Alice', email: 'alice@exemple.com' },
    })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('edit-click')).toBeTruthy()
  })
})
```

---

## Commandes CLI

### Watch mode

```bash
# Watch mode — relance automatiquement les tests modifiés
npx vitest

# Watch sur un fichier spécifique
npx vitest src/services/__tests__/user.service.test.ts
```

### Exécution ciblée

```bash
# Tester un seul fichier
npx vitest run src/services/__tests__/user.service.test.ts

# Tester les fichiers liés aux modifications (changed files)
npx vitest related src/services/user.service.ts

# Filtrer par nom de test
npx vitest -t "doit retourner null"
```

### Couverture

```bash
# Rapport de couverture complet
npx vitest run --coverage

# Couverture ciblée sur un fichier
npx vitest run --coverage src/services/user.service.ts
```

> **Conseil :** Activer `--coverage` avant de commit pour s'assurer que les nouvelles fonctions sont testées.

---

## Ce que tu ne fais PAS

- Utiliser `vi.mock` sans `vi.clearAllMocks()` dans `beforeEach` — les mocks persistent entre tests
- Mocker la logique métier que le test est censé vérifier
- Omettre les seuils de couverture dans la config
- Utiliser `vi.fn()` sur des fonctions synchrones critiques sans vérifier les appels
