#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# lib/context-cache.sh — Cache de contexte projet
# ─────────────────────────────────────────────────────────────────────────────
# Usage :
#   source "$LIB_DIR/context-cache.sh"
#
#   # Générer le cache après l'onboarding
#   generate_context_cache "$PROJECT_PATH" '{"languages":["typescript"]}' "CONVENTIONS.md"
#
#   # Valider le cache au démarrage
#   if validate_context_cache "$PROJECT_PATH"; then
#     echo "Cache valide"
#   else
#     echo "Cache invalide ou absent"
#   fi
#
#   # Vérifier l'existence
#   if cache_exists "$PROJECT_PATH"; then ...
#
#   # Lire les infos du cache (pour affichage)
#   cache_get_generated_at "$PROJECT_PATH"   # retourne la date ISO
#   cache_get_stack "$PROJECT_PATH"          # retourne le JSON stack
#   cache_file_path "$PROJECT_PATH"          # retourne le chemin du fichier
#
# Format de .opencode/context.json :
#   {
#     "version": "1.0",
#     "generated_at": "2026-05-28T10:30:00Z",
#     "stack": { "languages": [...], "frameworks": [...] },
#     "conventions": { "source": "CONVENTIONS.md", "hash": "sha256:..." },
#     "key_files": {
#       "package.json": "sha256:...",
#       "tsconfig.json": "sha256:..."
#     }
#   }
#
# Compatible bash 3.2 (macOS).
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_CONTEXT_CACHE_LOADED:-}" ] && return 0
_CONTEXT_CACHE_LOADED=1

# ─────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────
_CACHE_SUBDIR=".opencode"
_CACHE_FILENAME="context.json"
_CACHE_VERSION="1.0"

# Fichiers structurants à hasher par défaut (si trouvés dans le projet)
_CACHE_DEFAULT_KEY_FILES=(
  "package.json"
  "tsconfig.json"
  "tsconfig.base.json"
  "pyproject.toml"
  "Cargo.toml"
  "go.mod"
  "composer.json"
  "pom.xml"
  "build.gradle"
  "Gemfile"
  "requirements.txt"
  ".eslintrc.json"
  "eslint.config.js"
  "eslint.config.mjs"
  ".prettierrc"
  ".prettierrc.json"
  "biome.json"
  "CONVENTIONS.md"
  "ONBOARDING.md"
)

# ─────────────────────────────────────────
# HELPERS INTERNES
# ─────────────────────────────────────────

# Retourne le chemin absolu du fichier cache pour un projet
# Usage : path=$(cache_file_path "$project_path")
cache_file_path() {
  local project_path="${1:?cache_file_path: project_path requis}"
  printf '%s/%s/%s' "$project_path" "$_CACHE_SUBDIR" "$_CACHE_FILENAME"
}

# Calcule le hash SHA-256 d'un fichier (compatible macOS et Linux)
# Usage : hash=$(_cache_hash_file "/path/to/file")
# Retourne "" si le fichier n'existe pas
_cache_hash_file() {
  local filepath="${1:?_cache_hash_file: filepath requis}"
  if [ ! -f "$filepath" ]; then
    printf ''
    return 1
  fi
  # macOS : shasum -a 256 / Linux : sha256sum
  if command -v shasum &>/dev/null; then
    shasum -a 256 "$filepath" | awk '{print "sha256:" $1}'
  elif command -v sha256sum &>/dev/null; then
    sha256sum "$filepath" | awk '{print "sha256:" $1}'
  else
    # Fallback : taille du fichier (moins fiable mais évite l'échec)
    printf 'size:%s' "$(wc -c < "$filepath" | tr -d ' ')"
  fi
}

