# Shared helpers for deploy/install.sh and deploy/install-from-release.sh
# shellcheck shell=sh

log() { printf '[install] %s\n' "$*"; }
die() { printf '[install] ERROR: %s\n' "$*" >&2; exit 1; }

find_usb_mount() {
  for d in /mnt/usb-*; do
    if [ -d "$d" ] && [ -w "$d" ]; then
      echo "$d"
      return 0
    fi
  done
  return 1
}

wait_for_panel() {
  i=0
  while [ "$i" -lt 15 ]; do
    if wget -q -O /dev/null http://192.168.31.1:7777/login 2>/dev/null || \
       wget -q -O /dev/null http://192.168.31.1:7777/ 2>/dev/null || \
       wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

install_panel_init() {
  init_src="$1"
  if [ ! -f "$init_src" ] || [ ! -d /etc/init.d ]; then
    return 0
  fi
  cp -f "$init_src" /etc/init.d/xiaomi-vless-panel
  chmod +x /etc/init.d/xiaomi-vless-panel
  /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
  log "procd service enabled: /etc/init.d/xiaomi-vless-panel"
}

start_panel() {
  panel_bin="$1"
  panel_config="$2"

  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel stop 2>/dev/null || true
    sleep 1
    /etc/init.d/xiaomi-vless-panel start 2>/dev/null || /etc/init.d/xiaomi-vless-panel restart 2>/dev/null || true
    log "panel started via procd"
    return 0
  fi

  killall panel 2>/dev/null || true
  sleep 1
  nohup "$panel_bin" -config "$panel_config" >/dev/null 2>&1 &
  log "panel started in background (no procd)"
}

start_xray() {
  install_dir="$1"
  startup="${install_dir}/startup_xray_guest.sh"

  if [ -x /etc/init.d/xiaomi-vless-xray ]; then
    /etc/init.d/xiaomi-vless-xray stop 2>/dev/null || true
    sleep 1
    /etc/init.d/xiaomi-vless-xray start 2>/dev/null || /etc/init.d/xiaomi-vless-xray restart 2>/dev/null || true
    log "xray autostart service started via procd"
    return 0
  fi

  if [ -f "${install_dir}/xray.env" ] && [ -x "$startup" ]; then
    sh "$startup" >/dev/null 2>&1 &
    log "xray started via startup script"
    return 0
  fi

  log "xray not configured yet — complete onboarding and press Apply"
}

print_install_done() {
  install_dir="$1"
  panel_bin="$2"
  ver="$("$panel_bin" -version 2>/dev/null | head -1 || echo unknown)"

  echo ""
  echo "============================================"
  echo "  Xiaomi VLESS — установка завершена"
  echo "============================================"
  echo "  Версия:   ${ver:-unknown}"
  echo "  Каталог:  $install_dir"
  echo "  Панель:   http://192.168.31.1:7777"
  echo "  Логин:    admin / admin"
  echo ""
  echo "  Откройте /onboarding — скачайте Xray и"
  echo "  добавьте подписку, затем нажмите Apply."
  echo ""
  echo "  Лог panel:  tail -f ${install_dir}/panel.log"
  echo "  Лог boot:   tail -f /data/xiaomi-vless-boot.log"
  echo "  Лог VPN:    tail -f ${install_dir}/xray-startup.log"
  echo "============================================"
}
