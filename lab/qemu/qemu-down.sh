#!/bin/sh
# Stop the QEMU OpenWrt lab VM.
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

stop_pid() {
  pid="$1"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    i=0
    while [ "$i" -lt 20 ]; do
      kill -0 "$pid" 2>/dev/null || return 0
      i=$((i + 1))
      sleep 1
    done
    kill -9 "$pid" 2>/dev/null || true
  fi
}

if [ -f "$QEMU_PIDFILE" ]; then
  pid="$(cat "$QEMU_PIDFILE" 2>/dev/null || true)"
  if [ -n "$pid" ]; then
    if kill -0 "$pid" 2>/dev/null; then
      stop_pid "$pid"
    elif sudo kill -0 "$pid" 2>/dev/null; then
      sudo kill "$pid" 2>/dev/null || true
    fi
  fi
  rm -f "$QEMU_PIDFILE"
fi

pkill -f "qemu-system-aarch64.*${QEMU_LAB_NAME}" 2>/dev/null || true
qemu_log "QEMU stopped"
