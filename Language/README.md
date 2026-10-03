# Language contracts

The [Oct reference](reference/README.md) defines syntax, style, and supported features. The remaining directories contain executable valid and invalid language specimens.

[SDSL-V](SDSL-V/README.md) has its own compiler and conformance fixture set. Keep these fixtures under `Language/` so their role is distinct from [reader-facing examples](../Examples/README.md).

## Running the corpus

`oct test <directory>` runs one fixture directory. Run it from the repository root: fixtures name their data files relative to it.

The whole corpus is run by one Go test, in both execution lanes:

```text
go test -tags=integration ./cmd/oct -run TestLanguageCorpusRunsInBothLanes
```

It runs every directory that holds `.octest` files in the interpreted lane and in the compiled lane, evaluates directories of `[Artifact]` entry points with `oct artifact`, and checks every `.octfail`. `cmd/oct/language_corpus_test.go` lists what it treats differently, with the reason: package sets that are run from their root, the one directory another test runs with native grants, the directories that need a wrapper sidecar, and compiled-lane gaps (none at present). The lists are checked too. An entry whose directory is gone, or whose failure no longer happens, fails the test.

A new fixture directory needs no registration. If it does not pass in both lanes, the test fails until it does.

### Fixtures that belong to one lane

A fact whose subject is one execution lane says so in the source, with the reason: `[Interpreted("...")]` or `[Compiled("...")]`. The other lane reports it as skipped. Use this only when the two lanes are specified to behave differently. A feature that one lane is missing is not a reason; that fixture should fail until the feature exists. `Language/reference/tooling/31-octest.md` has the rules.

### Fixtures that must fail

An expected failure is an `.octfail`, not an `.octest` that a Go test expects to fail. There are three forms: `expect error:` for a source that must be rejected, `expect runtime error:` for a `Main` that must stop, and `expect artifact error:` for `[Artifact]` entry points that must fail. A failure that needs a second file or a manifest puts those in a `Packages/<Name>/` directory beside the fixture and imports it.

### Sidecars

Three directories call a wrapper sidecar. The test builds the sidecars it needs once, into a cache keyed by their sources (`internal/sidecarcache`), and reuses them until those sources, `go.mod`, `go.sum` or the Go toolchain change. The cache is in the user cache directory; `OCT_SIDECAR_CACHE_DIR` names another place. Every other directory runs with no sidecar available.

## Imports

A fixture may import a library from `Libraries/`. `Language/Packages` is a fixture domain, not a package root for the rest of `Language/`; a package placed there would be found before the library of the same name.
