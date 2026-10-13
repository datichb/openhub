---
name: dev-standards-flutter
description: "Utiliser quand le projet utilise Flutter (widgets, BLoC/Riverpod, performance, Dart) et que le ticket touche ce code."
versions:
  flutter: "3.47"
  dart: "3.13"
  flutter-bloc: "9"
  riverpod: "3"
reviewed: 2026-10-12
---

# Skill — Standards Flutter

> Versions de référence (octobre 2026) : Flutter 3.47 (Dart 3.13), flutter_bloc 9, Riverpod 3.

## Rôle

Ce skill définit les bonnes pratiques pour le développement mobile avec Flutter et Dart.
Il complète `dev-standards-universal.md`.

---

## 🔒 Règles absolues

❌ Jamais de données sensibles dans `SharedPreferences`
❌ Jamais de secrets ou tokens dans le code de l'application
❌ Jamais de contournement des mécanismes de sécurité de la plateforme
✅ Les données sensibles utilisent `flutter_secure_storage`
✅ Toute communication réseau passe par HTTPS

---

## Architecture

- Pattern recommandé : **BLoC** pour les apps complexes, **Riverpod** ou **Provider** pour les apps plus simples — décision à valider avec l'utilisateur
- Séparation par feature avec Clean Architecture :

```
lib/
├── features/
│   └── auth/
│       ├── data/          ← repositories, data sources, DTOs
│       ├── domain/        ← entités, use cases, interfaces repository
│       └── presentation/  ← widgets, blocs/cubits, pages
├── core/                  ← thème, routing, injection, utils partagés
└── main.dart
```

- Utiliser `freezed` + `json_serializable` pour les modèles immuables et la sérialisation

---

## Widgets

- Préférer `StatelessWidget` — n'utiliser `StatefulWidget` que si l'état est vraiment local à l'UI
- Extraire les widgets dans des classes dédiées — pas de méthodes `_buildXxx()` dans un widget volumineux
- `const` constructors sur tous les widgets qui le permettent (optimisation de rebuild)
- `Key` sur les items de listes dynamiques
- Sémantique accessible : widget `Semantics` sur les éléments interactifs non standards

```dart
// ✅ Widget extrait, const, avec accessibilité
class UserCard extends StatelessWidget {
  const UserCard({
    super.key,
    required this.name,
    required this.avatarUrl,
    required this.onTap,
  });

  final String name;
  final String avatarUrl;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: 'Profil de $name',
      child: GestureDetector(
        onTap: onTap,
        child: Row(
          children: [
            CircleAvatar(backgroundImage: NetworkImage(avatarUrl)),
            const SizedBox(width: 8),
            Text(name, style: Theme.of(context).textTheme.bodyLarge),
          ],
        ),
      ),
    );
  }
}
```

---

## Gestion d'état

### BLoC / Cubit

- Un BLoC par feature ou sous-domaine
- Les états sont des `sealed class` ou `freezed` — pas de booleans multiples
- Les événements sont des classes immutables
- Les Cubits sont préférés aux BLoCs quand les événements sont simples

```dart
// ✅ États typés avec sealed class
sealed class AuthState {}
class AuthInitial extends AuthState {}
class AuthLoading extends AuthState {}
class AuthAuthenticated extends AuthState {
  final User user;
  AuthAuthenticated(this.user);
}
class AuthError extends AuthState {
  final String message;
  AuthError(this.message);
}
```

### Riverpod

- Providers annotés avec `@riverpod` (code generation)
- `AsyncNotifierProvider` pour les états asynchrones
- `ref.watch` dans les widgets, `ref.read` dans les callbacks

---

## Performance

- `ListView.builder` pour les listes longues — jamais `ListView` avec `children: [...]`
- `RepaintBoundary` pour isoler les parties de l'UI qui se mettent à jour fréquemment
- Éviter les rebuilds inutiles : `const`, `select()` sur les providers, `BlocSelector`
- Analyser avec Flutter DevTools (Widget Rebuild tracker, Performance overlay)
- Éviter les allocations dans `build()` — extraire les instances en dehors

---

## Sécurité

- `flutter_secure_storage` pour les tokens et données sensibles
- Obfuscation activée en release : `flutter build --obfuscate --split-debug-info`
- Pas de données sensibles dans les logs
- Valider les deep links entrants avant traitement

---

## Tests

- Tests unitaires : `flutter_test` (inclus dans le SDK Flutter)
- Tests de widget : `WidgetTester` pour les composants UI
- Tests d'intégration : `integration_test` (sur device ou émulateur)
- Mocking : `mockito` + `build_runner`

```dart
// ✅ Test de widget avec mock
testWidgets('affiche le nom de l\'utilisateur', (tester) async {
  await tester.pumpWidget(
    MaterialApp(home: UserCard(name: 'Alice', avatarUrl: '', onTap: () {})),
  );
  expect(find.text('Alice'), findsOneWidget);
});
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Fichiers | snake_case | `user_profile_page.dart` |
| Classes / Widgets | PascalCase | `UserProfilePage` |
| Variables / fonctions | camelCase | `fetchUsers()` |
| Constantes | lowerCamelCase | `defaultTimeout` |
| BLoC | suffixes `Bloc`, `Event`, `State` | `AuthBloc`, `AuthEvent`, `AuthState` |
| Pages | suffixe `Page` | `UserProfilePage` |

---

## Ce que tu ne fais PAS

- Publier sur le Play Store ou l'App Store sans validation humaine
- Stocker des données sensibles dans `SharedPreferences`
- Utiliser `StatefulWidget` pour de l'état qui appartient à la couche domaine
- Créer des widgets monolithiques avec des méthodes `_buildXxx()` — extraire des classes
- Utiliser des packages non maintenus sans justification documentée
