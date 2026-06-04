# Octxiliary M29 Pdf/Image Interop via Oct-Mediated Bytes Transfer

## 1. Executive summary

M29 is a design/audit milestone only. It does not change production compiler, package-manager, protocol, sidecar, library, or interpreted behavior.

**Recommended interop strategy:** M30 should implement **Image export to bytes plus Pdf direct bytes drawing**:

- `Image.EncodePng(image: ImageHandle) -> Bytes ! Error`
- `Pdf.DrawImageBytes(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>) -> Int ! Error`
- `Pdf.DrawImageBytesSized(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`

The intended compiled flow is:

```oct
let img = Image.Load("plot.png")?
let png = Image.EncodePng(img)?
Pdf.DrawImageBytes(page, png, "png", 10px, 20px)?
```

**Existing transports are enough.** The current generic Octxiliary path already has `Bytes`, `String`, `Int`, dimensioned `Int<px>` lowered as integer payloads, wrapper handle arguments and returns, record arguments, and fallible `Int ! Error` / `Bytes ! Error` returns. M30 should require wrapper metadata and sidecar/library additions, not a protocol revision or new transport type.

**Recommended next implementation milestone:** M30 should add `Image.EncodePng` and Pdf direct bytes drawing, mark those additive APIs compiled-supported, and add focused sidecar/compiled/interpreted tests for the new APIs while keeping legacy `Pdf.DrawImage` and `Pdf.DrawImageSized` interpreted-compatible and compiled-deferred.

**Deferred:** Pdf-owned imported image handles, path/file drawing APIs, JPEG encode/draw support beyond what is cheap and explicit, artifact/file-backed large-payload transfer, image resource lifecycle/destructors, cross-family handle sharing, global handle brokers, sidecar-to-sidecar communication, package-manager sidecar build work, permission prompts, and lockfiles.

## 2. Current interpreted behavior

The current interpreted Pdf image path is an in-process compatibility bridge between the Image and Pdf wrappers:

1. `Libraries/Image/Image.Core.oct` exposes `Image.Load(path)` by calling the raw builtin `ImageLoad(path)?`, then wrapping the returned integer as `Image.ImageHandle { Handle: handle }`.
2. `Libraries/Pdf/Pdf.Core.oct` defines a separate public `Pdf.ImageHandle { Handle: Int }` record and sends `image.Handle` to `PdfDrawImage` / `PdfDrawImageSized`.
3. `Libraries/Pdf/Pdf.Core.octest` does not call `Image.Load`; it calls raw `ImageLoad(pngPath)!` from the Pdf package test, then constructs `ImageHandle { Handle: imageHandle }` where `ImageHandle` resolves to `Pdf.ImageHandle`.
4. The interpreter registers both wrapper families in one interpreter process. `internal/interpret/interpret.go` creates a shared `i.images` handle store, `internal/interpret/wrapper_image.go` allocates decoded images into that store, and `internal/interpret/wrapper_pdf.go` looks up Pdf image arguments in the same `i.images` store.

That works interpreted because the raw integer returned by `ImageLoad` is an index into one process-local interpreter table. The Pdf builtin can dereference it because it is not crossing a process, ABI, package family, or sidecar boundary.

That same integer identity is invalid in compiled Octxiliary sidecar mode:

- `Image.ImageHandle` belongs to the `Image` wrapper family and is owned by the `octxiliary-image` process.
- `Pdf.PdfPage` belongs to the `Pdf` wrapper family and is owned by the `octxiliary-pdf` process.
- A future `Pdf.ImageHandle`, if added, must also be owned by `octxiliary-pdf`, not by `octxiliary-image`.
- Sidecar handles are sidecar-process-local capabilities. Handle ID `1` in the Image sidecar and handle ID `1` in the Pdf sidecar are unrelated.
- The compiled generic wrapper transport carries handle family/type/ID to preserve ownership. Treating a public `Handle: Int` field as a cross-family capability would bypass that protection.

