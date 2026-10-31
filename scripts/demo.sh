#!/usr/bin/env bash
# Record docs/demo.gif with VHS (https://github.com/charmbracelet/vhs) when available.
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v vhs >/dev/null 2>&1; then
  echo "vhs not installed; generating a placeholder GIF with Python…"
  python3 scripts/gen_demo_gif.py
  exit 0
fi
vhs scripts/demo.tape
