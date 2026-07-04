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
COMMON_SH="$BUNDLE_DIR/deploy/install-common.sh"
if [ ! -f "$COMMON_SH" ]; then
  COMMON_SH="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)/deploy/install-common.sh"
fi
[ -f "$COMMON_SH" ] || { printf '[install] ERROR: missing install-common.sh\n' >&2; exit 1; }
# shellcheck source=deploy/install-common.sh
. "$COMMON_SH"

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
  for f in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-iptables-cron.sh xray-guest-sysctl.sh xiaomi-vless-failopen-guard.sh boot-xiaomi-vless.sh; do
    src="$BUNDLE_DIR/scripts/$f"
    [ -f "$src" ] || die "missing $src in bundle"
    cp -f "$src" "$INSTALL_DIR/scripts/$f"
    cp -f "$src" "$INSTALL_DIR/$f"
    chmod +x "$INSTALL_DIR/scripts/$f" "$INSTALL_DIR/$f"
  done
  log "panel and scripts installed to $INSTALL_DIR"
}

install_autostart_hooks() {
  if [ -f "$BUNDLE_DIR/deploy/install-autostart.sh" ]; then
    INSTALL_DIR="$INSTALL_DIR" USB_MOUNT="$USB_MOUNT" \
      INIT_SRC="$BUNDLE_DIR/deploy/xiaomi-vless-xray.init" \
      BOOT_SRC="$BUNDLE_DIR/scripts/boot-xiaomi-vless.sh" \
      sh "$BUNDLE_DIR/deploy/install-autostart.sh"
  fi

  init_src="$BUNDLE_DIR/deploy/xiaomi-vless-panel.init"
  install_panel_init "$init_src"
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

start_panel "$PANEL_BIN" "$PANEL_CONFIG"
if wait_for_panel "$PANEL_BIN"; then
  log "panel is responding on :7777"
else
  log "WARN: panel may still be starting — check: tail -f ${INSTALL_DIR}/panel.log"
fi

start_xray "$INSTALL_DIR"
print_install_done "$INSTALL_DIR" "$PANEL_BIN"
