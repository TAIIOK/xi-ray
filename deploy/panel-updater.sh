#!/bin/sh
# External updater for xiaomi-vless panel — survives panel exit and boot resume.
set -eu

ACTION="${1:-}"
LOCK_FILE="/tmp/xiaomi-vless-update.lock"

log() { echo "[panel-updater] $*"; }

find_home() {
  for d in /mnt/usb-*/xiaomi-vless; do
    if [ -d "$d" ]; then
      echo "$d"
      return 0
    fi
  done
  return 1
}

read_phase() {
  "$PANEL_BIN" -config "$CONFIG" -update-get-phase -update-home "$HOME" 2>/dev/null || echo idle
}

write_phase() {
  "$PANEL_BIN" -config "$CONFIG" -update-set-phase "$1" -update-home "$HOME"
}

with_lock() {
  if command -v flock >/dev/null 2>&1; then
    exec 9>"$LOCK_FILE"
    if ! flock -n 9; then
      log "another updater is running"
      exit 1
    fi
  else
    if ! mkdir "$LOCK_FILE.d" 2>/dev/null; then
      log "another updater is running"
      exit 1
    fi
    trap 'rmdir "$LOCK_FILE.d" 2>/dev/null || true' EXIT
  fi
  "$@"
}

backup_file() {
  src="$1"
  [ -f "$src" ] || return 0
  ts="$(date +%Y%m%d-%H%M%S)"
  cp -a "$src" "${src}.pre-update.${ts}"
  log "backup: ${src}.pre-update.${ts}"
}

restart_panel() {
  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel restart 2>/dev/null || /etc/init.d/xiaomi-vless-panel start 2>/dev/null || true
    return 0
  fi
  killall panel 2>/dev/null || true
  sleep 1
  nohup "$PANEL_BIN" -config "$CONFIG" >/dev/null 2>&1 &
}

restart_xray() {
  if [ -x /etc/init.d/xiaomi-vless-xray ]; then
    /etc/init.d/xiaomi-vless-xray restart 2>/dev/null || true
    return 0
  fi
  if [ -x "$STARTUP_SCRIPT" ]; then
    "$STARTUP_SCRIPT" >/dev/null 2>&1 || true
  fi
}

install_flash_hooks() {
  deploy_dir="$STAGING/deploy"
  [ -d "$deploy_dir" ] || return 0

  if [ -f "$CRON_FILE" ]; then
    cp -a "$CRON_FILE" "${CRON_FILE}.bak.$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
  fi

  if [ -f "$deploy_dir/xiaomi-vless-panel.init" ] && [ -d /etc/init.d ]; then
    cp "$deploy_dir/xiaomi-vless-panel.init" /etc/init.d/xiaomi-vless-panel
    chmod +x /etc/init.d/xiaomi-vless-panel
    /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
    log "panel init.d updated"
  fi

  if [ -f "$deploy_dir/install-autostart.sh" ]; then
    INSTALL_DIR="$HOME" USB_MOUNT="$(dirname "$HOME")" \
      INIT_SRC="$deploy_dir/xiaomi-vless-xray.init" \
      sh "$deploy_dir/install-autostart.sh"
    log "guest autostart refreshed"
  elif [ -f "$deploy_dir/xiaomi-vless-xray.init" ] && [ -d /etc/init.d ]; then
    cp "$deploy_dir/xiaomi-vless-xray.init" /etc/init.d/xiaomi-vless-xray
    chmod +x /etc/init.d/xiaomi-vless-xray
    /etc/init.d/xiaomi-vless-xray enable 2>/dev/null || true
    log "xray init.d updated"
  fi
}

do_apply() {
  phase="$(read_phase)"
  case "$phase" in
    verified|applying) ;;
    *)
      log "cannot apply in phase: $phase"
      exit 1
      ;;
  esac

  backup_file "$CONFIG"
  if ! "$PANEL_BIN" -config "$CONFIG" -update-apply -update-home "$HOME"; then
    log "apply failed"
    exit 1
  fi
  install_flash_hooks
  write_phase restarting
  restart_panel
  log "panel restarted — health check will run on startup"
}

do_rollback() {
  if ! "$PANEL_BIN" -config "$CONFIG" -update-rollback -update-home "$HOME"; then
    log "rollback failed"
    exit 1
  fi
  restart_panel
  restart_xray
  log "rollback complete"
}

do_resume() {
  phase="$(read_phase)"
  log "resume phase=$phase"
  case "$phase" in
    applying|verified)
      do_apply
      ;;
    restarting)
      restart_panel
      write_phase health_check
      ;;
    health_check)
      restart_panel
      ;;
    downloading|extracting)
      log "waiting for panel to continue download/extract"
      ;;
    *)
      log "nothing to resume"
      ;;
  esac
}

HOME="$(find_home)" || { log "xiaomi-vless home not found"; exit 1; }
PANEL_BIN="$HOME/panel"
CONFIG="$HOME/panel.json"
STAGING="$HOME/updates/staging"
STARTUP_SCRIPT="$HOME/startup_xray_guest.sh"
CRON_FILE="/etc/crontabs/root"

case "$ACTION" in
  apply)
    with_lock do_apply
    ;;
  rollback)
    with_lock do_rollback
    ;;
  resume)
    with_lock do_resume
    ;;
  health-check)
    write_phase health_check
    ;;
  *)
    echo "Usage: $0 apply|rollback|resume|health-check" >&2
    exit 1
    ;;
esac
