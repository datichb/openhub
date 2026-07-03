# Architecture : Adapters

Un **adapter** traduit les agents canoniques du hub vers le format natif
d'un outil IA cible (ex : opencode).

---

## Contrat obligatoire

Tout adapter (`scripts/adapters/<cible>.adapter.sh`) doit exporter **9 fonctions**.
Le chargement est effectué par `load_adapter()` dans `scripts/lib/adapter-manager.sh`,
qui vérifie via `declare -F` que les 9 fonctions existent après le `source`.

| Fonction | Rôle | Signature |
|----------|------|-----------|
| `adapter_validate` | Vérifie que l'outil cible est installé et accessible | `adapter_validate()` — retourne 0/1 |
| `adapter_needs_node` | Indique si Node.js est requis pour l'outil | `adapter_needs_node()` — `return 0` (oui) ou `return 1` (non) |
| `adapter_deploy_files` | **Phase 1** — Copie les agents canoniques vers le projet cible | `adapter_deploy_files deploy_dir project_id [provider_override]` |
| `adapter_deploy_skills` | **Phase 2** — Déploie les skills natives dans `.opencode/skills/` | `adapter_deploy_skills deploy_dir project_id` |
| `adapter_deploy_config` | **Phase 3** — Applique la configuration provider/model (ex: `opencode.json`) | `adapter_deploy_config deploy_dir project_id [provider_override]` |
| `adapter_deploy` | Wrapper de compatibilité — enchaîne Phase 1 + Phase 2 + Phase 3 | `adapter_deploy deploy_dir project_id [provider_override]` |
| `adapter_install` | Installe l'outil cible (appelé par `oh install`) | `adapter_install()` |
| `adapter_update` | Met à jour l'outil cible (appelé par `oh update`) | `adapter_update()` |
| `adapter_start` | Lance l'outil dans le projet (appelé par `oh start`) | `adapter_start project_path prompt project_id` |

### Séparation des phases

`oh deploy` exécute les trois phases séquentiellement en affichant une section visuelle distincte pour chacune :

```
▶  Phase 1 — Copie des agents
◆  12 agent(s) déployés

▶  Phase 2 — Déploiement des skills
◆  8 skills déployées

▶  Phase 3 — Configuration provider / model
◆  opencode.json  (modèle : amazon-bedrock/..., provider : bedrock)
```

`oh start --provider <provider>` n'exécute que **la Phase 3** lorsque les agents
sont déjà en place — les Phases 1 et 2 sont inutiles dans ce cas.

### Détail des paramètres

#### `adapter_deploy_files deploy_dir project_id [provider_override]`

- `deploy_dir` : chemin du répertoire projet où déployer (ex: `/home/user/mon-projet`)
- `project_id` : identifiant du projet dans `projects.md` (ex: `MON-PROJET`). Permet de
  lire la langue (`get_project_language`) et les filtres agents (`should_deploy_agent`).
- `provider_override` : ignoré en Phase 1 — présent pour homogénéité de signature.

Responsabilités :
1. Créer l'arborescence de sortie (ex : `.opencode/agents/`)
2. Charger les métadonnées agents via `_load_agent_metadata` (scan sans écriture)
3. Pour chaque agent retenu : appeler `build_agent_content` et écrire le fichier `.md`
4. Remplir les variables globales `_DEPLOY_FILES_AGENT_KEYS/VALS/FILES/COUNT`
5. Exposer `_DEPLOY_PRECOMPUTED_STACKS` pour réutilisation par la Phase 2 (évite un double calcul)

#### `adapter_deploy_skills deploy_dir project_id`

- `deploy_dir` : chemin du répertoire projet
- `project_id` : identifiant du projet (pour la détection de stack si Phase 1 non exécutée)

Responsabilités :
1. Réutiliser `_DEPLOY_PRECOMPUTED_STACKS` exposé par Phase 1 — ou recalculer si appel autonome
2. Collecter toutes les skills natives uniques : `native_skills` des frontmatters agents + stack skills résolus
3. Déployer chaque skill dans `.opencode/skills/<name>/SKILL.md`
4. Exposer `_DEPLOY_NATIVE_SKILLS_COUNT` et `_DEPLOY_NATIVE_SKILLS_SKIPPED`

**Autonomie :** cette fonction est appelable seule sans avoir exécuté `adapter_deploy_files`
au préalable — elle charge elle-même les métadonnées et recalcule les stacks si nécessaire.

