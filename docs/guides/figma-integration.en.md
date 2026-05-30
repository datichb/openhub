# Intégration Figma - Guide de démarrage

## ✅ Ce qui a été implémenté

### Infrastructure

- ✅ **MCP Server Figma** (`servers/figma-mcp/`)
  - Client API Figma avec gestion d'erreurs
  - 3 tools MCP : `search_figma_files`, `get_file_structure`, `detect_ui_signals`
  - Configuration TypeScript + build system
  
- ✅ **Scripts de build et déploiement**
  - `scripts/build-mcp.sh` : Compile les MCP servers
  - `scripts/check-mcp.sh` : Vérifie l'état de build
  - `scripts/lib/mcp-deploy.sh` : Fonctions de déploiement MCP

- ✅ **Skills d'intégration**
  - `skills/adapters/figma-scout-protocol.md` : Enrichissement Scout
  - `skills/adapters/figma-planner-protocol.md` : Enrichissement Planner

- ✅ **Agents modifiés**
  - `agents/planning/scout.md` : Référence skill + MCP server
  - `agents/planning/planner.md` : Référence skill + MCP server

- ✅ **Documentation**
  - `config/figma.conventions.md` : Conventions d'organisation Figma
  - `servers/README.md` : Documentation infrastructure MCP
  - `servers/figma-mcp/README.md` : Documentation spécifique Figma

- ✅ **Configuration**
  - `.gitignore` modifié (ignore dist/, node_modules/ des MCP)

---

## 🚀 Prochaines étapes

### 1. Configuration des tokens Figma

Créer le fichier `~/.config/opencode/config.json` avec vos tokens :

```bash
mkdir -p ~/.config/opencode
cat > ~/.config/opencode/config.json << 'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "env": {
    "FIGMA_PERSONAL_ACCESS_TOKEN": "figd_VOTRE_TOKEN_ICI",
    "FIGMA_TEAM_ID": "VOTRE_TEAM_ID_ICI"
  }
}
EOF
```

#### Comment obtenir ces valeurs :

**FIGMA_PERSONAL_ACCESS_TOKEN :**
1. Aller sur https://www.figma.com/developers/api#authentication
2. Section "Personal access tokens"
3. Générer un nouveau token avec scopes : `file:read`, `projects:read`

**FIGMA_TEAM_ID :**
1. Aller sur votre team Figma
2. L'ID est dans l'URL : `https://www.figma.com/files/team/123456/...`
3. Copier `123456`

### 2. Tester le build du MCP

Le MCP a déjà été compilé avec succès, mais vous pouvez le reconstruire :

```bash
# Depuis la racine du hub
bash scripts/build-mcp.sh

# Ou build un seul MCP
bash scripts/build-mcp.sh figma-mcp
```

### 3. Tester manuellement le MCP (optionnel)

```bash
cd servers/figma-mcp

# Avec vos tokens
FIGMA_PERSONAL_ACCESS_TOKEN=figd_xxx \
FIGMA_TEAM_ID=123456 \
npm start

# Le serveur démarre et attend des commandes via stdin
# Ctrl+C pour arrêter
```

### 4. Déployer vers un projet test

```bash
# Depuis la racine du hub
./oc.sh deploy MY-TEST-PROJECT

# Le script va :
# 1. Vérifier l'état de build des MCP
# 2. Proposer de les compiler si nécessaire
# 3. Copier les agents + skills + MCP vers le projet
# 4. Configurer opencode.json du projet
```

**Note :** Le déploiement automatique des MCP n'est pas encore intégré dans `cmd-deploy.sh`. Vous devrez soit :
- Modifier `cmd-deploy.sh` pour sourcer `scripts/lib/mcp-deploy.sh` et appeler les fonctions
- Ou déployer manuellement le MCP après le déploiement standard

### 5. Organiser vos fichiers Figma

Suivre les conventions définies dans `config/figma.conventions.md` :

- **Nommage :** `[Projet] - [Feature] - [Type]`
- **Tags :** `#feature-xxx`, `#ready-dev`, `#wip`
- **Pages :** Cover, Flows, UI Design, States, Dev Notes
- **Frames :** `[Type] - [Nom] - [État]`

### 6. Tester l'intégration end-to-end

Une fois le projet déployé :

```bash
cd ~/workspace/my-test-project

# Lancer OpenCode
opencode

# Invoquer le scout avec une feature UI
> Scout cette feature : tableau de bord utilisateur

# Le scout devrait :
# 1. Explorer la codebase (normal)
# 2. Chercher dans Figma (nouveau)
# 3. Inclure les données Figma dans son rapport
```

---

## 📋 Checklist de validation

### Infrastructure
- [ ] MCP compilé sans erreurs (`bash scripts/build-mcp.sh`)
- [ ] Dépendances installées (`servers/figma-mcp/node_modules/` existe)
- [ ] Fichiers dist présents (`servers/figma-mcp/dist/index.js` existe)

