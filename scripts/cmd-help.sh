#!/bin/bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

resolve_oc_lang

# ── Section rendering functions ───────────────────────────────────────────────

_section_1() {
  _h_section "$(t help.section.setup)"
  _h_cmd "$(t help.install.cmd)"       "$(t help.install.desc)"
  _h_cmd "$(t help.uninstall.cmd)"     "$(t help.uninstall.desc)"
  _h_cmd "$(t help.init.cmd)"          "$(t help.init.desc)"
  _h_cmd "$(t help.version.cmd)"       "$(t help.version.desc)"
  _h_cmd "$(t help.plugin_install.cmd)" "$(t help.plugin_install.desc)"
}

_section_2() {
  _h_section "$(t help.section.projects)"
  _h_cmd "$(t help.status.cmd)"              "$(t help.status.desc)"
  _h_sub "  --short"                         "$(t help.status_short.desc)"
  _h_cmd "$(t help.remove.cmd)"              "$(t help.remove.desc)"
  _h_sub "  --clean"                         "$(t help.remove_clean.desc)"
  _h_cmd "$(t help.project_rename.cmd)"      "$(t help.project_rename.desc)"
  _h_cmd "$(t help.project_move.cmd)"        "$(t help.project_move.desc)"
  _h_cmd "$(t help.project_configure.cmd)"   "$(t help.project_configure.desc)"
  _h_cmd "$(t help.worktree_list.cmd)"       "$(t help.worktree_list.desc)"
  _h_cmd "$(t help.worktree_create.cmd)"     "$(t help.worktree_create.desc)"
  _h_cmd "$(t help.worktree_remove.cmd)"     "$(t help.worktree_remove.desc)"
  _h_cmd "$(t help.worktree_cleanup.cmd)"    "$(t help.worktree_cleanup.desc)"
  _h_cmd "$(t help.worktree_status.cmd)"     "$(t help.worktree_status.desc)"
}

_section_3() {
  _h_section "$(t help.section.launch)"
  _h_cmd "$(t help.start.cmd)"                   "$(t help.start.desc)"
  _h_sub "  --dev"                               "$(t help.start_dev.desc)"
  _h_sub "  --dev --label <l>"                   "$(t help.start_dev_label.desc)"
  _h_sub "  --dev --assignee <u>"                "$(t help.start_dev_assignee.desc)"
  _h_sub "  --onboard"                           "$(t help.start_onboard.desc)"
  _h_sub "  --onboard --refresh"                 "$(t help.start_onboard_refresh.desc)"
  _h_sub "  --parallel"                          "$(t help.start_parallel.desc)"
  _h_sub "  --worktree [<branch>]"               "$(t help.start_worktree.desc)"
  _h_sub "  --agent <name>"                      "$(t help.start_agent.desc)"
  _h_sub "  --provider <p>"                      "$(t help.start_provider.desc)"
  _h_cmd "$(t help.quick.cmd)"                   "$(t help.quick.desc)"
}

_section_4() {
  _h_section "$(t help.section.analysis)"
  _h_cmd "$(t help.audit.cmd)"                "$(t help.audit.desc)"
  _h_sub "  --type <type>"                    "$(t help.audit_type.desc)"
  _h_cmd "$(t help.conventions.cmd)"          "$(t help.conventions.desc)"
  _h_sub "  --force"                          "$(t help.conventions_force.desc)"
  _h_cmd "$(t help.review.cmd)"               "$(t help.review.desc)"
  _h_sub "  --branch <branch>"               "$(t help.review_branch.desc)"
  _h_cmd "$(t help.debug.cmd)"                "$(t help.debug.desc)"
}

_section_5() {
  _h_section "$(t help.section.observability)"
  _h_cmd "$(t help.dashboard.cmd)"   "$(t help.dashboard.desc)"
  _h_cmd "$(t help.metrics.cmd)"     "$(t help.metrics.desc)"
  _h_cmd "$(t help.optimize.cmd)"    "$(t help.optimize.desc)"
  _h_cmd "$(t help.yield.cmd)"       "$(t help.yield.desc)"
}

_section_6() {
  _h_section "$(t help.section.deployment)"
  _h_cmd "$(t help.deploy.cmd)"       "$(t help.deploy.desc)"
  _h_sub "  --check"                  "$(t help.deploy_check.desc)"
  _h_sub "  --diff"                   "$(t help.deploy_diff.desc)"
  _h_cmd "$(t help.sync.cmd)"         "$(t help.sync.desc)"
  _h_sub "  --dry-run"                "$(t help.sync_dryrun.desc)"
}

