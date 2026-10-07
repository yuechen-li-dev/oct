# Json v2 — closing report

Date: 2026-10-07
Ladder: `internal/json/JSON_V2_LADDER.md`, closed
Branch: `claude/json-v2`, on `d44566d` (`main`, which has not moved). Not
merged. No pull request is open and nothing is tagged.

This is what is left after the ladder: what needs a decision from you, what
is still wrong, what I did not check, and what I would do next. Each fault
has an entry in `FEEDBACK.md` with how to see it.

## Where it stands

Oct has one `Json`. A program declares a type and the document is read as
that type or refused with the place:

```oct
Json.Load<T>(path)   Json.Parse<T>(text)   Json.Save(path, value)   Json.Text(value)
Artifact.WriteJson(path, value)            oct json infer <file.json>
```

- Both lanes read equal values, write equal bytes and give equal error
  text. There is no sidecar: a generated program imports
  `internal/octjson`, the code the interpreter runs.
- The first library is gone: seven builtins, the `IO` JSON functions and the
  raw graph, `Json.Object`, `cmd/octxiliary-json`.
- `Option<T>` exists (M1), and `F()?` is a statement (M4).
- Contracts: 59 facts and 62 `.octfail` under `Language/Builtins/Json`, and
  26 documents loaded with their inferred declarations under
  `Language/Tooling/JsonInfer`, all in both lanes. `octjson`, `jsontype` and
  `jsoninfer` are at 100% of statements.
- The whole tree was run in both lanes when the ladder closed, each test
  against its result when M1 closed. **No test went from passing to
  failing.** The four Go lanes fail only where they failed then.

**Is it production ready?** For documents the size of a configuration, a
summary or a message: yes. Four things stand between that and "yes" without
a qualifier, in the order I would weigh them:

1. A key such as `$schema` cannot be read into a record (decision 3).
2. Loading takes 47 to 95 times the file in memory (fault 1).
3. Two name clashes do not build in the compiled lane (faults 2 and 3).
4. A tagged array has no declaration (decision 4).

## Decisions that are yours

**1. Merge.** 44 commits, fast-forward onto `main`. I can open the pull
request; I have not, because you did not ask for one.

**2. The witness the ledger pins.**
`Evt2OctOracle/o0_structural_witness.json` is recorded in
`experiment_ledger.json` by its SHA-256. The new writer prints the same JSON
indented, so other bytes. I left the recorded file and the ledger alone.
`oct artifact Experiments/ZImageTurboNoiseRefiner0/M0` now rewrites the file
with another hash. Regenerate and re-pin, or keep the recorded bytes as the
evidence of that campaign: either is defensible, and the ledger is yours.

**3. Keys no field can match.** Json matches a key to a field with `_`, `-`,
`.`, spaces and case ignored, and a field name is letters and digits.
`$schema`, `@type`, `$ref` and `3d` therefore match nothing, and since an
unknown member is an error, an object with such a key cannot be loaded as a
record whatever is declared. JSON Schema, JSON-LD and OpenAPI all have them.
Two ways out:

- Let the match ignore every character that cannot be in an identifier, so
  `$schema` is `Schema`. No syntax, a change to `octjson.FoldName`, and
  "the declared type is the intent" stands. Writing gives `Schema`, not
  `$schema`, so a round trip changes the key.
- A field attribute that states the key, `[Key("$schema")]`. Covers writing.
  It is new surface.

I would take the first now and the second only when something has to write
such a key back.

**4. Tagged arrays (D8).** Corpus documents 06 and 07 are still refused.
What reads them is an enum whose variants carry records, chosen by a member
such as `type`. Payload enums exist in the language; what is missing is the
rule that says which member is the tag and how a variant's payload is found.
`oct json infer` already identifies the tag and its values, which is half of
what such a declaration needs. This is a design of its own, not a patch.

## What is still wrong

### A program that uses Json can meet these

| | Fault | Where |
|---|---|---|
| 1 | **Memory.** A 23 MB document of 200,000 rows takes 1.1 GB compiled and 2.2 GB interpreted to load and save (3 s and 16 s). The text becomes a tree, then Octagon data, then the materialiser's input, then the value. | Both lanes |
| 2 | Two refined concepts of one name in two packages (`Main.Port`, `Net.Port`) do not build once the program loads Octagon or JSON: "duplicate case". | Compiled |
| 3 | Two enums of one name in two packages do not build: the tag constants carry no package. | Compiled |
| 4 | `==` on two values of an enum whose payload holds an array panics. | Compiled |
| 5 | A file cannot read the fields of a value whose type is declared in a package it does not import, though `Json.Load` gave it the value. The message does not say so. | Typechecker |
| 6 | A `Void ! Error` function cannot `return error("...")`; it can only fail by propagating. | Typechecker |
| 7 | `oct run` prints `<invalid>` after a `Void ! Error` `Main`. | CLI |
| 8 | Indexing a value of a refined array concept gives the concept's type, and does not build compiled. | Both |

2 and 3 are one cause: a name in generated Go that leaves the package out.

### Octagon, which shares the materialiser

- `WriteOctagon` then `LoadOctagon` does not give a `Float` back: a whole
  value is written without a decimal point.
- `LoadOctagon<Vector<T>>` as the type argument is refused, while a record
  with a `Vector<T>` field loads.
- `WriteOctagon` of a vector is refused interpreted and written compiled.

### The lanes disagree, outside Json

