# OCTGO-M2 — Riemann dogfood on a strict-threshold seam

## 1. Result and selected seam

M2 dogfoods OCTGO-M1 in `github.com/yuechen-li-dev/Riemann/semantic` with one
function:

```go
func StrictlyAbove(value, threshold int) bool
```

This is the scalar exact-fixture form of M10's strict spectral convention.
Equality belongs to the remainder. It was selected because the existing M10
Octest implemented exactly this comparison, its equality boundary is central
to the Weyl transfer, and both parameters and the result fit M1 without any
type/runtime expansion.

The helper is not an OctGo-only wrapper. `compiler.CompileM10` now uses it to
compute the exact `diag(2,1,0)` threshold-count sanity observation instead of
embedding the answer `1`. Riemann therefore owns and exercises the operation
in its ordinary production compilation path.

The richer exact-rational theorem machinery remains unchanged. In particular,
`ExactRational.GreaterThan`, `ThresholdedCountFromMoments`, theorem contracts,
evidence kinds, compiler IR, and asymptotic composition remain Riemann-owned.

## 2. Companion and retained scientific vocabulary

`semantic/semantic.contracts.oct` contains the sole imported declaration:

```oct
go fn StrictlyAbove(value: Int, threshold: Int) -> Bool
```

`StrictlyPositiveGap` remains in `semantic/semantic.octest` as readable
scientific vocabulary with bounded `Require(Self > 0, ...)`. It is not a Go
binding and does not call the imported function. A static companion `Require`
records that the equality gap is zero. `Require(StrictlyAbove(...))` remains
forbidden and was not enabled.

## 3. Migrated research cases

The production-backed suite reuses the existing M10 research cases:

- exact threshold equality is excluded;
- an above/equal/below `[Theory]` table;
- Weyl transfer at the exact operator bound;
- the known false generalization `theta < ||E||`;
- near/far cancellation.

The equality Fact and every theory row call real Riemann Go. The permanent
`ThresholdBelowOperatorBoundFails` counterexample also calls real Riemann Go:
the perturbed value `1` crosses threshold `0`, while the near value `0` does
not. Independent rank/norm research notes remain in the original experiment;
they do not pretend to exercise this scalar seam.

Passing these cases means only that a bounded scientific regression passed.
It is not formal proof and does not alter Riemann theorem certification.

## 4. Duplicate logic removed

Before M2, `experiments/m10_threshold_window.octest` contained a 12-line Oct
`StrictlyAbove` implementation (plus a three-line Concept), while the Go M10
sanity observation hard-coded the resulting count. After M2:

- duplicated Oct helper functions removed: 1;
- duplicated Oct executable threshold implementations remaining: 0;
- duplicated theorem proofs moved across the bridge: 0;
- `go fn` declarations: 1;
- Go helper: 6 physical lines including its ownership comment;
- Go compiler refactor: 7 added / 1 removed lines around sanity-counting;
- companion: 5 lines;
- companion Octest: 41 lines;
- manifest: 25 lines;
- generated bridge: 31 lines;
- isolated bridge module file: 8 lines;
- focused ordinary Go test: 21 lines;
- old M10 Octest: 71 lines; remaining independent file: 30 lines.

For the selected scalar seam, user-authored executable semantics changed from
one Oct implementation plus one hard-coded Go result to one Go implementation
and zero Oct implementations. The record-valued exact-rational comparison is a
separate production representation and was deliberately not projected through
OctGo.

## 5. Actual compiled call path

```text
semantic.octest Fact/Theory
  -> compiled Oct wrapper call StrictlyAbove
  -> Octxiliary v0 request
  -> generated semantic/octgo_bridge main dispatch
  -> semantic.StrictlyAbove(arg0, arg1)
  -> Bool response
  -> compiled assertion
```

There is no interpreted fallback, reflection, symbol lookup, duplicate
dispatcher, or Oct compiler import in Riemann production packages.

## 6. Check, test, and local two-repository workflow

The generated bridge is an isolated nested Go module so ordinary Riemann
`go build ./...` skips it. Its `go.mod` uses relative replacements for sibling
checkouts named `Riemann` and `oct`; committed files contain no absolute path.
This also works around an upstream Oct module-zip collision between `Examples/`
and `Examples/` on Windows.

With both repositories checked out as siblings:

```text
1. cd oct
2. go build -o <temporary-output>/oct ./cmd/oct
3. <temporary-output>/oct check ../Riemann/semantic
4. <temporary-output>/oct test ../Riemann/semantic
5. cd ../Riemann
6. go test ./semantic -count=1
7. go test ./... -count=1
```

