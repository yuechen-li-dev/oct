# Concept EVT1 ergonomics thaw — 2026-11-05

Status: **MILESTONE VERIFIED** for the declared EVT1 C11 and Prometheus subset.

Scope: the three concrete Prometheus authoring frictions, explicitly authorized
as a small thaw. Concept base is `6756b94` plus the preserved ownership/Vulkan
work and these incremental changes. Oct consumer base is `eba4f392`.

## Resulting authoring path

```concept
Option<owned Buffer<byte>>[128] buffers;

export "C" int PromBlockDrain(ref BlockOwner owner)
{
    owner.detail = -6910;
    return BlockCode(ref owner, DrainBlock(ref owner));
}
```

Ordinary call arguments inherit the selected parameter's carrier context.
Nested `Option`/`Result` payloads, parentheses, conditionals, match expressions
and ordinary callees in template bodies use the same checker path. Existing
overload compatibility/scoring narrows candidates first; conflicting remaining
carrier contexts reject with `CV4182`. Exact error types, literal ranges and
explicit ownership/moves stay enforced.

`export "C"` functions expose exact symbols and a standalone hashed
`.exports.h`. It contains opaque referenced-object declarations and public
signatures. Scalars/foreign handles cross by value; concrete non-view objects
cross only as explicit mutable/const references. No hidden owner allocation,
owning parameter, borrowed return, private by-value layout, or automatic
Result/status translation is introduced. Cathedral rejects this export subset
until its native export linkage is independently qualified.

The concise fixed-array spelling was already implemented. The thaw makes
`T[N]` first-class alongside `T<array>[N]`, updates the canonical documentation
and uses it in the model-block source. Representation is unchanged and both
spellings emit byte-identical C/H for the equivalence specimen.

The first real native build exposed a precedence defect in
`OptionValue(ref const slot).field`. Concept's C lowerer now groups the payload
pointer; the runtime contract reads a const field and observes mutation through
another borrow of the same payload. The production accessors retain direct
field syntax. This is FB-118; the original call-context issue is FB-112.

## Prometheus boundary

The C11 adapter uses exported operations for buffer allocation/release/search,
borrowed buffer handles, device access, flags, completion/quarantine, counters,
timings, pipeline loading and execution. It no longer reaches into the owner's
fields or its buffer/pending carrier tags. Failure injection retains the original
phase consumption, completion quarantine and recovery semantics.

The private construction seam still allocates storage, inspects the creation
result and calls the explicit owner disposer. It also projects borrowed native
frames/recordings. Those are not advertised as a stable object ABI. Removing
them needs an explicit allocation/lifecycle design, rather than implicit boxing.

The generated public header is ignored and materialized by
`tools/prometheus_concept`. Its hash participates in deployment provenance;
stale/missing public headers are rejected just like C/H build products. The
native artifact manifest includes the public header as a disposable product.

## Verification

- Concept `TestErgonomics*`: strict C11 execution, negative payload/range/move/
  overload/export-signature cases, formatter and semantic-module roundtrips,
  identical array representation, explicit Cathedral export rejection.
- Independent C11 and C++23 consumers include only `.exports.h`; both link and
  execute against generated C in Normal and Verify modes. Three fresh complete
  generations in each mode match byte for byte (six total).
- Real CLI runtime contracts: four carrier/array/projection facts and one
  export-body fact pass in both Normal and Verify (ten fact executions).
- Focused adjacent carrier/context/storage/extern/scoped-cleanup regressions and
  unchanged checked-output artifacts pass. No checked outputs were regenerated.
- Native Windows launcher passes (142.080 s), using Clang C11/C++23 Concept
  archives and the MSVC host. The first failed native build is retained in the
  session evidence; it was repaired in the compiler and rebuilt.
