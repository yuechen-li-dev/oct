# Prometheus shader package M0 foundation

Status: foundation staged; native payload migration is not yet complete.

## Starting audit

At `2d59d41721de7f3a78ccc040163e34e037733e67`, production SPIR-V is
compiled into `reactor_shader_registry.c` through generated `*_spirv.h`
headers. `reactor_vulkan_sgemm.c` also owns the baseline inline SGEMM word
array. Direct `vkCreateShaderModule` calls exist in SGEMM, FFT, reductions,
model-block, transformer, and ray-query paths. The manifest inventory contains
55 current shader identities, including ray-query capability-probe and raw-hit
assets; two assets currently share identical content, so the staged package has
54 objects.

The M1 mechanical source scan separates the often-misleading raw match total:

| Category | Production count |
| --- | ---: |
| Generated `*_spirv.h` includes | 86 |
| Embedded `k_prom_*_spirv` array definitions | 59 |
| Payload symbol references | 757 |
| `sizeof(payload)` references | 71 |
| Direct `vkCreateShaderModule` call sites | 29 |
| Registry API references | 67 |

The direct-module total includes one independent SDSL-V test host. It is not
evidence of 951 independent runtime module paths: most matches are registry
payload wiring and generated-header metadata. Marionette tests add 26 generated
header includes and 197 payload-symbol references; generation/tooling has 24
separate references. This table is the M1 migration checklist baseline.

## Package format

`oct sdslv package build` creates one package named `prometheus.core`:

```text
out/prometheus/native/SerialCanonical/
  prometheus_reactor.{dll,so}       # existing native output
  shaders/
    manifest.json
    objects/sha256/<lowercase-sha256>.spv
```

The exact manifest schema is `prometheus.shader-package.v1`; its package
version is `1` and runtime ABI is `1`. It contains typed `artifacts`,
`kernels`, `variants`, `requirements`, `implementations`, and `provenance`
tables. An artifact has exactly a SHA-256 digest, byte count, and SPIR-V media
type. A variant refers to a kernel and an artifact and records the entry point,
workgroup, descriptor count, push-constant bytes, and presently-authoritative
dispatch envelope facts. Requirements are only named Vulkan/SPIR-V
extension/capability/feature facts.

There are no scripts, selectors, weights, expressions, paths supplied by the
manifest, execution order, or other behavioral fields. The package object path
is derived solely as `objects/sha256/<digest>.spv`; package validation rejects
bad lowercase digests, duplicate IDs, broken references, unsupported schema or
ABI, malformed requirement kinds, experimental-to-production leaks, missing
objects, size mismatch, SHA-256 mismatch, unaligned payloads, and invalid
SPIR-V magic.

`reactor_shader_ids.generated.h`, emitted next to the distribution, is a small
projection of stable kernel IDs and the runtime ABI only. It contains no object
paths, payload words, catalog, or policy. `manifest.json`, not a C header, is
the canonical catalog.

## Build boundary

`internal/prometheus/Make.oct` owns `StageShaderPackage`, which invokes the
typed `oct sdslv package build` command. The historical PowerShell generator
is not used by this new staging route. The command is deterministic and does
not invoke package-provided build code; it only reads the repository-owned
source manifest and payload source headers, then writes content-addressed
objects. Package checking is a separate pure `oct sdslv package check` command.

## Remaining migration blocker

This foundation deliberately does **not** claim standalone native execution.
The native reactor still reads its immutable process-global embedded registry;
there is no per-runtime C loader yet and therefore no safe way to direct a
runtime at the staged package without retaining the forbidden embedded
fallback. Moving only SGEMM would make the package authority partial and leave
the model/reduction/ray-query paths inconsistent.

The next bounded recommendation is to implement `reactor_shader_package.[ch]`
with a per-runtime strict parser and integrity-checked module loader, then
migrate all direct module creation routes in one compatibility-preserving
series. Do not begin camera, image, or shadow work before that migration.

## Deferred by design

No registry service, transport, download, publishing, signing, semver solver,
dependency graph, archive, user cache, garbage collection, hot reload, runtime
DXC, or third-party package install is included. A future resolver may place
an exact manifest into this same local layout; the prospective native loader
need not know its transport.

## M1A native-loader/raw-hit slice (local worktree)

The M1A worktree adds `reactor_shader_package.[ch]`.  A runtime configured with
the struct-size-gated `PrometheusReactorConfig.shader_package_root` opens one
immutable, independently owned package before Vulkan initialization.  The
loader copies the root, parses the package identity and all six table families,
retains artifact/kernel/variant/requirement/implementation lookup data, and
releases it exactly once during runtime destruction.  The field is read only
when `offsetof(shader_package_root) + sizeof(shader_package_root)` is within
`struct_size`; an older configuration remains valid.

