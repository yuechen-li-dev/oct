# How to approach Oct

Oct programs describe values and computation. Go implements the compiler and
runtime; you write Oct programs, tests, and contracts. This reference defines
the supported syntax and preferred style. [Language contracts](../README.md)
make the behavior executable. A disagreement is a gap to resolve explicitly,
not permission to copy an older workaround.

## Start with what you already know

Functions declare parameter and result types. `let` binds a value; `var` allows
rebinding. Conditions are Boolean. `for` visits a half-open integer range;
`while` follows an evolving condition. Arrays are values, and records are
updated by constructing a new value with `with`.

The main differences to learn early are:

1. Units are part of numeric types. A distance divided by a duration has a speed type.
2. A domain-shaped `concept` is a nominal value shape; an alias concept gives an existing type a transparent name.
3. A function declares failure as `T ! Error`; `?` propagates, `match` recovers, and postfix `!` deliberately unwraps.
4. An `Option<T>` represents ordinary absence and is read with enum `match`.
5. A captured function lists its environment explicitly. Arrays and captures keep value semantics.

Follow the [five lessons](README.md#learn-by-running-programs) before choosing
an advanced reading path.

## One language, distinct execution workflows

| Command | What it does |
| --- | --- |
| `oct run App/main.oct` | Executes source through the interpreter |
| `oct build App/main.oct` | Builds a standalone native executable through Go |
| `oct test App --execution compiled` | Requires native execution of selected tests, with no interpreter fallback |
| `oct test App --execution interpreted` | Checks those contracts in the interpreter |
| `oct test App --execution auto` | Tries compiled execution and reports permitted fallback |
| `oct artifact App --output-root out/generated` | Evaluates explicit build-time artifact entries and publishes confined outputs |
| `oct bench App` | Builds and runs benchmark entries; see the timing limits in the benchmark reference |

From the repository root, replace `oct` with `go run ./cmd/oct`. A native build
requires Go. A compiled program using wrapper-backed operations may also need
sidecars. Alternative targets admit smaller, explicitly documented subsets;
see [support](support.md) and [targets](tooling/targets.md).

`oct test` package selection matters: target a directory containing the test
package, such as `Language/ReferenceExamples/valid`. Running `oct test Language`
checks the negative corpus; it is not the whole positive language gate. The
[corpus guide](../README.md) gives the command for both full language lanes.

## Choose a construct by the work it owns

| Work | Start with |
| --- | --- |
| Compute one value | An ordinary function |
| Keep a domain value valid | A concept, units, or a refinement |
| Transform a local collection | A counted loop or eager array operation |
| Yield selected values from a source | A query |
| Compute independent item results | A batch |
| Make one ranked choice | `when utility` |
| Remember behavior across turns | A FLOW with explicit board state |
| Keep a repeated choice stable | A FLOW-local `when policy` |
| Pause a sequential computation for another FLOW | `async fn` / `await` |

The [glossary](glossary.md) explains the names. The
[coverage map](coverage.md) is the complete route into the detailed reference.
