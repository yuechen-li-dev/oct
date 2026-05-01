#!/usr/bin/env bash
# build_stub.sh — Prometheus native reactor + Marionette test suite builder
#
# Usage:
#   bash build_stub.sh           # normal build, warnings suppressed
#   VERBOSE=1 bash build_stub.sh # show all compiler output
set -euo pipefail

VERBOSE="${VERBOSE:-0}"

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
NATIVE_DIR="$ROOT_DIR/internal/prometheus/native"
OUT_DIR="$ROOT_DIR/out/prometheus/native"
REACTOR_DIR="$ROOT_DIR/internal/prometheus/reactor"
OBJ_DIR="$OUT_DIR/obj"

mkdir -p "$OUT_DIR" "$REACTOR_DIR" "$OBJ_DIR"

LIB_NAME="libprometheus_reactor.so"

COMMON_C=(
  "$NATIVE_DIR/reactor_api.c"
  "$NATIVE_DIR/reactor_judgment_engine.c"
  "$NATIVE_DIR/reactor_dominatus_blackboard.c"
  "$NATIVE_DIR/reactor_dominatus_filter.c"
  "$NATIVE_DIR/reactor_dominatus_sgemm_adapter.c"
  "$NATIVE_DIR/reactor_dominatus_slot_adapter.c"
  "$NATIVE_DIR/reactor_policy_memory.c"
  "$NATIVE_DIR/reactor_slot_hfsm.c"
  "$NATIVE_DIR/reactor_vulkan_common.c"
  "$NATIVE_DIR/reactor_vulkan_sgemm.c"
  "$NATIVE_DIR/reactor_vulkan_fft.c"
  "$NATIVE_DIR/reactor_vulkan_fused_reduction.c"
)

if [[ "$VERBOSE" == "1" ]]; then
  WARN_FLAGS=(-Wall -Wextra)
else
  WARN_FLAGS=(-w)
fi

# Step 1: Compile COMMON_C once to shared .o files
echo "[1/5] Compiling common C objects..."
COMMON_OBJS=()
for src in "${COMMON_C[@]}"; do
  base=$(basename "$src" .c)
  obj="$OBJ_DIR/${base}.o"
  COMMON_OBJS+=("$obj")
  if [[ "$VERBOSE" == "1" ]]; then
    cc -std=c11 -fPIC -O2 "${WARN_FLAGS[@]}" -c "$src" -o "$obj"
  else
    cc -std=c11 -fPIC -O2 "${WARN_FLAGS[@]}" -c "$src" -o "$obj" 2>/dev/null
  fi
done
echo "[1/5] Done."

# Step 2: Link the shared reactor library
echo "[2/5] Linking $LIB_NAME..."
if [[ "$VERBOSE" == "1" ]]; then
  cc -std=c11 -shared "${COMMON_OBJS[@]}" -pthread -lm -lvulkan -o "$OUT_DIR/$LIB_NAME"
else
  cc -std=c11 -shared "${COMMON_OBJS[@]}" -pthread -lm -lvulkan -o "$OUT_DIR/$LIB_NAME" 2>/dev/null
fi
cp "$OUT_DIR/$LIB_NAME" "$REACTOR_DIR/$LIB_NAME"
echo "Built reactor library: $OUT_DIR/$LIB_NAME"
echo "Copied for bridge discovery: $REACTOR_DIR/$LIB_NAME"

# Step 3: Collect Marionette C++ sources (no rg dependency)
echo "[3/5] Collecting Marionette sources..."
MARIONETTE_CPP=()
while IFS= read -r -d '' f; do
  MARIONETTE_CPP+=("$f")
done < <(find "$NATIVE_DIR/Marionette" -maxdepth 1 -name "*.cpp" \
  ! -name "test_main.cpp" \
  ! -name "test_main_slow.cpp" \
  ! -name "test_main_benchmarks.cpp" \
  ! -name "reactor_p11_m6_batch_tests.cpp" \
  -print0 | sort -z)

if [[ ${#MARIONETTE_CPP[@]} -eq 0 ]]; then
  echo "ERROR: No Marionette .cpp sources found in $NATIVE_DIR/Marionette" >&2
  exit 1
fi
echo "[3/5] Found ${#MARIONETTE_CPP[@]} Marionette source files."

# Step 4: Build the three test binaries
echo "[4/5] Building Marionette test binaries..."

do_build() {
  local out_bin="$1"
  local main_cpp="$2"
  local extra_src="$3"
  shift 3
  local defines=("$@")

  if [[ ! -f "$main_cpp" ]]; then
    echo "  SKIP $(basename "$out_bin") — entry point not found: $(basename "$main_cpp")"
    return 0
  fi

  local define_flags=()
  for d in "${defines[@]}"; do
    define_flags+=("-D$d")
  done

  local extra_src_args=()
  if [[ -n "$extra_src" ]]; then
    extra_src_args+=("$extra_src")
  fi

  echo -n "  Building $(basename "$out_bin")... "

  local repo_define="-DMARIONETTE_TEST_REPO_ROOT=\"$ROOT_DIR\""

  if [[ "$VERBOSE" == "1" ]]; then
    c++ -std=c++23 -O2 "${WARN_FLAGS[@]}" "$repo_define" \
      "${define_flags[@]}" \
      "${COMMON_OBJS[@]}" "${MARIONETTE_CPP[@]}" "${extra_src_args[@]}" "$main_cpp" \
      -pthread -lm -lvulkan -o "$out_bin"
  else
    c++ -std=c++23 -O2 "${WARN_FLAGS[@]}" "$repo_define" \
      "${define_flags[@]}" \
      "${COMMON_OBJS[@]}" "${MARIONETTE_CPP[@]}" "${extra_src_args[@]}" "$main_cpp" \
      -pthread -lm -lvulkan -o "$out_bin" 2>/dev/null
  fi
  echo "ok"
}

do_build \
  "$OUT_DIR/marionette_tests" \
  "$NATIVE_DIR/Marionette/test_main.cpp" \
  "" \
  "MARIONETTE_EXCLUDE_SLOW_TESTS" \
  "MARIONETTE_EXCLUDE_BENCHMARK_TESTS"

do_build \
  "$OUT_DIR/marionette_slow_tests" \
  "$NATIVE_DIR/Marionette/test_main_slow.cpp" \
  "$NATIVE_DIR/Marionette/reactor_p11_m6_batch_tests.cpp" \
  "MARIONETTE_EXCLUDE_BENCHMARK_TESTS"

do_build \
  "$OUT_DIR/marionette_benchmarks" \
  "$NATIVE_DIR/Marionette/test_main_benchmarks.cpp" \
  ""

# Step 5: Report
echo "[5/5] Build complete."
echo "Built Marionette tests:      $OUT_DIR/marionette_tests"
if [[ -f "$OUT_DIR/marionette_slow_tests" ]]; then echo "Built Marionette slow tests:  $OUT_DIR/marionette_slow_tests"; fi
if [[ -f "$OUT_DIR/marionette_benchmarks" ]]; then echo "Built Marionette benchmarks:  $OUT_DIR/marionette_benchmarks"; fi
