---
name: dev-standards-nuxtjs
description: Standards Nuxt.js — auto-imports, composables, useFetch/useAsyncData, Nitro, stores Pinia, conventions de fichiers.
---

# Skill — Standards Nuxt.js

## Rôle

Ce skill définit les bonnes pratiques pour le développement avec Nuxt.js (v3+).
Il complète `dev-standards-vuejs.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets côté client — les clés API restent dans les server routes ou les plugins serveur
❌ Jamais d'appel direct à la base de données depuis un composant ou un composable côté client
✅ Toute décision sur la stratégie de rendu (SSR/SSG/SPA/hybride) est soumise à validation explicite

---

## Structure du projet

```
app/                        ← code applicatif (ou src/ selon config)
├── components/             ← composants auto-importés (PascalCase)
├── composables/            ← composables auto-importés (useXxx)
├── layouts/                ← layouts auto-importés
├── pages/                  ← routes basées sur les fichiers
├── plugins/                ← plugins Nuxt (client/server)
├── stores/                 ← stores Pinia
└── utils/                  ← utilitaires auto-importés
server/
├── api/                    ← routes API Nitro (GET, POST, etc.)
├── middleware/             ← middleware serveur
└── utils/                  ← utilitaires serveur (non exposés au client)
public/                     ← assets statiques
nuxt.config.ts
```

---

## Auto-imports

Nuxt auto-importe les composants, composables et utils — ne pas importer manuellement ce qui est dans ces dossiers.

```vue
<!-- ✅ Pas besoin d'importer UserCard ni useUser -->
<script setup lang="ts">
const { user } = useUser()
</script>

<template>
  <UserCard :user="user" />
</template>
```

- Les composants dans `components/` sont auto-importés par nom de fichier
- Les composables dans `composables/` préfixés `use` sont auto-importés
- Les utils dans `utils/` sont auto-importés

---

## Fetch de données

### useAsyncData / useFetch

```vue
<script setup lang="ts">
// ✅ useFetch — wrapper de useAsyncData pour les appels HTTP
const { data: products, status, error } = await useFetch('/api/products', {
  key: 'products-list',
  lazy: false,
})

// ✅ useAsyncData — pour les sources non-HTTP
const { data: user } = await useAsyncData('current-user', () =>
  $fetch(`/api/users/${userId}`)
)
</script>
```

- `useFetch` pour les appels HTTP vers les routes Nitro ou des APIs externes
- `useAsyncData` pour les sources de données non-HTTP
- Toujours définir une `key` unique pour éviter les conflits de cache
- Gérer explicitement `status` (`pending`, `success`, `error`) dans le template

### Serveur uniquement — server/api/

```ts
// server/api/users/[id].get.ts
// ✅ Route Nitro — les secrets restent côté serveur
export default defineEventHandler(async (event) => {
  const id = getRouterParam(event, 'id')
  const user = await db.user.findUnique({ where: { id } })
  if (!user) throw createError({ statusCode: 404, message: 'Utilisateur introuvable' })
  return user
})
```

---

## Composables

- Un composable = une responsabilité
- Nommage `useXxx` systématiquement
- Retour explicite et typé
- Utiliser `useState` de Nuxt pour l'état partagé côté SSR (pas `ref` global)

```ts
// composables/useUser.ts
export function useUser() {
  const user = useState<User | null>('user', () => null)

  async function fetchUser(id: string) {
    user.value = await $fetch(`/api/users/${id}`)
  }

  return { user: readonly(user), fetchUser }
}
```

---

## Stores Pinia

```ts
// stores/cartStore.ts
export const useCartStore = defineStore('cart', () => {
  const items = ref<CartItem[]>([])
  const total = computed(() => items.value.reduce((sum, i) => sum + i.price, 0))

  function addItem(item: CartItem) {
    items.value.push(item)
  }

  return { items: readonly(items), total, addItem }
})
```

- Stores suffixés `Store` : `useCartStore`, `useAuthStore`
- Préférer la syntaxe setup (function) à la syntaxe options
- Ne pas mettre de logique de fetch directement dans le store — passer par des composables

---

## Middleware de navigation

```ts
// middleware/auth.ts
export default defineNuxtRouteMiddleware((to) => {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated.value) {
    return navigateTo('/login')
  }
})
```

- Middleware global : préfixe `global` dans le nom (`middleware/auth.global.ts`)
- Middleware de route : déclaré dans `definePageMeta`

---

## Stratégies de rendu

```ts
// nuxt.config.ts — rendu hybride
export default defineNuxtConfig({
  routeRules: {
    '/':              { prerender: true },     // SSG
    '/blog/**':       { isr: 3600 },           // ISR — revalide toutes les heures
    '/dashboard/**':  { ssr: false },          // SPA — rendu client uniquement
    '/api/**':        { cors: true },
  }
})
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Composants | PascalCase | `UserCard.vue` |
| Composables | camelCase préfixé `use` | `useUserProfile.ts` |
| Stores | camelCase suffixé `Store` | `useCartStore.ts` |
| Pages | kebab-case ou `[param]` | `user-profile.vue`, `[id].vue` |
| Routes API Nitro | `<nom>.<method>.ts` | `users.get.ts`, `users/[id].delete.ts` |
| Variables d'env publiques | Préfixe `NUXT_PUBLIC_` | `NUXT_PUBLIC_API_URL` |
| Variables d'env privées | Préfixe `NUXT_` | `NUXT_DATABASE_URL` |

---

## Ce que tu ne fais PAS

- Exposer des secrets dans `runtimeConfig.public` ou les variables `NUXT_PUBLIC_`
- Appeler la base de données depuis un composable côté client
- Utiliser `ref` global (hors `useState`) pour l'état partagé en SSR — risque de pollution entre requêtes
- Omettre la `key` dans `useFetch`/`useAsyncData`
- Ignorer la gestion des états `pending` et `error` dans le template
