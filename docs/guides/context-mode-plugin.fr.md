> 🇬🇧 [Read in English](context-mode-plugin.en.md)

# Guide d'installation du Plugin context-mode

Ce guide explique comment installer le plugin context-mode pour OpenCode depuis opencode-hub.

## Prérequis

1. **OpenCode** >= 1.15.0 installé
   ```bash
   opencode --version
   ```

2. **opencode-hub** cloné et configuré
   ```bash
   cd ~/.opencode-hub
   git pull
   ```

---

## Installation automatique (recommandé)

```bash
oc plugin install context-mode
```

Le script va :
1. Vérifier qu'OpenCode est installé
2. Ajouter `"context-mode"` au tableau `"plugin"` dans `.opencode/opencode.json`

OpenCode installera automatiquement le package npm `context-mode` depuis son cache au prochain démarrage, via son runtime Bun natif.

---

## Comment ça marche

context-mode est un **npm plugin natif OpenCode** (déclaré via `"plugin": ["context-mode"]` dans `opencode.json`). OpenCode gère l'installation, la mise à jour et le chargement automatiquement — aucun wrapper `.ts` ni gestion manuelle de `node_modules` nécessaire.

Le package est mis en cache dans `~/.cache/opencode/node_modules/` et chargé par le runtime Bun intégré d'OpenCode.

---

## Vérification de l'installation

### 1. Redémarrer OpenCode

Si OpenCode est en cours d'exécution, fermez-le et relancez-le depuis le répertoire du hub :

```bash
cd ~/.opencode-hub
opencode
```

### 2. Vérifier les logs

```bash
tail -f ~/.cache/opencode/logs/opencode.log | grep context-mode
```

Au démarrage de session, vous devriez voir le plugin chargé.

### 3. Tester le plugin

Dans OpenCode, ouvrez un fichier volumineux ou effectuez une recherche webfetch :

```
> Lis le fichier src/services/auth.service.ts
```

Si le fichier fait plus de ~4 000 tokens, un toast apparaît :
```
🗜️ context-mode sandboxed ~12.3K tokens (read)
```

### 4. Statistiques de session

En fin de session (fermeture OpenCode), un toast récapitulatif s'affiche :
```
🗜️ context-mode: 8 tools sandboxed, ~45.2K tokens économisés
```

---

## Ce que fait le plugin

Le plugin intervient sur trois axes complémentaires à RTK :

| Axe | Ce que RTK couvre | Ce que context-mode ajoute |
|-----|------------------|---------------------------|
| Outputs bash | ✅ `git diff`, `find`, `cat`, logs... | — |
| Outputs `read` / `webfetch` | ❌ | ✅ Indexés hors-contexte (SQLite + BM25) |
| Outputs MCP | ❌ | ✅ Idem |
| Session continuity | ❌ | ✅ Reprise via BM25 après compaction |

### Sandbox tools

Quand l'agent lit un fichier volumineux ou fait un webfetch, context-mode intercepte le résultat et l'indexe hors du contexte LLM. L'agent peut ensuite interroger l'index par similarité sémantique — seul le passage pertinent entre dans le contexte.

**Impact mesuré :** 80-98% de réduction sur les gros outputs (fichiers > 1K tokens, pages web complètes).

### Session continuity

Chaque événement de session est stocké en SQLite. Si OpenCode compacte automatiquement le contexte, l'agent retrouve l'état de la session via BM25 sans re-explorer la codebase.

**Impact mesuré :** 0 tokens gaspillés après compaction (vs. exploration complète de la codebase).

### Think in Code

Le plugin instruite l'agent à écrire un script d'analyse ciblé plutôt que de chaîner 10 appels `read`/`glob`/`grep`. Un script remplace une exploration multi-fichiers.

---

## Hooks OpenCode utilisés

| Hook | Stabilité | Rôle |
|------|-----------|------|
| `tool.execute.before` | Stable | Interception des appels `read`, `webfetch` avant exécution |
| `tool.execute.after` | Stable | Estimation des tokens économisés sur les gros outputs |
| `dispose` | Stable | Résumé de session (toast + log) |
| `experimental.chat.system.transform` | **Expérimental** | Injection des instructions context-mode dans le system prompt — pas besoin d'AGENTS.md |
| `experimental.session.compacting` | **Expérimental** | Session continuity après compaction automatique |

> **Note sur les hooks expérimentaux :** Les hooks `experimental.*` peuvent changer lors des mises à jour OpenCode. Le plugin fonctionne en mode dégradé (hooks stables uniquement) si ces hooks sont absents ou modifiés — la sandbox de base reste active. Mettre à jour le plugin après chaque mise à jour majeure d'OpenCode.

---

## Complémentarité avec RTK

RTK et context-mode sont **orthogonaux** — ils couvrent des couches différentes :

```
Commande bash       → RTK intercepte        → output compressé avant injection
Appel read/webfetch → context-mode intercepte → output indexé hors-contexte
```

Les deux plugins peuvent cohabiter sans conflit. L'ordre d'installation n'a pas d'importance.

**Stack complète recommandée :**
1. `oc plugin install rtk` — bash outputs (-60-90%)
2. `oc plugin install context-mode` — read/webfetch/MCP outputs (-80-98%) + session continuity

---

## Troubleshooting

### OpenCode ne charge pas context-mode

Vérifier que `.opencode/opencode.json` du hub contient bien `"context-mode"` dans le tableau `"plugin"` :

```bash
cat .opencode/opencode.json
# Attendu : { "$schema": "...", "plugin": ["context-mode"] }
```

Si absent, relancer l'installation :
```bash
oc plugin install context-mode
```

### Les hooks `experimental.*` ne sont pas actifs

Si `experimental.chat.system.transform` est absent dans votre version d'OpenCode, le plugin fonctionne en mode dégradé : la sandbox de base (tracking + estimation tokens) reste active, mais les instructions context-mode ne sont pas injectées dans le system prompt.

Vérifier la version d'OpenCode :
```bash
opencode --version
```

Si < 1.15.0, mettre à jour : `npm install -g opencode-ai`

### Conflit avec un MCP server `context-mode`

Si vous avez déjà un MCP server `context-mode` installé, le plugin OpenCode prend le dessus sur l'injection dans le system prompt mais les deux coexistent sans conflit fonctionnel.

---

## Métriques attendues

| Type d'output | Réduction tokens estimée |
|---------------|--------------------------|
| Fichier source (>1K tokens) | 80-95% |
| Page webfetch complète | 85-98% |
| Output MCP volumineux | 70-90% |
| Après compaction (session continuity) | 100% (0 tokens perdus) |

---

## Mise à jour

```bash
cd ~/.opencode-hub && git pull
# opencode met à jour le package automatiquement au prochain démarrage
```

## Désinstallation

```bash
oc plugin remove context-mode
# Puis relancer OpenCode
```

---

**Version :** 2.0.0 (2026-06-12)
**Compatible avec :** context-mode npm ^1.0.0, OpenCode >= 1.15.0