No `GOWORK`, environment variable, machine-specific path, Oct invocation from
Go, background service, or generated-source mutation is required. Generation
is explicit via `oct check ../Riemann/semantic --generate`.

## 7. Results and warm workflow cost

On 2026-08-12, the final lanes reported:

- `oct check`: success; one selected function; bridge fresh; Go not executed;
- `oct test`: 7 passed, 0 failed, 0 skipped; compiled 7, fallback 0;
- warm `oct check` runs: 179.8 ms, 173.9 ms, 178.6 ms (median 178.6 ms);
- warm `oct test` runs: 1563.3 ms, 1536.3 ms, 1527.3 ms (median 1536.3 ms);
- warm `go test ./semantic -count=1`: 626.6 ms, 605.6 ms, 615.8 ms
  (median 615.8 ms).

These are local wall-clock observations, not performance guarantees.

The dogfood verification also exposed unnecessary Riemann test cost. Compiler
tests repeatedly rebuilt the numerical M6 result through every later milestone
(seven M11 compilations, four M10, two M9, and two M8). Test-only immutable
fixtures now compile once per milestone, and the M6 determinism check compares
two independent builds instead of four. Mathematical assertions and production
compilation code are unchanged. `go test ./compiler -count=1` fell from 34.203
seconds to 12.228 seconds (64% faster); the full race lane now completes in
81.931 seconds, while the pre-refactor run had not completed at the 244-second
tool timeout.

## 8. Failure diagnostics and stale behavior

Deliberate isolated edits produced repairable diagnostics:

- Go signature drift from `int` to `float64` printed both full signatures and
  `parameter 2 is incompatible: Go maps to Float, Oct expects Int`;
- Go function removal named the missing exported identity
  `github.com/yuechen-li-dev/Riemann/semantic.StrictlyAbove`;
- Oct contract drift from `Int` to `Float` printed both full signatures and
  `Go maps to Int, Oct expects Float` before any freshness complaint;
- a hand-edited generated family string reported the exact stale bridge path
  and the repair command using `--generate`.

Validation-only check preserved the stale file byte-for-byte: its SHA-256 was
`D5BCA5B0E2A6C57F781DAFCF4E1AD6C43D1450A2CA920DB22651401FE18D93C4`
both before and after the failing check. All deliberate edits were restored.

These messages are sufficient for a coding agent to repair the public seam
without reading OctGo internals.

## 9. Oct fixes required by dogfood

Riemann exposed two narrow M1 host defects, both fixed in Oct:

1. package projection dereferenced `Pkg()` for the predeclared named `error`
   type and panicked while scanning a real package; it now records an empty
   package path and reports unsupported signatures normally;
2. adapter building assumed every generated bridge shared the host package's
   Go module; it now recognizes `octgo_bridge/go.mod` and builds that isolated
   module directly.

Focused `internal/octgo` and `internal/cli` regressions cover both paths. No Go
type support, interpreter fallback, theorem machinery, or M3 architecture was
added.

Final ordinary Riemann verification passed: `go test ./... -count=1`,
`go test -race ./...`, `go vet ./...`, and `go build ./...`. Oct's focused
OctGo/CLI regressions passed. Oct's full `go test ./...` had only the known 18
unrelated `internal/conceptvulkan` `CV3001` checked-output failures; all other
reported packages passed.

## 10. Practical judgment

M2 materially improves this Riemann slice. A change to the production strict
comparison now has three cheap and distinct consequences: `oct check` catches
identity/signature/adapter mistakes, `oct test` catches equality and known
mathematical counterexample regressions, and ordinary Go tests remain the fast
independent implementation lane. The research file no longer maintains a
parallel executable comparison.

The gain is real but deliberately narrow. OctGo pays rent for stable scalar
theorem helpers with durable boundary/counterexample cases; it would not pay
rent for Riemann's rational records, matrices, multi-result APIs, or proof
graph. The nested bridge module and sibling-checkout assumption are visible
workflow costs, but about 1.5 seconds for the enhanced lane is reasonable.

OctGo is worth retaining as an optional Riemann research lane. M2 teaches that
Oct works best as typed semantic intent plus readable scientific regression
memory over Go-owned implementation—not as a second implementation language or
a proof engine.

## 11. Exactly one next recommendation

Return to **Riemann M12**. Do not expand OctGo while the one retained scalar
seam continues to provide evidence about its maintenance value.
