# Shared helpers for lab Multipass scripts.
# shellcheck shell=sh

lab_common_init() {
  LAB_SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
  LAB_REPO_ROOT="$(CDPATH= cd -- "$LAB_SCRIPT_DIR/../.." && pwd)"
  LAB_VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"
  LAB_MOUNT_PATH="${LAB_MOUNT_PATH:-/home/ubuntu/xiaomi-vless}"
  LAB_CLOUD_INIT="${LAB_REPO_ROOT}/lab/multipass-cloud-init.yaml"
}

lab_log() { printf '==> %s\n' "$*" >&2; }
lab_die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

lab_require_multipass() {
  command -v multipass >/dev/null 2>&1 || lab_die "multipass not found — install: https://multipass.run/install"
}

lab_vm_state() {
  multipass info "$LAB_VM_NAME" 2>/dev/null | awk -F': ' '/^State:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}'
}

lab_vm_ip() {
  multipass info "$LAB_VM_NAME" | awk -F': ' '/^IPv4:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}'
}

lab_detect_vm_goarch() {
  case "$(multipass exec "$LAB_VM_NAME" -- uname -m)" in
    aarch64|arm64) echo arm64 ;;
    x86_64|amd64) echo amd64 ;;
    *) lab_die "unsupported VM CPU architecture" ;;
  esac
}

lab_build_panel() {
  GOARCH="$1"
  OUT="${LAB_REPO_ROOT}/dist/panel-linux-${GOARCH}"
  VERSION="$(git -C "$LAB_REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
  COMMIT="$(git -C "$LAB_REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  LDFLAGS="-s -w \
    -X github.com/taiiok/xiaomi-vless/internal/version.Version=${VERSION} \
    -X github.com/taiiok/xiaomi-vless/internal/version.Commit=${COMMIT} \
    -X github.com/taiiok/xiaomi-vless/internal/version.BuildDate=${BUILD_DATE}"

  lab_log "building panel for linux/${GOARCH}..."
  (
    cd "$LAB_REPO_ROOT"
    GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build \
      -ldflags "$LDFLAGS" \
      -o "$OUT" ./cmd/panel
  )
  lab_log "built $OUT"
  printf '%s' "$OUT"
}

lab_ensure_vm_running() {
  state="$(lab_vm_state || true)"
  case "$state" in
    Running)
      return 0
      ;;
    Stopped)
      lab_log "starting VM $LAB_VM_NAME"
      multipass start "$LAB_VM_NAME"
      ;;
    Deleted|"")
      lab_die "VM $LAB_VM_NAME not available (state: ${state:-missing}) — run: make lab-up"
      ;;
    *)
      lab_log "VM $LAB_VM_NAME in state '$state' — trying start"
      multipass start "$LAB_VM_NAME" || lab_die "cannot start VM — run: make lab-up"
      ;;
  esac
}

lab_transfer_staging() {
  panel_bin="$1"
  multipass transfer "$panel_bin" "${LAB_VM_NAME}:/tmp/panel-linux"
  multipass transfer "${LAB_REPO_ROOT}/lab/panel.json" "${LAB_VM_NAME}:/tmp/panel-lab.json"
  for script in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-iptables-cron.sh xray-guest-sysctl.sh xiaomi-vless-failopen-guard.sh boot-xiaomi-vless.sh; do
    multipass transfer "${LAB_REPO_ROOT}/scripts/${script}" "${LAB_VM_NAME}:/tmp/${script}"
  done
  for script in network-setup.sh guest-netns.sh; do
    multipass transfer "${LAB_REPO_ROOT}/lab/${script}" "${LAB_VM_NAME}:/tmp/${script}"
  done
  multipass transfer "${LAB_REPO_ROOT}/lab/provision-vm.sh" "${LAB_VM_NAME}:/tmp/xiaomi-vless-provision.sh"
  multipass exec "$LAB_VM_NAME" -- sudo chmod +x /tmp/xiaomi-vless-provision.sh
}

lab_run_provision() {
  keep_panel_json="${1:-0}"
  reset_config="${2:-0}"
  multipass exec "$LAB_VM_NAME" -- sudo env \
    LAB_REPO="$LAB_MOUNT_PATH" \
    LAB_PANEL_BIN=/tmp/panel-linux \
    LAB_PANEL_JSON=/tmp/panel-lab.json \
    LAB_STAGING=/tmp \
    LAB_KEEP_PANEL_JSON="$keep_panel_json" \
    LAB_RESET_CONFIG="$reset_config" \
    sh /tmp/xiaomi-vless-provision.sh
}

lab_deploy_panel_only() {
  panel_bin="$1"
  multipass transfer "$panel_bin" "${LAB_VM_NAME}:/tmp/panel-linux"
  multipass exec "$LAB_VM_NAME" -- sudo sh -s <<'REMOTE'
set -eu
INSTALL="/mnt/usb-lab/xiaomi-vless"
if systemctl is-active --quiet xiaomi-vless-panel.service 2>/dev/null; then
  systemctl stop xiaomi-vless-panel.service
fi
[ -x "$INSTALL/panel" ] && cp "$INSTALL/panel" "$INSTALL/panel.previous"
cp /tmp/panel-linux "$INSTALL/panel.new"
mv "$INSTALL/panel.new" "$INSTALL/panel"
chmod 755 "$INSTALL/panel"
systemctl start xiaomi-vless-panel.service
sleep 1
systemctl is-active xiaomi-vless-panel.service
REMOTE
}

lab_print_panel_url() {
  IP="$(lab_vm_ip)"
  [ -n "$IP" ] || return 0
  printf '\nPanel: http://%s:7777\n' "$IP"
}