# Échappe une chaîne pour JSON
# Usage : escaped=$(_cache_escape_json "valeur")
_cache_escape_json() {
  local s
  s=$(printf '%s' "$1")
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  # shellcheck disable=SC2016
  s=${s//$'\n'/'\n'}
  # shellcheck disable=SC2016
  s=${s//$'\t'/'\t'}
  printf '%s' "$s"
}

# Génère un timestamp ISO8601 UTC
_cache_timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# S'assure que le dossier .opencode existe dans le projet
_cache_ensure_dir() {
  local project_path="${1:?}"
  local cache_dir="${project_path}/${_CACHE_SUBDIR}"
  if [ ! -d "$cache_dir" ]; then
    mkdir -p "$cache_dir"
  fi
}

# ─────────────────────────────────────────
# GÉNÉRATION DU CACHE (mep.4.1)
# ─────────────────────────────────────────

# Génère .opencode/context.json pour un projet
#
# Usage :
#   generate_context_cache "$PROJECT_PATH" \
#     '{"languages":["typescript"],"frameworks":["vue"]}' \
#     "CONVENTIONS.md"
#
# Arguments :
#   $1 - project_path  : chemin absolu du projet
#   $2 - stack_json    : JSON de la stack détectée (ex: '{"languages":[...]}')
#                        Peut être "" pour une stack vide
#   $3 - conventions_source : nom du fichier conventions (ex: "CONVENTIONS.md")
#                        Peut être "" si pas de fichier conventions
#
# Retourne 0 si succès, 1 si erreur
generate_context_cache() {
  local project_path="${1:?generate_context_cache: project_path requis}"
  local stack_json="${2:-{\}}"
  local conventions_source="${3:-}"

  _cache_ensure_dir "$project_path"

  local cache_file
  cache_file=$(cache_file_path "$project_path")
  local tmp_file="${cache_file}.tmp.$$"
  local generated_at
  generated_at=$(_cache_timestamp)

  # Valider que stack_json est du JSON valide (fallback si vide ou invalide)
  if [ -z "$stack_json" ] || ! printf '%s' "$stack_json" | jq '.' &>/dev/null; then
    stack_json='{}'
  fi

  # Construire le bloc "conventions"
  local conventions_block='null'
  if [ -n "$conventions_source" ] && [ -f "${project_path}/${conventions_source}" ]; then
    local conv_hash
    conv_hash=$(_cache_hash_file "${project_path}/${conventions_source}")
    local conv_source_escaped
    conv_source_escaped=$(_cache_escape_json "$conventions_source")
    local conv_hash_escaped
    conv_hash_escaped=$(_cache_escape_json "$conv_hash")
    conventions_block="{\"source\":\"${conv_source_escaped}\",\"hash\":\"${conv_hash_escaped}\"}"
  fi

  # Construire le bloc "key_files" — scanner les fichiers structurants existants
  local key_files_json='{'
  local first=true
  local f hash escaped_name escaped_hash

  for f in "${_CACHE_DEFAULT_KEY_FILES[@]}"; do
    local full_path="${project_path}/${f}"
    if [ -f "$full_path" ]; then
      hash=$(_cache_hash_file "$full_path")
      escaped_name=$(_cache_escape_json "$f")
      escaped_hash=$(_cache_escape_json "$hash")
      if [ "$first" = true ]; then
        key_files_json="${key_files_json}\"${escaped_name}\":\"${escaped_hash}\""
        first=false
      else
        key_files_json="${key_files_json},\"${escaped_name}\":\"${escaped_hash}\""
      fi
    fi
  done
  key_files_json="${key_files_json}}"

  # Écriture atomique via fichier temporaire
  {
    printf '{\n'
    printf '  "version": "%s",\n' "$(_cache_escape_json "$_CACHE_VERSION")"
    printf '  "generated_at": "%s",\n' "$(_cache_escape_json "$generated_at")"
    printf '  "stack": %s,\n' "$stack_json"
    printf '  "conventions": %s,\n' "$conventions_block"
    printf '  "key_files": %s\n' "$key_files_json"
    printf '}\n'
  } > "$tmp_file"

  # Valider que le JSON produit est valide avant de le placer
  if ! jq '.' "$tmp_file" &>/dev/null; then
    rm -f "$tmp_file"
    return 1
  fi

  mv "$tmp_file" "$cache_file"
  return 0
}

# ─────────────────────────────────────────
# VALIDATION DU CACHE (mep.4.2)
# ─────────────────────────────────────────

# Vérifie si le cache existe pour un projet
# Usage : if cache_exists "$PROJECT_PATH"; then ...
# Retourne 0 si le fichier existe, 1 sinon
cache_exists() {
  local project_path="${1:?cache_exists: project_path requis}"
  [ -f "$(cache_file_path "$project_path")" ]
}

# Valide le cache de contexte pour un projet
#
# Usage :
#   if validate_context_cache "$PROJECT_PATH"; then
#     echo "Cache valide"
#   else
#     echo "Cache invalide"
#   fi
#
# Affiche des messages log_info/log_warn selon le cas.
# Retourne 0 si valide, 1 si invalide/absent/corrompu.
validate_context_cache() {
  local project_path="${1:?validate_context_cache: project_path requis}"
  local cache_file
  cache_file=$(cache_file_path "$project_path")

  # Cas 1 : cache absent
  if [ ! -f "$cache_file" ]; then
    return 1
  fi

  # Cas 2 : cache non parsable
  if ! jq '.' "$cache_file" &>/dev/null; then
    log_warn "Cache de contexte corrompu (JSON invalide) — régénération recommandée : oc start --onboard --refresh $PROJECT_ID"
    return 1
  fi

  # Extraire la date de génération pour le message
  local generated_at
  generated_at=$(jq -r '.generated_at // ""' "$cache_file" 2>/dev/null)

  # Cas 3 : vérifier chaque entrée de key_files
  local invalid_file=""
  local key_file hash_stored hash_current

  # Lire les key_files du cache et vérifier chaque hash
  while IFS='=' read -r key_file hash_stored; do
    [ -z "$key_file" ] && continue
    local full_path="${project_path}/${key_file}"

    # Si le fichier n'existe plus → cache invalide
    if [ ! -f "$full_path" ]; then
      invalid_file="$key_file (supprimé)"
      break
    fi

    hash_current=$(_cache_hash_file "$full_path")

    # Si le hash diffère → cache invalide
    if [ "$hash_current" != "$hash_stored" ]; then
      invalid_file="$key_file (modifié)"
      break
    fi
  done < <(jq -r '.key_files | to_entries[] | "\(.key)=\(.value)"' "$cache_file" 2>/dev/null)

  if [ -n "$invalid_file" ]; then
    log_warn "Cache de contexte invalide (${invalid_file}) — régénération : oc start --onboard --refresh"
    return 1
  fi

  # Cache valide
  if [ -n "$generated_at" ]; then
    log_info "Cache de contexte valide (généré le ${generated_at})"
  else
    log_info "Cache de contexte valide"
  fi
  return 0
}

# ─────────────────────────────────────────
# LECTURES (helpers de lecture du cache)
# ─────────────────────────────────────────

# Retourne la date de génération du cache
# Usage : date=$(cache_get_generated_at "$PROJECT_PATH")
cache_get_generated_at() {
  local project_path="${1:?}"
  local cache_file
  cache_file=$(cache_file_path "$project_path")
  [ -f "$cache_file" ] || return 1
  jq -r '.generated_at // ""' "$cache_file" 2>/dev/null
}

# Retourne le JSON de la stack détectée
# Usage : stack=$(cache_get_stack "$PROJECT_PATH")
cache_get_stack() {
  local project_path="${1:?}"
  local cache_file
  cache_file=$(cache_file_path "$project_path")
  [ -f "$cache_file" ] || return 1
  jq -c '.stack // {}' "$cache_file" 2>/dev/null
}

# Retourne le chemin du fichier de conventions (depuis le cache)
# Usage : conv=$(cache_get_conventions_source "$PROJECT_PATH")
cache_get_conventions_source() {
  local project_path="${1:?}"
  local cache_file
  cache_file=$(cache_file_path "$project_path")
  [ -f "$cache_file" ] || return 1
  jq -r '.conventions.source // ""' "$cache_file" 2>/dev/null
}

# Force la suppression du cache (utilisé par --refresh)
# Usage : cache_invalidate "$PROJECT_PATH"
cache_invalidate() {
  local project_path="${1:?}"
  local cache_file
  cache_file=$(cache_file_path "$project_path")
  if [ -f "$cache_file" ]; then
    rm -f "$cache_file"
    return 0
  fi
  return 1
}

# Injecte le champ "instructions" dans opencode.json selon l'état du cache
# Priorité : cache valide > ONBOARDING.md/CONVENTIONS.md > suppression du champ
# Usage : _inject_context_instructions "$PROJECT_PATH"
_inject_context_instructions() {
  local project_path="${1:?}"
  local config_file="$project_path/opencode.json"

  # opencode.json doit exister pour être modifiable
  [ -f "$config_file" ] || return 0

  local instructions_json="[]"
  local cache_file
  cache_file=$(cache_file_path "$project_path")

  if [ -f "$cache_file" ] && validate_context_cache "$project_path" 2>/dev/null; then
    # Cache valide — source de vérité unique
    instructions_json='[".opencode/context.json"]'
  else
    # Pas de cache valide — fallback sur les fichiers de contexte présents
    [ -f "$project_path/ONBOARDING.md" ]  && instructions_json=$(jq -n --argjson a "$instructions_json" '$a + ["ONBOARDING.md"]')
    [ -f "$project_path/CONVENTIONS.md" ] && instructions_json=$(jq -n --argjson a "$instructions_json" '$a + ["CONVENTIONS.md"]')
  fi

  # Mettre à jour opencode.json (écriture atomique)
  local _tmp_config="${config_file}.instructions.tmp"
  if [ "$instructions_json" = "[]" ]; then
    # Aucun contexte disponible — supprimer le champ instructions si présent
    jq 'del(.instructions)' "$config_file" > "$_tmp_config" && mv "$_tmp_config" "$config_file"
  else
    jq --argjson instr "$instructions_json" '. + {"instructions": $instr}' "$config_file" > "$_tmp_config" \
      && mv "$_tmp_config" "$config_file"
  fi
}
