#!/bin/sh
# Delete QEMU runtime state and optionally images/USB disk.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

sh "${SCRIPT_DIR}/qemu-down.sh"

if [ "${QEMU_PURGE_IMAGES:-0}" = "1" ]; then
  rm -f "$QEMU_IMAGE" "$QEMU_UBOOT" "$QEMU_USB_DISK" "${QEMU_IMAGE}.gz" "${QEMU_UBOOT}.gz"
  qemu_log "removed downloaded images"
fi

rm -rf "$QEMU_RUNTIME_DIR"
qemu_log "runtime cleared"
