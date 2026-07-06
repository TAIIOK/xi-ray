#!/bin/sh
# Ensure dropbear SSH is enabled in the OpenWrt rootfs image.
# Network config is left as OpenWrt defaults (LAN 192.168.1.1, WAN DHCP on eth1).
# See: https://openwrt.org/docs/guide-user/virtualization/qemu
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

DEBUGFS="${DEBUGFS:-$(command -v debugfs 2>/dev/null || true)}"
[ -n "$DEBUGFS" ] || DEBUGFS="$(find /opt/homebrew/Cellar/e2fsprogs -name debugfs 2>/dev/null | head -1)"
[ -n "$DEBUGFS" ] || { qemu_log "ERROR: debugfs not found — brew install e2fsprogs"; exit 1; }

ROOTFS_GZ="openwrt-${QEMU_OWRT_VERSION}-armsr-armv8-generic-ext4-rootfs.img.gz"
ROOTFS_URL="${QEMU_DOWNLOAD_BASE}/${ROOTFS_GZ}"
ROOTFS_IMG="${QEMU_IMAGES_DIR}/openwrt-rootfs-only.img"
MARKER="${QEMU_IMAGES_DIR}/.qemu-lab-patched-v5"

mkdir -p "$QEMU_IMAGES_DIR"

if [ ! -f "$ROOTFS_IMG" ]; then
  qemu_log "downloading rootfs image..."
  curl -fL --retry 3 -o "${ROOTFS_IMG}.gz" "$ROOTFS_URL"
  gzip -dc "${ROOTFS_IMG}.gz" > "$ROOTFS_IMG"
  rm -f "${ROOTFS_IMG}.gz"
fi

if [ -f "$MARKER" ]; then
  qemu_log "rootfs already patched for QEMU lab"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT HUP

# Stock OpenWrt network: eth0=LAN 192.168.1.1, eth1=WAN DHCP (192.0.2.0/24 in QEMU).
cat >"$TMP/network" <<'EOF'
config interface 'loopback'
	option device 'lo'
	option proto 'static'
	option ipaddr '127.0.0.1'
	option netmask '255.0.0.0'

config globals 'globals'
	option ula_prefix 'fd00::/48'

config device
	option name 'br-lan'
	option type 'bridge'
	list ports 'eth0'

config interface 'lan'
	option device 'br-lan'
	option proto 'static'
	option ipaddr '192.168.1.1'
	option netmask '255.255.255.0'
	option ip6assign '60'

config interface 'wan'
	option device 'eth1'
	option proto 'dhcp'
EOF

cat >"$TMP/dropbear" <<'EOF'
config dropbear main
	option enable '1'
	option PasswordAuth 'on'
	option RootPasswordAuth 'on'
	option Port '22'
EOF

qemu_log "patching rootfs (stock LAN 192.168.1.1 + dropbear)..."
"$DEBUGFS" -w -R "rm /etc/config/network" "$ROOTFS_IMG"
"$DEBUGFS" -w -R "write $TMP/network /etc/config/network" "$ROOTFS_IMG"
"$DEBUGFS" -w -R "rm /etc/config/dropbear" "$ROOTFS_IMG" 2>/dev/null || true
"$DEBUGFS" -w -R "write $TMP/dropbear /etc/config/dropbear" "$ROOTFS_IMG"

date -u +%Y-%m-%dT%H:%M:%SZ >"$MARKER"
qemu_log "rootfs patched: $ROOTFS_IMG"
