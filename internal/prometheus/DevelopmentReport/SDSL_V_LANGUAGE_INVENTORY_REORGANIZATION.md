# SDSL-V language inventory reorganization

## Outcome

This report records the evidence used to replace the milestone-accumulated
SDSL-V specification with a current implementation inventory divided into
Shared, Compute, and Graphics language sections. No parser, validator,
lowering, VD-MIR, emitter, fixture, or runtime behavior changed.

## Status methodology

The inventory uses the following evidence order:

1. lexer/token and parser acceptance;
2. AST representation and validator restrictions;
3. lowering and VD-MIR representation;
4. HLSL emitter support;
5. corpus/example coverage and, where present, DXC/SPIR-V/Vulkan evidence.

`[IMPLEMENTED]` requires end-to-end support for the stated restricted scope and
tests. `[PARTIAL]` identifies a parser/front-end-only or bounded path.
`[PLANNED]`, `[DEFERRED]`, and `[OUT OF SCOPE]` are used only where reports,
comments, or explicit implementation boundaries provide evidence. `[LEGACY]`
means accepted compatibility behavior, not removal.

## Files and areas audited

| Area | Primary evidence |
|---|---|
| Lexing/tokens | `internal/sdslv/lex/lex.go`, `internal/sdslv/token/token.go` |
| Parser and AST | `internal/sdslv/parse/parse.go`, `internal/sdslv/ast/ast.go` |
| Validation | `internal/sdslv/validate/validate.go`, `flow.go`, `tensor.go`, `testdecl.go`, `testinput.go`, and tests |
| Lowering and IR | `internal/sdslv/lower/lower.go`, `internal/sdslv/vdmir/*.go` and tests |
| Backend/toolchain | `internal/sdslv/emit/hlsl/*.go`, `internal/sdslv/toolchain/*.go` |
| Corpus | `internal/sdslv/testdata/language/**/*.sdslvvalid`, `.sdslvinvalid`, `.sdslvtest` |
| Examples/production | `Examples/SDSL-V/**`, `internal/prometheus/shaders/sdslv/production/**` |
| Runtime/reports | `internal/prometheus/DevelopmentReport/SDSL_V_M*.md`, `PROMETHEUS_SHADER_REGISTRY_GRAPHICS_BOUNDARY.md`, `docs/SDSL_V_M*.md` |
| Prior specification | former `docs/SDSL_V_LANGUAGE_SPEC.md` |

The audit also inspected `README.md`, `Language/reference/README.md`, and the
relevant repository instructions before documentation edits.

## Exact `switch` audit result

**SDSL-V switch status: NOT IMPLEMENTED.**

| Form | Token/parser | AST/validator | Lowering/backend | Fixtures |
|---|---|---|---|---|
| Condition `switch { case ... }` | No | No | No | No |
| Subject `switch value { ... }` | No | No | No | No |
| Statement-style switch | No | No | No | No |
| Enum switch | No | No | No | No |
| Enum `match` | Yes | Yes, exhaustive | Yes | Yes |
| Fallible `ok`/`err` match | No current parser form | No | No | No current SDSL-V fixture |

`token.go` has `match` and `case` keywords but no `switch` keyword.
`parse.go` has `parseMatchExpr` and `parseComptimeMatch`, but no switch parser.
`ast.go` has `MatchExpr`/`MatchArm`, but no switch node. `validate.go` checks
enum subjects, qualified variants, uniform arms, payload bindings, exhaustive
coverage, and direct-expression placement. Lowering materializes match; the
HLSL emitter emits the materialized branch structure. Its own `switch` output
is flow/test dispatcher implementation code, not source support.

## Major documentation moves

- Replaced milestone-first organization with Shared, Compute, and Graphics
  principal sections and compact implementation matrices.
- Moved lexical/declaration/type/control semantics to Shared; moved entry
  points, resources, barriers, flows, tiles, guards, tensors, test ABI, and
  Vulkan compute evidence to Compute.
- Created a non-empty Graphics inventory that distinguishes parser-reserved
  stage spelling from nonexistent graphics-stage lowering/pipelines.
