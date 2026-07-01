#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# provider-warnings.sh — Validation transparente des providers LLM
#
# Approche A : Pre-flight check (connectivité réseau, curl 3s)
# Approche C : Validation post-deploy (cohérence model ↔ bloc provider)
#
# Sourçage : cmd-start.sh (bloc contextuel) + adapter_start() (tous les cmds)
# Ne bloque jamais le lancement — affiche des warnings actionnables.
# ─────────────────────────────────────────────────────────────────────────────
[ -n "${_PROVIDER_WARNINGS_LOADED:-}" ] && return 0
_PROVIDER_WARNINGS_LOADED=1

# ── Constantes ────────────────────────────────────────────────────────────────
_PW_CURL_TIMEOUT=3
_PW_AUTH_JSON="${HOME}/.local/share/opencode/auth.json"

# Code de statut — utilisé par _validate_provider_connectivity()
readonly _PW_OK=0
readonly _PW_UNREACHABLE=1
readonly _PW_NO_CREDS=2
readonly _PW_BAD_URL=3
readonly _PW_NO_KEY=4
readonly _PW_NO_PROVIDER=5

# ─────────────────────────────────────────────────────────────────────────────
# _pw_line — Affiche une ligne dans le format du bloc contextuel oc start
#   $1 = label (aligné sur 10 chars)
#   $2 = contenu
# ─────────────────────────────────────────────────────────────────────────────
_pw_line() {
  local label="$1" content="$2"
  printf "${DIM}│${RESET}  %-10s %s\n" "$label" "$content"
}

# ─────────────────────────────────────────────────────────────────────────────
# _pw_hint — Affiche une ligne hint indentée (sans label)
#   $1 = message hint
# ─────────────────────────────────────────────────────────────────────────────
_pw_hint() {
  printf "${DIM}│${RESET}  %-10s ${DIM}→ %s${RESET}\n" "" "$1"
}

# ─────────────────────────────────────────────────────────────────────────────
# _validate_provider_config — Approche C
# Vérifie la cohérence entre le modèle et le bloc provider dans opencode.json
#
# $1 = config_file (chemin vers opencode.json)
# $2 = model       (ex: amazon-bedrock/anthropic.claude-sonnet-4-6)
# $3 = effective_provider (ex: bedrock)
#
# Retourne :
#   0 = cohérent
#   1 = modèle orphelin (prefix sans bloc provider)
# Exporte :
#   _PW_CONFIG_WARNING = message d'avertissement si problème détecté
# ─────────────────────────────────────────────────────────────────────────────
_validate_provider_config() {
  local config_file="$1" model="$2" effective_provider="$3"
  _PW_CONFIG_WARNING=""

  # Pas de fichier = pas de validation possible
  [ -f "$config_file" ] || return 0
  command -v jq &>/dev/null || return 0

  # Extraire le préfixe du modèle (partie avant le premier /)
  local model_prefix="${model%%/*}"

  # Si pas de préfixe (model == model_prefix), pas d'orphelin possible
  [ "$model_prefix" = "$model" ] && return 0

  # Vérifier que le bloc provider correspondant au préfixe existe
  local has_block
  has_block=$(jq -r --arg p "$model_prefix" \
    'if .provider[$p] != null then "true" else "false" end' \
    "$config_file" 2>/dev/null || echo "false")

  if [ "$has_block" = "false" ]; then
    _PW_CONFIG_WARNING="$(t provider.status_model_orphan)"
    return 1
  fi

  return 0
}

