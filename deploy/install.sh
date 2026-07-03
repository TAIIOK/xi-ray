#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

USB_MOUNT=""
for d in /mnt/usb-*; do
  if [ -d "$d" ]; then
    USB_MOUNT="$d"
    break
  fi
done

if [ -z "$USB_MOUNT" ]; then
  echo "USB not mounted under /mnt/usb-* — plug in USB and retry." >&2
  exit 1
fi

INSTALL_DIR="${USB_MOUNT}/xiaomi-vless"
PANEL_BIN="${INSTALL_DIR}/panel"
PANEL_CONFIG="${INSTALL_DIR}/panel.json"

mkdir -p "$INSTALL_DIR"
mkdir -p "${INSTALL_DIR}/updates/downloads" "${INSTALL_DIR}/updates/staging"
chmod 755 "$INSTALL_DIR"

if [ -f "${SCRIPT_DIR}/dist/panel-linux-arm64" ]; then
  cp "${SCRIPT_DIR}/dist/panel-linux-arm64" "$PANEL_BIN"
elif [ -f "${SCRIPT_DIR}/dist/panel" ]; then
  cp "${SCRIPT_DIR}/dist/panel" "$PANEL_BIN"
else
  echo "Build binary first: make build-arm64" >&2
  exit 1
fi
chmod +x "$PANEL_BIN"

if [ ! -f "$PANEL_CONFIG" ]; then
  cp "${SCRIPT_DIR}/deploy/panel.json.example" "$PANEL_CONFIG"
  sed -i "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG" 2>/dev/null || \
    sed -i '' "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG"
  chmod 600 "$PANEL_CONFIG"
fi

cp "${SCRIPT_DIR}/scripts/startup_xray_guest.sh" "${INSTALL_DIR}/startup_xray_guest.sh"
cp "${SCRIPT_DIR}/scripts/xray-guest-iptables.sh" "${INSTALL_DIR}/xray-guest-iptables.sh"
cp "${SCRIPT_DIR}/scripts/xray-guest-sysctl.sh" "${INSTALL_DIR}/xray-guest-sysctl.sh"
chmod +x "${INSTALL_DIR}/startup_xray_guest.sh" "${INSTALL_DIR}/xray-guest-iptables.sh" "${INSTALL_DIR}/xray-guest-sysctl.sh"

cp "${SCRIPT_DIR}/deploy/panel-updater.sh" "${INSTALL_DIR}/panel-updater.sh"
chmod +x "${INSTALL_DIR}/panel-updater.sh"

# Guest VPN autostart (startup_user.sh, uci, procd, cron)
INSTALL_DIR="$INSTALL_DIR" USB_MOUNT="$USB_MOUNT" sh "${SCRIPT_DIR}/deploy/install-autostart.sh"

# Web panel autostart — only boot hook stays on router /data
PANEL_MARKER="# xiaomi-vless-panel"
UPDATE_RESUME_MARKER="# xiaomi-vless-update-resume"
USER_STARTUP="/data/startup_user.sh"
if [ ! -f "$USER_STARTUP" ]; then
  printf '%s\n' '#!/bin/sh' > "$USER_STARTUP"
  chmod +x "$USER_STARTUP"
fi
if ! grep -q "$UPDATE_RESUME_MARKER" "$USER_STARTUP" 2>/dev/null; then
  echo "$UPDATE_RESUME_MARKER" >> "$USER_STARTUP"
  echo "[ -x ${INSTALL_DIR}/panel-updater.sh ] && ${INSTALL_DIR}/panel-updater.sh resume >/dev/null 2>&1" >> "$USER_STARTUP"
fi
if ! grep -q "$PANEL_MARKER" "$USER_STARTUP" 2>/dev/null; then
  echo "$PANEL_MARKER" >> "$USER_STARTUP"
  echo "sleep 25 && ${PANEL_BIN} -config ${PANEL_CONFIG} >/dev/null 2>&1 &" >> "$USER_STARTUP"
fi

if [ -d /etc/init.d ] && [ -f "${SCRIPT_DIR}/deploy/xiaomi-vless-panel.init" ]; then
  cp "${SCRIPT_DIR}/deploy/xiaomi-vless-panel.init" /etc/init.d/xiaomi-vless-panel
  chmod +x /etc/init.d/xiaomi-vless-panel
  /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
fi

echo "Installed panel to ${PANEL_BIN}"
echo "USB home: ${INSTALL_DIR}"
echo "Guest VPN autostart: ${INSTALL_DIR}/startup_xray_guest.sh"
echo "Log after reboot: tail -f ${INSTALL_DIR}/xray-startup.log"
echo "Open http://192.168.31.1:7777 (default admin/admin — change on first login)"
