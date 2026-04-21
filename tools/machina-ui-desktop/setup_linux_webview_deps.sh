#!/usr/bin/env bash
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Re-run as root (sudo) so apt packages can be installed." >&2
  exit 1
fi

apt-get update

webkit_pkg=""
webkit_pc=""
if apt-cache show libwebkit2gtk-4.0-dev >/dev/null 2>&1; then
  webkit_pkg="libwebkit2gtk-4.0-dev"
  webkit_pc="webkit2gtk-4.0"
elif apt-cache show libwebkit2gtk-4.1-dev >/dev/null 2>&1; then
  webkit_pkg="libwebkit2gtk-4.1-dev"
  webkit_pc="webkit2gtk-4.1"
else
  echo "Unable to locate libwebkit2gtk development package (4.0 or 4.1)." >&2
  exit 1
fi

apt-get install -y --no-install-recommends \
  pkg-config \
  libgtk-3-dev \
  "${webkit_pkg}" \
  xvfb

pkg-config --modversion gtk+-3.0
pkg-config --modversion "${webkit_pc}"

echo "Linux webview prerequisites installed."
echo "Run smoke check:"
echo "  CGO_ENABLED=1 xvfb-run -a env MACHINA_WEBVIEW_SMOKE=1 go test -tags machina_desktop_webview ./internal/machina/desktophost -run TestWebviewDriverFactoryConstructsAndInitializesNativeBinding -count=1"
