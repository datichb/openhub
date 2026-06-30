#!/bin/bash
# Synchronise les agents déployés sur tous les projets enregistrés localement.
# Usage : oc sync [--dry-run]
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
source "$LIB_DIR/adapter-manager.sh"
source "$LIB_DIR/prompt-builder.sh"
source "$LIB_DIR/progress-bar.sh"

DRY_RUN=false
for arg in "$@"; do
  [ "$arg" = "--dry-run" ] && DRY_RUN=true
done

if [ "$DRY_RUN" = true ]; then
  log_title "$(t sync.title_dryrun)"
else
  log_title "$(t sync.title)"
fi

# S'assurer que projects.md existe
ensure_projects_file

# Extraire tous les PROJECT_IDs enregistrés
project_ids=()
while IFS= read -r line; do project_ids+=("$line"); done \
  < <(grep "^## " "$PROJECTS_FILE" 2>/dev/null | sed 's/^## //' || true)

if [ ${#project_ids[@]} -eq 0 ]; then
  log_warn "$(t sync.no_projects)"
  exit 0
fi

deployed_count=0
skipped_count=0
failed_count=0
stale_count=0   # utilisé uniquement en dry-run
ok_count=0      # utilisé uniquement en dry-run

# Fichier temp pour stocker les erreurs de déploiement (bash 3.2 : pas de declare -A)
_sync_errors_file=$(mktemp "${TMPDIR:-/tmp}/oc-sync-errors-XXXXXX")
trap 'rm -f "$_sync_errors_file"' EXIT

echo ""

# Boucle sur les projets avec barre de progression
total_projects=${#project_ids[@]}
current_project=0

for project_id in "${project_ids[@]}"; do
  current_project=$((current_project + 1))
  
  # Afficher la progression
  _progress_bar $current_project $total_projects "$project_id"

  # Résoudre le chemin local
  local_path=$(get_project_path "$project_id" 2>/dev/null || true)

  if [ -z "$local_path" ]; then
    skipped_count=$((skipped_count + 1))
    continue
  fi

  # Expand ~ si nécessaire
  local_path="${local_path/#\~/$HOME}"

  if [ ! -d "$local_path" ]; then
    skipped_count=$((skipped_count + 1))
    continue
  fi

  if [ "$DRY_RUN" = true ]; then
    # ── Mode dry-run : vérifier la fraîcheur sans déployer ────────────────────
    project_stale=0
    project_ok=0

    gen_dir="$local_path/.opencode/agents"

    # Collecter les fichiers agents pour progression
    agent_files=()
    while IFS= read -r agent_file; do
      [ -f "$agent_file" ] && agent_files+=("$agent_file")
    done < <(find "$CANONICAL_AGENTS_DIR" -name "*.md" | sort)
    
    total_agents=${#agent_files[@]}
    current_agent=0

    # Boucle sur les agents avec progression
    for agent_file in "${agent_files[@]}"; do
      current_agent=$((current_agent + 1))
      agent_id=$(get_agent_id "$agent_file")
      
      # Afficher progression agents (sous-barre)
      _progress_bar $current_agent $total_agents "$agent_id"

      gen_file="$gen_dir/${agent_id}.md"

      if [ ! -f "$gen_file" ]; then
        project_stale=$((project_stale + 1))
        continue
      fi

      gen_mtime=$(stat -c %Y "$gen_file" 2>/dev/null || stat -f %m "$gen_file" 2>/dev/null)
      max_src_mtime=0

      agent_mtime=$(stat -c %Y "$agent_file" 2>/dev/null || stat -f %m "$agent_file" 2>/dev/null)
      [ "$agent_mtime" -gt "$max_src_mtime" ] && max_src_mtime=$agent_mtime

      while IFS= read -r skill; do
        [ -z "$skill" ] && continue
        skill_file="$SKILLS_DIR/${skill}.md"
        [ -f "$skill_file" ] || continue
        skill_mtime=$(stat -c %Y "$skill_file" 2>/dev/null || stat -f %m "$skill_file" 2>/dev/null)
        if [ "$skill_mtime" -gt "$max_src_mtime" ]; then
          max_src_mtime=$skill_mtime
        fi
      done < <(extract_frontmatter_list "$agent_file" "skills")

      if [ "$max_src_mtime" -gt "$gen_mtime" ]; then
        project_stale=$((project_stale + 1))
      else
        project_ok=$((project_ok + 1))
      fi
    done
    
    _progress_done

    stale_count=$((stale_count + project_stale))
    ok_count=$((ok_count + project_ok))

  else
    # ── Mode déploiement ──────────────────────────────────────────────────────
    deploy_ok=true
    load_adapter
    if ! adapter_validate 2>/dev/null; then
      # opencode non disponible sur ce projet → ignoré (pas une erreur de déploiement)
      skipped_count=$((skipped_count + 1))
      continue
    fi
    _err_tmp=$(mktemp "${TMPDIR:-/tmp}/oc-sync-err-XXXXXX")
    adapter_deploy "$local_path" "$project_id" >/dev/null 2>"$_err_tmp" \
      && deploy_ok=true || deploy_ok=false
    if [ "$deploy_ok" = false ]; then
      # Stocker l'erreur (première ligne significative) avec l'ID projet
      _first_err=$(grep -v '^$' "$_err_tmp" | head -1 || true)
      printf '%s\t%s\n' "$project_id" "${_first_err:-deploy error}" >> "$_sync_errors_file"
      failed_count=$((failed_count + 1))
    else
      deployed_count=$((deployed_count + 1))
    fi
    rm -f "$_err_tmp"
  fi
done

_progress_done

# Récapitulatif structuré
echo ""
if [ "$DRY_RUN" = true ]; then
  summary_lines=()
  summary_lines+=("$total_projects projets vérifiés")
  [ "$ok_count" -gt 0 ] && summary_lines+=("  - $ok_count agents à jour")
  [ "$stale_count" -gt 0 ] && summary_lines+=("  - $stale_count agents obsolètes")
  [ "$skipped_count" -gt 0 ] && summary_lines+=("  - $skipped_count projets ignorés")
  
  _progress_summary "Vérification terminée" "${summary_lines[@]}"
else
  summary_lines=()
  summary_lines+=("$total_projects projets traités")
  [ "$deployed_count" -gt 0 ] && summary_lines+=("  - $deployed_count $(t sync.result_deployed)")
  [ "$skipped_count"  -gt 0 ] && summary_lines+=("  - $skipped_count $(t sync.result_skipped)")
  [ "$failed_count"   -gt 0 ] && summary_lines+=("  - $failed_count $(t sync.failed_count)")
  
  _progress_summary "Synchronisation terminée" "${summary_lines[@]}"

  # Détail des projets en échec
  if [ "$failed_count" -gt 0 ] && [ -s "$_sync_errors_file" ]; then
    echo ""
    log_warn "$(t sync.failed_projects)"
    while IFS=$'\t' read -r _fail_id _fail_msg; do
      printf "  ${RED}✗${RESET} ${BOLD}%s${RESET} — %s\n" "$_fail_id" "$_fail_msg"
    done < "$_sync_errors_file"
  fi
fi

echo ""

# ── Rapport final ─────────────────────────────────────────────────────────────
if [ "$DRY_RUN" = true ]; then
  if [ "$stale_count" -gt 0 ]; then
    log_info "$(t sync.deploy_hint)"
    exit 1
  fi
fi

# Echec si des projets n'ont pas pu être déployés
if [ "$failed_count" -gt 0 ]; then
  exit 1
fi
