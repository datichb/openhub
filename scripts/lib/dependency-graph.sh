#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/dependency-graph.sh — Graphe de dépendances inter-fichiers TS/JS
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/dependency-graph.sh"
#
#   # Générer le graphe lors d'un déploiement
#   generate_dependency_graph "$PROJECT_PATH"
#
#   # Vérifier l'existence
#   if depgraph_exists "$PROJECT_PATH"; then ...
#
#   # Obtenir les importeurs d'un fichier
#   depgraph_get_imported_by "$PROJECT_PATH" "src/services/user.service.ts"
#
#   # Détecter un conflit potentiel entre deux fichiers
#   depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"
#
# Format de .opencode/dependency-graph.json :
#   {
#     "version": "1.0",
#     "generated_at": "2026-05-28T10:30:00Z",
#     "root": "/chemin/projet",
#     "stats": { "files_scanned": 42, "total_imports": 128 },
#     "nodes": {
#       "src/services/user.service.ts": {
#         "imports": ["src/repositories/user.repository.ts"],
#         "imported_by": ["src/controllers/user.controller.ts"]
#       }
#     }
#   }
#
# Limites :
#   - TypeScript, JavaScript, TSX et JSX uniquement
#   - Imports relatifs uniquement (commence par ./ ou ../)
#   - Regex simplifié (pas d'AST) — précision ~90%
#   - Limite de 2000 fichiers pour éviter les scans trop longs
#
# Compatible bash 3.2 (macOS).
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_DEPGRAPH_LOADED:-}" ] && return 0
_DEPGRAPH_LOADED=1

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
_DEPGRAPH_SUBDIR=".opencode"
_DEPGRAPH_FILENAME="dependency-graph.json"
_DEPGRAPH_MAX_FILES=2000
_DEPGRAPH_EXTENSIONS="ts tsx js jsx mts mjs"

# ─────────────────────────────────────────
# HELPERS INTERNES
# ─────────────────────────────────────────

# Retourne le chemin absolu du fichier graphe pour un projet
depgraph_file_path() {
  local project_path="${1:?depgraph_file_path: project_path requis}"
  printf '%s/%s/%s' "$project_path" "$_DEPGRAPH_SUBDIR" "$_DEPGRAPH_FILENAME"
}

# Génère un timestamp ISO8601 UTC
_depgraph_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Échappe une chaîne pour JSON
_depgraph_escape_json() {
  local s
  s=$(printf '%s' "$1")
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  printf '%s' "$s"
}

# Extrait les imports relatifs d'un fichier TS/JS
# Retourne une ligne par import, chemin relatif brut (sans extension obligatoire)
# Usage : _depgraph_extract_imports "/path/to/file.ts"
_depgraph_extract_imports() {
  local filepath="${1:?}"
  [ -f "$filepath" ] || return 0

  # Pattern 1 : import ... from './relative' ou from "../relative"
  # Pattern 2 : require('./relative') ou require("../relative")
  # Pattern 3 : import('./relative')
  # On capture uniquement les chemins relatifs (commençant par . ou ..)
  grep -oE "(from|require|import)\s*\(?['\"](\./|\.\./)[^'\"]+['\"]" "$filepath" 2>/dev/null \
    | grep -oE "['\"](\./|\.\./)[^'\"]+" \
    | sed "s/^['\"/]*//" \
    | sed "s/['\"]$//"
}

# Résout un chemin d'import relatif en chemin depuis la racine du projet
# Usage : resolved=$(_depgraph_resolve_import "$project_path" "$importer_file" "$import_path")
# Exemple : _depgraph_resolve_import "/proj" "src/a/b.ts" "../c.ts" → "src/c.ts"
_depgraph_resolve_import() {
  local project_path="${1:?}"
  local importer="${2:?}"  # relatif à project_path
  local import_path="${3:?}"

  # Répertoire de l'importeur
  local importer_dir
  importer_dir=$(dirname "$importer")

  # Construire le chemin combiné
  local combined="${importer_dir}/${import_path}"

  # Normaliser (résoudre .. et .)
  # On passe par le système de fichiers si possible, sinon normalisation manuelle
  local resolved
  if [ -e "${project_path}/${combined}" ]; then
    # Le fichier existe tel quel
    resolved=$(cd "${project_path}" && realpath --relative-to="." "${combined}" 2>/dev/null || python3 -c "import os,sys; print(os.path.normpath(sys.argv[1]))" "$combined" 2>/dev/null || printf '%s' "$combined")
  else
    # Tenter d'ajouter les extensions courantes
    local ext found=""
    for ext in ts tsx js jsx mts mjs; do
      if [ -e "${project_path}/${combined}.${ext}" ]; then
        combined="${combined}.${ext}"
        found=true
        break
      fi
    done
    # Tenter index.*
    if [ -z "$found" ]; then
      for ext in ts tsx js jsx mts mjs; do
        if [ -e "${project_path}/${combined}/index.${ext}" ]; then
          combined="${combined}/index.${ext}"
          break
        fi
      done
    fi
    resolved=$(printf '%s' "$combined" | sed 's|/\./|/|g' | sed 's|[^/]*/\.\./||g' | sed 's|^\./||')
  fi

  printf '%s' "$resolved"
}

