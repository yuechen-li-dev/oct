# MAKE-PROD-M0 native vertical status

This change introduces the first typed native direct-backend seam. `Make.NativePlan`
is an optional companion to `Plan()`, preserving existing `Make.Plan` literals.
`NativeTarget` uses closed language, kind, and profile enums, plus typed source,
input, dependency, include, define, link-library, output, copy, and variant data.

The backend lowers each source to its own command/state/trace action and derives
collision-resistant object names from target, variant, and normalized source
identity. Link actions consume those objects; a shared object set is reused by
the reactor and Marionette executables. The executor remains sequential; bounded
parallel scheduling is deferred to MAKE-PROD-M1.

During M0 `internal/prometheus/native/native_manifest.json` remains the canonical
inventory. A typed target may declare its manifest and section; the backend
validates source extensions and duplicates before lowering. The legacy generated
Windows/Linux fragments and scripts remain parity controls and are not invoked by
the native backend.

## Milestone declaration

MAKE-PROD-M0 is the direct Windows native-build vertical slice. Its successor,
MAKE-PROD-M1, provides Windows transitive-header correctness; the verified
Windows evidence is recorded in `docs/internal/make_prod_m1_windows_validation.md`.
GCC/Clang discovery remains explicitly deferred backend work.

Prometheus now declares SerialCanonical reactor, SDSL-V host, normal Marionette,
benchmark, and isolated M5b build-only targets. The M5b target carries
`PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT` and has an independent artifact
identity. No GPU target is executed by this plan.

The nested explicit Make-file import seam is fixed through canonical repository
root discovery: a lowercase implementation directory such as `internal/libraries`
can no longer shadow the repository `Libraries` import root on Windows. Pure
plan inspection now loads without a sidecar. The Windows direct route built the
SerialCanonical reactor, SDSL-V host, normal Marionette executable, and benchmark,
then passed `PrometheusNativeHarness_Smoke`; the isolated M5b variant built without
execution. M0 does not claim transitive-header-correct incremental compilation:
only declared inputs participate in staleness. MAKE-PROD-M1 owns discovered-input
state and compiler dependency collection.
