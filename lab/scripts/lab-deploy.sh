#!/bin/sh
# Deploy a fresh build to an existing lab VM (no VM recreate).
#
# Usage:
#   ./lab/scripts/lab-deploy.sh              # panel binary only
#   ./lab/scripts/lab-deploy.sh --full         # panel + scripts + systemd, keep panel.json
#   ./lab/scripts/lab-deploy.sh --full --reset # full redeploy + reset panel.json
#   make lab-deploy
#   make lab-deploy-full
#
# Environment:
#   LAB_SKIP_BUILD=1   use existing dist/panel-linux-*
#   LAB_VM_NAME        Multipass instance (default: xiaomi-vless-lab)
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/scripts/lab-common.sh
. "${SCRIPT_DIR}/lab-common.sh"
lab_common_init

MODE="panel"
RESET_CONFIG=0

for arg in "$@"; do
  case "$arg" in
    --full) MODE="full" ;;
    --reset|--reset-config) RESET_CONFIG=1 ;;
    --panel) MODE="panel" ;;
    -h|--help)
      cat <<'EOF'
Usage: lab-deploy.sh [--panel|--full] [--reset]

  --panel (default)  Build and replace panel binary, restart service
  --full             Redeploy panel + router scripts + systemd units
  --reset            With --full: overwrite panel.json with lab template

Environment: LAB_SKIP_BUILD=1, LAB_VM_NAME=...
EOF
      exit 0
      ;;
    *)
      lab_die "unknown argument: $arg (try --help)"
      ;;
  esac
done

lab_require_multipass
lab_ensure_vm_running

GOARCH="$(lab_detect_vm_goarch)"
PANEL_BIN="${LAB_REPO_ROOT}/dist/panel-linux-${GOARCH}"

if [ "${LAB_SKIP_BUILD:-0}" != "1" ]; then
  PANEL_BIN="$(lab_build_panel "$GOARCH")"
elif [ ! -f "$PANEL_BIN" ]; then
  lab_die "binary not found: $PANEL_BIN — run without LAB_SKIP_BUILD=1"
fi

case "$MODE" in
  panel)
    lab_log "deploying panel only to $LAB_VM_NAME"
    lab_deploy_panel_only "$PANEL_BIN"
    lab_log "panel restarted"
    ;;
  full)
    lab_log "full redeploy to $LAB_VM_NAME (VM stays running)"
    lab_transfer_staging "$PANEL_BIN"
    if [ "$RESET_CONFIG" = "1" ]; then
      lab_run_provision 0 1
    else
      lab_run_provision 1 0
    fi
    lab_log "full redeploy complete"
    ;;
esac

lab_print_panel_url
