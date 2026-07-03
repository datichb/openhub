# Démarrage rapide

Ce guide vous permet d'installer le hub et de lancer votre premier agent en moins de 10 minutes.

## Prérequis

| Outil | Version minimale | Vérification |
|-------|-----------------|--------------|
| Git | 2.x | `git --version` |
| curl | — | `curl --version` |

> Les autres dépendances (`jq`, `Node.js`, `opencode`, `bun`, `sqlite3`) sont proposées à l'installation — **chaque outil demande une confirmation explicite** avant d'être installé.
>
> **`sqlite3`** est requis pour `oh metrics` et `oh dashboard` (lecture de la base de sessions OpenCode). Il est **natif sur macOS** (`/usr/bin/sqlite3`) ; sur Linux il sera proposé à l'installation via `apt-get`.
>
> **Beads (`bd`)** est proposé à l'installation par `oh install` (via `brew install beads` ou curl).
> Le board kanban terminal (`oh beads board`) est intégré — aucune installation supplémentaire requise.

---

## 1. Installer le hub

### Option A — One-liner (recommandé)

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | bash
```

Le script automatise :
- Clone du repo dans `~/.openhub`
- Vérification des dépendances manquantes (`jq`, `Node.js`, `opencode`, `bun`) — **confirmation demandée avant chaque installation**
- Création de l'alias `oh` dans `~/.zshrc` ou `~/.bashrc` (propose garder / remplacer / renommer si un alias `oh` existe déjà)
- Initialisation des fichiers de config locaux
- Configuration du fournisseur LLM

Après l'installation, recharger le shell :

```bash
source ~/.zshrc   # ou source ~/.bashrc
```

> **Dossier d'installation personnalisé :** `OPENCODE_HUB_DIR=~/tools/oc bash install.sh`

---

### Option B — Installation manuelle

```bash
# 1. Cloner
git clone https://github.com/datichb/openhub.git ~/.openhub

# 2. Alias shell
echo 'alias oc="~/.openhub/oc.sh"' >> ~/.zshrc && source ~/.zshrc

# 3. Configurer
oh install
```

> Si `config/hub.json` existe déjà, une confirmation est demandée avant d'écraser
> la configuration. Répondez `N` pour conserver votre configuration existante.

---

## 2. Enregistrer un projet

```bash
oh init MON-APP ~/workspace/mon-app
```

Cette commande :
- Ajoute `MON-APP` dans `projects/projects.md`
- Associe le chemin local `~/workspace/mon-app`
- Propose de déployer les agents immédiatement

> **Convention `PROJECT_ID`** : lettres, chiffres, `-` et `_` uniquement. Pas d'espaces.

---

## 3. Déployer les agents

Si vous n'avez pas déployé lors du `oh init` :

```bash
# Déployer dans un projet spécifique
oh deploy MON-APP
```

Résultat attendu :

| Fichiers générés dans le projet |
|---------------------------------|
| `.opencode/agents/*.md` |

---

## 4. Lancer l'outil

```bash
oh start MON-APP
```

Lance l'outil par défaut (défini dans `config/hub.json`) dans le répertoire du projet.

Avec un prompt de démarrage :

```bash
oh start MON-APP "explique l'architecture du projet"
```

En mode développement (charge les tickets `ai-delegated` ouverts) :

```bash
oh start MON-APP --dev
```

Avec le board kanban terminal ouvert dans un second volet :

```bash
oh beads board MON-APP            # affiche le board une fois
oh beads board MON-APP --watch    # rafraîchissement en direct toutes les 5s
```

---

## 5. Vérifier le déploiement

```bash
oh deploy --check opencode MON-APP
```

Affiche pour chaque agent : `✓ À JOUR`, `⚠ OBSOLÈTE` ou `✗ MANQUANT`.

Après un `git pull` sur le hub (ou `oh update`) :

```bash
oh sync            # redéploie sur tous les projets
oh sync --dry-run  # vérifie sans déployer
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
- `"Audite la sécurité du projet"` → agent `auditor` (domaine security)
- `"Planifie le module de paiement"` → agent `planner`

---

## Mettre à jour le hub

### Mettre à jour les outils installés

```bash
oh update
```

Met à jour opencode, Beads, Beads UI, et les skills externes. Si des skills sont modifiés, propose de relancer `oh sync`.

### Mettre à jour les sources du hub

```bash
oh upgrade
```

Récupère les derniers scripts et agents du hub (`git pull`). Propose de relancer `oh sync` après une mise à jour réussie.

Pour basculer sur une version spécifique :

```bash
oh upgrade v1.1.0
```

Équivalent au one-liner :

```bash
curl -fsSL https://raw.githubusercontent.com/datichb/openhub/main/install.sh | VERSION=v1.1.0 bash
```

---

## Dépannage

| Symptôme | Solution |
|----------|----------|
| `oc: command not found` | Relancer `source ~/.zshrc` (ou `~/.bashrc`) après installation |
| `curl: command not found` | Installer curl, puis relancer le one-liner |
| `Node.js introuvable` | Relancer `oh install` — propose les installeurs disponibles |
| Agent absent dans l'outil | Relancer `oh deploy MON-APP` |
| Agent obsolète (`⚠ OBSOLÈTE`) | `oh deploy MON-APP` pour resynchroniser |
| `bd: command not found` | Installer Beads : `brew install beads` |
| Dossier d'install déjà existant | `OPENCODE_HUB_DIR=~/autre-chemin bash install.sh` |

---

## Désinstaller le hub

```bash
oh uninstall
# ou depuis n'importe où :
bash ~/.openhub/uninstall.sh
```

Le script guide la désinstallation en 4 étapes optionnelles (toutes avec confirmation) :

| Étape | Action | Défaut |
|-------|--------|--------|
| 1 | Nettoyer les agents déployés dans les projets | `[y/N]` |
| 2 | Supprimer `~/.openhub` | `[y/N]` |
| 3 | Retirer l'alias et les exports bun du fichier rc | `[Y/n]` |
| 4 | Désinstaller opencode, Beads, bun (séparément) | `[y/N]` |
