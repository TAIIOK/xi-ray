#!/bin/sh
# Register guest VPN autostart on Xiaomi BE7000 / OpenWrt-like systems.

set -eu

INSTALL_DIR="${INSTALL_DIR:-${USB_MOUNT:-/mnt/usb-ed49605f}/xiaomi-vless}"
STARTUP_SCRIPT="${INSTALL_DIR}/startup_xray_guest.sh"
IPTABLES_SCRIPT="${INSTALL_DIR}/xray-guest-iptables.sh"
SYSCTL_SCRIPT="${INSTALL_DIR}/xray-guest-sysctl.sh"
USER_STARTUP="/data/startup_user.sh"
CRON_FILE="/etc/crontabs/root"
ENV_FILE="${INSTALL_DIR}/xray.env"

MARKER="# xiaomi-vless-guest-vpn"

log() { echo "[autostart] $*"; }

ensure_executable() {
  for f in "$STARTUP_SCRIPT" "$IPTABLES_SCRIPT" "$SYSCTL_SCRIPT"; do
    if [ -f "$f" ]; then
      chmod +x "$f"
    fi
  done
}

append_once() {
  file="$1"
  line="$2"
  marker="$3"
  if [ ! -f "$file" ]; then
    touch "$file"
  fi
  if grep -q "$marker" "$file" 2>/dev/null; then
    log "already in $file: $marker"
    return 0
  fi
  echo "$marker" >> "$file"
  echo "$line" >> "$file"
  chmod +x "$file" 2>/dev/null || true
  log "added to $file"
}

install_user_startup() {
  if [ ! -f "$USER_STARTUP" ]; then
    printf '%s\n' '#!/bin/sh' > "$USER_STARTUP"
    chmod +x "$USER_STARTUP"
  fi
  append_once "$USER_STARTUP" "sleep 20 && ${STARTUP_SCRIPT}" "$MARKER"
}

install_uci_firewall() {
  if ! command -v uci >/dev/null 2>&1; then
    return 0
  fi
  if uci get firewall.startup_xray_guest >/dev/null 2>&1; then
    uci set firewall.startup_xray_guest.path="$STARTUP_SCRIPT"
    uci commit firewall
    log "uci firewall.startup_xray_guest path updated"
    return 0
  fi
  uci set firewall.startup_xray_guest=include
  uci set firewall.startup_xray_guest.type='script'
  uci set firewall.startup_xray_guest.path="$STARTUP_SCRIPT"
  uci set firewall.startup_xray_guest.enabled='1'
  uci commit firewall
  log "uci firewall include added"
}

install_procd_init() {
  init_src="$1"
  if [ ! -f "$init_src" ]; then
    return 0
  fi
  if [ ! -d /etc/init.d ]; then
    return 0
  fi
  cp "$init_src" /etc/init.d/xiaomi-vless-xray
  chmod +x /etc/init.d/xiaomi-vless-xray
  if [ -x /etc/rc.common ]; then
    /etc/init.d/xiaomi-vless-xray enable 2>/dev/null || true
    log "procd init installed: /etc/init.d/xiaomi-vless-xray"
  fi
}

install_cron() {
  mkdir -p "$(dirname "$CRON_FILE")" 2>/dev/null || true
  if [ ! -f "$CRON_FILE" ]; then
    touch "$CRON_FILE"
  fi

  if ! grep -q "$MARKER" "$CRON_FILE" 2>/dev/null; then
    {
      echo "$MARKER"
      echo "@reboot sleep 45 && ${STARTUP_SCRIPT} >/dev/null 2>&1"
      echo "*/2 * * * * ${IPTABLES_SCRIPT} >/dev/null 2>&1"
    } >> "$CRON_FILE"
    log "cron entries added"
  else
    log "cron already configured"
  fi

  if [ -x /etc/init.d/cron ]; then
    /etc/init.d/cron enable 2>/dev/null || true
    /etc/init.d/cron restart 2>/dev/null || true
  elif [ -x /etc/init.d/crond ]; then
    /etc/init.d/crond enable 2>/dev/null || true
    /etc/init.d/crond restart 2>/dev/null || true
  fi
}

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
INIT_SRC="${INIT_SRC:-${SCRIPT_DIR}/xiaomi-vless-xray.init}"

ensure_executable
install_user_startup
install_uci_firewall
install_procd_init "$INIT_SRC"
install_cron

log "Guest VPN autostart configured (USB: ${INSTALL_DIR})"
log "Test now: ${STARTUP_SCRIPT}"
log "After reboot check: tail -f ${INSTALL_DIR}/xray-startup.log"
