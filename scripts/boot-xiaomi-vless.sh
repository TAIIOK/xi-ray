#!/bin/sh
# Lives on router flash (/data/xiaomi-vless-boot.sh).
# Waits for USB mount, then starts panel and guest VPN. Safe to run multiple times.

set -u

LOG="/data/xiaomi-vless-boot.log"
LOCK="/tmp/xiaomi-vless-boot.lock"

log() {
  echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$LOG"
}

acquire_lock() {
  if [ -d "$LOCK.d" ]; then
    if ! ps w 2>/dev/null | grep -v grep | grep -q '[x]iaomi-vless-boot'; then
      rmdir "$LOCK.d" 2>/dev/null || true
    fi
  fi

  if command -v flock >/dev/null 2>&1; then
    exec 9>"$LOCK"
    if ! flock -n 9; then
      log "skip: another boot instance holds lock"
      exit 0
    fi
    return 0
  fi

  if ! mkdir "$LOCK.d" 2>/dev/null; then
    log "skip: lock busy ($LOCK.d)"
    exit 0
  fi
  trap 'rmdir "$LOCK.d" 2>/dev/null || true' EXIT
}

panel_running() {
  pidof panel >/dev/null 2>&1 && return 0
  ps w 2>/dev/null | grep -v grep | grep -q '[x]iaomi-vless/panel' && return 0
  return 1
}

find_home() {
  for d in /mnt/usb-*/xiaomi-vless; do
    if [ -x "$d/panel" ] && [ -f "$d/panel.json" ]; then
      echo "$d"
      return 0
    fi
  done
  return 1
}

wait_for_usb() {
  i=0
  while [ "$i" -lt 90 ]; do
    if home=$(find_home); then
      echo "$home"
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  return 1
}

wait_for_lan() {
  host="$1"
  i=0
  while [ "$i" -lt 30 ]; do
    if ping -c 1 -W 1 "$host" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i + 1))
    sleep 2
  done
  return 0
}

start_panel() {
  home="$1"
  killall panel 2>/dev/null || true
  sleep 2

  "$home/panel" -config "$home/panel.json" -listen "0.0.0.0:7777" >> "$home/panel.log" 2>&1 &
  sleep 3

  if panel_running; then
    log "panel started pid $(pidof panel 2>/dev/null || echo unknown)"
    return 0
  fi

  log "ERROR: panel failed to start — see $home/panel.log"
}

start_xray() {
  home="$1"
  if [ ! -x "$home/startup_xray_guest.sh" ] || [ ! -f "$home/xray.env" ]; then
    log "xray skipped (not configured yet)"
    return 0
  fi
  if pidof xray >/dev/null 2>&1; then
    log "xray already running pid $(pidof xray)"
    return 0
  fi
  sh "$home/startup_xray_guest.sh" >> "$home/xray-startup.log" 2>&1 &
  log "xray startup launched"
}

log "=== boot invoked pid=$$ ==="
acquire_lock
log "=== boot begin pid=$$ ==="

HOME=$(wait_for_usb) || {
  log "ERROR: xiaomi-vless on USB not found after 180s"
  exit 1
}
log "USB home: $HOME"

wait_for_lan "192.168.31.1"
log "LAN check done"

start_panel "$HOME"

if [ -x "$HOME/panel-updater.sh" ]; then
  ( "$HOME/panel-updater.sh" resume ) >> "$LOG" 2>&1 &
fi

start_xray "$HOME"

log "=== boot done ==="