The current interpreted bridge should therefore be documented as legacy interpreted compatibility, not as a compiled contract.

## 3. Handle ownership model recap

The M0/M1 Octxiliary ownership rule is:

> Handles never cross sidecar families. Data crosses sidecar families explicitly through the compiled Oct binary.

For Pdf/Image interop this means:

- `Image.ImageHandle` is owned by `octxiliary-image`.
- `Pdf.PdfPage` is owned by `octxiliary-pdf`.
- A future `Pdf.ImageHandle`, if introduced, must be owned by `octxiliary-pdf` and must refer only to resources registered inside the Pdf sidecar.
- Public records with `Handle: Int` are compatibility shells at the Oct boundary. They are not raw, portable, serializable, or cross-family capabilities.
- The compiled Oct binary is the orchestrator: it may call `octxiliary-image`, receive serialized `Bytes`, then call `octxiliary-pdf` with those bytes. The two sidecars must not call each other or share live resource tables.

## 4. Candidate interop strategies

### A. Direct bytes drawing

Potential APIs:

```oct
Pdf.DrawImageBytes(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>) -> Int ! Error
Pdf.DrawImageBytesSized(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error
```

Use:

```oct
let img = Image.Load("plot.png")?
let png = Image.EncodePng(img)?
Pdf.DrawImageBytes(page, png, "png", 10px, 20px)?
```

Pros:

- No new handle lifecycle, destructor, or Pdf-side image table is required.
- Uses existing `Bytes` transport and generic wrapper lowering.
- Does not permit `octxiliary-pdf` to consume `Image.ImageHandle`.
- Does not require `octxiliary-image` to know Pdf exists.
- Easy to test in sidecar-only and compiled package paths.
- The API names make the serialized transfer explicit and inspectable.

Cons:

- Repeated draws of the same image may re-decode and re-register bytes in the Pdf backend.
- Larger images pass through compiled Oct process memory and the current textual bytes frame encoding.
- Public additive Pdf APIs are required.
- The format string requires validation and clear documentation.

Assessment: this is the best first compiled interop path because it proves correct ownership with the smallest runtime and API surface.

### B. Pdf-owned image import handles

Potential APIs:

```oct
Pdf.ImportImageBytes(bytes: Bytes, format: String) -> Pdf.ImageHandle ! Error
Pdf.DrawImage(page: PdfPage, image: Pdf.ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error
Pdf.DrawImageSized(page: PdfPage, image: Pdf.ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error
```

Use:

```oct
let img = Image.Load("plot.png")?
let png = Image.EncodePng(img)?
let pdfImg = Pdf.ImportImageBytes(png, "png")?
Pdf.DrawImage(page, pdfImg, 10px, 20px)?
```

Pros:

- Pdf owns its image resources, preserving the capability model.
- Repeated draws can reuse the Pdf-side registered image.
- Preserves a conceptual `Pdf.ImageHandle` for users who think in terms of document resources.
- Can later align `DrawImage` / `DrawImageSized` compiled support with a Pdf-owned handle type.

Cons:

- Requires a Pdf image table and additional validation paths.
- Introduces lifecycle questions even if M30 still omits `Close`/destructor semantics.
- Makes the legacy interpreted `Pdf.ImageHandle` bridge more confusing because the same public record shape would have two meanings unless documented and migrated carefully.
- Still requires bytes import APIs before Image/Pdf interop works correctly.

Assessment: useful later for repeated-draw optimization, but too much surface for the first compiled interop milestone.

### C. Path/file drawing

Potential APIs:

```oct
Pdf.DrawImageFile(page: PdfPage, path: String, x: Int<px>, y: Int<px>) -> Int ! Error
Pdf.DrawImageFileSized(page: PdfPage, path: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error
```

Pros:

- Simple user story when an image already exists on disk.
- Does not require the Image sidecar.
- Avoids `Bytes` transfer cost for large files.
- Pdf sidecar implementation can open/decode/register the file directly.

Cons:

- Does not solve “I already have an `Image.ImageHandle`” unless the user saves/exports to a file first.
- Adds path lifecycle and artifact-attribution questions.
- Less compositional for generated or transformed images held in memory.
- May tempt wrapper authors to use files as implicit shared mutable state rather than explicit values.

Assessment: good future convenience API, not sufficient as the core Image-to-Pdf interop path.

### D. Image export to bytes plus Pdf direct bytes drawing

Potential Image APIs:

```oct
Image.EncodePng(image: ImageHandle) -> Bytes ! Error
Image.EncodeJpeg(image: ImageHandle, quality: Int) -> Bytes ! Error
```

Paired Pdf APIs are the direct bytes drawing APIs from strategy A.

Pros:

- Explicitly serializes the Image-owned resource into a normal Oct value.
- Keeps Image and Pdf sidecars independent.
- Enables a full compiled Image -> Pdf flow without new handle ownership rules.
- Reuses known codecs and existing Image sidecar storage.
- Can start with PNG only and add JPEG once format/quality policy is settled.

Cons:

- Requires additive Image API work as well as additive Pdf API work.
- PNG re-encoding may be expensive for JPEG source images or large rasters.
- Users must learn the two-step export/draw pattern.

Assessment: this is the recommended M30 strategy. It is strategy A plus the necessary Image-side export operation.

### E. True cross-sidecar handle broker

Possible shape: a runtime/global broker maps a handle from one sidecar family to another, or sidecars exchange opaque capabilities through a shared registry.

Pros:

- Could hide interop behind handle-looking APIs.
- Might enable zero-copy or shared-resource designs in a much larger runtime.

Cons:

- Violates sidecar-local handle policy.
- Requires lifecycle, ownership, revocation, package-manager, runtime, and security design far beyond Pdf images.
- Makes sidecars coupled and harder to test independently.
- Undermines family/type validation and makes raw handle confusion more likely.
- Is unnecessary when explicit `Bytes` or path transfer works.

Assessment: reject for M29/M30. Do not add cross-family handles, sidecar-to-sidecar calls, global handle brokers, shared registries, or handle serialization across program runs.

## 5. Recommended strategy

M30 should implement exactly one next strategy: **direct Pdf bytes drawing plus Image encode-to-bytes, PNG first**.

Add/mark compiled-supported:

```oct
Image.EncodePng(image: ImageHandle) -> Bytes ! Error
Pdf.DrawImageBytes(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>) -> Int ! Error
Pdf.DrawImageBytesSized(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error
```

Do not implement Pdf-owned `Pdf.ImageHandle` import in M30 unless implementation work uncovers a hard backend requirement that bytes drawing cannot satisfy. If such a hard requirement appears, stop and write down the narrower blocker rather than introducing a handle table opportunistically.

Rationale:

- It uses existing `Bytes` transport.
- It avoids new handle tables and lifecycle semantics.
- It keeps sidecar ownership clean and debuggable.
- It enables compiled Image -> Pdf interop immediately.
- It preserves legacy interpreted image APIs unchanged.
- It leaves repeated-draw optimization for a later, explicitly designed Pdf image-handle milestone.

## 6. Public API compatibility plan

M30 should be additive.

- Existing `Pdf.DrawImage` and `Pdf.DrawImageSized` remain public and continue to support the current interpreted legacy bridge behavior.
- Existing `Libraries/Pdf/Pdf.Core.octest` image tests should remain interpreted tests of that bridge.
- `Pdf.DrawImage` and `Pdf.DrawImageSized` remain compiled-deferred unless and until a Pdf-owned `Pdf.ImageHandle` import design is implemented.
- New docs and compiled examples should steer users to `Image.EncodePng` plus `Pdf.DrawImageBytes` / `Pdf.DrawImageBytesSized`.
- `Pdf.ImageHandle { Handle: raw }` should be described as an interpreted compatibility shell, not as a future compiled contract.
- If Pdf-owned image handles are added later, they should be introduced through explicit import APIs, not by accepting `Image.ImageHandle` or raw `Int` identities.

