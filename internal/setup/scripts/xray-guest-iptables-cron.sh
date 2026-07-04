#!/bin/sh
# Cron wrapper: skip iptables refresh while fail-open is active or xray is down.
# Lives on USB next to xray-guest-iptables.sh.

set -u

MARKER="${FAILOPEN_MARKER:-/data/xiaomi-vless-failopen}"
BASE="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
IPTABLES="${BASE}/xray-guest-iptables.sh"

if [ -f "$MARKER" ]; then
  exit 0
fi

if ! pidof xray >/dev/null 2>&1; then
  exit 0
fi

if [ ! -x "$IPTABLES" ]; then
  exit 0
fi

exec sh "$IPTABLES"
