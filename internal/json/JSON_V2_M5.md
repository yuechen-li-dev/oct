# Json v2 — M5: migration and removal

Date: 2026-10-07
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `eb0d5d9` (M4)

## Verdict

**SUCCESS.** The first Json library is removed, and every Oct file that used
it writes and reads typed values. Oct has one `Json`: `Json.Load<T>`,
`Json.Parse<T>`, `Json.Save`, `Json.Text`, and `Artifact.WriteJson` during
artifact evaluation, compiled with no sidecar.

One thing needs a decision from you, and five are not what the ladder's M5
says:

- **Decision: one recorded file was not regenerated.**
  `Evt2OctOracle/o0_structural_witness.json` has its SHA-256 recorded in the
  campaign ledger. See "The witness the ledger pins".
- **A rule of M4 changed.** During artifact evaluation `Json.Load<T>` reads
  an output of the phase. M4 refused every load. See "Artifact evaluation".
- **Two published files keep their keys.** Their records declare fields in
  the spelling of the schema they are published under. See "Keys".
- **`FmBrownNoiseKalman/M2` still cannot be evaluated**, for reasons that are
  not JSON, so its JSON was regenerated in a scratch copy.
- **The exit `grep` is not met to the letter.** See "Exit".
- **Inside package `Json` the bare names are not builtins.** `Random` and
  `Entropy` have that; `Array` and `Artifact` do not, and `Json` is as they
  are. Nothing is written inside package `Json` but its marker.

## What is removed

| | |
|---|---|
| Builtins | `JsonNormalize`, `JsonParse`, `JsonStringify`, `JsonLoad`, `JsonSave`, `JsonLower`, `JsonLoadStructured`, in the typechecker, the interpreter and the compiled lane; the typechecker's special case for a type named `JsonRawGraph` |
| `IO` | `IO.Json.oct` (410 lines: the raw graph, intent recovery, `Load`, `Save`, `Parse`, `Stringify`) and its 12 tests; 2 JSON facts of `IO.CoreWrappers` |
| `Json` | `Object`, and the Oct declarations of `Load` and `Save` |
| Sidecar | `cmd/octxiliary-json`, its entry in `tools/build_sidecars`, and the Go tests that drove it |
| Coexistence | The three rules that let both libraries be called: `Json.Load(path)` with no type, and a String given to `Json.Save` or `Artifact.WriteJson` taken as JSON text |

Go: 33 files, 122 lines added and 806 removed.

`Json` is a compiler-owned namespace wholesale. `Libraries/Json` holds a
marker so that `import Json` resolves, a README, and three tests. Package
`Json` cannot declare a function named after one of its builtins.

What the removed forms are now:

| Was | Is |
|---|---|
| `Json.Load(path)` | A compile error: "function 'Json.Load' expects 1 type argument, the type to read, got 0" |
| `Json.Save(path, text)`, `Artifact.WriteJson(path, text)` | The String is written as a JSON string |
| `Json.Object(text)`, `IO.Load(path)` | "package 'Json' has no function 'Object'", and the same for `IO` |

## What the 36 files became

| | Oct files | Change |
|---|---|---|
| `PrometheusShadowAuthorityRakeLab` M1 to M5 | 10 | One `ScenarioSummary` record each; the `[Artifact]` function loads it back and checks its count |
| `PrometheusNumericalHeterogeneityLab` M0, M1 | 4 | `M0SummaryJsonText` and `M1SummaryJsonText` built text; `M0BuildSummary` and `M1BuildSummary` build a record |
| `FmBrownNoiseKalman` M1 to M6 | 9 | Every summary is a record; M1's has nested records and an array. M2b's `.octagon` report and Markdown summary also held JSON text, and are a record and a key-value table |
| `OctErgonomicsLab` M0, M1 | 2 | One record each |
| `ZImageTurboMainTransformer0`, `ZImageTurboNoiseRefiner0` M0 | 2 | A JSON text literal each; now a record in its schema's spelling |
| `JsonIntentRecoveryLab` M0 | 1 | The test is removed; the lab is closed |
| `Libraries/ArtifactUsage`, `Language/Tooling/Artifacts/valid` | 2 | A record given to `Artifact.WriteJson` and `Json.Save` |
| `Libraries/IO`, `Libraries/Json` | 6 | Removed or rewritten, as above |

