#!/bin/sh
# Create guest namespace and run basic connectivity checks inside the lab VM.
set -eu

VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"

if ! multipass info "$VM_NAME" >/dev/null 2>&1; then
  echo "VM $VM_NAME not found — run ./lab/scripts/lab-up.sh"
  exit 1
fi

echo "Setting up guest namespace and running checks..."
multipass exec "$VM_NAME" -- sudo sh -s <<'REMOTE'
set -eu
INSTALL_DIR="/mnt/usb-lab/xiaomi-vless"
NETNS="guest-test"

sh "${INSTALL_DIR}/guest-netns.sh"

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
REMOTE

echo
echo "Manual guest shell:"
echo "  multipass shell $VM_NAME"
echo "  sudo ip netns exec guest-test sh"
