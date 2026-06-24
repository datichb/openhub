#!/bin/bash

set -euo pipefail

HUB_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPTS_DIR="$HUB_DIR/scripts"

# Source des variables communes
source "$SCRIPTS_DIR/common.sh"

# S'assurer que hub.json existe (créé depuis hub.json.example si absent)
ensure_hub_config

COMMAND="${1:-}"

case "$COMMAND" in
  install)         bash "$SCRIPTS_DIR/cmd-install.sh" "${@:2}" ;;
  uninstall)       bash "$SCRIPTS_DIR/cmd-uninstall.sh" "${@:2}" ;;
  init)            bash "$SCRIPTS_DIR/cmd-init.sh" "${@:2}" ;;
  list)            bash "$SCRIPTS_DIR/cmd-status.sh" --short ;;
  status)          bash "$SCRIPTS_DIR/cmd-status.sh" "${@:2}" ;;
  remove)          bash "$SCRIPTS_DIR/cmd-remove.sh" "${@:2}" ;;
  project)         bash "$SCRIPTS_DIR/cmd-project.sh" "${@:2}" ;;
  start)           bash "$SCRIPTS_DIR/cmd-start.sh" "${@:2}" ;;
  audit)           bash "$SCRIPTS_DIR/cmd-audit.sh" "${@:2}" ;;
  review)          bash "$SCRIPTS_DIR/cmd-review.sh" "${@:2}" ;;
  debug)           bash "$SCRIPTS_DIR/cmd-debug.sh" "${@:2}" ;;
  deploy)          bash "$SCRIPTS_DIR/cmd-deploy.sh" "${@:2}" ;;
  sync)            bash "$SCRIPTS_DIR/cmd-sync.sh" "${@:2}" ;;
  config)          bash "$SCRIPTS_DIR/cmd-config.sh" "${@:2}" ;;
  skills)          bash "$SCRIPTS_DIR/cmd-skills.sh" "${@:2}" ;;
  agent)           bash "$SCRIPTS_DIR/cmd-agent.sh" "${@:2}" ;;
  plugin)          bash "$SCRIPTS_DIR/cmd-plugin.sh" "${@:2}" ;;
  update)          bash "$SCRIPTS_DIR/cmd-update.sh" ;;
  upgrade)         bash "$SCRIPTS_DIR/cmd-upgrade.sh" "${@:2}" ;;
  service)         bash "$SCRIPTS_DIR/cmd-service.sh" "${@:2}" ;;
  figma)           bash "$SCRIPTS_DIR/cmd-service.sh" "${2:-list}" "${@:3}" figma ;;
  gitlab)          bash "$SCRIPTS_DIR/cmd-service.sh" "${2:-list}" "${@:3}" gitlab ;;
  gslides)         bash "$SCRIPTS_DIR/cmd-service.sh" "${2:-list}" "${@:3}" gslides ;;
  conventions)     bash "$SCRIPTS_DIR/cmd-conventions.sh" "${@:2}" ;;
  worktree)        bash "$SCRIPTS_DIR/cmd-worktree.sh" "${@:2}" ;;
  beads)           bash "$SCRIPTS_DIR/cmd-beads.sh" "${@:2}" ;;
  quick)           bash "$SCRIPTS_DIR/cmd-quick.sh" "${@:2}" ;;
  metrics)         bash "$SCRIPTS_DIR/cmd-metrics.sh" "${@:2}" ;;
  dashboard)       bash "$SCRIPTS_DIR/cmd-dashboard.sh" "${@:2}" ;;
  optimize)        bash "$SCRIPTS_DIR/cmd-optimize.sh" "${@:2}" ;;
  yield)           bash "$SCRIPTS_DIR/cmd-yield.sh" "${@:2}" ;;
  version|--version) bash "$SCRIPTS_DIR/cmd-version.sh" ;;
  help|--help|-h)  bash "$SCRIPTS_DIR/cmd-help.sh" ;;
  "")              bash "$SCRIPTS_DIR/cmd-help.sh" ;;
  *)
    resolve_oc_lang
    log_error "$(t cmd.unknown) : $COMMAND"
    bash "$SCRIPTS_DIR/cmd-help.sh"
    exit 1
    ;;
esac