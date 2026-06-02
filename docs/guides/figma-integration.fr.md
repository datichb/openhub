# Intégration Figma - Guide de démarrage

> 🇬🇧 [Read in English](figma-integration.en.md)

## Vue d'ensemble

L'intégration Figma enrichit les workflows de planification (Scout et Planner) avec le contexte design en interrogeant automatiquement l'API Figma pour détecter les maquettes, composants et signaux UX/UI.

### Fonctionnalités

- **Recherche automatique** de fichiers Figma par nom de feature
- **Détection de signaux UX/UI** : flows multi-étapes, composants visuels, états
- **Ajustement d'estimation** basé sur le nombre de composants détectés
- **Enrichissement automatique** des rapports Scout et plans Planner

---

## Configuration rapide

### 1. Configurer via `oc service`

La méthode recommandée est d'utiliser la commande `oc service setup` qui vous guide interactivement :

```bash
oc service setup figma
# ou via l'alias :
oc figma setup
```

Cette commande va :
1. Vous demander votre **Personal Access Token** Figma
2. Vous demander votre **Team ID**
3. Valider la connexion à l'API Figma
4. Sauvegarder la configuration dans `~/.config/opencode/config.json`
5. Builder automatiquement le serveur MCP si nécessaire

Vérifier l'état à tout moment :
```bash
oc service status figma
# ou :
oc figma status
```

### 2. Configuration manuelle (alternative)

Si vous préférez configurer manuellement, créez `~/.config/opencode/config.json` :

### 1. Obtenir vos tokens Figma

**Personal Access Token :**
1. Aller sur https://www.figma.com/developers/api#authentication
2. Section "Personal access tokens"
3. Créer un token avec scopes : `file:read`, `projects:read`

**Team ID :**
1. Ouvrir votre team Figma
2. L'ID est dans l'URL : `https://www.figma.com/files/team/123456/...`
3. Copier `123456`

Créer `~/.config/opencode/config.json` :

```json
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_xxx",
    "FIGMA_TEAM_ID": "123456"
  }
}
```

### 3. Organiser vos fichiers Figma

Suivre les conventions dans [`config/figma.conventions.md`](../../config/figma.conventions.md) :

- **Nommage** : `[Projet] - [Feature] - [Type]`
- **Tags** : `#feature-xxx`, `#ready-dev`, `#wip`
- **Pages** : Cover, Flows, UI Design, States, Dev Notes

### 4. Déployer

```bash
oc deploy opencode MY-PROJECT
```

Le MCP Server Figma sera déployé automatiquement avec les agents.

---

## Utilisation

### Avec Scout

```bash
> Scout cette feature: tableau de bord utilisateur
```

Le Scout va :
1. Explorer la codebase (workflow normal)
2. Chercher dans Figma : `search_figma_files("tableau de bord")`
3. Analyser les fichiers trouvés : `detect_ui_signals(fileId)`
4. Inclure les données Figma dans son rapport

**Rapport enrichi :**
```markdown
## 🎨 Contexte Figma détecté
- Fichiers : Dashboard - UI (URL Figma)
- Composants : 7 détectés
- Signaux : UX ⚠️ | UI ⚠️
- Complexité ajustée : S → M
```

### Avec Planner

```bash
> Planifie cette feature: processus d'inscription
```

Le Planner va :
1. **Phase 1.2** : Explorer la codebase
2. **Phase 1.3** : Explorer Figma (nouveau)
   - Chercher les maquettes liées
   - Détecter signaux UX/UI automatiquement
3. **Phase 1.5** : Proposer délégation designers si signaux détectés
4. **Phase 5** : Pré-remplir `--design` des tickets avec données Figma

---

## Tools MCP disponibles

### `search_figma_files`

Recherche des fichiers Figma par nom.

```typescript
Input: { query: "dashboard" }
Output: [
  { id: "abc123", name: "MonApp - Dashboard - UI", url: "...", lastModified: "..." }
]
```

### `get_file_structure`

Récupère la structure d'un fichier (frames, composants).

