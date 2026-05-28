# Figma MCP Server

MCP Server pour l'intégration Figma avec OpenCode.

## Fonctionnalités (v1)

- **search_figma_files** : Recherche de fichiers Figma par nom
- **get_file_structure** : Récupération de la structure d'un fichier (frames, composants)
- **detect_ui_signals** : Détection automatique de signaux UX/UI

## Configuration

Variables d'environnement requises (gérées dans `~/.config/opencode/config.json`) :

- `FIGMA_PERSONAL_ACCESS_TOKEN` : Personal Access Token Figma
- `FIGMA_TEAM_ID` : ID de votre team Figma

### Obtenir un token Figma

1. Aller sur https://www.figma.com/developers/api#authentication
2. Section "Personal access tokens"
3. Générer un nouveau token avec scopes : `file:read`, `projects:read`

### Trouver votre Team ID

1. Aller sur votre team Figma
2. L'ID est dans l'URL : `https://www.figma.com/files/team/123456/...`
3. Copier `123456`

## Développement

```bash
# Installation
npm install

# Build
npm run build

# Watch mode
npm run dev

# Test manuel
FIGMA_PERSONAL_ACCESS_TOKEN=xxx FIGMA_TEAM_ID=xxx npm start
```

## Architecture

```
src/
├── index.ts              ← Entry point MCP server
├── config.ts             ← Configuration (token, base URL)
├── client.ts             ← Wrapper API Figma
└── tools/
    ├── search-files.ts
    ├── get-file-structure.ts
    └── detect-ui-signals.ts
```

## Utilisation

Ce MCP est automatiquement déployé avec les agents via `oc deploy opencode MY-APP`.

Les agents `scout` et `planner` l'utilisent pour enrichir leur analyse contextuelle avec les données Figma.
