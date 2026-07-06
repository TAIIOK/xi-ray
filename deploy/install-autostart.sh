#!/bin/sh
# Register xiaomi-vless boot autostart on Xiaomi BE7000 / OpenWrt-like systems.

set -eu

INSTALL_DIR="${INSTALL_DIR:-${USB_MOUNT:-/mnt/usb-ed49605f}/xiaomi-vless}"
IPTABLES_SCRIPT="${INSTALL_DIR}/xray-guest-iptables.sh"
IPTABLES_CRON="${INSTALL_DIR}/xray-guest-iptables-cron.sh"
GUARD_SCRIPT="${GUARD_SCRIPT:-/data/xiaomi-vless-failopen-guard.sh}"
USER_STARTUP="${USER_STARTUP:-/data/startup_user.sh}"
CRON_FILE="${CRON_FILE:-/etc/crontabs/root}"
BOOT_SCRIPT="${BOOT_SCRIPT:-/data/xiaomi-vless-boot.sh}"
BOOT_SRC="${BOOT_SRC:-}"

MARKER_BOOT="# xiaomi-vless-boot"
MARKER_FAILOPEN="# xiaomi-vless-failopen-guard"
MARKER_GUEST="# xiaomi-vless-guest-vpn"
MARKER_PANEL="# xiaomi-vless-panel"
MARKER_UPDATE="# xiaomi-vless-update-resume"

log() { echo "[autostart] $*"; }

