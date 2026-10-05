# One definition per function: standard libraries, wrapper functions, hysteresis

Date: 2026-10-04
Base commit: `5465173` (the pass reported in
`utility_arms_vector_scope_2026_10_04.md`)

## Verdict

**Success** on both decisions taken after the previous report.

| Decision | Result |
|---|---|
| Option A for wrapper functions | Done. The standard libraries have one definition per function, a wrapper function has one definition, and a name with both is a compile error. |
| `hysteresis`, left to my judgement | It measures the leading arm against the committed arm's score at this evaluation. |

## Wrapper functions

### Before

Eleven standard libraries defined 45 functions twice. The interpreted lane ran
a source body, which called a builtin implemented inside `oct`. The compiled
lane threw that body away and substituted a call to a sidecar, described by a
`WrapperFunction` entry in the library's manifest. For 20 of the 45 the two
did not take the same arguments.

### Now

There are two kinds of function, and each has one definition.

| | A standard library function | A manifest wrapper function |
|---|---|---|
| Defined by | its source body | its `WrapperFunction` entry in `manifest.oct` |
| What it is | Oct code over builtins | native code outside the toolchain |
| Interpreted lane | runs the body; builtins run inside `oct` | sends the call to the sidecar |
| Compiled lane | runs the body; builtins go to a first-party sidecar | sends the call to the sidecar |
| Artifact evaluation | governed by the artifact effect rules | a native operation that needs a grant |
| Needs a sidecar | compiled only | both lanes |
| Example | `Hash.Sha256Text` | `Make.MakeToolRaw` |

A source function with the name of a wrapper function is rejected in both
lanes:

```
function WrapperDefinedTwice.Echo has two definitions: a source body, and an
entry in wrapper "test-wrapper" of the package manifest. A wrapper function
is defined by its manifest entry alone. To put Oct code in front of it, give
the manifest entry a name of its own and call that from the source function
```

The bodyless `go fn` declaration of an OctGo companion is exempt. It has no
body, and the OctGo host derives its wrapper entry from it.

### What changed

- **Compiled builtins.** `internal/builtin/sidecar.go` is a table of 39
  builtins with their sidecar, wire family, argument and result types and
  fallibility. The compiled lane lowers a call to one of them to the existing
  generic sidecar call. A handle is an `Int` in the builtin and a typed handle
  on the wire, so a sidecar still refuses a handle of the wrong kind.
- **Sidecars.** `octxiliary-pdf` takes the style of `PdfDrawTextStyled` as
  four integers, and `octxiliary-plot` takes size and labels as six flat
  arguments. They used to take `Pdf.TextStyle`, `Plot.Size` and `Plot.Labels`
  records, which only the manifest entries produced. The other first-party
  sidecars are unchanged; the test sidecar gained one function.
- **Manifests.** The eleven libraries declare no `Wrappers` and no
  `Kind: "wrapper"`. `Registry/registry.oct` lists them as `library`. `Make`
  is unchanged; its 15 functions were already defined by the manifest alone.
- **The rule**, in the typechecker, with the stub handling removed from the
  compiled lane.
- **Fixtures.** The generic wrapper fixture lost its stub bodies and its
  `[Compiled]` restriction and runs in both lanes. Two mismatch fixtures,
  which tested a stub disagreeing with its manifest entry, are replaced by one
  for the rule.

### Defects the stubs had been hiding

No function with a record or handle argument had ever been defined by its
manifest alone, because every such function also had a stub. Removing the
stubs exposed three defects, all fixed:

| Defect | Lane | Effect |
|---|---|---|
| The typechecker could not resolve a transport type that the manifest qualified with its own package's name | both | `unknown package 'Main'` for `Main.TestOptions` |
| The interpreter compared a record's local name with the manifest's qualified one | interpreted | `expects record Main.TestOptions, got TestOptions` |
| The compiled lane looked for an imported package's function among its source declarations only | compiled | `unknown function 'Pkg.Name'` for an imported wrapper function |

### What a compiled program gains and does not

A direct call to one of the 39 builtins now compiles, since the library's
call does. Eight library builtins still have no compiled implementation and
are refused by name, as before: `PdfDrawImage`, `PdfDrawImageSized`,
`JsonLower`, `JsonLoadStructured`, `CsvWriteTable`, `CsvWriteMatrix`,
`PlotLine`, `PlotScatter`.

### What this does not fix

Each of the 39 builtins is still implemented twice in Go, once in the
interpreter and once in a sidecar. The two were always separate; what is gone
is the second definition at the level of the language. Only the library
tests, run in both lanes with sidecars, compare the two implementations.
`FEEDBACK.md` has the entry, and `internal/plotrender` is the model for
closing it: both sides call one package.

Two orderings were checked by reading, not by a test. The plot sidecar reads
title, x label, y label and legend by position, and the PDF sidecar reads
size, red, green and blue by position; all are the same type, so no test
would notice two of them exchanged. They match the order in which
`Plot.Core.oct` and `Pdf.Core.oct` pass them.

## `hysteresis`

The policy used to compare the leading arm with the score recorded when the
committed arm was chosen. An arm committed at 10 whose score had fallen to 1
was held against a rival at 5, with `hysteresis: 2`.