_section_7() {
  _h_section "$(t help.section.updates)"
  _h_cmd "$(t help.update.cmd)"    "$(t help.update.desc)"
  _h_cmd "$(t help.upgrade.cmd)"   "$(t help.upgrade.desc)"
}

_section_8() {
  _h_section "$(t help.section.config)"
  _h_cmd "$(t help.config_set.cmd)"            "$(t help.config_set.desc)"
  _h_cmd "$(t help.config_get.cmd)"            "$(t help.config_get.desc)"
  _h_cmd "$(t help.config_list.cmd)"           "$(t help.config_list.desc)"
  _h_sub "  --providers"                       "$(t help.config_list_providers.desc)"
  _h_cmd "$(t help.config_unset.cmd)"          "$(t help.config_unset.desc)"
  _h_cmd "$(t help.config_language.cmd)"       "$(t help.config_language.desc)"
  _h_cmd "$(t help.config_init_providers.cmd)" "$(t help.config_init_providers.desc)"
  _h_cmd "$(t help.config_websearch.cmd)"      "$(t help.config_websearch.desc)"
}

_section_9() {
  _h_section "$(t help.section.services)"
  _h_cmd "$(t help.service_setup.cmd)"   "$(t help.service_setup.desc)"
  _h_cmd "$(t help.service_status.cmd)"  "$(t help.service_status.desc)"
  _h_cmd "$(t help.service_list.cmd)"    "$(t help.service_list.desc)"
  _h_cmd "$(t help.service_remove.cmd)"  "$(t help.service_remove.desc)"
  _h_cmd "$(t help.service_deploy.cmd)"  "$(t help.service_deploy.desc)"
}

_section_10() {
  _h_section "$(t help.section.beads)"
  _h_cmd "$(t help.beads_status.cmd)"                    "$(t help.beads_status.desc)"
  _h_cmd "$(t help.beads_init.cmd)"                      "$(t help.beads_init.desc)"
  _h_cmd "$(t help.beads_list.cmd)"                      "$(t help.beads_list.desc)"
  _h_cmd "$(t help.beads_show.cmd)"                      "$(t help.beads_show.desc)"
  _h_cmd "$(t help.beads_create.cmd)"                    "$(t help.beads_create.desc)"
  _h_cmd "$(t help.beads_open.cmd)"                      "$(t help.beads_open.desc)"
  _h_cmd "$(t help.beads_sync.cmd)"                      "$(t help.beads_sync.desc)"
  _h_cmd "$(t help.beads_tracker_status.cmd)"            "$(t help.beads_tracker_status.desc)"
  _h_cmd "$(t help.beads_tracker_setup.cmd)"             "$(t help.beads_tracker_setup.desc)"
  _h_cmd "$(t help.beads_tracker_switch.cmd)"            "$(t help.beads_tracker_switch.desc)"
  _h_cmd "$(t help.beads_tracker_set_sync_mode.cmd)"     "$(t help.beads_tracker_set_sync_mode.desc)"
  _h_cmd "$(t help.beads_board.cmd)"                     "$(t help.beads_board.desc)"
  _h_cmd "$(t help.beads_board_watch.cmd)"               "$(t help.beads_board_watch.desc)"
}

# ── Entry point ───────────────────────────────────────────────────────────────

FILTER="${1:-}"

echo -e "${BOLD}$(t help.title)${RESET}"
echo -e "$(t help.usage) oc <command> [arguments]  ${DIM}·  $(t help.hint)${RESET}"

case "$FILTER" in
  1)  _section_1 ;;
  2)  _section_2 ;;
  3)  _section_3 ;;
  4)  _section_4 ;;
  5)  _section_5 ;;
  6)  _section_6 ;;
  7)  _section_7 ;;
  8)  _section_8 ;;
  9)  _section_9 ;;
  10) _section_10 ;;
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
    _section_10

    _h_section "$(t help.section.examples)"
    _h_note "oc start"
    _h_note "oc audit --type security"
    _h_note "oc deploy all"
    ;;
  *)
    echo -e "${DIM}Section inconnue : $FILTER. Utilise un numéro de 1 à 10.${RESET}"
    ;;
esac

echo ""
