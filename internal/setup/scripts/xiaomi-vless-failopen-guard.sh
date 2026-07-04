#!/bin/sh
# Flash-resident guard: remove guest redirect when xray is down.
# Installed to /data/xiaomi-vless-failopen-guard.sh by install-autostart.sh.

set -u

MARKER="${FAILOPEN_MARKER:-/data/xiaomi-vless-failopen}"
LOG="/data/xiaomi-vless-failopen.log"

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"
}

iptables_hooked() {
  command -v iptables >/dev/null 2>&1 || return 1
  iptables -t nat -C PREROUTING -j XRAY_GUEST_TCP 2>/dev/null
}

run_teardown() {
  DOWN="$(ls /mnt/usb-*/xiaomi-vless/xray-guest-iptables-down.sh 2>/dev/null | head -1)"
  if [ -n "$DOWN" ] && [ -x "$DOWN" ]; then
    sh "$DOWN" >> "$LOG" 2>&1
    return $?
  fi

  iptables -t nat -D PREROUTING -j XRAY_GUEST_TCP 2>/dev/null || true
  iptables -t nat -F XRAY_GUEST_TCP 2>/dev/null || true
  iptables -t nat -X XRAY_GUEST_TCP 2>/dev/null || true

  iptables -t mangle -D PREROUTING -j XRAY_GUEST_UDP 2>/dev/null || true
  iptables -t mangle -F XRAY_GUEST_UDP 2>/dev/null || true
  iptables -t mangle -X XRAY_GUEST_UDP 2>/dev/null || true

  iptables -t nat -D PREROUTING -j XRAY_DNS 2>/dev/null || true
  iptables -t nat -F XRAY_DNS 2>/dev/null || true
  iptables -t nat -X XRAY_DNS 2>/dev/null || true

  if command -v ip >/dev/null 2>&1; then
    ip rule del fwmark 0x1 table 100 2>/dev/null || true
    ip route flush table 100 2>/dev/null || true
  fi
  return 0
}

if ! iptables_hooked; then
  exit 0
fi

if pidof xray >/dev/null 2>&1; then
  exit 0
fi

log "xray down with guest redirect active — enabling fail-open"
if run_teardown; then
  touch "$MARKER" 2>/dev/null || true
  log "fail-open applied"
else
  log "fail-open teardown failed"
fi
