#!/bin/sh
# Reset panel auth / onboarding / broken nodes. Keeps paths on USB.
set -eu

USB_MOUNT=""
for d in /mnt/usb-*; do
  if [ -d "$d" ]; then
    USB_MOUNT="$d"
    break
  fi
done

INSTALL_DIR="${USB_MOUNT:-/mnt/usb-ed49605f}/xiaomi-vless"
PANEL="${PANEL_BIN:-${INSTALL_DIR}/panel}"
CONFIG="${PANEL_CONFIG:-${INSTALL_DIR}/panel.json}"

if [ ! -x "$PANEL" ]; then
  echo "Panel binary not found: $PANEL" >&2
  exit 1
fi
if [ ! -f "$CONFIG" ]; then
  echo "Config not found: $CONFIG" >&2
  exit 1
fi

BACKUP="${CONFIG}.bak.$(date +%Y%m%d-%H%M%S)"
cp "$CONFIG" "$BACKUP"
echo "Backup: $BACKUP"

killall panel 2>/dev/null || true
sleep 1

"$PANEL" -config "$CONFIG" -reset "${1:-onboarding}"

nohup "$PANEL" -config "$CONFIG" >/dev/null 2>&1 &
sleep 1

echo ""
echo "Reset mode: ${1:-onboarding}"
echo "Login:    admin / admin"
echo "Open:     http://192.168.31.1:7777/onboarding"
