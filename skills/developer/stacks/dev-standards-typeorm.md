---
name: dev-standards-typeorm
description: Standards TypeORM — entities, repositories, migrations, relations, QueryBuilder et bonnes pratiques.
---

# Skill — Standards TypeORM

## Rôle

Ce skill définit les bonnes pratiques pour l'accès aux données avec TypeORM.
Il complète `dev-standards-typescript.md` et `dev-standards-backend.md`.

---

## 🔒 Règles absolues

❌ Jamais de `synchronize: true` en production — utiliser les migrations
❌ Jamais d'accès direct au `DataSource` depuis les controllers
❌ Jamais de `delete()` ou `update()` sans clause `where` explicite
✅ Toute migration est relue avant d'être appliquée en production

---

## Entities

```typescript
// ✅ Entity bien configurée
import {
  Entity, PrimaryGeneratedColumn, Column,
  CreateDateColumn, UpdateDateColumn,
  Index, OneToMany,
} from 'typeorm'

@Entity('users')
@Index(['email'], { unique: true })
@Index(['active', 'createdAt'])
export class User {
  @PrimaryGeneratedColumn('uuid')
  id: string

  @Column({ length: 100 })
  name: string

  @Column({ unique: true })
  email: string

  @Column({ select: false })  // exclu des SELECT par défaut
  password: string

  @Column({ default: true })
  active: boolean

  @CreateDateColumn()
  createdAt: Date

  @UpdateDateColumn()
  updatedAt: Date

  @OneToMany(() => Order, (order) => order.user)
  orders: Order[]
}
```

- `select: false` sur les colonnes sensibles (password, tokens)
- `@Index()` sur les colonnes filtrées et triées fréquemment
- `@CreateDateColumn()` et `@UpdateDateColumn()` systématiques

---

## Repositories custom

```typescript
// ✅ Repository custom avec logique de requête
@Injectable()
export class UserRepository {
  private repo: Repository<User>

  constructor(dataSource: DataSource) {
    this.repo = dataSource.getRepository(User)
  }

  async findById(id: string): Promise<User | null> {
    return this.repo.findOne({ where: { id } })
  }

  async findByEmail(email: string): Promise<User | null> {
    return this.repo
      .createQueryBuilder('user')
      .addSelect('user.password')  // réinclure le champ exclu si nécessaire
      .where('user.email = :email', { email })
      .getOne()
  }

  async findActiveUsers(options: { limit: number; cursor?: string }): Promise<User[]> {
    const qb = this.repo
      .createQueryBuilder('user')
      .where('user.active = true')
      .orderBy('user.createdAt', 'DESC')
      .limit(options.limit)

    if (options.cursor) {
      qb.andWhere('user.id < :cursor', { cursor: options.cursor })
    }

    return qb.getMany()
  }
}
```

---

## QueryBuilder

```typescript
// ✅ QueryBuilder pour les requêtes complexes
const result = await dataSource
  .createQueryBuilder(User, 'user')
  .leftJoinAndSelect('user.orders', 'order', 'order.status = :status', { status: 'CONFIRMED' })
  .where('user.active = :active', { active: true })
  .andWhere('user.createdAt > :since', { since: new Date('2024-01-01') })
  .orderBy('user.name', 'ASC')
  .skip(offset)
  .take(limit)
  .getManyAndCount()

// ✅ Toujours utiliser des paramètres liés — jamais d'interpolation directe
// ❌ .where(`user.name = '${name}'`)  — risque d'injection SQL
// ✅ .where('user.name = :name', { name })
```

---

## Transactions

```typescript
// ✅ Transaction avec QueryRunner
async createUserWithWelcomeCredit(data: CreateUserData): Promise<User> {
  const queryRunner = dataSource.createQueryRunner()
  await queryRunner.connect()
  await queryRunner.startTransaction()

  try {
    const user = queryRunner.manager.create(User, data)
    await queryRunner.manager.save(user)

    const credit = queryRunner.manager.create(Credit, { userId: user.id, amount: 10 })
    await queryRunner.manager.save(credit)

    await queryRunner.commitTransaction()
    return user
  } catch (err) {
    await queryRunner.rollbackTransaction()
    throw err
  } finally {
    await queryRunner.release()
  }
}
```

---

## Migrations

```bash
# Générer une migration depuis les changements d'entities
npx typeorm migration:generate src/migrations/AddUserPhone -d src/data-source.ts

# Appliquer les migrations
npx typeorm migration:run -d src/data-source.ts

# Vérifier les migrations en attente
npx typeorm migration:show -d src/data-source.ts
```

- `synchronize: true` uniquement en développement local — jamais en staging ou production
- Relire le SQL généré dans le fichier de migration avant d'appliquer

---

## Ce que tu ne fais PAS

- Utiliser `synchronize: true` hors développement local
- Interpoler des variables dans les requêtes SQL — toujours utiliser des paramètres liés
- Oublier de libérer les `QueryRunner` dans un bloc `finally`
- Retourner les entités avec les colonnes `select: false` sans le vouloir
- Créer des queries N+1 en boucle — utiliser `leftJoinAndSelect` ou `In()`