### Configuration
- [ ] Token Figma obtenu
- [ ] Team ID récupéré
- [ ] `~/.config/opencode/config.json` créé avec les bonnes valeurs
- [ ] Permissions token Figma : `file:read`, `projects:read`

### Déploiement
- [ ] Projet test enregistré dans le hub (`./oc.sh init TEST-PROJECT`)
- [ ] Agents déployés (`./oc.sh deploy TEST-PROJECT`)
- [ ] MCP copié vers `.opencode/servers/figma-mcp/`
- [ ] `opencode.json` contient la config MCP

### Organisation Figma
- [ ] Fichiers Figma renommés selon conventions
- [ ] Tags ajoutés (`#feature-xxx`, `#ready-dev`)
- [ ] Pages organisées (Flows, UI, States, etc.)
- [ ] Dev Resources ajoutés (liens vers tickets si existants)

### Test end-to-end
- [ ] Scout invoqué sur une feature UI
- [ ] Recherche Figma effectuée
- [ ] Données Figma incluses dans le rapport
- [ ] Signaux UX/UI détectés automatiquement
- [ ] URLs Figma présentes dans le rapport

---

## 🔧 Dépannage

### Le MCP ne trouve pas le token

**Erreur :** `FIGMA_PERSONAL_ACCESS_TOKEN environment variable is required`

**Solution :**
1. Vérifier que `~/.config/opencode/config.json` existe
2. Vérifier que le token est bien dans la section `env`
3. Redémarrer OpenCode après modification de la config

### Aucun fichier Figma trouvé

**Erreur :** `Aucun fichier Figma trouvé pour la recherche : "xxx"`

**Causes possibles :**
1. Team ID incorrect
2. Fichiers Figma mal nommés (pas de match avec la recherche)
3. Permissions token insuffisantes
4. Fichiers dans un autre team

**Solution :**
- Vérifier le Team ID dans l'URL Figma
- Renommer les fichiers selon les conventions
- Vérifier les scopes du token : `file:read`, `projects:read`

### Erreur lors du build TypeScript

**Erreur :** `tsc: command not found` ou erreurs de compilation

**Solution :**
```bash
cd servers/figma-mcp
rm -rf node_modules package-lock.json
npm install
npm run build
```

### Le déploiement ignore les MCP

**Cause :** Le script `cmd-deploy.sh` n'appelle pas encore les fonctions MCP.

**Solution temporaire :**
Déployer manuellement après le déploiement standard :

```bash
# Depuis la racine du hub
source scripts/lib/mcp-deploy.sh

# Définir les variables
HUB_DIR=$(pwd)
DEPLOY_DIR=~/workspace/my-test-project

# Déployer
check_and_build_mcp
deploy_mcp_servers "$DEPLOY_DIR"
configure_mcp_in_project "$DEPLOY_DIR"
```

---

## 🎯 Tests suggérés

### Test 1 : Recherche simple

**Feature :** "Dashboard utilisateur"

**Attendu :**
- `search_figma_files("dashboard")` trouve des fichiers
- URLs Figma valides retournées
- Dates de dernière modification présentes

### Test 2 : Détection de signaux

**Feature :** "Flow inscription multi-étapes"

**Attendu :**
- Recherche trouve le fichier inscription
- `detect_ui_signals` détecte le flow multi-étapes
- Signaux UX et UI marqués à true
- Complexité estimée (L ou XL)
- Recommandations d'escalade au planner

### Test 3 : Enrichissement Scout

**Feature :** "Page de paramètres"

**Attendu :**
- Scout cherche dans Figma automatiquement
- Rapport contient section "🎨 Contexte Figma"
- Composants listés
- Estimation ajustée selon composants détectés

### Test 4 : Enrichissement Planner

**Feature :** "Interface de gestion des tags"

**Attendu :**
- Planner exécute Phase 1.3 (Exploration Figma)
- Récap Phase 1 contient données Figma
- Phase 1.5 proposée si signaux détectés
- Tickets créés avec champ `--design` pré-rempli

---

## 📚 Ressources

- **Documentation Figma API :** https://www.figma.com/developers/api
- **MCP Protocol :** https://modelcontextprotocol.io/
- **OpenCode Docs :** https://opencode.ai/docs

---

## 🚧 Limitations actuelles (v1)

- ❌ Pas de webhooks (notifications temps réel)
- ❌ Pas de création de commentaires Figma (lecture seule)
- ❌ Pas de création de Dev Resources (liens tickets → Figma)
- ❌ Pas d'extraction de design tokens (Variables Figma)
- ❌ Pas de cache intelligent (chaque appel = requête API)

Ces fonctionnalités pourront être ajoutées dans les versions futures selon les besoins.

---

## 📞 Support

En cas de problème :
1. Vérifier les logs OpenCode
2. Tester le MCP manuellement (voir section 3)
3. Vérifier les tokens et permissions Figma
4. Consulter `config/figma.conventions.md` pour l'organisation

**Le MCP Server Figma est maintenant prêt à être testé !**