# ─────────────────────────────────────────────────────────────────────────────
# _validate_provider_connectivity — Approche A
# Test de connectivité léger (curl 3s) vers l'endpoint du provider
#
# $1 = provider    (ex: mammouth, anthropic, bedrock, github-copilot)
# $2 = api_key     (clé API ou bearer token)
# $3 = base_url    (URL de base, peut être vide)
# $4 = aws_region  (optionnel, pour bedrock)
#
# Retourne un code _PW_* (0=OK, 1=injoignable, 2=no_creds, 3=bad_url, 4=no_key)
# ─────────────────────────────────────────────────────────────────────────────
_validate_provider_connectivity() {
  local provider="$1" api_key="${2:-}" base_url="${3:-}" aws_region="${4:-}"

  # Skip si curl absent ou pas de TTY (CI/CD — évite un délai de 3s inutile)
  # La variable _PW_FORCE_TTY permet de forcer le check dans les tests BATS.
  command -v curl &>/dev/null || return $_PW_OK
  if [ "${_PW_FORCE_TTY:-0}" != "1" ] && ! [ -t 1 ]; then return $_PW_OK; fi

  # Aucun provider configuré — cas explicite, pas de test réseau possible
  [ -z "$provider" ] && return $_PW_NO_PROVIDER

  case "$provider" in

    mammouth|github-models|ollama|litellm)
      # Résoudre l'URL effective
      local url="$base_url"
      [ -z "$url" ] && url=$(get_provider_info "$provider" "default_base_url" 2>/dev/null || true)
      [ -z "$url" ] && return $_PW_NO_CREDS

      # Détecter le suffixe /chat/completions (erreur courante — le AI SDK l'ajoute lui-même)
      if [[ "$url" == */chat/completions ]]; then
        return $_PW_BAD_URL
      fi

      # Vérifier la clé
      [ -z "$api_key" ] && return $_PW_NO_KEY

      # Test GET /models
      curl -sf \
        --max-time "$_PW_CURL_TIMEOUT" \
        -H "Authorization: Bearer ${api_key}" \
        "${url%/}/models" >/dev/null 2>&1 \
        && return $_PW_OK \
        || return $_PW_UNREACHABLE
      ;;

    anthropic)
      [ -z "$api_key" ] && return $_PW_NO_KEY

      local http_code
      http_code=$(curl -sf \
        --max-time "$_PW_CURL_TIMEOUT" \
        -o /dev/null -w "%{http_code}" \
        -H "x-api-key: ${api_key}" \
        -H "anthropic-version: 2023-06-01" \
        -H "content-type: application/json" \
        "https://api.anthropic.com/v1/messages" 2>/dev/null || echo "000")

      # 400/401/413/429 = API joignable (erreurs métier, pas réseau)
      # 000 = injoignable
      case "$http_code" in
        200|400|401|413|429) return $_PW_OK ;;
        *) return $_PW_UNREACHABLE ;;
      esac
      ;;

    bedrock)
      # Bearer token injecté → OK (sera validé au runtime par OpenCode)
      [ -n "${AWS_BEARER_TOKEN_BEDROCK:-}" ] && return $_PW_OK

      # Vérifier la présence de credentials AWS
      if command -v aws &>/dev/null; then
        local _region="${aws_region:-eu-west-3}"
        aws sts get-caller-identity --region "$_region" >/dev/null 2>&1 \
          && return $_PW_OK
      fi

      # Vérifier ~/.aws/credentials ou env vars
      if [ -n "${AWS_ACCESS_KEY_ID:-}" ] || \
         [ -n "${AWS_PROFILE:-}" ] || \
         [ -f "${HOME}/.aws/credentials" ]; then
        return $_PW_OK
      fi

      return $_PW_NO_CREDS
      ;;

    github-copilot)
      # Authentification OAuth — vérifier que le token existe dans auth.json d'OpenCode
      if [ -f "$_PW_AUTH_JSON" ] && command -v jq &>/dev/null; then
        jq -e '."github-copilot" // empty' "$_PW_AUTH_JSON" >/dev/null 2>&1 \
          && return $_PW_OK
      fi
      return $_PW_NO_CREDS
      ;;

    openrouter)
      [ -z "$api_key" ] && return $_PW_NO_KEY

      curl -sf \
        --max-time "$_PW_CURL_TIMEOUT" \
        -H "Authorization: Bearer ${api_key}" \
        "https://openrouter.ai/api/v1/models" >/dev/null 2>&1 \
        && return $_PW_OK \
        || return $_PW_UNREACHABLE
      ;;

    *)
      # Provider non reconnu — skip gracieux
      return $_PW_OK
      ;;
  esac
}

