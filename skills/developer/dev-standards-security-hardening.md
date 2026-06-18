---
name: dev-standards-security-hardening
description: Standards d'implémentation sécurisée avancés — CORS, CSP, headers HTTP, hashing, chiffrement, gestion des sessions et tokens, rate limiting, hardening applicatif.
---

# Skill — Standards de Sécurité Hardening

## Rôle

Ce skill couvre l'implémentation des mécanismes de sécurité applicative avancés.
Il complète `dev-standards-security.md` (pratiques préventives) en fournissant
les patterns d'implémentation concrets pour le durcissement applicatif.

Pour l'audit de la sécurité existante, utiliser `auditor` (domaine security).

---

## Headers HTTP de sécurité

Chaque application web expose des headers de sécurité explicites :

```
Content-Security-Policy: default-src 'self'; script-src 'self'; object-src 'none'
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), camera=(), microphone=()
```

- **CSP** : définir une politique stricte dès le départ — ne pas utiliser `unsafe-inline` ni `unsafe-eval`
- **HSTS** : activer en production uniquement (bloquant si mal configuré en dev)
- **X-Frame-Options** : `DENY` par défaut sauf iframe explicitement requise
- Utiliser un middleware centralisé (helmet.js, SecurityHeadersBundle, etc.)

---

## CORS

```typescript
// ✅ Configuration explicite et restrictive
const corsOptions = {
  origin: process.env.ALLOWED_ORIGINS?.split(',') ?? [],
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'],
  allowedHeaders: ['Content-Type', 'Authorization'],
  credentials: true,
  maxAge: 86400, // 24h de mise en cache du preflight
}
```

- Jamais de `origin: '*'` avec `credentials: true` — interdit par la spec
- Lister explicitement les origines autorisées — pas de wildcard en production
- Restreindre les méthodes et headers au strict nécessaire
- Tester les preflights en développement

---

## Hashing des mots de passe

```typescript
// ✅ bcrypt avec cost factor adapté
import bcrypt from 'bcrypt'

const SALT_ROUNDS = 12 // minimum 10, recommandé 12-14 en 2024

async function hashPassword(plain: string): Promise<string> {
  return bcrypt.hash(plain, SALT_ROUNDS)
}

async function verifyPassword(plain: string, hash: string): Promise<boolean> {
  return bcrypt.compare(plain, hash)
}
```

- **bcrypt** (ou argon2id) uniquement — jamais MD5, SHA-1, SHA-256 seuls pour les mots de passe
- Cost factor adapté au matériel cible — viser ~100-300ms par hash
- Jamais stocker un mot de passe en clair, même temporairement
- Jamais logger un mot de passe, même hashé

---

## Tokens JWT

```typescript
// ✅ Bonnes pratiques JWT
const ACCESS_TOKEN_TTL = '15m'  // courte durée
const REFRESH_TOKEN_TTL = '7d'  // longue durée, rotation obligatoire

// Algorithme asymétrique recommandé en production
const token = jwt.sign(payload, privateKey, {
  algorithm: 'RS256',
  expiresIn: ACCESS_TOKEN_TTL,
  issuer: process.env.JWT_ISSUER,
  audience: process.env.JWT_AUDIENCE,
})
```

- Valider `iss`, `aud`, `exp` à chaque vérification
- Access token court (≤ 15 min) + refresh token long avec rotation
- Stocker les refresh tokens côté serveur (DB ou Redis) — révocation possible
- Invalider les tokens compromis via une liste de révocation ou un `jti` en base
- Algorithme asymétrique (RS256/ES256) recommandé si les tokens sont vérifiés par des tiers
- Jamais stocker un JWT dans `localStorage` si XSS est possible — préférer `httpOnly cookie`

---

## Sessions

- `httpOnly: true`, `secure: true` (HTTPS), `sameSite: 'strict'` ou `'lax'`
- Régénérer l'ID de session après authentification (protection fixation de session)
- TTL explicite côté serveur — ne pas se fier uniquement à l'expiration côté client
- Invalider la session côté serveur à la déconnexion

```typescript
// ✅ Cookie de session sécurisé
res.cookie('session', token, {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  sameSite: 'strict',
  maxAge: 3600 * 1000, // 1h
})
```

---

## Rate limiting

```typescript
// ✅ Rate limiting par IP + par utilisateur authentifié
import rateLimit from 'express-rate-limit'

// Endpoints sensibles (login, reset password, register)
export const authLimiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 min
  max: 10,
  standardHeaders: true,
  legacyHeaders: false,
  message: { error: { code: 'RATE_LIMIT_EXCEEDED', message: 'Trop de tentatives' } },
})

// API générale
export const apiLimiter = rateLimit({
  windowMs: 60 * 1000, // 1 min
  max: 100,
})
```

- Appliquer sur tous les endpoints publics — priorité aux endpoints d'authentification
- Rate limiting par IP en premier lieu, par utilisateur authentifié si applicable
- Retourner `429 Too Many Requests` avec `Retry-After` header
- En production, utiliser Redis pour partager le compteur entre instances

---

## Chiffrement des données sensibles

```typescript
// ✅ Chiffrement symétrique AES-256-GCM
import crypto from 'crypto'

const ALGORITHM = 'aes-256-gcm'
const KEY = Buffer.from(process.env.ENCRYPTION_KEY!, 'hex') // 32 bytes

function encrypt(plaintext: string): { iv: string; tag: string; data: string } {
  const iv = crypto.randomBytes(16)
  const cipher = crypto.createCipheriv(ALGORITHM, KEY, iv)
  const encrypted = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()])
  return {
    iv: iv.toString('hex'),
    tag: cipher.getAuthTag().toString('hex'),
    data: encrypted.toString('hex'),
  }
}
```

- **AES-256-GCM** pour le chiffrement symétrique (authentifié — détecte la falsification)
- IV (nonce) aléatoire unique par opération — jamais réutilisé avec la même clé
- Clé de chiffrement dans les variables d'environnement, jamais dans le code
- Rotation des clés planifiée — prévoir le déchiffrement avec l'ancienne clé pendant la transition

---

## Validation et désérialisation

- Toujours valider la taille maximale des payloads (body parser avec limite)
- Pas de désérialisation d'objets depuis des sources non fiables (pickle Python, Java serialize)
- Valider les fichiers uploadés : type MIME réel (magic bytes), pas seulement l'extension
- Stocker les uploads en dehors du webroot — jamais servir directement sans validation

```typescript
// ✅ Limite de taille des requêtes
app.use(express.json({ limit: '1mb' }))
app.use(express.urlencoded({ extended: true, limit: '1mb' }))
```

---

## Ce que tu ne fais PAS

- Implémenter un algorithme cryptographique maison — utiliser les bibliothèques éprouvées
- Désactiver les vérifications de certificat SSL en production (`rejectUnauthorized: false`)
- Introduire une dépendance de sécurité sans vérification préalable (`npm audit`)
- Merger un changement de configuration de sécurité sans review explicite
- Effectuer un audit de sécurité — c'est le rôle de `auditor` (domaine security)
