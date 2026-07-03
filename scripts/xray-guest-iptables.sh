#!/bin/sh
# Template: panel overwrites /data/xray-guest-iptables.sh on Apply.
# Apply guest-network VPN iptables rules for Xray transparent proxy.

GUEST_SUBNET="192.168.33.0/24"
TCP_PORT=12346
UDP_PORT=12345
MARK=0x1
TABLE=100
VPN_HOST=""

get_vpn_ip() {
    IP=$(nslookup "$VPN_HOST" 2>/dev/null | awk '/^Address [0-9]*:/{print $NF}' | tail -1)
    echo "$IP" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}$' && echo "$IP" && return
    IP=$(ping -c 1 -W 3 "$VPN_HOST" 2>/dev/null | sed -n 's/.*(\([0-9.]*\)).*/\1/p')
    echo "$IP" | grep -qE '^([0-9]{1,3}\.){3}[0-9]{1,3}$' && echo "$IP" && return
    echo ""
}

VPN_IP=$(get_vpn_ip)

sysctl -w net.ipv4.ip_forward=1 >/dev/null
sysctl -w net.ipv4.conf.all.rp_filter=0 >/dev/null
sysctl -w net.ipv4.conf.br-guest.rp_filter=0 >/dev/null
sysctl -w net.ipv4.conf.eth0.rp_filter=0 >/dev/null

ip rule del fwmark $MARK table $TABLE 2>/dev/null
ip rule add fwmark $MARK table $TABLE
ip route flush table $TABLE 2>/dev/null
ip route add local 0.0.0.0/0 dev lo table $TABLE

iptables -t nat -N XRAY_GUEST_TCP 2>/dev/null
iptables -t nat -F XRAY_GUEST_TCP

iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 0.0.0.0/8       -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 10.0.0.0/8      -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 127.0.0.0/8     -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 169.254.0.0/16  -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 172.16.0.0/12   -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 192.168.0.0/16  -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 224.0.0.0/4     -j RETURN
iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d 240.0.0.0/4     -j RETURN
[ -n "$VPN_IP" ] && iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -d $VPN_IP -j RETURN

iptables -t nat -A XRAY_GUEST_TCP -s $GUEST_SUBNET -p tcp -j REDIRECT --to-ports $TCP_PORT

iptables -t nat -D PREROUTING -j XRAY_GUEST_TCP 2>/dev/null
iptables -t nat -A PREROUTING -j XRAY_GUEST_TCP

iptables -t mangle -N XRAY_GUEST_UDP 2>/dev/null
iptables -t mangle -F XRAY_GUEST_UDP

iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 0.0.0.0/8       -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 10.0.0.0/8      -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 127.0.0.0/8     -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 169.254.0.0/16  -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 172.16.0.0/12   -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 192.168.0.0/16  -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 224.0.0.0/4     -j RETURN
iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d 240.0.0.0/4     -j RETURN
[ -n "$VPN_IP" ] && iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -d $VPN_IP -j RETURN

iptables -t mangle -A XRAY_GUEST_UDP -s $GUEST_SUBNET -p udp -j TPROXY --tproxy-mark $MARK --on-port $UDP_PORT

iptables -t mangle -D PREROUTING -j XRAY_GUEST_UDP 2>/dev/null
iptables -t mangle -A PREROUTING -j XRAY_GUEST_UDP

iptables -t nat -F XRAY_DNS 2>/dev/null
iptables -t nat -D PREROUTING -j XRAY_DNS 2>/dev/null

echo "Rules applied. VPN IP: ${VPN_IP:-NOT RESOLVED}"
