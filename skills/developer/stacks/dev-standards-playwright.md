---
name: dev-standards-playwright
description: Standards Playwright — configuration, locators sémantiques, fixtures, waits, Page Object Model et bonnes pratiques E2E.
---

# Skill — Standards Playwright

> Versions de référence (octobre 2026) : Playwright 1.64.

## Rôle

Ce skill définit les bonnes pratiques pour les tests E2E avec Playwright.
Il complète `dev-standards-testing.md`.

---

## 🔒 Règles absolues

❌ Jamais d'attentes temporelles fixes (`page.waitForTimeout()`) — utiliser les waits sémantiques
❌ Jamais de `page.locator('.class-name')` si un sélecteur accessible existe
✅ Chaque test est idempotent — nettoyage via `beforeEach`/`afterEach`
✅ Les tests E2E ne testent que les parcours critiques

---

## Configuration

```typescript
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: [['html'], ['list']],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
  ],
  webServer: {
    command: 'npm run start',
    url: 'http://localhost:3000',
    reuseExistingServer: !process.env.CI,
  },
})
```

---

## Locators — sélecteurs sémantiques

```typescript
// ✅ Par rôle ARIA (préféré)
page.getByRole('button', { name: 'Se connecter' })
page.getByRole('textbox', { name: 'Email' })
page.getByRole('heading', { name: 'Tableau de bord' })
page.getByRole('link', { name: 'Accueil' })

// ✅ Par label de formulaire
page.getByLabel('Mot de passe')

// ✅ Par texte visible
page.getByText('Bienvenue, Alice')

// ✅ Par placeholder
page.getByPlaceholder('Rechercher...')

// ✅ Par test-id (dernier recours)
page.getByTestId('user-avatar')

// ❌ Sélecteurs CSS fragiles — à éviter
page.locator('.btn-primary')
page.locator('#form > div:nth-child(2) > input')
```

---

## Waits sémantiques

```typescript
// ✅ Attendre un état visible
await expect(page.getByRole('alert')).toBeVisible()
await expect(page.getByText('Utilisateur créé')).toBeVisible()

// ✅ Attendre qu'un élément disparaisse
await expect(page.getByRole('progressbar')).toBeHidden()

// ✅ Attendre une navigation — l'assertion attend d'elle-même
await page.getByRole('button', { name: 'Se connecter' }).click()
await expect(page).toHaveURL(/\/dashboard/)

// ✅ Attendre une réponse réseau
const responsePromise = page.waitForResponse('**/api/users')
await page.getByRole('button', { name: 'Charger les utilisateurs' }).click()
await responsePromise

// ❌ Attente temporelle — jamais
await page.waitForTimeout(2000)
```

---

## Page Object Model

```typescript
// pages/LoginPage.ts
import { type Page, type Locator } from '@playwright/test'

export class LoginPage {
  readonly emailInput: Locator
  readonly passwordInput: Locator
  readonly submitButton: Locator
  readonly errorMessage: Locator

  constructor(private page: Page) {
    this.emailInput = page.getByLabel('Email')
    this.passwordInput = page.getByLabel('Mot de passe')
    this.submitButton = page.getByRole('button', { name: 'Se connecter' })
    this.errorMessage = page.getByRole('alert')
  }

  async goto() {
    await this.page.goto('/login')
  }

  async login(email: string, password: string) {
    await this.emailInput.fill(email)
    await this.passwordInput.fill(password)
    await this.submitButton.click()
  }
}

// tests/e2e/auth.spec.ts
import { test, expect } from '@playwright/test'
import { LoginPage } from '../pages/LoginPage'

test.describe('Authentification', () => {
  test('connexion réussie redirige vers le dashboard', async ({ page }) => {
    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await loginPage.login('alice@exemple.com', 'SecretPass1')

    await expect(page).toHaveURL('/dashboard')
    await expect(page.getByRole('heading', { name: 'Tableau de bord' })).toBeVisible()
  })

  test('identifiants incorrects affiche un message d\'erreur', async ({ page }) => {
    const loginPage = new LoginPage(page)
    await loginPage.goto()
    await loginPage.login('alice@exemple.com', 'mauvais-mdp')

    await expect(loginPage.errorMessage).toBeVisible()
    await expect(loginPage.errorMessage).toContainText('Identifiants incorrects')
  })
})
```

---

## Fixtures

- Authentification : se connecter une seule fois dans un **setup project** et réutiliser le `storageState` (approche recommandée par Playwright) plutôt que via l'UI dans chaque test

```typescript
// tests/auth.setup.ts — exécuté une fois, avant les projets qui en dépendent
import { test as setup, expect } from '@playwright/test'
import { LoginPage } from '../pages/LoginPage'

const authFile = 'playwright/.auth/user.json'

setup('authenticate', async ({ page }) => {
  const loginPage = new LoginPage(page)
  await loginPage.goto()
  await loginPage.login(process.env.TEST_USER_EMAIL!, process.env.TEST_USER_PASSWORD!)
  await expect(page).toHaveURL(/\/dashboard/)
  await page.context().storageState({ path: authFile })
})

// playwright.config.ts
// projects: [
//   { name: 'setup', testMatch: /.*\.setup\.ts/ },
//   { name: 'chromium', use: { ...devices['Desktop Chrome'], storageState: authFile }, dependencies: ['setup'] },
// ]
```

- `playwright/.auth/` est dans `.gitignore`

---

## Ce que tu ne fais PAS

- Utiliser `page.waitForTimeout()` — waits sémantiques uniquement
- Sélectionner les éléments par classes CSS ou sélecteurs fragiles
- Partager de l'état entre tests — chaque test est indépendant
- Tester des comportements qui relèvent des tests unitaires ou d'intégration
- Omettre les screenshots et traces en CI
