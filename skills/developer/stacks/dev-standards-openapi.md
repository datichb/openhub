---
name: dev-standards-openapi
description: Standards OpenAPI 3.x — structure du document, $ref, schémas réutilisables, sécurité, génération de code et bonnes pratiques.
---

# Skill — Standards OpenAPI

## Rôle

Ce skill définit les bonnes pratiques pour la rédaction et la maintenance de contrats
d'API au format OpenAPI 3.x.
Il complète `dev-standards-api.md` avec les règles spécifiques au format OpenAPI.

---

## 🔒 Règles absolues

❌ Jamais de schéma copié-collé entre endpoints — utiliser `$ref`
❌ Jamais d'endpoint sans description et sans réponse d'erreur documentée
❌ Jamais de `type: object` sans `properties` définis
✅ Le contrat est défini avant l'implémentation (schema-first)

---

## Structure du document

```yaml
# ✅ Document OpenAPI bien structuré
openapi: 3.1.0
info:
  title: Mon API
  version: 1.0.0
  description: |
    API de gestion des utilisateurs et des commandes.

    ## Authentification
    Toutes les routes (sauf `/auth/login`) nécessitent un Bearer token JWT.

servers:
  - url: https://api.exemple.com/v1
    description: Production
  - url: https://api.staging.exemple.com/v1
    description: Staging

tags:
  - name: users
    description: Gestion des utilisateurs
  - name: orders
    description: Gestion des commandes

security:
  - BearerAuth: []

paths:
  /users/{id}:
    get:
      summary: Récupérer un utilisateur
      tags: [users]
      parameters:
        - $ref: '#/components/parameters/UserId'
      responses:
        '200':
          description: Utilisateur trouvé
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UserResponse'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '403':
          $ref: '#/components/responses/Forbidden'
        '404':
          $ref: '#/components/responses/NotFound'

components:
  # Tout est centralisé dans components — jamais de définition inline répétée
```

---

## Schémas réutilisables

```yaml
# ✅ components/schemas — définitions centralisées
components:
  schemas:
    # UUID réutilisable
    UUID:
      type: string
      format: uuid
      example: "550e8400-e29b-41d4-a716-446655440000"

    # Timestamp réutilisable
    Timestamp:
      type: string
      format: date-time
      example: "2024-01-15T10:30:00Z"

    # Schéma de création
    UserCreate:
      type: object
      required: [email, name, password]
      properties:
        email:
          type: string
          format: email
          example: alice@exemple.com
        name:
          type: string
          minLength: 2
          maxLength: 100
          example: Alice Dupont
        password:
          type: string
          minLength: 8
          writeOnly: true   # jamais retourné en réponse

    # Schéma de réponse — sans password
    UserResponse:
      type: object
      required: [id, email, name, createdAt]
      properties:
        id:
          $ref: '#/components/schemas/UUID'
        email:
          type: string
          format: email
        name:
          type: string
        createdAt:
          $ref: '#/components/schemas/Timestamp'

    # Erreur standard
    ErrorResponse:
      type: object
      required: [error]
      properties:
        error:
          type: object
          required: [code, message]
          properties:
            code:
              type: string
              example: NOT_FOUND
            message:
              type: string
              example: Utilisateur introuvable
            requestId:
              type: string
            details:
              type: array
              items:
                type: object
                properties:
                  field: { type: string }
                  message: { type: string }

    # Pagination cursor-based
    PaginatedUsers:
      type: object
      properties:
        data:
          type: array
          items:
            $ref: '#/components/schemas/UserResponse'
        pagination:
          type: object
          properties:
            cursor:
              type: string
              nullable: true
            hasMore:
              type: boolean
            limit:
              type: integer
```

---

## Réponses réutilisables

```yaml
  responses:
    Unauthorized:
      description: Non authentifié
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
          example:
            error:
              code: UNAUTHORIZED
              message: Token manquant ou invalide

    Forbidden:
      description: Accès non autorisé
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'

    NotFound:
      description: Ressource introuvable
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'

    ValidationError:
      description: Données invalides
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
```

---

## Paramètres réutilisables

```yaml
  parameters:
    UserId:
      name: id
      in: path
      required: true
      schema:
        $ref: '#/components/schemas/UUID'
      description: Identifiant unique de l'utilisateur

    LimitParam:
      name: limit
      in: query
      schema:
        type: integer
        minimum: 1
        maximum: 100
        default: 20

    CursorParam:
      name: cursor
      in: query
      schema:
        type: string
      description: Curseur de pagination opaque
```

---

## Sécurité

```yaml
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: Token JWT obtenu via POST /auth/login
```

---

## Génération de code

- Utiliser des outils de codegen pour générer les types/clients depuis le contrat :
  - TypeScript : `openapi-typescript`, `orval`, `@hey-api/openapi-ts`
  - Python : `openapi-python-client`, `datamodel-code-generator`
  - Java : `openapi-generator`
- Les types générés ne sont jamais modifiés manuellement — modifier le contrat source
- Le contrat est versionné dans le dépôt (`openapi.yaml` ou `docs/api/`)

---

## Ce que tu ne fais PAS

- Copier-coller des schémas — utiliser `$ref` systématiquement
- Documenter un endpoint sans ses réponses d'erreur (401, 404, 422 selon le cas)
- Utiliser `additionalProperties: true` sans justification
- Modifier les types générés manuellement
- Livrer un endpoint sans mettre à jour le contrat OpenAPI
