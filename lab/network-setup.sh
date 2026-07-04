#!/bin/sh
# Lab router network: WAN (eth0/ens*) + br-lan + br-guest.
# Idempotent — safe to run on every boot.

set -eu

log() { echo "[lab-network] $*"; }

wan_iface() {
  ip -4 route show default 2>/dev/null | awk '{print $5; exit}'
}

WAN="$(wan_iface)"
[ -n "$WAN" ] || WAN="eth0"

sysctl -w net.ipv4.ip_forward=1 >/dev/null
sysctl -w net.ipv4.conf.all.rp_filter=0 >/dev/null
sysctl -w net.ipv4.conf.default.rp_filter=0 >/dev/null

for br in br-lan br-guest; do
  ip link show "$br" >/dev/null 2>&1 || ip link add "$br" type bridge
  ip link set "$br" up
done

ip addr show br-lan | grep -q '192.168.31.1/' || ip addr add 192.168.31.1/24 dev br-lan
ip addr show br-guest | grep -q '192.168.33.1/' || ip addr add 192.168.33.1/24 dev br-guest

# sysctl script references these interfaces on the real router.
sysctl -w "net.ipv4.conf.br-guest.rp_filter=0" >/dev/null 2>&1 || true
sysctl -w "net.ipv4.conf.br-lan.rp_filter=0" >/dev/null 2>&1 || true
sysctl -w "net.ipv4.conf.${WAN}.rp_filter=0" >/dev/null 2>&1 || true

iptables -C POSTROUTING -t nat -s 192.168.31.0/24 -o "$WAN" -j MASQUERADE 2>/dev/null || \
  iptables -t nat -A POSTROUTING -s 192.168.31.0/24 -o "$WAN" -j MASQUERADE

iptables -C POSTROUTING -t nat -s 192.168.33.0/24 -o "$WAN" -j MASQUERADE 2>/dev/null || \
  iptables -t nat -A POSTROUTING -s 192.168.33.0/24 -o "$WAN" -j MASQUERADE

log "WAN=$WAN br-lan=192.168.31.1/24 br-guest=192.168.33.1/24"
