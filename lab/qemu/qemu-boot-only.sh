#!/bin/sh
# Start QEMU OpenWrt and wait for SSH (no provision). For debugging.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init
qemu_require_tools

mkdir -p "$QEMU_RUNTIME_DIR" "$QEMU_IMAGES_DIR"
sh "${SCRIPT_DIR}/download-images.sh"
sh "${SCRIPT_DIR}/create-usb-disk.sh"

if ! qemu_is_running; then
  qemu_start_vm
else
  qemu_log "QEMU already running"
fi

qemu_wait_for_ssh
qemu_print_panel_url
