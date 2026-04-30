---
name: dev-standards-laravel
description: Standards Laravel — Eloquent, controllers, Form Requests, services, middlewares, queues et bonnes pratiques.
---

# Skill — Standards Laravel

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec Laravel.
Il complète `dev-standards-backend.md` et `dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les controllers — déléguer aux services ou actions
❌ Jamais de secrets en dur dans le code — utiliser `.env` et `config()`
❌ Jamais de requêtes brutes sans paramètres liés (risque d'injection SQL)
✅ Toute entrée externe est validée via une `FormRequest`

---

## Structure du projet

```
app/
├── Http/
│   ├── Controllers/
│   │   └── Api/
│   │       └── UserController.php
│   ├── Requests/           ← Form Requests (validation)
│   │   ├── StoreUserRequest.php
│   │   └── UpdateUserRequest.php
│   ├── Resources/          ← API Resources (transformation)
│   │   └── UserResource.php
│   └── Middleware/
├── Models/
│   └── User.php
├── Services/               ← logique métier
│   └── UserService.php
├── Actions/                ← actions unitaires (alternative aux services)
│   └── CreateUserAction.php
└── Repositories/           ← abstraction de l'accès aux données (optionnel)
```

---

## Models Eloquent

- Définir `$fillable` ou `$guarded` explicitement — jamais les deux
- Les relations sont typées avec PHPDoc ou les return types
- Les scopes locaux pour les requêtes fréquentes

```php
// ✅ Model bien configuré
class User extends Authenticatable
{
    protected $fillable = ['name', 'email'];

    protected $hidden = ['password', 'remember_token'];

    protected $casts = [
        'email_verified_at' => 'datetime',
        'password'          => 'hashed',
    ];

    // ✅ Scope local pour les requêtes fréquentes
    public function scopeActive(Builder $query): Builder
    {
        return $query->where('is_active', true);
    }

    // ✅ Relation typée
    public function orders(): HasMany
    {
        return $this->hasMany(Order::class);
    }
}
```

---

## Form Requests (validation)

```php
// ✅ FormRequest — validation et autorisation séparées
class StoreUserRequest extends FormRequest
{
    public function authorize(): bool
    {
        return true; // ou vérification de permission
    }

    public function rules(): array
    {
        return [
            'name'     => ['required', 'string', 'min:2', 'max:100'],
            'email'    => ['required', 'email', 'unique:users,email'],
            'password' => ['required', 'string', 'min:8', 'confirmed'],
        ];
    }

    public function messages(): array
    {
        return [
            'email.unique' => 'Cet email est déjà utilisé.',
        ];
    }
}
```

---

## Controllers

```php
// ✅ Controller mince — délègue au service
class UserController extends Controller
{
    public function __construct(private UserService $userService) {}

    public function show(User $user): UserResource
    {
        return new UserResource($user);
    }

    public function store(StoreUserRequest $request): JsonResponse
    {
        $user = $this->userService->create($request->validated());
        return (new UserResource($user))->response()->setStatusCode(201);
    }

    public function update(UpdateUserRequest $request, User $user): UserResource
    {
        $updated = $this->userService->update($user, $request->validated());
        return new UserResource($updated);
    }
}
```

---

## API Resources (transformation)

```php
// ✅ Resource — transforme le modèle en réponse API
class UserResource extends JsonResource
{
    public function toArray(Request $request): array
    {
        return [
            'id'         => $this->id,
            'name'       => $this->name,
            'email'      => $this->email,
            'created_at' => $this->created_at->toISOString(),
            // password jamais exposé
        ];
    }
}
```

---

## Services

```php
// ✅ Service — logique métier isolée
class UserService
{
    public function create(array $data): User
    {
        return DB::transaction(function () use ($data) {
            $user = User::create([
                'name'     => $data['name'],
                'email'    => $data['email'],
                'password' => $data['password'], // hashé via cast 'hashed'
            ]);
            SendWelcomeEmail::dispatch($user);
            return $user;
        });
    }

    public function update(User $user, array $data): User
    {
        $user->update($data);
        return $user->fresh();
    }
}
```

---

## Migrations

```php
// ✅ Migration bien structurée
Schema::create('users', function (Blueprint $table) {
    $table->uuid('id')->primary();
    $table->string('name');
    $table->string('email')->unique();
    $table->string('password');
    $table->boolean('is_active')->default(true)->index();
    $table->timestamps();
    $table->softDeletes();
});
```

- Ne jamais modifier une migration déjà exécutée en production — créer une nouvelle migration
- Les colonnes fréquemment filtrées ont un index

---

## Queues et jobs

```php
// ✅ Job pour les opérations asynchrones
class SendWelcomeEmail implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public function __construct(private User $user) {}

    public function handle(Mailer $mailer): void
    {
        $mailer->to($this->user->email)->send(new WelcomeMail($this->user));
    }

    public function failed(\Throwable $exception): void
    {
        Log::error('Échec envoi email de bienvenue', [
            'user_id' => $this->user->id,
            'error'   => $exception->getMessage(),
        ]);
    }
}
```

---

## Tests

```php
// ✅ Test de feature avec RefreshDatabase
class UserControllerTest extends TestCase
{
    use RefreshDatabase;

    public function test_create_user_returns_201(): void
    {
        $response = $this->postJson('/api/users', [
            'name'                  => 'Alice',
            'email'                 => 'alice@exemple.com',
            'password'              => 'SecretPass1',
            'password_confirmation' => 'SecretPass1',
        ]);

        $response->assertCreated()
                 ->assertJsonPath('data.email', 'alice@exemple.com')
                 ->assertJsonMissing(['password']);

        $this->assertDatabaseHas('users', ['email' => 'alice@exemple.com']);
    }
}
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les controllers
- Utiliser `$guarded = []` (tout fillable) — définir `$fillable` explicitement
- Exposer les passwords dans les API Resources
- Modifier des migrations déjà exécutées en production
- Ignorer les transactions pour les opérations multi-étapes