It now compares the two arms' scores at this evaluation, which is what
"the rival must lead by more than `hysteresis`" says. The recorded score is
gone from the site state, from flow checkpoints and from the Verilog
profile's ports (`UtilitySite<N>Score`). Checkpoint versions are unchanged
from the previous pass, which was never released.

The 24 directories that use `when policy` give the same results as before in
both lanes. No existing program had a committed arm whose score moved enough
to matter.

## Process correction

The two earlier reports of this series listed two Go lanes, the default one
and `-tags=integration`. CI runs four. The other two are `-tags=toolchain`
and the slow wrapper lane (`toolchain` with `OCT_WRAPPER_PATH` and
`OCT_SLOW_TESTS=1`). Between them they hold 28 of `cmd/oct`'s test files,
every wrapper test among them. I had not run them. They are run here at three commits:

| Commit | `-tags=toolchain` | Slow wrapper lane |
|---|---|---|
| `e2637b9`, main | 69 ok, 1 fail | 68 pass |
| `c34b33c`, before the wrapper work | 71 ok, 1 fail | 65 pass |
| final | 71 ok, 1 fail | 65 pass |

The earlier passes broke nothing in them. The slow lane lost three tests
between the first two rows because those passes turned three Go tests into
`.octfail` contracts. The failing package throughout is
`internal/sdslv/test`, which needs `dxc`.

These lanes found two things in this pass that the others did not: the OctGo
`go fn` case above, and a Go test that named a deleted fact.

## Evidence

Machine: linux/amd64, 2 cores. "Before" is `5465173` for the sweeps without
sidecars and `c34b33c` for the sweep with sidecars; the two differ only by
the `hysteresis` change. "After" is the final commit.

| Check | Before | After |
|---|---|---|
| The 12 wrapper libraries, each from its own directory, sidecars present, interpreted | 89 pass, 5 fail | the same, test for test |
| The same, compiled | 71 pass, 23 fail | the same, test for test |
| Whole sweep, compiled, every sidecar present | 2369 pass, 168 fail, 2 skip | 2374 pass, 168 fail, 1 skip |
| Whole sweep, interpreted, no sidecar | 2493 pass, 40 fail, 5 skip | 2497 pass, 43 fail, 3 skip |
| Whole sweep, compiled, no sidecar | 2314 pass, 222 fail, 2 skip | 2315 pass, 227 fail, 1 skip |
| `go test ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| `go test -tags=integration ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=toolchain ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| Slow wrapper lane | 65 pass | 65 pass |

- **No existing test changed status** in any of the three sweeps. Every
  difference is a test that this pass added, removed, or released from a lane
  restriction.
- **With every sidecar present** the compiled lane gains five passing tests,
  all new, and loses one skip, a fact that was deleted with the stub it
  tested.
- **Without sidecars** the new tests of three fixture directories fail,
  because they exist to reach a sidecar. Two of those directories now fail in
  the interpreted lane as well as the compiled one: a wrapper function goes
  to its sidecar in both. `TestLanguageCorpusRunsInBothLanes` runs them with
  their sidecars, in the integration lane, and passes.
- **The library failures are the old ones.** Compiled: 12 tests of `IO` reach
  `JsonLoadStructured` and 6 of `Pdf` reach `PdfDrawImage`, neither of which
  the compiled lane implements. The other five, in both lanes, load image
  files that a Go test generates beside the library; they pass in the slow
  wrapper lane.
- `cmd/oct-mcp` timed out once in the default lane while another job was
  running. It passes alone, as it did in the previous pass.
- The Go failures are `internal/sdslv/test`, which needs `dxc`, and
  `internal/document`, which needs LaTeX.
- Both Verilog testbenches pass in Icarus Verilog 12, run by hand.

### Fault injection

33 faults, one at a time, in a separate worktree: 8 in the `hysteresis`
change, 8 in the one-definition rule and the three hidden defects, 14 in the
compiled builtin path and its table, and 3 in the two sidecars.

| | Count |
|---|---|
| Caught by the tests as first written | 27 |
| Did not build, or missed code that had since changed; rewritten and caught | 2 |
| Not caught; a test was added and now catches it | 1 |
| Not caught, with no observable effect today | 2 |
| Not caught, an invariant no program can violate | 1 |

- **The one that needed a test:** the plot sidecar's width and height
  exchanged. The sidecar test now reads the image back. That check found that
  a plot is 4/3 the size it is asked for, in both lanes; it is in `FEEDBACK.md`
  and not fixed here.
- **The two with no effect:** naming a wrapper function's records for the
  caller and for the callee as they cross a package boundary. The code mirrors
  what a source function's call does, and that code does not rename records
  at all, which is a defect of its own: in the interpreted lane a record of
  an imported package does not equal the same record returned by that
  package. It is in `FEEDBACK.md` with a reproduction and the reason the
  obvious fix is wrong.
- **The invariant:** the compiled lane's check that a bodyless `go fn`
  declaration agrees with the wrapper entry derived from it.

## Not done

- One Go implementation per builtin, as above.
- The eight builtins without a compiled implementation.
- `Language/Testing/CompiledOctxiliary` and `InterpretedOctxiliary` keep
  names that no longer describe them.
- `Libraries/Make/manifest.oct` declares a module directory that does not
  exist, as the eleven libraries' manifests did.
- `Experiments/` was not touched.
