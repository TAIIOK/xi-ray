#!/bin/sh
# Fully remove xiaomi-vless from router flash and USB.
# Safe by default: dry-run without --yes.
set -eu

MARKER_BOOT="# xiaomi-vless-boot"
MARKER_GUEST="# xiaomi-vless-guest-vpn"
MARKER_PANEL="# xiaomi-vless-panel"
MARKER_UPDATE="# xiaomi-vless-update-resume"

USER_STARTUP="/data/startup_user.sh"
CRON_FILE="/etc/crontabs/root"
BOOT_SCRIPT="/data/xiaomi-vless-boot.sh"
SYSCTL_CONF="/etc/sysctl.d/99-xray-guest.conf"

DO_IT=0
ROUTER_ONLY=0
KEEP_XRAY=0

log() { printf '[uninstall] %s\n' "$*"; }
warn() { printf '[uninstall] WARN: %s\n' "$*" >&2; }
action() {
  if [ "$DO_IT" -eq 1 ]; then
    log "DO: $*"
    "$@"
  else
    log "WOULD: $*"
  fi
}

usage() {
  cat <<'EOF'
Usage: uninstall.sh [options]

Remove xiaomi-vless panel, guest VPN hooks, iptables rules, and USB data.

Options:
  --yes           Perform removal (default is dry-run)
  --router-only   Clean router only; keep USB files
  --keep-xray     Remove xiaomi-vless/ on USB but keep xray/
  --purge-usb     Explicitly remove xiaomi-vless/ and xray/ on USB (default with --yes)
  -h, --help      Show this help

Examples:
  sh uninstall.sh                  # preview
  sh uninstall.sh --yes            # full cleanup
  sh uninstall.sh --yes --router-only
  sh uninstall.sh --yes --keep-xray
EOF
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --yes) DO_IT=1 ;;
      --router-only) ROUTER_ONLY=1 ;;
      --keep-xray) KEEP_XRAY=1 ;;
      --purge-usb) KEEP_XRAY=0 ;;
      -h|--help) usage; exit 0 ;;
      *) die "unknown option: $1" ;;
    esac
    shift
  done
}

die() {
  printf '[uninstall] ERROR: %s\n' "$*" >&2
  exit 1
}

find_usb_mount() {
  for d in /mnt/usb-*; do
    if [ -d "$d/xiaomi-vless" ]; then
      echo "$d"
      return 0
    fi
  done
  for d in /mnt/usb-*; do
    if [ -d "$d" ] && [ -w "$d" ]; then
      echo "$d"
      return 0
    fi
  done
  return 1
}

# Remove marker line and the next N lines from a file (in-place when DO_IT=1).
remove_marker_block() {
  file="$1"
  marker="$2"
  lines_after="$3"

  [ -f "$file" ] || return 0
  if ! grep -qF "$marker" "$file" 2>/dev/null; then
    return 0
  fi

  if [ "$DO_IT" -eq 0 ]; then
    log "WOULD: remove marker block from $file: $marker (+${lines_after} lines)"
    return 0
  fi

  tmp="${file}.xiaomi-vless-uninstall.$$"
  awk -v marker="$marker" -v n="$lines_after" '
    BEGIN { skip = 0 }
    index($0, marker) {
      skip = n + 1
      next
    }
    skip > 0 {
      skip--
      next
    }
    { print }
  ' "$file" > "$tmp" && mv "$tmp" "$file"
  log "removed marker block from $file: $marker"
}

stop_services() {
  for svc in xiaomi-vless-panel xiaomi-vless-xray; do
    if [ -x "/etc/init.d/$svc" ]; then
      action /etc/init.d/"$svc" stop 2>/dev/null || true
      if [ "$DO_IT" -eq 1 ]; then
        /etc/init.d/"$svc" disable 2>/dev/null || true
      else
        log "WOULD: /etc/init.d/$svc disable"
      fi
    fi
  done

  if pidof panel >/dev/null 2>&1; then
    if [ "$DO_IT" -eq 1 ]; then
      killall panel 2>/dev/null || true
      log "stopped panel process"
    else
      log "WOULD: killall panel"
    fi
  fi

  if pidof xray >/dev/null 2>&1; then
    if [ "$DO_IT" -eq 1 ]; then
      killall xray 2>/dev/null || true
      log "stopped xray process"
    else
      log "WOULD: killall xray"
    fi
  fi

  if [ "$DO_IT" -eq 1 ]; then
    sleep 1
  fi
}

cleanup_iptables() {
  if ! command -v iptables >/dev/null 2>&1; then
    warn "iptables not found — skipping rule cleanup"
    return 0
  fi

  if [ "$DO_IT" -eq 0 ]; then
    log "WOULD: remove iptables chains XRAY_GUEST_TCP, XRAY_GUEST_UDP, XRAY_DNS"
    log "WOULD: remove ip rule fwmark 0x1 table 100 and flush route table 100"
    return 0
  fi

  iptables -t nat -D PREROUTING -j XRAY_GUEST_TCP 2>/dev/null || true
  iptables -t nat -F XRAY_GUEST_TCP 2>/dev/null || true
  iptables -t nat -X XRAY_GUEST_TCP 2>/dev/null || true

  iptables -t mangle -D PREROUTING -j XRAY_GUEST_UDP 2>/dev/null || true
  iptables -t mangle -F XRAY_GUEST_UDP 2>/dev/null || true
  iptables -t mangle -X XRAY_GUEST_UDP 2>/dev/null || true

  iptables -t nat -D PREROUTING -j XRAY_DNS 2>/dev/null || true
  iptables -t nat -F XRAY_DNS 2>/dev/null || true
  iptables -t nat -X XRAY_DNS 2>/dev/null || true

  if command -v ip >/dev/null 2>&1; then
    ip rule del fwmark 0x1 table 100 2>/dev/null || true
    ip route flush table 100 2>/dev/null || true
  fi

  log "iptables and ip rules removed"
}

