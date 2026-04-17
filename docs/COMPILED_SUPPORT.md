# `oct build` Compiled Support Matrix (M88)

This page is the repository truth surface for compiled mode (`oct build`).

Goal: keep the compiled boundary explicit and testable.

- **Supported** means there is compile+run coverage for that family.
- **Unsupported** means `oct build` fails with an explicit deterministic diagnostic (`compiled mode does not yet support ...`).

## Supported in compiled mode (current)

| Surface | Status | Evidence |
| --- | --- | --- |
| Functions, returns, variables, assignment, control-flow blocks | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunSubsetProgram`, `TestLowerProgramBuildsMIRShape`) |
| Records (construction, field access, updates), enums | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunCrossPackageFallibleAndEnum`, `TestCompileAndRunNamedRecordArraySurface`) |
| Arrays (including named-record arrays) and indexing | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunNamedRecordArraySurface`) |
| Fallible functions (`! Error`, `?`, `!`, `match`) | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunCrossPackageFallibleAndEnum`, `TestCompileAndRunFalliblePropagationAndMatch / TestCompileAndRunFallibleUnwrap`) |
| Package imports / cross-package calls | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunSubsetProgram`, `TestCompileAndRunCrossPackageFallibleAndEnum`) |
| `if` statements and `if` expressions (condition-switch style) | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunIfExpressionConditionSwitchSurface`, branch MIR tests) |
| `switch` expressions (subject + condition forms) | Supported | `cmd/oct/m19_enum_switch_test.go` (`TestM19EnumAwareSwitch`) + `cmd/oct/m21_string_ergonomics_test.go` (`switch expression string result`) |
| `while` statements | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunLoopLoweringParity`) |
| `for` range loops (`start..end`, `start..end step n`) | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunLoopLoweringParity`) |
| Flows (`Step`, `Active`, `Result`, `Complete`, `StateHistory`, `ResumeTarget`) | Supported | `internal/build/compiler_test.go` flow tests `TestCompileAndRunFlowCoreRuntimeBuiltins` |
| `when` in flow states | Supported | `internal/build/compiler_test.go` (`TestCompileFlowDecisionDoesNotUseSpecialCaseShimPath`, `TestCompileFlowBoardAndWhenActionBlock`) |
| Flow `board` fields | Supported | `internal/build/compiler_test.go` (`TestCompileFlowBoardAndWhenActionBlock`) |
| `remember` / `resume` in flows | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunFlowRememberResume...`, `TestCompileFlowRememberResumeMIRDump`) |
| `batch` expression | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunBatchParameterSweepAndOrder`) |
| Octagon I/O (`WriteOctagon`, `LoadOctagon`) | Supported | `internal/build/compiler_test.go` (`TestCompileAndRunOctagonRoundTrip`) |
| Selected string/util built-ins (`ToString`, `Float`, `Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`) | Supported | `internal/build/compiler.go` builtin lowering + compile tests |

## Intentionally unsupported in compiled mode (current)

These are rejected with deterministic diagnostics.

| Surface | Diagnostic shape |
| --- | --- |
| top-level statement `when` (non-flow) | `compiled mode does not yet support when` |
| standalone range expressions (outside `for` lowering) | `compiled mode does not yet support range` |
| vector literals | `compiled mode does not yet support vector literals` |
| matrix literals | `compiled mode does not yet support matrix literals` |
| utility `when` expression outside supported flow lowering path | `compiled mode does not yet support utility when` |
| unsupported built-ins (for example plotting, XLSX, trig/math, UI, Einstein/tensor helpers not lowered yet) | `compiled mode does not yet support builtin <Name>` |

## M86 hardening guarantees

- Unsupported features should fail early from lowering, not via accidental fallout.
- Unsupported built-ins now report as unsupported built-ins (not unknown function lookups).
- This matrix is **descriptive**, not aspirational. If support changes, update this page and tests in the same change.
