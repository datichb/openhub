---
name: dev-standards-java
description: Utiliser quand le projet contient du Java (Maven ou Gradle) — Java moderne, records, null, exceptions, concurrence, tests JUnit 5, conventions de nommage.
---

# Skill — Standards Java

> Versions de référence (octobre 2026) : Java 25 LTS (21 minimum), Maven 3.9, Gradle 9, JUnit 5.

## Rôle

Ce skill définit les conventions du langage Java, quel que soit le type de projet
(bibliothèque, CLI, service, batch). Il ne couvre pas un framework : pour Spring Boot,
`dev-standards-springboot` s'ajoute à celui-ci. Il complète `dev-standards-universal.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets en dur dans le code ni dans les fichiers de build — variables d'environnement ou coffre de secrets
❌ Jamais de `catch (Exception e) {}` vide : une exception est traitée, enrichie et relancée, ou journalisée
❌ Jamais de SQL construit par concaténation — `PreparedStatement` ou requêtes paramétrées
❌ Jamais de désérialisation Java native (`ObjectInputStream`) sur des données externes
✅ Les versions des dépendances sont figées (BOM Maven, catalogue de versions Gradle) et le wrapper (`mvnw`, `gradlew`) est commité

---

## Organisation du projet

- Disposition standard Maven/Gradle : `src/main/java`, `src/main/resources`, `src/test/java`
- Organisation **par fonctionnalité** plutôt que par couche technique
- Visibilité minimale : `package-private` par défaut, `public` seulement pour l'API du paquet
- Modules JPMS (`module-info.java`) pour les bibliothèques publiées ; facultatifs pour une application

```
src/main/java/com/exemple/facturation/
├── facture/
│   ├── Facture.java            ← record ou classe du domaine
│   ├── FactureService.java     ← logique métier
│   └── FactureRepository.java  ← interface d'accès aux données
├── client/
└── commun/                     ← types partagés, erreurs
```

---

## Java moderne

- `record` pour les objets de valeur immuables (DTOs, identifiants, résultats)
- `sealed interface` + `record` pour modéliser des alternatives fermées
- `switch` en expression et filtrage par motif (`instanceof`, motifs de record) plutôt que des cascades de `if`
- `var` pour les variables locales quand le type est évident à la lecture
- Blocs de texte (`"""`) pour le SQL, le JSON ou les gabarits multi-lignes
- Collections immuables : `List.of`, `Map.of`, `Stream.toList()`

```java
// ✅ Alternatives fermées et switch exhaustif (pas de default)
public sealed interface Paiement permits Carte, Virement {}
public record Carte(String numeroMasque) implements Paiement {}
public record Virement(String iban) implements Paiement {}

static String libelle(Paiement p) {
    return switch (p) {
        case Carte c -> "Carte " + c.numeroMasque();
        case Virement v -> "Virement " + v.iban();
    };
}
```

---

## Null et validation

- Pas de `null` en retour d'une méthode publique : `Optional<T>` pour une valeur absente, collection vide pour une liste
- `Optional` uniquement en type de retour — jamais en champ ni en paramètre
- Valider les paramètres à l'entrée (`Objects.requireNonNull`, constructeur compact de record)
- Annotations de nullité (JSpecify `@Nullable`, `@NullMarked`) si l'équipe les utilise ; les appliquer partout dans le paquet

```java
// ✅ Validation dans le constructeur compact
public record Montant(BigDecimal valeur, Currency devise) {
    public Montant {
        Objects.requireNonNull(valeur, "valeur");
        Objects.requireNonNull(devise, "devise");
        if (valeur.signum() < 0) throw new IllegalArgumentException("montant négatif");
    }
}
```

---

## Exceptions

- Exceptions non vérifiées (`RuntimeException`) pour les erreurs du domaine, avec un message utile et la cause d'origine
- Exceptions vérifiées réservées aux cas où l'appelant peut réellement se rétablir
- `try-with-resources` pour tout `AutoCloseable` (fichiers, connexions, flux)
- Ne jamais perdre la cause : `throw new FactureIntrouvable(id, e)`
- Montants : `BigDecimal`, jamais `double` ; dates : `java.time` (`Instant`, `LocalDate`), jamais `Date`

---

## Concurrence

- **Threads virtuels** (`Executors.newVirtualThreadPerTaskExecutor()`) pour les traitements d'E/S bloquants
- Pas de `synchronized` autour d'appels bloquants longs ; préférer `ReentrantLock` ou des structures de `java.util.concurrent`
- État partagé immuable par défaut ; sinon `ConcurrentHashMap`, `AtomicLong`
- Jamais de `new Thread()` nu ni de `Thread.sleep` pour attendre un résultat

---

## Build et qualité

- Compilation avec `-Xlint:all` et `-Werror` quand c'est réaliste
- Formatage automatique (Spotless avec google-java-format ou Palantir) vérifié en CI
- Analyse statique : Error Prone ou SpotBugs ; dépendances auditées (OWASP Dependency-Check ou équivalent)
- Journalisation via SLF4J avec paramètres (`log.info("facture {} créée", id)`), jamais de `System.out`

---

## Tests

- **JUnit 5** (Jupiter) + **AssertJ** ; **Mockito** pour isoler les dépendances externes
- Tests paramétrés (`@ParameterizedTest`) pour les cas limites
- Tests d'intégration avec **Testcontainers** pour les bases et les courtiers de messages réels
- Nom des tests : le comportement attendu, pas le nom de la méthode

```java
// ✅ Test lisible avec AssertJ
@Test
void refuse_un_montant_negatif() {
    assertThatThrownBy(() -> new Montant(new BigDecimal("-1"), Currency.getInstance("EUR")))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("négatif");
}
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Classes / Interfaces / Records | UpperCamelCase | `FactureService`, `Montant` |
| Méthodes / variables | lowerCamelCase | `calculerTotal()`, `dateEcheance` |
| Constantes (`static final`) | SCREAMING_SNAKE_CASE | `TAUX_TVA_NORMAL` |
| Paquets | minuscules, domaine inversé | `com.exemple.facturation` |
| Tests | classe testée + `Test` | `FactureServiceTest` |
| Fichiers | même nom que le type public | `Facture.java` |

---

## Ce que tu ne fais PAS

- Monter de version de Java ou d'outil de build sans validation humaine
- Ajouter une dépendance sans vérifier qu'une API du JDK ne suffit pas
- Utiliser `Date`, `Calendar`, `Vector` ou `Hashtable`
- Renvoyer `null` depuis une API publique ou attraper `Throwable`
- Écrire des getters/setters mutables pour un simple objet de valeur — utiliser un `record`
