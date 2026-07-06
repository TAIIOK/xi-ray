#!/bin/sh
# OpenWrt lab network: LAN (eth0) + br-guest + WAN NAT.
# Idempotent — safe to run on every boot.
set -eu

log() { echo "[qemu-network] $*"; }

WAN="$(ip -4 route show default 2>/dev/null | awk '{print $5; exit}')"
[ -n "$WAN" ] || WAN="eth1"
LAN="$(uci -q get network.lan.device 2>/dev/null || echo br-lan)"
[ -n "$LAN" ] || LAN="br-lan"

sysctl -w net.ipv4.ip_forward=1 >/dev/null
sysctl -w net.ipv4.conf.all.rp_filter=0 >/dev/null
sysctl -w net.ipv4.conf.default.rp_filter=0 >/dev/null

# Lab uses raw iptables chains like on BE7000; stop fw4 to avoid clashes.
if [ -x /etc/init.d/firewall ]; then
  /etc/init.d/firewall stop 2>/dev/null || true
fi

ip link show br-guest >/dev/null 2>&1 || ip link add br-guest type bridge
ip link set br-guest up
ip addr show br-guest | grep -q '192.168.33.1/' || ip addr add 192.168.33.1/24 dev br-guest

# QEMU lab: br-guest has no Wi‑Fi port; add a veth so the bridge is operational (UP).
if ! ip -br link show br-guest 2>/dev/null | grep -qE '(^| )UP( |$)'; then
  ip link add veth-guest-lab type veth peer name veth-guest-lab-br 2>/dev/null || true
  ip link set veth-guest-lab-br master br-guest 2>/dev/null || true
  ip link set veth-guest-lab up 2>/dev/null || true
  ip link set veth-guest-lab-br up 2>/dev/null || true
fi

# Keep OpenWrt LAN on 192.168.1.1; optional BE7000-like LAN subnet.
if [ "${QEMU_LAN_SUBNET:-}" = "192.168.31.0/24" ]; then
  uci -q set network.lan.ipaddr='192.168.31.1' 2>/dev/null || true
  uci -q set network.lan.netmask='255.255.255.0' 2>/dev/null || true
  uci commit network 2>/dev/null || true
  /etc/init.d/network reload 2>/dev/null || true
fi

for iface in br-guest "$LAN" "$WAN"; do
  sysctl -w "net.ipv4.conf.${iface}.rp_filter=0" >/dev/null 2>&1 || true
done

if command -v iptables >/dev/null 2>&1 && iptables -V >/dev/null 2>&1; then
  iptables -C POSTROUTING -t nat -s 192.168.1.0/24 -o "$WAN" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s 192.168.1.0/24 -o "$WAN" -j MASQUERADE
  iptables -C POSTROUTING -t nat -s 192.168.31.0/24 -o "$WAN" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s 192.168.31.0/24 -o "$WAN" -j MASQUERADE
  iptables -C POSTROUTING -t nat -s 192.168.33.0/24 -o "$WAN" -j MASQUERADE 2>/dev/null || \
    iptables -t nat -A POSTROUTING -s 192.168.33.0/24 -o "$WAN" -j MASQUERADE
else
  log "WARN: iptables unavailable — skipping NAT rules"
fi

log "LAN=$LAN WAN=$WAN br-guest=192.168.33.1/24"
