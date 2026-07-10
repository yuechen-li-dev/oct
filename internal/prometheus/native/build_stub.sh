#!/usr/bin/env bash
# Compatibility entry point. The complete Linux build lives in build_linux.sh.
set -euo pipefail
echo "warning: build_stub.sh is deprecated; invoking build_linux.sh" >&2
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/build_linux.sh" "$@"
