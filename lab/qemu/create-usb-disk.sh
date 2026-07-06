#!/bin/sh
# Create the lab "USB flash" virtio disk (ext4, mounted at /mnt/usb-lab in guest).
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

qemu_require_tools
command -v qemu-img >/dev/null 2>&1 || qemu_die "qemu-img not found — install qemu"

USB_SIZE="${QEMU_USB_SIZE:-2G}"

if [ -f "$QEMU_USB_DISK" ]; then
  qemu_log "USB disk already exists: $QEMU_USB_DISK"
  exit 0
fi

mkdir -p "$(dirname "$QEMU_USB_DISK")"
qemu_log "creating USB disk ($USB_SIZE) at $QEMU_USB_DISK"
qemu-img create -f qcow2 "$QEMU_USB_DISK" "$USB_SIZE"
qemu_log "USB disk image created (will be formatted ext4 on first provision)"
