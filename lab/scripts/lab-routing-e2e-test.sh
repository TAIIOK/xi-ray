#!/bin/bash
# E2E routing test with live verbose output.
# Runs INSIDE lab VM in one multipass session (fast, no silent hangs).
#
# Usage:
#   ./lab/scripts/lab-routing-e2e-test.sh
#   LAB_E2E_QUICK=1 ./lab/scripts/lab-routing-e2e-test.sh   # direct/proxy/block only
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lab/scripts/lab-common.sh
source "${SCRIPT_DIR}/lab-common.sh"
lab_common_init
lab_require_multipass
lab_ensure_vm_running

VM_IP="$(lab_vm_ip)"
GOARCH="$(lab_detect_vm_goarch)"
SET_PASS_BIN="${LAB_REPO_ROOT}/dist/lab-set-password-linux-${GOARCH}"
QUICK="${LAB_E2E_QUICK:-0}"

log() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }

log "build lab-set-password (${GOARCH})"
mkdir -p "${LAB_REPO_ROOT}/dist"
( cd "$LAB_REPO_ROOT" && GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -o "$SET_PASS_BIN" ./lab/cmd/lab-set-password )

log "transfer helper to VM"
multipass transfer "$SET_PASS_BIN" "${LAB_VM_NAME}:/tmp/lab-set-password"

log "start in-VM test session (output streams below)"
log "VM=$LAB_VM_NAME ip=$VM_IP quick=$QUICK"
echo "────────────────────────────────────────"

multipass exec "$LAB_VM_NAME" -- sudo env QUICK="$QUICK" bash -s <<'REMOTE'
set -euo pipefail

log() { printf '[vm %s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
die() { log "FAIL: $*"; exit 1; }

PANEL_JSON="/mnt/usb-lab/xiaomi-vless/panel.json"
PANEL_BAK="/tmp/panel.routing-e2e.bak"
INSTALL="/mnt/usb-lab/xiaomi-vless"
NETNS="guest-test"
BASE="http://127.0.0.1:7777"
COOKIE="/tmp/routing-e2e.cookie"
TEST_PASS="lab-routing-e2e"
TARGET_IP="https://ipinfo.io/ip"
TARGET_PAGE="https://ipinfo.io/what-is-my-ip"
MATCHER="domain:ipinfo.io"
APPLY_TIMEOUT=90
GUEST_TIMEOUT=12

cleanup() {
  log "restore panel.json + restart panel"
  cp "$PANEL_BAK" "$PANEL_JSON" 2>/dev/null || true
  systemctl restart xiaomi-vless-panel 2>/dev/null || true
  rm -f "$COOKIE" /tmp/lab-set-password "$PANEL_BAK"
}
trap cleanup EXIT

run_timeout() {
  local sec=$1; shift
  log "exec (timeout ${sec}s): $*"
  local t0=$SECONDS
  if timeout "$sec" "$@"; then
    log "done in $((SECONDS - t0))s"
    return 0
  fi
  local rc=$?
  log "FAILED rc=$rc after $((SECONDS - t0))s: $*"
  return "$rc"
}

log "backup panel.json + temp password"
cp "$PANEL_JSON" "$PANEL_BAK"
chmod +x /tmp/lab-set-password
/tmp/lab-set-password "$PANEL_JSON" "$TEST_PASS"
systemctl restart xiaomi-vless-panel
sleep 2

log "setup guest netns"
sh "${INSTALL}/guest-netns.sh" >/dev/null 2>&1 || true

login() {
  run_timeout 15 curl -sS -c "$COOKIE" -b "$COOKIE" -X POST "$BASE/login" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d "username=admin&password=${TEST_PASS}" -o /dev/null -w 'login_http=%{http_code}\n'
}

apply_routing() {
  local label=$1 body=$2
  log "APPLY routing: ${label}"
  login
  local t0=$SECONDS
  local resp
  resp="$(run_timeout "$APPLY_TIMEOUT" curl -sS -b "$COOKIE" -X PUT "$BASE/api/routing" \
    -H 'Content-Type: application/json' -d "$body")" || die "apply curl timeout/fail for ${label}"
  log "apply response ($((SECONDS - t0))s): $(echo "$resp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("ok=",d.get("ok"),"msg=",(d.get("apply") or {}).get("message",d.get("message","")))' 2>/dev/null || echo "$resp")"
  echo "$resp" | python3 -c "
import json,sys
d=json.load(sys.stdin)
if not d.get('ok'):
    print(json.dumps(d, indent=2)); sys.exit(1)
a=d.get('apply') or {}
if not a.get('ok'):
    print(json.dumps(d, indent=2)); sys.exit(1)
"
  sh "${INSTALL}/guest-netns.sh" >/dev/null 2>&1 || true
  sleep 1
  pidof xray >/dev/null 2>&1 || die "xray died after apply ${label}"
}

guest_ip() {
  local url=$1 errf=/tmp/guest-curl.err out
  : >"$errf"
  if out="$(run_timeout "$GUEST_TIMEOUT" ip netns exec "$NETNS" curl -4 -sS --connect-timeout 4 --max-time 8 "$url" 2>"$errf")"; then
    if [ -n "$out" ]; then
      echo "$out"
      return 0
    fi
  fi
  log "guest curl FAIL url=$url: $(tail -1 "$errf" 2>/dev/null || echo unknown)"
  echo FAIL
}

guest_code() {
  local url=$1
  run_timeout "$GUEST_TIMEOUT" ip netns exec "$NETNS" \
    curl -4 -sS -o /dev/null -w '%{http_code}' --connect-timeout 4 --max-time 8 "$url" 2>/dev/null || echo 000
}

payload() {
  local order=$1 rules=$2
  # default=proxy: lab guest netns needs VPN path for DNS; per-domain rules override it.
  printf '{"apply":true,"routing":{"domain_strategy":"IPIfNonMatch","rule_order":%s,"default_guest_action":"proxy","bypass_private":true,"bypass_vpn_hosts":true,"rules":%s}}' "$order" "$rules"
}

ensure_xray() {
  if pidof xray >/dev/null 2>&1; then
    log "xray running pid=$(pidof xray)"
    return 0
  fi
  log "xray NOT running — POST /api/apply"
  login
  local resp
  resp="$(run_timeout "$APPLY_TIMEOUT" curl -sS -b "$COOKIE" -X POST "$BASE/api/apply" -H 'Content-Type: application/json' -d '{}')" \
    || die "POST /api/apply timeout/failed"
  log "apply: $(echo "$resp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("message",d))' 2>/dev/null || echo "$resp")"
  pidof xray >/dev/null 2>&1 || die "xray still down after apply — check: journalctl -u xiaomi-vless-panel -n 30"
}

