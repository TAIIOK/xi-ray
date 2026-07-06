#!/bin/sh
# End-to-end panel self-update + rollback on QEMU OpenWrt (procd, no GitHub).
#
# Builds two panel versions locally, deploys the older one, applies the newer
# via panel-updater.sh, then rolls back to panel.previous.
#
# Usage:
#   make qemu-update-test
#   ./lab/qemu/qemu-update-test.sh
#
# Requires: QEMU lab running (make qemu-up)
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init
qemu_require_tools

qemu_is_running || qemu_die "QEMU not running — run: make qemu-up"
qemu_wait_for_ssh

OLD_VERSION="v9.9.8-qemu-old"
NEW_VERSION="v9.9.9-qemu-new"
INSTALL_DIR="/mnt/usb-lab/xiaomi-vless"
GOARCH="arm64"

qemu_log "QEMU OpenWrt update E2E (linux/$GOARCH)"

build_panel() {
  version="$1"
  out="$2"
  commit="$(git -C "$QEMU_REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  ldflags="-s -w \
    -X github.com/taiiok/xiaomi-vless/internal/version.Version=${version} \
    -X github.com/taiiok/xiaomi-vless/internal/version.Commit=${commit} \
    -X github.com/taiiok/xiaomi-vless/internal/version.BuildDate=${build_date}"
  (
    cd "$QEMU_REPO_ROOT"
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
  cp "$QEMU_REPO_ROOT"/scripts/startup_xray_guest.sh \
    "$QEMU_REPO_ROOT"/scripts/xray-guest-iptables.sh \
    "$QEMU_REPO_ROOT"/scripts/xray-guest-iptables-cron.sh \
    "$QEMU_REPO_ROOT"/scripts/xray-guest-sysctl.sh \
    "$QEMU_REPO_ROOT"/scripts/xiaomi-vless-failopen-guard.sh \
    "$QEMU_REPO_ROOT"/scripts/boot-xiaomi-vless.sh \
    "$stage/scripts/"
  cp "$QEMU_REPO_ROOT"/deploy/xiaomi-vless-panel.init \
    "$QEMU_REPO_ROOT"/deploy/xiaomi-vless-xray.init \
    "$QEMU_REPO_ROOT"/deploy/xiaomi-vless-boot.init \
    "$QEMU_REPO_ROOT"/deploy/hotplug-usb-xiaomi-vless.sh \
    "$QEMU_REPO_ROOT"/deploy/install-autostart.sh \
    "$QEMU_REPO_ROOT"/deploy/install-common.sh \
    "$QEMU_REPO_ROOT"/deploy/panel-updater.sh \
    "$QEMU_REPO_ROOT"/deploy/panel.json.example \
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
  qemu_log "bundle ready: $out_tar"
}

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT HUP

OLD_PANEL="$TMP/panel-old"
NEW_PANEL="$TMP/panel-new"
BUNDLE="$TMP/update-bundle.tar.gz"

qemu_log "building $OLD_VERSION panel..."
build_panel "$OLD_VERSION" "$OLD_PANEL"
qemu_log "building $NEW_VERSION panel + bundle..."
build_panel "$NEW_VERSION" "$NEW_PANEL"
build_bundle "$NEW_PANEL" "$NEW_VERSION" "$BUNDLE"

qemu_log "deploying $OLD_VERSION to guest..."
qemu_ssh "INSTALL='$INSTALL_DIR'
[ -x \"\$INSTALL/panel\" ] && cp \"\$INSTALL/panel\" \"\$INSTALL/panel.previous\"
/etc/init.d/xiaomi-vless-panel stop 2>/dev/null || killall panel 2>/dev/null || true
sleep 1"
qemu_scp "$OLD_PANEL" "$(qemu_ssh_target):/tmp/panel-old"
qemu_ssh "cp /tmp/panel-old '$INSTALL_DIR/panel' && chmod 755 '$INSTALL_DIR/panel'
/etc/init.d/xiaomi-vless-panel restart
i=0
while [ \"\$i\" -lt 30 ]; do
  wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null && exit 0
  i=\$((i + 1)); sleep 1
done
exit 1"

qemu_scp "$BUNDLE" "$(qemu_ssh_target):/tmp/qemu-update-bundle.tar.gz"
qemu_scp "${QEMU_REPO_ROOT}/deploy/panel-updater.sh" "$(qemu_ssh_target):/tmp/panel-updater.sh"

qemu_log "running panel-updater apply on guest..."
qemu_ssh "set -eu
INSTALL_DIR='$INSTALL_DIR'
OLD_VERSION='$OLD_VERSION'
NEW_VERSION='$NEW_VERSION'

cp /tmp/panel-updater.sh \"\$INSTALL_DIR/panel-updater.sh\"
chmod +x \"\$INSTALL_DIR/panel-updater.sh\"

mkdir -p \"\$INSTALL_DIR/updates/staging\" \"\$INSTALL_DIR/updates/downloads\"
rm -rf \"\$INSTALL_DIR/updates/staging\"/*
tar -xzf /tmp/qemu-update-bundle.tar.gz -C \"\$INSTALL_DIR/updates/staging\"

\"\$INSTALL_DIR/panel\" -config \"\$INSTALL_DIR/panel.json\" \
  -update-home \"\$INSTALL_DIR\" -update-set-phase verified

if ! sh \"\$INSTALL_DIR/panel-updater.sh\" apply; then
  echo 'panel-updater apply failed' >&2
  tail -50 \"\$INSTALL_DIR/panel-update.log\" 2>/dev/null || true
  exit 1
fi

phase=\"\$(\"\$INSTALL_DIR/panel\" -config \"\$INSTALL_DIR/panel.json\" \
  -update-home \"\$INSTALL_DIR\" -update-get-phase)\"
case \"\$phase\" in
  restarting|health_check|idle|completed) ;;
  *)
    echo \"unexpected phase after apply: \$phase\" >&2
    tail -50 \"\$INSTALL_DIR/panel-update.log\" 2>/dev/null || true
    exit 1
    ;;
esac

i=0
while [ \"\$i\" -lt 30 ]; do
  wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null && break
  i=\$((i + 1)); sleep 1
done
[ \"\$i\" -lt 30 ] || { echo 'panel HTTP did not come back after apply' >&2; exit 1; }

for svc in xiaomi-vless-panel xiaomi-vless-xray xiaomi-vless-boot; do
  [ -x \"/etc/init.d/\$svc\" ] || { echo \"missing init.d: \$svc\" >&2; exit 1; }
done

[ -f \"\$INSTALL_DIR/panel.previous\" ] || { echo 'panel.previous missing after apply' >&2; exit 1; }

got=\"\$(\"\$INSTALL_DIR/panel\" -version 2>/dev/null | awk '{print \$2}')\"
got=\"\${got#v}\"
want=\"\${NEW_VERSION#v}\"
[ \"\$got\" = \"\$want\" ] || { echo \"version mismatch after apply: got=\$got want=\$want\" >&2; exit 1; }

echo \"OK apply: version \$got, procd init.d present\"
"

qemu_log "running panel-updater rollback on guest..."
qemu_ssh "set -eu
INSTALL_DIR='$INSTALL_DIR'
OLD_VERSION='$OLD_VERSION'

if ! sh \"\$INSTALL_DIR/panel-updater.sh\" rollback; then
  echo 'panel-updater rollback failed' >&2
  tail -50 \"\$INSTALL_DIR/panel-update.log\" 2>/dev/null || true
  exit 1
fi

phase=\"\$(\"\$INSTALL_DIR/panel\" -config \"\$INSTALL_DIR/panel.json\" \
  -update-home \"\$INSTALL_DIR\" -update-get-phase)\"
[ \"\$phase\" = rolled_back ] || { echo \"unexpected phase after rollback: \$phase\" >&2; exit 1; }

i=0
while [ \"\$i\" -lt 30 ]; do
  wget -q -O /dev/null http://127.0.0.1:7777/login 2>/dev/null && break
  i=\$((i + 1)); sleep 1
done
[ \"\$i\" -lt 30 ] || { echo 'panel HTTP did not come back after rollback' >&2; exit 1; }

got=\"\$(\"\$INSTALL_DIR/panel\" -version 2>/dev/null | awk '{print \$2}')\"
got=\"\${got#v}\"
want=\"\${OLD_VERSION#v}\"
[ \"\$got\" = \"\$want\" ] || { echo \"version mismatch after rollback: got=\$got want=\$want\" >&2; exit 1; }

/etc/init.d/xiaomi-vless-xray restart 2>/dev/null || true

echo \"OK rollback: version \$got, phase rolled_back\"
"

qemu_log "QEMU OpenWrt update + rollback E2E passed"

# Restore production panel binary (test leaves lab-tagged build on USB).
qemu_log "restoring production panel after update test..."
PANEL_BIN="$(qemu_build_panel)"
qemu_ssh "/etc/init.d/xiaomi-vless-panel stop 2>/dev/null || killall panel 2>/dev/null || true; sleep 1"
qemu_scp "$PANEL_BIN" "$(qemu_ssh_target):/tmp/panel-linux"
qemu_scp "${QEMU_REPO_ROOT}/scripts/startup_xray_guest.sh" "$(qemu_ssh_target):/mnt/usb-lab/xiaomi-vless/startup_xray_guest.sh"
qemu_ssh "chmod 755 /tmp/panel-linux /mnt/usb-lab/xiaomi-vless/startup_xray_guest.sh
INSTALL='$INSTALL_DIR'
[ -x \"\$INSTALL/panel\" ] && cp \"\$INSTALL/panel\" \"\$INSTALL/panel.previous\"
cp /tmp/panel-linux \"\$INSTALL/panel\" && chmod 755 \"\$INSTALL/panel\"
rm -f /data/xiaomi-vless-failopen 2>/dev/null || true
/etc/init.d/xiaomi-vless-panel restart
/etc/init.d/xiaomi-vless-xray restart
sleep 5
wget -q -O /dev/null http://127.0.0.1:7777/login
pidof xray >/dev/null
"

qemu_print_panel_url