Objects are never addressed by a manifest path.  The loader derives only
`<root>/objects/sha256/<validated-lowercase-digest>.spv`, verifies the declared
length, SHA-256, word alignment, SPIR-V magic, entry point, and declared
`LocalSize`, creates the `VkShaderModule`, and immediately releases the object
bytes.  `prom_shader_package_artifact_open_count` is bounded diagnostic test
instrumentation; it is not selection policy.

The analytic/procedural raw-hit pipeline now asks the central seam for exact
variant `kernel-55-default`.  Its reactor no longer includes the generated
raw-hit header, and the raw-hit record/header were removed from
`reactor_shader_registry.c`; no compiled raw-hit payload can be selected as a
fallback.  The package builder still stages the verified historical generated
header as its source representation.  The remaining capability-probe shader
and every other shader family remain intentionally embedded for the next
mechanical migration.

The repeat scan after this slice is: 84 production generated-header includes,
57 embedded payload-array definitions, 696 payload-symbol mentions, 70
`sizeof(k_prom_...)` uses, and 29 direct `vkCreateShaderModule` calls.  The
last count is unchanged because
the one raw-hit call moved into the single package seam, which is now itself
the bounded module-creation site.  The registry now has 43 embedded assets;
raw-hit is package-only.  These are not claims that all remaining matches are
independent module paths.

On the local Windows Vulkan device, the focused M1A ABI/ownership test,
raw-hit cold/warm execution, complete raw numerical corpus, and mixed-scene
isolation all pass.  The warm assertion observed one object open at pipeline
creation and no additional opens across repeated dispatch.  The corpus maxima
were `t=9.22933504e-07`, position `9.22933504e-07`, normal
`9.22933504e-07`, and barycentric `1.3038516e-08`.

This is a vertical-slice result, not global M1 completion: SGEMM, reductions,
FFT, model blocks, DVT-2/Z-Image, capability probe, triangle/procedural
ray-query paths, and registry payloads remain embedded.  Linux and
validation-enabled RTX runs have not yet been collected in this worktree.

## Subsequent mechanical migration progress (local worktree)

The central seam now also creates the FFT, capability-probe, reduction,
resident-model-block, and M42/M44/M45/M46/M47 transformer modules.  The
reduction state borrows the immutable package from its owning common runtime;
it never owns or caches object bytes.  The package catalog gained explicit
numeric identities 56--66 for the cooperative and historical transformer
headers that previously had only textual experimental-manifest identities.
This is a catalog projection, not selector policy.

The current staged package check reports 66 kernels and 65 unique content
objects.  The duplicate difference is intentional content addressing: two
logical entries share identical SPIR-V and produce one object.  Windows native
build, smoke, reduction persistent-ring, and resident-model-block warm
authorities passed after this conversion.

This still does not meet the completed M1 claim.  The source scan is 67
production generated-header includes, 205 payload-symbol references, 54
`sizeof(k_prom_...)` uses, and 18 direct `vkCreateShaderModule` calls.  The
remaining compiled payload authority is concentrated in
`reactor_shader_registry.c` and the SGEMM module constructors, with the
registry still retaining the generated payload words for the selector metadata
API.  The strict native parser also needs its complete unknown-field and
malformed-package matrix before a complete schema claim is warranted.

The primary SGEMM runtime constructors subsequently moved kernels 1--14,
including the former inline baseline, to `prom_shader_package_create_module`.
The Windows reactor build, native smoke, and persistent-reduction authority
still pass with those modules loaded from the staged package.  This narrows the
remaining compiled authority to the registry's legacy payload-bearing metadata
and explicit audit/benchmark descriptor APIs, which still expose payload words
for test-oriented arbitrary-module creation.  They must be converted to
package identities (or confined to non-production fixtures) before claiming a
complete external-payload migration.

## Metadata-only registry and standalone baseline closure

The remaining binary registry projection was replaced with a compact native
metadata table: stable kernel IDs map deterministically to
`kernel-<id>-default`, while selector dispatch, admission, and non-binary
provenance facts remain native.  It has no SPIR-V pointer, payload byte count,
or generated-header include.  The only direct Vulkan module calls are the
verified package seam and one explicitly non-production arbitrary-SPIR-V audit
seam used by the audit benchmarks.

The former inline SGEMM baseline is now
`native/shaders/historical/sgemm_baseline_scalar.spv.base64`, a checked source
artifact decoded only by `oct sdslv package build`.  It preserves the exact
2,668-byte object digest
`e5610fc9a63daea8e83188a1cb9c63856225f85b2e6d61bd1e3fc7dcf560a5d9`.
Production C now has zero generated payload-header includes, payload-symbol
references, and embedded-payload `sizeof` expressions.  Audit tests load the
same staged object as test input; they no longer link the removed C symbol.

Windows package build/check, reactor build, native smoke, the registry
manifest-drift test, `go test ./internal/prometheus/...`, and
`go test ./internal/sdslv/bench` pass in this worktree.  The content package
remains 66 logical kernels and 65 digest-addressed objects.  The remaining
non-package SPIR-V text is historical/generation source or test-only audit
input, not compiled production reactor authority.
