#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Re-run as root (sudo) so apt packages can be installed." >&2
  exit 1
fi

apt-get update
apt-get install -y --no-install-recommends \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.0-dev \
  xvfb

pkg-config --modversion gtk+-3.0
pkg-config --modversion webkit2gtk-4.0

echo "Linux webview prerequisites installed."
echo "Run smoke check:"
echo "  CGO_ENABLED=1 xvfb-run -a env MACHINA_WEBVIEW_SMOKE=1 go test -tags machina_desktop_webview ./internal/machina/desktophost -run TestWebviewDriverFactoryConstructsAndInitializesNativeBinding -count=1"