- All 29 selected native GPU/integration tests pass, zero skips and no Vulkan
  lifetime/synchronization findings, including shared-device legacy SGEMM,
  failure/quarantine/recovery, resident rebind/static audit, BF16 ingress,
  reductions, RMSNorm, attention/RoPE and the real-weight AdaLN witness.
  Fresh logs: `out/test-artifacts/concept_evt1_ergonomics/`, summary
  `.tmp/ergonomics-native.log`. This retains the M5 test assertions unchanged.
- Regeneration after the final checker adjustment confirms all nine native
  C/header build inputs match the GPU-tested build byte for byte; current
  compiler provenance and `-check` pass.
- The unfiltered serial Go run was stopped after `TestEVT2h3Transport` exposed
  hash churn from adding a field to gob-serialized `FunctionDecl`. The final
  implementation carries an internal export marker in the existing attribute
  metadata; no new transported declaration field remains. Source authors cannot
  spell the reserved marker directly. Non-carrier parenthesized expressions also
  keep their existing typing/lowering path. Both frozen artifact hashes and the
  unchanged 100-fresh-generation transport checks pass after repair, along with
  the complete focused ergonomic/context/failure/extern/storage/cleanup lane.
  Evidence: `.tmp/ergonomics-concept-full.jsonl` (failed/interrupted, never counted
  green), `.tmp/ergonomics-repair-focused.log`. No frozen hash or codec was updated.
- The complete short compiler sweep finished in 510.213 s with one failing
  runtime root: discovery mistook the internal export marker for test metadata
  and tried to run three C entry points. Discovery now excludes that semantic
  marker. The export fixture has an explicit one-fact discovery regression.
  The complete Quick runtime/backend/CLI lane passes after this repair (36.15 s).
  The original short-sweep exit remains failed; only test discovery and its
  regression changed afterward, and the complete affected test-runner/native
  selection is rerun separately. Evidence: `.tmp/ergonomics-compiler-final.log`
  (initial failure), `.tmp/ergonomics-quick-final.log` (repaired pass).
- The enumerated native/framework run passes all 22 roots, zero skips/failures
  (559.151 s). It covers the complete test-runner selection plus ergonomic tests,
  frozen match transport and native match/Option/Result execution. Normal/Verify
  parity, invalid-tag traps, frozen code hashes and independent MSVC/Clang ABI
  probes pass. Log: `.tmp/ergonomics-native-framework-final.log`.
- Final negative-case review also rejects mismatched already-typed conditional
  carrier arms (`CV4116`) and verifies that parentheses preserve live-borrow move
  rejection. The borrow helper retains its original implementation; only carrier
  expressions receive the new parenthesized expected-type path. The complete
  focused typing/context/failure/storage/borrow/checked-output and match/Option/
  Result artifact selection passes afterward. Option retains its 100 fresh
  transport trials; Result retains its frozen hash and ten ordinary-milestone
  trials. Logs: `.tmp/ergonomics-final-regressions.log` and
  `.tmp/ergonomics-carrier-artifacts-final.log`.
  These final checks strengthen rejection of invalid programs and restore the
  existing borrow law; the valid native fixtures remain admitted with their
  checked artifacts. All nine Prometheus C/header inputs remain byte-identical,
  so the completed native execution evidence is reused for unchanged inputs.
- The final current-code Quick lane passes in 30.61 s, followed by a fresh
  `go vet ./...` pass. Current compiler provenance regeneration and `-check`
  pass with all nine GPU-tested C/header inputs unchanged.
- Concept `go vet ./...` passes. A complete unfiltered Go rerun and full checkpoint
  qualification are not claimed. Frozen root/legacy Zig suites are skipped because
  neither path nor its build infrastructure changed.
- Oct targeted Prometheus/generation/workspace Go tests and vet pass; the three
  `Language/SDSL-V/Modules` corpus cases and workspace check pass. Tracker counts
  are seven open, eleven deferred, ninety-nine resolved and one superseded,
  with 118 distinct issue IDs. Generated C/H, public headers and evidence remain
  ignored.

The original full O19 payload remains incomplete; these checks do not claim an
original full-image oracle run. This milestone does not qualify full shared/
prefetch model profiles or a general native/C++ object ABI.
