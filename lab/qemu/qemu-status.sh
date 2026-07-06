#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

if qemu_is_running; then
  printf 'QEMU: running'
  if [ -f "$QEMU_PIDFILE" ]; then
    printf ' (pid %s)' "$(cat "$QEMU_PIDFILE")"
  fi
  printf '\n'
else
  printf 'QEMU: stopped\n'
  exit 1
fi

printf 'Panel: http://127.0.0.1:%s\n' "$QEMU_PANEL_PORT"
printf 'SSH:   ssh -p %s root@127.0.0.1\n' "$QEMU_SSH_PORT"

if command -v curl >/dev/null 2>&1; then
  if curl -fsS --connect-timeout 2 "http://127.0.0.1:${QEMU_PANEL_PORT}/login" >/dev/null 2>&1; then
    printf 'HTTP:  panel reachable\n'
  else
    printf 'HTTP:  panel not responding yet\n'
  fi
fi

qemu_ssh 'sh -s' <<'REMOTE'
set -eu
INSTALL="/mnt/usb-lab/xiaomi-vless"

echo
echo "--- procd services ---"
for svc in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot cron; do
  if [ -x "/etc/init.d/$svc" ]; then
    printf '%s: ' "$svc"
    /etc/init.d/"$svc" status 2>/dev/null || echo unknown
  fi
done

echo
echo "--- panel version ---"
[ -x "$INSTALL/panel" ] && "$INSTALL/panel" -version 2>/dev/null | head -1 || echo missing

echo
echo "--- guest iptables ---"
iptables -t nat -L XRAY_GUEST_TCP -v -n 2>/dev/null | head -5 || echo not configured yet

echo
echo "--- USB mount ---"
df -h /mnt/usb-lab 2>/dev/null || echo not mounted
REMOTE