- Split shaped storage (`ndarray`), construction (`Fill`/`Generate`), and
  indexed computation (`tensor`/`Sum`) into separate concepts.
- Moved implementation-history narration to the milestone appendix and isolated
  compatibility forms in a legacy appendix.
- Separated language semantics from VD-MIR/HLSL details and stated the bounded
  hardware-proof scope rather than treating generated HLSL as the language.

## Contradictions resolved in favor of implementation/tests

| Prior documentation claim | Implementation evidence | Resolution |
|---|---|---|
| Runtime condition and subject `switch` expressions exist | No keyword, parser, AST, validator, lowering, or fixtures | Removed as implemented syntax; listed as planned general selection only. |
| Runtime `match` has enum and fallible `ok`/`err` forms | Parser accepts qualified enum arms only; AST has enum fields only | Kept enum `match` as implemented; marked fallible form absent/deferred. |
| Graphics-adjacent stage words imply graphics support | VD-MIR has only `ComputeEntryPoint`; HLSL emits compute entries; no graphics runtime | Marked `vertex`/`pixel` spelling partial and all graphics pipeline features planned/deferred. |
| Milestone “adds” wording describes current status | Later code contains broader/narrower restrictions and proof levels | Replaced milestone labels with status vocabulary; preserved history only in appendix. |
| Historical M9 example is a current executable contract | `Examples/SDSL-V/M9/PayloadEnumBasic.sdslv` fails current parsing because `sum` is now a reserved token at line 24 | Did not alter language behavior or silently repair the example; recorded as a stale example/fixture inconsistency for a separate compatibility decision. |

## Implemented but previously under-emphasized

- Explicit source spans/provenance through diagnostics and generated foreign
  HLSL markers.
- Exact bounded `.sdslvtest` ABI ownership, typed Fact/Theory rows, and real
  Vulkan proof for the inline-HLSL test route.
- Flow stack constraints, barrier-aware flow validation, and the distinction
  between source transitions and generated dispatcher logic.
- `Fill` exactly-once operand semantics, Generate binder order, and ndarray
  separation from nested fixed arrays.

## Documented but unimplemented or overstated before this pass

- Both runtime general `switch` forms.
- Fallible runtime match syntax in the SDSL-V parser/AST currently audited.
- Any implication that vertex/pixel tokens establish graphics compilation.
- Any implication that texture/sampler declarations or arbitrary resource tests
  are usable through inline HLSL or `.sdslvtest`.

## Shared / Compute / Graphics summary

- **Shared:** declaration/type/expression fundamentals, fixed arrays and
  ndarrays, construction, ordinary/comptime control, enum match, diagnostics,
  attributes, and the bounded test surface are implemented at their stated
  scope. General switch and module linking are not implemented.
- **Compute:** this is the complete current execution model: compute entries,
  structured buffers, thread builtins, workgroup/local tile forms, guarded
  access, flows, tensors/Sum, HLSL escape hatches, and bounded Vulkan proof.
- **Graphics:** no graphics-stage model or pipeline is implemented. `vertex`
  and `pixel` spellings are parser-recognized only; interface/pipeline work is
  future repository-backed design.

## Unresolved questions

None block the reorganization. Fallible matching, general switch semantics,
and graphics-stage design intentionally require separate implementation/design
work and are not inferred by this documentation pass.

## Validation performed

Passed from the repository root:

- `go test ./internal/source`
- `go test ./internal/diagnostic`
- `go test ./internal/sdslv/...`
- `go test ./cmd/oct`
- `go test ./internal/... ./cmd/oct`
- `go run ./tools/prometheus_native_manifest -check`
- `bash -n internal/prometheus/native/build_linux.sh`
- `git diff --check`
- `oct sdslv check` for representative M0, M31b flow, M32b tensor, M33a
  ndarray, and M33b construction examples; `oct sdslv test ... --list` for the
  M33b test suite.

The representative M9 enum example did **not** pass current parsing:
`PayloadEnumBasic.sdslv` uses the now-reserved word `sum` as a local name. That
pre-existing inconsistency is documented above and was intentionally not fixed
in this no-behavior-change documentation pass.
