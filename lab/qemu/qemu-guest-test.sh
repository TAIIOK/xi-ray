#!/bin/sh
# Guest namespace connectivity test inside QEMU OpenWrt lab.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

qemu_is_running || qemu_die "QEMU not running — run: make qemu-up"
qemu_wait_for_ssh

qemu_ssh "env LAB_FAILOPEN_TEST=${LAB_FAILOPEN_TEST:-0} sh -s" <<'REMOTE'
set -eu
INSTALL="/mnt/usb-lab/xiaomi-vless"
NETNS="guest-test"

sh "${INSTALL}/guest-netns.sh"

echo
echo "--- guest ping gateway ---"
ip netns exec "$NETNS" ping -c 2 -W 2 192.168.33.1

echo
echo "--- guest DNS ---"
ip netns exec "$NETNS" curl -4 -sS --connect-timeout 8 https://ifconfig.me || \
  echo "curl failed (VPN/iptables may not be configured yet)"

echo
echo "--- iptables counters ---"
iptables -t nat -L XRAY_GUEST_TCP -v -n 2>/dev/null | head -5 || true

if [ "${LAB_FAILOPEN_TEST:-0}" = "1" ]; then
  echo
  echo "--- fail-open test ---"
  killall xray 2>/dev/null || true
  sleep 1
  sh "${INSTALL}/xiaomi-vless-failopen-guard.sh" || true
  if iptables -t nat -C PREROUTING -j XRAY_GUEST_TCP 2>/dev/null; then
    echo "FAIL: guest redirect still active after fail-open"
    exit 1
  fi
  if [ ! -f /data/xiaomi-vless-failopen ]; then
    echo "FAIL: fail-open marker missing"
    exit 1
  fi
  echo "OK: fail-open removed guest redirect"
fi
REMOTE

echo
echo "Manual guest shell:"
echo "  make qemu-shell"
echo "  ip netns exec guest-test sh"