```typescript
Input: { fileId: "abc123" }
Output: {
  frames: [...],
  componentsCount: 7
}
```

### `detect_ui_signals`

Détecte automatiquement les signaux UX/UI et estime la complexité.

```typescript
Input: { fileId: "abc123" }
Output: {
  hasUXSignal: true,
  hasUISignal: true,
  componentsCount: 7,
  complexity: "M",
  reasoning: [...],
  recommendations: [...]
}
```

---

## Architecture

```
opencode-hub/
├── servers/figma-mcp/        ← MCP Server TypeScript
│   ├── src/
│   │   ├── index.ts          ← Entry point
│   │   ├── client.ts         ← Wrapper API Figma
│   │   ├── config.ts         ← Configuration tokens
│   │   └── tools/            ← 3 tools MCP
│   └── dist/                 ← Compilé
├── skills/adapters/
│   ├── figma-scout-protocol.md
│   └── figma-planner-protocol.md
└── scripts/
    ├── build-mcp.sh          ← Build MCP
    ├── check-mcp.sh          ← Vérifie build
    └── lib/mcp-deploy.sh     ← Déploiement
```

---

## Tests

### Test 1 : Scout simple

```bash
# Dans un projet avec maquettes Figma
> Scout cette feature: page paramètres

# Vérifier dans le rapport :
- Section "🎨 Contexte Figma" présente
- URLs Figma valides
- Composants listés
- Estimation ajustée si > 3 composants
```

### Test 2 : Planner avec signaux

```bash
> Planifie cette feature: flow inscription

# Vérifier :
- Phase 1.3 exécutée (exploration Figma)
- Récap Phase 1 contient données Figma
- Phase 1.5 proposée si signaux détectés
- Tickets créés avec --design pré-rempli
```

---

## Dépannage

### Aucun fichier Figma trouvé

**Erreur :** `Aucun fichier Figma trouvé pour la recherche : "xxx"`

**Solutions :**
- Vérifier que le Team ID est correct
- Renommer fichiers Figma selon conventions (`[Projet] - [Feature] - [Type]`)
- Vérifier scopes du token : `file:read`, `projects:read`

### Token non reconnu

**Erreur :** `FIGMA_PERSONAL_ACCESS_TOKEN environment variable is required`

**Solutions :**
- Vérifier que `~/.config/opencode/config.json` existe
- Vérifier syntaxe JSON (virgules, guillemets)
- Redémarrer OpenCode après modification

### Build MCP échoue

```bash
cd servers/figma-mcp
rm -rf node_modules package-lock.json
npm install
npm run build
```

---

## Limitations actuelles (v1)

- ❌ Pas de webhooks (notifications temps réel)
- ❌ Pas de création de commentaires Figma (lecture seule)
- ❌ Pas de liens tickets → Figma (Dev Resources)
- ❌ Pas d'extraction design tokens (Variables Figma)
- ❌ Pas de cache (chaque appel = requête API)

Ces fonctionnalités pourront être ajoutées en v2+ selon les besoins.

---

## Évolutions futures

**v2 : Traçabilité bidirectionnelle**
- `create_figma_comment(fileId, message)`
- `link_ticket_to_figma(fileId, ticketId)`

**v3 : Design tokens**
- `get_design_tokens(fileId)`
- `get_component_specs(componentId)`

**v4 : Webhooks**
- Notifications temps réel sur changements Figma
- Synchronisation automatique

---

## Ressources

- **API Figma** : https://www.figma.com/developers/api
- **Conventions Figma** : [`config/figma.conventions.md`](../../config/figma.conventions.md)
- **Infrastructure MCP** : [`servers/README.md`](../../servers/README.md)
- **MCP Protocol** : https://modelcontextprotocol.io/

---

## Support

En cas de problème :
1. Consulter ce guide de dépannage
2. Vérifier les logs OpenCode
3. Tester le MCP manuellement : `cd servers/figma-mcp && npm start`
4. Vérifier configuration tokens Figma

**L'intégration Figma est prête à enrichir vos workflows de planification !** 🎨
