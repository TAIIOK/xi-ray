# Shared helpers for QEMU OpenWrt lab scripts.
# shellcheck shell=sh

qemu_common_init() {
  QEMU_SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
  QEMU_REPO_ROOT="$(CDPATH= cd -- "$QEMU_SCRIPT_DIR/../.." && pwd)"
  QEMU_LAB_NAME="${QEMU_LAB_NAME:-xiaomi-vless-qemu}"
  QEMU_OWRT_VERSION="${QEMU_OWRT_VERSION:-24.10.0}"
  QEMU_MEMORY="${QEMU_MEMORY:-1024}"
  QEMU_CPUS="${QEMU_CPUS:-2}"
  QEMU_SSH_PORT="${QEMU_SSH_PORT:-2222}"
  QEMU_PANEL_PORT="${QEMU_PANEL_PORT:-7777}"
  QEMU_GUEST_IP="${QEMU_GUEST_IP:-192.168.33.10}"
  QEMU_RUNTIME_DIR="${QEMU_RUNTIME_DIR:-${QEMU_REPO_ROOT}/lab/qemu/runtime}"
  QEMU_IMAGES_DIR="${QEMU_IMAGES_DIR:-${QEMU_REPO_ROOT}/lab/qemu/images}"
  QEMU_USB_DISK="${QEMU_USB_DISK:-${QEMU_IMAGES_DIR}/usb-lab.qcow2}"
  QEMU_PIDFILE="${QEMU_PIDFILE:-${QEMU_RUNTIME_DIR}/qemu.pid}"
  QEMU_SSH_KNOWN_HOSTS="${QEMU_SSH_KNOWN_HOSTS:-${QEMU_RUNTIME_DIR}/known_hosts}"
  # OpenWrt QEMU docs: LAN 192.168.1.0/24, WAN 192.0.2.0/24 (internet via eth1).
  QEMU_LAN_NET="${QEMU_LAN_NET:-192.168.1.0/24}"
  QEMU_WAN_NET="${QEMU_WAN_NET:-192.0.2.0/24}"
  QEMU_LAN_GUEST_IP="${QEMU_LAN_GUEST_IP:-192.168.1.1}"
  QEMU_DOWNLOAD_BASE="https://downloads.openwrt.org/releases/${QEMU_OWRT_VERSION}/targets/armsr/armv8"
  QEMU_IMAGE_GZ="openwrt-${QEMU_OWRT_VERSION}-armsr-armv8-generic-ext4-combined-efi.img.gz"
  QEMU_IMAGE="${QEMU_IMAGES_DIR}/openwrt-${QEMU_OWRT_VERSION}-armsr-armv8.img"
  QEMU_KERNEL_URL="${QEMU_DOWNLOAD_BASE}/openwrt-${QEMU_OWRT_VERSION}-armsr-armv8-generic-kernel.bin"
  QEMU_KERNEL="${QEMU_IMAGES_DIR}/openwrt-kernel.bin"
  QEMU_ROOTFS="${QEMU_IMAGES_DIR}/openwrt-rootfs-only.img"
  QEMU_ROOTFS_GZ="openwrt-${QEMU_OWRT_VERSION}-armsr-armv8-generic-ext4-rootfs.img.gz"
  QEMU_GUEST_IP_FILE="${QEMU_RUNTIME_DIR}/guest-ip"
  # vmnet needs sudo on macOS; slirp + hostfwd works without privileges.
  QEMU_USE_VMNET="${QEMU_USE_VMNET:-0}"
  if [ "$(uname -s)" = Darwin ]; then
    QEMU_VMNET_START="${QEMU_VMNET_START:-192.168.64.10}"
    QEMU_VMNET_END="${QEMU_VMNET_END:-192.168.64.254}"
  fi
  QEMU_USB_MOUNT="/mnt/usb-lab"
  QEMU_INSTALL_DIR="${QEMU_USB_MOUNT}/xiaomi-vless"
}

qemu_log() { printf '==> %s\n' "$*" >&2; }
qemu_die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

qemu_require_tools() {
  command -v qemu-system-aarch64 >/dev/null 2>&1 || \
    qemu_die "qemu-system-aarch64 not found — install: brew install qemu"
  command -v curl >/dev/null 2>&1 || qemu_die "curl not found"
  command -v ssh >/dev/null 2>&1 || qemu_die "ssh not found"
  command -v scp >/dev/null 2>&1 || qemu_die "scp not found"
}