#### `adapter_deploy_config deploy_dir project_id [provider_override]`

- `deploy_dir` : chemin du répertoire projet
- `project_id` : identifiant du projet (pour résoudre le provider et la clé API)
- `provider_override` : override du provider (ex: `bedrock`, `anthropic`)

Responsabilités :
1. Charger les métadonnées agents si `_DEPLOY_FILES_AGENT_KEYS` est vide (appel direct sans Phase 1)
2. Résoudre le modèle et le provider effectifs
3. Construire et écrire le fichier de configuration (ex: `opencode.json`)
4. Exposer `_DEPLOY_CONFIG_CLAMPS` : nombre d'agents dont le plancher modèle a été appliqué
5. Pour les adapters sans configuration : no-op explicite

**Autonomie :** cette fonction est appelable seule sans avoir exécuté `adapter_deploy_files`
au préalable — elle charge elle-même les métadonnées nécessaires.

#### `adapter_deploy deploy_dir project_id [provider_override]`

Wrapper de compatibilité qui enchaîne `adapter_deploy_files`, `adapter_deploy_skills` puis `adapter_deploy_config`.
Utilisé par `cmd-deploy.sh --diff`, `cmd-sync.sh`, `cmd-provider.sh` et les tests.

#### `adapter_start project_path prompt project_id`

- `project_path` : chemin absolu du répertoire projet
- `prompt` : prompt initial (peut être vide)
- `project_id` : identifiant du projet (pour configuration spécifique)

---

## Fonctions utilitaires disponibles

Un adapter a accès aux fonctions de `common.sh` et `prompt-builder.sh` :

| Fonction | Usage |
|----------|-------|
| `extract_frontmatter_value file key` | Lit une valeur du frontmatter YAML |
| `extract_frontmatter_list file key` | Parse une liste YAML inline → une valeur par ligne |
| `strip_frontmatter file` | Retourne le corps sans le frontmatter |
| `get_agent_id file` | Retourne l'`id` du frontmatter |
| `get_agent_mode file` | Retourne le `mode` du frontmatter (`primary` par défaut) |
| `get_effective_agent_mode file project_id` | Mode effectif : override projet > frontmatter > `primary` |
| `build_agent_content file [target] [lang]` | Assemble le contenu complet (header + skills + corps) |
| `get_project_language project_id` | Retourne la langue du projet (ou vide) |
| `get_project_api_provider project_id` | Retourne le provider API (anthropic, litellm, etc.) |
| `get_project_api_key project_id` | Retourne la clé API |
| `get_project_api_base_url project_id` | Retourne la base URL (ou vide) |

### Variables globales remplies par `adapter_deploy_files` / `_load_agent_metadata`

Ces variables sont disponibles pour `adapter_deploy_skills` et `adapter_deploy_config` après la Phase 1 :

| Variable | Contenu |
|----------|---------|
| `_DEPLOY_FILES_AGENT_KEYS` | Tableau des `agent_id` retenus |
| `_DEPLOY_FILES_AGENT_VALS` | Tableau des modes effectifs (`primary`, `subagent`, …) |
| `_DEPLOY_FILES_AGENT_FILES` | Tableau des chemins source canoniques |
| `_DEPLOY_FILES_COUNT` | Nombre d'agents retenus |
| `_DEPLOY_PRECOMPUTED_STACKS` | Stack skills précalculés (réutilisés par Phase 2) |

Variables exposées par `adapter_deploy_skills` après la Phase 2 :

| Variable | Contenu |
|----------|---------|
| `_DEPLOY_NATIVE_SKILLS_COUNT` | Nombre de skills natives déployées |
| `_DEPLOY_NATIVE_SKILLS_SKIPPED` | Nombre de skills ignorées (source introuvable) |

Variables exposées par `adapter_deploy_config` après la Phase 3 :

