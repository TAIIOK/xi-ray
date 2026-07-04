#!/bin/sh
# Integration test: routing API save, empty-settings protection, apply → xray config.
#
# Usage: ./lab/scripts/lab-routing-test.sh
# Requires: running lab VM (make lab-up), curl, python3
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/scripts/lab-common.sh
. "${SCRIPT_DIR}/lab-common.sh"
lab_common_init
lab_require_multipass
lab_ensure_vm_running

VM_IP="$(lab_vm_ip)"
BASE="http://${VM_IP}:7777"
PANEL_JSON="/mnt/usb-lab/xiaomi-vless/panel.json"
PANEL_BACKUP="/tmp/xiaomi-vless-panel.routing-test.bak"
XRAY_CONFIG="/mnt/usb-lab/xray/config.json"
TEST_PASS="lab-routing-test"
COOKIE_JAR="$(mktemp)"
TMP_SETTINGS="$(mktemp)"
SET_PASS_BIN="${LAB_REPO_ROOT}/dist/lab-set-password-linux-$(lab_detect_vm_goarch)"
trap 'rm -f "$COOKIE_JAR" "$TMP_SETTINGS"; multipass exec "$LAB_VM_NAME" -- sudo rm -f "$PANEL_BACKUP" 2>/dev/null || true' EXIT

lab_log "routing integration test on $LAB_VM_NAME ($VM_IP)"

lab_log "building lab-set-password helper"
GOARCH="$(lab_detect_vm_goarch)"
mkdir -p "${LAB_REPO_ROOT}/dist"
(
  cd "$LAB_REPO_ROOT"
  GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build -o "$SET_PASS_BIN" ./lab/cmd/lab-set-password
)

lab_log "backup panel.json on VM and set temporary test password"
multipass exec "$LAB_VM_NAME" -- sudo cp "$PANEL_JSON" "$PANEL_BACKUP"
multipass transfer "$SET_PASS_BIN" "${LAB_VM_NAME}:/tmp/lab-set-password"
multipass exec "$LAB_VM_NAME" -- sudo chmod +x /tmp/lab-set-password
multipass exec "$LAB_VM_NAME" -- sudo /tmp/lab-set-password "$PANEL_JSON" "$TEST_PASS"
multipass exec "$LAB_VM_NAME" -- sudo rm -f /tmp/lab-set-password
multipass exec "$LAB_VM_NAME" -- sudo systemctl restart xiaomi-vless-panel
sleep 2

restore_panel() {
  multipass exec "$LAB_VM_NAME" -- sudo cp "$PANEL_BACKUP" "$PANEL_JSON"
  multipass exec "$LAB_VM_NAME" -- sudo systemctl restart xiaomi-vless-panel
}
trap 'restore_panel 2>/dev/null || true; rm -f "$COOKIE_JAR" "$TMP_SETTINGS"; multipass exec "$LAB_VM_NAME" -- sudo rm -f "$PANEL_BACKUP" 2>/dev/null || true' EXIT

login() {
  curl -sS -c "$COOKIE_JAR" -b "$COOKIE_JAR" -X POST "$BASE/login" \
    -H 'Content-Type: application/x-www-form-urlencoded' \
    -d "username=admin&password=${TEST_PASS}" -o /dev/null -w '%{http_code}'
}

CODE="$(login)"
if [ "$CODE" != "303" ]; then
  lab_die "login failed with HTTP $CODE"
fi

api() {
  method=$1
  path=$2
  body=${3:-}
  if [ -n "$body" ]; then
    curl -sS -b "$COOKIE_JAR" -X "$method" "$BASE$path" \
      -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -b "$COOKIE_JAR" -X "$method" "$BASE$path"
  fi
}

lab_log "save custom routing rules"
RESP="$(api PUT /api/routing '{
  "apply": false,
  "routing": {
    "domain_strategy": "IPIfNonMatch",
    "rule_order": ["direct", "block", "proxy"],
    "default_guest_action": "proxy",
    "bypass_private": true,
    "bypass_vpn_hosts": true,
    "rules": [
      {"id":"cn","name":"CN sites","action":"direct","domains":["geosite:cn"],"enabled":true},
      {"id":"ads","name":"Block ads","action":"block","domains":["geosite:category-ads-all"],"enabled":true}
    ]
  }
}')"
echo "$RESP" | python3 -c "import json,sys; d=json.load(sys.stdin); assert d.get('ok') is True, d"

lab_log "save settings with empty routing payload (must not reset rules)"
api GET /api/settings > "$TMP_SETTINGS"
EMPTY_SETTINGS="$(python3 - "$TMP_SETTINGS" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    settings = json.load(f)
settings["routing"] = {}
print(json.dumps(settings))
PY
)"
api PUT /api/settings "$EMPTY_SETTINGS" > /dev/null

RESP="$(api GET /api/routing)"
echo "$RESP" | python3 -c "
import json, sys
d = json.load(sys.stdin)
rules = d.get('routing', {}).get('rules', [])
assert len(rules) == 2, rules
assert rules[0]['domains'] == ['geosite:cn'], rules
print('routing preserved after empty settings save')
"

lab_log "apply routing and verify xray config.json"
RESP="$(api PUT /api/routing '{
  "apply": true,
  "routing": {
    "domain_strategy": "IPIfNonMatch",
    "rule_order": ["direct", "block", "proxy"],
    "default_guest_action": "proxy",
    "bypass_private": true,
    "bypass_vpn_hosts": true,
    "rules": [
      {"id":"cn","name":"CN sites","action":"direct","domains":["geosite:cn"],"enabled":true},
      {"id":"ads","name":"Block ads","action":"block","domains":["geosite:category-ads-all"],"enabled":true}
    ]
  }
}')"
echo "$RESP" | python3 -c "
import json, sys
d = json.load(sys.stdin)
assert d.get('ok') is True, d
apply = d.get('apply') or {}
assert apply.get('ok') is True, apply
print('apply ok:', apply.get('message',''))
"

multipass exec "$LAB_VM_NAME" -- sudo python3 - "$XRAY_CONFIG" <<'PY'
import json, sys
rules = json.load(open(sys.argv[1], encoding="utf-8"))["routing"]["rules"]
hits = []
for i, r in enumerate(rules):
    dom = r.get("domain") or []
    if any("geosite:cn" in x for x in dom):
        hits.append(("cn", i, r.get("outboundTag"), dom))
    if any("category-ads" in x for x in dom):
        hits.append(("ads", i, r.get("outboundTag"), dom))
assert len(hits) >= 2, f"expected cn+ads rules in xray config, got {hits}"
cn = next(h for h in hits if h[0] == "cn")
ads = next(h for h in hits if h[0] == "ads")
assert cn[1] < ads[1], f"direct rule must precede block: cn@{cn[1]} ads@{ads[1]}"
assert cn[2] == "direct", cn
assert ads[2] == "block", ads
print("xray config routing ok: cn@direct before ads@block")
PY

lab_log "all routing integration checks passed"
