#!/bin/sh
# One-command install: download latest release from GitHub and run install.sh
#
#   curl -fsSL https://raw.githubusercontent.com/TAIIOK/xi-ray/main/scripts/quick-install.sh | sh
#
# Or:
#   wget -O - https://raw.githubusercontent.com/TAIIOK/xi-ray/main/scripts/quick-install.sh | sh
set -eu

REPO="${PANEL_UPDATE_REPO:-TAIIOK/xi-ray}"
WORK="${TMPDIR:-/tmp}/xiaomi-vless-install.$$"
ARCH="" 
URL=""

log() { printf '[quick-install] %s\n' "$*"; }
die() { printf '[quick-install] ERROR: %s\n' "$*" >&2; exit 1; }

cleanup() { rm -rf "$WORK" 2>/dev/null || true; }
trap cleanup EXIT

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

fetch_latest_url() {
  if command -v curl >/dev/null 2>&1; then
    json="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")" || die "failed to fetch release info"
  elif command -v wget >/dev/null 2>&1; then
    json="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest")" || die "failed to fetch release info"
  else
    die "curl or wget required"
  fi

  tag="$(printf '%s' "$json" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/')"
  [ -n "$tag" ] || die "could not parse release tag"

  asset="xiaomi-vless-${tag}-linux-arm64.tar.gz"
  URL="$(printf '%s' "$json" | tr ',' '\n' | grep -F "$asset" | grep browser_download_url | head -1 | sed 's/.*"\(https[^"]*\)".*/\1/')"
  [ -n "$URL" ] || die "release $tag missing asset $asset"
  ARCH="$asset"
  log "latest release: $tag"
}

download_and_extract() {
  mkdir -p "$WORK"
  dest="$WORK/$ARCH"
  log "downloading $URL"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$dest"
  else
    wget -qO "$dest" "$URL"
  fi
  log "extracting..."
  tar xzf "$dest" -C "$WORK"
}

run_installer() {
  if [ -f "$WORK/install.sh" ]; then
    sh "$WORK/install.sh"
    return 0
  fi
  installer="$(find "$WORK" -maxdepth 2 -name install.sh -type f 2>/dev/null | head -1)"
  [ -n "$installer" ] || die "install.sh not found in extracted bundle"
  sh "$installer"
}

need_cmd tar
need_cmd grep
need_cmd sed

fetch_latest_url
download_and_extract
run_installer
