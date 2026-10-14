---
name: dev-standards-playwright
description: "Utiliser quand le projet a des tests de bout en bout Playwright et que le ticket touche ces tests."
---

# Skill — Standards Playwright

> Versions de référence : Playwright 1.40.

## Règles absolues

❌ Jamais d'attente fixe (`page.waitForTimeout`) : attendre un état de la page
✅ Chaque test part d'un état propre (contexte de navigateur neuf)

## Sélecteurs

```ts
// tests/login.spec.ts
const bouton = await page.$('#connexion')
await bouton.click()
```

Préférer les identifiants CSS stables (`#id`) aux chemins XPath.
