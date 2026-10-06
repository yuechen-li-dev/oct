# Json v2 — M1: `Option<T>`

Date: 2026-10-06
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `7d2fade`

## Verdict

**SUCCESS.** `Option<T>` is a builtin enum with the variants `None` and
`Some(T)`. It works in the typechecker, the interpreted lane and the compiled
lane, as Octagon data in both loaders and both writers, in `oct fmt` and in the
editor grammar. The Verilog profile and the WebAssembly target refuse it by
name. The risk clause did not fire: nothing here is a template enum, and
`template enum` is still unsupported.

Three things needed your eye. They are under "Decisions to confirm", and all
three were confirmed on 2026-10-06.

## What a user can now write

```oct
record Reading {
    Station: String
    Level:   Option<Float<m>>
}

fn Half(value: Int) -> Option<Int> {
    if value % 2 == 0 {
        return Option.Some(value / 2)
    }
    return Option.None
}

let wet = Reading { Station: "A" Level: Option.Some(1.5m) }
let dry = wet with { Level: Option.None }

let level = match wet.Level {
    case Option.Some(v) => v
    case Option.None => 0.0m
}

if dry.Level == Option.None { ... }

let none = Option<Float>.None          // nothing declares the type, so it is written
```

- `Option.None` and `Option.Some(value)` take `T` from the place the value
  goes. `Option<T>.None` and `Option<T>.Some(value)` write it.
- `T` is never worked out from the payload. `let x = Option.Some(1.5)` is an
  error that says to declare the type or to write `Option<T>.Some(...)`.
- The payload is a `T` as a declared type decides it: `Option.Some(1)` in an
  `Option<Float>` holds `1.0`, in both lanes.
- `Option<A>` and `Option<B>` do not convert, and a value is not its option:
  `let x: Option<Int> = 5` is an error.
- Everything an enum does, an option does: exhaustive `match`, `switch` on
  `Option.None`, `==` and `!=`, fields, table cells, arrays, flow boards,
  parameters, turn inputs and `yield`, results, template arguments, enum
  payloads, `when utility Option<T>`.

The reference is `Language/reference/language/12-enums.md`, section "Option".

## How `T` reaches the two lanes

The typechecker is the only pass that knows the type a place declares. The
interpreter has no static types, and lowering types expressions on its own.
Had each lane worked out `T` again, there would have been three
implementations of "where does `T` come from" to keep equal.

So the typechecker writes `T` into the program. The parser gives all four
spellings one shape, a call of `Option.Variant` with one type argument. Where
the source wrote none, that argument is an empty slot marked `Inferred`, and
the typechecker fills it. A pass that runs after the typechecker reads every
Option construction as if its type argument had been written, and neither
lane contains any inference:

- the interpreter converts the payload to `T` at the construction;
- lowering builds the type string from `T` and coerces the payload to it.

A lane that meets an unfilled slot stops with "reached the interpreter
(lowering) without the type the typechecker gives it". Go tests hold both to
that.

Two other mechanisms were tried and dropped:

- **Monomorphising `Option<T>` through the template elaborator.** A template
  is instantiated in each package that uses it, so `Option<Int>` in two
  packages would be two types; and the elaborator runs before types exist, so
  it cannot type `Option.None`.
- **Letting each lane infer.** The interpreter would have stamped option
  values with their payload type and converted late, at declared sites. It
  was wrong for `var xs = [Option<Float>.None]` followed by
  `xs[0] = Option.Some(1)`, where no value is at hand to say what `1` should
  become.

Compiled representation: `Option<T>` is one Go struct for each `T` a program
names, with the layout of every compiled enum. A first version used a Go
generic type; `TestTemplateTortureM0ErasesAndEmitsDeterministicGo` forbids
that shape in generated code, and it was right to. The Go name spells the
payload's type string reversibly (`Option<Float<m>>` is
`__octOption_Float_3cm_3e`), so the emitter finds the option types a program
uses in the Go it has just written and declares those. A program that names
no option gets no declarations; a program that uses the Octagon builtins gets
one more lookup function either way.

## What changed

| File | Change |
|---|---|
| `internal/ast/program.go` | `AsOptionConstruction`, `TypeRef.Inferred`, the names of the type and its variants. No new node type. |
| `internal/parse/parse.go`, `data.go` | The four spellings. `Option.None(...)` is a parse error. A record, enum, concept, function, flow or package named `Option` is a parse error. `Option.Some(value)` and `Option.None` are Octagon data. |
| `internal/project/parametric.go` | `Option<T>` passes through the elaborator; it is not a template. |
| `internal/typecheck/option.go` (new), `typecheck.go` | The type, its two variants as an enum, construction, the places that give `T`, switch labels, utility candidates, and `typeRefOf`, which writes a type back as source would. Each checker keeps the option types it has formed by name, because a type name alone does not say what its payload is. |
| `internal/interpret/option.go` (new), `interpret.go`, `octagon_load.go`, `octagon_emit.go`, `flow_checkpoint.go` | Construction, the builtin declaration, Octagon load, checkpoint restore of an option board field. An option value is an enum value named `Option`; it belongs to no package. |
| `internal/build/option.go` (new), `lower_expr.go`, `emit_go.go`, `emit_go_value.go`, `emit_go_runtime.go` | Type strings, the Go type and its declarations, construction, `match`, `switch` labels, equality by value, the Octagon loader's description of an option type. |
| `internal/build/lower_value.go` | **Compiled-lane bug fix, not specific to Option.** See "Bugs found". |
| `tools/vscode-oct/syntaxes/oct.tmLanguage.json` | `Option` is a type name. |
| `Language/reference/language/12-enums.md`, `02-types.md`, `tooling/34-octagon.md` | Reference. |

