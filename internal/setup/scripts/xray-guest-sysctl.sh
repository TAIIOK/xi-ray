#!/bin/sh
# Persistent sysctl for guest TProxy on Xiaomi BE7000.

CONF="/etc/sysctl.d/99-xray-guest.conf"

mkdir -p /etc/sysctl.d 2>/dev/null || true

cat > "$CONF" << 'EOF'
net.ipv4.ip_forward=1
net.ipv4.conf.all.rp_filter=0
net.ipv4.conf.default.rp_filter=0
net.ipv4.conf.br-guest.rp_filter=0
net.ipv4.conf.br-lan.rp_filter=0
net.ipv4.conf.eth0.rp_filter=0
EOF

if command -v sysctl >/dev/null 2>&1; then
  sysctl -p "$CONF" 2>/dev/null || sysctl -w net.ipv4.ip_forward=1 net.ipv4.conf.all.rp_filter=0
fi
