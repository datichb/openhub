# Optimisations de performance

> Documentation des optimisations de performance implémentées dans openhub

## Vue d'ensemble

Ce document décrit les optimisations de performance apportées au workflow `oc deploy`, en particulier pour réduire le nombre d'appels subprocess et améliorer les temps d'exécution.

## Métriques

| Métrique | Avant | Après | Gain |
|----------|-------|-------|------|
| Temps deploy (30 agents) | ~40s | ~38s | -5% (-2s) |
| Appels subprocess (Phase 3) | 90+ | 30 | -67% |
| Lectures api-keys.local.md | 30+ | 1 | -97% |
| Lectures hub.json (provider) | 30 | 1 | -97% |

## Optimisations implémentées

### 1. Cache api-keys.local.md (scripts/lib/api-keys.sh)

**Problème** : Le fichier `api-keys.local.md` était lu 30+ fois lors du deploy (une fois par agent), utilisant `awk` à chaque appel.

**Solution** :
- Nouvelle fonction `api_keys_load_cache()` qui lit le fichier UNE FOIS
- Stockage des valeurs en variables globales (`_API_KEYS_CACHE_*`)
- `_api_keys_get()` utilise le cache si disponible, sinon fallback sur `awk`

**Gain** : 4s → 0s pour 30 appels (mesuré en isolation)

```bash
# Utilisation
api_keys_load_cache "PROJECT_ID"  # Charger le cache une fois
_api_keys_get "PROJECT_ID" "provider"  # Utilise le cache (instantané)
```

**Variables de cache** :
- `_API_KEYS_CACHE_LOADED` : 0|1, indique si le cache est chargé
- `_API_KEYS_CACHE_PROJECT_ID` : ID du projet en cache
- `_API_KEYS_CACHE_PROVIDER` : valeur de `provider`
- `_API_KEYS_CACHE_MODEL` : valeur de `model`
- `_API_KEYS_CACHE_KEY` : valeur de `api_key`
- `_API_KEYS_CACHE_BASE_URL` : valeur de `base_url`
- `_API_KEYS_CACHE_REGION` : valeur de `region`

### 2. Frontmatter pré-lu (scripts/lib/prompt-builder.sh)

**Problème** : Le frontmatter de chaque agent était extrait 2+ fois par agent :
- Une fois par `extract_permission_json()` avec `sed`
- Une fois par `resolve_agent_model()` via `read_agent_frontmatter()`

**Solution** :
- `read_agent_frontmatter()` capture maintenant le frontmatter brut complet dans `_fm_raw`
- `extract_permission_json()` accepte un paramètre optionnel `$2` pour recevoir le frontmatter pré-lu
- Dans `adapter_deploy_config()`, appel de `read_agent_frontmatter()` UNE FOIS, puis réutilisation de `_fm_raw`

**Gain** : 2s → 0s pour 30 appels d'`extract_permission_json` (mesuré en isolation)

```bash
# Avant (2 lectures par agent)
perm_json=$(extract_permission_json "$agent_file")  # sed + lecture
read_agent_frontmatter "$agent_file"                # lecture séparée

# Après (1 lecture par agent)
read_agent_frontmatter "$agent_file"                          # 1 seule lecture
perm_json=$(extract_permission_json "$agent_file" "$_fm_raw") # réutilise _fm_raw
```

**Variables exposées par `read_agent_frontmatter()`** :
- `_fm_id` : valeur du champ `id:`
- `_fm_skills` : valeur brute du champ `skills:` (ex: `[skill/a, skill/b]`)
- `_fm_model` : valeur du champ `model:` (optionnel)
- `_fm_raw` : **NOUVEAU** - frontmatter complet (toutes les lignes)

### 3. Bash pur au lieu de subprocesses (scripts/lib/prompt-builder.sh)

**Problème** : `_get_agent_family()` utilisait `dirname` + `basename` = 2 subprocesses par agent.

**Solution** : Utiliser l'expansion de paramètres bash (built-in, 0 subprocess)

**Gain** : 2s → 0s pour 30 appels (mesuré en isolation)

