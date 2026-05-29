# RTK Usage — Token-Optimized Commands

**RTK (Rust Token Killer)** filtre automatiquement les sorties de commandes CLI pour économiser **60-90% de tokens** lors de l'interaction avec l'outil IA.

## Principe de Fonctionnement

RTK intercepte les commandes bash et applique des filtres intelligents :
- Supprime les informations redondantes
- Compacte les tableaux et listes
- Réduit la verbosité des logs
- Conserve uniquement les données essentielles

**Économies typiques :**
- `git diff` : 70-90% tokens économisés
- Tests : 60-80% tokens économisés
- Logs : 50-70% tokens économisés
- Listings : 60-75% tokens économisés

---

## Commandes à Utiliser Systématiquement

### Tests & CI/CD

```bash
# JavaScript/TypeScript
rtk jest --coverage                    # Au lieu de: jest --coverage
rtk vitest run                         # Au lieu de: vitest run
rtk playwright test                    # Au lieu de: playwright test

# Python
rtk pytest -v                          # Au lieu de: pytest -v
rtk pytest --cov                       # Au lieu de: pytest --cov

# Go
rtk go test ./...                      # Au lieu de: go test ./...
rtk go vet ./...                       # Au lieu de: go vet ./...

# Ruby/Rails
rtk rspec spec/                        # Au lieu de: rspec spec/
rtk rake test                          # Au lieu de: rake test
```

### Inspection de Fichiers

```bash
# Lire un gros fichier
rtk read large-file.ts                 # Au lieu de: cat large-file.ts

# Inspecter structure JSON (nouveauté RTK 0.42.0+)
rtk json package.json --keys-only      # Affiche uniquement les clés
rtk json tsconfig.json --depth 2       # Profondeur limitée

# Recherche dans fichiers
rtk grep "pattern" src/                # Au lieu de: grep -r "pattern" src/
```

### Git

```bash
# Diff
rtk git diff HEAD~10 HEAD              # Au lieu de: git diff HEAD~10 HEAD
rtk git diff --staged                  # Au lieu de: git diff --staged

# Log
rtk git log --oneline -50              # Au lieu de: git log --oneline -50
rtk git log --stat                     # Au lieu de: git log --stat

# Status (optionnel - déjà compact)
rtk git status --porcelain             # Version ultra-compacte
```

### Listings & Navigation

```bash
# Liste de fichiers
rtk ls -la packages/                   # Au lieu de: ls -la packages/
rtk ls -lh src/                        # Au lieu de: ls -lh src/

# Arborescence
rtk tree src/ --depth 3                # Au lieu de: tree src/
rtk find . -name "*.ts"                # Au lieu de: find . -name "*.ts"
```

### Build & Linting

```bash
# TypeScript
rtk tsc --noEmit                       # Au lieu de: tsc --noEmit
rtk npx eslint src/                    # Au lieu de: npx eslint src/

# Python
rtk mypy src/                          # Au lieu de: mypy src/
rtk ruff check .                       # Au lieu de: ruff check .

# Go
rtk golangci-lint run                  # Au lieu de: golangci-lint run
```

---

## Cas d'Usage Avancés (RTK 0.42.0+)

### Vérifier Structure JSON Sans Lire Toutes les Valeurs

**Scénario :** Inspecter un gros `package.json` ou `tsconfig.json` avant de décider quoi lire.

```bash
# Voir uniquement la structure (keys)
rtk json package.json --keys-only

# Sortie exemple :
# - name
# - version
# - scripts
#   - build
#   - test
#   - dev
# - dependencies
# - devDependencies
```

**Ensuite**, lire seulement la section nécessaire :
```bash
read package.json | grep -A 10 '"scripts"'
```

### Analyser Dépendances Projet

```bash
# Vue d'ensemble compacte des dépendances
rtk deps

# Sortie : liste compacte npm/yarn/pnpm avec versions essentielles
```

---

## Monitoring des Économies

### Statistiques Projet Courant

```bash
rtk gain --project              # Stats isolées du projet actuel
rtk gain --project --daily      # Breakdown quotidien
```

### Statistiques Globales

```bash
rtk gain                        # Vue d'ensemble globale
rtk gain --history              # Historique des commandes récentes
rtk gain --graph                # Graphique ASCII des savings quotidiens
```

### Identifier Opportunités Manquées

```bash
rtk discover                    # Analyse l'historique pour trouver les commandes non-optimisées
rtk discover --all              # Tous les projets
```

---

## Quand NE PAS Utiliser RTK

### Commandes Interactives

```bash
# ❌ Éviter pour les commandes interactives
npm init                        # Pas rtk npm init
git add -p                      # Pas rtk git add -p
```

### Commandes Déjà Compactes

```bash
# ⚠️ Optionnel (peu de gain)
git status                      # Déjà compact
ls file.txt                     # Single file
```

### Commandes de Modification

```bash
# ✅ Utiliser directement (pas de sortie verbose à filtrer)
git commit -m "message"         # Pas besoin de rtk
npm install package             # Pas besoin de rtk
```

---

## Vérification Automatique

Le plugin RTK pour OpenCode **réécrit automatiquement** les commandes bash. Vous n'avez **pas besoin** de préfixer manuellement avec `rtk` — le plugin le fait pour vous.

**Cependant**, connaître les commandes RTK aide à :
1. Comprendre quelles optimisations sont appliquées
2. Utiliser RTK en dehors d'OpenCode (terminal standard)
3. Déboguer si une commande ne fonctionne pas comme attendu

---

## Ressources

- **Documentation RTK** : [rtk-ai.app](https://www.rtk-ai.app/)
- **GitHub** : [github.com/rtk-ai/rtk](https://github.com/rtk-ai/rtk)
- **Plugin OpenCode Hub** : `plugins/rtk/README.md`

---

## Exemples de Workflows

### Workflow 1 : Audit de Code

```bash
# 1. Voir la structure du projet
rtk tree src/ --depth 2

# 2. Rechercher un pattern
rtk grep "TODO" src/

# 3. Lire un fichier suspect
rtk read src/components/UserForm.tsx

# 4. Voir les modifications récentes
rtk git log --oneline -20

# Tokens économisés : ~70% vs commandes standard
```

### Workflow 2 : Debug de Tests

```bash
# 1. Lancer les tests avec sortie compacte
rtk vitest run

# 2. Inspecter le fichier de test qui échoue
rtk read src/utils/validator.test.ts

# 3. Voir le diff des dernières modifications
rtk git diff HEAD~3 HEAD -- src/utils/validator.ts

# Tokens économisés : ~80% vs commandes standard
```

### Workflow 3 : Revue de PR

```bash
# 1. Voir les fichiers modifiés
rtk git diff --name-status main...feature-branch

# 2. Diff complet filtré
rtk git diff main...feature-branch

# 3. Vérifier le build
rtk tsc --noEmit

# 4. Lancer les tests
rtk jest --coverage

# Tokens économisés : ~85% vs commandes standard
```

---

**Dernière mise à jour :** RTK 0.42.0 (2026-05-29)