| Variable | Contenu |
|----------|---------|
| `_DEPLOY_CONFIG_MODEL` | Modèle résolu global |
| `_DEPLOY_CONFIG_PROVIDER` | Provider effectif |
| `_DEPLOY_CONFIG_SIZE` | Taille du fichier `opencode.json` généré |
| `_DEPLOY_CONFIG_TOTAL` | Nombre total d'agents configurés |
| `_DEPLOY_CONFIG_SUBAGENTS` | Nombre d'agents en mode subagent |
| `_DEPLOY_CONFIG_DISABLED` | Nombre d'agents natifs désactivés |
| `_DEPLOY_CONFIG_PERMS` | Nombre d'agents avec permissions restreintes |
| `_DEPLOY_CONFIG_CLAMPS` | Nombre d'agents dont le plancher modèle a été appliqué |
| `_DEPLOY_CONFIG_SKIP` | `true` si `opencode.json` était déjà à jour (pas de réécriture) |

---

## Créer un nouvel adapter

1. Créer `scripts/adapters/<cible>.adapter.sh` avec les **8 fonctions** du contrat
2. Le fichier sera chargé automatiquement par `load_adapter` — aucune modification de
   `adapter-manager.sh` n'est nécessaire
3. Tester : `oh deploy <cible>` puis vérifier les fichiers générés

### Exemple minimal

```bash
#!/bin/bash
# scripts/adapters/mon-outil.adapter.sh

adapter_validate() {
  command -v mon-outil &>/dev/null || { log_error "mon-outil non installé"; return 1; }
}

adapter_needs_node() { return 1; }

# Phase 1 : copie des fichiers agents
adapter_deploy_files() {
  local deploy_dir="${1:-$HUB_DIR}"
  local project_id="${2:-}"
  local out_dir="$deploy_dir/.mon-outil/agents"
  mkdir -p "$out_dir"

  local lang=""
  [ -n "$project_id" ] && lang=$(get_project_language "$project_id")

  _DEPLOY_FILES_AGENT_KEYS=()
  _DEPLOY_FILES_AGENT_VALS=()
  _DEPLOY_FILES_AGENT_FILES=()
  _DEPLOY_FILES_COUNT=0
  _DEPLOY_PRECOMPUTED_STACKS=""

  while IFS= read -r f; do
    [ -f "$f" ] || continue
    local agent_id; agent_id=$(get_agent_id "$f")
    should_deploy_agent "$project_id" "$agent_id" || continue
    build_agent_content "$f" "mon-outil" "$lang" "$deploy_dir" > "$out_dir/${agent_id}.md"
    local eff_mode; eff_mode=$(get_effective_agent_mode "$f" "$project_id")
    _DEPLOY_FILES_AGENT_KEYS+=("$agent_id")
    _DEPLOY_FILES_AGENT_VALS+=("$eff_mode")
    _DEPLOY_FILES_AGENT_FILES+=("$f")
    _DEPLOY_FILES_COUNT=$((_DEPLOY_FILES_COUNT + 1))
  done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
}

# Phase 2 : déploiement des skills natives (no-op si non applicable)
adapter_deploy_skills() {
  _DEPLOY_NATIVE_SKILLS_COUNT=0
  _DEPLOY_NATIVE_SKILLS_SKIPPED=0
  # Implémenter deploy_native_skills si l'outil supporte les skills chargées à la demande
}

# Phase 3 : configuration provider/model (no-op si non applicable)
adapter_deploy_config() {
  log_info "  Aucune configuration provider/model à appliquer (non supporté par mon-outil)"
}

# Wrapper de compatibilité
adapter_deploy() {
  adapter_deploy_files "${1:-}" "${2:-}" "${3:-}"
  adapter_deploy_skills "${1:-}" "${2:-}"
  adapter_deploy_config "${1:-}" "${2:-}" "${3:-}"
}

adapter_install() {
  log_info "Installation de mon-outil..."
  # ...
}

adapter_update() {
  log_info "Mise à jour de mon-outil..."
  # ...
}

adapter_start() {
  local project_path="$1" prompt="${2:-}" project_id="${3:-}"
  cd "$project_path" || exit 1
  exec mon-outil
}
```

---

## Adapters existants

| Cible | Fichier | Node requis | Spécificités |
|-------|---------|-------------|--------------|
| opencode | `opencode.adapter.sh` | Oui | Phase 1 : `.opencode/agents/*.md` — Phase 2 : `.opencode/skills/<name>/SKILL.md` — Phase 3 : `opencode.json` (provider, model, modes subagent, permissions, agents désactivés) |

### Comportement par mode selon la cible

| Mode agent | opencode |
|-----------|----------|
| `primary` | Déployé normalement, absent du bloc `"agent":` |
| `subagent` | Déployé normalement, listé dans `"agent": { "mode": "subagent" }` |