This plan preserves public APIs and tests while giving compiled users a correct path.

## 7. Transport and compiler requirements

No new Octxiliary protocol transport is needed for M30.

Required transport shapes already exist or are represented by existing rules:

- `Image.EncodePng`: `Image.ImageHandle` handle argument, `Bytes` fallible return.
- Optional future `Image.EncodeJpeg`: `Image.ImageHandle` handle argument, `Int` quality argument, `Bytes` fallible return.
- `Pdf.DrawImageBytes`: `Pdf.PdfPage` handle argument, `Bytes`, `String`, `Int<px>`, `Int<px>`, `Int` fallible return.
- `Pdf.DrawImageBytesSized`: same as `DrawImageBytes` plus `Int<px>` width and height.

The existing generic wrapper lowering supports:

- handle argument packing with family/type metadata;
- handle return reconstruction;
- `Bytes` value arguments and returns;
- scalar `String` and `Int` values;
- dimensioned `Int<px>` as integer payloads;
- fallible wrapper result propagation.

Known compiler/protocol gap for M30: none identified from the M29 audit. If M30 finds a bug in generic `Bytes` args/returns or multi-wrapper compiled orchestration, that bug should be fixed narrowly as a compiler/runtime defect without changing the protocol or adding a new transport type.

Manifest/API sketch:

- `Libraries/Image/Image.Core.oct` adds `fn EncodePng(image: ImageHandle) -> Bytes ! Error { return ImageEncodePng(image.Handle)? }`.
- `Libraries/Image/manifest.oct` adds `WrapperFunction { OctName: "EncodePng" WireName: "ImageEncodePng" Args: ["Image.ImageHandle"] Return: "Bytes" Fallible: true }`.
- `Libraries/Pdf/Pdf.Core.oct` adds direct bytes drawing functions that call `PdfDrawImageBytes(page.Handle, bytes, format, x, y)?` and `PdfDrawImageBytesSized(page.Handle, bytes, format, x, y, width, height)?`.
- `Libraries/Pdf/manifest.oct` adds wrapper functions for the new Pdf calls with `Pdf.PdfPage`, `Bytes`, `String`, and `Int<px>` argument metadata.
- `Pdf.DrawImage` and `Pdf.DrawImageSized` are not added to the compiled manifest in M30.

## 8. Sidecar design sketch

### `octxiliary-image`

Implement an encode helper using the existing image table:

- Accept one `Image.ImageHandle` argument.
- Validate the handle family/type/positive sidecar-local ID through the existing table lookup.
- Encode the stored `image.Image` to PNG bytes using the existing PNG encoder path.
- Return `octxiliary.Value{Kind: ValueBytes, Bytes: encoded}`.
- Return sidecar errors for invalid handles or backend encode failures.
- Optionally add JPEG later as `Image.EncodeJpeg(image, quality)` with explicit quality validation, e.g. `1..100`, but PNG should be enough for M30.

### `octxiliary-pdf`

Implement direct bytes drawing against the existing Pdf page table:

- Accept `Pdf.PdfPage`, `Bytes`, `String format`, `x`, `y`, and optionally `width`, `height`.
- Validate page handle family/type/ID through the existing page table.
- Validate `x` and `y` according to current Pdf coordinate rules: non-negative `Int<px>`.
- Validate explicit width/height as positive `Int<px>` for the sized variant.
- Normalize supported format strings deterministically, starting with `"png"` and possibly accepting `"PNG"` only by documented case-insensitive normalization if desired.
- Decode or register the bytes inside the Pdf renderer for a single draw.
- For the natural-size variant, determine size from the decoded image or backend image info.
- Draw using the same `px` to PDF-point conversion as existing Pdf text/page paths.
- Return `Int` value `0` on success.
- Return sidecar errors for invalid bytes, unsupported format, invalid page handle, invalid coordinate/size, and backend failures.

The Pdf sidecar must not accept an `Image.ImageHandle`, must not ask the Image sidecar to encode anything, and must not maintain shared Image-side resource state.