```bash
# Avant (2 subprocesses)
_get_agent_family() {
  local dir
  dir=$(dirname "$agent_file")    # subprocess 1
  basename "$dir"                 # subprocess 2
}

# Après (0 subprocess)
_get_agent_family() {
  local dir="${agent_file%/*}"    # Équivalent dirname (bash pur)
  echo "${dir##*/}"                # Équivalent basename (bash pur)
}
```

**Même optimisation dans `adapter_deploy_files()`** (scripts/adapters/opencode.adapter.sh:310) :
```bash
# Avant
local _family=$(dirname "$_asource" | xargs basename)  # 3 subprocesses !

# Après
local _dir="${_asource%/*}"       # bash pur
local _family="${_dir##*/}"        # bash pur
```

### 4. Cache get_hub_default_provider() (scripts/lib/providers.sh)

**Problème** : `hub.json` était lu 30 fois via `jq` pour obtenir le provider par défaut.

**Solution** : Mise en cache du résultat au premier appel

**Gain** : ~1s pour 30 appels (mesuré en isolation)

```bash
# Variables de cache
_HUB_DEFAULT_PROVIDER_CACHE=""
_HUB_DEFAULT_PROVIDER_CACHE_LOADED=0

get_hub_default_provider() {
  [ -f "$HUB_CONFIG" ] || return 1
  
  # Si le cache est déjà chargé, le retourner directement
  if [ "$_HUB_DEFAULT_PROVIDER_CACHE_LOADED" = "1" ]; then
    echo "$_HUB_DEFAULT_PROVIDER_CACHE"
    return 0
  fi
  
  # Charger depuis hub.json et mettre en cache
  _HUB_DEFAULT_PROVIDER_CACHE=$(jq -r '.default_provider.name // empty' "$HUB_CONFIG" 2>/dev/null)
  _HUB_DEFAULT_PROVIDER_CACHE_LOADED=1
  echo "$_HUB_DEFAULT_PROVIDER_CACHE"
}
```

### 5. Précalcul dans adapter_deploy_config() (scripts/adapters/opencode.adapter.sh)

**Optimisation** : Calcul de `_hub_default_provider` en une seule fois au début de Phase 3, au lieu de 30 fois dans la boucle.

```bash
# Précalculer les 3 niveaux hub.json + provider en une seule fois
local _hub_agent_models="" _hub_family_models="" _hub_global_model="" _hub_default_provider=""
if command -v jq &>/dev/null && [ -f "$HUB_CONFIG" ]; then
  _hub_agent_models=$(jq -r '.agent_models.agents // {} | tojson' "$HUB_CONFIG" 2>/dev/null || true)
  _hub_family_models=$(jq -r '.agent_models.families // {} | tojson' "$HUB_CONFIG" 2>/dev/null || true)
  _hub_global_model=$(jq -r '.opencode.model // empty' "$HUB_CONFIG" 2>/dev/null || true)
  _hub_default_provider=$(jq -r '.default_provider.name // empty' "$HUB_CONFIG" 2>/dev/null || true)
fi
```

## Compatibilité et fallbacks

Toutes les optimisations maintiennent une **rétrocompatibilité totale** :

1. **`_api_keys_get()`** : Si le cache n'est pas chargé, utilise l'ancienne méthode `awk`
2. **`extract_permission_json()`** : Si `$2` est vide, extrait le frontmatter avec `sed` (ancien comportement)
3. **Caches globaux** : Tous les caches ont des flags `_*_LOADED` pour détecter s'ils sont initialisés

## Profiling et mesure

### Micro-benchmarks

Les gains mesurés **en isolation** (hors contexte deploy réel) :

```bash
# Test 1 : _get_agent_family
for i in {1..30}; do _get_agent_family "agents/planning/pathfinder.md"; done
# Avant : 2s (dirname + basename)
# Après : 0s (bash pur)

# Test 2 : extract_permission_json
for i in {1..30}; do extract_permission_json "agents/planning/pathfinder.md"; done
# Avant : 2s (sed × 30)
# Après : 0s (avec _fm_raw pré-lu)

# Test 3 : _api_keys_get
api_keys_load_cache "PROJECT_ID"
for i in {1..30}; do _api_keys_get "PROJECT_ID" "provider"; done
# Avant : 4s (awk × 30)
# Après : 0s (cache)
```

### End-to-end

