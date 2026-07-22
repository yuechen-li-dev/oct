#!/usr/bin/env sh
set -eu
version=${1:?usage: new_oct_release.sh VERSION OUTPUT_DIRECTORY}
out=${2:?usage: new_oct_release.sh VERSION OUTPUT_DIRECTORY}
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
name="oct-${version}-linux-amd64"
stage="$out/stage-$name"
root="$stage/$name"
archive="$out/$name.tar.gz"
mkdir -p "$out"
rm -rf "$stage" "$archive"
mkdir -p "$root/runtime/internal/octxiliary" "$root/sidecars"
(
    cd "$repo"
    go build -trimpath -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=$version" -o "$root/oct" ./cmd/oct
    go run ./tools/build_sidecars --out "$root/sidecars"
)
cp "$repo/LICENSE" "$root/LICENSE"
cp "$repo/docs/releases/INSTALL_1_0.md" "$root/INSTALL.md"
cp "$repo/go.mod" "$repo/go.sum" "$root/runtime/"
find "$repo/internal/octxiliary" -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' -exec cp {} "$root/runtime/internal/octxiliary/" \;
tar -C "$stage" --sort=name --owner=0 --group=0 --numeric-owner -czf "$archive" "$name"
(
    cd "$out"
    find . -maxdepth 1 -type f \( -name 'oct-*.zip' -o -name 'oct-*.tar.gz' \) -printf '%f\n' | sort | xargs sha256sum > checksums.sha256
)
printf '%s\n' "$archive"
