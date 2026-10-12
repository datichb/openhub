---
name: dev-standards-angular
description: Standards Angular — standalone components, signals, injection de dépendances, RxJS, routing, conventions et bonnes pratiques.
---

# Skill — Standards Angular

> Versions de référence (octobre 2026) : Angular 22.

## Rôle

Ce skill définit les bonnes pratiques pour le développement avec Angular (v21+).
Il complète `dev-standards-universal.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Pas de logique métier dans les composants — extraire dans des services
❌ Jamais de manipulation directe du DOM — utiliser les abstractions Angular (`Renderer2`, bindings)
✅ Toute décision sur la gestion d'état (Signals, NgRx, Akita) est soumise à validation explicite

---

## Standalone Components

- Utiliser les **standalone components** systématiquement — pas de NgModules pour les nouveaux développements
- Standalone par défaut depuis v19 : ne plus écrire `standalone: true`
- `imports` directement dans le décorateur `@Component`
- Control flow natif dans les templates (`@if`, `@for` avec `track`, `@switch`, `@defer`) — plus de `*ngIf` / `*ngFor`, donc plus besoin de `CommonModule`

```typescript
// ✅ Standalone component moderne
@Component({
  selector: 'app-user-card',
  imports: [RouterLink],
  template: `
    <article>
      <h3>{{ user().name }}</h3>
      <p>{{ user().email }}</p>
      @if (user().roles.length) {
        <ul>
          @for (role of user().roles; track role) {
            <li>{{ role }}</li>
          }
        </ul>
      }
      <button (click)="onEdit()">Modifier</button>
    </article>
  `,
})
export class UserCard {
  user = input.required<User>()
  edit = output<void>()

  onEdit() {
    this.edit.emit()
  }
}
```

---

## Signals et zoneless

- Préférer les **Signals** pour l'état local et réactif — plus performants que les Observables pour l'état synchrone
- `input()` / `output()` / `model()` pour les inputs/outputs (remplace `@Input`/`@Output`)
- `computed()` pour les valeurs dérivées
- `effect()` avec parcimonie — uniquement pour les effets de bord
- Zoneless par défaut depuis v21 : pas de `zone.js`, pas de `provideZoneChangeDetection` — la détection de changement repose sur les signals, l'`async` pipe et les événements de template

```typescript
// ✅ Composant avec Signals
@Component({ ... })
export class Counter {
  count = signal(0)
  doubled = computed(() => this.count() * 2)

  increment() {
    this.count.update(c => c + 1)
  }
}
```

---

## Injection de dépendances

- Utiliser `inject()` dans le corps du constructeur ou en dehors des constructeurs (v14+)
- Les services sont `providedIn: 'root'` par défaut sauf si leur scope est limité

```typescript
// ✅ inject() moderne
@Component({ ... })
export class UserList {
  private userService = inject(UserService)
  private router = inject(Router)

  users = toSignal(this.userService.getUsers(), { initialValue: [] })
}
```

---

## Services

- Un service = une responsabilité (données, logique métier, communication API)
- Les composants ne contiennent pas de logique métier — ils délèguent aux services
- Les appels HTTP sont dans des services dédiés, jamais dans les composants

```typescript
// ✅ Service HTTP typé
@Injectable({ providedIn: 'root' })
export class UserService {
  private http = inject(HttpClient)

  getUsers(): Observable<User[]> {
    return this.http.get<User[]>('/api/users')
  }

  updateUser(id: string, data: Partial<User>): Observable<User> {
    return this.http.patch<User>(`/api/users/${id}`, data)
  }
}
```

---

## RxJS

- Utiliser RxJS pour les flux asynchrones complexes (WebSocket, opérations enchaînées)
- Préférer les Signals pour l'état simple et synchrone
- `async` pipe dans les templates pour gérer les abonnements automatiquement
- Unsubscribe systématique : `takeUntilDestroyed()` (v16+) ou `DestroyRef`

```typescript
// ✅ Gestion des abonnements avec takeUntilDestroyed
@Component({ ... })
export class Notifications {
  private destroyRef = inject(DestroyRef)

  ngOnInit() {
    this.notificationService.stream$
      .pipe(takeUntilDestroyed(this.destroyRef))
      .subscribe(notification => this.handleNotification(notification))
  }
}
```

---

## Routing

```typescript
// ✅ Routes standalone avec lazy loading
export const routes: Routes = [
  {
    path: 'dashboard',
    loadComponent: () =>
      import('./pages/dashboard/dashboard').then(m => m.Dashboard),
    canActivate: [authGuard],
  },
  {
    path: 'users',
    loadChildren: () =>
      import('./features/users/users.routes').then(m => m.USERS_ROUTES),
  },
]
```

- Lazy loading systématique pour les features (`loadComponent` / `loadChildren`)
- Guards et resolvers fonctionnels (`CanActivateFn`, `ResolveFn`) avec `inject()` — pas de classes de guard

---

## Formulaires

- **Reactive Forms** pour les formulaires complexes avec validation
- **Template-driven forms** uniquement pour les formulaires simples
- Validation côté client avec les validators Angular + validation côté serveur

```typescript
// ✅ Reactive Form avec validation
@Component({ imports: [ReactiveFormsModule] })
export class LoginForm {
  private fb = inject(FormBuilder)

  form = this.fb.group({
    email: ['', [Validators.required, Validators.email]],
    password: ['', [Validators.required, Validators.minLength(8)]],
  })

  onSubmit() {
    if (this.form.invalid) return
    this.authService.login(this.form.getRawValue())
  }
}
```

---

## Conventions

| Élément | Convention | Exemple |
|---|---|---|
| Composants | kebab-case sélecteur + PascalCase classe, sans suffixe (style guide v20+) | `app-user-card`, `UserCard` |
| Services | suffixe `Service` | `UserService` |
| Guards | fonction camelCase suffixée `Guard` | `authGuard` |
| Resolvers | fonction camelCase suffixée `Resolver` | `userResolver` |
| Pipes | suffixe `Pipe` | `FormatDatePipe` |
| Fichiers | kebab-case | `user-card.ts` |
| Signals d'input | `input()` / `input.required()` | `user = input.required<User>()` |
| Signals d'output | `output()` | `edit = output<void>()` |

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les composants — la déléguer aux services
- Manipuler le DOM directement — utiliser les bindings Angular ou `Renderer2`
- Oublier de se désabonner des Observables — `takeUntilDestroyed` systématique
- Utiliser des NgModules pour les nouveaux développements
- Ignorer les Signals pour l'état local — ne pas tout passer par RxJS
