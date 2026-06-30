#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# project.sh — Registre projets (projects.md + paths.local.md) et native agents
#
# Usage normal : sourcé par common.sh (qui garantit l'environnement complet).
#
# Exception documentée : install.sh source ce fichier directement (ÉTAPE 4),
# en dehors de common.sh. Cette exception est intentionnelle : install.sh est
# un script autonome (distribué via curl | bash) qui ne peut pas sourcer
# common.sh sans que le repo soit déjà cloné.
#
# Prérequis obligatoires avant sourçage direct :
#   - log_info, log_warn, log_error définis
#   - PROJECTS_FILE, PROJECTS_EXAMPLE_FILE, PATHS_FILE, API_KEYS_FILE définis
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_PROJECT_LOADED:-}" ] && return 0
_PROJECT_LOADED=1

# S'assure que projects.md existe localement (copié depuis projects.example.md si absent)
ensure_projects_file() {
  if [ ! -f "$PROJECTS_FILE" ]; then
    if [ -f "$PROJECTS_EXAMPLE_FILE" ]; then
      cp "$PROJECTS_EXAMPLE_FILE" "$PROJECTS_FILE"
      log_info "projects.md créé depuis projects.example.md"
    else
      mkdir -p "$(dirname "$PROJECTS_FILE")"
      cat > "$PROJECTS_FILE" <<'PROJEOF'
# Registre des projets

<!-- FORMAT
## <PROJECT_ID>
- Nom : <nom lisible>
- Stack : <technologies>
- Board Beads : <PROJECT_ID>
- Tracker : <jira|gitlab|none>
- Labels : <liste séparée par virgules>
-->

---

*Aucun projet enregistré pour l'instant.*
*Ajouter un projet : ./oc.sh init*
PROJEOF
      log_info "projects.md créé"
    fi
  fi
}

# S'assure que paths.local.md existe localement
ensure_paths_file() {
  if [ ! -f "$PATHS_FILE" ]; then
    mkdir -p "$(dirname "$PATHS_FILE")"
    echo "# Chemins locaux (ignoré par git)" > "$PATHS_FILE"
    log_info "paths.local.md créé"
  fi
}

# S'assure que api-keys.local.md existe localement avec permissions 600
ensure_api_keys_file() {
  if [ ! -f "$API_KEYS_FILE" ]; then
    mkdir -p "$(dirname "$API_KEYS_FILE")"
    cat > "$API_KEYS_FILE" <<'KEYSEOF'
# Clés API par projet (ignoré par git)
# Format :
#   [PROJECT_ID]
#   model=claude-opus-4-5
#   provider=anthropic
#   api_key=sk-ant-...
#   base_url=https://...  # optionnel
KEYSEOF
    chmod 600 "$API_KEYS_FILE"
    log_info "api-keys.local.md créé (permissions 600)"
  fi
}

# Crée config/hub.json depuis hub.json.example s'il n'existe pas encore
ensure_hub_config() {
  if [ ! -f "$HUB_CONFIG" ]; then
    if [ -f "$HUB_CONFIG_EXAMPLE" ]; then
      cp "$HUB_CONFIG_EXAMPLE" "$HUB_CONFIG"
      log_info "config/hub.json créé depuis hub.json.example — configurez votre provider avec : ./oc.sh config set"
    else
      # Fallback : hub.json.example absent — générer un squelette minimal
      # Version "0.0.0" pour signaler explicitement le problème à l'utilisateur
      local _fallback_version="0.0.0"
      mkdir -p "$(dirname "$HUB_CONFIG")"
      cat > "$HUB_CONFIG" <<HUBEOF
{
  "version": "${_fallback_version}",
  "default_provider": {
    "name": "",
    "api_key": "",
    "base_url": "",
    "model": ""
  },
  "opencode": {
    "model": "${DEFAULT_MODEL}",
    "disabled_native_agents": ["build", "plan", "general", "explore", "scout"]
  },
  "cli": {
    "language": "fr"
  }
}
HUBEOF
      log_warn "config/hub.json.example introuvable — squelette généré avec version ${_fallback_version}"
      log_info "config/hub.json créé (vide) — configurez votre provider avec : ./oc.sh config set"
    fi
  fi
}

