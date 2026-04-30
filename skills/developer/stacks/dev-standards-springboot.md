---
name: dev-standards-springboot
description: Standards Spring Boot — beans, JPA/Hibernate, REST controllers, validation, sécurité, tests et bonnes pratiques Java.
---

# Skill — Standards Spring Boot

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec Spring Boot.
Il complète `dev-standards-backend.md` et `dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les controllers — déléguer aux services
❌ Jamais de secrets en dur dans le code — utiliser `application.properties` + variables d'environnement
❌ Jamais d'exposition directe des entités JPA dans les réponses API — utiliser des DTOs
✅ Toute entrée externe est validée avec Bean Validation (`@Valid`)

---

## Structure du projet

```
src/main/java/com/exemple/app/
├── config/                 ← configuration (Security, CORS, etc.)
├── controller/             ← REST controllers
│   └── UserController.java
├── service/                ← logique métier
│   └── UserService.java
├── repository/             ← accès aux données (Spring Data JPA)
│   └── UserRepository.java
├── entity/                 ← entités JPA
│   └── User.java
├── dto/                    ← Data Transfer Objects
│   ├── CreateUserRequest.java
│   ├── UpdateUserRequest.java
│   └── UserResponse.java
├── exception/              ← exceptions custom + handler global
│   ├── UserNotFoundException.java
│   └── GlobalExceptionHandler.java
└── mapper/                 ← mappers entity ↔ DTO (MapStruct)
    └── UserMapper.java
```

---

## Entités JPA

```java
// ✅ Entité JPA bien configurée
@Entity
@Table(name = "users", indexes = {
    @Index(name = "idx_users_email", columnList = "email", unique = true)
})
public class User {

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private UUID id;

    @Column(nullable = false, length = 100)
    private String name;

    @Column(nullable = false, unique = true)
    private String email;

    @Column(nullable = false)
    private String passwordHash;

    @Column(nullable = false)
    private boolean active = true;

    @CreationTimestamp
    private Instant createdAt;

    @UpdateTimestamp
    private Instant updatedAt;

    // getters, setters ou Lombok @Data/@Getter/@Setter
}
```

---

## DTOs et validation

```java
// ✅ DTO d'entrée avec Bean Validation
public record CreateUserRequest(
    @NotBlank @Email
    String email,

    @NotBlank @Size(min = 2, max = 100)
    String name,

    @NotBlank @Size(min = 8)
    @Pattern(regexp = ".*[A-Z].*", message = "Le mot de passe doit contenir au moins une majuscule")
    String password
) {}

// ✅ DTO de réponse — sans données sensibles
public record UserResponse(
    UUID id,
    String email,
    String name,
    Instant createdAt
) {}
```

---

## Controllers

```java
// ✅ Controller mince — délègue au service
@RestController
@RequestMapping("/api/v1/users")
@RequiredArgsConstructor
public class UserController {

    private final UserService userService;

    @GetMapping("/{id}")
    public ResponseEntity<UserResponse> findOne(@PathVariable UUID id) {
        return ResponseEntity.ok(userService.findOneOrThrow(id));
    }

    @PostMapping
    public ResponseEntity<UserResponse> create(@Valid @RequestBody CreateUserRequest request) {
        UserResponse created = userService.create(request);
        URI location = URI.create("/api/v1/users/" + created.id());
        return ResponseEntity.created(location).body(created);
    }

    @PatchMapping("/{id}")
    public ResponseEntity<UserResponse> update(
        @PathVariable UUID id,
        @Valid @RequestBody UpdateUserRequest request
    ) {
        return ResponseEntity.ok(userService.update(id, request));
    }
}
```

---

## Services

```java
// ✅ Service avec logique métier et gestion des erreurs
@Service
@Transactional
@RequiredArgsConstructor
public class UserService {

    private final UserRepository userRepository;
    private final UserMapper userMapper;
    private final PasswordEncoder passwordEncoder;

    @Transactional(readOnly = true)
    public UserResponse findOneOrThrow(UUID id) {
        User user = userRepository.findById(id)
            .orElseThrow(() -> new UserNotFoundException(id));
        return userMapper.toResponse(user);
    }

    public UserResponse create(CreateUserRequest request) {
        if (userRepository.existsByEmail(request.email())) {
            throw new ConflictException("Cet email est déjà utilisé : " + request.email());
        }
        User user = new User();
        user.setEmail(request.email());
        user.setName(request.name());
        user.setPasswordHash(passwordEncoder.encode(request.password()));
        return userMapper.toResponse(userRepository.save(user));
    }
}
```

---

## Gestion des erreurs

```java
// ✅ Handler global des exceptions
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(UserNotFoundException.class)
    public ProblemDetail handleNotFound(UserNotFoundException ex) {
        ProblemDetail detail = ProblemDetail.forStatusAndDetail(HttpStatus.NOT_FOUND, ex.getMessage());
        detail.setProperty("code", "NOT_FOUND");
        return detail;
    }

    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ProblemDetail handleValidation(MethodArgumentNotValidException ex) {
        ProblemDetail detail = ProblemDetail.forStatusAndDetail(HttpStatus.UNPROCESSABLE_ENTITY, "Données invalides");
        detail.setProperty("code", "VALIDATION_ERROR");
        detail.setProperty("fields", ex.getBindingResult().getFieldErrors().stream()
            .collect(Collectors.toMap(FieldError::getField, FieldError::getDefaultMessage)));
        return detail;
    }
}
```

---

## Repository

```java
// ✅ Repository Spring Data JPA
@Repository
public interface UserRepository extends JpaRepository<User, UUID> {

    boolean existsByEmail(String email);

    Optional<User> findByEmail(String email);

    @Query("SELECT u FROM User u WHERE u.active = true ORDER BY u.createdAt DESC")
    List<User> findAllActive();
}
```

---

## Tests

```java
// ✅ Test d'intégration avec @SpringBootTest
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
@AutoConfigureMockMvc
class UserControllerIntegrationTest {

    @Autowired
    private MockMvc mockMvc;

    @Test
    void createUser_returnsCreated() throws Exception {
        mockMvc.perform(post("/api/v1/users")
                .contentType(MediaType.APPLICATION_JSON)
                .content("""
                    {"email":"alice@exemple.com","name":"Alice","password":"SecretPass1"}
                    """))
            .andExpect(status().isCreated())
            .andExpect(jsonPath("$.email").value("alice@exemple.com"))
            .andExpect(jsonPath("$.passwordHash").doesNotExist());
    }
}
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les controllers
- Exposer des entités JPA directement dans les réponses API
- Omettre `@Transactional` sur les opérations d'écriture
- Utiliser `@Transactional` sur les controllers — uniquement sur les services
- Créer des requêtes N+1 avec des relations Lazy non anticipées