No Oct source builds JSON from strings. Most sites were converted by a
script that reads the `String.Concat` list and refuses anything it does not
fully understand; the nested ones were written by hand.

Each experiment's own report or README has a short section on the change.

## Artifact evaluation

During `oct artifact` evaluation `IO.ReadText`, `IO.ReadLines`, `IO.ReadBytes`
and `IO.Exists` read an output the phase has already published, where it is
staged, and refuse any other path. The first library's `JsonLoad` was refused
outright, and M4 gave `Json.Load<T>` the same refusal.

Seven recorded experiments read their JSON summary back inside the
`[Artifact]` function: Rake Lab M1 to M5 and Numerical Heterogeneity Lab M0
and M1. **None of the seven could be evaluated before M5.** The migration
could not regenerate their JSON without either deleting the read-back or
fixing the rule.

`Json.Load<T>` now follows the rule the text readers have, through the same
seam (`prepareArtifactRead`). A failed read-back names the path the program
wrote, not the staging directory (`octjson.LoadFileAs`). Its own commit:
`b5c6f4e`.

The rule was in the code and not in the reference for any reader. It is in
`31-octest.md` now. The CSV readers are still refused outright, which is why
`FmBrownNoiseKalman/M2` cannot be evaluated; that is in `FEEDBACK.md`.

## Keys

A record is written with its field names as declared (3.6). The ladder said
the keys of the recorded files change from `totalCases` to `TotalCases`, and
for the summaries of the experiments they did.

The two Z-Image witnesses are different: each is published under a named
schema (`oct.prometheus.evt2.o0.structural-witness.v1`) whose keys are
`token_0`, `rope_axes`, `qkv_order`. Changing them would break the schema
while keeping its name. Their records declare the fields in the schema's
spelling, so the keys, their order and their values are what they were. The
reference says so in one sentence. There is no renaming mechanism, and this
needs none.

## Recorded outputs

Checked by evaluating the artifacts of all 20 directories that write JSON
into a scratch output root, before the migration and after it, and comparing
every published file with the recorded one.

| | Before M5 | After M5 |
|---|---|---|
| Directories that evaluate | 12 of 20 | 19 of 20 |
| Published files identical to the recorded ones | (JSON differs by design) | 60 |

- **15 recorded `.json` files regenerated.** Each holds the same values as
  before, compared as parsed JSON with keys folded; only then was it
  replaced. Every non-JSON file those directories publish is byte-identical.
- **Not regenerated:** `o0_structural_witness.json` (next section).
- **`PrometheusShadowAuthorityRakeLab/M3/FINDINGS.md` is left as recorded.**
  It is not the file the `[Artifact]` function writes: it is longer and has a
  summary the function does not produce. This could not be seen before M5,
  because the directory did not evaluate.
- **`FmBrownNoiseKalman/M2` fails as it did**, on `CsvRead`, and has two more
  faults behind that one (a second `[Artifact]` function that publishes the
  same files, and a progress file published once per sweep case). Its
  `metrics.json` was regenerated in a scratch copy with the three set aside:
  same values. Its recorded `metrics.csv` and `m2a_report.md` hold other
  numbers than the code now computes; the experiment's report already says
  they describe the Random v1 noise. Left as recorded.

### The witness the ledger pins

`internal/prometheus/DevelopmentReport/artifacts/Evt2OctOracle/experiment_ledger.json`
records, for the completed O0 experiment, "witness SHA-256 6292c9e2..." and
`"artifact_identities": ["o0_structural_witness.json:6292c9e2..."]`.

`Json` writes an indented layout and the recorded witness is compact, so the
same JSON is other bytes. I did not regenerate the file and did not touch the
ledger: the recorded file is still the one the ledger names.

The cost: `oct artifact Experiments/ZImageTurboNoiseRefiner0/M0` now rewrites
that file with another SHA-256. Either regenerate it and re-pin the ledger,
or keep the recorded bytes as the evidence of that campaign and accept that
the command no longer reproduces them. That is your call; the ledger is a
campaign record and I do not think it is mine to rewrite. I found the pin by
searching for the file's name. A hash recorded without the name would not
have been found.

## Contracts

`Language/Builtins/Json`, every directory in both lanes:

