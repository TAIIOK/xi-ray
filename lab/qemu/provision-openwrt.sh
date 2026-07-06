#!/bin/sh
# Provisions the QEMU OpenWrt guest (run inside the VM as root via SSH).
set -eu

STAGING="${QEMU_STAGING:-/tmp/qemu-staging}"
USB_MOUNT="/mnt/usb-lab"
INSTALL_DIR="${USB_MOUNT}/xiaomi-vless"
XRAY_DIR="${USB_MOUNT}/xray"
XRAY_BIN="${XRAY_DIR}/bin/xray"
DATA_DIR="${USB_MOUNT}/data"

log() { echo "[qemu-provision] $*"; }
die() { echo "[qemu-provision] ERROR: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root"

setup_data_symlink() {
  if [ ! -e /data ]; then
    ln -sf "$DATA_DIR" /data
    log "/data -> $DATA_DIR"
  fi
}

install_staged_opkg() {
  root="${STAGING}/opkg-root"
  [ -d "$root" ] || return 0
  log "installing pinned opkg packages from staging bundle..."
  (cd "$root" && tar -cf - .) | tar -xf - -C /
}

install_opkg_packages() {
  log "installing OpenWrt packages..."
  install_staged_opkg
  # Internet via WAN (eth1 / 192.0.2.0/24) per OpenWrt QEMU docs.
  opkg update >/dev/null 2>&1 || opkg update
  for pkg in \
    e2fsprogs kmod-fs-ext4 block-mount \
    curl libcurl4 ca-bundle unzip zlib libbpf1 libelf1 \
    iptables libxtables12 ip-full ip-bridge kmod-tun \
    kmod-ipt-nat kmod-ipt-filter kmod-veth \
    bind-dig; do
    opkg install "$pkg" >/dev/null 2>&1 || opkg install "$pkg" || log "WARN: package $pkg missing"
  done
  fix_opkg_binaries
}

fix_opkg_binaries() {
  # First boot on QEMU sometimes leaves 0-byte stubs; reinstall from staging if curl still broken.
  log "verifying opkg binaries (ip, curl, iptables)..."
  if ! curl --version >/dev/null 2>&1; then
    install_staged_opkg
  fi
  if opkg update >/dev/null 2>&1; then
    for pkg in \
      libiptext0 libiptext6-0 libiptext-nft0 libnftnl11 \
      xtables-nft iptables-nft libxtables12 \
      kmod-crypto-crc32c kmod-nfnetlink kmod-nft-core kmod-nft-compat kmod-nf-ipt \
      kmod-ipt-core kmod-ipt-nat kmod-ipt-filter kmod-ipt-ipopt; do
      opkg install --force-reinstall "$pkg" >/dev/null 2>&1 \
        || opkg install --force-reinstall "$pkg" \
        || log "WARN: reinstall failed: $pkg"
    done
  fi
  for pkg in zlib libbpf1 libelf1 ip-full iptables libxtables12 curl libcurl4; do
    opkg install --force-reinstall "$pkg" >/dev/null 2>&1 \
      || opkg install --force-reinstall "$pkg" \
      || log "WARN: reinstall failed: $pkg"
  done
  if [ ! -s /etc/ssl/certs/ca-certificates.crt ]; then
    log "restoring ca-bundle from offline staging bundle..."
    install_staged_opkg
  fi
  if [ ! -s /etc/ssl/certs/ca-certificates.crt ]; then
    opkg install --force-reinstall ca-bundle >/dev/null 2>&1 \
      || opkg install --force-reinstall ca-bundle \
      || log "WARN: reinstall failed: ca-bundle"
  fi
  if [ ! -s /etc/ssl/certs/ca-certificates.crt ]; then
    die "ca-bundle missing — HTTPS subscription fetch will fail"
  fi
  if ! /sbin/ip -br link >/dev/null 2>&1; then
    die "/sbin/ip broken after opkg install — check WAN/internet in guest"
  fi
  if ! curl --version >/dev/null 2>&1; then
    die "/usr/bin/curl broken after opkg install — VPN health probe needs curl"
  fi
}

verify_staged_xray() {
  for f in xray geoip.dat geosite.dat; do
    if [ ! -s "${STAGING}/xray/${f}" ]; then
      die "staged xray/${f} missing or empty — re-run make qemu-up on host"
    fi
  done
}

is_mounted() {
  grep -q " $1 " /proc/mounts 2>/dev/null
}

mount_usb_disk() {
  mkdir -p "$USB_MOUNT"
  if is_mounted "$USB_MOUNT"; then
    log "USB already mounted at $USB_MOUNT"
    return 0
  fi

  dev=""
  for candidate in /dev/vdb1 /dev/vdb /dev/sdb1 /dev/sdb; do
    [ -b "$candidate" ] || continue
    dev="$candidate"
    break
  done
  [ -n "$dev" ] || die "USB virtio disk not found"

  if ! blkid "$dev" 2>/dev/null | grep -q ext4; then
    log "formatting $dev as ext4..."
    mkfs.ext4 -F -L usb-lab "$dev"
  fi

  grep -q "$USB_MOUNT" /etc/fstab 2>/dev/null || \
    echo "$dev $USB_MOUNT ext4 defaults 0 2" >> /etc/fstab
  mount -t ext4 "$dev" "$USB_MOUNT" 2>/dev/null || mount "$dev" "$USB_MOUNT"
  is_mounted "$USB_MOUNT" || die "failed to mount $USB_MOUNT"
  log "mounted $dev at $USB_MOUNT"
}

install_panel_binary() {
  src="$1"
  dst="${INSTALL_DIR}/panel"
  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel stop 2>/dev/null || true
  fi
  killall panel 2>/dev/null || true
  sleep 1
  if [ -f "$dst" ]; then
    cp "$dst" "${dst}.previous"
  fi
  cp "$src" "${dst}.new"
  mv "${dst}.new" "$dst"
  chmod 755 "$dst"
}

install_script() {
  name="$1"
  [ -f "${STAGING}/${name}" ] || die "missing staged file: $name"
  cp "${STAGING}/${name}" "${INSTALL_DIR}/${name}"
  chmod +x "${INSTALL_DIR}/${name}"
}

download_xray() {
  ZIP_URL="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip"
  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT INT HUP

  log "downloading Xray (arm64)..."
  curl -fsSL "$ZIP_URL" -o "${TMP}/xray.zip"
  unzip -qo "${TMP}/xray.zip" -d "${TMP}/xray"
  xray_src="$(find "${TMP}/xray" -maxdepth 2 -name xray -type f | head -1)"
  if [ -n "$xray_src" ]; then
    cp "$xray_src" "$XRAY_BIN"
    chmod 755 "$XRAY_BIN"
  else
    die "xray binary not found in archive"
  fi
  [ -s "$XRAY_BIN" ] || die "xray binary is empty after install"

  curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" \
    -o "${XRAY_DIR}/bin/geoip.dat"
  curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" \
    -o "${XRAY_DIR}/bin/geosite.dat"
  log "Xray installed to $XRAY_BIN"
}

start_panel() {
  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
    /etc/init.d/xiaomi-vless-panel restart 2>/dev/null || /etc/init.d/xiaomi-vless-panel start
    i=0
    while [ "$i" -lt 20 ]; do
      if wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null; then
        return 0
      fi
      i=$((i + 1))
      sleep 1
    done
    return 1
  fi
  die "xiaomi-vless-panel init.d missing"
}

install_opkg_packages
mount_usb_disk
setup_data_symlink

mkdir -p "$INSTALL_DIR" "${XRAY_DIR}/bin" "${INSTALL_DIR}/updates/downloads" \
  "${INSTALL_DIR}/updates/staging" "$DATA_DIR"
chmod 755 "$INSTALL_DIR"

if [ -f "${STAGING}/panel-linux" ]; then
  install_panel_binary "${STAGING}/panel-linux"
else
  die "staged panel binary missing"
fi
log "panel installed"

if [ "${QEMU_RESET_CONFIG:-0}" = "1" ]; then
  cp "${STAGING}/panel-lab.json" "${INSTALL_DIR}/panel.json"
  log "panel.json reset from lab template"
  killall xray 2>/dev/null || true
  echo '{}' > "${XRAY_DIR}/config.json"
  rm -f "${INSTALL_DIR}/panel.previous" /data/xiaomi-vless-failopen
  rm -rf "${INSTALL_DIR}/updates/staging/"* 2>/dev/null || true
elif [ -f "${INSTALL_DIR}/panel.json" ] && [ "${QEMU_KEEP_PANEL_JSON:-0}" = "1" ]; then
  log "keeping existing panel.json"
elif [ -f "${STAGING}/panel-lab.json" ]; then
  cp "${STAGING}/panel-lab.json" "${INSTALL_DIR}/panel.json"
else
  die "panel.json not found"
fi
chmod 600 "${INSTALL_DIR}/panel.json"

for script in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-iptables-cron.sh \
  xray-guest-sysctl.sh xiaomi-vless-failopen-guard.sh boot-xiaomi-vless.sh \
  panel-updater.sh network-setup-openwrt.sh guest-netns.sh; do
  install_script "$script"
done

cp "${STAGING}/panel-updater.sh" "${INSTALL_DIR}/panel-updater.sh"
chmod +x "${INSTALL_DIR}/panel-updater.sh"

if [ ! -x "$XRAY_BIN" ] || [ ! -s "$XRAY_BIN" ]; then
  if [ -x "${STAGING}/xray/xray" ] && [ -s "${STAGING}/xray/xray" ]; then
    verify_staged_xray
    mkdir -p "${XRAY_DIR}/bin"
    cp "${STAGING}/xray/xray" "$XRAY_BIN"
    chmod 755 "$XRAY_BIN"
    cp "${STAGING}/xray/geoip.dat" "${XRAY_DIR}/bin/geoip.dat"
    cp "${STAGING}/xray/geosite.dat" "${XRAY_DIR}/bin/geosite.dat"
    log "Xray installed from staging"
  else
    download_xray
  fi
else
  log "keeping existing Xray at $XRAY_BIN"
fi
[ -f "${XRAY_DIR}/config.json" ] || echo '{}' > "${XRAY_DIR}/config.json"

sh "${INSTALL_DIR}/network-setup-openwrt.sh"

# procd autostart — same path as BE7000 (no systemd).
cp "${STAGING}/xiaomi-vless-panel.init" /etc/init.d/xiaomi-vless-panel
cp "${STAGING}/xiaomi-vless-xray.init" /etc/init.d/xiaomi-vless-xray
cp "${STAGING}/xiaomi-vless-boot.init" /etc/init.d/xiaomi-vless-boot
chmod +x /etc/init.d/xiaomi-vless-panel /etc/init.d/xiaomi-vless-xray /etc/init.d/xiaomi-vless-boot

INSTALL_DIR="$INSTALL_DIR" \
USB_MOUNT="$USB_MOUNT" \
INIT_SRC="${STAGING}/xiaomi-vless-xray.init" \
BOOT_SRC="${STAGING}/boot-xiaomi-vless.sh" \
BOOT_SCRIPT="/data/xiaomi-vless-boot.sh" \
GUARD_SCRIPT="/data/xiaomi-vless-failopen-guard.sh" \
USER_STARTUP="/data/startup_user.sh" \
CRON_FILE="/etc/crontabs/root" \
sh "${STAGING}/install-autostart.sh"

if start_panel; then
  log "panel running via procd"
else
  log "ERROR: panel failed — check: logread | tail -50; cat ${INSTALL_DIR}/panel.log"
  exit 1
fi

log "provision complete"
log "USB mount: $USB_MOUNT"
log "panel: http://127.0.0.1:7777 (host port-forward)"
