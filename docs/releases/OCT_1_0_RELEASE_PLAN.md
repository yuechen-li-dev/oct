# Oct 1.0 Release Plan (RC1 -> RC2 -> GA)

## Final GA procedure

Before any irreversible action, start from the approved clean revision and repeat the artifact commands
in `INSTALL_1_0.md` and the layered conformance gate below. Confirm the final
files are exactly `oct-1.0.0-windows-amd64.zip`,
`oct-1.0.0-linux-amd64.tar.gz`, and `checksums.sha256`, and independently verify
the manifest on Windows and Linux.

After a human approves those exact outputs: create an annotated `v1.0.0` tag at
the approved revision, push that tag, create the GitHub release, upload only
the two verified archives plus `checksums.sha256`, and publish the installation
instructions. Stop rather than publish if any archive hash, version output,
conformance count, fallback count, required archive entry, or host smoke test
differs from the recorded GA evidence.

## RC2 bounded work

1. **COMP-002** and **COMP-003** are closed: the stable-library compiled sweep
   covers all 28 declared libraries with zero fallback. Keep this sweep in the
   candidate evidence; do not relabel established libraries experimental to
   bypass future parity failures.
2. Resolve **API-001**: publish a stable-library manifest that marks each
   public module/API stable, experimental, wrapper-required, or excluded.
3. **PKG-001 is closed:** `1.0.0-rc.1` Windows and Linux archives are built,
   checksummed, extracted, and smoke-tested outside the checkout. The final GA
   operation changes only the injected version to `1.0.0`, repeats this gate,
   then obtains human approval before tagging and publication.
4. Turn the conformance gate below into a recorded RC2 result and repair only
   defects whose intended behavior is already clear.

## RC2/GA release gates

Run the following layered gate; each layer covers a distinct claim:

```powershell
go test -count=1 -parallel 8 ./...
go test -count=1 -parallel 8 -tags=integration ./...
go test -count=1 -parallel 8 -tags=toolchain ./...
# Invalid-contract baseline (the Language container has no valid-package root):
go run ./cmd/oct test Language --execution interpreted
# Representative core interpreted/compiled parity fixture:
go run ./cmd/oct test Examples/SmartGreenhouseController --execution interpreted
go run ./cmd/oct test Examples/SmartGreenhouseController --execution compiled
go run ./tools/build_sidecars --out dist/sidecars
$env:OCT_SLOW_TESTS = "1"
$env:OCT_WRAPPER_PATH = "$PWD\dist\sidecars"
go test -count=1 -parallel 8 -tags=toolchain ./cmd/oct -run 'Wrapper|Octxiliary|IO|Csv|Json|Xlsx|Pdf|Image|Plot|Compiled'
go build -o dist/oct.exe ./cmd/oct
.\dist\oct.exe version
git diff --check
```

Before GA, RC2 must close GATE-001 by supplying an explicitly documented,
owner-approved valid-corpus target set (or a small target manifest/driver) for
both interpreter and compiled claims. The `Language` container invocation alone
is an invalid-contract baseline, not an all-language test. Native hardware,
Prometheus integration, GPU/model workloads, and WebView lanes are separate
evidence, not generic 1.0 gates.

## GA artifact expectations

Produce and verify versioned Windows x86-64 and Linux x86-64 `oct` binaries,
the selected sidecars for the promised wrapper surface, checksums, `oct version`
output, help/run/build/test/fmt smoke results, and Go-module installation
instructions. Confirm that no generated build/test artifacts, `dist/`, or local
caches are committed. A human explicitly approves the final version/tag only
after all gates are green.

## Non-goals and stop conditions

No metaprogramming, speculative syntax, compiler rewrite, repository cleanup,
Prometheus completion, OctMake expansion, Machina UI implementation, standard
library expansion, 1.0 version change, tag, publication, or broad CI redesign
belongs to this plan. Stop RC2 and return to an owner decision if compiled
coverage requires redefining accepted language semantics, a broad backend
rewrite, or a compatibility promise that cannot be objectively tested.

## Bounded RC2 task prompt

> **OCT-1.0 RC2 — Blocker Closure, Packaging, and Release Candidate.** Read
> `docs/releases/OCT_1_0_{CONTRACT,READINESS,RELEASE_PLAN}.md`, preserve the
> dirty worktree, and close only COMP-001, API-001, PKG-001, and GATE-001.
> Establish an owner-approved compiled support boundary or close its bounded
> gaps; publish the stable/experimental library list; create an honest valid
> corpus conformance target; and prove release-shaped Windows/Linux build,
> version, install, and sidecar expectations without tagging or publishing.
> Do not add language features, broaden libraries, complete experimental
> systems, rewrite the compiler, or alter 1.0 semantics without an explicit
> owner decision. Run the recorded layered gates and report each result,
> remaining blocker, and artifact provenance.
