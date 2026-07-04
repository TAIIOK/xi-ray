#!/bin/sh
# Reset lab panel login to admin / admin (panel.json is kept otherwise).
#
# Usage: ./lab/scripts/lab-reset-password.sh
#        make lab-reset-password
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/scripts/lab-common.sh
. "${SCRIPT_DIR}/lab-common.sh"
lab_common_init
lab_require_multipass
lab_ensure_vm_running

PANEL_JSON="/mnt/usb-lab/xiaomi-vless/panel.json"
GOARCH="$(lab_detect_vm_goarch)"
SET_PASS_BIN="${LAB_REPO_ROOT}/dist/lab-set-password-linux-${GOARCH}"

lab_log "reset panel password to admin/admin on $LAB_VM_NAME"
mkdir -p "${LAB_REPO_ROOT}/dist"
(
  cd "$LAB_REPO_ROOT"
  GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -o "$SET_PASS_BIN" ./lab/cmd/lab-set-password
)

multipass transfer "$SET_PASS_BIN" "${LAB_VM_NAME}:/tmp/lab-set-password"
multipass exec "$LAB_VM_NAME" -- sudo sh -s <<'REMOTE'
set -eu
systemctl stop xiaomi-vless-panel.service
chmod +x /tmp/lab-set-password
/tmp/lab-set-password /mnt/usb-lab/xiaomi-vless/panel.json admin
rm -f /tmp/lab-set-password
systemctl start xiaomi-vless-panel.service
sleep 1
code="$(curl -sS -c /tmp/cj -b /tmp/cj -X POST http://127.0.0.1:7777/login \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'username=admin&password=admin' -o /dev/null -w '%{http_code}')"
if [ "$code" != "303" ]; then
  echo "login verify failed: HTTP $code" >&2
  exit 1
fi
echo "login admin/admin OK"
REMOTE

lab_print_panel_url
lab_log "password reset complete — use admin / admin"