# Vérifie qu'un PROJECT_ID est fourni
require_project_id() {
  local id="${1:-}"
  if [ -z "$id" ]; then
    log_error "PROJECT_ID requis"
    exit 1
  fi
}

# Retourne le chemin local d'un projet
# Retourne 1 si paths.local.md est absent (ne fait pas exit pour permettre l'usage en subshell)
get_project_path() {
  local id="$1"
  if [ ! -f "$PATHS_FILE" ]; then
    log_warn "Fichier paths.local.md introuvable — chemin local non disponible"
    return 1
  fi
  # || true : évite que pipefail propage exit 1 si grep ne matche rien
  # head -1 : protection contre doublons dans paths.local.md
  # ^ : ancrage en début de ligne pour éviter les faux positifs (PROJ vs PROJ-FULL)
  grep "^${id}=" "$PATHS_FILE" | head -1 | cut -d'=' -f2- | tr -d ' ' || true
}

# Vérifie qu'un projet existe dans projects.md
# Utilise une comparaison de ligne exacte pour éviter les faux positifs
# (ex: "## PROJ" ne doit pas matcher "## PROJ-FR")
project_exists() {
  local id="$1"
  awk -v section="## ${id}" '$0 == section { found=1; exit } END { exit !found }' "$PROJECTS_FILE" 2>/dev/null
}

# Vérifie qu'un chemin existe dans paths.local.md
# ^ : ancrage en début de ligne pour éviter les faux positifs (PROJ vs PROJ-FULL)
path_exists() {
  local id="$1"
  grep -q "^${id}=" "$PATHS_FILE" 2>/dev/null
}

# Normalise un PROJECT_ID en majuscules
normalize_project_id() {
  echo "$1" | tr '[:lower:]' '[:upper:]'
}

# Résout le chemin local d'un projet : normalise l'ID, vérifie l'existence,
# lit paths.local.md, expand ~, vérifie le dossier. Imprime le chemin sur stdout.
# Exit 1 avec message d'erreur si une étape échoue.
# @param $1 — PROJECT_ID (sera normalisé en majuscules)
resolve_project_path() {
  local id
  id=$(normalize_project_id "$1")

  if ! project_exists "$id"; then
    log_error "Projet $id introuvable → ./oc.sh list"
    exit 1
  fi

  local path
  path=$(get_project_path "$id")
  path="${path/#\~/$HOME}"

  if [ -z "$path" ]; then
    log_error "Aucun chemin local pour $id → ./oc.sh init $id"
    exit 1
  fi

  if [ ! -d "$path" ]; then
    log_error "Dossier introuvable : $path"
    exit 1
  fi

  echo "$path"
}

# Lit un champ "- <field> : <value>" dans le bloc d'un projet de projects.md
# Usage interne — utiliser les fonctions publiques ci-dessous
# @param $1 — PROJECT_ID
# @param $2 — nom du champ (ex: "Tracker", "Langue", "Labels")
_get_project_field() {
  local id="$1" field="$2"
  # -v section : évite l'injection regex via $id (caractères spéciaux dans l'identifiant)
  awk -v section="## ${id}" -v field="$field" '
    $0 == section {found=1; next}
    found && /^## /{exit}
    found && $0 ~ "^- " field " :" {print; exit}
  ' "$PROJECTS_FILE" \
    | sed "s/^- ${field} : *//"
}

