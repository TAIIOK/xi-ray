#!/bin/sh
# Open a shell in the lab VM.
set -eu

VM_NAME="${LAB_VM_NAME:-xiaomi-vless-lab}"
multipass shell "$VM_NAME"
