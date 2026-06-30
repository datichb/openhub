#!/bin/bash
# session-title.sh — Génération du titre de session opencode
# Extrait les mots clés significatifs d'un prompt en filtrant les stop-words FR+EN.
# Usage : source session-title.sh  puis  _build_session_title <args>

# ─────────────────────────────────────────────────────────────────────────────
# Stop-words FR + EN (liste embarquée, aucune dépendance externe)
# ─────────────────────────────────────────────────────────────────────────────
_STOPWORDS_FR="je tu il elle nous vous ils elles me te se le la les un une des du de d en au aux et ou mais donc or ni car ce que qui ne pas plus très bien aussi comme avec sur dans par pour sans être avoir faire aller vouloir pouvoir voir savoir prendre mettre alors ensuite puis après avant"
_STOPWORDS_EN="i you he she we they me my your his her our their it its a an the is are was were be been being have has had do does did will would could should may might shall can must that this these those of in on at to for with from by about into through during after before above below between under along while although because since until when where who which how all any both each few more most other some such no nor not only own same so than then there when where why just"

# ─────────────────────────────────────────────────────────────────────────────
# _build_session_title <mode_dev> <mode_onboard> <mode_parallel>
#                      <prompt> <project_id> [worktree_branch]
#                      [dev_ticket_id] [dev_ticket_title]
#
# Affiche le titre de session calculé sur stdout.
# Priorité : modes spéciaux > extraction mots clés > fallback PROJECT_ID — date
# ─────────────────────────────────────────────────────────────────────────────
_build_session_title() {
  local mode_dev="$1" mode_onboard="$2" mode_parallel="$3"
  local prompt="$4" project_id="$5" worktree_branch="${6:-}"
  local dev_ticket_id="${7:-}" dev_ticket_title="${8:-}"

  # Modes spéciaux sans prompt
  if [ "$mode_onboard" = true ]; then
    echo "$(t start.session_title_onboard): ${project_id}"
    return
  fi
  if [ "$mode_parallel" = true ]; then
    echo "$(t start.session_title_parallel): ${worktree_branch:-${project_id}}"
    return
  fi

  # Extraction des mots significatifs du prompt
  local keywords=""
  if [ -n "$prompt" ]; then
    # 1. Détecter les références tickets (#123, CP-3, PROJ-42, etc.)
    local ticket_refs
    ticket_refs=$(echo "$prompt" | grep -oE '(#[0-9]+|[A-Z][A-Z0-9]+-[0-9]+)' | tr '\n' ' ' | xargs)

    # 2. Mettre le prompt en minuscules, supprimer la ponctuation
    local clean
    clean=$(echo "$prompt" | tr '[:upper:]' '[:lower:]' | tr -d '[:punct:]' | tr -s ' ')

    # 3. Filtrer stop-words FR+EN mot par mot
    local word filtered_words=()
    for word in $clean; do
      local is_stop=false
      local sw
      for sw in $_STOPWORDS_FR $_STOPWORDS_EN; do
        if [ "$word" = "$sw" ]; then
          is_stop=true
          break
        fi
      done
      $is_stop || filtered_words+=("$word")
    done

    # 4. Assembler : refs tickets + mots filtrés, tronquer à 50 chars
    local body="${filtered_words[*]:-}"
    if [ -n "$ticket_refs" ] && [ -n "$body" ]; then
      keywords="${ticket_refs} — ${body}"
    elif [ -n "$ticket_refs" ]; then
      keywords="$ticket_refs"
    else
      keywords="$body"
    fi

    # Tronquer à 50 caractères
    if [ ${#keywords} -gt 50 ]; then
      keywords="${keywords:0:48}…"
    fi
  fi

  # Préfixe mode dev
  if [ "$mode_dev" = true ]; then
    local prefix
    prefix="$(t start.session_title_dev)"
    if [ -n "$dev_ticket_id" ] && [ -n "$dev_ticket_title" ]; then
      # Nettoyer le titre du ticket des stop-words
      local tclean tw filtered_tw=()
      tclean=$(echo "$dev_ticket_title" | tr '[:upper:]' '[:lower:]' | tr -d '[:punct:]' | tr -s ' ')
      for tw in $tclean; do
        local ts=false sw2
        for sw2 in $_STOPWORDS_FR $_STOPWORDS_EN; do
          [ "$tw" = "$sw2" ] && { ts=true; break; }
        done
        $ts || filtered_tw+=("$tw")
      done
      local ticket_kw="${filtered_tw[*]:-$dev_ticket_title}"
      [ ${#ticket_kw} -gt 40 ] && ticket_kw="${ticket_kw:0:38}…"
      echo "${prefix}: ${dev_ticket_id} — ${ticket_kw}"
    else
      [ -n "$keywords" ] && echo "${prefix}: ${keywords}" || echo "${prefix}: ${project_id}"
    fi
    return
  fi

  # Cas général
  if [ -n "$keywords" ]; then
    echo "$keywords"
  else
    echo "${project_id} — $(date +%Y-%m-%d)"
  fi
}
