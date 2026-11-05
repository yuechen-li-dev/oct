# Native coding primers

These are the current upstream YAML v2 primers from
[yuechen-li-dev/primer](https://github.com/yuechen-li-dev/primer), pinned at
[`648212bf6b445d3910a7d3941961c47b0140ca91`](https://github.com/yuechen-li-dev/primer/tree/648212bf6b445d3910a7d3941961c47b0140ca91).
Updated 2026-11-05. The old Markdown v1 primers have been replaced; historical
reports referring to them describe the guidance used at those earlier commits.

Read [common-primer.yaml](common-primer.yaml), then the applicable profiles:

- [C++23](cpp-primer.yaml), with [examples](cpp-primer-examples.yaml).
- [Vulkan API overlay](vulkan-primer.yaml), with [examples](vulkan-primer-examples.yaml).

Requirements state correctness obligations. Defaults admit departures when the
actual domain or repository benefits; explain the tradeoff and verify it.
They do not prohibit a C++ feature merely by name or introduce an approval step.
Repository architecture and commands remain in [AGENTS.md](../AGENTS.md).

The vendored YAML and [license](LICENSE) are byte-identical to that upstream
commit. Primer examples illustrate host logic; native compiler and GPU support
still require evidence from the changed production path.
