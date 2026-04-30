---
name: dev-standards-nextjs
description: Standards Next.js — App Router, Server Components, Server Actions, rendering strategies, gestion du cache, conventions de fichiers.
---

# Skill — Standards Next.js

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

```tsx
// ✅ ISR — revalidation toutes les heures
export const revalidate = 3600

async function BlogPost({ params }: { params: { slug: string } }) {
  const post = await fetchPost(params.slug)
  return <Article post={post} />
}

// ✅ Forcer le rendu dynamique
export const dynamic = 'force-dynamic'
```

---

## Fetch et gestion du cache

- Utiliser le `fetch` étendu de Next.js avec les options de cache dans les Server Components
- `cache: 'no-store'` pour les données dynamiques par requête
- `next: { revalidate: N }` pour l'ISR

```tsx
// ✅ Fetch avec stratégie de cache explicite
const data = await fetch('https://api.exemple.com/products', {
  next: { revalidate: 3600 },  // ISR — revalide toutes les heures
})

// ✅ Fetch dynamique
const data = await fetch('https://api.exemple.com/cart', {
  cache: 'no-store',
})
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
  email: z.string().email(),
})

export async function updateUser(formData: FormData) {
  const parsed = schema.safeParse({
    name: formData.get('name'),
    email: formData.get('email'),
  })

  if (!parsed.success) {
    return { error: parsed.error.flatten() }
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
  const product = await fetchProduct(params.id)
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
| Variables d'env publiques | Préfixe `NEXT_PUBLIC_` |
| Variables d'env privées | Sans préfixe — côté serveur uniquement |

---

## Ce que tu ne fais PAS

- Exposer des secrets dans les Client Components ou les variables `NEXT_PUBLIC_`
- Appeler la base de données depuis un Client Component
- Ajouter `'use client'` sur des layouts ou des composants sans interactivité
- Ignorer les stratégies de cache — documenter le choix de rendu pour chaque page
