# Changelog

## v0.1.0 — initial preview

Oct 0.1 is an early preview of a scientific programming language and toolchain for reproducible research, portable computation, and AI-assisted experimentation.

Highlights:

- scientific language core with source contracts under `Language/`;
- interpreted and compiled execution paths;
- SI units and scientific numeric/library surfaces;
- xUnit-style tests, artifacts, and benchmarks;
- Octomata flow/state machines;
- tensors and Einstein notation support in the language surface;
- package manager MVP with local/Git source sync and transitive exact dependency graph sync;
- source-controlled canonical first-party registry at `Registry/registry.oct`;
- canonical `Mathematics@0.1.0` package name, with no `Math` alias;
- optional project-root `lock.octagon` for locked sync;
- Octxiliary wrapper sidecars declared by manifests;
- explicit native sidecar builds through `oct pkg build-wrappers --allow-native`;
- Go-based native binary path for compiled programs.

Pre-1.0 notes:

- language syntax and semantics may change before 1.0;
- public Go APIs, including sidecar helper APIs, may evolve;
- package registry format and lockfile contents may evolve;
- the standard library/package APIs are not stable;
- hosted registry, publishing, auth, signing, `.octpkg` artifacts, semver ranges, `latest`, and package solver behavior are not part of v0.1;
- performance is not final and should not be treated as a release guarantee.
