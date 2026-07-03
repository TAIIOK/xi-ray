#!/bin/sh
# One-shot fix for broken autostart on a live router (boot with USB).
# Run on the router:
#   sh /mnt/usb-ed49605f/xiaomi-vless/scripts/boot-xiaomi-vless.sh   # test
# Or pipe from dev machine after copying boot script to USB.
#
#   ssh root@192.168.31.1 'sh -s' < deploy/fix-autostart.sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BOOT_SRC="${SCRIPT_DIR}/../scripts/boot-xiaomi-vless.sh"
[ -f "$BOOT_SRC" ] || BOOT_SRC="${SCRIPT_DIR}/scripts/boot-xiaomi-vless.sh"
[ -f "$BOOT_SRC" ] || BOOT_SRC="/mnt/usb-ed49605f/xiaomi-vless/boot-xiaomi-vless.sh"

INSTALL_DIR=""
for d in /mnt/usb-*/xiaomi-vless; do
  if [ -x "$d/panel" ]; then
    INSTALL_DIR="$d"
    break
  fi
done

[ -n "$INSTALL_DIR" ] || { echo "[fix] ERROR: xiaomi-vless not found on USB" >&2; exit 1; }
[ -f "$BOOT_SRC" ] || { echo "[fix] ERROR: boot-xiaomi-vless.sh not found" >&2; exit 1; }

export INSTALL_DIR USB_MOUNT="$(dirname "$INSTALL_DIR")" BOOT_SRC
sh "${SCRIPT_DIR}/install-autostart.sh"

echo "[fix] running boot script now..."
/data/xiaomi-vless-boot.sh
sleep 5

if pidof panel >/dev/null 2>&1; then
  echo "[fix] OK: panel pid $(pidof panel)"
else
  echo "[fix] WARN: panel not running — see /data/xiaomi-vless-boot.log"
fi

echo "[fix] boot log:"
tail -15 /data/xiaomi-vless-boot.log 2>/dev/null || true
echo "[fix] reboot to verify: reboot"
