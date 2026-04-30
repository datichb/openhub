---
name: dev-standards-docker
description: Standards Docker — Dockerfile multi-stage, utilisateur non-root, Docker Compose, healthchecks, gestion des images et registries.
---

# Skill — Standards Docker

## Rôle

Ce skill définit les bonnes pratiques pour la containerisation avec Docker.
Il complète `dev-standards-devops.md`.

---

## 🔒 Règles absolues

❌ Jamais de secrets, tokens ou credentials dans un Dockerfile ou une image
❌ Jamais de tag `latest` en production — toujours une version épinglée
❌ Jamais de process tournant en `root` dans un conteneur de production
✅ Multi-stage build systématique pour les images de production
✅ `.dockerignore` exhaustif sur tout projet containerisé

---

## Dockerfile

- Image de base épinglée à une version spécifique — pas de `latest`
- Multi-stage build systématique :
  - Stage `builder` : compilation, installation des dépendances de build
  - Stage `production` : uniquement les artefacts nécessaires à l'exécution
- Créer un utilisateur dédié non-root et basculer avec `USER`
- `.dockerignore` exhaustif : exclure les `node_modules`, `.git`, `*.log`, fichiers de dev, `.env`
- Instructions `RUN` regroupées pour minimiser les layers (`&&` avec `\`)
- `COPY` granulaire : copier d'abord les fichiers de dépendances, puis le code source (optimise le cache)

```dockerfile
# ✅ Multi-stage, non-root, version épinglée
FROM node:20.11-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --frozen-lockfile
COPY . .
RUN npm run build

FROM node:20.11-alpine AS production
WORKDIR /app
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder --chown=appuser:appgroup /app/dist ./dist
COPY --from=builder --chown=appuser:appgroup /app/node_modules ./node_modules
USER appuser
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:3000/health || exit 1
CMD ["node", "dist/main.js"]
```

### .dockerignore

```
node_modules/
.git/
.gitignore
*.log
*.md
.env
.env.*
!.env.example
dist/
coverage/
.nyc_output/
```

---

## Docker Compose

- Un fichier `docker-compose.yml` pour le développement local
- Un `docker-compose.override.yml` pour les surcharges locales — non versionné (dans `.gitignore`)
- Les services ont des `healthcheck` définis
- Les volumes de données sont nommés — pas de chemins absolus
- Les réseaux sont explicitement définis — pas de reliance sur le réseau `default`
- Les variables d'environnement sensibles sont injectées depuis `.env` — jamais en dur

```yaml
# ✅ Docker Compose avec healthchecks et réseaux explicites
services:
  app:
    build:
      context: .
      target: production
    ports:
      - "3000:3000"
    depends_on:
      db:
        condition: service_healthy
    environment:
      DATABASE_URL: ${DATABASE_URL}
    networks:
      - backend
    restart: unless-stopped

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: ${POSTGRES_DB}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER}"]
      interval: 5s
      timeout: 5s
      retries: 5
    volumes:
      - db_data:/var/lib/postgresql/data
    networks:
      - backend

volumes:
  db_data:

networks:
  backend:
    driver: bridge
```

---

## Gestion des images et registries

- Taguer les images avec le SHA de commit + tag sémantique si release :
  - `myapp:abc1234` — toujours présent
  - `myapp:v1.2.3` — sur les releases versionnées
  - `myapp:latest` — uniquement en développement local, **jamais en production**
- Scanner les images avant push (Trivy, Grype, Snyk Container)
- Nettoyer régulièrement les images non utilisées (politique de rétention)
- Utiliser un registry privé pour les images propriétaires

---

## Sécurité

- Ne jamais inclure de fichiers `.env` dans l'image — toujours dans `.dockerignore`
- Pas de secrets dans les `ARG` ou `ENV` du Dockerfile (visibles dans l'historique de l'image)
- Utiliser les secrets Docker BuildKit pour les secrets de build si nécessaire :
  ```dockerfile
  RUN --mount=type=secret,id=npmrc,target=/root/.npmrc npm ci
  ```
- Vérifier les vulnérabilités des images de base régulièrement
- Activer `--no-cache` en CI pour forcer la récupération des dernières images de base

---

## Ce que tu ne fais PAS

- Utiliser `latest` comme tag d'image en production
- Faire tourner des processus en `root` dans les conteneurs
- Inclure des secrets dans les images Docker
- Omettre le `.dockerignore`
- Créer des images sans healthcheck pour les services critiques
