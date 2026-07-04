#!/bin/sh
# Show lab VM status: services, panel, iptables counters.
set -eu

VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"

if ! multipass info "$VM_NAME" >/dev/null 2>&1; then
  echo "VM $VM_NAME not found — run ./lab/scripts/lab-up.sh"
  exit 1
fi

IP="$(multipass info "$VM_NAME" | awk -F': ' '/^IPv4:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}')"
STATE="$(multipass info "$VM_NAME" | awk -F': ' '/^State:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}')"

echo "VM:    $VM_NAME ($STATE)"
echo "IP:    ${IP:-unknown}"
echo "Panel: http://${IP}:7777"
echo

multipass exec "$VM_NAME" -- sudo sh -s <<'REMOTE'
set -eu
echo "--- systemd ---"
systemctl is-active xiaomi-vless-network.service xiaomi-vless-panel.service 2>/dev/null || true
echo
echo "--- processes ---"
pidof panel xray 2>/dev/null || echo "panel/xray not running"
echo
echo "--- bridges ---"
ip -br addr show br-lan br-guest 2>/dev/null || true
echo
echo "--- iptables (guest TCP) ---"
iptables -t nat -L XRAY_GUEST_TCP -v -n 2>/dev/null || echo "XRAY_GUEST_TCP chain not applied yet (complete onboarding + Apply)"
echo
echo "--- SOCKS probe ---"
curl -4 -sS --connect-timeout 5 -x socks5h://127.0.0.1:10808 https://ifconfig.me 2>/dev/null \
  && echo || echo "SOCKS not ready (start Xray via panel Apply)"
REMOTE
