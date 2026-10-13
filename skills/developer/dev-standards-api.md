---
name: dev-standards-api
description: "Utiliser quand le ticket touche une API publique : versioning, pagination, erreurs, rate limiting, idempotence, webhooks."
---

# Skill — Standards API

## Rôle

Ce skill définit les bonnes pratiques spécifiques à la conception et l'implémentation
d'APIs publiques, partenaires ou d'intégration.
Il complète `dev-standards-backend.md` (architecture) et `dev-standards-security.md` (sécurité).

---

## Règles du domaine `api`

- Schema-first (OpenAPI ou GraphQL) avant tout code
- Idempotence sur PUT/DELETE/PATCH — clé d'idempotence sur POST si nécessaire
- Timeout explicite sur tous les appels sortants
- Validation HMAC sur tous les webhooks entrants

## Règles du domaine `fullstack`

- Définir le contrat d'API en premier (types partagés, codes d'erreur)
- Implémenter backend puis frontend dans cet ordre
- Tests aux deux niveaux

---

## Versioning

- **Prefixe d'URL** : `/v1/`, `/v2/` — approche recommandée pour les APIs REST publiques
- **Header** : `Accept-Version: v2` — acceptable pour les APIs internes
- Documenter la politique de support (durée de vie des versions, calendrier de dépréciation)
- Une version dépréciée retourne `Deprecation: true` et `Sunset: <date>` dans les headers
- Ne jamais supprimer une version sans avoir notifié les consommateurs avec un préavis documenté

```
Deprecation: true
Sunset: Sat, 31 Dec 2026 23:59:59 GMT
Link: <https://api.example.com/v2/docs>; rel="successor-version"
```

---

## Pagination

### Cursor-based (recommandé pour les grandes collections)

```json
{
  "data": [...],
  "pagination": {
    "cursor": "eyJpZCI6MTIzfQ==",
    "hasMore": true,
    "limit": 20
  }
}
```

- Stable lors d'insertions concurrentes
- Scalable — ne nécessite pas de `COUNT(*)`
- Cursor opaque (base64) — ne pas exposer le format interne

### Offset/limit (acceptable pour les petites collections)

```json
{
  "data": [...],
  "pagination": {
    "total": 247,
    "offset": 40,
    "limit": 20,
    "next": "/v1/resources?offset=60&limit=20",
    "prev": "/v1/resources?offset=20&limit=20"
  }
}
```

- Inclure `total` pour permettre la navigation par page
- Limiter `limit` à une valeur maximale configurable (ex: 100)
- Valider `offset` et `limit` — rejeter les valeurs négatives ou hors plage

---

## Format de réponse uniforme

### Succès

```json
{
  "data": { ... },
  "meta": {
    "requestId": "req_abc123",
    "timestamp": "2026-03-30T10:00:00Z"
  }
}
```

### Erreur

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Données de requête invalides",
    "details": [
      { "field": "email", "message": "Format invalide", "value": "not-an-email" }
    ],
    "requestId": "req_abc123"
  }
}
```

- `code` : constante machine-readable (snake_case ou SCREAMING_SNAKE_CASE) — stable entre versions
- `message` : description humaine lisible — peut évoluer
- `details` : liste des erreurs de validation champ par champ
- `requestId` : identifiant de corrélation pour les logs

---

## Codes HTTP sémantiques

| Situation | Code |
|-----------|------|
| Succès, retourne une ressource | 200 |
| Création réussie | 201 |
| Action réussie sans contenu retourné | 204 |
| Requête invalide (validation) | 400 |
| Non authentifié | 401 |
| Authentifié mais non autorisé | 403 |
| Ressource non trouvée | 404 |
| Conflit (doublon, état incompatible) | 409 |
| Entité non traitable (validation sémantique) | 422 |
| Trop de requêtes | 429 |
| Erreur serveur interne | 500 |
| Service temporairement indisponible | 503 |

Ne jamais retourner 200 avec un corps contenant `{ success: false }`.

---

## Idempotence

- `GET`, `HEAD`, `OPTIONS`, `DELETE` : idempotents par définition
- `PUT` : idempotent — appliquer plusieurs fois produit le même résultat
- `PATCH` : idempotent si la modification est absolue (set), non idempotent si relative (increment)
- `POST` : non idempotent par défaut — utiliser une clé d'idempotence pour les opérations critiques

```
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

```
// ✅ Gestion de la clé d'idempotence (pseudocode)
function createOrder(dto, idempotencyKey):
  existing = cache.get("idem:" + idempotencyKey)
  if existing: return existing  // replay de la réponse précédente

  result = orderService.create(dto)
  cache.set("idem:" + idempotencyKey, result, ttl: 86400)
  return result
```

---

## Contrat d'API (schema-first)

- Définir le contrat **avant** d'implémenter (schema-first)
- Chaque endpoint a une description, des paramètres typés et des réponses documentées
- Réutiliser les schémas communs par référence — pas de copier-coller
- Les spécificités du format de contrat (OpenAPI, GraphQL SDL, AsyncAPI, etc.) sont définies dans le skill dédié à la stack du projet

---

## Breaking changes

Un breaking change est toute modification qui casse un consommateur existant :

| Type | Exemples |
|------|---------|
| Suppression | Champ supprimé, endpoint supprimé, valeur d'enum retirée |
| Renommage | Champ renommé, endpoint renommé |
| Changement de type | `string` → `number`, `object` → `array` |
| Contrainte plus stricte | Validation ajoutée, champ devenu obligatoire |
| Comportement modifié | Sémantique d'un code HTTP changée |

**Règle :** tout breaking change nécessite une nouvelle version majeure.

**Évolutions non-breaking (rétrocompatibles) :**
- Ajout d'un champ optionnel en réponse
- Ajout d'un endpoint
- Ajout d'une valeur d'enum (si les consommateurs ignorent les valeurs inconnues)
- Contrainte plus souple (champ optionnel devient nullable)

---

## Webhooks sortants

```
// ✅ Webhook sécurisé et fiable (pseudocode)
function sendWebhook(url, payload, secret):
  body      = serialize(payload)
  timestamp = now().toString()
  signature = hmac_sha256(secret, timestamp + "." + body)

  http.post(url,
    headers: {
      "Content-Type": "application/json",
      "X-Webhook-Timestamp": timestamp,
      "X-Webhook-Signature": "sha256=" + signature,
    },
    body: body,
    timeout: 5s
  )
```

- Signer les payloads (HMAC-SHA256) — permettre aux consommateurs de vérifier l'authenticité
- Timeout explicite sur chaque appel
- Retry avec backoff exponentiel sur les erreurs 5xx (pas sur les 4xx)
- Logger le résultat de chaque tentative d'envoi
- Exposer un endpoint de test pour que les consommateurs puissent valider leur intégration

---

## Rate limiting (côté API exposée)

- Retourner les headers standards :
  ```
  X-RateLimit-Limit: 100
  X-RateLimit-Remaining: 42
  X-RateLimit-Reset: 1711790400
  Retry-After: 60
  ```
- Différencier les limites par tier (free, pro, enterprise)
- Documenter les limites dans la documentation d'intégration

---

## Ce que tu ne fais PAS

- Modifier un contrat d'API existant sans versionner
- Retourner des données non nécessaires dans les réponses (over-fetching)
- Exposer des IDs internes séquentiels — utiliser des UUID ou des identifiants opaques
- Implémenter une auth maison — déléguer à un middleware d'authentification éprouvé
- Livrer un endpoint sans contrat à jour