# Retourne le provider de tracker d'un projet (jira|gitlab|none)
get_project_tracker() {
  local raw
  raw=$(_get_project_field "$1" "Tracker")
  raw=$(echo "$raw" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  echo "${raw:-none}"
}

# Retourne la langue de travail d'un projet (ex: "english", "spanish")
# Retourne une chaîne vide si le champ est absent (comportement par défaut : français)
get_project_language() {
  local raw
  raw=$(_get_project_field "$1" "Langue")
  raw=$(echo "$raw" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  echo "${raw:-}"
}

# Retourne la liste des labels d'un projet (ex: "feature,fix,front,back")
# Retourne une chaîne vide si le champ est absent
get_project_labels() {
  local raw
  raw=$(_get_project_field "$1" "Labels")
  echo "${raw:-}"
}

# Retourne la liste CSV des agents sélectionnés pour un projet
# Retourne "all" si le champ est absent ou vide (= déployer tous les agents)
get_project_agents() {
  local raw
  raw=$(_get_project_field "$1" "Agents")
  echo "${raw:-all}"
}

# Retourne le mode de synchronisation d'un projet (bidirectional|pull-only|push-only)
# Retourne "bidirectional" si le champ est absent (comportement historique)
get_project_sync_mode() {
  local raw
  raw=$(_get_project_field "$1" "Sync mode")
  raw=$(echo "$raw" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  echo "${raw:-bidirectional}"
}

# Met à jour le champ "- Sync mode :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur : bidirectional | pull-only | push-only
_set_project_sync_mode() {
  local id="$1" new_mode="$2"
  # Whitelist stricte — protège contre l'injection Perl
  case "$new_mode" in
    bidirectional|pull-only|push-only) ;;
    *) log_error "Sync mode invalide : $new_mode (bidirectional | pull-only | push-only)"; return 1 ;;
  esac
  # Remplacer si le champ existe déjà
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Sync mode : [^\n]+)}{\${1}- Sync mode : ${new_mode}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Sync mode : ${new_mode}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Tracker :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Tracker : [^\n]+\n)}{\${1}- Sync mode : ${new_mode}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Sync mode : ${new_mode}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Labels :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Labels : [^\n]+\n)}{\${1}- Sync mode : ${new_mode}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Sync mode : ${new_mode}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Sync mode dans le bloc $id de projects.md"
  return 1
}

# Retourne la liste CSV des overrides de mode pour un projet
# Format : "agent-id:mode,agent-id:mode,..."
# Retourne "" si le champ est absent (= utiliser les modes du frontmatter agent)
get_project_modes() {
  local raw
  raw=$(_get_project_field "$1" "Modes")
  echo "${raw:-}"
}

# Met à jour le champ "- Modes :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur CSV "agent-id:mode,..." (ou "" pour supprimer)
_set_project_modes() {
  local id="$1" new_modes="$2"
  if [ -z "$new_modes" ]; then
    # Supprimer le champ si valeur vide
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- Modes : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Modes : [^\n]+)}{\${1}- Modes : ${new_modes}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Modes : ${new_modes}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Agents :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Agents : [^\n]+\n)}{\${1}- Modes : ${new_modes}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Modes : ${new_modes}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Modes dans le bloc $id de projects.md"
  return 1
}

# Retourne le mode effectif d'un agent pour un projet donné
# Priorité : override projet > frontmatter agent > "primary" (défaut)
# @param $1 — agent_file (chemin vers le .md canonique)
# @param $2 — project_id (peut être vide)
get_effective_agent_mode() {
  local agent_file="$1" project_id="${2:-}"
  local agent_id
  agent_id=$(get_agent_id "$agent_file" 2>/dev/null || basename "$agent_file" .md)

  # Chercher un override dans le projet
  if [ -n "$project_id" ]; then
    local modes_csv
    modes_csv=$(get_project_modes "$project_id")
    if [ -n "$modes_csv" ]; then
      # Chercher "agent-id:mode" dans le CSV
      local override
      override=$(printf '%s\n' "$modes_csv" | tr ',' '\n' | grep "^${agent_id}:" | head -1 | cut -d: -f2)
      if [ -n "$override" ]; then
        echo "$override"
        return
      fi
    fi
  fi

  # Fallback : lire le frontmatter agent (inline pour éviter dépendance prompt-builder)
  local mode
  mode=$(grep '^mode:' "$agent_file" 2>/dev/null | head -1 | sed 's/^mode:[[:space:]]*//')
  echo "${mode:-primary}"
}

