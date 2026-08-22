# FLOW-TURN-M1 — Concept-aware inputs, yield checkpoints, and typed host ABI

## 1. Verdict

**Success**

The ordinary Concept/type path now reaches flow turn input correctly, and
embeddable compiled Go exposes typed construction, stepping, yielded values,
logical checkpoint export, and exact restore. The independent consumer probe
uses no Oct source, interpreter package, or compiler-private symbol.

## 2. M0 carry-forward

M0 already established typed `accepts`, non-completing `yield`, continuation at
the following instruction, turn-scoped input clearing, private-board and
resume/utility persistence, and rejection of locals that survive yield. M1
does not redesign those semantics or introduce a coroutine, actor, mailbox, or
second MIR/runtime.

## 3. Concept-aware `accepts`

`accepts input: T` is an ordinary type position. A record-shaped Concept is the
existing concrete nominal record product; its literal is accepted, while a
separately declared same-shape record is not implicitly converted. Transparent
aliases erase normally. Refined inputs use the ordinary proof/checked-
constructor rules. Nominal enums remain the closed-sum protocol choice,
including payload variants.

The audit found and fixed two general seams: Concept expansion had omitted flow
input/yield type references, and `Step` did not invoke ordinary refinement
admission after expected-type checking.

## 4. `satisfies` decision

No `satisfies` syntax was added. Current Concepts deliberately combine
transparent aliases, nominal record-shaped values, and refined admission. A
structural conformance relation would change general record identity and is
not justified by flows. The authoritative rule remains ordinary assignability
and admission.

## 5. Logical checkpoint schema

The versioned logical model owns package/flow identity, flow fingerprint,
named active state, continuation cursor, construction parameter schema and
values, board schema and values, resume slot, feature-observed history,
utility-site identity/current choice/score/commit age, yield schema, last yield
and presence, and host step accounting where applicable. The interpreter
schema is version 2; compiled facades emit the same semantic fields as
deterministic typed JSON behind a flow-specific checkpoint value.

Restore has machine-readable reasons for version, flow, fingerprint, missing
state, invalid continuation, board, construction, utility-site, and yield
schema mismatches. Generated consumer tests mutate each category.

## 6. Yield-safe persistence

A checkpoint represents the machine after yield. The cursor already points to
the following instruction. Turn input is cleared and omitted. The last yield
is retained solely so restored `DidYield`/`Yielded` observability is exact.
Static rejection of locals surviving yield means no environment or arbitrary
object graph is serialized; durable values must live in construction state,
board state, the resume slot, or utility state.

## 7. Compiled checkpoint export

`emitGoFlowHostFacade` is called only by embeddable source emission and wraps
the existing emitted flow struct. `Checkpoint` reads its typed persistent
fields at a yield boundary, validates lifecycle state, constructs the logical
payload, and uses deterministic `encoding/json` struct encoding. It does not
use reflection, pointers, ASTs, interpreter types, unsafe, or a generic object
serializer.

## 8. Compiled restore

`RestoreDurableController` validates every compatibility discriminator, maps
the stored state name back to the private state ID, validates the cursor,
materializes a fresh private generated flow, and assigns construction, board,
resume, history, scalar utility, and yield state. The proof compares an
uninterrupted run with checkpoint/restore on the next input. Both yield `29`
and have identical boards; the policy case is chosen so resetting commitment
state would incorrectly yield `28`.

## 9. Utility/resume/history handling

Scalar utility sites retain typed generated state and are checkpointed without
`any`. The resume slot is stored as a state name and revalidated. History is
included only when existing feature analysis says `StateHistory` is observed.
The interpreter now serializes supported scalar/array utility values rather
than rejecting all active utility state. Generic non-scalar utility-map values
remain explicitly rejected by the compiled host checkpoint API.

## 10. Generated Go host facade

Representative external use is:

```go
machine := generated.NewDurableController(1)
turn, err := machine.Step(generated.Main_TurnMessage{Amount: 3})
decision, err := turn.Yielded()
checkpoint, err := machine.Checkpoint()
restored, err := generated.RestoreDurableController(checkpoint)
```

The public surface contains no state/instruction integers. The serialized
cursor remains an opaque implementation detail inside the checkpoint bytes.
This is an experimental ABI, not an Oct 1.0 compatibility promise.

## 11. Concept input through Go facade

A record-shaped Concept uses the existing generated concrete Go struct, such
as `Main_TurnMessage`; no universal wrapper is needed. Refined Concepts use
their emitted Go alias representation, but the facade reuses the generated
checked constructor on every host Step and exposes `AdmitConcept` helpers, so a
base value cannot bypass refinement admission. Enums use the generated nominal
enum type plus typed exported variant constructors. The
current enum representation still has a private implementation-level `any`
payload field, but external construction is statically typed.

