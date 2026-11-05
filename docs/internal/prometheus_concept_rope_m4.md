# Concept standalone RoPE and workspace cleanup

Date: 2026-11-05

Status: **MILESTONE VERIFIED — Success (end state A).**

Standalone Gemma positional encoding now uses the same Concept attachment and
pipeline as the normalized resident path. The remaining Gemma C file contains
only the pure RMSNorm planner and CPU comparison/reference routines. Its final
Vulkan executor, pipeline ownership, staging buffers, descriptors, barriers and
legacy state fields have been removed.

## Standalone ownership

`PrometheusRmsNorm.concept` owns four reusable standalone buffers: input, cosine,
sine and output. They are independent of resident Q/K and the score destination.
The shared `RecordRope` function records both standalone calls and the existing
normalization tail against package variant 68, through the established Concept
Vulkan pipeline/binding/recording APIs. No shader source or public packet layout
changed. The accepted geometry remains 15 tokens, one or eight heads, width 256.

The public source/table hashes retain their existing FNV-u32 serialization.
Uploads and final readback use the foundational shared-memory path. A warm call
allocates no buffers; growing from K to Q and shrinking again preserve the
logical extent. No command-buffer pooling or throughput improvement is claimed.

Each submitted standalone operation retains its owned submission until observed
completion. Record/submit failures preserve the caller's output; uncertain
completion retains all buffers. Retry drains the prior fence before overwriting
input. Detach and original-owner destruction drain before releasing resources.
The native adapter only validates/projects packets and uses the existing shared
queue authority. Calls on one owner remain externally serialized.

The GPU regression interposes a standalone call with different input while
attention Q/K are resident, then verifies the original score result. It also
checks exact BF16 half-pair arithmetic against an independent CPU oracle, both
head counts, growth/shrink/repeat, malformed alias/extent/nonfinite inputs,
one-shot record/submit/observation failures, retry, direct quarantine detach,
reattach on the original device, and automatic destruction.

## Issues closed while porting

- **FB-104:** The workspace checker now builds/checks the package catalog through
  `internal/prometheus/shaderpackage`, whose authority includes object digests and
  imported-source provenance. A bounded reader checks the existing native selector
  macros against package-owned IDs/names, entry points, languages and authority.
  It no longer mistakes absent source strings for missing ownership. Existing
  generated-header, declared source hash, module digest, benchmark identity,
  historical evidence and production/experimental checks remain active. Negative
  tests retain rejection of missing, duplicate, malformed and inconsistent rows.
- **FB-102:** Source-only Go emission returned before declaring `__octFloatLiteral`.
  The helper now precedes that return; process entry remains executable-only.
  The host boundary test reads the original scalar-loop recon specimen and
  type-checks its emitted Go, with no embedded replacement Oct program. It failed
  with the reported undefined-helper error before the fix and passes afterwards.

Both entries moved from open to resolved, preserving reproduction and evidence.
The tracker now has 7 open, 11 deferred, 95 resolved and 1 superseded issue. Its
example template uses `FB-NNN` rather than repeating a real issue identifier.
The workspace guide now describes the package catalog, native projection and
Concept runtime roles. The original FB-104/102 failures remain in
`.tmp/rope-m4-workspace-baseline.log` and `.tmp/rope-m4-fb102-baseline.log`.

## Verification

| Check | Result |
| --- | --- |
| Windows native package/DLL/test/benchmark/SDSL-V-host build | Pass |
| Standalone RoPE GPU regression | 1 passed, zero skips; exact BF16 oracle match |
| RMSNorm/resident attention/failure regressions | 4 passed, zero skips |
| M46 planning/oracle + matched-input GPU proof | 3 passed, zero skips |
| Reduction/softmax/SGEMM/public-ABI regressions | 27 passed, zero skips |
| Total selected native facts | 35 passed, zero skips/failures, validation enabled |
| SDSL-V imported-module GPU corpus | 3 passed |
| Full default `internal/build` Go test package | Pass |
| Workspace/shader-package/conformance/tooling Go tests and vet | Pass |
| Compiled FloatClamp01 facts | 2 passed, zero interpreted fallback |
| FloatClamp01 rejection corpus | 2 passed |
| Current workspace checker + native manifest parity | Pass |

The native checks ran on the RTX 3070. Logs use `.tmp/rope-m4-*`. Generated C,
headers, provenance, objects and archives stay under ignored `.native-build/`.
M3's existing Concept compiler fixes are retained; this slice changes no compiler
or foundational library code in that repository, so its long full qualification
was not repeated. Linux GPU execution and new performance qualification remain
unclaimed.

Representative commands:

```powershell
cmd /d /c internal\prometheus\native\build_windows.cmd
$env:PROMETHEUS_VK_VALIDATION = '1'
out/prometheus/native/marionette_tests.exe PrometheusConceptRope_
out/prometheus/native/marionette_tests.exe PrometheusConceptRmsNorm
out/prometheus/native/marionette_tests.exe PrometheusConceptAttention_
go test ./internal/build -count=1
go test ./tools/sdslv_workspace_check ./internal/prometheus/shaderpackage ./internal/sdslv/conformance -count=1
go run ./tools/sdslv_workspace_check
go run ./tools/prometheus_native_manifest
go run ./cmd/oct sdslv test Language/SDSL-V/Modules
go run ./cmd/oct test Language/Builtins/FloatClamp01/valid/float_clamp01_compiled.octest --execution compiled
```

## Remaining boundary

The former Gemma normalization/positional/score executors no longer need the
legacy mixed model state. Larger model-block/Z-Image owners still use that state
and remain separate port milestones. Open FEEDBACK entries are not treated as
closed by this cleanup unless the entry carries new executable evidence.
