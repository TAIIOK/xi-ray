#!/bin/sh
# External updater for xiaomi-vless panel — survives panel exit and boot resume.
set -eu

ACTION="${1:-}"
LOCK_FILE="/tmp/xiaomi-vless-update.lock"

log() { echo "[panel-updater] $*"; }

find_home() {
  if [ -n "${PANEL_UPDATE_TEST_HOME:-}" ] && [ -d "$PANEL_UPDATE_TEST_HOME" ]; then
    echo "$PANEL_UPDATE_TEST_HOME"
    return 0
  fi
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

panel_listen_args() {
  if [ -n "${PANEL_LISTEN:-}" ]; then
    printf '%s' "-listen ${PANEL_LISTEN}"
    return 0
  fi
  if [ -f "$CONFIG" ] && command -v grep >/dev/null 2>&1; then
    addr="$(grep -o '"listen_addr"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG" 2>/dev/null | sed 's/.*"\([^"]*\)"$/\1/' | head -1)"
    if [ -n "$addr" ]; then
      host="${addr%%:*}"
      printf '%s' "-listen ${host:-0.0.0.0}"
      return 0
    fi
  fi
  printf '%s' "-listen 0.0.0.0"
}

uses_systemd_panel() {
  command -v systemctl >/dev/null 2>&1 && systemctl cat xiaomi-vless-panel.service >/dev/null 2>&1
}

