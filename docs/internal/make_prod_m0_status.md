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

Prometheus now declares SerialCanonical reactor, SDSL-V host, normal Marionette,
benchmark, and isolated M5b build-only targets. The M5b target carries
`PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT` and has an independent artifact
identity. No GPU target is executed by this plan.

Current boundary: this checkout cannot load `internal/prometheus/Make.oct` as an
ordinary project because that directory has no Oct package manifest and package
resolution searches `internal/Libraries/Make`, not the repository `Libraries`.
The existing plan had the same unresolved bootstrap layout. As a result a real
Windows build and smoke have not been claimed. The next bounded task is to fix
that package-root/bootstrap seam, then validate MSVC environment discovery,
Windows output lowering, smoke execution, and failure evidence before scripts
can be retired.
