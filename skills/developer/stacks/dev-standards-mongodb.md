---
name: dev-standards-mongodb
description: Standards MongoDB / Mongoose — schemas, indexes, validation, agrégations, transactions et bonnes pratiques.
---

# Skill — Standards MongoDB / Mongoose

## Rôle

Ce skill définit les bonnes pratiques pour l'accès aux données avec MongoDB via Mongoose.
Il complète `dev-standards-backend.md`.

---

## 🔒 Règles absolues

❌ Jamais de `deleteMany({})` ou `updateMany({})` sans filtre — risque de suppression totale
❌ Jamais de données sensibles (passwords, tokens) retournées dans les requêtes par défaut
❌ Jamais d'opérateurs `$where` avec des chaînes — risque d'injection
✅ Tout schema a une validation explicite des champs obligatoires

---

## Schemas Mongoose

```typescript
// models/user.model.ts
import { Schema, model, Document, Types } from 'mongoose'

export interface IUser extends Document {
  _id: Types.ObjectId
  name: string
  email: string
  password: string
  active: boolean
  createdAt: Date
  updatedAt: Date
}

const userSchema = new Schema<IUser>(
  {
    name: {
      type: String,
      required: [true, 'Le nom est obligatoire'],
      trim: true,
      minlength: 2,
      maxlength: 100,
    },
    email: {
      type: String,
      required: [true, "L'email est obligatoire"],
      unique: true,
      lowercase: true,
      trim: true,
      match: [/^\S+@\S+\.\S+$/, 'Format email invalide'],
    },
    password: {
      type: String,
      required: true,
      select: false,  // exclu des requêtes par défaut
      minlength: 8,
    },
    active: { type: Boolean, default: true, index: true },
  },
  {
    timestamps: true,         // createdAt et updatedAt automatiques
    toJSON: {
      transform: (_doc, ret) => {
        delete ret.password   // sécurité supplémentaire
        return ret
      },
    },
  },
)

// Index composé pour les requêtes fréquentes
userSchema.index({ active: 1, createdAt: -1 })

export const User = model<IUser>('User', userSchema)
```

---

## Requêtes

```typescript
// ✅ Projection explicite — ne pas retourner tous les champs
const user = await User.findById(id).select('name email createdAt').lean()

// ✅ Pagination cursor-based
const users = await User.find(
  { active: true, ...(cursor && { _id: { $lt: cursor } }) },
  { name: 1, email: 1, createdAt: 1 }
)
  .sort({ _id: -1 })
  .limit(20)
  .lean()

// ✅ .lean() pour les lectures — retourne des objets JS purs (plus performant)
// ⚠️ Ne pas utiliser .lean() si les méthodes de document sont nécessaires

// ✅ findOne avec réinclusion du champ select:false
const userWithPassword = await User
  .findOne({ email })
  .select('+password')
```

---

## Indexes

```typescript
// ✅ Indexes définis dans le schema
userSchema.index({ email: 1 }, { unique: true })
userSchema.index({ active: 1, createdAt: -1 })
userSchema.index({ 'address.city': 1 })   // index sur champ imbriqué

// ✅ TTL index pour les données expirantes
sessionSchema.index({ expiresAt: 1 }, { expireAfterSeconds: 0 })

// ✅ Index text pour la recherche full-text
productSchema.index({ name: 'text', description: 'text' })
```

---

## Agrégations

```typescript
// ✅ Pipeline d'agrégation documenté
const stats = await Order.aggregate([
  // Étape 1 : filtrer les commandes récentes
  { $match: { status: 'CONFIRMED', createdAt: { $gte: new Date('2024-01-01') } } },

  // Étape 2 : regrouper par utilisateur
  {
    $group: {
      _id: '$userId',
      totalOrders: { $sum: 1 },
      totalAmount: { $sum: '$amount' },
      avgAmount: { $avg: '$amount' },
    },
  },

  // Étape 3 : filtrer les utilisateurs avec plus de 5 commandes
  { $match: { totalOrders: { $gte: 5 } } },

  // Étape 4 : trier par montant décroissant
  { $sort: { totalAmount: -1 } },

  { $limit: 10 },
])
```

---

## Transactions (MongoDB 4+)

```typescript
// ✅ Transaction multi-documents
const session = await mongoose.startSession()

try {
  await session.withTransaction(async () => {
    const user = await User.create([userData], { session })
    await Credit.create([{ userId: user[0]._id, amount: 10 }], { session })
  })
} finally {
  await session.endSession()
}
```

Les transactions nécessitent un replica set ou MongoDB Atlas.

---

## Middlewares Mongoose

```typescript
// ✅ Middleware pre-save pour le hachage du mot de passe
userSchema.pre('save', async function (next) {
  if (!this.isModified('password')) return next()
  this.password = await bcrypt.hash(this.password, 12)
  next()
})

// ✅ Méthode d'instance custom
userSchema.methods.comparePassword = async function (candidate: string): Promise<boolean> {
  return bcrypt.compare(candidate, this.password)
}
```

---

## Ce que tu ne fais PAS

- Utiliser `deleteMany({})` ou `updateMany({})` sans filtre
- Retourner les champs sensibles par défaut (password, tokens)
- Omettre `.lean()` sur les requêtes de lecture sans besoin de méthodes de document
- Créer des agrégations sans expliquer chaque étape du pipeline
- Omettre les indexes sur les champs de filtre et de tri fréquents