| | |
|---|---|
| `valid` (46 facts) | Added: a String is saved as a JSON string and loaded back. Removed: the two facts of the first library beside the new one |
| `invalid` (62 `.octfail`) | Added: `Json.Load` with no type argument; `Json.Object`; a redeclaration in package `Json`; a read-back that fails names the published path. Changed: a load of a file the phase did not publish is refused with the text the text readers give |
| `artifact` | The `[Artifact]` function loads back the record and the table it published |
| `packages` (4), `corpus` (7) | Unchanged |

`Libraries/Json`: three tests, both lanes, no sidecar.

## Verification

Run from a clean checkout of the committed tree.

- **Go tests:** `octjson`, `jsontype`, `dimension`, `builtin`, `parse`,
  `typecheck`, `interpret`, `build`, `project`, `tester`, `ocfmt`: all pass.
  `octjson` and `jsontype` are at 100% of statements. `go build ./...` and
  `go vet -tags toolchain ./cmd/oct` are clean.
- **Corpus:** `TestLanguageCorpusRunsInBothLanes` for the Json, Octagon,
  Fallible and Artifacts trees and for every `.octfail` under `Language`.
- **Directories M5 touches, both lanes, against the M1 baseline:** 43
  directories: the 20 that write JSON, the libraries touched and their
  neighbours, and the directories M4 ran. **No test went from passing to
  failing.** The differences are the tests removed with the first library
  (14 in `Libraries/IO`, 3 in `Libraries/Json`, replaced by 3), one renamed,
  and tests added since M1.
- **The compiled lane without sidecars:** `Libraries/IO` fails 16 tests, all
  CSV and file tests that need `octxiliary-csv` or `octxiliary-io`; it failed
  29 before, the other 13 being JSON tests that are gone. One JSON test that
  "passed" compiled was asserting an error and got the missing-sidecar error.
- **The slow wrapper lane**, because sidecar code changed: the 12 remaining
  sidecars build, and `TestIOCoreWrappers`,
  `TestCsvReadMatrixCsvReadRowsCsvReadTableAutoCompiledWithoutFallback` and
  `TestIOXlsxWrapper` pass with them, all of `Libraries/IO` compiled.
- **Artifacts:** as in "Recorded outputs".

Fault injection:

| Code | Changes | Caught | Left |
|---|---|---|---|
| `octjson/read.go` (mechanical) | 6 | 6 | |
| `builtin/json.go` (mechanical) | 13 | 11 | 2 did not build |
| The read-back, the namespace, the redeclaration rule, the rules that replaced the first library's (15 chosen by hand, both lanes) | 15 | 13 | 2 not observable |

The two left remove the guard each lane has for a Json call that reaches it
with no type. The typechecker refuses or fills in every such call, so no
program gets there.

One change survived the first pass: a failed read-back naming the staging
directory. The contract for the published path was added for it.

## Exit

The ladder's exit asked that `grep` find no name from 3.10 outside the ladder
and the milestone reports. In Go and Oct sources it finds one file: the
contract that `Json.Object` does not exist. It also finds, and these stay:

- the dated records under `docs/internal`, `docs/reports` and
  `internal/libraries`, and the reports of `JsonIntentRecoveryLab`;
- the CHANGELOG and `FEEDBACK.md`;
- the notes this milestone added to the experiments' reports and to the lab's
  closing README, which say what was removed.

The live documents are updated: the reference, `Libraries/IO/README.md`,
`Libraries/Artifact/README.md`, `Libraries/Json/README.md` (new),
`docs/COMPILED_SUPPORT.md`, `docs/internal/octxiliary.md`.

`Libraries/Artifact/README.md` was not in the ladder's list and was wrong in
three ways: it said `Json` exposes only `Load` and `Save`, pointed to
`Json.Stringify`, and suggested `Json.Save` inside an `[Artifact]` function,
which the phase rejects.

## Found outside Json

Not changed; each is in `FEEDBACK.md`.

- The CSV readers cannot read an output back during artifact evaluation,
  while the text readers and `Json.Load<T>` can.
- An `.octfail` expectation cannot contain a double quote, and the reference
  does not say how the substring is read.
- `FmBrownNoiseKalman/M2` cannot be evaluated, and two of its recorded files
  are stale.
- A recorded artifact can be pinned by a hash elsewhere with nothing
  connecting the two.

## Next

M6: `oct json infer`, which writes the declarations for a document. The
seven corpus documents and the seven under `Libraries/IO/testdata` are kept
for its goldens. The whole-tree sweeps in both lanes run once when the ladder
closes, after M6.