qemu_host() {
  if [ "$QEMU_USE_VMNET" = "1" ] && [ -f "$QEMU_GUEST_IP_FILE" ]; then
    cat "$QEMU_GUEST_IP_FILE"
    return 0
  fi
  printf '127.0.0.1'
}

qemu_ssh_port() {
  if [ "$QEMU_USE_VMNET" = "1" ]; then
    printf '22'
  else
    printf '%s' "$QEMU_SSH_PORT"
  fi
}

qemu_ssh_target() {
  printf 'root@%s' "$(qemu_host)"
}

qemu_ssh_opts() {
  printf '%s' "-F /dev/null -o StrictHostKeyChecking=no -o UserKnownHostsFile=${QEMU_SSH_KNOWN_HOSTS} -o LogLevel=ERROR -o ConnectTimeout=5 -o ConnectionAttempts=1 -o BatchMode=yes"
}

qemu_ssh_ready() {
  ssh $(qemu_ssh_opts) -p "$(qemu_ssh_port)" "$(qemu_ssh_target)" "echo qemu-ssh-ready" 2>/dev/null | grep -q qemu-ssh-ready
}

qemu_discover_guest_ip() {
  qemu_log "discovering guest IP on vmnet (${QEMU_VMNET_START}-${QEMU_VMNET_END})..."
  start="${QEMU_VMNET_START##*.}"
  end="${QEMU_VMNET_END##*.}"
  prefix="${QEMU_VMNET_START%.*}"
  i="$start"
  while [ "$i" -le "$end" ]; do
    ip="${prefix}.${i}"
    if ssh $(qemu_ssh_opts) -o ConnectTimeout=1 -p 22 "root@${ip}" "echo qemu-ssh-ready" 2>/dev/null | grep -q qemu-ssh-ready; then
      printf '%s' "$ip" > "$QEMU_GUEST_IP_FILE"
      qemu_log "guest IP: $ip"
      return 0
    fi
    i=$((i + 1))
  done
  return 1
}

qemu_wait_for_ssh() {
  mkdir -p "$QEMU_RUNTIME_DIR"
  touch "$QEMU_SSH_KNOWN_HOSTS"
  rm -f "$QEMU_GUEST_IP_FILE"

  if [ "$QEMU_USE_VMNET" = "1" ]; then
    qemu_log "waiting for OpenWrt on vmnet (first boot ~20-40s)..."
    sleep 20
    i=0
    while [ "$i" -lt 40 ]; do
      if ! qemu_is_running; then
        qemu_die "QEMU exited — see ${QEMU_RUNTIME_DIR}/qemu-stdout.log"
      fi
      if qemu_discover_guest_ip; then
        return 0
      fi
      i=$((i + 1))
      if [ $((i % 5)) -eq 0 ]; then
        qemu_log "still scanning vmnet... ${i}/40"
      fi
      sleep 2
    done
    qemu_die "guest IP not found on vmnet — try: make qemu-down && make qemu-up"
  fi

  qemu_log "waiting for OpenWrt SSH on 127.0.0.1:${QEMU_SSH_PORT}..."
  sleep 15
  i=0
  while [ "$i" -lt 60 ]; do
    if ! qemu_is_running; then
      qemu_die "QEMU exited — see ${QEMU_RUNTIME_DIR}/qemu-stdout.log"
    fi
    if qemu_ssh_ready; then
      qemu_log "SSH ready"
      return 0
    fi
    i=$((i + 1))
    if [ $((i % 5)) -eq 0 ]; then
      qemu_log "still waiting... ${i}/60"
    fi
    sleep 2
  done
  qemu_die "SSH not ready after 2 min"
}

qemu_ssh() {
  # shellcheck disable=SC2048
  ssh $(qemu_ssh_opts) -p "$(qemu_ssh_port)" "$(qemu_ssh_target)" "$@"
}

qemu_scp() {
  # OpenWrt dropbear has no /usr/libexec/sftp-server — use legacy scp protocol.
  # shellcheck disable=SC2048
  scp -O $(qemu_ssh_opts) -P "$(qemu_ssh_port)" "$@"
}

