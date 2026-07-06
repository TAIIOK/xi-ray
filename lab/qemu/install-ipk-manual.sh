#!/bin/sh
# Manually unpack OpenWrt .ipk archives (opkg SSL is broken before ca-bundle exists).
set -eu

install_ipk_manual() {
  ipk="$1"
  [ -s "$ipk" ] || { echo "missing ipk: $ipk" >&2; return 1; }
  work="$(mktemp -d)"
  trap 'rm -rf "$work"' EXIT INT HUP
  cp "$ipk" "$work/pkg.ipk"
  (
    cd "$work"
    ar x pkg.ipk
    [ -f data.tar.gz ] || exit 1
    tar -xzf data.tar.gz -C /
    if [ -f postinst ]; then
      chmod +x postinst 2>/dev/null || true
      sh ./postinst 2>/dev/null || true
    fi
  )
  rm -rf "$work"
  trap - EXIT INT HUP
}

install_ipk_dir() {
  dir="$1"
  [ -d "$dir" ] || return 0
  for ipk in "$dir"/ca-bundle_*.ipk "$dir"/zlib_*.ipk "$dir"/libbpf1_*.ipk \
    "$dir"/libelf1_*.ipk "$dir"/libmnl0_*.ipk "$dir"/libnl-tiny1_*.ipk \
    "$dir"/ip-full_*.ipk "$dir"/libmbedtls21_*.ipk "$dir"/libnghttp2-14_*.ipk \
    "$dir"/libcurl4_*.ipk "$dir"/curl_*.ipk; do
    [ -f "$ipk" ] || continue
    install_ipk_manual "$ipk"
  done
}
