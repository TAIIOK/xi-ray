#!/bin/sh
# Redeploy panel binary to running QEMU OpenWrt lab (procd restart).
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init
qemu_require_tools

FULL=0
KEEP_CONFIG=0
RESET_CONFIG=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --full) FULL=1 ;;
    --keep-config) KEEP_CONFIG=1 ;;
    --reset) RESET_CONFIG=1 ;;
    *) qemu_die "unknown arg: $1 (use --full, --keep-config, --reset)" ;;
  esac
  shift
done

qemu_is_running || qemu_die "QEMU not running — run: make qemu-up"
qemu_wait_for_ssh

PANEL_BIN="$(qemu_build_panel)"

if [ "$FULL" -eq 1 ]; then
  STAGE="$(mktemp -d)"
  trap 'rm -rf "$STAGE"' EXIT INT HUP
  qemu_stage_files "$STAGE" "$PANEL_BIN"
  qemu_transfer_staging "$STAGE"
  qemu_ssh "QEMU_STAGING=/tmp/qemu-staging QEMU_KEEP_PANEL_JSON=${KEEP_CONFIG} QEMU_RESET_CONFIG=${RESET_CONFIG} sh /tmp/qemu-staging/provision-openwrt.sh"
  qemu_print_panel_url
  exit 0
fi

TMP_PANEL="$(mktemp)"
TMP_UPDATER="$(mktemp)"
trap 'rm -f "$TMP_PANEL" "$TMP_UPDATER"' EXIT INT HUP
cp "$PANEL_BIN" "$TMP_PANEL"
cp "${QEMU_REPO_ROOT}/deploy/panel-updater.sh" "$TMP_UPDATER"
chmod 755 "$TMP_PANEL" "$TMP_UPDATER"

qemu_scp "$TMP_PANEL" "$(qemu_ssh_target):/tmp/panel-linux"
qemu_scp "$TMP_UPDATER" "$(qemu_ssh_target):/tmp/panel-updater.sh"

qemu_ssh 'sh -s' <<'REMOTE'
set -eu
INSTALL="/mnt/usb-lab/xiaomi-vless"
[ -x "$INSTALL/panel" ] && cp "$INSTALL/panel" "$INSTALL/panel.previous"
cp /tmp/panel-linux "$INSTALL/panel.new"
mv "$INSTALL/panel.new" "$INSTALL/panel"
chmod 755 "$INSTALL/panel"
cp /tmp/panel-updater.sh "$INSTALL/panel-updater.sh"
chmod +x "$INSTALL/panel-updater.sh"
/etc/init.d/xiaomi-vless-panel restart
i=0
while [ "$i" -lt 15 ]; do
  if wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null; then
    exit 0
  fi
  i=$((i + 1))
  sleep 1
done
/etc/init.d/xiaomi-vless-panel status || true
exit 1
REMOTE

qemu_log "panel redeployed"
qemu_print_panel_url
