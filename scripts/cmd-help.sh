#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Layout constants ──────────────────────────────────────────────────────────
# CMD_W: width of the command column. Sized to fit the longest command string.
CMD_W=40
SUB_INDENT=2   # extra spaces for variant lines

# ── Helper functions ──────────────────────────────────────────────────────────

# Print a section header: bold numbered title + separator.
_section() {
  local title
  title="$(t "$1")"
  echo ""
  echo -e "${BOLD}${title}${RESET}"
  printf '%.0s─' $(seq 1 52); echo
}

# Print one command row: cyan command column + description.
_cmd() {
  local cmd desc
  cmd="$(t "$1.cmd")"
  desc="$(t "$1.desc")"
  if [ "${#cmd}" -le "$CMD_W" ]; then
    printf "  ${CYAN}%-${CMD_W}s${RESET}  %s\n" "$cmd" "$desc"
  else
    printf "  ${CYAN}%s${RESET}\n" "$cmd"
    printf "  %*s  %s\n" "$CMD_W" "" "$desc"
  fi
}

# Print a variant/sub-option row indented under the parent command.
# $1 = flag/variant text (e.g. "--dev")  $2 = description
_sub() {
  local flag desc
  flag="$(t "$1.cmd")"
  desc="$(t "$1.desc")"
  # Strip the base command prefix to keep only the flag part for readability
  # e.g. "start [PROJECT_ID] --dev" → "--dev"
  flag="${flag##* --}"; flag="--${flag}"
  printf "  %*s${DIM}%-$((CMD_W - SUB_INDENT))s${RESET}  %s\n" \
    "$SUB_INDENT" "" "$flag" "$desc"
}

# Print a plain indented note (used for examples).
_note() {
  echo "  $1"
}

# ── Section rendering functions ───────────────────────────────────────────────

_section_1() {
  _section help.section.setup
  _cmd help.install
  _cmd help.uninstall
  _cmd help.init
  _cmd help.version
}

_section_2() {
  _section help.section.projects
  _cmd    help.status
  _sub    help.status_short
  _cmd    help.remove
  _sub    help.remove_clean
  _cmd    help.project_rename
  _cmd    help.project_move
  _cmd    help.project_configure
}

_section_3() {
  _section help.section.launch
  _cmd  help.start
  _sub  help.start_dev
  _sub  help.start_dev_label
  _sub  help.start_dev_assignee
  _sub  help.start_onboard
  _sub  help.start_onboard_refresh
  _sub  help.start_parallel
  _sub  help.start_worktree
  _sub  help.start_agent
  _sub  help.start_provider
  _cmd  help.quick
}

_section_4() {
  _section help.section.analysis
  _cmd  help.audit
  _sub  help.audit_type
  _cmd  help.conventions
  _sub  help.conventions_force
  _cmd  help.review
  _sub  help.review_branch
  _cmd  help.debug
}

_section_5() {
  _section help.section.observability
  _cmd help.dashboard
  _cmd help.metrics
  _cmd help.optimize
  _cmd help.yield
}

_section_6() {
  _section help.section.maintenance
  _cmd help.deploy
  _sub help.deploy_check
  _sub help.deploy_diff
  _cmd help.sync
  _sub help.sync_dryrun
  _cmd help.update
  _cmd help.upgrade
  _cmd help.worktree_list
  _cmd help.worktree_create
  _cmd help.worktree_remove
  _cmd help.worktree_cleanup
  _cmd help.worktree_status
}

_section_7() {
  _section help.section.config
  _cmd help.config_set
  _cmd help.config_get
  _cmd help.config_list
  _sub help.config_list_providers
  _cmd help.config_unset
  _cmd help.config_language
  _cmd help.config_init_providers
  _cmd help.config_websearch
  _cmd help.plugin_install
}

_section_8() {
  _section help.section.services
  _cmd help.service_setup
  _cmd help.service_status
  _cmd help.service_list
  _cmd help.service_remove
  _cmd help.service_deploy
}

_section_9() {
  _section help.section.beads
  _cmd help.beads_status
  _cmd help.beads_init
  _cmd help.beads_list
  _cmd help.beads_show
  _cmd help.beads_create
  _cmd help.beads_open
  _cmd help.beads_sync
  _cmd help.beads_tracker_status
  _cmd help.beads_tracker_setup
  _cmd help.beads_tracker_switch
  _cmd help.beads_tracker_set_sync_mode
  _cmd help.beads_board
  _cmd help.beads_board_watch
}

# ── Entry point ───────────────────────────────────────────────────────────────

FILTER="${1:-}"

# Header always shown
echo -e "${BOLD}$(t help.title)${RESET}"
echo -e "$(t help.usage) oc <command> [arguments]  ${DIM}·  $(t help.hint)${RESET}"

case "$FILTER" in
  1) _section_1 ;;
  2) _section_2 ;;
  3) _section_3 ;;
  4) _section_4 ;;
  5) _section_5 ;;
  6) _section_6 ;;
  7) _section_7 ;;
  8) _section_8 ;;
  9) _section_9 ;;
  "")
    _section_1
    _section_2
    _section_3
    _section_4
    _section_5
    _section_6
    _section_7
    _section_8
    _section_9

    _section help.section.examples
    _note "oc start"
    _note "oc audit --type security"
    _note "oc deploy all"
    ;;
  *)
    echo -e "${DIM}Section inconnue : $FILTER. Utilise un numéro de 1 à 9.${RESET}"
    ;;
esac

echo ""
