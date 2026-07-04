#!/bin/sh
# Create an isolated "guest device" in network namespace (192.168.33.10).
set -eu

NETNS="${LAB_GUEST_NETNS:-guest-test}"
GUEST_IP="${LAB_GUEST_IP:-192.168.33.10}"
BRIDGE="${LAB_GUEST_BRIDGE:-br-guest}"

log() { echo "[lab-guest] $*"; }

[ "$(id -u)" -eq 0 ] || { echo "run as root" >&2; exit 1; }

ip link show "$BRIDGE" >/dev/null 2>&1 || {
  echo "bridge $BRIDGE not found — run network-setup.sh first" >&2
  exit 1
}

ip netns delete "$NETNS" 2>/dev/null || true
ip link delete veth-guest-br 2>/dev/null || true
ip netns add "$NETNS"
ip link add veth-guest type veth peer name veth-guest-br

ip link set veth-guest netns "$NETNS"
ip link set veth-guest-br master "$BRIDGE"
ip link set veth-guest-br up

ip netns exec "$NETNS" ip addr flush dev veth-guest 2>/dev/null || true
ip netns exec "$NETNS" ip addr add "${GUEST_IP}/24" dev veth-guest
ip netns exec "$NETNS" ip link set veth-guest up
ip netns exec "$NETNS" ip link set lo up
ip netns exec "$NETNS" ip route replace default via 192.168.33.1

mkdir -p "/etc/netns/${NETNS}"
printf 'nameserver 8.8.8.8\nnameserver 1.1.1.1\n' > "/etc/netns/${NETNS}/resolv.conf"

log "namespace $NETNS ready at $GUEST_IP (bridge $BRIDGE)"
log "examples:"
log "  ip netns exec $NETNS curl -4 -s https://ifconfig.me"
log "  ip netns exec $NETNS ping -c 3 8.8.8.8"
