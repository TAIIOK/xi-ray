#!/bin/sh
# Build release bundle: panel + scripts + deploy + manifest.json
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-dev}"
VERSION="${VERSION#v}"
TAG="v${VERSION}"
OUT_DIR="${ROOT}/dist/release"
STAGE="${OUT_DIR}/staging"
ARCHIVE="${OUT_DIR}/xiaomi-vless-${TAG}-linux-arm64.tar.gz"

rm -rf "$STAGE"
mkdir -p "$STAGE/scripts" "$STAGE/deploy" "$OUT_DIR"

make build-arm64 VERSION="v${VERSION}"

cp dist/panel-linux-arm64 "$STAGE/panel"
chmod +x "$STAGE/panel"

sha256sum "$STAGE/panel" | awk '{print $1}' > "$STAGE/panel.sha256"

cp scripts/startup_xray_guest.sh scripts/xray-guest-iptables.sh scripts/xray-guest-iptables-cron.sh scripts/xray-guest-sysctl.sh scripts/xiaomi-vless-failopen-guard.sh scripts/boot-xiaomi-vless.sh "$STAGE/scripts/"
cp deploy/xiaomi-vless-panel.init deploy/xiaomi-vless-xray.init deploy/xiaomi-vless-boot.init deploy/hotplug-usb-xiaomi-vless.sh deploy/install-autostart.sh deploy/install-common.sh deploy/panel-updater.sh deploy/panel.json.example "$STAGE/deploy/"
cp deploy/install-from-release.sh "$STAGE/install.sh"
cp deploy/uninstall.sh "$STAGE/uninstall.sh"
cp deploy/fix-autostart.sh "$STAGE/deploy/"
chmod +x "$STAGE/install.sh" "$STAGE/uninstall.sh" "$STAGE/deploy/"*.sh "$STAGE/scripts/"*.sh 2>/dev/null || true

python3 - "$STAGE" "$VERSION" "$TAG" <<'PY'
import hashlib, json, os, sys
from datetime import datetime, timezone

stage, version, tag = sys.argv[1], sys.argv[2], sys.argv[3]
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
    "min_config_version": 2,
    "platform": "linux/arm64",
    "released_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "assets": assets,
    "notes_url": f"https://github.com/TAIIOK/xi-ray/releases/tag/{tag}",
}
with open(os.path.join(stage, "manifest.json"), "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY

tar -czf "$ARCHIVE" -C "$STAGE" .
sha256sum "$ARCHIVE" | awk '{print $1}' > "${ARCHIVE}.sha256"

echo "Bundle: $ARCHIVE"
echo "SHA256: $(cat "${ARCHIVE}.sha256")"
