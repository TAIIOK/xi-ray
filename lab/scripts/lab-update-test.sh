#!/bin/sh
# End-to-end panel self-update test on lab VM (systemd, no GitHub).
#
# Builds two panel versions locally, deploys the older one, applies the newer
# via panel-updater.sh (same path as Settings → Обновить), then asserts panel
# is healthy and OpenWrt init.d hooks were not installed.
#
# Usage:
#   make lab-update-test
#   ./lab/scripts/lab-update-test.sh
#
# Requires: lab VM running (make lab-up)
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/scripts/lab-common.sh
. "${SCRIPT_DIR}/lab-common.sh"
lab_common_init

OLD_VERSION="v9.9.8-lab-old"
NEW_VERSION="v9.9.9-lab-new"
INSTALL_DIR="/mnt/usb-lab/xiaomi-vless"

lab_require_multipass
lab_ensure_vm_running

GOARCH="$(lab_detect_vm_goarch)"
lab_log "lab update E2E on $LAB_VM_NAME (linux/$GOARCH)"

build_panel() {
  version="$1"
  out="$2"
  commit="$(git -C "$LAB_REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  ldflags="-s -w \
    -X github.com/taiiok/xiaomi-vless/internal/version.Version=${version} \
    -X github.com/taiiok/xiaomi-vless/internal/version.Commit=${commit} \
    -X github.com/taiiok/xiaomi-vless/internal/version.BuildDate=${build_date}"
  (
    cd "$LAB_REPO_ROOT"
    GOOS=linux GOARCH="$GOARCH" CGO_ENABLED=0 go build \
      -ldflags "$ldflags" \
      -o "$out" ./cmd/panel
  )
}

bundle_sha256() {
  python3 - "$1" <<'PY'
import hashlib, sys
with open(sys.argv[1], "rb") as f:
    print(hashlib.sha256(f.read()).hexdigest())
PY
}

