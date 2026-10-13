---
name: dev-standards-react-native
description: "Utiliser quand le projet utilise React Native (navigation, état, performance, sécurité) et que le ticket touche ce code."
---

# Skill — Standards React Native

> Versions de référence (octobre 2026) : React Native 0.87, Expo SDK 57, React 19.

## Rôle

Ce skill définit les bonnes pratiques pour le développement mobile avec React Native.
Il complète `dev-standards-universal.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets, tokens ou clés API dans le code de l'application
❌ Jamais de données sensibles dans AsyncStorage non chiffré
❌ Jamais de contournement des mécanismes de sécurité de la plateforme
✅ Les tokens d'authentification sont stockés via `expo-secure-store` (Expo) ou `react-native-keychain`
✅ Toute communication réseau passe par HTTPS

---

## Architecture

- Utiliser **Expo** pour les nouveaux projets (prebuild / development builds si accès natif avancé nécessaire)
- Séparation stricte : logique métier dans des hooks/services, UI dans les composants
- Organisation par feature :

```
src/
├── features/
│   └── auth/
│       ├── components/    ← composants UI spécifiques à la feature
│       ├── hooks/         ← logique métier (useAuthState, useLogin)
│       ├── services/      ← appels API, transformations
│       └── screens/       ← écrans (AuthScreen, LoginScreen)
├── navigation/            ← navigateurs globaux
├── store/                 ← stores partagés
└── shared/                ← composants, hooks et utils partagés
```

- Navigation : **React Navigation** (stack, tab, drawer selon le besoin)
- État global : **Zustand** (léger) ou **Redux Toolkit** (complexe) — décision à valider avec l'utilisateur
- Cache des requêtes : **TanStack Query** (`useQuery`, `useMutation`)

---

## Composants

- Composants fonctionnels avec hooks uniquement — pas de class components
- Props typées avec TypeScript (`interface Props`)
- `StyleSheet.create()` pour les styles — pas de styles inline dans le JSX (sauf valeurs dynamiques)
- Les composants de présentation ne font pas d'appels réseau directement
- Accessibilité : `accessibilityRole`, `accessibilityLabel` sur tous les éléments interactifs

```tsx
// ✅ Composant typé, styles extraits, accessibilité présente
interface UserCardProps {
  name: string
  avatarUrl: string
  onPress: () => void
}

export function UserCard({ name, avatarUrl, onPress }: UserCardProps) {
  return (
    <Pressable
      style={styles.container}
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={`Voir le profil de ${name}`}
    >
      <Image
        source={{ uri: avatarUrl }}
        style={styles.avatar}
        accessibilityLabel={`Avatar de ${name}`}
      />
      <Text style={styles.name}>{name}</Text>
    </Pressable>
  )
}

const styles = StyleSheet.create({
  container: { flexDirection: 'row', alignItems: 'center', padding: 12 },
  avatar: { width: 40, height: 40, borderRadius: 20 },
  name: { marginLeft: 8, fontSize: 16 },
})
```

---

## Performance

- Utiliser `FlatList` ou `FlashList` pour les listes longues — jamais `ScrollView` + `map`
- Activer **React Compiler** plutôt que `React.memo()`, `useCallback` et `useMemo` manuels — ceux-ci uniquement si un problème de perf est mesuré
- Éviter les fonctions anonymes dans les props de composants de liste (crée de nouveaux refs)
- Hermes et la New Architecture sont activés d'office (New Architecture seule possible depuis 0.82) — vérifier la compatibilité des bibliothèques natives avant adoption
- Lazy loading des écrans avec `React.lazy` ou le lazy loading intégré de React Navigation

---

## Sécurité

- Secrets API stockés côté serveur — l'app ne contient jamais de clés privées
- `react-native-keychain` pour les tokens d'authentification
- Désactiver les logs en production (`__DEV__` guard)
- Certificate pinning via `react-native-ssl-pinning` pour les apps critiques
- Valider les deep links entrants (source, paramètres) avant traitement

---

## Tests

- Tests unitaires des hooks : `renderHook` depuis `@testing-library/react-native`
- Tests de composants : `@testing-library/react-native` (interactions, rendu conditionnel)
- Tests E2E : **Detox** ou **Maestro**
- Mocker les modules natifs explicitement dans `jest.setup.ts`

---

## Conventions de nommage

| Élément | Convention | Exemple |
|---|---|---|
| Composants | PascalCase | `UserProfileScreen.tsx` |
| Hooks | camelCase préfixé `use` | `useAuthState.ts` |
| Screens | suffixe `Screen` | `HomeScreen.tsx` |
| Navigateurs | suffixe `Navigator` | `MainTabNavigator.tsx` |
| Stores Zustand | camelCase suffixé `Store` | `authStore.ts` |
| Services | suffixe `Service` | `userService.ts` |

---

## Ce que tu ne fais PAS

- Publier sur l'App Store sans validation humaine
- Stocker des données sensibles dans AsyncStorage non chiffré
- Utiliser des class components
- Appeler l'API directement depuis un composant UI — passer par un hook ou un service
- Utiliser des APIs React Native dépréciées sans justification documentée
