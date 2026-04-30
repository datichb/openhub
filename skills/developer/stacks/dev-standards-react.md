---
name: dev-standards-react
description: Standards React — hooks, composition, gestion d'état (Context/Zustand/Redux), performance, accessibilité et conventions.
---

# Skill — Standards React

## Rôle

Ce skill définit les bonnes pratiques pour le développement frontend avec React.
Il complète `dev-standards-universal.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Pas de logique métier dans les composants UI — extraire dans des hooks ou des services
❌ Pas de class components sur les nouveaux développements
✅ Toute décision sur la structure de l'état global (Context, Zustand, Redux) est soumise à validation explicite

---

## Composants

- Composants fonctionnels avec hooks uniquement — pas de class components
- Un composant = une responsabilité (présentation ou logique, pas les deux)
- Props typées avec TypeScript (`interface Props` ou `type Props`)
- Les composants de présentation ne font pas d'appels réseau — passer par des hooks

```tsx
// ✅ Composant de présentation typé, sans logique
interface UserCardProps {
  name: string
  email: string
  onEditClick: () => void
}

export function UserCard({ name, email, onEditClick }: UserCardProps) {
  return (
    <article>
      <h3>{name}</h3>
      <p>{email}</p>
      <button onClick={onEditClick} aria-label={`Modifier ${name}`}>
        Modifier
      </button>
    </article>
  )
}
```

---

## Hooks

### Règles des hooks

- Appeler les hooks uniquement au niveau supérieur — jamais dans des conditions ou des boucles
- Préfixer les hooks custom avec `use` : `useUserProfile`, `useCartItems`
- Un hook custom = une responsabilité

### Hooks courants — bonnes pratiques

```tsx
// ✅ useEffect avec cleanup et dépendances explicites
useEffect(() => {
  const controller = new AbortController()

  fetchUser(userId, { signal: controller.signal })
    .then(setUser)
    .catch((err) => {
      if (err.name !== 'AbortError') setError(err)
    })

  return () => controller.abort()
}, [userId])  // dépendance explicite

// ✅ useMemo uniquement si calcul mesurément coûteux
const sortedItems = useMemo(
  () => items.slice().sort((a, b) => a.name.localeCompare(b.name)),
  [items]
)

// ✅ useCallback pour les fonctions passées à des composants mémoïsés
const handleSubmit = useCallback((data: FormData) => {
  onSubmit(data)
}, [onSubmit])
```

- `useMemo` et `useCallback` uniquement si un problème de performance est mesuré
- `useEffect` avec des dépendances explicites et exhaustives — pas de tableau vide `[]` sauf au montage justifié

---

## Gestion d'état

| Scope | Solution recommandée |
|---|---|
| État local UI (champ, toggle) | `useState` |
| État partagé simple (thème, auth) | React Context |
| État global complexe | Zustand ou Redux Toolkit |
| État serveur (cache, fetch) | TanStack Query (`useQuery`, `useMutation`) |

- Pas de prop drilling au-delà de 2 niveaux — utiliser Context ou un store
- L'état serveur n'est pas dupliqué dans un store global — TanStack Query est la source de vérité

---

## Performance

- `React.memo()` sur les composants de présentation qui reçoivent les mêmes props souvent
- `useCallback` et `useMemo` uniquement après mesure — ne pas sur-optimiser
- `React.lazy` + `Suspense` pour le code splitting des routes ou composants lourds
- Éviter les re-renders inutiles : déplacer l'état au plus bas dans l'arbre

```tsx
// ✅ Code splitting d'une route
const UserDashboard = React.lazy(() => import('./pages/UserDashboard'))

function App() {
  return (
    <Suspense fallback={<PageLoader />}>
      <UserDashboard />
    </Suspense>
  )
}
```

---

## Accessibilité

- Éléments interactifs : `<button>` pour les actions, `<a>` pour la navigation
- `aria-label` sur les éléments sans texte visible
- Gestion du focus après les actions (modals, navigation)
- Les formulaires ont des `<label>` liés à leurs inputs

---

## Tests

- Tests de composants avec **React Testing Library** — tester le comportement, pas l'implémentation
- Requêter les éléments par rôle et accessibilité (`getByRole`, `getByLabelText`)
- Éviter `getByTestId` sauf si aucune alternative sémantique

```tsx
// ✅ Test par comportement et rôle
it('appelle onSubmit avec les données du formulaire', async () => {
  const onSubmit = vi.fn()
  render(<LoginForm onSubmit={onSubmit} />)

  await userEvent.type(screen.getByLabelText('Email'), 'alice@exemple.com')
  await userEvent.type(screen.getByLabelText('Mot de passe'), 'secret')
  await userEvent.click(screen.getByRole('button', { name: 'Se connecter' }))

  expect(onSubmit).toHaveBeenCalledWith({
    email: 'alice@exemple.com',
    password: 'secret',
  })
})
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Composants | PascalCase | `UserCard.tsx` |
| Hooks custom | camelCase préfixé `use` | `useAuthState.ts` |
| Context | suffixe `Context` + `Provider` | `AuthContext.tsx` |
| Pages | suffixe `Page` | `UserProfilePage.tsx` |
| Props | camelCase | `onEditClick`, `isLoading` |

---

## Ce que tu ne fais PAS

- Utiliser des class components
- Mettre de la logique métier dans les composants UI
- Dupliquer l'état serveur dans un store global quand TanStack Query est disponible
- Utiliser `any` dans les types de props
- Omettre les dépendances dans `useEffect` sans justification
