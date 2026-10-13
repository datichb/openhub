---
name: dev-standards-nextjs
description: "Utiliser quand le projet utilise Next.js (App Router, Server Components/Actions, cache) et que le ticket touche ce code."
---

# Skill — Standards Next.js

> Versions de référence (octobre 2026) : Next.js 16.4, React 19, Zod 4.

## Rôle

Ce skill définit les bonnes pratiques pour le développement avec Next.js (App Router).
Il complète `dev-standards-react.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets côté client — toutes les clés API restent dans les Server Components ou les Route Handlers
❌ Jamais d'appel direct à la base de données depuis un Client Component
✅ Toute décision sur la stratégie de rendu (SSR/SSG/ISR/CSR) est soumise à validation explicite

---

## App Router — Structure

```
app/
├── layout.tsx              ← layout racine (HTML, body, providers)
├── page.tsx                ← page d'accueil
├── loading.tsx             ← skeleton global
├── error.tsx               ← error boundary global
├── not-found.tsx
├── (auth)/                 ← route group (pas dans l'URL)
│   ├── login/page.tsx
│   └── register/page.tsx
├── dashboard/
│   ├── layout.tsx          ← layout imbriqué
│   ├── page.tsx
│   └── [userId]/
│       └── page.tsx        ← route dynamique
└── api/
    └── webhooks/
        └── route.ts        ← Route Handler
```

---

## Server Components vs Client Components

### Server Components (par défaut)

- Utilisés par défaut dans App Router — pas de `'use client'` = Server Component
- Accès direct aux données (DB, API externe) sans exposer les secrets
- Pas d'interactivité, pas de hooks React, pas d'événements navigateur

```tsx
// ✅ Server Component — fetch de données sans exposer de secret
async function UserProfile({ userId }: { userId: string }) {
  const user = await db.user.findUnique({ where: { id: userId } })
  if (!user) notFound()

  return <UserCard name={user.name} email={user.email} />
}
```

### Client Components

- `'use client'` uniquement quand nécessaire : état (`useState`), effets (`useEffect`), événements, APIs navigateur
- Pousser le `'use client'` au plus bas dans l'arbre — les feuilles interactives, pas les layouts
- Passer les données depuis le Server Component via les props

```tsx
// ✅ Client Component minimal — uniquement pour l'interactivité
'use client'

interface EditButtonProps {
  userId: string
}

export function EditButton({ userId }: EditButtonProps) {
  const [isOpen, setIsOpen] = useState(false)
  return (
    <>
      <button onClick={() => setIsOpen(true)}>Modifier</button>
      {isOpen && <EditModal userId={userId} onClose={() => setIsOpen(false)} />}
    </>
  )
}
```

---

## Stratégies de rendu

| Stratégie | Quand l'utiliser |
|---|---|
| **SSR** (dynamique) | Données personnalisées par utilisateur, temps réel |
| **SSG** (statique) | Pages marketing, documentation, blog |
| **ISR** | Données semi-statiques avec revalidation périodique |
| **CSR** | Dashboards interactifs, données personnelles post-auth |

- `params` et `searchParams` sont des **Promise** (depuis Next 15) — toujours `await` (ou `use()` côté client)

```tsx
// ✅ params asynchrone
export default async function BlogPost({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params
  const post = await fetchPost(slug)
  return <Article post={post} />
}
```

---

## Fetch et gestion du cache (Cache Components)

- Depuis Next 15, `fetch` n'est plus mis en cache par défaut — inutile d'ajouter `cache: 'no-store'`
- Next 16 : activer `cacheComponents: true` (défaut des nouveaux projets `create-next-app`) — tout est dynamique par défaut, on choisit explicitement ce qui est mis en cache
- Mettre en cache avec la directive `'use cache'` (fonction, composant ou page), toujours accompagnée d'un `cacheLife`
- Envelopper les lectures non cachées (données par requête, `cookies()`, `headers()`) dans un `<Suspense>`
- Avec Cache Components, les route segment configs `revalidate` / `dynamic` ne s'utilisent plus

```tsx
// ✅ Donnée mise en cache explicitement
import { cacheLife, cacheTag } from 'next/cache'

export async function getProducts() {
  'use cache'
  cacheLife('hours')
  cacheTag('products')
  return db.product.findMany()
}

// ✅ Donnée par requête — streamée dans un Suspense
export default function Page() {
  return (
    <Suspense fallback={<CartSkeleton />}>
      <Cart />
    </Suspense>
  )
}
```

---

## Server Actions

- Utiliser les Server Actions pour les mutations de données (formulaires, boutons)
- Valider les inputs côté serveur avec zod ou une bibliothèque de validation

```tsx
// ✅ Server Action avec validation
'use server'

import { z } from 'zod'

const schema = z.object({
  name: z.string().min(1).max(100),
  email: z.email(),
})

export async function updateUser(formData: FormData) {
  const parsed = schema.safeParse({
    name: formData.get('name'),
    email: formData.get('email'),
  })

  if (!parsed.success) {
    return { error: z.flattenError(parsed.error) }
  }

  await db.user.update({ where: { id: userId }, data: parsed.data })
  revalidatePath('/profile')
}
```

---

## Métadonnées et SEO

```tsx
// ✅ Métadonnées statiques
export const metadata: Metadata = {
  title: 'Mon Application',
  description: 'Description de la page',
}

// ✅ Métadonnées dynamiques
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params
  const product = await fetchProduct(id)
  return { title: product.name, description: product.description }
}
```

---

## Conventions

| Élément | Convention |
|---|---|
| Pages | `page.tsx` dans le dossier de la route |
| Layouts | `layout.tsx` |
| Composants partagés | `components/` à la racine ou dans `_components/` dans le segment |
| Server Actions | `actions/` ou `_actions.ts` co-localisé |
| Route Handlers | `route.ts` dans `app/api/` |
| Proxy (ex-middleware) | `proxy.ts` à la racine, fonction `proxy()` — `middleware.ts` est déprécié depuis Next 16 |
| Variables d'env publiques | Préfixe `NEXT_PUBLIC_` |
| Variables d'env privées | Sans préfixe — côté serveur uniquement |

---

## Ce que tu ne fais PAS

- Exposer des secrets dans les Client Components ou les variables `NEXT_PUBLIC_`
- Appeler la base de données depuis un Client Component
- Ajouter `'use client'` sur des layouts ou des composants sans interactivité
- Ignorer les stratégies de cache — documenter le choix de rendu pour chaque page
