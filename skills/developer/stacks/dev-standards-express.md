---
name: dev-standards-express
description: Standards Express / Fastify — routing, middleware, validation, gestion des erreurs, structure du projet et bonnes pratiques.
---

# Skill — Standards Express / Fastify

## Rôle

Ce skill définit les bonnes pratiques pour le développement backend avec Express.js
ou Fastify.
Il complète `dev-standards-universal.md`, `dev-standards-backend.md` et
`dev-standards-api.md`.

---

## 🔒 Règles absolues

❌ Jamais de logique métier dans les handlers de route — déléguer aux services
❌ Jamais de secrets en dur dans le code — utiliser les variables d'environnement
❌ Jamais de `next(err)` silencieux sans logger l'erreur
✅ Toute entrée externe est validée avant traitement

---

## Structure du projet

```
src/
├── app.ts                  ← création et configuration de l'app
├── server.ts               ← démarrage du serveur (listen)
├── config/                 ← chargement et validation de la config
├── middleware/             ← middleware globaux (auth, logger, cors)
├── routes/                 ← définition des routes par domaine
│   └── users/
│       ├── users.router.ts
│       ├── users.handler.ts    ← handlers (équivalent controller)
│       ├── users.service.ts
│       └── users.schema.ts     ← schémas de validation (zod, joi, etc.)
├── services/               ← logique métier partagée
├── repositories/           ← accès aux données
└── errors/                 ← classes d'erreurs custom
```

---

## Routing

```typescript
// ✅ Router organisé par domaine
import { Router } from 'express'
import { authenticate } from '../middleware/auth'
import { validateBody } from '../middleware/validate'
import { createUserSchema, updateUserSchema } from './users.schema'
import * as handler from './users.handler'

export const usersRouter = Router()

usersRouter.get('/:id', authenticate, handler.findOne)
usersRouter.post('/', authenticate, validateBody(createUserSchema), handler.create)
usersRouter.patch('/:id', authenticate, validateBody(updateUserSchema), handler.update)
usersRouter.delete('/:id', authenticate, handler.remove)
```

---

## Handlers

- Les handlers reçoivent, valident (via middleware) et délèguent au service
- Pas de logique métier dans les handlers
- Réponses typées et cohérentes

```typescript
// ✅ Handler mince
export const findOne: RequestHandler = async (req, res, next) => {
  try {
    const user = await usersService.findOneOrThrow(req.params.id)
    res.json({ data: user })
  } catch (err) {
    next(err)
  }
}

export const create: RequestHandler = async (req, res, next) => {
  try {
    const user = await usersService.create(req.body)
    res.status(201).json({ data: user })
  } catch (err) {
    next(err)
  }
}
```

---

## Validation des inputs

```typescript
// ✅ Middleware de validation avec zod
import { z } from 'zod'
import type { RequestHandler } from 'express'

export function validateBody(schema: z.ZodSchema): RequestHandler {
  return (req, res, next) => {
    const result = schema.safeParse(req.body)
    if (!result.success) {
      res.status(422).json({
        error: {
          code: 'VALIDATION_ERROR',
          details: result.error.flatten(),
        },
      })
      return
    }
    req.body = result.data
    next()
  }
}

// ✅ Schéma de validation
export const createUserSchema = z.object({
  email: z.string().email(),
  name: z.string().min(2).max(100),
  password: z.string().min(8),
})
```

---

## Gestion des erreurs

```typescript
// ✅ Classe d'erreur métier
export class AppError extends Error {
  constructor(
    public readonly statusCode: number,
    public readonly code: string,
    message: string,
  ) {
    super(message)
    this.name = 'AppError'
  }
}

export class NotFoundError extends AppError {
  constructor(resource: string, id: string) {
    super(404, 'NOT_FOUND', `${resource} introuvable : ${id}`)
  }
}

// ✅ Error handler global — toujours en dernier middleware
export const errorHandler: ErrorRequestHandler = (err, req, res, _next) => {
  logger.error({ err, path: req.path, method: req.method }, 'Erreur non gérée')

  if (err instanceof AppError) {
    res.status(err.statusCode).json({
      error: { code: err.code, message: err.message, requestId: req.id },
    })
    return
  }

  res.status(500).json({
    error: { code: 'INTERNAL_ERROR', message: 'Une erreur interne est survenue', requestId: req.id },
  })
}
```

---

## Middleware essentiels

```typescript
// ✅ Configuration de l'app avec middleware essentiels
import express from 'express'
import helmet from 'helmet'
import cors from 'cors'
import { requestId } from './middleware/request-id'
import { requestLogger } from './middleware/logger'

export function createApp() {
  const app = express()

  app.use(helmet())                     // headers de sécurité
  app.use(cors({ origin: config.CORS_ORIGIN }))
  app.use(express.json({ limit: '1mb' }))
  app.use(requestId)                    // injecter un ID unique dans chaque requête
  app.use(requestLogger)                // logger structuré

  app.use('/api/v1/users', usersRouter)
  app.use('/api/v1/products', productsRouter)

  app.use(errorHandler)                 // toujours en dernier
  return app
}
```

---

## Fastify — spécificités

Si le projet utilise Fastify plutôt qu'Express, les mêmes principes s'appliquent avec les adaptations suivantes :

```typescript
// ✅ Route Fastify avec schéma de validation JSON Schema
const createUserSchema = {
  body: {
    type: 'object',
    required: ['email', 'name'],
    properties: {
      email: { type: 'string', format: 'email' },
      name: { type: 'string', minLength: 2 },
    },
  },
} as const

fastify.post('/users', { schema: createUserSchema }, async (request, reply) => {
  const user = await usersService.create(request.body)
  return reply.status(201).send({ data: user })
})
```

- Utiliser les schémas JSON Schema intégrés pour la validation et la sérialisation
- `fastify-plugin` pour les plugins réutilisables
- `pino` (inclus dans Fastify) pour le logging structuré

---

## Ce que tu ne fais PAS

- Mettre de la logique métier dans les handlers de route
- Ignorer les erreurs avec `next()` sans les logger
- Omettre le middleware `helmet` et `cors`
- Utiliser `req.body` sans validation préalable
- Créer un seul fichier `routes.ts` monolithique — organiser par domaine