build_bundle() {
  panel_bin="$1"
  version="$2"
  out_tar="$3"
  stage="$(mktemp -d)"
  trap 'rm -rf "$stage"' EXIT INT HUP

  mkdir -p "$stage/scripts" "$stage/deploy"
  cp "$panel_bin" "$stage/panel"
  chmod +x "$stage/panel"
  cp "$LAB_REPO_ROOT"/scripts/startup_xray_guest.sh \
    "$LAB_REPO_ROOT"/scripts/xray-guest-iptables.sh \
    "$LAB_REPO_ROOT"/scripts/xray-guest-iptables-cron.sh \
    "$LAB_REPO_ROOT"/scripts/xray-guest-sysctl.sh \
    "$LAB_REPO_ROOT"/scripts/xiaomi-vless-failopen-guard.sh \
    "$LAB_REPO_ROOT"/scripts/boot-xiaomi-vless.sh \
    "$stage/scripts/"
  cp "$LAB_REPO_ROOT"/deploy/xiaomi-vless-panel.init \
    "$LAB_REPO_ROOT"/deploy/xiaomi-vless-xray.init \
    "$LAB_REPO_ROOT"/deploy/xiaomi-vless-boot.init \
    "$LAB_REPO_ROOT"/deploy/hotplug-usb-xiaomi-vless.sh \
    "$LAB_REPO_ROOT"/deploy/install-autostart.sh \
    "$LAB_REPO_ROOT"/deploy/install-common.sh \
    "$LAB_REPO_ROOT"/deploy/panel-updater.sh \
    "$LAB_REPO_ROOT"/deploy/panel.json.example \
    "$stage/deploy/"
  chmod +x "$stage/deploy/"*.sh "$stage/scripts/"*.sh 2>/dev/null || true

  panel_sum="$(bundle_sha256 "$stage/panel")"
  version="${version#v}"
  python3 - "$stage" "$version" "$GOARCH" "$panel_sum" <<'PY'
import hashlib, json, os, sys
from datetime import datetime, timezone

stage, version, goarch, panel_sum = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
assets = {}
for root, _, files in os.walk(stage):
    for name in files:
        if name == "manifest.json":
            continue
        path = os.path.join(root, name)
        rel = os.path.relpath(path, stage).replace("\\", "/")
        with open(path, "rb") as f:
            digest = hashlib.sha256(f.read()).hexdigest()
        assets[rel] = {"path": rel, "sha256": digest}

manifest = {
    "version": version,
    "min_config_version": 1,
    "platform": f"linux/{goarch}",
    "released_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "assets": assets,
    "notes_url": f"https://example.com/{version}",
}
with open(os.path.join(stage, "manifest.json"), "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY

  tar -czf "$out_tar" -C "$stage" .
  lab_log "bundle ready: $out_tar"
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT HUP

OLD_PANEL="$TMP/panel-old"
NEW_PANEL="$TMP/panel-new"
BUNDLE="$TMP/update-bundle.tar.gz"

lab_log "building $OLD_VERSION panel..."
build_panel "$OLD_VERSION" "$OLD_PANEL"
lab_log "building $NEW_VERSION panel + bundle..."
build_panel "$NEW_VERSION" "$NEW_PANEL"
build_bundle "$NEW_PANEL" "$NEW_VERSION" "$BUNDLE"

lab_log "deploying $OLD_VERSION to VM..."
lab_deploy_panel_only "$OLD_PANEL"

multipass transfer "$BUNDLE" "${LAB_VM_NAME}:/tmp/lab-update-bundle.tar.gz"
multipass transfer "${LAB_REPO_ROOT}/deploy/panel-updater.sh" "${LAB_VM_NAME}:/tmp/panel-updater.sh"

lab_log "running panel-updater apply on VM..."
multipass exec "$LAB_VM_NAME" -- sudo env \
  OLD_VERSION="$OLD_VERSION" \
  NEW_VERSION="$NEW_VERSION" \
  INSTALL_DIR="$INSTALL_DIR" \
  sh -s <<'REMOTE'
set -eu
INSTALL_DIR="${INSTALL_DIR:-/mnt/usb-lab/xiaomi-vless}"
OLD_VERSION="${OLD_VERSION:-v9.9.8-lab-old}"
NEW_VERSION="${NEW_VERSION:-v9.9.9-lab-new}"

cp /tmp/panel-updater.sh "$INSTALL_DIR/panel-updater.sh"
chmod +x "$INSTALL_DIR/panel-updater.sh"

mkdir -p "$INSTALL_DIR/updates/staging" "$INSTALL_DIR/updates/downloads"
rm -rf "$INSTALL_DIR/updates/staging"/*
tar -xzf /tmp/lab-update-bundle.tar.gz -C "$INSTALL_DIR/updates/staging"

"$INSTALL_DIR/panel" -config "$INSTALL_DIR/panel.json" \
  -update-home "$INSTALL_DIR" -update-set-phase verified

if ! sh "$INSTALL_DIR/panel-updater.sh" apply; then
  echo "panel-updater apply failed" >&2
  tail -50 "$INSTALL_DIR/panel-update.log" 2>/dev/null || true
  exit 1
fi

phase="$("$INSTALL_DIR/panel" -config "$INSTALL_DIR/panel.json" \
  -update-home "$INSTALL_DIR" -update-get-phase)"
if [ "$phase" != "restarting" ] && [ "$phase" != "health_check" ] && [ "$phase" != "idle" ] && [ "$phase" != "completed" ]; then
  echo "unexpected phase after apply: $phase" >&2
  tail -50 "$INSTALL_DIR/panel-update.log" 2>/dev/null || true
  exit 1
fi

i=0
while [ "$i" -lt 30 ]; do
  if curl -fsS --connect-timeout 2 http://127.0.0.1:7777/login >/dev/null 2>&1; then
    break
  fi
  i=$((i + 1))
  sleep 1
done
if [ "$i" -ge 30 ]; then
  echo "panel HTTP did not come back" >&2
  systemctl status xiaomi-vless-panel.service --no-pager || true
  journalctl -u xiaomi-vless-panel -n 30 --no-pager || true
  exit 1
fi

if ! systemctl is-active --quiet xiaomi-vless-panel.service; then
  echo "xiaomi-vless-panel.service not active" >&2
  exit 1
fi

for legacy in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot; do
  if [ -f "/etc/init.d/$legacy" ]; then
    echo "incompatible init.d still present: /etc/init.d/$legacy" >&2
    exit 1
  fi
done

if [ ! -f "$INSTALL_DIR/panel.previous" ]; then
  echo "panel.previous missing after apply" >&2
  exit 1
fi

got="$("$INSTALL_DIR/panel" -version 2>/dev/null | awk '{print $2}')"
got="${got#v}"
want="${NEW_VERSION#v}"
if [ "$got" != "$want" ]; then
  echo "panel version mismatch: got=$got want=$want" >&2
  exit 1
fi

echo "OK: lab update E2E passed (version $got, systemd active, no init.d)"
REMOTE

lab_log "lab update E2E passed"
lab_print_panel_url
