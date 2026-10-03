# Language contracts

The [Oct reference](reference/README.md) defines syntax, style, and supported features. The remaining directories contain executable valid and invalid language specimens.

[SDSL-V](SDSL-V/README.md) has its own compiler and conformance fixture set. Keep these fixtures under `Language/` so their role is distinct from [reader-facing examples](../Examples/README.md).

## Running the corpus

`oct test <directory>` runs one fixture directory. Run it from the repository root: fixtures name their data files relative to it.

The whole corpus is run by one Go test, in both execution lanes:

```text
go test -tags=integration ./cmd/oct -run TestLanguageCorpusRunsInBothLanes
```

It runs every directory that holds `.octest` files in the interpreted lane and in the compiled lane, evaluates directories of `[Artifact]` entry points with `oct artifact`, and checks every `.octfail`. A directory that cannot be run that way is listed in `cmd/oct/language_corpus_test.go` with the reason: package sets that are run from their root, expected failures and wrapper fixtures that another test owns, and compiled-lane gaps. The list is checked too. An entry whose directory is gone, or whose failure no longer happens, fails the test.

A new fixture directory needs no registration. If it does not pass in both lanes, the test fails until it does or until it is listed with a reason.

## Imports

A fixture may import a library from `Libraries/`. `Language/Packages` is a fixture domain, not a package root for the rest of `Language/`; a package placed there would be found before the library of the same name.