ensure_xray

log "baseline IPs"
WAN="$(run_timeout 15 curl -4 -sS --connect-timeout 4 --max-time 10 https://ifconfig.me)" || WAN=""
VPN="$(run_timeout 15 curl -4 -sS --connect-timeout 4 --max-time 10 -x socks5h://127.0.0.1:10808 https://ifconfig.me)" || VPN=""
log "WAN=$WAN"
log "VPN=$VPN"
[ -n "$WAN" ] && [ -n "$VPN" ] || die "cannot measure baseline IPs"
[ "$WAN" != "$VPN" ] || die "WAN and VPN IPs must differ for proxy test"

is_blocked() {
  case "$1" in
    000|000000|"") return 0 ;;
    *) return 1 ;;
  esac
}

RULE='[{"id":"ipinfo","name":"ipinfo","action":"ACTION","domains":["domain:ipinfo.io"],"enabled":true}]'

# --- 1 direct ---
log "=== TEST 1/6 direct ==="
apply_routing "direct" "$(payload '["direct","proxy","block"]' "${RULE/ACTION/direct}")"
GOT="$(guest_ip "$TARGET_IP")"
PAGE="$(guest_code "$TARGET_PAGE")"
log "guest ipinfo.io/ip = $GOT (expect WAN $WAN)"
log "guest what-is-my-ip HTTP = $PAGE (expect 200)"
[ "$GOT" = "$WAN" ] || die "direct: expected $WAN got $GOT"
[ "$PAGE" = "200" ] || die "direct page: expected HTTP 200 got $PAGE"
log "PASS direct"

# --- 2 proxy ---
log "=== TEST 2/6 proxy ==="
apply_routing "proxy" "$(payload '["direct","proxy","block"]' "${RULE/ACTION/proxy}")"
GOT="$(guest_ip "$TARGET_IP")"
log "guest ipinfo.io/ip = $GOT (expect VPN $VPN)"
[ "$GOT" = "$VPN" ] || die "proxy: expected $VPN got $GOT"
log "PASS proxy"

# --- 3 block ---
log "=== TEST 3/6 block ==="
apply_routing "block" "$(payload '["direct","proxy","block"]' "${RULE/ACTION/block}")"
CODE="$(guest_code "$TARGET_IP")"
log "guest ipinfo.io/ip HTTP = $CODE (expect blocked)"
is_blocked "$CODE" || die "block: expected blocked got HTTP $CODE"
log "PASS block"

if [ "$QUICK" = "1" ]; then
  log "LAB_E2E_QUICK=1 — skip chain tests"
  log "ALL QUICK TESTS PASSED"
  exit 0
fi

# --- 4 block before direct ---
log "=== TEST 4/6 chain block->direct ==="
apply_routing "block+direct" "$(payload '["block","direct","proxy"]' \
  '[{"id":"b","name":"block","action":"block","domains":["domain:ipinfo.io"],"enabled":true},{"id":"d","name":"direct","action":"direct","domains":["domain:ipinfo.io"],"enabled":true}]')"
CODE="$(guest_code "$TARGET_IP")"
log "HTTP=$CODE (expect blocked)"
is_blocked "$CODE" || die "block-first: expected blocked got $CODE"
log "PASS block->direct"

# --- 5 direct before proxy ---
log "=== TEST 5/6 chain direct->proxy ==="
apply_routing "direct+proxy" "$(payload '["direct","proxy","block"]' \
  '[{"id":"d","name":"direct","action":"direct","domains":["domain:ipinfo.io"],"enabled":true},{"id":"p","name":"proxy","action":"proxy","domains":["domain:ipinfo.io"],"enabled":true}]')"
GOT="$(guest_ip "$TARGET_IP")"
log "ip=$GOT (expect WAN $WAN)"
[ "$GOT" = "$WAN" ] || die "direct-first: expected $WAN got $GOT"
log "PASS direct->proxy"

# --- 6 proxy before direct ---
log "=== TEST 6/6 chain proxy->direct ==="
apply_routing "proxy+direct" "$(payload '["proxy","direct","block"]' \
  '[{"id":"p","name":"proxy","action":"proxy","domains":["domain:ipinfo.io"],"enabled":true},{"id":"d","name":"direct","action":"direct","domains":["domain:ipinfo.io"],"enabled":true}]')"
GOT="$(guest_ip "$TARGET_IP")"
log "ip=$GOT (expect VPN $VPN)"
[ "$GOT" = "$VPN" ] || die "proxy-first: expected $VPN got $GOT"
log "PASS proxy->direct"

log "ALL 6 TESTS PASSED"
REMOTE

rc=$?
echo "────────────────────────────────────────"
if [ "$rc" -eq 0 ]; then
  log "SUCCESS"
else
  log "FAILED (exit $rc)"
fi
exit "$rc"