# Vérifie si un agent doit être déployé pour un project_id donné
# Retourne 0 si oui, 1 si non
# Si project_id vide ou agents=all → toujours déployer
should_deploy_agent() {
  local project_id="$1" agent_id="$2"
  [ -z "$project_id" ] && return 0
  local agents_csv
  agents_csv=$(get_project_agents "$project_id")
  [ -z "$agents_csv" ] || [ "$agents_csv" = "all" ] && return 0
  echo ",$agents_csv," | grep -qF ",$agent_id,"
}

# Detect OS
detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "macos" ;;
    *)       echo "unknown" ;;
  esac
}

# ─────────────────────────────────────────
# NATIVE AGENTS — désactivation OpenCode
# ─────────────────────────────────────────

# Lit opencode.disabled_native_agents dans hub.json (tableau JSON → CSV)
# Retourne "" si le champ est absent ou si le tableau est vide
# Fallback bash si jq est absent ; retourne 1 avec log_error si les deux échouent
get_hub_disabled_native_agents() {
  [ -f "$HUB_CONFIG" ] || return 0

  if command -v jq &>/dev/null; then
    local arr
    arr=$(jq -r '(.opencode.disabled_native_agents // []) | @csv' "$HUB_CONFIG" 2>/dev/null \
      | tr -d '"')
    echo "${arr:-}"
    return 0
  fi

  # Fallback bash sans jq : parse le tableau JSON avec grep/sed
  # Format attendu : "disabled_native_agents": ["val1","val2",...]
  local raw_array
  raw_array=$(grep -o '"disabled_native_agents"[[:space:]]*:[[:space:]]*\[[^]]*\]' "$HUB_CONFIG" 2>/dev/null \
    | grep -o '\[[^]]*\]' | tr -d '[]"' | tr ',' '\n' | tr -d ' ' | grep -v '^$' | paste -sd ',' -)

  if [ -n "$raw_array" ]; then
    echo "$raw_array"
    return 0
  fi

  # Vérifier si la clé est présente mais vide (tableau vide [])
  if grep -q '"disabled_native_agents"' "$HUB_CONFIG" 2>/dev/null; then
    echo ""
    return 0
  fi

  # Clé absente du hub.json — comportement normal, pas d'erreur
  echo ""
  return 0
}

# Lit "- Disable agents :" dans projects.md pour un projet donné
# Retourne "" si le champ est absent
get_project_disabled_native_agents() {
  local raw
  raw=$(_get_project_field "$1" "Disable agents")
  echo "${raw:-}"
}

# Écrit/met à jour "- Disable agents :" dans le bloc d'un projet dans projects.md
# Si valeur vide → supprime le champ
# Insérer après "- Agents :"
# @param $1 — PROJECT_ID
# @param $2 — valeur CSV (ou "" pour supprimer)
_set_project_disabled_native_agents() {
  local id="$1" new_val="$2"
  if [ -z "$new_val" ]; then
    # Supprimer le champ si valeur vide
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- Disable agents : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Disable agents : [^\n]+)}{\${1}- Disable agents : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Disable agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Agents :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Agents : [^\n]+\n)}{\${1}- Disable agents : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Disable agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback générique : insérer après le dernier champ du bloc
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- [^\n]+\n)}{\${1}- Disable agents : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Disable agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ 'Disable agents' dans le bloc $id de projects.md"
  return 1
}

# ─────────────────────────────────────────
# EXTERNAL AGENTS — agents projet externes
# ─────────────────────────────────────────

# Retourne la valeur brute du champ "External agents" d'un projet
# Format : "path:substitute:hub-id|path:complement|..."
# Retourne "" si le champ est absent
# @param $1 — PROJECT_ID
get_project_external_agents() {
  local raw
  raw=$(_get_project_field "$1" "External agents")
  echo "${raw:-}"
}

# Écrit/met à jour le champ "External agents" dans le bloc d'un projet dans projects.md
# Si valeur vide → supprime le champ
# @param $1 — PROJECT_ID
# @param $2 — valeur (ou "" pour supprimer)
_set_project_external_agents() {
  local id="$1" new_val="$2"
  if [ -z "$new_val" ]; then
    # Supprimer le champ si valeur vide
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- External agents : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- External agents : [^\n]+)}{\${1}- External agents : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -qF -- "- External agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Agents :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Agents : [^\n]+\n)}{\${1}- External agents : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -qF -- "- External agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Disable agents :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Disable agents : [^\n]+\n)}{\${1}- External agents : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -qF -- "- External agents : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ 'External agents' dans le bloc $id de projects.md"
  return 1
}

