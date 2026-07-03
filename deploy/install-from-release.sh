#!/bin/sh
# Install xiaomi-vless from an extracted release bundle.
#
#   tar xzf xiaomi-vless-v1.0.0-linux-arm64.tar.gz -C /tmp
#   sh /tmp/install.sh
#
# Or from inside the extracted directory:
#   cd xiaomi-vless-... && sh install.sh
set -eu

BUNDLE_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

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

verify_panel() {
  if [ ! -f "$BUNDLE_DIR/panel" ]; then
    die "panel binary not found in $BUNDLE_DIR — extract the release archive first"
  fi
  if [ ! -x "$BUNDLE_DIR/panel" ]; then
    chmod +x "$BUNDLE_DIR/panel"
  fi
  if [ -f "$BUNDLE_DIR/panel.sha256" ] && command -v sha256sum >/dev/null 2>&1; then
    want="$(awk '{print $1}' "$BUNDLE_DIR/panel.sha256")"
    got="$(sha256sum "$BUNDLE_DIR/panel" | awk '{print $1}')"
    if [ "$want" != "$got" ]; then
      die "panel checksum mismatch (want $want got $got)"
    fi
    log "panel checksum OK"
  fi
}

write_config() {
  example="$BUNDLE_DIR/deploy/panel.json.example"
  if [ ! -f "$example" ]; then
    example="$BUNDLE_DIR/panel.json.example"
  fi
  if [ ! -f "$PANEL_CONFIG" ]; then
    if [ ! -f "$example" ]; then
      die "panel.json.example missing in bundle"
    fi
    cp "$example" "$PANEL_CONFIG"
    sed -i "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG" 2>/dev/null || \
      sed -i '' "s|/mnt/usb-ed49605f|${USB_MOUNT}|g" "$PANEL_CONFIG"
    chmod 600 "$PANEL_CONFIG"
    log "created $PANEL_CONFIG"
  else
    log "keeping existing $PANEL_CONFIG"
  fi
}

install_files() {
  cp -f "$BUNDLE_DIR/panel" "$PANEL_BIN"
  chmod +x "$PANEL_BIN"

  cp -f "$BUNDLE_DIR/deploy/panel-updater.sh" "$INSTALL_DIR/panel-updater.sh"
  chmod +x "$INSTALL_DIR/panel-updater.sh"

  mkdir -p "$INSTALL_DIR/scripts"
  for f in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-sysctl.sh; do
    src="$BUNDLE_DIR/scripts/$f"
    [ -f "$src" ] || die "missing $src in bundle"
    cp -f "$src" "$INSTALL_DIR/scripts/$f"
    cp -f "$src" "$INSTALL_DIR/$f"
    chmod +x "$INSTALL_DIR/scripts/$f" "$INSTALL_DIR/$f"
  done
  log "panel and scripts installed to $INSTALL_DIR"
}

install_autostart_hooks() {
  PANEL_MARKER="# xiaomi-vless-panel"
  UPDATE_RESUME_MARKER="# xiaomi-vless-update-resume"
  USER_STARTUP="/data/startup_user.sh"

  if [ ! -f "$USER_STARTUP" ]; then
    printf '%s\n' '#!/bin/sh' > "$USER_STARTUP"
    chmod +x "$USER_STARTUP"
  fi

  if ! grep -q "$UPDATE_RESUME_MARKER" "$USER_STARTUP" 2>/dev/null; then
    {
      echo "$UPDATE_RESUME_MARKER"
      echo "[ -x ${INSTALL_DIR}/panel-updater.sh ] && ${INSTALL_DIR}/panel-updater.sh resume >/dev/null 2>&1"
    } >> "$USER_STARTUP"
  fi

  if ! grep -q "$PANEL_MARKER" "$USER_STARTUP" 2>/dev/null; then
    {
      echo "$PANEL_MARKER"
      echo "sleep 25 && ${PANEL_BIN} -config ${PANEL_CONFIG} >/dev/null 2>&1 &"
    } >> "$USER_STARTUP"
  fi

  if [ -f "$BUNDLE_DIR/deploy/install-autostart.sh" ]; then
    INSTALL_DIR="$INSTALL_DIR" USB_MOUNT="$USB_MOUNT" \
      INIT_SRC="$BUNDLE_DIR/deploy/xiaomi-vless-xray.init" \
      sh "$BUNDLE_DIR/deploy/install-autostart.sh"
  fi

  if [ -f "$BUNDLE_DIR/deploy/xiaomi-vless-panel.init" ] && [ -d /etc/init.d ]; then
    cp -f "$BUNDLE_DIR/deploy/xiaomi-vless-panel.init" /etc/init.d/xiaomi-vless-panel
    chmod +x /etc/init.d/xiaomi-vless-panel
    /etc/init.d/xiaomi-vless-panel enable 2>/dev/null || true
  fi
}

start_panel() {
  if [ -x /etc/init.d/xiaomi-vless-panel ]; then
    /etc/init.d/xiaomi-vless-panel stop 2>/dev/null || true
    sleep 1
    /etc/init.d/xiaomi-vless-panel start 2>/dev/null || /etc/init.d/xiaomi-vless-panel restart 2>/dev/null || true
    log "panel started via procd"
    return 0
  fi
  killall panel 2>/dev/null || true
  sleep 1
  nohup "$PANEL_BIN" -config "$PANEL_CONFIG" >/dev/null 2>&1 &
  log "panel started in background"
}

print_done() {
  ver="$("$PANEL_BIN" -version 2>/dev/null | head -1 || echo unknown)"
  echo ""
  echo "============================================"
  echo "  Xiaomi VLESS — установка завершена"
  echo "============================================"
  echo "  Версия:   ${ver:-unknown}"
  echo "  Каталог:  $INSTALL_DIR"
  echo "  Панель:   http://192.168.31.1:7777"
  echo "  Логин:    admin / admin"
  echo ""
  echo "  Откройте /onboarding — скачайте Xray и"
  echo "  добавьте подписку, затем нажмите Apply."
  echo ""
  echo "  Лог panel:  tail -f ${INSTALL_DIR}/panel.log"
  echo "  Лог VPN:    tail -f ${INSTALL_DIR}/xray-startup.log"
  echo "============================================"
}

# --- main ---

log "bundle: $BUNDLE_DIR"
verify_panel

USB_MOUNT="$(find_usb_mount)" || die "USB not found under /mnt/usb-* — plug in USB stick and retry"

INSTALL_DIR="${USB_MOUNT}/xiaomi-vless"
PANEL_BIN="${INSTALL_DIR}/panel"
PANEL_CONFIG="${INSTALL_DIR}/panel.json"

mkdir -p "$INSTALL_DIR"
mkdir -p "${INSTALL_DIR}/updates/downloads" "${INSTALL_DIR}/updates/staging"
chmod 755 "$INSTALL_DIR"

log "USB mount: $USB_MOUNT"
log "install dir: $INSTALL_DIR"

install_files
write_config
install_autostart_hooks
start_panel
print_done