cleanup_startup_user() {
  remove_marker_block "$USER_STARTUP" "$MARKER_BOOT" 1
  remove_marker_block "$USER_STARTUP" "$MARKER_GUEST" 1
  remove_marker_block "$USER_STARTUP" "$MARKER_PANEL" 1
  remove_marker_block "$USER_STARTUP" "$MARKER_UPDATE" 1
}

cleanup_cron() {
  remove_marker_block "$CRON_FILE" "$MARKER_BOOT" 2
  remove_marker_block "$CRON_FILE" "$MARKER_GUEST" 2
}

cleanup_uci_firewall() {
  if ! command -v uci >/dev/null 2>&1; then
    return 0
  fi
  if ! uci get firewall.startup_xray_guest >/dev/null 2>&1; then
    return 0
  fi

  if [ "$DO_IT" -eq 0 ]; then
    log "WOULD: uci delete firewall.startup_xray_guest && uci commit firewall"
    return 0
  fi

  uci delete firewall.startup_xray_guest
  uci commit firewall
  if [ -x /etc/init.d/firewall ]; then
    /etc/init.d/firewall reload 2>/dev/null || true
  fi
  log "removed uci firewall.startup_xray_guest"
}

cleanup_hotplug() {
  path="/etc/hotplug.d/block/99-xiaomi-vless"
  if [ ! -f "$path" ]; then
    return 0
  fi
  if [ "$DO_IT" -eq 0 ]; then
    log "WOULD: rm -f $path"
    return 0
  fi
  rm -f "$path"
  log "removed $path"
}

cleanup_initd() {
  for svc in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot; do
    path="/etc/init.d/$svc"
    if [ ! -f "$path" ]; then
      continue
    fi
    if [ "$DO_IT" -eq 0 ]; then
      log "WOULD: rm -f $path"
    else
      rm -f "$path"
      log "removed $path"
    fi
  done
}

cleanup_sysctl() {
  if [ ! -f "$SYSCTL_CONF" ]; then
    return 0
  fi
  action rm -f "$SYSCTL_CONF"
}

cleanup_legacy_data() {
  for path in /data/xiaomi-vless /data/startup_xray_guest.sh /data/xray-guest-iptables.sh "$BOOT_SCRIPT" /data/xiaomi-vless-boot.log; do
    if [ -e "$path" ]; then
      if [ "$DO_IT" -eq 0 ]; then
        log "WOULD: rm -rf $path"
      else
        rm -rf "$path"
        log "removed legacy $path"
      fi
    fi
  done
}

cleanup_temp() {
  for path in /tmp/xiaomi-vless-update.lock /tmp/xiaomi-vless-update.lock.d /tmp/xiaomi-vless-boot.lock /tmp/xiaomi-vless-boot.lock.d; do
    if [ -e "$path" ]; then
      action rm -rf "$path"
    fi
  done
}

cleanup_usb() {
  if [ "$ROUTER_ONLY" -eq 1 ]; then
    log "skipping USB cleanup (--router-only)"
    return 0
  fi

  usb_mount=""
  if usb_mount="$(find_usb_mount)"; then
    :
  else
    warn "USB mount not found — skipping USB file removal (router hooks still cleaned)"
    return 0
  fi

  install_dir="${usb_mount}/xiaomi-vless"
  xray_dir="${usb_mount}/xray"

  if [ -d "$install_dir" ]; then
    if [ "$DO_IT" -eq 0 ]; then
      log "WOULD: rm -rf $install_dir"
    else
      rm -rf "$install_dir"
      log "removed $install_dir"
    fi
  fi

  if [ "$KEEP_XRAY" -eq 1 ]; then
    log "keeping $xray_dir (--keep-xray)"
    return 0
  fi

  if [ -d "$xray_dir" ]; then
    if [ "$DO_IT" -eq 0 ]; then
      log "WOULD: rm -rf $xray_dir"
    else
      rm -rf "$xray_dir"
      log "removed $xray_dir"
    fi
  fi
}

print_summary() {
  echo ""
  if [ "$DO_IT" -eq 0 ]; then
    echo "Dry-run complete. Re-run with --yes to apply."
  else
    echo "Xiaomi VLESS removed."
    echo "  Guest Wi-Fi is no longer proxied through Xray."
    echo "  Panel http://192.168.31.1:7777 is no longer available."
    echo "  Reboot recommended to fully reset sysctl values."
  fi
  echo ""
}

# --- main ---

parse_args "$@"

if [ "$DO_IT" -eq 0 ]; then
  log "dry-run mode (pass --yes to apply)"
else
  log "removing xiaomi-vless..."
fi

stop_services
cleanup_iptables
cleanup_startup_user
cleanup_cron
cleanup_uci_firewall
cleanup_hotplug
cleanup_initd
cleanup_sysctl
cleanup_legacy_data
cleanup_temp
cleanup_usb
print_summary
