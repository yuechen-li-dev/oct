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
| final | TOOLCHAIN_FINAL | SLOW_FINAL |

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

EVIDENCE_TABLE

### Fault injection

FAULTS

## Not done

- One Go implementation per builtin, as above.
- The eight builtins without a compiled implementation.
- `Language/Testing/CompiledOctxiliary` and `InterpretedOctxiliary` keep
  names that no longer describe them.
- `Libraries/Make/manifest.oct` declares a module directory that does not
  exist, as the eleven libraries' manifests did.
- `Experiments/` was not touched.
