#!/bin/sh
# Launch QEMU OpenWrt lab and provision panel (procd, no systemd).
#
# Usage:
#   ./lab/qemu/qemu-up.sh
#   make qemu-up
#
# Environment:
#   QEMU_OWRT_VERSION   OpenWrt release (default: 24.10.0)
#   QEMU_MEMORY         RAM in MB (default: 1024)
#   QEMU_CPUS           vCPUs (default: 2)
#   QEMU_SSH_PORT       host SSH forward (default: 2222)
#   QEMU_PANEL_PORT     host panel forward (default: 7777)
#   QEMU_RECREATE       set to 1 to stop VM and reprovision
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init
qemu_require_tools

mkdir -p "$QEMU_RUNTIME_DIR" "$QEMU_IMAGES_DIR"

if [ "${QEMU_RECREATE:-0}" = "1" ] && qemu_is_running; then
  qemu_log "recreating VM — stopping running instance"
  sh "${SCRIPT_DIR}/qemu-down.sh"
fi

sh "${SCRIPT_DIR}/download-images.sh"
sh "${SCRIPT_DIR}/create-usb-disk.sh"

if ! qemu_is_running; then
  qemu_start_vm
else
  qemu_log "QEMU already running"
fi

qemu_wait_for_ssh

NEED_PROVISION=1
if [ "${QEMU_RECREATE:-0}" != "1" ]; then
  if qemu_ssh "test -x /mnt/usb-lab/xiaomi-vless/panel" 2>/dev/null; then
    NEED_PROVISION=0
    qemu_log "already provisioned — skipping (QEMU_RECREATE=1 to force)"
  fi
fi

if [ "$NEED_PROVISION" != "1" ]; then
  qemu_print_panel_url
  exit 0
fi

PANEL_BIN="$(qemu_build_panel)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT INT HUP
qemu_stage_files "$STAGE" "$PANEL_BIN"
qemu_transfer_staging "$STAGE"

qemu_log "provisioning OpenWrt guest (panel, Xray, procd autostart)..."
qemu_ssh "QEMU_STAGING=/tmp/qemu-staging QEMU_KEEP_PANEL_JSON=0 QEMU_RESET_CONFIG=0 sh /tmp/qemu-staging/provision-openwrt.sh"

cat <<EOF

QEMU OpenWrt lab is ready.

  OpenWrt:  ${QEMU_OWRT_VERSION} (armsr/armv8)
  Panel:    http://127.0.0.1:${QEMU_PANEL_PORT}
  Setup:    http://127.0.0.1:${QEMU_PANEL_PORT}/onboarding  (admin / admin)
  SSH:      ssh -p ${QEMU_SSH_PORT} root@127.0.0.1

Useful commands:
  ./lab/qemu/qemu-shell.sh
  ./lab/qemu/qemu-status.sh
  ./lab/qemu/qemu-guest-test.sh
  ./lab/qemu/qemu-deploy.sh
  ./lab/qemu/qemu-down.sh

Inside guest (as root):
  ip netns exec guest-test curl -4 https://ifconfig.me
  iptables -t nat -L XRAY_GUEST_TCP -v -n
  /etc/init.d/xiaomi-vless-panel restart

EOF