qemu_is_running() {
  if [ -f "$QEMU_PIDFILE" ]; then
    pid="$(cat "$QEMU_PIDFILE" 2>/dev/null || true)"
    if [ -n "$pid" ]; then
      if kill -0 "$pid" 2>/dev/null; then
        return 0
      fi
      if sudo kill -0 "$pid" 2>/dev/null; then
        return 0
      fi
    fi
    rm -f "$QEMU_PIDFILE"
  fi
  pgrep -f "qemu-system-aarch64.*${QEMU_LAB_NAME}" >/dev/null 2>&1
}

qemu_start_vm() {
  qemu_log "starting QEMU OpenWrt (${QEMU_OWRT_VERSION}, ${QEMU_CPUS}c/${QEMU_MEMORY}M)"

  # https://openwrt.org/docs/guide-user/virtualization/qemu — Apple Silicon section
  qemu_cpu="cortex-a72"
  qemu_accel=""
  if [ "$(uname -s)" = Darwin ]; then
    qemu_cpu="host"
    qemu_accel="-accel hvf"
  fi

  if [ "$QEMU_USE_VMNET" = "1" ]; then
    # shellcheck disable=SC2086
    set -- \
      -name "$QEMU_LAB_NAME" \
      -machine virt,highmem=off \
      $qemu_accel \
      -cpu "$qemu_cpu" \
      -smp "$QEMU_CPUS" \
      -m "$QEMU_MEMORY" \
      -kernel "$QEMU_KERNEL" \
      -append "root=/dev/vda rootwait console=ttyAMA0,115200n8 earlycon=pl011,0x9000000" \
      -drive "file=${QEMU_ROOTFS},format=raw,if=virtio,cache=writethrough" \
      -drive "file=${QEMU_USB_DISK},format=qcow2,if=virtio,cache=writethrough" \
      -netdev "vmnet-shared,id=lan,start-address=${QEMU_VMNET_START},end-address=${QEMU_VMNET_END},subnet-mask=255.255.255.0" \
      -device virtio-net,netdev=lan \
      -netdev "user,id=wan,net=${QEMU_WAN_NET}" \
      -device virtio-net,netdev=wan \
      -device virtio-rng-pci \
      -chardev null,id=char0 \
      -serial chardev:char0 \
      -D "${QEMU_RUNTIME_DIR}/qemu.log"
  else
    # shellcheck disable=SC2086
    set -- \
      -name "$QEMU_LAB_NAME" \
      -machine virt,highmem=off \
      $qemu_accel \
      -cpu "$qemu_cpu" \
      -smp "$QEMU_CPUS" \
      -m "$QEMU_MEMORY" \
      -kernel "$QEMU_KERNEL" \
      -append "root=/dev/vda rootwait console=ttyAMA0,115200n8 earlycon=pl011,0x9000000" \
      -drive "file=${QEMU_ROOTFS},format=raw,if=virtio,cache=writethrough" \
      -drive "file=${QEMU_USB_DISK},format=qcow2,if=virtio,cache=writethrough" \
      -netdev "user,id=lan,net=${QEMU_LAN_NET},hostfwd=tcp:127.0.0.1:${QEMU_SSH_PORT}-${QEMU_LAN_GUEST_IP}:22,hostfwd=tcp:127.0.0.1:${QEMU_PANEL_PORT}-${QEMU_LAN_GUEST_IP}:7777" \
      -device virtio-net,netdev=lan \
      -netdev "user,id=wan,net=${QEMU_WAN_NET}" \
      -device virtio-net,netdev=wan \
      -device virtio-rng-pci \
      -chardev null,id=char0 \
      -serial chardev:char0 \
      -D "${QEMU_RUNTIME_DIR}/qemu.log"
  fi

  if [ "$(uname -s)" = Darwin ]; then
    export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES
    if [ "$QEMU_USE_VMNET" = "1" ]; then
      qemu_log "vmnet requires sudo on macOS — enter password if prompted"
      # shellcheck disable=SC2086
      sudo -E nohup qemu-system-aarch64 "$@" >> "${QEMU_RUNTIME_DIR}/qemu-stdout.log" 2>&1 &
      sudo sh -c "echo $! > '${QEMU_PIDFILE}'"
    else
      nohup qemu-system-aarch64 "$@" >> "${QEMU_RUNTIME_DIR}/qemu-stdout.log" 2>&1 &
      echo $! > "$QEMU_PIDFILE"
    fi
  else
    qemu-system-aarch64 "$@" -daemonize -pidfile "$QEMU_PIDFILE"
  fi

  sleep 3
  qemu_is_running || qemu_die "QEMU failed to start — see ${QEMU_RUNTIME_DIR}/qemu.log and qemu-stdout.log"
}

