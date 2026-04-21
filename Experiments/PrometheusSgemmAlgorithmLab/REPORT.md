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