# Retourne les entrées de type "substitution" du champ External agents
# Format de sortie : une entrée par ligne "path:hub-id"
# @param $1 — PROJECT_ID
get_project_substitute_agents() {
  local raw
  raw=$(get_project_external_agents "$1")
  [ -z "$raw" ] && return 0
  # Splitter sur | et filtrer les :substitute:
  echo "$raw" | tr '|' '\n' | while IFS= read -r entry; do
    entry="${entry#"${entry%%[![:space:]]*}"}"  # trim left
    entry="${entry%"${entry##*[![:space:]]}"}"  # trim right
    [[ "$entry" == *":substitute:"* ]] || continue
    local path hub_id
    path="${entry%%:substitute:*}"
    hub_id="${entry##*:substitute:}"
    [ -n "$path" ] && [ -n "$hub_id" ] && echo "${path}:${hub_id}"
  done
}

# Retourne les entrées de type "complement" du champ External agents
# Format de sortie : un chemin par ligne
# @param $1 — PROJECT_ID
get_project_complement_agents() {
  local raw
  raw=$(get_project_external_agents "$1")
  [ -z "$raw" ] && return 0
  # Splitter sur | et filtrer les :complement
  echo "$raw" | tr '|' '\n' | while IFS= read -r entry; do
    entry="${entry#"${entry%%[![:space:]]*}"}"  # trim left
    entry="${entry%"${entry##*[![:space:]]}"}"  # trim right
    [[ "$entry" == *":complement" ]] || continue
    local path
    path="${entry%:complement}"
    [ -n "$path" ] && echo "$path"
  done
}

# ─────────────────────────────────────────
# MCP — sélection par projet
# ─────────────────────────────────────────

# Retourne la liste des MCP activés pour un projet
# Valeurs : CSV (ex: "figma-mcp,gitlab-mcp"), "all" ou "none"
# Retourne "none" si le champ est absent (opt-in obligatoire)
# @param $1 — PROJECT_ID
get_project_mcp() {
  local raw
  raw=$(_get_project_field "$1" "MCP")
  raw=$(echo "$raw" | tr -d '[:space:]')
  echo "${raw:-none}"
}

# Écrit/met à jour le champ "- MCP :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur : CSV, "all" ou "none"
_set_project_mcp() {
  local id="$1" new_val="$2"
  # Remplacer si le champ existe déjà dans le bloc du projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- MCP : [^\n]+)}{\${1}- MCP : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- MCP : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Agents :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Agents : [^\n]+\n)}{\${1}- MCP : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- MCP : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Disable agents :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Disable agents : [^\n]+\n)}{\${1}- MCP : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- MCP : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après le dernier "- " du bloc projet
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- [^\n]+\n)}{\${1}- MCP : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- MCP : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ MCP dans le bloc $id de projects.md"
  return 1
}

# Vérifie si un MCP server doit être déployé pour un projet donné
# Retourne 0 si oui, 1 si non
# @param $1 — PROJECT_ID
# @param $2 — server_name (ex: "figma-mcp")
should_deploy_mcp() {
  local project_id="$1" server_name="$2"
  local mcp_csv
  mcp_csv=$(get_project_mcp "$project_id")
  # "all" → déployer
  [ "$mcp_csv" = "all" ] && return 0
  # "none" ou vide → ne pas déployer
  [ -z "$mcp_csv" ] || [ "$mcp_csv" = "none" ] && return 1
  # CSV → vérifier la présence du serveur
  echo ",$mcp_csv," | grep -qF ",$server_name,"
}

# ─────────────────────────────────────────
# WORKTREE — configuration par projet
# ─────────────────────────────────────────

# ─────────────────────────────────────────
# SETTERS — champs de base du projet
# ─────────────────────────────────────────

# Met à jour le champ "- Langue :" dans le bloc d'un projet dans projects.md
# Si valeur vide → supprime le champ
# @param $1 — PROJECT_ID
# @param $2 — valeur (ex: "english", "spanish") ou "" pour supprimer
_set_project_language() {
  local id="$1" new_val="$2"
  if [ -z "$new_val" ]; then
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- Langue : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Langue : [^\n]+)}{\${1}- Langue : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Langue : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Labels :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Labels : [^\n]+\n)}{\${1}- Langue : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Langue : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Tracker :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Tracker : [^\n]+\n)}{\${1}- Langue : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Langue : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après le dernier champ du bloc
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- [^\n]+\n)}{\${1}- Langue : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Langue : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Langue dans le bloc $id de projects.md"
  return 1
}

