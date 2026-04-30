---
name: dev-standards-angular
description: Standards Angular — standalone components, signals, injection de dépendances, RxJS, routing, conventions et bonnes pratiques.
---

# Skill — Standards Angular

## Rôle

Ce skill définit les bonnes pratiques pour le développement avec Angular (v17+).
Il complète `dev-standards-universal.md`, `dev-standards-typescript.md` et
`dev-standards-frontend.md`.

---

## 🔒 Règles absolues

❌ Pas de logique métier dans les composants — extraire dans des services
❌ Jamais de manipulation directe du DOM — utiliser les abstractions Angular (`Renderer2`, bindings)
✅ Toute décision sur la gestion d'état (Signals, NgRx, Akita) est soumise à validation explicite

---

## Standalone Components (v17+)

- Utiliser les **standalone components** systématiquement — pas de NgModules pour les nouveaux développements
- `imports` directement dans le décorateur `@Component`

```typescript
// ✅ Standalone component moderne
@Component({
  selector: 'app-user-card',
  standalone: true,
  imports: [CommonModule, RouterLink],
  template: `
    <article>
      <h3>{{ user().name }}</h3>
      <p>{{ user().email }}</p>
      <button (click)="onEdit()">Modifier</button>
    </article>
  `,
})
export class UserCardComponent {
  user = input.required<User>()
  edit = output<void>()

  onEdit() {
    this.edit.emit()
  }
}
```

---

## Signals (v17+)

- Préférer les **Signals** pour l'état local et réactif — plus performants que les Observables pour l'état synchrone
- `input()` / `output()` pour les inputs/outputs (remplace `@Input`/`@Output`)
- `computed()` pour les valeurs dérivées
- `effect()` avec parcimonie — uniquement pour les effets de bord

```typescript
// ✅ Composant avec Signals
@Component({ standalone: true, ... })
export class CounterComponent {
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
@Component({ standalone: true })
export class UserListComponent {
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
@Component({ standalone: true })
export class NotificationsComponent {
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
      import('./pages/dashboard/dashboard.component').then(m => m.DashboardComponent),
    canActivate: [AuthGuard],
  },
  {
    path: 'users',
    loadChildren: () =>
      import('./features/users/users.routes').then(m => m.USERS_ROUTES),
  },
]
```

- Lazy loading systématique pour les features (`loadComponent` / `loadChildren`)
- Guards injectables avec `inject()` (syntaxe fonctionnelle v15+)

---

## Formulaires

- **Reactive Forms** pour les formulaires complexes avec validation
- **Template-driven forms** uniquement pour les formulaires simples
- Validation côté client avec les validators Angular + validation côté serveur

```typescript
// ✅ Reactive Form avec validation
@Component({ standalone: true, imports: [ReactiveFormsModule] })
export class LoginFormComponent {
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
| Composants | kebab-case sélecteur + PascalCase classe | `app-user-card`, `UserCardComponent` |
| Services | suffixe `Service` | `UserService` |
| Guards | suffixe `Guard` | `AuthGuard` |
| Resolvers | suffixe `Resolver` | `UserResolver` |
| Pipes | suffixe `Pipe` | `FormatDatePipe` |
| Fichiers | kebab-case | `user-card.component.ts` |
| Signals d'input | `input()` / `input.required()` | `user = input.required<User>()` |
| Signals d'output | `output()` | `edit = output<void>()` |

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les composants — la déléguer aux services
- Manipuler le DOM directement — utiliser les bindings Angular ou `Renderer2`
- Oublier de se désabonner des Observables — `takeUntilDestroyed` systématique
- Utiliser des NgModules pour les nouveaux développements
- Ignorer les Signals pour l'état local — ne pas tout passer par RxJS
