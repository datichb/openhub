#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Layout constants ──────────────────────────────────────────────────────────
# CMD_W: width reserved for the command column (pad with spaces).
# Chosen to accommodate the longest standard command string without truncation.
CMD_W=44

# ── Helper functions ──────────────────────────────────────────────────────────

# Print a section header: bold title followed by a separator line.
_section() {
  local title
  title="$(t "$1")"
  echo ""
  echo -e "${BOLD}${title}${RESET}"
  printf '%*s\n' "${#title}" '' | tr ' ' '─'
}

# Print one command row: cyan command column + normal description column.
# For commands that fit within CMD_W, command and description are on one line.
_cmd() {
  local cmd desc
  cmd="$(t "$1.cmd")"
  desc="$(t "$1.desc")"
  if [ "${#cmd}" -le "$CMD_W" ]; then
    printf "  ${CYAN}%-${CMD_W}s${RESET}  %s\n" "$cmd" "$desc"
  else
    # Long signature: command on its own line, description indented below.
    printf "  ${CYAN}%s${RESET}\n" "$cmd"
    printf "  %*s  %s\n" "$CMD_W" "" "$desc"
  fi
}

# Print a plain indented note (used for deploy targets, examples, …).
_note() {
  echo "  $1"
}

# ── Help output ───────────────────────────────────────────────────────────────

echo -e "${BOLD}$(t help.title)${RESET}"
echo -e "$(t help.usage) ./oc.sh <command> [arguments]"

_section help.section.setup
_cmd help.install
_cmd help.uninstall
_cmd help.init
_cmd help.version

_section help.section.projects
_cmd help.status
_cmd help.status_short
_cmd help.remove
_cmd help.remove_clean
_cmd help.project_rename
_cmd help.project_move

_section help.section.launch
_cmd help.start
_cmd help.start_dev
_cmd help.start_dev_label
_cmd help.start_dev_assignee
_cmd help.start_onboard

_section help.section.analysis
_cmd help.audit
_cmd help.audit_type
_cmd help.conventions
_cmd help.conventions_force
_cmd help.review
_cmd help.review_branch
_cmd help.debug
_cmd help.metrics

_section help.section.maintenance
_cmd help.deploy
_cmd help.deploy_check
_cmd help.deploy_diff
_cmd help.sync
_cmd help.sync_dryrun
_cmd help.update
_cmd help.upgrade

_section help.section.config
_cmd help.config_set
_cmd help.config_get
_cmd help.config_list
_cmd help.config_list_providers
_cmd help.config_unset
_cmd help.config_language
_cmd help.config_init_providers

_section help.section.deploy_targets
_note "$(t help.deploy_target.opencode)"

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

_section help.section.examples
_note "./oc.sh start"
_note "./oc.sh audit security"
_note "./oc.sh deploy all"
echo ""