## 9. Test plan for recommended M30

### Sidecar tests

`cmd/octxiliary-image`:

- `ImageEncodePng` returns non-empty bytes for a loaded PNG fixture.
- `ImageEncodePng` returns non-empty bytes for a loaded JPEG fixture.
- Invalid image handle returns an error.
- Corrupt/missing load tests remain covered by existing Image behavior.

`cmd/octxiliary-pdf`:

- `PdfDrawImageBytes` succeeds for valid PNG bytes.
- `PdfDrawImageBytesSized` succeeds for valid PNG bytes.
- Invalid bytes return an error.
- Unsupported format returns an error.
- Invalid page handle returns an error.
- Invalid coordinate and invalid size behavior matches current Pdf validation rules.

### Compiled package tests

Add a focused compiled Image -> Pdf bytes interop test:

1. Load an image fixture through `Image.Load`.
2. Encode it with `Image.EncodePng`.
3. Create a page with `Pdf.NewPage`.
4. Draw with `Pdf.DrawImageBytes` or `Pdf.DrawImageBytesSized`.
5. Save with `Pdf.Save`.
6. Assert the PDF exists and is non-empty.

Add missing-sidecar diagnostics coverage where practical:

- missing `octxiliary-image` reports a clear error at `Image.Load` or `Image.EncodePng`;
- missing `octxiliary-pdf` reports a clear error at `Pdf.NewPage`, `Pdf.DrawImageBytes`, or `Pdf.Save`.

### Interpreted tests

- Existing Pdf image tests remain passing unchanged.
- New additive APIs should also pass interpreted if M30 implements interpreted builtins for API parity.
- The interpreted legacy bridge should not be removed, renamed, or redefined in M30.

### Regression tests

Run at least:

```sh
go test ./internal/octxiliary
go test ./internal/pkgmgr ./internal/project
go test ./cmd/oct -run 'GenericOctxiliary|CompiledOctxiliary|Hash|Compression|Time|Text|Archive|Json|Csv|Markdown|Plot|Xlsx|Image|Pdf|UtilityWrappers'
go test ./cmd/oct -run '^TestPkgWrappers'
go test ./internal/... ./cmd/oct
```

M30 may add narrower sidecar tests under `cmd/octxiliary-image` and `cmd/octxiliary-pdf` if those packages gain test files.

## 10. Artifact/path considerations

Bytes transfer avoids temporary files, path cleanup, and implicit file-sharing contracts between sidecar families. That is the correct M30 default.

The tradeoff is memory and frame size. Large images will be decoded in `octxiliary-image`, encoded to a `Bytes` value, materialized in the compiled Oct process, transmitted to `octxiliary-pdf`, and decoded or registered there. The current textual bytes encoding may also be verbose. This is acceptable for M0/M30 because correctness and ownership are more important than large-image throughput.

Future optimizations can add explicit path/file APIs or artifact/file-backed transfer without changing handle semantics. A future file-backed transfer should remain an explicit value or artifact path chosen by the Oct program or package-manager/runtime policy; it should not become hidden sidecar-to-sidecar sharing.

## 11. Third-party wrapper authoring implications

Pdf/Image interop should become the model for third-party wrapper resource sharing:

- Wrapper authors should treat sidecar handles as family-local capabilities.
- Public `Handle: Int` fields are not authorization to share raw integers across packages.
- Cross-wrapper resource sharing should use serialized values (`Bytes`, `String`, numeric arrays, records) or explicit export/import operations.
- If a consuming wrapper wants to cache imported resources, it should own its own imported handle type and expose an explicit import API.
- Package-manager tooling can later display which wrapper APIs exchange `Bytes` or path artifacts, making data movement inspectable.
- Explicit transfer APIs are more portable than hidden sidecar interop because they survive separate processes, separate hosts, test replay, artifact capture, and future sandboxing.

