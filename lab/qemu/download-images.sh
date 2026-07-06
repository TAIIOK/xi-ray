#!/bin/sh
# Download OpenWrt armsr/armv8 QEMU images for the lab.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

mkdir -p "$QEMU_IMAGES_DIR"

download_if_missing() {
  url="$1"
  dest="$2"

  if [ -f "$dest" ]; then
    qemu_log "already present: $(basename "$dest")"
    return 0
  fi

  qemu_log "downloading $(basename "$dest")..."
  if echo "$url" | grep -q '\.gz$'; then
    curl -fL --retry 3 --retry-delay 2 -o "${dest}.gz" "$url"
    gzip -dc "${dest}.gz" > "$dest"
    rm -f "${dest}.gz"
    [ -s "$dest" ] || qemu_die "decompressed file is empty: $dest"
  else
    curl -fL --retry 3 --retry-delay 2 -o "$dest" "$url"
  fi
}

download_if_missing "$QEMU_KERNEL_URL" "$QEMU_KERNEL"
download_if_missing "${QEMU_DOWNLOAD_BASE}/${QEMU_ROOTFS_GZ}" "$QEMU_ROOTFS"

if [ "$(uname -s)" = Darwin ]; then
  sh "${SCRIPT_DIR}/patch-openwrt-rootfs.sh"
fi

qemu_log "OpenWrt images ready in $QEMU_IMAGES_DIR"
