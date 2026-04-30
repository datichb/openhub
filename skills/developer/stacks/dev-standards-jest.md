---
name: dev-standards-jest
description: Standards Jest — configuration, mocking (jest.mock, jest.fn, jest.spyOn), matchers, coverage et bonnes pratiques.
---

# Skill — Standards Jest

## Rôle

Ce skill définit les bonnes pratiques pour les tests avec Jest.
Il complète `dev-standards-testing.md` et s'applique aux projets utilisant Jest
(React, NestJS, Node.js, etc.).

---

## Configuration

```javascript
// jest.config.ts
import type { Config } from 'jest'

const config: Config = {
  preset: 'ts-jest',
  testEnvironment: 'node',   // 'jsdom' pour les tests UI
  roots: ['<rootDir>/src'],
  testMatch: ['**/__tests__/**/*.test.ts', '**/*.spec.ts'],
  setupFilesAfterFramework: ['<rootDir>/jest.setup.ts'],
  coverageThreshold: {
    global: {
      lines: 80,
      functions: 80,
      branches: 75,
    },
  },
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/src/$1',  // alias de chemins
  },
}

export default config
```

---

## Structure des tests

```typescript
// ✅ Structure AAA avec describe imbriqués
describe('UserService', () => {
  let service: UserService
  let mockRepository: jest.Mocked<UserRepository>

  beforeEach(() => {
    mockRepository = {
      findById: jest.fn(),
      create: jest.fn(),
      findByEmail: jest.fn(),
    } as jest.Mocked<UserRepository>

    service = new UserService(mockRepository)
  })

  afterEach(() => {
    jest.clearAllMocks()
  })

  describe('findOneOrThrow', () => {
    it('retourne l\'utilisateur quand il existe', async () => {
      const mockUser = { id: '1', email: 'alice@exemple.com', name: 'Alice' }
      mockRepository.findById.mockResolvedValue(mockUser)

      const result = await service.findOneOrThrow('1')

      expect(result).toEqual(mockUser)
      expect(mockRepository.findById).toHaveBeenCalledWith('1')
    })

    it('lève une erreur quand l\'utilisateur n\'existe pas', async () => {
      mockRepository.findById.mockResolvedValue(null)
      await expect(service.findOneOrThrow('inexistant')).rejects.toThrow()
    })
  })
})
```

---

## Mocking

### jest.mock — module entier

```typescript
// ✅ Mock d'un module (hoisted automatiquement en haut du fichier)
jest.mock('../services/emailService', () => ({
  EmailService: jest.fn().mockImplementation(() => ({
    send: jest.fn().mockResolvedValue({ success: true }),
  })),
}))
```

### jest.fn — fonction mock

```typescript
const mockSendEmail = jest.fn().mockResolvedValue({ success: true })

// Retourner des valeurs différentes à chaque appel
const mockFetch = jest.fn()
  .mockResolvedValueOnce({ status: 200, data: user })
  .mockRejectedValueOnce(new Error('Réseau indisponible'))

// Vérification
expect(mockSendEmail).toHaveBeenCalledTimes(1)
expect(mockSendEmail).toHaveBeenCalledWith(
  expect.objectContaining({ to: 'alice@exemple.com' })
)
```

### jest.spyOn — espionner sans remplacer

```typescript
// ✅ Espionner et overrider temporairement
const spy = jest.spyOn(console, 'error').mockImplementation(() => {})

// Restaurer après le test
afterEach(() => {
  spy.mockRestore()
})
```

### jest.useFakeTimers — contrôle du temps

```typescript
beforeEach(() => {
  jest.useFakeTimers()
  jest.setSystemTime(new Date('2024-01-15'))
})

afterEach(() => {
  jest.useRealTimers()
})

it('expire le token après 1 heure', () => {
  const token = createToken()
  jest.advanceTimersByTime(3600 * 1000)
  expect(isExpired(token)).toBe(true)
})
```

---

## Matchers utiles

```typescript
expect(result).toBe(42)
expect(result).toEqual({ id: '1', name: 'Alice' })
expect(result).toMatchObject({ name: 'Alice' })
expect(array).toContain('item')
expect(array).toHaveLength(3)
expect(fn).toThrow('message d\'erreur')
expect(promise).rejects.toThrow(NotFoundException)
expect(mock).toHaveBeenCalledWith(expect.objectContaining({ email: 'alice@exemple.com' }))
expect(value).toBeNull()
expect(value).toBeDefined()
expect(result).toMatchSnapshot()          // snapshot test
```

---

## Tests de composants React avec Jest + RTL

```typescript
// ✅ Test par comportement avec React Testing Library
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LoginForm } from './LoginForm'

describe('LoginForm', () => {
  it('appelle onSubmit avec les valeurs du formulaire', async () => {
    const onSubmit = jest.fn()
    render(<LoginForm onSubmit={onSubmit} />)

    await userEvent.type(screen.getByLabelText('Email'), 'alice@exemple.com')
    await userEvent.type(screen.getByLabelText('Mot de passe'), 'secret')
    await userEvent.click(screen.getByRole('button', { name: 'Se connecter' }))

    expect(onSubmit).toHaveBeenCalledWith({
      email: 'alice@exemple.com',
      password: 'secret',
    })
  })
})
```

---

## Ce que tu ne fais PAS

- Omettre `jest.clearAllMocks()` dans `afterEach` — les mocks persistent entre tests
- Utiliser `jest.mock` avec des factories qui référencent des variables locales (hoisting)
- Mocker la logique métier que le test est censé vérifier
- Utiliser `getByTestId` quand un sélecteur sémantique existe
- Écrire des snapshot tests sans comprendre la structure snapshotée