## 12. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Large image memory cost | Accept for M30; document that bytes transfer is the simple correct path; consider future artifact/file-backed transfer. |
| Repeated decoding/registering cost | Defer Pdf-owned `ImportImageBytes -> Pdf.ImageHandle` to a later optimization milestone. |
| Format string ambiguity | Start with a small documented set, preferably `"png"`; normalize case only if documented; reject unsupported values clearly. |
| Divergence between interpreted `DrawImage` and compiled `DrawImageBytes` | Keep legacy APIs unchanged and document the bridge as interpreted compatibility; add new interpreted builtins for bytes APIs if M30 wants parity. |
| Public API proliferation | Add only the minimum three APIs in M30; defer file and Pdf-owned import APIs. |
| Confusion between `Image.ImageHandle` and `Pdf.ImageHandle` | Use names and docs that emphasize `Image.EncodePng` exports bytes and `Pdf.DrawImageBytes` consumes bytes; do not compile-support legacy raw-handle `DrawImage`. |
| Accidental cross-family handle loopholes | Keep manifest transport types family-qualified; do not add Pdf image functions that accept `Image.ImageHandle`; test invalid handle family/type paths. |
| Missing sidecar diagnostics complexity | Add focused missing-sidecar tests around both Image and Pdf calls; keep error messages tied to the failing sidecar command. |
| Backend-specific natural image sizing | Derive natural size from decoded bytes or registered image info in one helper and cover it with sidecar tests. |
| JPEG quality and format policy | Defer `Image.EncodeJpeg` unless M30 has spare scope; PNG first is sufficient for interop. |

## 13. Final recommendation

**Exact recommended M30 milestone:** implement compiled Pdf/Image interop using explicit Oct-mediated bytes transfer:

1. Add `Image.EncodePng(image: ImageHandle) -> Bytes ! Error`.
2. Add `Pdf.DrawImageBytes(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>) -> Int ! Error`.
3. Add `Pdf.DrawImageBytesSized(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`.
4. Add Image sidecar PNG encode support.
5. Add Pdf sidecar PNG bytes draw support.
6. Add focused sidecar, compiled, interpreted-parity, and regression tests.
7. Mark only those new APIs compiled-supported.

**Exact M30 non-goals:** no cross-family handles; no Pdf direct consumption of `Image.ImageHandle`; no `octxiliary-image` to `octxiliary-pdf` calls; no global broker; no shared registries; no handle serialization across runs; no `Close`/destructor semantics; no protocol transport changes; no package-manager sidecar build changes; no permission prompts; no lockfiles; no migration of legacy `DrawImage` / `DrawImageSized` to compiled support; no redesign or removal of interpreted Pdf image behavior; no unrelated compiler/codegen fixes.

**Future follow-up:** if repeated image draws become important, add a later Pdf-owned image resource milestone:

```oct
Pdf.ImportImageBytes(bytes: Bytes, format: String) -> Pdf.ImageHandle ! Error
Pdf.DrawImage(page: PdfPage, image: Pdf.ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error
Pdf.DrawImageSized(page: PdfPage, image: Pdf.ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error
```

That follow-up must keep `Pdf.ImageHandle` owned by `octxiliary-pdf` and must treat the old raw integer bridge as interpreted compatibility only.

## M30 implementation note

M30 implements the design recommended here with an explicit PNG bytes transfer path:

- `Image.EncodePng(image: Image.ImageHandle) -> Bytes ! Error` exports the Image sidecar's image handle as serialized PNG bytes.
- `Pdf.DrawImageBytes(...)` and `Pdf.DrawImageBytesSized(...)` pass those bytes to the Pdf sidecar for one direct draw operation.
- The compiled Oct binary mediates the transfer; `octxiliary-image` and `octxiliary-pdf` still never communicate directly.
- Handles remain sidecar-family-local. M30 does not add cross-family handles, a global broker, shared registries, Pdf-owned image handles, path/file drawing APIs, or a transport/protocol revision.
- PNG is the supported M30 image bytes format. Existing interpreted `Pdf.DrawImage` / `Pdf.DrawImageSized` remain legacy bridge APIs and are not migrated to compiled support by M30.