qemu_build_panel() {
  OUT="${QEMU_REPO_ROOT}/dist/panel-linux-arm64"
  VERSION="$(git -C "$QEMU_REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
  COMMIT="$(git -C "$QEMU_REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)"
  BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  LDFLAGS="-s -w \
    -X github.com/taiiok/xiaomi-vless/internal/version.Version=${VERSION} \
    -X github.com/taiiok/xiaomi-vless/internal/version.Commit=${COMMIT} \
    -X github.com/taiiok/xiaomi-vless/internal/version.BuildDate=${BUILD_DATE}"

  qemu_log "building panel for linux/arm64..."
  (
    cd "$QEMU_REPO_ROOT"
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build \
      -ldflags "$LDFLAGS" \
      -o "$OUT" ./cmd/panel
  )
  qemu_log "built $OUT"
  printf '%s' "$OUT"
}

qemu_extract_opkg_bundle() {
  # Merge .ipk contents on the host (OpenWrt guest has no ar/binutils).
  # Cached downloads are gzip-compressed tarballs wrapping debian-binary/control/data.
  dest="$1"
  src_dir="$2"
  mkdir -p "$dest"
  for ipk in "$src_dir"/*.ipk; do
    [ -f "$ipk" ] || continue
    work="$(mktemp -d)"
    if gzip -dc "$ipk" 2>/dev/null | tar -xf - -C "$work" 2>/dev/null; then
      :
    elif tar -xf "$ipk" -C "$work" 2>/dev/null; then
      :
    else
      cp "$ipk" "$work/pkg.ipk"
      (cd "$work" && ar x pkg.ipk) || qemu_die "cannot unpack ipk: $(basename "$ipk")"
    fi
    [ -f "$work/data.tar.gz" ] || qemu_die "invalid ipk (no data.tar.gz): $(basename "$ipk")"
    tar -xzf "$work/data.tar.gz" -C "$work"
    find "$work" -name '._*' -delete 2>/dev/null || true
    rm -f "$work"/pkg.ipk "$work"/debian-binary "$work"/control.tar.gz "$work"/data.tar.gz
    (cd "$work" && tar -cf - .) | (cd "$dest" && tar -xf -)
    rm -rf "$work"
  done
}

qemu_install_opkg_bundle_on_guest() {
  bundle_dir="$1"
  [ -d "$bundle_dir" ] || return 0
  qemu_log "installing opkg bundle on guest rootfs..."
  tar -C "$bundle_dir" -cf - . | qemu_ssh "tar -xf - -C /"
}

qemu_stage_opkg() {
  stage="$1"
  opkg_stage="${stage}/opkg"
  cache="${QEMU_IMAGES_DIR}/opkg-cache"
  manifest="${QEMU_SCRIPT_DIR}/opkg-cache.manifest"
  mkdir -p "$opkg_stage" "$cache"
  [ -f "$manifest" ] || return 0

  while IFS= read -r url || [ -n "$url" ]; do
    case "$url" in
      ''|\#*) continue ;;
    esac
    name="$(basename "$url")"
    if [ ! -s "${cache}/${name}" ]; then
      qemu_log "downloading opkg cache: $name"
      curl -fsSL "$url" -o "${cache}/${name}"
      [ -s "${cache}/${name}" ] || qemu_die "failed to download $url"
    fi
    cp "${cache}/${name}" "${opkg_stage}/${name}"
  done < "$manifest"
}

qemu_stage_xray() {
  stage="$1"
  xray_stage="${stage}/xray"
  cache="${QEMU_IMAGES_DIR}/xray-arm64"
  mkdir -p "$xray_stage" "$cache"

  if [ ! -x "${cache}/xray" ]; then
    qemu_log "downloading Xray (arm64) on host..."
    tmp="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap 'rm -rf "$tmp"' EXIT INT HUP
    curl -fsSL "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-arm64-v8a.zip" \
      -o "${tmp}/xray.zip"
    unzip -qo "${tmp}/xray.zip" -d "${tmp}/unzipped"
    xray_bin="$(find "${tmp}/unzipped" -name xray -type f | head -1)"
    [ -n "$xray_bin" ] || qemu_die "xray binary not found in release zip"
    cp "$xray_bin" "${cache}/xray"
    chmod 755 "${cache}/xray"
    [ -s "${cache}/xray" ] || qemu_die "xray cache is empty after download"
    curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat" \
      -o "${cache}/geoip.dat"
    curl -fsSL "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat" \
      -o "${cache}/geosite.dat"
    rm -rf "$tmp"
    trap - EXIT INT HUP
  fi

  cp "${cache}/xray" "${xray_stage}/xray"
  cp "${cache}/geoip.dat" "${xray_stage}/geoip.dat"
  cp "${cache}/geosite.dat" "${xray_stage}/geosite.dat"
  chmod 755 "${xray_stage}/xray"
}

qemu_stage_files() {
  stage="$1"
  panel_bin="$2"
  rm -rf "$stage"
  mkdir -p "$stage"

  qemu_stage_opkg "$stage"
  qemu_extract_opkg_bundle "${stage}/opkg-root" "${stage}/opkg"
  qemu_stage_xray "$stage"

  cp "$panel_bin" "$stage/panel-linux"
  cp "${QEMU_REPO_ROOT}/lab/qemu/panel.json" "$stage/panel-lab.json"
  cp "${QEMU_REPO_ROOT}/deploy/panel-updater.sh" "$stage/panel-updater.sh"
  cp "${QEMU_REPO_ROOT}/lab/qemu/provision-openwrt.sh" "$stage/provision-openwrt.sh"
  cp "${QEMU_REPO_ROOT}/lab/qemu/install-ipk-manual.sh" "$stage/install-ipk-manual.sh"
  cp "${QEMU_REPO_ROOT}/lab/qemu/network-setup-openwrt.sh" "$stage/network-setup-openwrt.sh"
  cp "${QEMU_REPO_ROOT}/lab/guest-netns.sh" "$stage/guest-netns.sh"

  for script in startup_xray_guest.sh xray-guest-iptables.sh xray-guest-iptables-cron.sh \
    xray-guest-sysctl.sh xiaomi-vless-failopen-guard.sh boot-xiaomi-vless.sh; do
    cp "${QEMU_REPO_ROOT}/scripts/${script}" "$stage/${script}"
  done

  for init in xiaomi-vless-panel.init xiaomi-vless-xray.init xiaomi-vless-boot.init \
    hotplug-usb-xiaomi-vless.sh install-autostart.sh install-common.sh; do
    cp "${QEMU_REPO_ROOT}/deploy/${init}" "$stage/${init}"
  done

  chmod +x "$stage"/*.sh "$stage/provision-openwrt.sh" "$stage/network-setup-openwrt.sh" \
    "$stage/guest-netns.sh" 2>/dev/null || true
}

qemu_transfer_staging() {
  stage="$1"
  qemu_log "uploading staging files to guest /tmp/qemu-staging..."
  qemu_ssh "rm -rf /tmp/qemu-staging && mkdir -p /tmp/qemu-staging"
  tar -C "$stage" -cf - . | qemu_ssh "tar -xf - -C /tmp/qemu-staging"
}

qemu_print_panel_url() {
  if [ "$QEMU_USE_VMNET" = "1" ] && [ -f "$QEMU_GUEST_IP_FILE" ]; then
    ip="$(cat "$QEMU_GUEST_IP_FILE")"
    printf '\nPanel: http://%s:7777\n' "$ip"
    printf 'SSH:   ssh root@%s\n' "$ip"
    return 0
  fi
  printf '\nPanel: http://127.0.0.1:%s\n' "$QEMU_PANEL_PORT"
  printf 'SSH:   ssh -p %s root@127.0.0.1\n' "$QEMU_SSH_PORT"
}
