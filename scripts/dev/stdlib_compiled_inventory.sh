#!/usr/bin/env bash
set -u

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
LOGDIR="$ROOT/.tmp/stdlib-compiled-inventory"
WRAPPER_DIR="$ROOT/.tmp/m22-wrappers"
SIDEcars=(io hash compression time text archive json csv plot xlsx image pdf)

mkdir -p "$LOGDIR/perlib" "$WRAPPER_DIR"
: > "$LOGDIR/commands.tsv"

run_logged() {
  local name="$1" mode="$2" timeout_s="$3"
  shift 3
  local log="$LOGDIR/${name}.log"
  local start end status duration
  mkdir -p "$(dirname "$log")"
  printf 'RUN %s: %s\n' "$name" "$*"
  start=$(date +%s)
  (cd "$ROOT" && OCT_WRAPPER_PATH="$WRAPPER_DIR" timeout "${timeout_s}s" "$@") >"$log" 2>&1
  status=$?
  end=$(date +%s)
  duration=$((end - start))
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$name" "$mode" "$status" "$duration" "$timeout_s" "$*" >> "$LOGDIR/commands.tsv"
  printf 'DONE %s status=%s duration=%ss log=%s\n' "$name" "$status" "$duration" "$log"
}

printf 'Building sidecars into %s\n' "$WRAPPER_DIR"
for sidecar in "${SIDEcars[@]}"; do
  run_logged "build_octxiliary_${sidecar}" build 180 go build -o "$WRAPPER_DIR/octxiliary-$sidecar" "./cmd/octxiliary-$sidecar"
done

run_logged go_test_internal_cmd go 300 go test ./internal/... ./cmd/oct
run_logged libs_interpreted interpreted 180 go run ./cmd/oct test Libraries --execution interpreted
run_logged libs_compiled compiled 300 go run ./cmd/oct test Libraries --execution compiled
run_logged libs_auto auto 180 go run ./cmd/oct test Libraries --execution auto

while IFS= read -r dir; do
  base=${dir#Libraries/}
  safe=${base//[^A-Za-z0-9_]/_}
  for mode in interpreted compiled auto; do
    run_logged "perlib/${safe}_${mode}" "$mode" 180 go run ./cmd/oct test "$dir" --execution "$mode"
  done
done < <(cd "$ROOT" && find Libraries -mindepth 1 -maxdepth 1 -type d | sort)

printf '\nSummary by per-library compiled command:\n'
awk -F '\t' '$1 ~ /^perlib\/.*_compiled$/ { total++; if ($3 == 0) pass++; else fail++; printf "%-36s status=%s duration=%ss\n", $1, $3, $4 } END { printf "pass=%d fail=%d total=%d\n", pass+0, fail+0, total+0 }' "$LOGDIR/commands.tsv"
printf 'Logs written under %s\n' "$LOGDIR"