`oct fmt` needed no change. It works from tokens, and formats all four
spellings.

## Tests added

| Where | What |
|---|---|
| `Language/Types/Option/valid` (3 files, 36 facts, both lanes) | Every place that gives `T`; the written form; `match`, `switch`, equality for each scalar and for array, record, enum and nested option payloads; records, `with`, tables, arrays as values, nested arrays; flows with option parameters, board fields, board elements, turn inputs and yields; a board field that starts as `None`; template functions and records over `T`, and an option as a template argument; fallible functions; function values and captures; refined, dimensioned, vector, matrix and function payloads; `when utility`; `Assert.Equal`; `Append`. |
| `Language/Types/Option/invalid` (41 `.octfail`) | No type at the site, for each variant; wrong payload type; `Option<A>` where `Option<B>` is declared; a value where its option is declared and the reverse; non-exhaustive `match` and `switch`; bare `Some` and `None`; unknown variant; arity; `Option<Void>`; missing and extra type arguments; declaring `Option`; `?` and `!`; equality across option types and with the payload; ordering; fallible, `Void` and refined payloads; switch labels; utility candidates. |
| `Language/Types/Option/packages` (5 facts, both lanes) | An option of an imported record and of an imported enum, built on either side of the package boundary, compared, and loaded from Octagon. |
| `Language/Data/Octagon/Load/valid`, `Load/invalid` (8 facts, both lanes), fixtures under `Language/Data/Octagon` | Load of options in fields, arrays, nested, with dimensions, at the top level. Refusals: a plain value where an option is declared and the reverse, a payload of another type or dimension, another enum, a malformed option, a written type argument. |
| `Language/Data/Octagon/Emission/valid` | The emission contract writes an option and an array of options. |
| `Language/Types/EnumsAssociated/valid/enum_as_enum_payload.octest`, `Language/Packages/ImportedRecordIdentity` | Contracts for the first two bugs below; neither uses Option. |
| `Language/Profiles/VerilogM0/invalid/option.octfail`, `internal/wasm` | The two refusals. |
| Go, host side | `internal/parse/option_test.go` (the node shape, labels, malformed syntax, `Option < limit` still a comparison); `internal/typecheck/option_test.go` (the typechecker writes `T`, and again on a second pass); `internal/build/option_test.go` (type strings, Go names and their read-back, declarations, lowering refuses an unchecked program); `internal/interpret` (the interpreter refuses an unchecked program, the Octagon golden, a checkpoint round trip); `internal/build/compiler_test.go` (compiled write and load, against the same golden). |

The Octagon write is checked in Go because a `[Fact]` has nowhere to write a
file. Both lanes write `Language/Data/Octagon/valid/option_written.octagon`
byte for byte.

## Bugs found

Fixed here, each with a contract, because an Option contract could not pass
without the fix:

1. **Compiled: an enum written directly as the payload of another enum, and
   bound to a variable, was not the payload.**
   `let full = Crate.Holding(Parcel.Weighed(2.5))` built a `Crate` holding
   itself, and the `match` that read it panicked. The adapter from lowered Go
   text to MIR took the text of the whole expression for an inner expression.
   This is ordinary enums, on `main` today.
2. **Interpreted: a record returned by another package's function was not
   equal to the same record built by the caller**, and an array literal of
   one of each was "mixed element kinds". The record kept its unqualified
   name when it left its package; an enum did not. Contract:
   `Language/Packages/ImportedRecordIdentity` (3 facts, both lanes), which
   needs no Option. This fix is its own commit.

   It changes what the interpreted lane prints. `Geometry.Origin()` printed
   `Point{X: 0, Y: 0}`, and `Geometry.Point { X: 0 Y: 0 }` written in Main
   printed `Geometry.Point{X: 0, Y: 0}`. Both now print the second form.
   **One existing Go assertion pinned the first form and was changed:**
   `TestM18PackageCoexistenceWithMutableLocalReassignment` now expects
   `Geometry.Point{X: 3, Y: 4}`.
3. **Interpreted: `LoadOctagon<Lib.Record>` from another package failed** on
   any field whose type belongs to `Lib`. Field types were resolved in the
   loading package. M3 loads JSON through these materialisers, so this would
   have surfaced there.

