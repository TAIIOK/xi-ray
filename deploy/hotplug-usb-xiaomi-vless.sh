#!/bin/sh
# OpenWrt hotplug: start xiaomi-vless when USB storage appears (boot with flash drive).
[ "$ACTION" = "add" ] || exit 0
[ -x /data/xiaomi-vless-boot.sh ] || exit 0

case "$DEVTYPE" in
  partition|disk) ;;
  *) exit 0 ;;
esac

# Let the kernel finish mounting /mnt/usb-* first.
(sleep 8 && /data/xiaomi-vless-boot.sh) >/dev/null 2>&1 &
