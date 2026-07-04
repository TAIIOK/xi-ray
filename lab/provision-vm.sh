#!/bin/sh
# Provisions the Multipass lab VM (run inside the VM as root).
set -eu

REPO="${LAB_REPO:-/home/ubuntu/xiaomi-vless}"
STAGING="${LAB_STAGING:-/tmp}"
USB_MOUNT="/mnt/usb-lab"
INSTALL_DIR="${USB_MOUNT}/xiaomi-vless"
XRAY_DIR="${USB_MOUNT}/xray"
XRAY_BIN="${XRAY_DIR}/bin/xray"

log() { echo "[lab-provision] $*"; }
die() { echo "[lab-provision] ERROR: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root (sudo sh lab/provision-vm.sh)"

mkdir -p "$INSTALL_DIR" "${XRAY_DIR}/bin" "${INSTALL_DIR}/updates/downloads" "${INSTALL_DIR}/updates/staging"
chmod 755 "$INSTALL_DIR"

install_panel_binary() {
  src=$1
  dst="${INSTALL_DIR}/panel"
  systemctl stop xiaomi-vless-panel.service 2>/dev/null || true
  killall panel 2>/dev/null || true
  sleep 1
  if [ -f "$dst" ]; then
    cp "$dst" "${dst}.previous"
  fi
  cp "$src" "${dst}.new"
  mv "${dst}.new" "$dst"
  chmod 755 "$dst"
}

start_panel_service() {
  systemctl stop xiaomi-vless-panel.service 2>/dev/null || true
  killall panel 2>/dev/null || true
  sleep 1
  systemctl reset-failed xiaomi-vless-panel.service 2>/dev/null || true
  systemctl start xiaomi-vless-panel.service
  i=0
  while [ "$i" -lt 15 ]; do
    if systemctl is-active --quiet xiaomi-vless-panel.service; then
      return 0
    fi
    i=$((i + 1))
    sleep 1
  done
  return 1
}

if [ -n "${LAB_PANEL_BIN:-}" ] && [ -f "$LAB_PANEL_BIN" ]; then
  install_panel_binary "$LAB_PANEL_BIN"
elif [ -f "${STAGING}/panel-linux" ]; then
  install_panel_binary "${STAGING}/panel-linux"
else
  PANEL_SRC=""
  for candidate in \
    "${REPO}/dist/panel-linux-arm64" \
    "${REPO}/dist/panel-linux-amd64" \
    "${REPO}/dist/panel"; do
    if [ -f "$candidate" ]; then
      PANEL_SRC="$candidate"
      break
    fi
  done
  [ -n "$PANEL_SRC" ] || die "panel binary not found — lab-up.sh should build and transfer it first"
  install_panel_binary "$PANEL_SRC"
fi
log "panel installed"

if [ "${LAB_RESET_CONFIG:-0}" = "1" ]; then
  if [ -n "${LAB_PANEL_JSON:-}" ] && [ -f "$LAB_PANEL_JSON" ]; then
    cp "$LAB_PANEL_JSON" "${INSTALL_DIR}/panel.json"
  elif [ -f "${REPO}/lab/panel.json" ]; then
    cp "${REPO}/lab/panel.json" "${INSTALL_DIR}/panel.json"
  else
    die "panel.json template not found"
  fi
  log "panel.json reset from lab template"
  log "clearing lab runtime state (xray, fail-open, staging)"
  killall xray 2>/dev/null || true
  echo '{}' > "${XRAY_DIR}/config.json"
  rm -f "${INSTALL_DIR}/panel.previous"
  rm -f /data/xiaomi-vless-failopen
  rm -rf "${INSTALL_DIR}/updates/staging/"* 2>/dev/null || true
elif [ -f "${INSTALL_DIR}/panel.json" ] && [ "${LAB_KEEP_PANEL_JSON:-0}" = "1" ]; then
  log "keeping existing panel.json"
elif [ -n "${LAB_PANEL_JSON:-}" ] && [ -f "$LAB_PANEL_JSON" ]; then
  cp "$LAB_PANEL_JSON" "${INSTALL_DIR}/panel.json"
elif [ -f "${REPO}/lab/panel.json" ]; then
  cp "${REPO}/lab/panel.json" "${INSTALL_DIR}/panel.json"
else
  die "panel.json not found"
fi
chmod 600 "${INSTALL_DIR}/panel.json"

install_script() {
  name="$1"
  if [ -f "${STAGING}/${name}" ]; then
    cp "${STAGING}/${name}" "${INSTALL_DIR}/${name}"
  elif [ -f "${REPO}/scripts/${name}" ]; then
    cp "${REPO}/scripts/${name}" "${INSTALL_DIR}/${name}"
  else
    die "missing script: $name"
  fi
  chmod +x "${INSTALL_DIR}/${name}"
}

for script in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-iptables-cron.sh xray-guest-sysctl.sh xiaomi-vless-failopen-guard.sh boot-xiaomi-vless.sh; do
  install_script "$script"
done

if [ -f "${STAGING}/panel-updater.sh" ]; then
  cp "${STAGING}/panel-updater.sh" "${INSTALL_DIR}/panel-updater.sh"
  chmod +x "${INSTALL_DIR}/panel-updater.sh"
  log "panel-updater.sh installed from staging"
elif [ -f "${REPO}/deploy/panel-updater.sh" ]; then
  cp "${REPO}/deploy/panel-updater.sh" "${INSTALL_DIR}/panel-updater.sh"
  chmod +x "${INSTALL_DIR}/panel-updater.sh"
  log "panel-updater.sh installed"
else
  die "missing deploy/panel-updater.sh"
fi

for script in network-setup.sh guest-netns.sh; do
  if [ -f "${STAGING}/${script}" ]; then
    cp "${STAGING}/${script}" "${INSTALL_DIR}/${script}"
  elif [ -f "${REPO}/lab/${script}" ]; then
    cp "${REPO}/lab/${script}" "${INSTALL_DIR}/${script}"
  else
    die "missing lab script: $script"
  fi
  chmod +x "${INSTALL_DIR}/${script}"
done

download_xray() {
  ARCH="$(uname -m)"
  case "$ARCH" in
    aarch64|arm64)
      ZIP_URL="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip"
      ;;
    x86_64|amd64)
      ZIP_URL="https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip"
      ;;
    *)
      die "unsupported VM architecture: $ARCH"
      ;;
  esac

  TMP="$(mktemp -d)"
  trap 'rm -rf "$TMP"' EXIT INT HUP

  log "downloading Xray ($ARCH)..."
  curl -fsSL "$ZIP_URL" -o "${TMP}/xray.zip"
  unzip -qo "${TMP}/xray.zip" -d "${TMP}/xray"
  install -m 755 "${TMP}/xray/xray" "$XRAY_BIN"

  curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" \
    -o "${XRAY_DIR}/bin/geoip.dat"
  curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" \
    -o "${XRAY_DIR}/bin/geosite.dat"

  log "Xray installed to $XRAY_BIN"
}

if [ ! -x "$XRAY_BIN" ]; then
  download_xray
else
  log "keeping existing Xray at $XRAY_BIN"
fi

if [ ! -f "${XRAY_DIR}/config.json" ]; then
  echo '{}' > "${XRAY_DIR}/config.json"
fi

sh "${INSTALL_DIR}/network-setup.sh"

cat > /etc/systemd/system/xiaomi-vless-network.service << EOF
[Unit]
Description=Xiaomi VLESS lab network bridges
Before=xiaomi-vless-panel.service
After=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=${INSTALL_DIR}/network-setup.sh

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/xiaomi-vless-panel.service << EOF
[Unit]
Description=Xiaomi VLESS Panel (lab)
After=network-online.target xiaomi-vless-network.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/panel -config ${INSTALL_DIR}/panel.json -listen 0.0.0.0:7777
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable xiaomi-vless-network.service xiaomi-vless-panel.service
systemctl restart xiaomi-vless-network.service
if start_panel_service; then
  log "panel service is running"
else
  log "ERROR: panel service failed — check: journalctl -u xiaomi-vless-panel -n 50"
  exit 1
fi

log "provision complete"
log "USB mount: $USB_MOUNT"
log "panel config: ${INSTALL_DIR}/panel.json"
