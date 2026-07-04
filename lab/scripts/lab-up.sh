#!/bin/sh
# Launch or refresh the Multipass lab VM (router simulator).
#
# Usage:
#   ./lab/scripts/lab-up.sh
#   make lab-up
#
# Environment:
#   LAB_VM_NAME   VM name (default: xiaomi-vless-lab)
#   LAB_CPUS      vCPUs (default: 2)
#   LAB_MEM       RAM, e.g. 2G (default: 2G)
#   LAB_DISK      Disk, e.g. 8G (default: 8G)
#   LAB_IMAGE     Multipass image (default: 24.04)
#   LAB_RECREATE  set to 1 to delete and recreate the VM
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REPO_ROOT="$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)"

VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"
CPUS="${LAB_CPUS:-2}"
MEM="${LAB_MEM:-2G}"
DISK="${LAB_DISK:-8G}"
IMAGE="${LAB_IMAGE:-24.04}"
MOUNT_PATH="/home/ubuntu/xiaomi-vless"
CLOUD_INIT="${REPO_ROOT}/lab/multipass-cloud-init.yaml"
PANEL_BIN=""

log() { printf '==> %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

command -v multipass >/dev/null 2>&1 || die "multipass not found — install: https://multipass.run/install"

vm_state() {
  multipass info "$VM_NAME" 2>/dev/null | awk -F': ' '/^State:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}'
}

wait_for_cloud_init() {
  log "waiting for cloud-init..."
  i=0
  while [ "$i" -lt 60 ]; do
    if multipass exec "$VM_NAME" -- cloud-init status 2>/dev/null | grep -q 'status: done'; then
      return 0
    fi
    i=$((i + 1))
    sleep 5
  done
  die "cloud-init did not finish in time"
}

detect_vm_goarch() {
  case "$(multipass exec "$VM_NAME" -- uname -m)" in
    aarch64|arm64) echo arm64 ;;
    x86_64|amd64) echo amd64 ;;
    *) die "unsupported VM CPU architecture" ;;
  esac
}

build_panel() {
  GOARCH="$1"
  OUT="${REPO_ROOT}/dist/panel-linux-${GOARCH}"
  log "building panel for linux/${GOARCH}..."
  (
    cd "$REPO_ROOT"
    GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build \
      -ldflags "-s -w" \
      -o "$OUT" ./cmd/panel
  )
  log "built $OUT"
  PANEL_BIN="$OUT"
}

launch_vm() {
  log "launching VM $VM_NAME ($IMAGE, ${CPUS}c/${MEM}/${DISK})"
  multipass launch "$IMAGE" \
    --name "$VM_NAME" \
    --cpus "$CPUS" \
    --memory "$MEM" \
    --disk "$DISK" \
    --cloud-init "$CLOUD_INIT"
  wait_for_cloud_init
}

purge_vm() {
  multipass delete "$VM_NAME" --purge 2>/dev/null || true
}

ensure_vm() {
  if [ "${LAB_RECREATE:-0}" = "1" ]; then
    log "recreating VM $VM_NAME"
    purge_vm
  fi

  state="$(vm_state || true)"
  case "$state" in
    Running)
      log "VM $VM_NAME already running"
      ;;
    Stopped)
      log "starting VM $VM_NAME"
      multipass start "$VM_NAME"
      ;;
    Deleted)
      log "VM $VM_NAME is deleted — purging stale record and recreating"
      purge_vm
      launch_vm
      ;;
    "")
      launch_vm
      ;;
    *)
      log "VM $VM_NAME in state '$state' — trying start"
      if multipass start "$VM_NAME" 2>/dev/null; then
        :
      else
        log "start failed — recreating VM"
        purge_vm
        launch_vm
      fi
      ;;
  esac
}

ensure_mount() {
  state="$(vm_state || true)"
  [ "$state" = "Running" ] || die "VM is not running (state: ${state:-missing})"

  if multipass exec "$VM_NAME" -- test -d "${MOUNT_PATH}/.git" 2>/dev/null; then
    log "mount already active: $MOUNT_PATH"
    return 0
  fi

  log "mounting repo into VM"
  multipass umount "${VM_NAME}:${MOUNT_PATH}" 2>/dev/null || true
  multipass mount "$REPO_ROOT" "${VM_NAME}:${MOUNT_PATH}"
}

vm_ip() {
  multipass info "$VM_NAME" | awk -F': ' '/^IPv4:/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2; exit}'
}

ensure_vm
ensure_mount

GOARCH="$(detect_vm_goarch)"
build_panel "$GOARCH"

log "provisioning VM (panel, Xray, network, systemd)..."
multipass transfer "$PANEL_BIN" "${VM_NAME}:/tmp/panel-linux"
multipass transfer "${REPO_ROOT}/lab/panel.json" "${VM_NAME}:/tmp/panel-lab.json"
for script in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-sysctl.sh boot-xiaomi-vless.sh; do
  multipass transfer "${REPO_ROOT}/scripts/${script}" "${VM_NAME}:/tmp/${script}"
done
for script in network-setup.sh guest-netns.sh; do
  multipass transfer "${REPO_ROOT}/lab/${script}" "${VM_NAME}:/tmp/${script}"
done
multipass transfer "${REPO_ROOT}/lab/provision-vm.sh" "${VM_NAME}:/tmp/xiaomi-vless-provision.sh"
multipass exec "$VM_NAME" -- sudo chmod +x /tmp/xiaomi-vless-provision.sh
multipass exec "$VM_NAME" -- sudo env \
  LAB_REPO="$MOUNT_PATH" \
  LAB_PANEL_BIN=/tmp/panel-linux \
  LAB_PANEL_JSON=/tmp/panel-lab.json \
  LAB_STAGING=/tmp \
  sh /tmp/xiaomi-vless-provision.sh

IP="$(vm_ip)"
[ -n "$IP" ] || die "could not read VM IP"

cat <<EOF

Lab VM is ready.

  VM:     $VM_NAME
  IP:     $IP
  Panel:  http://${IP}:7777
  Setup:  http://${IP}:7777/onboarding  (admin / admin)

Useful commands:
  ./lab/scripts/lab-shell.sh
  ./lab/scripts/lab-status.sh
  ./lab/scripts/lab-guest-test.sh
  ./lab/scripts/lab-down.sh

Inside the VM (as root):
  ip netns exec guest-test curl -4 https://ifconfig.me
  iptables -t nat -L XRAY_GUEST_TCP -v -n

EOF