detect_use_systemd_panel() {
  if [ "${XIAOMI_VLESS_USE_SYSTEMD:-}" = "1" ]; then
    return 0
  fi
  if command -v systemctl >/dev/null 2>&1 && systemctl cat xiaomi-vless-panel.service >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

remove_legacy_initd() {
  for legacy in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot; do
    if [ -f "/etc/init.d/$legacy" ]; then
      rm -f "/etc/init.d/$legacy"
      log "removed incompatible init.d/$legacy (host uses systemd)"
    fi
  done
}

resolve_boot_src() {
  if [ -n "$BOOT_SRC" ] && [ -f "$BOOT_SRC" ]; then
    echo "$BOOT_SRC"
    return 0
  fi
  script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
  for candidate in \
    "${script_dir}/../scripts/boot-xiaomi-vless.sh" \
    "${script_dir}/boot-xiaomi-vless.sh" \
    "${INSTALL_DIR}/boot-xiaomi-vless.sh"; do
    if [ -f "$candidate" ]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

install_boot_script() {
  src="$(resolve_boot_src)" || die "boot-xiaomi-vless.sh not found"
  mkdir -p "$(dirname "$BOOT_SCRIPT")"
  cp -f "$src" "$BOOT_SCRIPT"
  chmod +x "$BOOT_SCRIPT"
  dest="${INSTALL_DIR}/boot-xiaomi-vless.sh"
  if [ "$src" != "$dest" ]; then
    cp -f "$src" "$dest"
    chmod +x "$dest"
  fi
  log "boot script installed: $BOOT_SCRIPT"
}

install_failopen_guard() {
  script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
  for candidate in \
    "${script_dir}/../scripts/xiaomi-vless-failopen-guard.sh" \
    "${INSTALL_DIR}/xiaomi-vless-failopen-guard.sh" \
    "${script_dir}/xiaomi-vless-failopen-guard.sh"; do
    if [ -f "$candidate" ]; then
      mkdir -p "$(dirname "$GUARD_SCRIPT")"
      cp -f "$candidate" "$GUARD_SCRIPT"
      chmod +x "$GUARD_SCRIPT"
      cp -f "$candidate" "${INSTALL_DIR}/xiaomi-vless-failopen-guard.sh"
      chmod +x "${INSTALL_DIR}/xiaomi-vless-failopen-guard.sh"
      log "fail-open guard installed: $GUARD_SCRIPT"
      return 0
    fi
  done
  log "WARN: xiaomi-vless-failopen-guard.sh not found — skip"
}

install_iptables_cron_wrapper() {
  script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
  for candidate in \
    "${script_dir}/../scripts/xray-guest-iptables-cron.sh" \
    "${INSTALL_DIR}/xray-guest-iptables-cron.sh" \
    "${script_dir}/xray-guest-iptables-cron.sh"; do
    if [ -f "$candidate" ]; then
      cp -f "$candidate" "$IPTABLES_CRON"
      chmod +x "$IPTABLES_CRON"
      log "iptables cron wrapper installed: $IPTABLES_CRON"
      return 0
    fi
  done
  log "WARN: xray-guest-iptables-cron.sh not found — cron will call iptables script directly"
}

remove_marker_block() {
  file="$1"
  marker="$2"
  [ -f "$file" ] || return 0
  awk -v m="$marker" '
    $0 ~ m { skip=1; next }
    skip && /^#/ { skip=0 }
    skip && /^[^#]/ { skip=0 }
    !skip { print }
  ' "$file" > "${file}.tmp" 2>/dev/null && mv "${file}.tmp" "$file" || true
  sed -i "/${marker}/d" "$file" 2>/dev/null || \
    sed -i '' "/${marker}/d" "$file" 2>/dev/null || true
  sed -i '/startup_xray_guest\.sh/d' "$file" 2>/dev/null || \
    sed -i '' '/startup_xray_guest\.sh/d' "$file" 2>/dev/null || true
  sed -i '/xiaomi-vless\/panel /d' "$file" 2>/dev/null || \
    sed -i '' '/xiaomi-vless\/panel /d' "$file" 2>/dev/null || true
  sed -i '/panel-updater\.sh resume/d' "$file" 2>/dev/null || \
    sed -i '' '/panel-updater\.sh resume/d' "$file" 2>/dev/null || true
}

cleanup_legacy_hooks() {
  remove_marker_block "$USER_STARTUP" "$MARKER_GUEST"
  remove_marker_block "$USER_STARTUP" "$MARKER_PANEL"
  remove_marker_block "$USER_STARTUP" "$MARKER_UPDATE"
  remove_marker_block "$CRON_FILE" "$MARKER_GUEST"
}

install_user_startup() {
  mkdir -p "$(dirname "$USER_STARTUP")"
  if [ ! -f "$USER_STARTUP" ]; then
    printf '%s\n' '#!/bin/sh' > "$USER_STARTUP"
    chmod +x "$USER_STARTUP"
  fi
  cleanup_legacy_hooks
  if grep -q "$MARKER_BOOT" "$USER_STARTUP" 2>/dev/null; then
    log "boot hook already in $USER_STARTUP"
    return 0
  fi
  {
    echo "$MARKER_BOOT"
    echo "[ -x ${BOOT_SCRIPT} ] && ${BOOT_SCRIPT} >/dev/null 2>&1 &"
  } >> "$USER_STARTUP"
  chmod +x "$USER_STARTUP"
  log "boot hook added to $USER_STARTUP"
}

remove_uci_firewall() {
  if ! command -v uci >/dev/null 2>&1; then
    return 0
  fi
  if ! uci get firewall.startup_xray_guest >/dev/null 2>&1; then
    return 0
  fi
  uci delete firewall.startup_xray_guest
  uci commit firewall
  log "removed firewall.startup_xray_guest (blocks WAN/NAT on boot)"
}

install_hotplug() {
  hotplug_dir="/etc/hotplug.d/block"
  hotplug_dst="${hotplug_dir}/99-xiaomi-vless"
  hotplug_src="${SCRIPT_DIR}/hotplug-usb-xiaomi-vless.sh"
  if [ ! -f "$hotplug_src" ]; then
    hotplug_src="${INSTALL_DIR}/hotplug-usb-xiaomi-vless.sh"
  fi
  if [ ! -f "$hotplug_src" ] || [ ! -d "$hotplug_dir" ]; then
    return 0
  fi
  cp -f "$hotplug_src" "$hotplug_dst"
  chmod +x "$hotplug_dst"
  cp -f "$hotplug_src" "${INSTALL_DIR}/hotplug-usb-xiaomi-vless.sh"
  chmod +x "${INSTALL_DIR}/hotplug-usb-xiaomi-vless.sh"
  log "hotplug installed: $hotplug_dst"
}

install_boot_init() {
  if detect_use_systemd_panel; then
    log "skip boot init.d — host uses systemd (xiaomi-vless-panel.service)"
    return 0
  fi
  init_src="${SCRIPT_DIR}/xiaomi-vless-boot.init"
  if [ ! -f "$init_src" ] || [ ! -d /etc/init.d ]; then
    return 0
  fi
  cp -f "$init_src" /etc/init.d/xiaomi-vless-boot
  chmod +x /etc/init.d/xiaomi-vless-boot
  if [ -x /etc/rc.common ]; then
    /etc/init.d/xiaomi-vless-boot enable 2>/dev/null || true
    log "procd boot service enabled: /etc/init.d/xiaomi-vless-boot"
  fi
}

install_procd_init() {
  if detect_use_systemd_panel; then
    log "skip xray init.d — host uses systemd"
    return 0
  fi
  init_src="$1"
  if [ ! -f "$init_src" ] || [ ! -d /etc/init.d ]; then
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
  [ -f "$CRON_FILE" ] || touch "$CRON_FILE"

  grep -v xiaomi-vless-boot "$CRON_FILE" 2>/dev/null \
    | grep -v '/data/xiaomi-vless-boot.sh' \
    | grep -v xiaomi-vless-failopen-guard \
    | grep -v 'xray-guest-iptables' > "${CRON_FILE}.tmp" 2>/dev/null || true
  if [ -s "${CRON_FILE}.tmp" ]; then
    mv "${CRON_FILE}.tmp" "$CRON_FILE"
  else
    rm -f "${CRON_FILE}.tmp"
  fi

  {
    echo "$MARKER_BOOT"
    echo "@reboot sleep 30 && ${BOOT_SCRIPT} >/dev/null 2>&1"
    if ! detect_use_systemd_panel; then
      echo "* * * * * pidof panel >/dev/null 2>&1 || ${BOOT_SCRIPT} >/dev/null 2>&1"
    fi
  } >> "$CRON_FILE"
  if detect_use_systemd_panel; then
    log "cron boot added (panel watchdog skipped — systemd manages panel)"
  else
    log "cron boot + panel watchdog added"
  fi

  if [ -x "$GUARD_SCRIPT" ] && ! grep -q 'xiaomi-vless-failopen-guard.sh' "$CRON_FILE" 2>/dev/null; then
    echo "* * * * * ${GUARD_SCRIPT} >/dev/null 2>&1 # ${MARKER_FAILOPEN}" >> "$CRON_FILE"
    log "cron fail-open guard added"
  fi

  cron_iptables="$IPTABLES_CRON"
  if [ ! -x "$cron_iptables" ]; then
    cron_iptables="$IPTABLES_SCRIPT"
  fi
  if [ -x "$cron_iptables" ] && ! grep -q 'xray-guest-iptables' "$CRON_FILE" 2>/dev/null; then
    echo "*/2 * * * * ${cron_iptables} >/dev/null 2>&1" >> "$CRON_FILE"
    log "cron iptables refresh added"
  fi

  if [ -x /etc/init.d/cron ]; then
    /etc/init.d/cron enable 2>/dev/null || true
    /etc/init.d/cron restart 2>/dev/null || true
  elif [ -x /etc/init.d/crond ]; then
    /etc/init.d/crond enable 2>/dev/null || true
    /etc/init.d/crond restart 2>/dev/null || true
  fi
}

die() { log "ERROR: $*"; exit 1; }

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
INIT_SRC="${INIT_SRC:-${SCRIPT_DIR}/xiaomi-vless-xray.init}"
BOOT_SRC="${BOOT_SRC:-${SCRIPT_DIR}/../scripts/boot-xiaomi-vless.sh}"

install_boot_script
install_failopen_guard
install_iptables_cron_wrapper
remove_uci_firewall
install_user_startup
install_hotplug
install_boot_init
install_procd_init "$INIT_SRC"
install_cron

if detect_use_systemd_panel; then
  remove_legacy_initd
fi

log "Autostart configured (USB: ${INSTALL_DIR})"
log "Boot log: tail -f /data/xiaomi-vless-boot.log"
log "Test now: ${BOOT_SCRIPT}"