wait_panel_http() {
  i=0
  while [ "$i" -lt 30 ]; do
    if command -v curl >/dev/null 2>&1; then
      if curl -fsS --connect-timeout 2 http://127.0.0.1:7777/login >/dev/null 2>&1; then
        return 0
      fi
    elif uses_systemd_panel && systemctl is-active --quiet xiaomi-vless-panel.service 2>/dev/null; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

# Restart from outside the panel service cgroup (updater is often a child of panel and dies on stop).
schedule_panel_restart() {
  if command -v systemd-run >/dev/null 2>&1 && uses_systemd_panel; then
    unit="xiaomi-vless-panel-restart-$$"
    log "scheduling systemctl restart via systemd-run (unit=$unit)"
    if systemd-run --collect --unit="$unit" /bin/systemctl restart xiaomi-vless-panel.service; then
      return 0
    fi
    log "WARN: systemd-run failed — falling back to direct restart"
  fi
  restart_panel
}

restart_panel() {
  if uses_systemd_panel; then
    log "systemctl restart xiaomi-vless-panel.service"
    if ! systemctl restart xiaomi-vless-panel.service; then
      log "ERROR: systemctl restart failed"
      systemctl status xiaomi-vless-panel.service --no-pager -l 2>&1 | tail -10 || true
      return 1
    fi
    if wait_panel_http; then
      return 0
    fi
    log "WARN: panel restarted but HTTP not ready yet"
    return 0
  fi
  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel restart 2>/dev/null || /etc/init.d/xiaomi-vless-panel start 2>/dev/null || true
    sleep 2
    return 0
  fi
  killall panel 2>/dev/null || true
  sleep 1
  listen_args="$(panel_listen_args)"
  # shellcheck disable=SC2086
  (
    cd "$HOME" || exit 1
    nohup "$PANEL_BIN" -config "$CONFIG" $listen_args >> "$HOME/panel-update.log" 2>&1 &
  )
  sleep 2
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

remove_legacy_initd() {
  for legacy in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot; do
    if [ -f "/etc/init.d/$legacy" ]; then
      rm -f "/etc/init.d/$legacy"
      log "removed incompatible init.d/$legacy (host uses systemd)"
    fi
  done
}

install_flash_hooks() {
  deploy_dir="$STAGING/deploy"
  [ -d "$deploy_dir" ] || return 0

  if [ -f "$CRON_FILE" ]; then
    cp -a "$CRON_FILE" "${CRON_FILE}.bak.$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
  fi

  use_systemd_panel=0
  if command -v systemctl >/dev/null 2>&1 && systemctl cat xiaomi-vless-panel.service >/dev/null 2>&1; then
    use_systemd_panel=1
  fi

  if [ -f "$deploy_dir/xiaomi-vless-panel.init" ] && [ -d /etc/init.d ]; then
    if [ "$use_systemd_panel" = "1" ]; then
      rm -f /etc/init.d/xiaomi-vless-panel 2>/dev/null || true
      log "skip panel init.d — host uses systemd (xiaomi-vless-panel.service)"
    else
      cp "$deploy_dir/xiaomi-vless-panel.init" /etc/init.d/xiaomi-vless-panel
      chmod +x /etc/init.d/xiaomi-vless-panel
      /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
      log "panel init.d updated"
    fi
  fi

  if [ -f "$deploy_dir/install-autostart.sh" ]; then
    autostart_env="INSTALL_DIR=$HOME USB_MOUNT=$(dirname "$HOME") INIT_SRC=$deploy_dir/xiaomi-vless-xray.init"
    if [ "$use_systemd_panel" = "1" ]; then
      autostart_env="$autostart_env XIAOMI_VLESS_USE_SYSTEMD=1"
      autostart_env="$autostart_env BOOT_SCRIPT=$HOME/boot-xiaomi-vless.sh"
      autostart_env="$autostart_env GUARD_SCRIPT=$HOME/xiaomi-vless-failopen-guard.sh"
      autostart_env="$autostart_env USER_STARTUP=$HOME/startup_user.sh"
      autostart_env="$autostart_env CRON_FILE=$HOME/crontab.root"
    fi
    # shellcheck disable=SC2086
    env $autostart_env sh "$deploy_dir/install-autostart.sh"
    if [ "$use_systemd_panel" = "1" ]; then
      remove_legacy_initd
    fi
    log "guest autostart refreshed"
  elif [ -f "$deploy_dir/xiaomi-vless-xray.init" ] && [ -d /etc/init.d ]; then
    if [ "$use_systemd_panel" = "1" ]; then
      rm -f /etc/init.d/xiaomi-vless-xray 2>/dev/null || true
      log "skip xray init.d — host uses systemd"
    else
      cp "$deploy_dir/xiaomi-vless-xray.init" /etc/init.d/xiaomi-vless-xray
      chmod +x /etc/init.d/xiaomi-vless-xray
      /etc/init.d/xiaomi-vless-xray enable 2>/dev/null || true
      log "xray init.d updated"
    fi
  fi
}

run_post_update() {
  if "$PANEL_BIN" -config "$CONFIG" -post-update >> "$HOME/panel-update.log" 2>&1; then
    log "post-update tasks completed"
  else
    log "WARN: post-update failed — panel will retry on startup"
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
  if [ -f "$STAGING/deploy/panel-updater.sh" ]; then
    cp "$STAGING/deploy/panel-updater.sh" "$HOME/panel-updater.sh"
    chmod +x "$HOME/panel-updater.sh"
    log "panel-updater.sh synced from staging"
  fi
  if uses_systemd_panel; then
    write_phase restarting
    schedule_panel_restart
    log "binary swapped — panel will restart; post-update runs on new panel startup"
  else
    install_flash_hooks
    run_post_update
    write_phase restarting
    if restart_panel; then
      log "panel restarted — health check will run on startup"
    else
      log "WARN: panel restart failed"
    fi
  fi
}

do_rollback() {
  if ! "$PANEL_BIN" -config "$CONFIG" -update-rollback -update-home "$HOME"; then
    log "rollback failed"
    exit 1
  fi
  if uses_systemd_panel; then
    schedule_panel_restart
  else
    restart_panel
    restart_xray
  fi
  write_phase rolled_back
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
      if uses_systemd_panel && systemctl is-active --quiet xiaomi-vless-panel.service 2>/dev/null; then
        write_phase health_check
        log "panel already running — defer to startup health check"
      else
        if uses_systemd_panel; then
          schedule_panel_restart || log "WARN: resume restart failed"
        else
          restart_panel || log "WARN: resume restart failed"
        fi
        write_phase health_check
      fi
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

# All updater output goes to panel-update.log (survives nohup / service restart).
exec >>"$HOME/panel-update.log" 2>&1
log "action=$ACTION pid=$$"

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
