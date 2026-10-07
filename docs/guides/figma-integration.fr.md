> 🇬🇧 [Read in English](figma-integration.en.md)

# Intégration Figma - Guide de démarrage

## Vue d'ensemble

L'intégration Figma donne à l'agent `designer` (seul agent avec l'accès MCP Figma) un accès en **lecture seule** à vos fichiers Figma : structure d'un fichier, nœuds, styles. Les autres agents (pathfinder, planner, onboarder…) lui délèguent la reconnaissance Figma (`Mode: recon`) pour relier une feature à ses maquettes, repérer les composants et signaux UX/UI, et ajuster leurs estimations.

Le serveur MCP Figma est **intégré au binaire `oh`** (`oh mcp serve figma`) : rien à installer ni à compiler.

---

## Configuration

### 1. Obtenir un token Figma

**Personal Access Token :**
1. Aller sur https://www.figma.com/developers/api#authentication
2. Section "Personal access tokens"
3. Créer un token avec les scopes : `current_user:read`, `file_content:read`, `file_metadata:read`, `projects:read`, `library_assets:read`

### 2. Configurer via `oh mcp setup`

```bash
oh mcp setup                 # choisir Figma, puis saisir le token
oh mcp setup -p mon-projet   # token propre à un projet
```

L'assistant :
1. Demande votre **Personal Access Token** Figma (saisie masquée)
2. Le stocke dans le trousseau système (clé `figma-token`, ou `figma-token-<id-projet>` pour un projet)
3. Active le service : bloc `[mcp.figma]` de `hub.toml`, ou surcharge du projet

Sans token saisi, le serveur lit la variable d'environnement `FIGMA_TOKEN`.

Vérifier et gérer le service :

```bash
oh mcp status                       # état des services MCP (avec -p : surcharges du projet)
oh mcp enable figma                 # activer (hub, ou -p pour un projet)
oh mcp disable figma -p mon-projet  # désactiver pour un projet
oh mcp reset figma -p mon-projet    # revenir à la configuration du hub
oh doctor                           # ligne « Clés API »
```

### 3. Organiser vos fichiers Figma

Conventions recommandées :

- **Nommage** : `[Projet] - [Feature] - [Type]`
- **Tags** : `#feature-xxx`, `#ready-dev`, `#wip`
- **Pages** : Cover, Flows, UI Design, States, Dev Notes

### 4. Lancer une session

```bash
oh run <workflow> -p MON-PROJET
```

Aucune étape de déploiement (`oh deploy` supprimé en v5) : une fois activé, le serveur Figma est déclaré dans le paquet de session au lancement. Un workflow sans champ `mcp:` reçoit les serveurs MCP du projet ; avec `mcp:`, seulement ceux listés (voir [Workflows : CLI](../reference/cli-workflows.fr.md)). Pour vérifier : `oh bundle show <workflow>`.

---

## Utilisation

### Avec Pathfinder

```bash
> Pathfinder cette feature: tableau de bord utilisateur
```

Le Pathfinder va :
1. Explorer la codebase (workflow normal)
2. Déléguer au `designer` (`Mode: recon`) la recherche et l'analyse des maquettes liées (composants, signaux UX/UI)
3. Inclure les données Figma dans son rapport

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
2. **Phase 1.3** : Explorer Figma (délégation au `designer`, `Mode: recon`)
   - Chercher les maquettes liées
   - Détecter les signaux UX/UI
3. **Phase 1.5** : Proposer la délégation au designer si des signaux sont détectés
4. **Phase 5** : Pré-remplir `--design` des tickets avec les données Figma

---

## Tools MCP disponibles

| Tool | Rôle | Entrée |
|------|------|--------|
| `figma_get_file` | Récupère un fichier Figma (structure, frames, composants) | `file_key` |
| `figma_get_node` | Récupère un nœud précis d'un fichier | `file_key`, `node_id` |
| `figma_get_styles` | Récupère les styles d'un fichier | `file_key` |

La clé de fichier (`file_key`) se lit dans l'URL Figma : `https://www.figma.com/file/<file_key>/...`.

---

## Architecture

L'implémentation se trouve dans `cli/internal/mcp/figma/` (serveur MCP stdio JSON-RPC, client de l'API Figma). Les protocoles des agents sont dans `skills/designer/figma-recon-protocol.md` et `skills/designer/figma-deep-protocol.md`.

Au runtime, le serveur est démarré par opencode avec la commande déclarée dans le paquet de session :

```bash
oh mcp serve figma --token-key figma-token
```

Le token est lu dans le trousseau par `oh mcp serve` : il n'est jamais écrit dans le paquet ni dans le projet.

---

## Tests

### Test 1 : Pathfinder simple

```bash
# Dans un projet avec maquettes Figma
> Pathfinder cette feature: page paramètres

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

Le serveur MCP Figma n'a pas d'outil de recherche (outils : `figma_get_file`, `figma_get_node`, `figma_get_styles`). Les agents trouvent les fichiers par leur **URL** (`figma.com/file/<clé>/…` ou `figma.com/design/<clé>/…`) : dans le ticket, le brief, le wiki, le README ou la documentation du projet. Sans URL, l'agent te la demande.

**Pour que les maquettes soient trouvées :**
- Mettre l'URL du fichier Figma dans le ticket ou dans `docs/wiki/`
- Donner directement l'URL à l'agent
- Vérifier les scopes du token : `current_user:read`, `file_content:read`, `file_metadata:read`, `projects:read`, `library_assets:read`

### Token non reconnu

**Erreur :** `FIGMA_TOKEN environment variable not set`

**Solutions :**
- Relancer `oh mcp setup` (Figma) pour stocker le token dans le trousseau
- Vérifier que le service est actif : `oh mcp status`
- Relancer la session : le paquet est reconstruit au lancement

### Problèmes serveur MCP

Le serveur MCP Figma étant intégré au binaire `oh`, il n'y a pas d'étape de build séparée. Si le serveur ne démarre pas :

```bash
oh mcp status                 # vérifier la configuration du service
oh bundle show <workflow>     # vérifier que figma est dans le paquet
oh mcp serve figma            # tester le serveur à la main (stdio)
```

### Timeout API Figma

**Symptôme :** l'agent mentionne `⚠️ Figma indisponible (timeout)` dans son rapport.

**Causes possibles :** connexion lente, gros fichier Figma, API Figma surchargée. Les requêtes expirent après 30 s.

---

## Limitations actuelles

- ❌ Pas de webhooks (notifications temps réel)
- ❌ Pas de création de commentaires Figma (lecture seule)
- ❌ Pas de liens tickets → Figma (Dev Resources)
- ❌ Pas d'extraction des design tokens (Variables Figma)
- ❌ Pas de cache (chaque appel = requête API)

---

## Évolutions futures

**Traçabilité bidirectionnelle**
- `create_figma_comment(fileId, message)`
- `link_ticket_to_figma(fileId, ticketId)`

**Design tokens**
- `get_design_tokens(fileId)`
- `get_component_specs(componentId)`

**Webhooks**
- Notifications temps réel sur changements Figma
- Synchronisation automatique

---

## Ressources

- **API Figma** : https://www.figma.com/developers/api
- **MCP Protocol** : https://modelcontextprotocol.io/
- **Référence des services MCP** : [Services](../reference/services.fr.md)

---

## Support

En cas de problème :
1. Consulter ce guide de dépannage
2. Lancer `oh doctor`
3. Tester le MCP manuellement : `oh mcp serve figma`
4. Vérifier la configuration : `oh mcp status`
