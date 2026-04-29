#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
NATIVE_DIR="$ROOT_DIR/internal/prometheus/native"
OUT_DIR="$ROOT_DIR/out/prometheus/native"
REACTOR_DIR="$ROOT_DIR/internal/prometheus/reactor"
mkdir -p "$OUT_DIR" "$REACTOR_DIR"
LIB_NAME="libprometheus_reactor.so"
COMMON_C=("$NATIVE_DIR/reactor_api.c" "$NATIVE_DIR/reactor_judgment_engine.c" "$NATIVE_DIR/reactor_dominatus_blackboard.c" "$NATIVE_DIR/reactor_dominatus_sgemm_adapter.c" "$NATIVE_DIR/reactor_dominatus_slot_adapter.c" "$NATIVE_DIR/reactor_policy_memory.c" "$NATIVE_DIR/reactor_slot_hfsm.c" "$NATIVE_DIR/reactor_vulkan_common.c" "$NATIVE_DIR/reactor_vulkan_sgemm.c" "$NATIVE_DIR/reactor_vulkan_fft.c" "$NATIVE_DIR/reactor_vulkan_fused_reduction.c")
cc -std=c11 -fPIC -shared "${COMMON_C[@]}" -pthread -lm -lvulkan -o "$OUT_DIR/$LIB_NAME"
cp "$OUT_DIR/$LIB_NAME" "$REACTOR_DIR/$LIB_NAME"
MARIONETTE_CPP=($(rg --files "$NATIVE_DIR/Marionette" | rg '\.cpp$' | rg -v 'test_main(_slow|_benchmarks)?\.cpp$'))

c++ -std=c++23 -O2 -DMARIONETTE_TEST_REPO_ROOT="\"$ROOT_DIR\"" -DMARIONETTE_EXCLUDE_SLOW_TESTS -DMARIONETTE_EXCLUDE_BENCHMARK_TESTS "${COMMON_C[@]}" "${MARIONETTE_CPP[@]}" "$NATIVE_DIR/Marionette/test_main.cpp" -pthread -lm -lvulkan -o "$OUT_DIR/marionette_tests"
c++ -std=c++23 -O2 -DMARIONETTE_TEST_REPO_ROOT="\"$ROOT_DIR\"" -DMARIONETTE_EXCLUDE_BENCHMARK_TESTS "${COMMON_C[@]}" "${MARIONETTE_CPP[@]}" "$NATIVE_DIR/Marionette/test_main_slow.cpp" -pthread -lm -lvulkan -o "$OUT_DIR/marionette_slow_tests"
c++ -std=c++23 -O2 -DMARIONETTE_TEST_REPO_ROOT="\"$ROOT_DIR\"" "${COMMON_C[@]}" "${MARIONETTE_CPP[@]}" "$NATIVE_DIR/Marionette/test_main_benchmarks.cpp" -pthread -lm -lvulkan -o "$OUT_DIR/marionette_benchmarks"
echo "Built Marionette tests: $OUT_DIR/marionette_tests"
echo "Built Marionette slow tests: $OUT_DIR/marionette_slow_tests"
echo "Built Marionette benchmarks: $OUT_DIR/marionette_benchmarks"