## 12. Independence proof

`TestEmbeddableFlowHostFacadeCheckpointRestoreAndIndependence` emits one Go
package into an empty temporary module and compiles a separate consumer package.
`go test ./...` passes there with no Oct source file, repository import,
generation hook, interpreter, or runtime source loading. The generated file is
48,906 bytes in the measured specimen, which includes four public flow facades.

## 13. Feature specialization

The facade is emitted only in embeddable mode. Normal compiled executables are
unchanged. Non-yielding flows do not get checkpoint payloads/helpers;
non-input flows do not get input fields or typed Step parameters; history,
resume, board, and scalar utility fields remain controlled by the existing MIR
feature analysis. Regular specialization structure tests remain green.

## 14. Dogfood

The language corpus contains direct record-shaped Concept input, compile-time-
proved refined input, nominal enum/payload input, a durable board/resume/
utility yielding flow, and a generator. The embeddable specimen exercises the
same durable controller, Concept record, enum constructor, and generator from
an external Go package.

## 15. Performance

Windows/amd64, Ryzen 7 7700X, three samples:

| Operation | Time | Bytes/op | Allocs/op |
|---|---:|---:|---:|
| typed facade `Step` + `Yielded` | 13.83–13.90 ns | 0 | 0 |
| checkpoint export | 540–614 ns | 752 | 2 |
| checkpoint restore | 3.49–3.51 µs | 808 | 18 |

Resident facade turns remain allocation-free. Checkpoint allocation is
intentional. Isolated checkpoint sizes are 286 bytes for a plain reactive
flow, 321 bytes for a boarded generator, 505 bytes for the boarded
resume/utility controller, and 341 bytes for a history-enabled boarded
generator. Identity/fingerprint/schema strings dominate the two smaller
payloads; typed board, utility, resume, and history fields account for the
incremental sizes.

## 16. Compatibility

Targeted `internal/concept`, `internal/typecheck`, `internal/interpret`, and
`internal/build` tests pass. New valid cases pass interpreted and compiled with
zero fallback. Existing M0 syntax and runtime behavior remain unchanged;
checkpoint version changed from 1 to 2 so old interpreter checkpoints fail
explicitly instead of being misread.

The repository-wide `go test -count=1 -parallel 8 ./...` passed every package
except the pre-existing `internal/conceptvulkan` checked-output lane, whose 18
EVT1 fixtures report `CV3001 stale or hand-edited generated output`. This is
the same unrelated drift recorded by M0; FLOW-TURN-M1 changes none of those
files.

## 17. Pressure findings

- **Bug, fixed:** Concept expansion omitted flow input/yield type references.
- **Bug, fixed:** refined Concept admission was bypassed by `Step`.
- **Compiler gap:** record-field access through an enum payload binding is not
  lowered in flow MIR; enum host dogfood therefore uses an `Int` payload.
- **API gap, fixed:** generated flows had only private host symbols.
- **Compiler gap:** generic non-scalar controller utility checkpoints remain
  unsupported; typed scalar utility sites work.
- **Documentation gap, fixed:** Concepts, flows, generators, yield lifetime,
  checkpoints, and the host ABI are now explicit in the reference.
- **Not worth changing:** nominal records are not made structurally assignable
  merely to imitate a separate message/conformance system.

## 18. OctetDB Write readiness

The requested architecture is now viable without changing Database-Scheduler:
a host can own a keyed registry and bounded mailbox, construct a generated Oct
flow, call typed `Step` with a Concept record or nominal command, read a typed
decision, commit externally, and persist/restore the compiled checkpoint. The
host remains responsible for mailbox ordering, database commit, and checkpoint
file durability.

## 19. Async/await readiness

FLOW-TURN is a sufficient semantic substrate for a future design in which
`await` lowers to the same explicit state machine and persistence boundary. No
async/await syntax or scheduler should be added until cancellation, error, and
host wakeup contracts are designed.

## 20. Remaining limitations

The checkpoint byte encoding and generated naming are experimental. Compiled
generic non-scalar utility-map state is rejected. Generated enums retain an
internal `any` payload representation despite typed public constructors.
Interpreter logical values currently checkpoint scalar/array construction,
yield, board, and utility values; general record/enum construction parameters
need an explicit recursive logical-value extension. History size isolation is
limited by module-wide builtin reachability. There is no source-level Oct
checkpoint builtin; compatibility failures are therefore host implementation
tests rather than duplicate `.octfail` semantics.

## 21. Exactly one next recommendation

Resume DBSCHED-M7 using the generated typed facade and compiled checkpoint,
while keeping registry, bounded mailbox, commit ordering, and storage in the
host.