# ─────────────────────────────────────────────────────────────────────────────
# _display_provider_status — Point d'entrée principal
# Résout le provider effectif, lance les validations, affiche le statut.
# Conçu pour s'insérer dans le bloc contextuel de cmd-start.sh.
#
# $1 = project_id       (optionnel)
# $2 = provider_override (optionnel — ex: depuis --provider)
# $3 = config_file      (optionnel — chemin vers opencode.json du projet)
# ─────────────────────────────────────────────────────────────────────────────
_display_provider_status() {
  local project_id="${1:-}" provider_override="${2:-}" config_file="${3:-}"

  # Résoudre le provider et le modèle effectifs
  local effective_provider model api_key base_url aws_region
  effective_provider=$(get_effective_provider "$project_id" "$provider_override")
  model=$(_get_opencode_model "$project_id" "$provider_override" 2>/dev/null || echo "")

  # Récupérer les credentials du projet ou du hub
  api_key=""
  base_url=""
  aws_region=""
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id" 2>/dev/null; then
    api_key=$(get_project_api_key "$project_id" 2>/dev/null || true)
    base_url=$(get_project_api_base_url "$project_id" 2>/dev/null || true)
    aws_region=$(get_project_api_region "$project_id" 2>/dev/null || true)
  else
    api_key=$(get_hub_default_api_key 2>/dev/null || true)
    base_url=$(get_hub_default_base_url 2>/dev/null || true)
    aws_region=$(jq -r '.default_provider.region // empty' "$HUB_CONFIG" 2>/dev/null || true)
  fi

  # ── Approche C : Validation post-deploy ─────────────────────────────────────
  local config_warning=""
  if [ -n "$config_file" ] && [ -f "$config_file" ] && [ -n "$model" ]; then
    _validate_provider_config "$config_file" "$model" "$effective_provider" || true
    config_warning="${_PW_CONFIG_WARNING:-}"
  fi

  # ── Approche A : Pre-flight connectivité ────────────────────────────────────
  local connectivity_code=$_PW_OK
  _validate_provider_connectivity \
    "$effective_provider" "$api_key" "$base_url" "$aws_region"
  connectivity_code=$?

  # ── Affichage du statut ──────────────────────────────────────────────────────
  local label="Provider"

  if [ "$connectivity_code" -eq "$_PW_NO_PROVIDER" ]; then
    # Aucun provider configuré ni au niveau projet ni au niveau hub
    _pw_line "$label" "⚠️  $(t provider.status_not_configured)"
    _pw_hint "$(t provider.hint_connect)"
    _pw_hint "$(t provider.hint_hub_config)"

  elif [ -n "$config_warning" ]; then
    # Incohérence model/provider (Approche C)
    _pw_line "$label" "⚠️  ${effective_provider} — ${config_warning}"
    _pw_hint "$(t provider.hint_connect)"
    _pw_hint "$(t provider.warn_deploy_hint)"

  elif [ "$connectivity_code" -eq "$_PW_BAD_URL" ]; then
    # URL malformée avec /chat/completions
    _pw_line "$label" "⚠️  ${effective_provider} — $(t provider.status_bad_url)"
    _pw_hint "$(t provider.hint_check_url) : ${base_url%/chat/completions}"
    _pw_hint "$(t provider.hint_hub_config)"

  elif [ "$connectivity_code" -eq "$_PW_NO_KEY" ]; then
    # Clé API absente
    _pw_line "$label" "⚠️  ${effective_provider} — $(t provider.status_no_key)"
    _pw_hint "$(t provider.hint_connect)"
    _pw_hint "$(t provider.hint_hub_config)"

  elif [ "$connectivity_code" -eq "$_PW_NO_CREDS" ]; then
    # Credentials non trouvées (Bedrock, GitHub Copilot)
    _pw_line "$label" "⚠️  ${effective_provider} — $(t provider.status_no_creds)"
    if [ "$effective_provider" = "bedrock" ]; then
      _pw_hint "$(t provider.hint_aws_creds)"
    fi
    _pw_hint "$(t provider.hint_connect)"

  elif [ "$connectivity_code" -eq "$_PW_UNREACHABLE" ]; then
    # Endpoint injoignable
    _pw_line "$label" "⚠️  ${effective_provider} — $(t provider.status_unreachable)"
    _pw_hint "$(t provider.hint_check_network)"
    _pw_hint "$(t provider.hint_connect)"

  else
    # Tout OK
    _pw_line "$label" "✅ ${effective_provider} — $(t provider.status_ok)"
  fi
}