- `0.1 + 0.2 != 0.3` is true interpreted and false compiled: Go folds the
  literals exactly.
- `Sqrt` and `Ln` outside their domain stop the interpreter and give a NaN or
  an infinity compiled.
- `v + v` on two vectors does not build compiled unless the program uses
  another linear-algebra operation.
- `BoardSnapshot(machine)` does not build compiled when two flows of one
  package have one result type.

### The language

- A `match` case label names its variant and the enum name before it is not
  checked: `case Anything.Some(v)` is accepted.
- `record Range { ... }` is accepted and the name still means the builtin.

### Tooling and experiments

- The CSV readers cannot read an output back during artifact evaluation,
  while the text readers and `Json.Load<T>` can.
  `Experiments/FmBrownNoiseKalman/M2` cannot be evaluated because of it, and
  has two duplicate output paths besides; its recorded `metrics.csv` and
  `m2a_report.md` are stale.
- `PrometheusShadowAuthorityRakeLab/M3/FINDINGS.md` as recorded is not what
  its artifact function writes.
- An `.octfail` expectation cannot contain a double quote.
- `oct test` of a directory with no tests says "unknown package 'Main'".
- A recorded artifact can be pinned by a hash elsewhere and nothing connects
  the two (decision 2 is the instance).

## Limits that are the design

Not faults; stated so they are not rediscovered as faults.

- There is no untyped JSON value (D7), and an unknown member is an error
  (D4). A document whose shape is not known is declared with
  `oct json infer` first.
- A written key is the field name exactly. A schema with `snake_case` keys
  is a record with `snake_case` fields; there is no renaming.
- A millisecond count is a plain number (D13). Prefixed units are a language
  decision that was not taken here.
- `oct json infer` reads one document. It cannot tell an object of names
  from a record, makes nothing singular, and proposes no enum. A
  `--table <path>` option would let a person say "this object is a table"
  without editing the output; I did not add it because the ladder did not
  ask for it.

## What I did not check

- **Windows and macOS.** Everything ran on Linux. Positions with CRLF are
  tested in `octjson`; the wording of a file error is this package's own, but
  which OS error maps to which wording was only exercised here.
- **Fuzzing.** The parser is held to JSONTestSuite, 3000 generated types
  round-trip, and 4000 generated documents load with what is inferred. No
  fuzzer was run.
- **Documents larger than 23 MB**, and deep nesting beyond the 512 limit's
  own test.
- **`oct-mcp`.** It does not expose `oct json infer`. An agent would use it.
- **The editor grammar**, for `Json` and for `Option`.

## The tree outside this phase

For whoever works on it next, the UI work in particular.

- **Compiled, with sidecars: 213 of 2906 tests fail, as at M1.** About 190
  are UI builtins the compiled lane does not have: `Button` and `AbsoluteBox`
  account for most, across `Libraries/UI`, `SignalLab`, `Storefront` and
  `ControlPanel`. Six more are `Pdf.DrawImage`.
- **Interpreted: 15 of 2906 fail, as at M1.** They want a sidecar or a
  fixture that a run from the repository root does not find
  (`MakeHostPrivileged`, the test wrapper, an image by a relative path).
  One, in `PrometheusSgemmAlgorithmLab/M19`, exceeds a cycle-time bound.
  `Libraries/SymbolicRegression`'s end-to-end test is the same kind: it
  failed at M1 and passed now, under different load.
- **Go:** `internal/sdslv/test` and one LaTeX test need tools this machine
  lacks, in every lane.
- **A full test run dirties the tree:** it rewrites the tracked
  `cmd/oct/analysis_output.png` and leaves an `.xlsx` and two PDFs at the
  root. `git add -A` after one commits them.
- **For UI code that reads or writes JSON:** declare a record and call
  `Json.Load<T>` or `Json.Save`. `IO.Load`, `IO.Save`, `Json.Object` and
  JSON built with `String.Concat` are gone and will not compile. For a
  payload whose shape is not written down, `oct json infer` prints the
  declarations.

## A correction I owe you

M5's report said it was verified and one Go test was failing: the
integration-lane test that ran the lab test M5 removed. The bounded
verification the ladder asks of a milestone does not run that lane, and I
searched for removed names, not for the path of the removed test. The
whole-tree run found it and it is fixed. It is the second time in this
ladder that a milestone was reported green and was not (the first was the
fixtures of M2 that `.gitignore` kept out of the commit). Both were found by
running from a clean checkout or by running everything, and neither by the
bounded tests. For a milestone that removes things, I would now run the
integration lane of `cmd/oct` as well; it takes two minutes.

## What I would do next

1. **Merge**, after decisions 1 and 2.
2. **`$schema`** (decision 3, first option): small, and it removes the one
   case where a real document cannot be loaded at all.
3. **Faults 2 and 3**, together: qualify the generated names with the
   package. One cause, both lanes' contracts already exist to extend.
4. **CSV read-back in the artifact phase**, then clean and regenerate
   `FmBrownNoiseKalman/M2`. One seam (`prepareArtifactRead`), as for Json.
5. **Memory**, if large files are a use: decode into the value directly
   instead of through three intermediate trees. That is a change to how
   `octjson` meets the materialiser and needs its own ladder step.
6. **Tagged arrays** (decision 4), as a design.
7. **The Octagon `Float` round trip**: a written value that does not read
   back is a fault in the native format, and the fix is in the two writers.
8. The rest, as they are met.