# ─────────────────────────────────────────
# GÉNÉRATION DU GRAPHE (mep.4.3)
# ─────────────────────────────────────────

# Génère .opencode/dependency-graph.json pour un projet
#
# Usage :
#   generate_dependency_graph "$PROJECT_PATH"
#
# Arguments :
#   $1 - project_path : chemin absolu du projet
#
# Retourne 0 si succès, 1 si erreur ou pas de fichiers TS/JS
generate_dependency_graph() {
  local project_path="${1:?generate_dependency_graph: project_path requis}"

  # S'assurer que le dossier .opencode existe
  local cache_dir="${project_path}/${_DEPGRAPH_SUBDIR}"
  if [ ! -d "$cache_dir" ]; then
    mkdir -p "$cache_dir"
  fi

  local graph_file
  graph_file=$(depgraph_file_path "$project_path")
  local tmp_file="${graph_file}.tmp.$$"
  local generated_at
  generated_at=$(_depgraph_timestamp)

  # Collecter les fichiers TS/JS du projet (hors node_modules, dist, build, .opencode)
  local files=()
  local file_count=0

  # Construire l'expression find pour les extensions supportées
  local find_expr=()
  local first_ext=true
  for ext in $_DEPGRAPH_EXTENSIONS; do
    if [ "$first_ext" = true ]; then
      find_expr+=("-name" "*.${ext}")
      first_ext=false
    else
      find_expr+=("-o" "-name" "*.${ext}")
    fi
  done

  while IFS= read -r f; do
    [ "$file_count" -ge "$_DEPGRAPH_MAX_FILES" ] && break
    # Chemin relatif depuis project_path
    local rel_path="${f#${project_path}/}"
    files+=("$rel_path")
    file_count=$((file_count + 1))
  done < <(
    find "$project_path" \
      -not -path "*/node_modules/*" \
      -not -path "*/.git/*" \
      -not -path "*/dist/*" \
      -not -path "*/build/*" \
      -not -path "*/.opencode/*" \
      -not -path "*/coverage/*" \
      -not -path "*/.nuxt/*" \
      -not -path "*/.next/*" \
      -not -path "*/vendor/*" \
      \( "${find_expr[@]}" \) \
      -type f 2>/dev/null | sort
  )

  # Aucun fichier trouvé → pas de graphe
  if [ "${#files[@]}" -eq 0 ]; then
    return 1
  fi

  # Phase 1 : construire le mapping imports pour chaque fichier
  # Les nœuds JSON sont écrits directement dans tmp_file au fur et à mesure
  # (évite la concaténation O(n²) d'une grande string bash)

  local total_imports=0
  local nodes_tmp_file="${graph_file}.nodes.tmp.$$"
  local first_node=true

  local f rel_path
  for rel_path in "${files[@]}"; do
    local full_path="${project_path}/${rel_path}"
    [ -f "$full_path" ] || continue

    # Extraire les imports de ce fichier
    local raw_imports=()
    while IFS= read -r imp; do
      [ -n "$imp" ] && raw_imports+=("$imp")
    done < <(_depgraph_extract_imports "$full_path")

    # Résoudre chaque import en chemin relatif au projet
    local resolved_imports=()
    local imp
    for imp in "${raw_imports[@]}"; do
      local resolved
      resolved=$(_depgraph_resolve_import "$project_path" "$rel_path" "$imp")
      # Ne garder que les imports qui correspondent à des fichiers existants
      if [ -f "${project_path}/${resolved}" ]; then
        resolved_imports+=("$resolved")
        total_imports=$((total_imports + 1))
      fi
    done

    # Construire le JSON pour ce nœud (imports seulement, imported_by sera ajouté en Phase 2)
    local imports_json="["
    local first_import=true
    local ri
    for ri in "${resolved_imports[@]}"; do
      local escaped
      escaped=$(_depgraph_escape_json "$ri")
      if [ "$first_import" = true ]; then
        imports_json="${imports_json}\"${escaped}\""
        first_import=false
      else
        imports_json="${imports_json},\"${escaped}\""
      fi
    done
    imports_json="${imports_json}]"

    local escaped_rel
    escaped_rel=$(_depgraph_escape_json "$rel_path")

    # Append direct dans le fichier temporaire — O(1) par nœud au lieu de O(n) en bash string
    if [ "$first_node" = true ]; then
      printf '"%s":{"imports":%s,"imported_by":[]}' "$escaped_rel" "$imports_json" >> "$nodes_tmp_file"
      first_node=false
    else
      printf ',"%s":{"imports":%s,"imported_by":[]}' "$escaped_rel" "$imports_json" >> "$nodes_tmp_file"
    fi
  done

  # Phase 2 : construire les imported_by via post-processing jq
  # Assembler le JSON complet depuis le fichier de nœuds (pas de string multi-Mo en mémoire bash)
  local nodes_content=""
  [ -f "$nodes_tmp_file" ] && nodes_content=$(cat "$nodes_tmp_file")
  rm -f "$nodes_tmp_file"

  local pre_json
  pre_json=$(printf '{"version":"1.0","generated_at":"%s","root":"%s","stats":{"files_scanned":%d,"total_imports":%d},"nodes":{%s}}' \
    "$(_depgraph_escape_json "$generated_at")" \
    "$(_depgraph_escape_json "$project_path")" \
    "${#files[@]}" \
    "$total_imports" \
    "$nodes_content")

  # Utiliser jq pour calculer les imported_by (inversion du graphe imports)
  local final_json
  final_json=$(printf '%s' "$pre_json" | jq '
    . as $graph |
    .nodes |= (
      . as $nodes |
      reduce (
        $nodes | to_entries[] |
        .key as $src |
        .value.imports[] |
        { target: ., src: $src }
      ) as $edge (
        $nodes;
        if has($edge.target) then
          .[$edge.target].imported_by += [$edge.src]
        else
          .
        end
      )
    )
  ' 2>/dev/null) || final_json="$pre_json"

  # Écriture atomique
  printf '%s\n' "$final_json" > "$tmp_file"

  # Valider JSON avant de placer
  if ! jq '.' "$tmp_file" &>/dev/null; then
    rm -f "$tmp_file"
    return 1
  fi

  mv "$tmp_file" "$graph_file"
  return 0
}

# ─────────────────────────────────────────
# LECTURES ET REQUÊTES
# ─────────────────────────────────────────

# Vérifie si le graphe existe pour un projet
depgraph_exists() {
  local project_path="${1:?}"
  [ -f "$(depgraph_file_path "$project_path")" ]
}

# Retourne la liste des fichiers qui importent un fichier donné
# Usage : depgraph_get_imported_by "$PROJECT_PATH" "src/services/user.service.ts"
# Retourne une ligne par fichier importeur
depgraph_get_imported_by() {
  local project_path="${1:?}"
  local target_file="${2:?}"
  local graph_file
  graph_file=$(depgraph_file_path "$project_path")
  [ -f "$graph_file" ] || return 1

  local escaped
  escaped=$(_depgraph_escape_json "$target_file")
  jq -r --arg f "$target_file" '.nodes[$f].imported_by[]? // empty' "$graph_file" 2>/dev/null
}

# Retourne la liste des fichiers importés par un fichier donné
# Usage : depgraph_get_imports "$PROJECT_PATH" "src/controllers/user.controller.ts"
depgraph_get_imports() {
  local project_path="${1:?}"
  local source_file="${2:?}"
  local graph_file
  graph_file=$(depgraph_file_path "$project_path")
  [ -f "$graph_file" ] || return 1

  jq -r --arg f "$source_file" '.nodes[$f].imports[]? // empty' "$graph_file" 2>/dev/null
}

# Détecte si deux fichiers sont liés (l'un importe l'autre, directement)
# Retourne 0 si liés, 1 sinon
# Usage : if depgraph_are_linked "$PROJECT_PATH" "src/a.ts" "src/b.ts"; then
depgraph_are_linked() {
  local project_path="${1:?}"
  local file_a="${2:?}"
  local file_b="${3:?}"
  local graph_file
  graph_file=$(depgraph_file_path "$project_path")
  [ -f "$graph_file" ] || return 1

  # Vérifier si A importe B
  if jq -e --arg a "$file_a" --arg b "$file_b" \
    '.nodes[$a].imports[]? == $b' "$graph_file" &>/dev/null; then
    return 0
  fi

  # Vérifier si B importe A
  if jq -e --arg a "$file_a" --arg b "$file_b" \
    '.nodes[$b].imports[]? == $a' "$graph_file" &>/dev/null; then
    return 0
  fi

  return 1
}

# Retourne les stats du graphe
# Usage : depgraph_stats "$PROJECT_PATH"
depgraph_stats() {
  local project_path="${1:?}"
  local graph_file
  graph_file=$(depgraph_file_path "$project_path")
  [ -f "$graph_file" ] || return 1
  jq -r '"Fichiers scannés: \(.stats.files_scanned) | Imports trouvés: \(.stats.total_imports)"' \
    "$graph_file" 2>/dev/null
}
