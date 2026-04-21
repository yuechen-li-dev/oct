# Prometheus SGEMM Algorithm Lab

## Purpose

Prometheus SGEMM Algorithm Lab is a correctness-first Oct experiment track for matrix-multiplication design work. It is intended for scientific prototyping, algorithm exploration, and policy exploration before low-level implementation work.

## Why this is separate from the benchmark harness

This lab captures reference algorithm behavior and design reasoning in Oct. The benchmark harness answers performance questions. Keeping them separate prevents benchmark concerns from distorting baseline algorithm validation.

## M0 scope

M0 is the baseline capture milestone.

It establishes the experiment scaffold and records a reference matmul path by lifting the current `LinearAlgebra.Core.MatMul()` baseline and its supporting `.octest` into this experiment.

## M0 baseline source and coding-shape update

The M0 implementation is a semantic lift of `LinearAlgebra.Core.MatMul()` plus required helper functions (`FlatIndex` and matrix validation).

The copied baseline is rewritten to match current style conventions by replacing `while` loops that encode structured iteration with `for` loops, without changing behavior.

## Forward plan

Future milestones in this lab will evaluate algorithm variants and controller-policy ideas in Oct first, then use proven raw SGEMM paths and policy outcomes as port-ready references for later Reactor implementation.

## M1 scope

M1 introduces the first controlled SGEMM algorithm variants in this lab while keeping correctness as the primary objective.

### Variants introduced

- `MatMul_IKJ`: loop-order variant using `i -> k -> j`
- `MatMul_KIJ`: loop-order variant using `k -> i -> j`
- `MatMul_Blocked`: single-level blocked variant over `i` and `j` with baseline inner ordering
- `MatMul_Blocked_IKJ`: single-level blocked variant over `i` and `j` with `i -> k -> j` inner ordering

### Factors explored

- **Loop order**: compared baseline `i -> j -> k` against `i -> k -> j` and `k -> i -> j`
- **Blocking**: added one block-size parameter and applied one-level `i`/`j` tiling with explicit edge-tile handling

### Correctness confirmation

M1 `.octest` coverage verifies cross-variant equality against baseline across:

- small square shapes (`2x2`, `3x3`)
- rectangular shape (`3x5 * 5x2`)
- medium square shape (`16x16`)

The test coverage also verifies:

- blocked variant behavior for block sizes `1`, `2`, and `4`, including non-divisible dimensions
- consistent rejection of incompatible shapes across all variants
- rejection of invalid blocked parameters (`blockSize <= 0`)

### Structural observations (non-performance)

- Loop-order variants are now isolated as explicit, readable kernels with identical I/O contracts.
- Blocked variants currently tile only output-space dimensions (`i`, `j`) and keep multiply-accumulate semantics straightforward.
- Shared validation and indexing helpers reduce drift risk between variants and establish a repeatable pattern for later milestones.
