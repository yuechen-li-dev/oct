# Mx103d Report — Image.Core MVP Wrapper Wave

## 1) Why `Image.Core` before `Plot.Core`

`Image.Core` establishes images as first-class artifacts independent of plotting concerns.
By stabilizing load/save/metadata first, later wrapper waves (`Plot.Core`, `Pdf.Core`, and image-processing helpers) can compose around a shared image handle substrate instead of each introducing separate file/image conventions.

## 2) Backend and dependency choices

This MVP uses Go's standard `image` stack:

- decode: `image.Decode` with registered codecs from `image/png` and `image/jpeg`
- encode: `png.Encode` and `jpeg.Encode`

No new third-party image dependency was introduced.
Existing plotting remains backed by `gonum/plot` and still writes PNG directly (`internal/interpret/plot.go`); `Image.Core` does not re-home plotting in this wave.

## 3) API surface implemented

`Image.Core` exposes:

- `Load(path: String) -> ImageHandle ! Error`
- `Save(image: ImageHandle, path: String) -> Int ! Error`
- `Width(image: ImageHandle) -> Int<px>`
- `Height(image: ImageHandle) -> Int<px>`
- `Format(image: ImageHandle) -> String`

`ImageHandle` is a thin wrapper record over an internal handle (`Handle: Int`), matching established wrapper-handle style.
The handle remains intentionally opaque/non-unit-typed; only dimensions are pixel-typed (`px`).

## 4) Wrapper substrate reuse and additions

Reused Mx103a substrate pieces:

- invocation/arity: `newWrapperCall(...)`, `expectArity(...)`
- argument decode: `stringArg(...)`, `intArg(...)`
- result lifting: `wrapperIntResult(...)`, `wrapperStringResult(...)`, `wrapperIntDimensionResult(...)`
- error envelope: `wrapperErrorf(...)`, `wrapperErrorResult(...)`
- registry composition: `newWrapperBuiltinRegistry(...)`
- handle storage: `wrapperHandleStore[...]`

New helper/lifting support added:

- `wrapperIntDimensionResult(...)` for returning unit-typed integer wrapper values (used for `Int<px>` image dimensions).

No ad hoc wrapper decoding, error envelope, or one-off registry pathway was introduced.

## 5) Deliberately excluded

Out of scope for Mx103d:

- pixel mutation APIs
- resize/crop/rotate/filter/composition
- plotting re-home (`Plot.Core`)
- PDF generation APIs
- broad format catalogs beyond practical PNG/JPEG MVP

## 6) Tests proving the MVP

`Libraries/Image/Image.Core.octest` includes:

- happy-path roundtrip: load fixture PNG, inspect metadata, save JPEG, reload and verify metadata
- deterministic fixture metadata checks for JPEG fixture
- missing file failure
- corrupt/invalid image failure
- unsupported output extension failure

`cmd/oct/m103d_image_core_wrappers_test.go` synthesizes deterministic PNG/JPEG/corrupt fixtures at runtime before invoking `oct test`, then verifies pass markers and cleans up generated artifacts.

## 7) Explicit inconsistency/doc-gap notes

- Language reference still groups plotting as builtin-oriented behavior while this wave introduces an `Image.Core` standard-library-first foundation. This report and updated `17-standard-libraries.md` make the new ownership explicit.
- User-request signatures used `Unit`; Oct reference types use `Void`. This wave keeps established wrapper return posture (`Int ! Error`) for side-effect wrappers, consistent with existing Mx103 wrapper families.

## 8) What this enables next

With a stable image wrapper foundation now in place, next waves can build:

- `Plot.Core` output APIs that target `ImageHandle` or save through `Image.Core`
- `Pdf.Core` image embedding/export helpers
- constrained image-processing additions (transformations/inspection) without redesigning artifact boundaries
