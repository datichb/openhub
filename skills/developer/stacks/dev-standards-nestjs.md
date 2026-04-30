---
name: dev-standards-nestjs
description: Standards NestJS — modules, providers, controllers, guards, pipes, interceptors, validation DTO, injection de dépendances.
---

# Skill — Standards NestJS

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec NestJS.
Il complète `dev-standards-universal.md`, `dev-standards-typescript.md`,
`dev-standards-backend.md` et `dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les controllers — déléguer aux services
❌ Jamais d'accès direct à la base de données depuis un controller ou un service d'application
❌ Jamais de secrets en dur dans le code — utiliser `ConfigService`
✅ Toute entrée externe est validée via un DTO avec `class-validator`

---

## Architecture des modules

```
src/
├── app.module.ts               ← module racine
├── config/                     ← configuration (ConfigModule, validation env)
├── common/                     ← guards, pipes, interceptors, filters partagés
│   ├── guards/
│   ├── pipes/
│   ├── interceptors/
│   └── filters/
└── features/
    └── users/
        ├── users.module.ts
        ├── users.controller.ts
        ├── users.service.ts
        ├── users.repository.ts  ← optionnel selon l'ORM
        ├── dto/
        │   ├── create-user.dto.ts
        │   └── update-user.dto.ts
        └── entities/
            └── user.entity.ts
```

- Un module = une feature — pas de module "fourre-tout"
- `forwardRef()` à éviter — signe d'une dépendance circulaire à résoudre architecturalement

---

## Controllers

- Les controllers reçoivent, valident (via pipes) et délèguent — pas de logique métier
- Décorateurs de méthode HTTP sémantiques : `@Get`, `@Post`, `@Patch`, `@Delete`
- Réponses typées avec les DTOs de réponse

```typescript
// ✅ Controller mince — délègue au service
@Controller('users')
@UseGuards(JwtAuthGuard)
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Get(':id')
  async findOne(@Param('id', ParseUUIDPipe) id: string): Promise<UserResponseDto> {
    return this.usersService.findOneOrThrow(id)
  }

  @Post()
  @HttpCode(HttpStatus.CREATED)
  async create(@Body() dto: CreateUserDto): Promise<UserResponseDto> {
    return this.usersService.create(dto)
  }

  @Patch(':id')
  async update(
    @Param('id', ParseUUIDPipe) id: string,
    @Body() dto: UpdateUserDto,
  ): Promise<UserResponseDto> {
    return this.usersService.update(id, dto)
  }
}
```

---

## DTOs et validation

- Tous les inputs sont des classes DTO avec des décorateurs `class-validator`
- `ValidationPipe` global avec `whitelist: true` et `forbidNonWhitelisted: true`
- DTOs d'entrée et de sortie distincts — ne jamais retourner une entité directement

```typescript
// ✅ DTO d'entrée avec validation
export class CreateUserDto {
  @IsEmail()
  @IsNotEmpty()
  email: string

  @IsString()
  @MinLength(2)
  @MaxLength(100)
  name: string

  @IsString()
  @MinLength(8)
  @Matches(/^(?=.*[A-Z])(?=.*\d)/, {
    message: 'Le mot de passe doit contenir au moins une majuscule et un chiffre',
  })
  password: string
}

// ✅ DTO de réponse — exclut les champs sensibles
export class UserResponseDto {
  @Expose()
  id: string

  @Expose()
  email: string

  @Expose()
  name: string

  @Expose()
  createdAt: Date
  // password non exposé
}
```

---

## Services

- La logique métier est dans les services
- Les services peuvent appeler d'autres services ou des repositories
- Les erreurs métier lèvent des exceptions NestJS (`NotFoundException`, `ConflictException`, etc.)

```typescript
@Injectable()
export class UsersService {
  constructor(private readonly usersRepository: UsersRepository) {}

  async findOneOrThrow(id: string): Promise<UserResponseDto> {
    const user = await this.usersRepository.findById(id)
    if (!user) throw new NotFoundException(`Utilisateur ${id} introuvable`)
    return plainToInstance(UserResponseDto, user, { excludeExtraneousValues: true })
  }

  async create(dto: CreateUserDto): Promise<UserResponseDto> {
    const existing = await this.usersRepository.findByEmail(dto.email)
    if (existing) throw new ConflictException('Cet email est déjà utilisé')

    const hashed = await bcrypt.hash(dto.password, 12)
    const user = await this.usersRepository.create({ ...dto, password: hashed })
    return plainToInstance(UserResponseDto, user, { excludeExtraneousValues: true })
  }
}
```

---

## Guards et décorateurs custom

```typescript
// ✅ Guard JWT réutilisable
@Injectable()
export class JwtAuthGuard extends AuthGuard('jwt') {
  handleRequest<T>(err: Error, user: T): T {
    if (err || !user) throw err ?? new UnauthorizedException()
    return user
  }
}

// ✅ Décorateur pour récupérer l'utilisateur courant
export const CurrentUser = createParamDecorator(
  (data: keyof User | undefined, ctx: ExecutionContext) => {
    const request = ctx.switchToHttp().getRequest<Request & { user: User }>()
    return data ? request.user[data] : request.user
  },
)
```

---

## Configuration

```typescript
// ✅ Validation de la config au démarrage
const envSchema = Joi.object({
  DATABASE_URL: Joi.string().required(),
  JWT_SECRET: Joi.string().min(32).required(),
  PORT: Joi.number().default(3000),
})

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      validationSchema: envSchema,
    }),
  ],
})
export class AppModule {}
```

---

## Tests

```typescript
// ✅ Test unitaire d'un service avec mock du repository
describe('UsersService', () => {
  let service: UsersService
  let repository: jest.Mocked<UsersRepository>

  beforeEach(async () => {
    const module = await Test.createTestingModule({
      providers: [
        UsersService,
        { provide: UsersRepository, useValue: { findById: jest.fn(), create: jest.fn() } },
      ],
    }).compile()

    service = module.get(UsersService)
    repository = module.get(UsersRepository)
  })

  it('lève NotFoundException si l\'utilisateur n\'existe pas', async () => {
    repository.findById.mockResolvedValue(null)
    await expect(service.findOneOrThrow('non-existant')).rejects.toThrow(NotFoundException)
  })
})
```

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les controllers
- Retourner des entités directement depuis les controllers — toujours passer par des DTOs de réponse
- Utiliser `any` dans les types de DTO
- Omettre `whitelist: true` dans le `ValidationPipe` global
- Créer des dépendances circulaires entre modules
