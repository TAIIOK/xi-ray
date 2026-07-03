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

mkdir -p "$BASE" 2>/dev/null || true

log "=== startup_xray_guest.sh begin ==="

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

if ! "$XRAY" run -c "$CONFIG" >> "$LOG" 2>&1 & then
  log "ERROR: failed to launch xray"
  exit 1
fi

sleep 3

if ! pidof xray >/dev/null 2>&1; then
  log "ERROR: xray process not running after start"
  exit 1
fi

log "xray started pid $(pidof xray)"

sleep 8

if apply_iptables_with_retry; then
  log "iptables applied"
else
  log "WARN: iptables apply failed, cron will retry"
fi

log "=== startup_xray_guest.sh done ==="
