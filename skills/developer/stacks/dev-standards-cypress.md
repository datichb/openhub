---
name: dev-standards-cypress
description: Standards Cypress — configuration, commandes custom, cy.intercept, sélecteurs, fixtures et bonnes pratiques E2E.
---

# Skill — Standards Cypress

## Rôle

Ce skill définit les bonnes pratiques pour les tests E2E avec Cypress.
Il complète `dev-standards-testing.md`.

---

## 🔒 Règles absolues

❌ Jamais de `cy.wait(N)` avec un délai fixe — utiliser les waits sémantiques (`cy.intercept`, alias)
❌ Jamais de sélecteurs CSS fragiles — utiliser `data-cy` ou les sélecteurs accessibles
✅ Chaque test est idempotent — état réinitialisé via `beforeEach`
✅ Les appels réseau critiques sont interceptés avec `cy.intercept`

---

## Configuration

```javascript
// cypress.config.ts
import { defineConfig } from 'cypress'

export default defineConfig({
  e2e: {
    baseUrl: 'http://localhost:3000',
    specPattern: 'cypress/e2e/**/*.cy.ts',
    supportFile: 'cypress/support/e2e.ts',
    video: false,
    screenshotOnRunFailure: true,
    retries: { runMode: 2, openMode: 0 },
    env: {
      apiUrl: 'http://localhost:3001',
    },
  },
})
```

---

## Sélecteurs

```typescript
// ✅ data-cy — attribut dédié aux tests, stable et sémantique
cy.get('[data-cy="login-button"]').click()
cy.get('[data-cy="email-input"]').type('alice@exemple.com')

// ✅ Attributs d'accessibilité quand disponibles
cy.get('[aria-label="Fermer la modal"]').click()
cy.contains('button', 'Se connecter').click()

// ❌ Sélecteurs CSS fragiles
cy.get('.btn-primary')
cy.get('#root > div > form > button:last-child')
```

---

## cy.intercept — interception réseau

```typescript
// ✅ Intercepter et aliaser pour attendre la réponse
cy.intercept('POST', '/api/auth/login').as('loginRequest')
cy.intercept('GET', '/api/users').as('getUsers')

cy.get('[data-cy="login-button"]').click()

// Attendre la réponse plutôt qu'un délai fixe
cy.wait('@loginRequest').then((interception) => {
  expect(interception.response?.statusCode).to.eq(200)
})

// ✅ Mocker une réponse
cy.intercept('GET', '/api/users', {
  statusCode: 200,
  body: [{ id: '1', name: 'Alice', email: 'alice@exemple.com' }],
}).as('getUsers')

// ✅ Simuler une erreur serveur
cy.intercept('POST', '/api/orders', { statusCode: 500, body: { error: 'Erreur serveur' } }).as('failedOrder')
```

---

## Commandes custom

```typescript
// cypress/support/commands.ts
declare global {
  namespace Cypress {
    interface Chainable {
      login(email: string, password: string): Chainable<void>
      dataCy(value: string): Chainable<JQuery<HTMLElement>>
    }
  }
}

// ✅ Commande de connexion réutilisable
Cypress.Commands.add('login', (email: string, password: string) => {
  cy.session([email, password], () => {
    cy.visit('/login')
    cy.get('[data-cy="email-input"]').type(email)
    cy.get('[data-cy="password-input"]').type(password)
    cy.get('[data-cy="login-button"]').click()
    cy.url().should('include', '/dashboard')
  })
})

// ✅ Shorthand pour data-cy
Cypress.Commands.add('dataCy', (value: string) => {
  return cy.get(`[data-cy="${value}"]`)
})
```

---

## Fixtures

```typescript
// cypress/fixtures/users.json
// {
//   "alice": { "id": "1", "name": "Alice", "email": "alice@exemple.com" }
// }

cy.fixture('users').then((users) => {
  cy.intercept('GET', '/api/users/1', users.alice).as('getUser')
})
```

---

## Structure des tests

```typescript
// cypress/e2e/auth/login.cy.ts
describe('Authentification — connexion', () => {
  beforeEach(() => {
    cy.visit('/login')
  })

  it('connexion réussie redirige vers le dashboard', () => {
    cy.intercept('POST', '/api/auth/login', { statusCode: 200, body: { token: 'abc' } }).as('login')

    cy.dataCy('email-input').type('alice@exemple.com')
    cy.dataCy('password-input').type('SecretPass1')
    cy.dataCy('login-button').click()

    cy.wait('@login')
    cy.url().should('include', '/dashboard')
    cy.dataCy('welcome-message').should('contain', 'Bienvenue')
  })

  it('affiche une erreur avec des identifiants incorrects', () => {
    cy.intercept('POST', '/api/auth/login', { statusCode: 401, body: { error: 'Identifiants incorrects' } }).as('failedLogin')

    cy.dataCy('email-input').type('alice@exemple.com')
    cy.dataCy('password-input').type('mauvais-mdp')
    cy.dataCy('login-button').click()

    cy.wait('@failedLogin')
    cy.dataCy('error-message').should('be.visible').and('contain', 'Identifiants incorrects')
  })
})
```

---

## Ce que tu ne fais PAS

- Utiliser `cy.wait(N)` avec un nombre fixe de millisecondes
- Sélectionner les éléments par classes CSS ou IDs non stables
- Faire des tests qui dépendent de l'ordre d'exécution
- Ignorer les intercepteurs réseau pour les opérations asynchrones
- Dupliquer la logique de connexion dans chaque test — utiliser `cy.login()` custom