# ─────────────────────────────────────────────────────────────────────────────
# _warn_provider_if_needed — Version légère pour adapter_start()
# Affiche un avertissement minimal si le provider semble invalide.
# Pas de bloc contextuel — juste un log_warn simple.
# Appelé dans les commandes qui ne passent pas par le bloc contextuel
# (cmd-quick, cmd-review, cmd-audit, cmd-conventions, cmd-debug).
#
# $1 = project_id       (optionnel)
# $2 = provider_override (optionnel)
# ─────────────────────────────────────────────────────────────────────────────
_warn_provider_if_needed() {
  local project_id="${1:-}" provider_override="${2:-}"

  # Skip si pas de TTY (CI/CD)
  if [ "${_PW_FORCE_TTY:-0}" != "1" ] && ! [ -t 1 ]; then return 0; fi

  local effective_provider api_key base_url aws_region
  effective_provider=$(get_effective_provider "$project_id" "$provider_override")

  api_key=""
  base_url=""
  aws_region=""
  if [ -n "$project_id" ] && api_keys_entry_exists "$project_id" 2>/dev/null; then
    api_key=$(get_project_api_key "$project_id" 2>/dev/null || true)
    base_url=$(get_project_api_base_url "$project_id" 2>/dev/null || true)
    aws_region=$(get_project_api_region "$project_id" 2>/dev/null || true)
  else
    api_key=$(get_hub_default_api_key 2>/dev/null || true)
    base_url=$(get_hub_default_base_url 2>/dev/null || true)
    aws_region=$(jq -r '.default_provider.region // empty' "$HUB_CONFIG" 2>/dev/null || true)
  fi

  local connectivity_code=$_PW_OK
  _validate_provider_connectivity \
    "$effective_provider" "$api_key" "$base_url" "$aws_region"
  connectivity_code=$?

  case "$connectivity_code" in
    "$_PW_NO_PROVIDER")
      log_warn "$(t provider.status_not_configured)"
      log_warn "$(t provider.hint_connect)"
      log_warn "$(t provider.hint_hub_config)"
      ;;
    "$_PW_BAD_URL")
      log_warn "Provider ${effective_provider} — $(t provider.status_bad_url)"
      log_warn "$(t provider.hint_check_url) : ${base_url%/chat/completions}"
      ;;
    "$_PW_NO_KEY")
      log_warn "Provider ${effective_provider} — $(t provider.status_no_key)"
      log_warn "$(t provider.hint_connect)"
      ;;
    "$_PW_NO_CREDS")
      log_warn "Provider ${effective_provider} — $(t provider.status_no_creds)"
      [ "$effective_provider" = "bedrock" ] && log_warn "$(t provider.hint_aws_creds)"
      log_warn "$(t provider.hint_connect)"
      ;;
    "$_PW_UNREACHABLE")
      log_warn "Provider ${effective_provider} — $(t provider.status_unreachable)"
      log_warn "$(t provider.hint_check_network)"
      log_warn "$(t provider.hint_connect)"
      ;;
  esac

  return 0
}
