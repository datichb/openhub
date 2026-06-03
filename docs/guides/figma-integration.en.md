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
  - `scripts/lib/mcp-deploy.sh` : Fonctions de déploiement MCP (appelées automatiquement par `oc deploy`)

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

La méthode recommandée est d'utiliser la commande `oc service setup` :

```bash
oc service setup figma
# or via alias:
oc figma setup
```

This command will:
1. Prompt for your **Personal Access Token**
2. Prompt for your **Team ID**
3. Validate the Figma API connection
4. Save config to `~/.config/opencode/config.json`
5. Auto-build the MCP server if needed

Check status at any time:
```bash
oc service status figma
```

### Alternative: manual configuration

Create `~/.config/opencode/config.json` manually:

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

#### How to get these values:

**FIGMA_PERSONAL_ACCESS_TOKEN :**
1. Aller sur https://www.figma.com/settings (onglet **Security**)
2. Section "Personal access tokens" → cliquer **Generate new token**
3. Sélectionner les scopes suivants (voir détail ci-dessous) :
   - `current_user:read` — requis pour valider le token via `/v1/me`
   - `file_content:read` — lire le contenu des fichiers Figma
   - `file_metadata:read` — lire les métadonnées des fichiers
   - `project_metadata:read` — lire les métadonnées des projets
   - `projects:read` — lister les projets et fichiers d'une team
   - `library_assets:read` — lire les composants et styles publiés
   - `file_variables:read` _(optionnel, Enterprise uniquement)_ — lire les variables/design tokens

> **Note :** Les anciens scopes `file:read` et `files:read` sont **dépréciés** par Figma. Utiliser les scopes granulaires ci-dessus.
> Référence officielle : https://developers.figma.com/docs/rest-api/scopes

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

### 4. Deploy to a project

```bash
# From the hub root
./oc.sh deploy MY-TEST-PROJECT

# The script will:
# 1. Copy agents + skills to the project
# 2. Generate opencode.json with provider/model config
# 3. Build the MCP server if needed (Phase 4)
# 4. Copy MCP files to .opencode/servers/
# 5. Write the mcp block into opencode.json
```

To deploy only the MCP server into an already-deployed project:

```bash
oc service figma deploy --project MY-TEST-PROJECT
# or via alias:
oc figma deploy --project MY-TEST-PROJECT
```

To rebuild the MCP server globally (without deploying to a project):

```bash
oc figma deploy
```

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
- [ ] Permissions token Figma : `current_user:read`, `file_content:read`, `file_metadata:read`, `project_metadata:read`, `projects:read`, `library_assets:read`

### Déploiement
- [ ] Projet test enregistré dans le hub (`./oc.sh init TEST-PROJECT`)
- [ ] Agents + MCP déployés (`./oc.sh deploy TEST-PROJECT`)
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

### Le token est refusé ("token invalide")

**Erreur :** `Invalid scope(s): ... This endpoint requires the current_user:read scope`

**Cause :** Le Personal Access Token a été créé sans le scope `current_user:read`, qui est obligatoire pour l'endpoint de validation `/v1/me` utilisé par `oc service status figma`.

**Solution :** Régénérer un nouveau token en cochant tous les scopes requis :

| Scope | Usage |
|---|---|
| `current_user:read` | Validation du token (endpoint `/v1/me`) |
| `file_content:read` | Lecture du contenu des fichiers Figma |
| `file_metadata:read` | Lecture des métadonnées de fichiers |
| `project_metadata:read` | Lecture des métadonnées de projets |
| `projects:read` | Listage des projets et fichiers d'une team |
| `library_assets:read` | Lecture des composants et styles publiés |
| `file_variables:read` | _(Optionnel, Enterprise uniquement)_ Design tokens |

> Les anciens scopes `file:read` / `files:read` sont **dépréciés** — ne pas les utiliser.

Une fois le nouveau token créé, le mettre à jour :
```bash
oc service setup figma
```

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
- Vérifier les scopes du token : `current_user:read`, `file_content:read`, `file_metadata:read`, `project_metadata:read`, `projects:read`, `library_assets:read`

### Erreur lors du build TypeScript

**Erreur :** `tsc: command not found` ou erreurs de compilation

**Solution :**
```bash
cd servers/figma-mcp
rm -rf node_modules package-lock.json
npm install
npm run build
```

### Le MCP n'est pas configuré dans opencode.json

**Cause :** Le projet n'a pas encore été déployé, ou le déploiement a été fait avec une ancienne version du hub.

**Solution :** Relancer le déploiement complet ou déployer uniquement le MCP :

```bash
# Redéploiement complet
./oc.sh deploy MY-PROJECT

# Ou uniquement le MCP dans un projet existant
oc figma deploy --project MY-PROJECT
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
