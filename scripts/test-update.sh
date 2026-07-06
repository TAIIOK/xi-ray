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

(
  cd "$ROOT"
  go build -o "$HOME/panel" ./cmd/panel
)
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

echo "== install-autostart skips init.d on systemd =="
AUTOSTART_LOG="$TMP/autostart.log"
mkdir -p "$TMP/autostart-home/scripts"
cp "$ROOT/scripts/boot-xiaomi-vless.sh" "$TMP/autostart-home/scripts/"
cp "$ROOT/deploy/xiaomi-vless-xray.init" "$TMP/autostart-home/"
XIAOMI_VLESS_USE_SYSTEMD=1 \
  INSTALL_DIR="$TMP/autostart-home" \
  INIT_SRC="$TMP/autostart-home/xiaomi-vless-xray.init" \
  BOOT_SRC="$TMP/autostart-home/scripts/boot-xiaomi-vless.sh" \
  BOOT_SCRIPT="$TMP/autostart-boot.sh" \
  USER_STARTUP="$TMP/autostart-startup.sh" \
  CRON_FILE="$TMP/autostart-crontab" \
  GUARD_SCRIPT="$TMP/autostart-guard.sh" \
  sh "$ROOT/deploy/install-autostart.sh" >"$AUTOSTART_LOG" 2>&1
grep -q "skip xray init.d" "$AUTOSTART_LOG"
grep -q "skip boot init.d" "$AUTOSTART_LOG"

echo "== panel-updater apply (test home) =="
UPD_HOME="$TMP/updater-home"
mkdir -p "$UPD_HOME/updates/staging/deploy" "$UPD_HOME/updates/staging/scripts"
cp "$ROOT/deploy/panel.json.example" "$UPD_HOME/panel.json"
sed -i.bak "s|/mnt/usb-ed49605f|$TMP|g" "$UPD_HOME/panel.json" 2>/dev/null || \
  sed -i '' "s|/mnt/usb-ed49605f|$TMP|g" "$UPD_HOME/panel.json"
(
  cd "$ROOT"
  go build -o "$UPD_HOME/panel" ./cmd/panel
)
chmod +x "$UPD_HOME/panel"
cp "$UPD_HOME/panel" "$UPD_HOME/updates/staging/panel"
cp "$ROOT/scripts/startup_xray_guest.sh" "$UPD_HOME/updates/staging/scripts/"
cp "$ROOT/scripts/boot-xiaomi-vless.sh" "$UPD_HOME/updates/staging/scripts/"
cp "$ROOT/deploy/install-autostart.sh" "$UPD_HOME/updates/staging/deploy/"
cp "$ROOT/deploy/xiaomi-vless-xray.init" "$UPD_HOME/updates/staging/deploy/"
cp "$ROOT/deploy/panel-updater.sh" "$UPD_HOME/panel-updater.sh"
chmod +x "$UPD_HOME/panel-updater.sh"
"$UPD_HOME/panel" -config "$UPD_HOME/panel.json" -update-home "$UPD_HOME" -update-set-phase verified

FAKE_BIN="$TMP/bin"
mkdir -p "$FAKE_BIN"
cat >"$FAKE_BIN/systemctl" <<'EOF'
#!/bin/sh
case "$1" in
  cat)
    case "$2" in
      xiaomi-vless-panel.service) exit 0 ;;
    esac
    exit 1
    ;;
  stop|start|is-active) exit 0 ;;
esac
exit 1
EOF
chmod +x "$FAKE_BIN/systemctl"

PANEL_UPDATE_TEST_HOME="$UPD_HOME" \
  PATH="$FAKE_BIN:$PATH" \
  XIAOMI_VLESS_USE_SYSTEMD=1 \
  sh "$UPD_HOME/panel-updater.sh" apply

test -f "$UPD_HOME/panel.previous"
phase="$("$UPD_HOME/panel" -config "$UPD_HOME/panel.json" -update-home "$UPD_HOME" -update-get-phase)"
test "$phase" = "restarting"

echo "OK: update smoke tests passed"
