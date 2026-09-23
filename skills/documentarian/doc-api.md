---
name: doc-api
description: Documentation d'API — détection de l'existant, format OpenAPI 3.x, contrats d'interface, documentation des breaking changes, guide d'utilisation narratif.
---

# Skill — Documentation API

## Étape -1 — Détection monorepo

Avant toute action, vérifier si le dépôt est un monorepo :

```bash
ls pnpm-workspace.yaml lerna.json nx.json turbo.json 2>/dev/null
ls packages/*/package.json apps/*/package.json 2>/dev/null
cat Cargo.toml 2>/dev/null | head -5  # [workspace] section
```

**Si monorepo détecté :**
1. Identifier le package cible de la tâche en cours (depuis le contexte, le ticket, ou en demandant à l'utilisateur)
2. Définir `$PROJECT_DIR` = chemin du package (ex: `packages/api-gateway/`)
3. Scoper toutes les recherches et chemins à `$PROJECT_DIR` au lieu de la racine du dépôt

**Si pas de monorepo :** continuer normalement avec la racine comme scope.

---

## Étape 0 — Détecter l'existant

Avant de documenter ou modifier une spec API :

```bash
# Spec OpenAPI / Swagger
find . -name "openapi.yaml" -o -name "openapi.json" \
       -o -name "swagger.yaml" -o -name "swagger.json" 2>/dev/null

# Autres emplacements courants
ls docs/api/ 2>/dev/null
ls api/ 2>/dev/null
ls .api/ 2>/dev/null

# Annotations dans le code (JSDoc, PHPDoc, Python docstrings)
grep -r "@swagger" src/ 2>/dev/null | head -5
grep -r "@openapi" src/ 2>/dev/null | head -5

# Version OpenAPI utilisée
head -5 openapi.yaml 2>/dev/null
```

Lire la spec existante avant toute modification. S'adapter à la version et au style en place.

---

## Structure OpenAPI 3.x — référence

### Squelette minimal

```yaml
openapi: 3.1.0
info:
  title: Nom de l'API
  version: 1.0.0
  description: |
    Description courte de ce que fait l'API.

    ## Authentification
    Cette API utilise JWT Bearer tokens. Voir [/auth/login](#tag/Auth/operation/login).

servers:
  - url: https://api.exemple.com/v1
    description: Production
  - url: http://localhost:3000/v1
    description: Développement local

tags:
  - name: Auth
    description: Authentification et gestion des tokens
  - name: Users
    description: Gestion des utilisateurs

paths:
  /auth/login:
    post:
      tags: [Auth]
      summary: Connexion utilisateur
      # ...

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT

security:
  - bearerAuth: []
```

### Documentation d'un endpoint

```yaml
/users/{id}:
  get:
    tags: [Users]
    summary: Récupérer un utilisateur
    description: |
      Retourne les informations publiques d'un utilisateur.
      Les champs `email` et `phone` ne sont retournés que pour l'utilisateur
      authentifié lui-même ou un administrateur.
    operationId: getUserById
    parameters:
      - name: id
        in: path
        required: true
        description: Identifiant unique de l'utilisateur (UUID v4)
        schema:
          type: string
          format: uuid
          example: "550e8400-e29b-41d4-a716-446655440000"
    responses:
      "200":
        description: Utilisateur trouvé
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/User'
            example:
              id: "550e8400-e29b-41d4-a716-446655440000"
              name: "Alice Dupont"
              email: "alice@exemple.com"
              createdAt: "2024-01-15T10:30:00Z"
      "404":
        description: Utilisateur non trouvé
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Error'
            example:
              code: "USER_NOT_FOUND"
              message: "Aucun utilisateur avec l'ID fourni"
      "401":
        $ref: '#/components/responses/Unauthorized'
    security:
      - bearerAuth: []
```

### Schemas réutilisables

```yaml
components:
  schemas:
    User:
      type: object
      required: [id, name, createdAt]
      properties:
        id:
          type: string
          format: uuid
          description: Identifiant unique
          readOnly: true
        name:
          type: string
          minLength: 1
          maxLength: 100
          description: Nom complet de l'utilisateur
        email:
          type: string
          format: email
          description: Adresse email — visible uniquement par l'utilisateur lui-même
        createdAt:
          type: string
          format: date-time
          description: Date de création en ISO 8601 (UTC)
          readOnly: true

    Error:
      type: object
      required: [code, message]
      properties:
        code:
          type: string
          description: Code d'erreur machine-readable
          example: "VALIDATION_ERROR"
        message:
          type: string
          description: Message d'erreur lisible par un humain
        details:
          type: array
          items:
            type: object
          description: Détails supplémentaires (erreurs de validation par champ)

  responses:
    Unauthorized:
      description: Token manquant ou invalide
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
          example:
            code: "UNAUTHORIZED"
            message: "Token JWT manquant ou expiré"

    Forbidden:
      description: Accès refusé — authentifié mais sans les droits nécessaires
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/Error'
```

---

## Codes HTTP — utilisation correcte

| Code | Signification | Quand l'utiliser |
|------|--------------|-----------------|
| `200` | OK | Lecture réussie, mise à jour réussie |
| `201` | Created | Création réussie — inclure `Location: /resource/id` |
| `204` | No Content | Suppression réussie, action sans corps de réponse |
| `400` | Bad Request | Validation échouée, paramètres manquants ou invalides |
| `401` | Unauthorized | Non authentifié (token absent ou expiré) |
| `403` | Forbidden | Authentifié mais sans les droits |
| `404` | Not Found | Ressource inexistante |
| `409` | Conflict | Conflit d'état (doublon, version obsolète) |
| `422` | Unprocessable Entity | Données syntaxiquement correctes mais sémantiquement invalides |
| `429` | Too Many Requests | Rate limiting — inclure `Retry-After` |
| `500` | Internal Server Error | Erreur serveur non anticipée |

**Règle :** ne jamais retourner `200` avec un champ `success: false` dans le corps.
Utiliser le code HTTP approprié.

---

## Documentation narrative (au-delà de la spec)

La spec OpenAPI documente le **contrat technique**. Un guide narratif explique
**comment utiliser l'API** : flux d'authentification, cas d'usage types, pagination,
gestion des erreurs.

### Structure type — Guide d'utilisation API

```markdown
# Guide d'utilisation — API [Nom]

## Authentification

1. Obtenir un token via `POST /auth/login`
2. Inclure le token dans toutes les requêtes : `Authorization: Bearer <token>`
3. Les tokens expirent après 24h — rafraîchir via `POST /auth/refresh`

### Exemple

```bash
# 1. Se connecter
curl -X POST https://api.exemple.com/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@exemple.com", "password": "secret"}'

# Réponse
{ "token": "eyJ...", "expiresAt": "2024-01-16T10:30:00Z" }

# 2. Utiliser le token
curl https://api.exemple.com/v1/users/me \
  -H "Authorization: Bearer eyJ..."
```

## Pagination

La pagination utilise le curseur (cursor-based) pour les listes.

```bash
# Première page (20 résultats par défaut)
GET /users?limit=20

# Page suivante (utiliser le cursor retourné)
GET /users?limit=20&cursor=eyJpZCI6MTB9
```

Réponse paginée :
```json
{
  "data": [...],
  "pagination": {
    "cursor": "eyJpZCI6MzB9",
    "hasMore": true,
    "total": 150
  }
}
```

## Gestion des erreurs

Toutes les erreurs suivent le format :
```json
{ "code": "ERROR_CODE", "message": "Description lisible", "details": [...] }
```

| Code | Cause fréquente | Solution |
|------|----------------|---------|
| `UNAUTHORIZED` | Token expiré | Rafraîchir le token |
| `VALIDATION_ERROR` | Champ manquant | Vérifier `details` pour le champ concerné |
| `RATE_LIMITED` | Trop de requêtes | Attendre `Retry-After` secondes |
```

---

## Documentation des breaking changes

Un breaking change est toute modification qui casse les clients existants.

### Identifier un breaking change

| Changement | Breaking ? |
|-----------|-----------|
| Supprimer un endpoint | Oui |
| Supprimer un champ de réponse | Oui |
| Changer le type d'un champ | Oui |
| Rendre un champ obligatoire dans la requête | Oui |
| Changer la sémantique d'un code HTTP | Oui |
| Ajouter un champ optionnel dans la réponse | Non |
| Ajouter un endpoint | Non |
| Rendre un champ optionnel (était obligatoire) | Non |
| Améliorer un message d'erreur | Non |

### Documenter un breaking change dans la spec

```yaml
/users/{id}:
  delete:
    summary: Supprimer un utilisateur
    description: |
      **⚠️ Breaking change v2.0** : cet endpoint retourne désormais `204 No Content`
      au lieu de `200 OK` avec le corps `{ "deleted": true }`.
      
      Migrer les clients qui lisaient le corps de réponse.
    deprecated: false
```

### Documenter la migration dans le CHANGELOG

```markdown
## [2.0.0] — 2024-03-01

### Breaking Changes

- **DELETE /users/{id}** : retourne maintenant `204 No Content` (était `200 OK`)
  — supprimer la lecture du corps de réponse dans les clients
- **GET /users** : le champ `total` a été renommé `count`
  — mettre à jour les références à `response.total`

### Migration depuis v1.x

Voir [guide de migration](docs/api/migration-v1-to-v2.md).
```

---

## Checklist avant de livrer une documentation API

- [ ] Tous les endpoints ont un `summary` et une `description`
- [ ] Tous les paramètres ont une `description` et un `example`
- [ ] Toutes les réponses possibles sont documentées (200, 4xx, 5xx)
- [ ] Les schemas `$ref` sont utilisés pour éviter la duplication
- [ ] Le flux d'authentification est documenté avec un exemple `curl`
- [ ] Les breaking changes sont identifiés et documentés
- [ ] La pagination est documentée si applicable
- [ ] Les rate limits sont documentés si applicable
