#!/bin/sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
# shellcheck source=lab/qemu/qemu-common.sh
. "${SCRIPT_DIR}/qemu-common.sh"
qemu_common_init

qemu_is_running || qemu_die "QEMU not running — run: make qemu-up"
exec ssh $(qemu_ssh_opts) -p "$(qemu_ssh_port)" -t "$(qemu_ssh_target)"