# Met à jour le champ "- Tracker :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur : jira | gitlab | none
_set_project_tracker() {
  local id="$1" new_val="$2"
  case "$new_val" in
    jira|gitlab|none) ;;
    *) log_error "Tracker invalide : $new_val (jira | gitlab | none)"; return 1 ;;
  esac
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Tracker : [^\n]+)}{\${1}- Tracker : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Tracker : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Stack :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Stack : [^\n]+\n)}{\${1}- Tracker : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Tracker : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Tracker dans le bloc $id de projects.md"
  return 1
}

# Met à jour le champ "- Stack :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur libre (ex: "Vue 3 + Laravel")
_set_project_stack() {
  local id="$1" new_val="$2"
  [ -z "$new_val" ] && { log_error "Stack ne peut pas être vide"; return 1; }
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Stack : [^\n]+)}{\${1}- Stack : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Stack : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Nom :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Nom : [^\n]+\n)}{\${1}- Stack : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Stack : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Stack dans le bloc $id de projects.md"
  return 1
}

# Met à jour le champ "- Labels :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur CSV (ex: "feature,fix,front,back") ou "" pour vider
_set_project_labels() {
  local id="$1" new_val="$2"
  if [ -z "$new_val" ]; then
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)(- Labels : [^\n]+)}{${1}- Labels : }ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Labels : [^\n]+)}{\${1}- Labels : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Labels : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Tracker :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Tracker : [^\n]+\n)}{\${1}- Labels : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Labels : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après "- Stack :"
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Stack : [^\n]+\n)}{\${1}- Labels : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Labels : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Labels dans le bloc $id de projects.md"
  return 1
}

# ─────────────────────────────────────────
# WORKTREE — configuration par projet
# ─────────────────────────────────────────

# Met à jour le champ "- Worktree :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur : enabled | disabled
_set_project_worktree_enabled() {
  local id="$1" new_val="$2"
  case "$new_val" in
    enabled|disabled) ;;
    *) log_error "Worktree invalide : $new_val (enabled | disabled)"; return 1 ;;
  esac
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Worktree : [^\n]+)}{\${1}- Worktree : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Tracker :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Tracker : [^\n]+\n)}{\${1}- Worktree : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Fallback : insérer après le dernier champ du bloc
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n(?:(?!^##)[^\n]*\n)*?- [^\n]+\n)}{\${1}- Worktree : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ Worktree dans le bloc $id de projects.md"
  return 1
}

# Met à jour le champ "- Worktree auto cleanup :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — valeur : true | false
_set_project_worktree_auto_cleanup() {
  local id="$1" new_val="$2"
  case "$new_val" in
    true|false) ;;
    *) log_error "Worktree auto cleanup invalide : $new_val (true | false)"; return 1 ;;
  esac
  if [ "$new_val" = "false" ]; then
    # Supprimer le champ si false (valeur par défaut)
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- Worktree auto cleanup : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Worktree auto cleanup : [^\n]+)}{\${1}- Worktree auto cleanup : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree auto cleanup : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Worktree :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Worktree : [^\n]+\n)}{\${1}- Worktree auto cleanup : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree auto cleanup : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ 'Worktree auto cleanup' dans le bloc $id de projects.md"
  return 1
}