The fix for 2 made the interpreted Octagon writer meet qualified record
names, which it refused, as it already refused qualified enum names. It now
writes a type under its own name, which is what the compiled writer writes.

Found and not fixed (all in `FEEDBACK.md`, Open):

- `WriteOctagon` then `LoadOctagon` loses a `Float` with a whole value in
  both lanes (`1.0` is written `1`, and refused as an Int), and the compiled
  writer writes no dimensions at all.
- In the compiled lane two packages cannot each declare an enum of one name
  (`Mode_Fast_tag redeclared`).
- In the compiled lane `==` on an ordinary enum whose payload holds an array
  panics. Option does not: its `==` is by value.
- A `match` case label's enum name is not checked: `case Anything.Some(v)`.
- Compiled `BoardSnapshot` refuses a package with two flows of one result
  type.

## Where the ladder text was wrong

Section 3.1 is amended in place, with the old wording noted.

- It said `==` and `!=` "where `T` has them". An enum compares for every
  payload type, so an option does.
- Its list of places that give `T` was short. An assignment, an enum payload,
  the other operand of a comparison or of `Assert.Equal`, the second argument
  of `Append`, a turn input, a `yield` and a utility candidate give it too,
  and `if`, `match` and `switch` pass it to their arms. Without these,
  `x == Option.None` and `current = Option.None` would have needed the
  written form.

## Verification

Run on commit `2420f77`, linux/amd64, 2 cores. The baseline is the set of
sweeps recorded on `05fbb87`; between that commit and the base of this
milestone only documents and one `.octfail` changed.

| Run | Baseline | After M1 |
|---|---|---|
| Whole tree, interpreted | 2757 pass, 16 fail, 3 skip | 2810 pass, 16 fail, 3 skip |
| Whole tree, compiled, no sidecars | 2488 pass, 287 fail, 1 skip | 2541 pass, 287 fail, 1 skip |
| Whole tree, compiled, with sidecars | 2552 pass, 223 fail, 1 skip | 2605 pass, 223 fail, 1 skip |
| Wrapper libraries, both lanes | 160 pass, 28 fail | identical, test for test |
| `.octfail` under `Language` (`oct test Language`) | 509 pass | 551 pass |
| Go, default lane | 71 packages ok, 1 failing | the same packages |
| Go, integration lane | 70 ok, 2 failing | the same packages |
| Go, toolchain lane | 71 ok, 1 failing | the same packages |
| Go, slow wrapper lane | passes | passes |

No test changed from pass to fail or from fail to pass in any run. The 53
new passes in each sweep are the facts this milestone added; the 42 new
`.octfail` are its 41 and the Verilog one. The failing Go packages are the
ones that failed before (`internal/sdslv` in every lane, `internal/document`
in the integration lane), with the same failing tests.
`TestLanguageCorpusRunsInBothLanes` is green and runs the three new
directories in both lanes.

The sweeps visit directories that hold `.octest` files, so they do not run a
directory of `.octfail` alone. Those were run separately on both binaries:
98 directories, 509 contracts before and 551 after, all passing.

Fault injection: 113 faults in the code this milestone added, each built and
run against the contracts and Go tests that should notice it. 112 were
caught. The first pass caught 91 and missed 17, and 5 faults were written
badly and did not build. The 17 led to the removal of an unreachable check in
three places, one change of behaviour (ordering an option against
`Option.None` now reports the ordering, not a missing type), new or
corrected facts and `.octfail`, and four Go tests. The one survivor is equivalent: in the
compiled equality, a `nil` payload on one side only cannot occur, because
the tags are compared first and `None` alone has no payload.

## Decisions to confirm

1. **The typechecker now writes into the program it checks.** This is the
   first thing a later pass reads from the typechecker. It is one slot, in
   one kind of node, and both lanes fail loudly if it is empty. The
   alternative is the status quo, in which each lane works types out again;
   the lanes have disagreed that way before (the declared-type work of
   2026-10-05). If you would rather the typechecker stay a pure checker, the
   cost is the two lane-side inferences described above, and I do not
   recommend it.
2. **`Option` is refused as the name of a declaration, and allowed as the
   name of a variable.** After `let Option = ...`, `Option.x` still reads as
   a variant. *(Settled 2026-10-06: the parser resolves the name by scope, as
   it does `vector`, so `Option.x` reads the value where one is in scope.)*
3. **The change to printed output in fix 2.** An imported record now prints
   one way where it printed two, and I changed the one Go assertion that
   pinned the other form. The fix is commit `8ce4249`, alone, so it can be
   reverted alone; two Option facts that compare an imported record with one
   built by the caller would then fail in the interpreted lane.

## Next

M2, `internal/octjson`: parser, schema, decode and encode in Go, with no Oct
wiring. Nothing in M1 changes its scope. The Octagon writer defects above do
not block the ladder, since JSON has its own writer (M4); they are worth a
fix of their own before anyone relies on `WriteOctagon` for `Float` data.
