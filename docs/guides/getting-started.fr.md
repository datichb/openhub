# Démarrage rapide

Ce guide vous permet d'installer le hub et de lancer votre premier agent en moins de 10 minutes.

## Prérequis

| Outil | Version minimale | Vérification |
|-------|-----------------|--------------|
| Git | 2.x | `git --version` |
| curl | — | `curl --version` |

> Les autres dépendances (`jq`, `Node.js`, `opencode`, `bun`) sont proposées à l'installation — **chaque outil demande une confirmation explicite** avant d'être installé.
>
> **Beads (`bd`)** est proposé à l'installation par `oc install` (via `brew install beads` ou curl).
> Le board kanban terminal (`oc beads board`) est intégré — aucune installation supplémentaire requise.

---

## 1. Installer le hub

### Option A — One-liner (recommandé)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | bash
```

Le script automatise :
- Clone du repo dans `~/.opencode-hub`
- Vérification des dépendances manquantes (`jq`, `Node.js`, `opencode`, `bun`) — **confirmation demandée avant chaque installation**
- Création de l'alias `oc` dans `~/.zshrc` ou `~/.bashrc` (propose garder / remplacer / renommer si un alias `oc` existe déjà)
- Initialisation des fichiers de config locaux
- Configuration interactive des cibles AI et du provider LLM

Après l'installation, recharger le shell :

```bash
source ~/.zshrc   # ou source ~/.bashrc
```

> **Dossier d'installation personnalisé :** `OPENCODE_HUB_DIR=~/tools/oc bash install.sh`

---

### Option B — Installation manuelle

```bash
# 1. Cloner
git clone https://github.com/datichb/opencode-hub.git ~/.opencode-hub

# 2. Alias shell
echo 'alias oc="~/.opencode-hub/oc.sh"' >> ~/.zshrc && source ~/.zshrc

# 3. Configurer
oc install
```

`oc install` est interactif et vous demande de choisir les cibles à activer :

| Choix | Cibles configurées |
|-------|--------------------|
| 1 (défaut) | OpenCode |
| 2 | OpenCode |
| 3 | Tout (OpenCode + OpenCode) |

> Si `config/hub.json` existe déjà, une confirmation est demandée avant d'écraser
> la configuration. Répondez `N` pour conserver votre configuration existante.

---

## 2. Enregistrer un projet

```bash
oc init MON-APP ~/workspace/mon-app
```

Cette commande :
- Ajoute `MON-APP` dans `projects/projects.md`
- Associe le chemin local `~/workspace/mon-app`
- Propose de déployer les agents immédiatement

> **Convention `PROJECT_ID`** : lettres, chiffres, `-` et `_` uniquement. Pas d'espaces.

---

## 3. Déployer les agents

Si vous n'avez pas déployé lors du `oc init` :

```bash
# Déployer dans un projet spécifique
oc deploy opencode MON-APP
oc deploy all MON-APP   # toutes les cibles actives
```

Résultat attendu selon la cible :

| Cible | Fichiers générés dans le projet |
|-------|---------------------------------|
| `opencode` | `.opencode/agents/*.md` |
| `opencode` | `.opencode/agents/*.md` |

---

## 4. Lancer l'outil

```bash
oc start MON-APP
```

Lance l'outil par défaut (défini dans `config/hub.json`) dans le répertoire du projet.

Avec un prompt de démarrage :

```bash
oc start MON-APP "explique l'architecture du projet"
```

En mode développement (charge les tickets `ai-delegated` ouverts) :

```bash
oc start MON-APP --dev
```

Avec le board kanban terminal ouvert dans un second volet :

```bash
oc beads board MON-APP            # affiche le board une fois
oc beads board MON-APP --watch    # rafraîchissement en direct toutes les 5s
```

---

## 5. Vérifier le déploiement

```bash
oc deploy --check opencode MON-APP
```

Affiche pour chaque agent : `✓ À JOUR`, `⚠ OBSOLÈTE` ou `✗ MANQUANT`.

Après un `git pull` sur le hub (ou `oc update`) :

```bash
oc sync            # redéploie sur tous les projets
oc sync --dry-run  # vérifie sans déployer
```

---

## Résultat attendu

À l'issue de ces étapes, dans le répertoire de votre projet :

```
mon-app/
└── .opencode/
    └── agents/
        ├── orchestrator.md
        ├── planner.md
        ├── reviewer.md
        ├── qa-engineer.md
        ├── debugger.md
        ├── auditor.md
        ├── developer-frontend.md
        └── ...
```

Vous pouvez maintenant invoquer n'importe quel agent dans OpenCode :
- `"Implémente la feature de connexion utilisateur"` → agent `orchestrator`
- `"Audite la sécurité du projet"` → agent `auditor-security`
- `"Planifie le module de paiement"` → agent `planner`

---

## Mettre à jour le hub

### Mettre à jour les outils installés

```bash
oc update
```

Met à jour opencode, Beads, Beads UI, et les skills externes. Si des skills sont modifiés, propose de relancer `oc sync`.

### Mettre à jour les sources du hub

```bash
oc upgrade
```

Récupère les derniers scripts et agents du hub (`git pull`). Propose de relancer `oc sync` après une mise à jour réussie.

Pour basculer sur une version spécifique :

```bash
oc upgrade v1.1.0
```

Équivalent au one-liner :

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/opencode-hub/main/install.sh | VERSION=v1.1.0 bash
```

---

## Dépannage

| Symptôme | Solution |
|----------|----------|
| `oc: command not found` | Relancer `source ~/.zshrc` (ou `~/.bashrc`) après installation |
| `curl: command not found` | Installer curl, puis relancer le one-liner |
| `Node.js introuvable` | Relancer `oc install` — propose les installeurs disponibles |
| Agent absent dans l'outil | Relancer `oc deploy <target> MON-APP` |
| Agent obsolète (`⚠ OBSOLÈTE`) | `oc deploy <target> MON-APP` pour resynchroniser |
| `bd: command not found` | Installer Beads : `brew install beads` |
| Dossier d'install déjà existant | `OPENCODE_HUB_DIR=~/autre-chemin bash install.sh` |

---

## Désinstaller le hub

```bash
oc uninstall
# ou depuis n'importe où :
bash ~/.opencode-hub/uninstall.sh
```

Le script guide la désinstallation en 4 étapes optionnelles (toutes avec confirmation) :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets | `[y/N]` |
| 2 | Supprimer `~/.opencode-hub` | `[y/N]` |
| 3 | Retirer l'alias et les exports bun du fichier rc | `[Y/n]` |
| 4 | Désinstaller opencode, Beads, bun (séparément) | `[y/N]` |
