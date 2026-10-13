---
name: dev-standards-kotlin
description: "Utiliser quand le projet utilise Kotlin/Android (Compose, MVVM, Coroutines, Hilt) et que le ticket touche ce code."
versions:
  kotlin: "2.4"
  ksp: "2.3"
reviewed: 2026-10-12
---

# Skill — Standards Kotlin (Android natif)

> Versions de référence (octobre 2026) : Kotlin 2.4 (K2), KSP 2.3, Jetpack Compose.

## Rôle

Ce skill définit les bonnes pratiques pour le développement Android natif avec Kotlin
et Jetpack Compose — il ne couvre pas Kotlin côté serveur (Ktor, Spring) ni Kotlin Multiplatform.
Il complète `dev-standards-universal.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets ou tokens dans le code de l'application
❌ Jamais de données sensibles dans `SharedPreferences` non chiffré
❌ Jamais de logs en production sans guard `BuildConfig.DEBUG`
✅ Les données sensibles sont chiffrées avec une clé de l'`Android Keystore` (`EncryptedSharedPreferences` est déprécié)
✅ Toute communication réseau passe par HTTPS — Network Security Config activé

---

## Architecture

- **Jetpack Compose** pour toutes les nouvelles UI (API stable)
- Architecture **MVVM + Clean** : `ViewModel` → `UseCase` → `Repository`
- `StateFlow` / `SharedFlow` pour les états réactifs
- Injection de dépendances : **Hilt** (recommandé) ou **Koin**
- Compilateur K2 (défaut depuis Kotlin 2.0) ; processeurs d'annotations via **KSP** (Hilt, Room, Moshi) — kapt est en mode maintenance
- Organisation par feature :

```
app/src/main/java/com/exemple/
├── feature/
│   └── auth/
│       ├── data/          ← repositories, data sources, DTOs, Room DAOs
│       ├── domain/        ← entités, use cases, interfaces repository
│       └── presentation/  ← ViewModel, composables, état UI
├── core/                  ← thème, navigation, injection, utils
└── MainActivity.kt
```

---

## Kotlin moderne

- Coroutines + Flow pour toute logique asynchrone — pas de callbacks
- `data class` pour les modèles immuables (DTOs, entités de présentation)
- `sealed class` / `sealed interface` pour les états UI et les résultats d'opérations
- Extension functions pour enrichir les types sans héritage
- `Result<T>` ou un type `Outcome<T>` pour les résultats avec erreur explicite

```kotlin
// ✅ ViewModel avec StateFlow et sealed state
@HiltViewModel
class UserListViewModel @Inject constructor(
    private val getUsersUseCase: GetUsersUseCase
) : ViewModel() {

    private val _uiState = MutableStateFlow<UiState>(UiState.Loading)
    val uiState: StateFlow<UiState> = _uiState.asStateFlow()

    init { loadUsers() }

    private fun loadUsers() {
        viewModelScope.launch {
            getUsersUseCase()
                .onSuccess { users -> _uiState.value = UiState.Success(users) }
                .onFailure { error -> _uiState.value = UiState.Error(error.message ?: "Erreur inconnue") }
        }
    }

    sealed interface UiState {
        data object Loading : UiState
        data class Success(val users: List<User>) : UiState
        data class Error(val message: String) : UiState
    }
}
```

---

## Jetpack Compose

- Les composables de présentation reçoivent des données et émettent des événements — pas d'accès direct au ViewModel depuis les composables feuilles
- `remember` et `rememberSaveable` pour l'état local de l'UI
- `LazyColumn` / `LazyRow` pour les listes longues — jamais `Column` + `forEach`
- Les composables sont préfixés par le nom de la feature si spécifiques

```kotlin
// ✅ Composable sans logique, état passé en paramètre
@Composable
fun UserCard(
    name: String,
    avatarUrl: String,
    onCardClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Card(
        onClick = onCardClick,
        modifier = modifier.semantics { contentDescription = "Profil de $name" }
    ) {
        Row(modifier = Modifier.padding(12.dp)) {
            AsyncImage(model = avatarUrl, contentDescription = "Avatar de $name")
            Spacer(modifier = Modifier.width(8.dp))
            Text(text = name, style = MaterialTheme.typography.bodyLarge)
        }
    }
}
```

---

## Performance

- `LazyColumn` / `LazyRow` avec `key` stables pour les listes
- `derivedStateOf` pour les calculs dérivés coûteux
- Éviter les recompositions inutiles : `remember`, `key`, décomposer les composables
- Analyser avec le Composition Tracing et le Layout Inspector d'Android Studio

---

## Sécurité

- Clés de chiffrement dans l'`Android Keystore` (API plateforme) pour les données sensibles — plus `EncryptedSharedPreferences`, déprécié
- `Network Security Config` (XML) pour forcer HTTPS et le certificate pinning
- R8 activé en release (`isMinifyEnabled = true` dans `build.gradle.kts`)
- Guard `BuildConfig.DEBUG` autour de tous les logs de développement
- Valider les deep links entrants (Intent source, paramètres)

---

## Tests

- Tests unitaires : **JUnit 5** + **Mockk**
- Tests de composables Compose : **Compose UI Test** (`ComposeTestRule`)
- Tests d'intégration : **Espresso** pour les flows critiques
- Les ViewModels sont testés indépendamment avec `runTest` et un `StandardTestDispatcher` injecté via `Dispatchers.setMain` (`TestCoroutineDispatcher` est déprécié)

```kotlin
// ✅ Test ViewModel avec Turbine (Flow testing)
@Test
fun `loadUsers emits Success state when use case succeeds`() = runTest {
    val mockUseCase = mockk<GetUsersUseCase> {
        coEvery { invoke() } returns Result.success(listOf(User("1", "Alice")))
    }
    val viewModel = UserListViewModel(mockUseCase)

    viewModel.uiState.test {
        val state = awaitItem()
        assertIs<UserListViewModel.UiState.Success>(state)
        assertEquals(1, state.users.size)
    }
}
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Classes / Interfaces | UpperCamelCase | `UserRepository`, `GetUsersUseCase` |
| Fonctions / variables | lowerCamelCase | `loadUsers()`, `currentUser` |
| Constantes (`companion object`) | SCREAMING_SNAKE_CASE | `MAX_RETRY_COUNT` |
| Packages | lowercase arborescent | `com.exemple.feature.auth` |
| Composables | PascalCase | `UserProfileScreen`, `UserCard` |
| Fichiers | même nom que la classe principale | `UserListViewModel.kt` |

---

## Ce que tu ne fais PAS

- Publier sur le Play Store sans validation humaine
- Stocker des données sensibles dans `SharedPreferences` non chiffré
- Ajouter `kapt` quand le processeur supporte KSP
- Utiliser des `AsyncTask` ou des callbacks — Coroutines + Flow uniquement
- Accéder au ViewModel depuis les composables feuilles — remonter l'état au niveau Screen
- Utiliser des singletons non injectés (Hilt/Koin) — jamais d'`object` global avec état
