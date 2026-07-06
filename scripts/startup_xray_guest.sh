#!/bin/sh
# Start Xray guest VPN after router boot.
# All paths live on USB next to this script (xiaomi-vless/ on the flash drive).

set -u

BASE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
ENV_FILE="${BASE}/xray.env"
LOG="${BASE}/xray-startup.log"
IPTABLES="${BASE}/xray-guest-iptables.sh"
SYSCTL="${BASE}/xray-guest-sysctl.sh"
XRAY=""
CONFIG=""

if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  . "$ENV_FILE"
fi

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"
}

wait_for_usb() {
  i=0
  while [ "$i" -lt 60 ]; do
    if [ -n "$XRAY" ] && [ -x "$XRAY" ] && [ -n "$CONFIG" ] && [ -f "$CONFIG" ]; then
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  return 1
}

wait_for_network() {
  i=0
  while [ "$i" -lt 30 ]; do
    if ping -c 1 -W 2 192.168.31.1 >/dev/null 2>&1 || ping -c 1 -W 2 8.8.8.8 >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  return 0
}

apply_iptables_with_retry() {
  i=0
  while [ "$i" -lt 5 ]; do
    if [ -x "$IPTABLES" ]; then
      if sh "$IPTABLES" >> "$LOG" 2>&1; then
        return 0
      fi
    fi
    i=$((i + 1))
    sleep 3
  done
  return 1
}

fail_open() {
  MARKER="/data/xiaomi-vless-failopen"
  DOWN="${BASE}/xray-guest-iptables-down.sh"
  log "fail-open: removing guest redirect rules"
  if [ -x "$DOWN" ]; then
    sh "$DOWN" >> "$LOG" 2>&1 || true
  else
    iptables -t nat -D PREROUTING -j XRAY_GUEST_TCP 2>/dev/null || true
    iptables -t nat -F XRAY_GUEST_TCP 2>/dev/null || true
    iptables -t nat -X XRAY_GUEST_TCP 2>/dev/null || true
    iptables -t mangle -D PREROUTING -j XRAY_GUEST_UDP 2>/dev/null || true
    iptables -t mangle -F XRAY_GUEST_UDP 2>/dev/null || true
    iptables -t mangle -X XRAY_GUEST_UDP 2>/dev/null || true
    ip rule del fwmark 0x1 table 100 2>/dev/null || true
    ip route flush table 100 2>/dev/null || true
  fi
  touch "$MARKER" 2>/dev/null || true
  log "fail-open: guest traffic on direct path"
}

LOCK_FILE="${BASE}/.xray-startup.lock"

acquire_lock() {
  if command -v flock >/dev/null 2>&1; then
    exec 9>"$LOCK_FILE"
    flock -n 9 || return 1
    return 0
  fi
  if ! mkdir "${LOCK_FILE}.d" 2>/dev/null; then
    return 1
  fi
  trap 'rmdir "${LOCK_FILE}.d" 2>/dev/null || true' EXIT INT HUP
  return 0
}

mkdir -p "$BASE" 2>/dev/null || true

log "=== startup_xray_guest.sh begin ==="

if ! acquire_lock; then
  log "another startup_xray_guest.sh is running — skip"
  exit 0
fi

if ! wait_for_usb; then
  log "ERROR: xray binary or config not found (${XRAY} / ${CONFIG})"
  exit 1
fi

wait_for_network
log "USB and network ready"

if [ -x "$SYSCTL" ]; then
  sh "$SYSCTL" >> "$LOG" 2>&1 || true
fi

killall xray 2>/dev/null || true
sleep 1

if [ -n "${XRAY_LOCATION_ASSET:-}" ]; then
  export XRAY_LOCATION_ASSET
fi

"$XRAY" run -c "$CONFIG" >> "$LOG" 2>&1 &
sleep 3

if ! pidof xray >/dev/null 2>&1; then
  log "ERROR: xray process not running after start"
  fail_open
  exit 1
fi

log "xray started pid $(pidof xray)"

sleep 8

if apply_iptables_with_retry; then
  log "iptables applied"
  rm -f /data/xiaomi-vless-failopen 2>/dev/null || true
else
  log "WARN: iptables apply failed, cron will retry"
fi

log "=== startup_xray_guest.sh done ==="
