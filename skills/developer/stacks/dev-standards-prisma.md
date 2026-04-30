---
name: dev-standards-prisma
description: Standards Prisma — schema, migrations, client, relations, transactions, performances et bonnes pratiques.
---

# Skill — Standards Prisma

## Rôle

Ce skill définit les bonnes pratiques pour l'accès aux données avec Prisma ORM.
Il complète `dev-standards-typescript.md` et `dev-standards-backend.md`.

---

## 🔒 Règles absolues

❌ Jamais de `prisma db push` en production — utiliser les migrations (`prisma migrate deploy`)
❌ Jamais d'exposition du `PrismaClient` directement dans les controllers
❌ Jamais de `deleteMany({})` ou `updateMany({})` sans clause `where` explicite
✅ Toute migration destructrice est relue avant d'être appliquée

---

## Schema

```prisma
// ✅ schema.prisma bien structuré
generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
  url      = env("DATABASE_URL")
}

model User {
  id        String   @id @default(uuid())
  email     String   @unique
  name      String
  password  String
  active    Boolean  @default(true)
  createdAt DateTime @default(now())
  updatedAt DateTime @updatedAt

  orders    Order[]

  @@index([email])
  @@index([active, createdAt])
  @@map("users")
}

model Order {
  id        String      @id @default(uuid())
  status    OrderStatus @default(PENDING)
  total     Decimal     @db.Decimal(10, 2)
  userId    String
  createdAt DateTime    @default(now())

  user      User        @relation(fields: [userId], references: [id])

  @@index([userId, createdAt])
  @@map("orders")
}

enum OrderStatus {
  PENDING
  CONFIRMED
  CANCELLED
}
```

- `@@map()` pour contrôler le nom de la table en base
- `@@index()` sur les colonnes filtrées et triées fréquemment
- `@updatedAt` systématique sur les modèles mutables

---

## Client Prisma — singleton

```typescript
// lib/prisma.ts — instance unique
import { PrismaClient } from '@prisma/client'

const globalForPrisma = globalThis as unknown as { prisma: PrismaClient }

export const prisma =
  globalForPrisma.prisma ??
  new PrismaClient({
    log: process.env.NODE_ENV === 'development' ? ['query', 'warn', 'error'] : ['error'],
  })

if (process.env.NODE_ENV !== 'production') globalForPrisma.prisma = prisma
```

---

## Requêtes

```typescript
// ✅ Select explicite — ne pas retourner tous les champs
const user = await prisma.user.findUnique({
  where: { id },
  select: {
    id: true,
    email: true,
    name: true,
    createdAt: true,
    // password exclu
  },
})

// ✅ Pagination cursor-based
const users = await prisma.user.findMany({
  where: { active: true },
  orderBy: { createdAt: 'desc' },
  take: 20,
  cursor: cursor ? { id: cursor } : undefined,
  skip: cursor ? 1 : 0,
})

// ✅ Relations chargées explicitement
const userWithOrders = await prisma.user.findUnique({
  where: { id },
  include: {
    orders: {
      where: { status: 'CONFIRMED' },
      orderBy: { createdAt: 'desc' },
      take: 5,
    },
  },
})
```

---

## Transactions

```typescript
// ✅ Transaction interactive pour les opérations multi-étapes
const result = await prisma.$transaction(async (tx) => {
  const user = await tx.user.create({
    data: { email, name, password: hashedPassword },
  })

  const welcomeCredit = await tx.credit.create({
    data: { userId: user.id, amount: 10, reason: 'welcome' },
  })

  return { user, welcomeCredit }
})

// ✅ Transaction séquentielle (plus performante pour les opérations indépendantes)
const [updatedUser, log] = await prisma.$transaction([
  prisma.user.update({ where: { id }, data: { name } }),
  prisma.auditLog.create({ data: { userId: id, action: 'name_updated' } }),
])
```

---

## Migrations

```bash
# ✅ Workflow de migration
# Développement — génère et applique
npx prisma migrate dev --name "add_user_phone"

# Production — applique uniquement les migrations en attente
npx prisma migrate deploy

# Vérifier l'état des migrations
npx prisma migrate status
```

- Ne jamais utiliser `prisma db push` en production — il ne génère pas de fichier de migration
- Relire le fichier SQL généré dans `prisma/migrations/` avant d'appliquer

---

## Tests

```typescript
// ✅ Mock du PrismaClient avec jest-mock-extended
import { mockDeep, DeepMockProxy } from 'jest-mock-extended'
import { PrismaClient } from '@prisma/client'

// prisma.mock.ts
export const prismaMock = mockDeep<PrismaClient>()

jest.mock('./lib/prisma', () => ({
  prisma: prismaMock,
}))

// users.service.test.ts
it('retourne null si l\'utilisateur n\'existe pas', async () => {
  prismaMock.user.findUnique.mockResolvedValue(null)
  const result = await userService.findById('non-existant')
  expect(result).toBeNull()
})
```

---

## Ce que tu ne fais PAS

- Utiliser `prisma db push` en production
- Retourner tous les champs d'un modèle incluant les mots de passe ou données sensibles
- Créer des queries N+1 en boucle — utiliser `include` ou `findMany` avec `in`
- Omettre les indexes sur les colonnes de filtrage et de tri fréquents
- Partager une instance `PrismaClient` non singleton (overhead de connexions)
