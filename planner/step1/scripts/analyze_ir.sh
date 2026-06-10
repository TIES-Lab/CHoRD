#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <input.bc|input.ll> <out.json>"
  exit 1
fi

IN="$1"
OUT="$2"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${DIR}/../build/mgvm-kernel-analyzer"

if [[ ! -x "${BIN}" ]]; then
  echo "Binary not found: ${BIN}"
  echo "Build first: mkdir -p build && cd build && cmake .. && make -j"
  exit 1
fi

"${BIN}" --input "${IN}" --output "${OUT}"
echo "Wrote ${OUT}"
