#!/usr/bin/env sh
set -eu
version=${1:?usage: new_oct_release.sh VERSION OUTPUT_DIRECTORY}
out=${2:?usage: new_oct_release.sh VERSION OUTPUT_DIRECTORY}
repo=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
install_suffix=$(printf '%s\n' "$version" | sed -n 's/^\([0-9][0-9]*\)\.\([0-9][0-9]*\)\..*/\1_\2/p')
if [ -z "$install_suffix" ]; then
    printf 'release version must begin with major.minor.patch: %s\n' "$version" >&2
    exit 1
fi
install_guide="$repo/docs/releases/INSTALL_${install_suffix}.md"
if [ ! -f "$install_guide" ]; then
    printf 'missing release install guide: %s\n' "$install_guide" >&2
    exit 1
fi
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
cp "$install_guide" "$root/INSTALL.md"
cp "$repo/go.mod" "$repo/go.sum" "$root/runtime/"
find "$repo/internal/octxiliary" -maxdepth 1 -type f -name '*.go' ! -name '*_test.go' -exec cp {} "$root/runtime/internal/octxiliary/" \;
tar -C "$stage" --sort=name --owner=0 --group=0 --numeric-owner -czf "$archive" "$name"
(
    cd "$out"
    find . -maxdepth 1 -type f \( -name 'oct-*.zip' -o -name 'oct-*.tar.gz' \) -printf '%f\n' | sort | xargs sha256sum > checksums.sha256
)
printf '%s\n' "$archive"
