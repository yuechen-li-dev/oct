#!/usr/bin/env bash
set -euo pipefail

if ! command -v pkg-config >/dev/null 2>&1; then
  echo "missing dependency: pkg-config"
  exit 1
fi

missing=0
for lib in xkbcommon wayland-client; do
  if pkg-config --exists "$lib"; then
    echo "ok: $lib"
  else
    echo "missing pkg-config module: $lib"
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  cat <<'MSG'
Gio native lane prerequisites are incomplete.
Install distro-appropriate development packages, then retry.
MSG
  exit 1
fi

echo "Gio Linux pkg-config preflight passed."
