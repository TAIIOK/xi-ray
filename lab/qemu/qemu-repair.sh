#!/bin/sh
# Repair common QEMU lab issues on a running guest (empty opkg stubs, missing br-guest, empty xray).
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

qemu_is_running || qemu_die "QEMU not running — run: make qemu-up"

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT INT HUP
qemu_stage_opkg "$STAGE"
qemu_extract_opkg_bundle "${STAGE}/opkg-root" "${STAGE}/opkg"

qemu_log "installing opkg bundle on guest..."
qemu_install_opkg_bundle_on_guest "${STAGE}/opkg-root"

qemu_log "verifying curl/ip and guest network..."
qemu_ssh 'set -eu
curl --version >/dev/null
/sbin/ip -br addr show >/dev/null
# QEMU rootfs often keeps 0-byte stubs even after opkg install; force real binaries over WAN.
opkg update >/dev/null 2>&1 || true
for pkg in \
  libiptext0 libiptext6-0 libiptext-nft0 libnftnl11 \
  xtables-nft iptables-nft libxtables12 \
  kmod-crypto-crc32c kmod-nfnetlink kmod-nft-core kmod-nft-compat kmod-nf-ipt \
  kmod-ipt-core kmod-ipt-nat kmod-ipt-filter kmod-ipt-ipopt; do
  opkg install --force-reinstall "$pkg" >/dev/null 2>&1 \
    || opkg install --force-reinstall "$pkg" \
    || echo "WARN: reinstall failed: $pkg" >&2
done
depmod -a 2>/dev/null || true
for m in crc32c nfnetlink nf_tables nft_compat; do
  modprobe "$m" 2>/dev/null || true
done
sh /mnt/usb-lab/xiaomi-vless/network-setup-openwrt.sh
sh /mnt/usb-lab/xiaomi-vless/xray-guest-iptables.sh
'

CACHE="${QEMU_IMAGES_DIR}/xray-arm64"
if [ ! -s "${CACHE}/xray" ]; then
  qemu_stage_xray "$STAGE"
  CACHE="${STAGE}/xray"
fi

qemu_log "refreshing Xray binaries on USB..."
qemu_ssh 'killall xray 2>/dev/null || true
/etc/init.d/xiaomi-vless-xray stop 2>/dev/null || true
sleep 1'
qemu_ssh "mkdir -p /mnt/usb-lab/xray/bin"
for f in xray geoip.dat geosite.dat; do
  qemu_scp "${CACHE}/${f}" "$(qemu_ssh_target):/mnt/usb-lab/xray/bin/${f}"
done
qemu_ssh 'chmod 755 /mnt/usb-lab/xray/bin/xray
/etc/init.d/xiaomi-vless-xray restart
sleep 3
code=$(curl -4 -sS -o /dev/null -w "%{http_code}" --connect-timeout 15 -x socks5h://127.0.0.1:10808 https://www.google.com/generate_204 || echo 000)
echo "SOCKS probe: ${code}"
[ "${code}" = "204" ] || exit 1
sh /mnt/usb-lab/xiaomi-vless/xray-guest-iptables.sh
/etc/init.d/xiaomi-vless-panel restart 2>/dev/null || true'

qemu_log "repair done — refresh panel in browser, then Apply config"
