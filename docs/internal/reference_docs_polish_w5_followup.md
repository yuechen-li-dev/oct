# Reference docs polish after W5 scaffolding and wrapper/tensor work

## Summary

This pass updates the user-facing `Language/reference` docs after W5 package scaffolding and recent wrapper/Octxiliary and tensor work.

Docs touched:

- `Language/reference/tooling/35-cli.md`
- `Language/reference/tooling/31-octest.md`
- `Language/reference/tooling/33-oct-pkg.md`
- `Language/reference/language/11-records.md`
- `Language/reference/runtime/21-octomata.md`
- `Language/reference/language/16-vectors-and-matrices.md`
- `Language/reference/00-overview.md`

## Fixed stale statements

- Removed the stale claim that `oct test` has no compiled test path.
- Documented `oct test --execution interpreted`, `--execution compiled`, and default `auto` behavior.
- Kept caveats explicit: compiled execution is valid but not every package/feature is guaranteed to compile, and wrapper sidecar availability can affect compiled wrapper tests.

## Records and `with`

- Made `Language/reference/language/11-records.md` the owner of immutable record `with` updates.
- Kept Octomata docs focused on behavior/data separation and cross-linked to the records page instead of teaching the full `with` concept there.

## Testing reference structure

- Restructured `31-octest.md` into scannable sections for overview, `[Fact]`, `[Theory]`, suites, assertions, fallible tests, skips/timeouts, execution modes, file/layout conventions, and lane policy.

## Package tooling updates

- Updated `oct pkg wrappers` docs for planning-only behavior and deterministic inert registry output.
- Documented current wrapper inspection behavior: current-package metadata is inspectable, only direct dependencies with `Source` are fetched, and non-fetchable dependencies such as `OctStd` are ignored by wrapper planning.
- Added `oct new library`, `oct new experiment`, and `oct new wrapper-library` user-facing limitations: no flags, strict PascalCase names, target `./<Name>`, fails if target exists, wrapper sidecar scaffold is not built or run.

## Known remaining docs gaps

- Third-party wrapper manifest hardening and native sidecar build lifecycle are still future work and intentionally remain caveated.
- Compiled parity remains package/test-coverage driven; docs should continue avoiding blanket claims that every test package compiles.
