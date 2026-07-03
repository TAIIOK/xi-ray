#!/bin/sh
# Install xiaomi-vless from git checkout (developer path).
#
#   make build-arm64
#   ssh root@192.168.31.1 'sh -s' < deploy/install.sh
#
# Or on the router from a cloned repo:
#   make install
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
COMMON_SH="${SCRIPT_DIR}/deploy/install-common.sh"
[ -f "$COMMON_SH" ] || { echo "missing $COMMON_SH" >&2; exit 1; }
# shellcheck source=deploy/install-common.sh
. "$COMMON_SH"

USB_MOUNT="$(find_usb_mount)" || die "USB not mounted under /mnt/usb-* — plug in USB and retry"

INSTALL_DIR="${USB_MOUNT}/xiaomi-vless"
PANEL_BIN="${INSTALL_DIR}/panel"
PANEL_CONFIG="${INSTALL_DIR}/panel.json"

log "source: $SCRIPT_DIR"
log "USB mount: $USB_MOUNT"
log "install dir: $INSTALL_DIR"

mkdir -p "$INSTALL_DIR"
mkdir -p "${INSTALL_DIR}/updates/downloads" "${INSTALL_DIR}/updates/staging"
chmod 755 "$INSTALL_DIR"

if [ -f "${SCRIPT_DIR}/dist/panel-linux-arm64" ]; then
  cp "${SCRIPT_DIR}/dist/panel-linux-arm64" "$PANEL_BIN"
elif [ -f "${SCRIPT_DIR}/dist/panel" ]; then
  cp "${SCRIPT_DIR}/dist/panel" "$PANEL_BIN"
else
  die "Build binary first: make build-arm64"
fi
chmod +x "$PANEL_BIN"
log "panel binary installed"

if [ ! -f "$PANEL_CONFIG" ]; then
  cp "${SCRIPT_DIR}/deploy/panel.json.example" "$PANEL_CONFIG"
  sed -i "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG" 2>/dev/null || \
    sed -i '' "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG"
  chmod 600 "$PANEL_CONFIG"
  log "created $PANEL_CONFIG"
else
  log "keeping existing $PANEL_CONFIG"
fi

cp "${SCRIPT_DIR}/scripts/startup_xray_guest.sh" "${INSTALL_DIR}/startup_xray_guest.sh"
cp "${SCRIPT_DIR}/scripts/xray-guest-iptables.sh" "${INSTALL_DIR}/xray-guest-iptables.sh"
cp "${SCRIPT_DIR}/scripts/xray-guest-sysctl.sh" "${INSTALL_DIR}/xray-guest-sysctl.sh"
cp "${SCRIPT_DIR}/scripts/boot-xiaomi-vless.sh" "${INSTALL_DIR}/boot-xiaomi-vless.sh"
cp "${SCRIPT_DIR}/deploy/hotplug-usb-xiaomi-vless.sh" "${INSTALL_DIR}/hotplug-usb-xiaomi-vless.sh"
chmod +x "${INSTALL_DIR}/startup_xray_guest.sh" "${INSTALL_DIR}/xray-guest-iptables.sh" \
  "${INSTALL_DIR}/xray-guest-sysctl.sh" "${INSTALL_DIR}/boot-xiaomi-vless.sh" \
  "${INSTALL_DIR}/hotplug-usb-xiaomi-vless.sh"
log "guest VPN and boot scripts installed"

cp "${SCRIPT_DIR}/deploy/panel-updater.sh" "${INSTALL_DIR}/panel-updater.sh"
chmod +x "${INSTALL_DIR}/panel-updater.sh"

INSTALL_DIR="$INSTALL_DIR" USB_MOUNT="$USB_MOUNT" \
  INIT_SRC="${SCRIPT_DIR}/deploy/xiaomi-vless-xray.init" \
  BOOT_SRC="${SCRIPT_DIR}/scripts/boot-xiaomi-vless.sh" \
  sh "${SCRIPT_DIR}/deploy/install-autostart.sh"

install_panel_init "${SCRIPT_DIR}/deploy/xiaomi-vless-panel.init"

start_panel "$PANEL_BIN" "$PANEL_CONFIG"
if wait_for_panel "$PANEL_BIN"; then
  log "panel is responding on :7777"
else
  log "WARN: panel may still be starting — check: tail -f ${INSTALL_DIR}/panel.log"
fi

start_xray "$INSTALL_DIR"
print_install_done "$INSTALL_DIR" "$PANEL_BIN"
