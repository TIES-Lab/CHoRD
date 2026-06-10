#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "Usage: $0 <gfxArch> <input.cl> <out.bc>"
  echo "Example: $0 gfx906 kernels.cl kernels.bc"
  exit 1
fi

GFX="$1"
IN="$2"
OUT="$3"

clang -x cl -cl-std=CL2.0 -O2 -emit-llvm -c \
  --target=amdgcn-amd-amdhsa -mcpu="${GFX}" \
  -o "${OUT}" "${IN}"

echo "Wrote ${OUT}"