# Met à jour le champ "- Worktree base branch :" dans le bloc d'un projet dans projects.md
# @param $1 — PROJECT_ID
# @param $2 — nom de branche (ex: "main", "master", "develop")
_set_project_worktree_base_branch() {
  local id="$1" new_val="$2"
  [ -z "$new_val" ] && { log_error "Worktree base branch ne peut pas être vide"; return 1; }
  # Validation légère : pas d'espaces
  if echo "$new_val" | grep -q '[[:space:]]'; then
    log_error "Worktree base branch invalide : pas d'espaces autorisés"
    return 1
  fi
  if [ "$new_val" = "main" ]; then
    # Supprimer le champ si "main" (valeur par défaut)
    perl -i -0777pe "
      s{(^## \Q${id}\E\n.*?)- Worktree base branch : [^\n]+\n}{\$1}ms
    " "$PROJECTS_FILE" 2>/dev/null
    return 0
  fi
  # Remplacer si existant
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?)(- Worktree base branch : [^\n]+)}{\${1}- Worktree base branch : ${new_val}}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree base branch : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Worktree auto cleanup :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Worktree auto cleanup : [^\n]+\n)}{\${1}- Worktree base branch : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree base branch : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  # Insérer après "- Worktree :" si présent
  if perl -i -0777pe "
    s{(^## \Q${id}\E\n.*?- Worktree : [^\n]+\n)}{\${1}- Worktree base branch : ${new_val}\n}ms
  " "$PROJECTS_FILE" 2>/dev/null && grep -q -- "- Worktree base branch : ${new_val}" "$PROJECTS_FILE"; then
    return 0
  fi
  log_error "Impossible d'insérer le champ 'Worktree base branch' dans le bloc $id de projects.md"
  return 1
}

# Retourne le statut d'activation des worktrees pour un projet
# Valeurs : "enabled" | "disabled" (défaut si champ absent)
# @param $1 — PROJECT_ID
get_project_worktree_enabled() {
  local raw
  raw=$(_get_project_field "$1" "Worktree")
  raw=$(echo "$raw" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  echo "${raw:-disabled}"
}

# Retourne la configuration d'auto-cleanup des worktrees pour un projet
# Valeurs : "true" | "false" (défaut si champ absent)
# @param $1 — PROJECT_ID
get_project_worktree_auto_cleanup() {
  local raw
  raw=$(_get_project_field "$1" "Worktree auto cleanup")
  raw=$(echo "$raw" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')
  echo "${raw:-false}"
}

# Retourne la branche de base pour le cleanup des worktrees
# Valeurs : "main" | "master" | toute branche (défaut : "main")
# @param $1 — PROJECT_ID
get_project_worktree_base_branch() {
  local raw
  raw=$(_get_project_field "$1" "Worktree base branch")
  raw=$(echo "$raw" | tr -d '[:space:]')
  echo "${raw:-main}"
}

# ─────────────────────────────────────────────────────────────────────────────
# Verrouillage des écritures dans projects.md
#
# Protection contre les accès concurrents :
# - Les cmd-*.sh qui font des écritures directes (perl -i, cat >>) acquièrent
#   le verrou OC_LOCK_PROJECTS explicitement avant chaque opération.
# - Les fonctions _set_project_* héritent du verrou via _do_locked_projects_write
#   si la lib filelock est disponible.
#
# Note : l'approche eval/declare -f est évitée car elle provoque des délais
# au sourçage dans les tests. La protection est garantie par les cmd-*.sh.
# ─────────────────────────────────────────────────────────────────────────────

# Wrapper de verrouillage centralisé pour les écritures dans projects.md.
# Utilisé par les cmd-*.sh pour protéger les appels directs à perl -i / cat >>.
# Usage : _do_locked_projects_write <fonction> [args...]
_do_locked_projects_write() {
  if command -v _acquire_lock &>/dev/null; then
    _acquire_lock "${OC_LOCK_PROJECTS:-projects}" 10 || {
      log_error "filelock: timeout — impossible d'obtenir le verrou sur projects.md"
      return 1
    }
    local _ret=0
    "$@" || _ret=$?
    _release_lock "${OC_LOCK_PROJECTS:-projects}"
    return $_ret
  else
    "$@"
  fi
}

