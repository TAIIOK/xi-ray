#!/bin/sh
# Stop or delete the lab VM.
#
# Usage:
#   ./lab/scripts/lab-down.sh          # stop
#   LAB_PURGE=1 ./lab/scripts/lab-down.sh   # delete VM
set -eu

VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"

if [ "${LAB_PURGE:-0}" = "1" ]; then
  echo "Deleting VM $VM_NAME..."
  multipass delete "$VM_NAME" --purge
else
  echo "Stopping VM $VM_NAME..."
  multipass stop "$VM_NAME" 2>/dev/null || echo "VM not running or not found"
fi
