#!/bin/sh
# Smoke test for update state machine (local, no router required).
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

HOME="$TMP/xiaomi-vless"
mkdir -p "$HOME/updates/downloads" "$HOME/updates/staging"
cp "$ROOT/deploy/panel.json.example" "$HOME/panel.json"
sed -i.bak "s|/mnt/usb-ed49605f|$TMP|g" "$HOME/panel.json" 2>/dev/null || \
  sed -i '' "s|/mnt/usb-ed49605f|$TMP|g" "$HOME/panel.json"

make -C "$ROOT" build >/dev/null
cp "$ROOT/dist/panel" "$HOME/panel"
chmod +x "$HOME/panel"

echo "== phase get/set =="
"$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-set-phase verified
phase="$("$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-get-phase)"
test "$phase" = "verified"

echo "== apply without staging should fail =="
if "$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-apply 2>/dev/null; then
  echo "expected apply to fail without staging" >&2
  exit 1
fi

echo "== staging + apply =="
cp "$HOME/panel" "$HOME/updates/staging/panel"
mkdir -p "$HOME/updates/staging/scripts"
cp "$ROOT/scripts/startup_xray_guest.sh" "$HOME/updates/staging/scripts/"
"$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-set-phase verified
"$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-apply
test -f "$HOME/panel.previous"

echo "== rollback =="
"$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-rollback
phase="$("$HOME/panel" -config "$HOME/panel.json" -update-home "$HOME" -update-get-phase)"
test "$phase" = "restarting"

echo "OK: update smoke tests passed"
