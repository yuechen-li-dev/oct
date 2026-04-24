#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
NATIVE_DIR="$ROOT_DIR/internal/prometheus/native"
OUT_DIR="$ROOT_DIR/out/prometheus/native"
REACTOR_DIR="$ROOT_DIR/internal/prometheus/reactor"

mkdir -p "$OUT_DIR" "$REACTOR_DIR"

LIB_NAME="libprometheus_reactor.so"

cc -std=c11 -fPIC -shared \
  "$NATIVE_DIR/reactor_api.c" \
  "$NATIVE_DIR/reactor_judgment_engine.c" \
  "$NATIVE_DIR/reactor_dominatus_blackboard.c" \
  "$NATIVE_DIR/reactor_dominatus_sgemm_adapter.c" \
  "$NATIVE_DIR/reactor_policy_memory.c" \
  "$NATIVE_DIR/reactor_slot_hfsm.c" \
  "$NATIVE_DIR/reactor_vulkan.c" \
  -pthread \
  -lm \
  -lvulkan \
  -o "$OUT_DIR/$LIB_NAME"

cp "$OUT_DIR/$LIB_NAME" "$REACTOR_DIR/$LIB_NAME"

c++ -std=c++23 -O2 \
  -DMARIONETTE_TEST_REPO_ROOT="\"$ROOT_DIR\"" \
  "$NATIVE_DIR/reactor_api.c" \
  "$NATIVE_DIR/reactor_judgment_engine.c" \
  "$NATIVE_DIR/reactor_dominatus_blackboard.c" \
  "$NATIVE_DIR/reactor_dominatus_sgemm_adapter.c" \
  "$NATIVE_DIR/reactor_policy_memory.c" \
  "$NATIVE_DIR/reactor_slot_hfsm.c" \
  "$NATIVE_DIR/reactor_vulkan.c" \
  "$NATIVE_DIR/Marionette"/*.cpp \
  -pthread \
  -lm \
  -lvulkan \
  -o "$OUT_DIR/marionette_tests"

echo "Built reactor library: $OUT_DIR/$LIB_NAME"
echo "Copied for bridge discovery: $REACTOR_DIR/$LIB_NAME"
echo "Built Marionette tests: $OUT_DIR/marionette_tests"
