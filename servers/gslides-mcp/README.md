# Google Slides MCP Server

Serveur MCP pour l'extraction de branding d'entreprise depuis des templates Google Slides.
Utilisé par le `documentarian` pour générer des présentations Marp aux couleurs de l'entreprise du projet.

## Fonctionnalités

| Tool | Description |
|------|-------------|
| `get_template_branding` | Extrait couleurs, polices et CSS Marp depuis un template Google Slides |
| `list_presentations` | Liste les présentations accessibles par le Service Account |

## Prérequis

- Compte Google Cloud avec l'**API Google Slides** et l'**API Google Drive** activées
- Un **Service Account** avec une clé JSON
- Le template Google Slides **partagé en lecture** avec l'email du Service Account

## Configuration

### 1. Créer un Service Account

```bash
# Via Google Cloud CLI
gcloud iam service-accounts create opencode-slides \
  --description="OpenCode — lecture des templates Google Slides" \
  --display-name="opencode-slides"

# Télécharger la clé JSON
gcloud iam service-accounts keys create sa-key.json \
  --iam-account=opencode-slides@<PROJECT_ID>.iam.gserviceaccount.com

# Encoder en base64 (valeur à configurer dans oc gslides setup)
base64 -i sa-key.json | tr -d '\n'
```

### 2. Activer les APIs Google

Dans [Google Cloud Console](https://console.cloud.google.com/apis/library) :
- Activer **Google Slides API**
- Activer **Google Drive API**

### 3. Partager le template avec le Service Account

1. Ouvrir le template dans Google Slides
2. Cliquer sur "Partager"
3. Ajouter l'email du SA (`opencode-slides@<project>.iam.gserviceaccount.com`)
4. Rôle : **Lecteur** (lecture seule suffit)

### 4. Configurer via oc CLI

```bash
# Configuration globale (SA partagé entre tous les projets)
oc gslides setup

# Configuration par projet (template spécifique à ce client)
oc gslides setup --project <project-id>

# Vérifier l'accès
oc gslides status
oc gslides status --project <project-id>
```

## Variables d'environnement

| Variable | Requis | Description |
|----------|--------|-------------|
| `GOOGLE_SERVICE_ACCOUNT_KEY` | ✅ | Contenu JSON du SA key encodé en **base64** |
| `GOOGLE_SLIDES_TEMPLATE_ID` | ✗ | ID du template par défaut (optionnel — peut être fourni à la demande) |
| `GOOGLE_SLIDES_TIMEOUT` | ✗ | Timeout en ms (défaut : `30000`) |
| `GOOGLE_SLIDES_MAX_RETRIES` | ✗ | Tentatives en cas d'erreur réseau (défaut : `2`) |

L'ID du template se trouve dans l'URL Google Slides :
```
https://docs.google.com/presentation/d/{GOOGLE_SLIDES_TEMPLATE_ID}/edit
```

## Utilisation (via agent documentarian)

Le `documentarian` appelle automatiquement `get_template_branding` si le MCP est configuré.
Le CSS retourné est injecté dans le frontmatter Marp :

```markdown
---
marp: true
paginate: true
style: |
  section {
    background: #1a1a2e;
    color: #ffffff;
    font-family: 'Montserrat', Arial, sans-serif;
  }
  h1, h2, h3 {
    color: #e94560;
  }
---
```

## Développement

```bash
cd servers/gslides-mcp

# Installer les dépendances
npm install

# Compiler TypeScript → dist/
npm run build

# Lancer les tests
npm test

# Mode watch (tests)
npm run test:watch

# Vérification TypeScript sans compilation
npm run typecheck
```

## Architecture

```
src/
├── index.ts                    ← Entry point MCP server
├── config.ts                   ← Lecture env vars + validation SA key
├── client.ts                   ← GoogleAuth (SA) + axios + extractBranding()
├── tools/
│   ├── get-template-branding.ts  ← Outil principal
│   └── list-presentations.ts     ← Outil de découverte
└── tests/
    ├── config.test.ts
    ├── client.test.ts
    └── tools/
        ├── get-template-branding.test.ts
        └── list-presentations.test.ts
```

## Déploiement

```bash
# Depuis la racine du hub
bash scripts/build-mcp.sh gslides-mcp
oc deploy --project <project-id>
```

## Limitations connues

- Avec un Service Account, seules les présentations **partagées avec le SA** sont accessibles
- L'extraction du branding est basée sur `masters[0]` — les thèmes multi-masters ne sont pas gérés
- Pour les présentations dans un Google Workspace avec des restrictions de partage externe, contacter l'admin Google Workspace pour autoriser le partage avec des comptes de service externes