```bash
time ./oc.sh deploy PROJECT_ID
# Avant : ~40s
# Après : ~38s (-5%)
```

## Pistes d'optimisation futures

Pour améliorer encore les performances (objectif : -50%, soit ~20s) :

### 1. Parallélisation (gain potentiel : 10-15s)

Utiliser `xargs -P` ou GNU `parallel` pour builder les agents en parallèle :

```bash
# Concept (non implémenté)
find agents -name "*.md" | xargs -P 4 -I {} bash -c 'build_agent_content "{}"'
```

**Complexité** : Élevée
- Synchronisation des barres de progression
- Gestion d'erreurs distribuée
- Accumulation des résultats

### 2. Cache persistent (gain potentiel : 5-10s)

Stocker les résultats de calculs coûteux dans `~/.cache/opencode/` :

- Stack skills détectés (par projet)
- Frontmatters parsés (par hash de fichier)
- Résolutions de modèle

**Complexité** : Moyenne
- Invalidation du cache (modification de fichiers)
- Gestion de l'espace disque

### 3. Réécriture partielle en langage compilé (gain potentiel : 10-20s)

Réécrire les fonctions les plus coûteuses en Go/Rust :

- `detect_stack` (scan de fichiers)
- `build_agent_content` (parsing + assembly)
- `strip_frontmatter` (manipulation de texte)

**Complexité** : Très élevée
- Nouveaux binaires à compiler et distribuer
- Compatibilité cross-platform
- Maintenance de code supplémentaire

## Références

### Commits

- `fafbabd` : perf: optimize oc deploy performance (-5%)

### Fichiers modifiés

- `scripts/lib/api-keys.sh` : Cache api-keys.local.md
- `scripts/lib/prompt-builder.sh` : Frontmatter pré-lu, bash pur
- `scripts/lib/providers.sh` : Cache hub default provider
- `scripts/adapters/opencode.adapter.sh` : Utilisation des caches

### Tests de performance

Relancer les benchmarks :

```bash
# Test complet
time ./oc.sh deploy PROJECT_ID

# Test avec --no-progress (légèrement plus rapide)
time ./oc.sh deploy PROJECT_ID --no-progress
```

## Principes de performance bash

### ✅ À faire

1. **Minimiser les subprocesses** : Préférer les built-ins bash (`${var%pattern}`, `case`, `[[`)
2. **Cacher les lectures I/O** : Lire les fichiers UNE fois, stocker en mémoire
3. **Éviter les pipes multiples** : `cat file | grep x | sed y` → `sed -n '/x/s/y/p' file`
4. **Utiliser les variables globales avec parcimonie** : Documenter clairement (`_*_CACHE`)

### ❌ À éviter

1. **Subprocess dans boucle** : `for i in {1..100}; do $(dirname "$x"); done` = 100 forks !
2. **Lectures multiples du même fichier** : Cacher en variable ou array
3. **Regex complexes en bash** : Utiliser `awk`/`sed` si nécessaire, mais UNE fois
4. **Globaux non documentés** : Toujours préfixer `_` et documenter le cycle de vie

## Troubleshooting

### Le cache ne semble pas fonctionner

Vérifier que la fonction de chargement est appelée **avant** la boucle :

```bash
# ✅ Correct
api_keys_load_cache "$project_id"
for agent in ...; do
  provider=$(_api_keys_get "$project_id" "provider")  # Utilise le cache
done

# ❌ Incorrect
for agent in ...; do
  api_keys_load_cache "$project_id"  # Charge à chaque itération !
  provider=$(_api_keys_get "$project_id" "provider")
done
```

### Les performances ne s'améliorent pas

1. **Profiler à nouveau** : Les goulots peuvent avoir changé
2. **Vérifier les tests micro** : Les optimisations isolées fonctionnent-elles ?
3. **Analyser l'I/O** : Utiliser `iostat` ou `iotop` pour voir si le goulot est le disque
4. **Variables système** : Disk cache, CPU load, etc.

### Régression fonctionnelle

Tous les fallbacks sont en place. Si une optimisation cause un bug :

1. Le cache est-il bien invalidé/réinitialisé entre les invocations ?
2. La variable `_*_LOADED` est-elle bien à 0 au démarrage ?
3. Le fallback est-il testé (cas où cache non chargé) ?
