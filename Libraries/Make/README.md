# Make

`Make` is a first-party Octxiliary wrapper package for future `oct make` host-capability primitives. It is intentionally side-effectful and is not part of Oct's ambient scientific runtime.

The `octxiliary-makehost` sidecar requires explicit make authority (`OCT_MAKE_AUTHORITY=1` in MAKE1 tests and future `oct make` plumbing). Ordinary `oct run`/test execution that reaches these functions without authority receives a clear capability error instead of process or file access.

Process execution uses `program` plus `args`; it does not invoke a shell string. Launch failures are fallible errors. Non-zero process exits return `ProcessResult` with the non-zero `ExitCode`, captured `Stdout`, and captured `Stderr`.

File primitives are intended for project build orchestration. MAKE1 does not yet enforce project-root path policy inside the sidecar; future `oct make` will provide authority and project-root scoping. Ninja backends, C/C++ helpers, Go helpers, target DAGs, and `Make.oct` plan execution are deferred.
