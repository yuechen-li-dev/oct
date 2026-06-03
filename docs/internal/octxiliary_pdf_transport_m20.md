# Octxiliary M20 Pdf Compiled Migration and Image Interop Design Audit

## 1. Executive summary

M20 is a design/audit milestone only. It does not migrate `Libraries/Pdf`, add protocol/compiler/package-manager behavior, change Image, change Pdf tests, introduce cross-family handles, or add a global handle broker.

**Recommended next Pdf migration strategy:** M21 should migrate the Pdf text/page/save subset only:

- `NewPage(width: Int<px>, height: Int<px>) -> PdfPage ! Error`
- `DrawText(page: PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`
- `DrawTextStyled(page: PdfPage, x: Int<px>, y: Int<px>, text: String, style: TextStyle) -> Int ! Error`
- `Save(page: PdfPage, path: String) -> Int ! Error`
- `DefaultTextStyle() -> TextStyle` remains pure/local Oct code and should not be sidecar-backed.

**Can Pdf migrate without cross-family handles?** Yes, but only the text/page/save subset can migrate safely now. `Pdf.PdfPage` is naturally a Pdf-owned sidecar handle, and `Pdf.TextStyle` is a Pdf-owned record argument. Existing M18/M19 handle transport plus M16 record-argument transport are enough for that subset.

**Recommended M21 scope:** add an `octxiliary-pdf` sidecar and Pdf wrapper manifest metadata for the text/page/save subset, declare `Pdf.PdfPage` as a handle transport type, declare `Pdf.TextStyle` as a record transport type, and leave image functions compiled-unsupported.

**Deferred:** `DrawImage`, `DrawImageSized`, and the current `Pdf.ImageHandle` bridge must remain interpreted-only until Pdf receives an image interop design that preserves M0 handle ownership. The likely follow-up is either a Pdf-owned image import API (`LoadImage` or `ImportImage`) or path/bytes drawing APIs (`DrawImageFile`, `DrawImageBytes`, sized variants). M0 should continue to reject true cross-sidecar `Image.ImageHandle` sharing.

## 2. Current Pdf API inventory

The public Pdf wrapper currently lives in `Libraries/Pdf/Pdf.Core.oct`. Its tests live in `Libraries/Pdf/Pdf.Core.octest`. The interpreted builtins are registered by `pdfWrapperBuiltins` in `internal/interpret/wrapper_pdf.go`.

### Public functions

| Function | Signature | Public wrapper behavior | Interpreted builtin path | Tests using it | Required transport shapes | M18/M19 migratable? |
| --- | --- | --- | --- | --- | --- | --- |
| `NewPage` | `NewPage(width: Int<px>, height: Int<px>) -> PdfPage ! Error` | Calls `PdfNewPage(width, height)?` and wraps the returned integer as `PdfPage { Handle: handle }`. | `PdfNewPage` -> `evalPdfNewPageBuiltin` -> `newWrapperPDFPage` -> `i.pdfPages.allocate(page)`. | `BasicPageCreateTextAndSave`, `DrawImageAndSave`, `DrawImageSizedAndStyledText`, `SaveInvalidPathFails`, `InvalidImageHandleFails`. | Args: two dimensioned integers (`Int<px>`). Return: Pdf-owned handle record `Pdf.PdfPage`. Fallible error channel. | **Yes.** Requires a handle return for `Pdf.PdfPage`, which current handle transport supports. |
| `DrawText` | `DrawText(page: PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error` | Passes `page.Handle`, `x`, `y`, and `text` to `PdfDrawText`. | `PdfDrawText` -> `evalPdfDrawTextBuiltin` -> `evalPdfTextArgs` -> `i.pdfPages.get(pageHandle)` -> `page.drawText(...)`. | `BasicPageCreateTextAndSave`, `InvalidPageHandleFails`. | Args: Pdf handle, two `Int<px>`, `String`. Return: `Int`. Fallible error channel. | **Yes.** Pdf handle arguments plus scalar/string args are supported. |
| `DrawTextStyled` | `DrawTextStyled(page: PdfPage, x: Int<px>, y: Int<px>, text: String, style: TextStyle) -> Int ! Error` | Currently flattens `TextStyle` into `style.Size`, `style.ColorR`, `style.ColorG`, `style.ColorB` for `PdfDrawTextStyled`. | `PdfDrawTextStyled` -> `evalPdfDrawTextStyledBuiltin` -> `evalPdfTextArgs` -> `pxArg` and `colorChannelArg` validation -> `page.drawText(...)`. | `DrawImageSizedAndStyledText`. | Recommended compiled shape: Args: Pdf handle, two `Int<px>`, `String`, `Pdf.TextStyle` record. Return: `Int`. Fallible error channel. | **Yes.** It is migratable with current record argument transport. The public Oct wrapper may be left as-is only if M21 chooses flattened args; the recommended manifest is cleaner as a declared record arg and may need wrapper/compiler alignment with the actual public call shape. |
| `DrawImage` | `DrawImage(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error` | Passes `page.Handle` and `image.Handle` to `PdfDrawImage`. | `PdfDrawImage` -> `evalPdfDrawImageBuiltin` -> `evalPdfImageArgs` -> `i.pdfPages.get(pageHandle)` and `i.images.get(imageHandle)` -> `page.drawImage(...)`. | `DrawImageAndSave`, `InvalidImageHandleFails`. | Args if migrated directly: Pdf handle plus an image handle. The current public record is `Pdf.ImageHandle`, but tests populate it from `ImageLoad` raw integer identity. | **No.** Blocked by image interop. Compiled Pdf must not treat `Pdf.ImageHandle` as a disguised `Image.ImageHandle` from `octxiliary-image`. |
| `DrawImageSized` | `DrawImageSized(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error` | Passes `page.Handle`, `image.Handle`, coordinates, width, and height to `PdfDrawImageSized`. | `PdfDrawImageSized` -> `evalPdfDrawImageSizedBuiltin` -> `evalPdfImageArgs` -> `i.images.get(imageHandle)` -> size validation -> `page.drawImage(...)`. | `DrawImageSizedAndStyledText`. | Args if migrated directly: Pdf handle, image handle, four `Int<px>`. | **No.** Blocked by image interop for the same reason as `DrawImage`. |
| `Save` | `Save(page: PdfPage, path: String) -> Int ! Error` | Passes `page.Handle` and `path` to `PdfSave`. | `PdfSave` -> `evalPdfSaveBuiltin` -> `i.pdfPages.get(pageHandle)` -> `page.save(path)`. | `BasicPageCreateTextAndSave`, `DrawImageAndSave`, `DrawImageSizedAndStyledText`, `SaveInvalidPathFails`. | Args: Pdf handle, `String`. Return: `Int`. Fallible error channel. | **Yes.** Pdf handle argument plus string path and fallible integer result are supported. |
| `DefaultTextStyle` | `DefaultTextStyle() -> TextStyle` | Constructs `TextStyle { Size: 16px ColorR: 0 ColorG: 0 ColorB: 0 }` locally. | No builtin. | `DrawImageSizedAndStyledText`. | No transport. Pure record construction in Oct. | **Yes, local only.** It does not need Octxiliary metadata. |

### Public record/handle types

| Type | Current public shape | Current role | M21 recommendation |
| --- | --- | --- | --- |
| `PdfPage` | `record PdfPage { Handle: Int }` | Page/document resource identity allocated by Pdf builtins and stored in `i.pdfPages`. | Declare `Pdf.PdfPage` as a `handle` transport type. Wire payload carries family/type/ID; public record remains compatibility shell. |
| `TextStyle` | `record TextStyle { Size: Int<px>, ColorR: Int, ColorG: Int, ColorB: Int }` | Text draw style: size in px and RGB channels. | Declare `Pdf.TextStyle` as a `record` transport type for `DrawTextStyled`; validate size and color channels in sidecar. |
| `Pdf.ImageHandle` | `record ImageHandle { Handle: Int }` | Documented as bridging to the existing `ImageLoad` handle identity. Tests wrap raw `ImageLoad` integers in this record. | Do not declare or migrate in M21. Keep interpreted behavior unchanged; design an additive future Pdf-owned image import or path/bytes API. |

## 3. Current interpreted image bridge

The current Pdf image path relies on an in-process interpreter convenience:

1. Pdf tests create a PNG fixture with Plot helpers.
2. Pdf tests call `ImageLoad(pngPath)!`, which allocates an image in the interpreter's `i.images` store and returns a raw integer handle.
3. Pdf tests then construct `Pdf.ImageHandle { Handle: imageHandle }`, even though that record is local to the Pdf package.
4. `DrawImage` and `DrawImageSized` pass `image.Handle` to Pdf builtins.
5. `evalPdfImageArgs` in the Pdf interpreter implementation calls `i.images.get(imageHandle)` directly, then hands the decoded `image.Image` to the Pdf renderer.

This works only because interpreted Pdf and Image builtins share the same interpreter process and the same `i.images` handle store. The integer is not merely data; it is a process-local identity into one Go interpreter object graph.

Compiled Octxiliary cannot preserve this by reinterpreting a `Pdf.ImageHandle` integer as an `Image.ImageHandle` from `octxiliary-image`. M0 handle policy deliberately makes handles sidecar-family-local capabilities. A compiled `Image.Load` handle belongs to the Image sidecar (`family = "Image"`, `type = "Image.ImageHandle"`), while a future compiled Pdf image handle would belong to the Pdf sidecar (`family = "Pdf"`, likely `type = "Pdf.ImageHandle"`). Those are not interchangeable, even if both public records contain a single `Handle: Int` field.

## 4. Transport feasibility table

| Public function/type | Classification | Reason |
| --- | --- | --- |
| `NewPage` | `migratable_now` | Existing transports support `Int<px>` args and handle returns. Pdf can own `Pdf.PdfPage` handles in `octxiliary-pdf`. |
| `DrawText` | `migratable_now` | Existing transports support handle args, `Int<px>`, `String`, `Int` return, and fallible errors. |
| `DrawTextStyled` | `migratable_with_current_record_handle_transport` | Existing transports support a handle arg plus a declared `Pdf.TextStyle` record arg. Sidecar must validate style fields. |
| `Save` | `migratable_now` | Existing transports support handle args, `String` path, `Int` return, and fallible errors. |
| `DefaultTextStyle` | `migratable_now` as pure local | No sidecar needed; it constructs a record locally. |
| `DrawImage` | `blocked_by_image_interop` | Current public behavior depends on `i.images.get(imageHandle)`. Cross-sidecar Image handle consumption is forbidden in M0. |
| `DrawImageSized` | `blocked_by_image_interop` | Same as `DrawImage`; also needs size args but those are not the blocker. |
| `Pdf.ImageHandle` | `blocked_by_image_interop` for compiled support | It currently aliases Image's interpreted integer identity. A Pdf-owned compiled version needs a new import/source story. |

No Pdf public function is currently blocked by dynamic `Any`, package-manager sidecar builds, lockfiles, native permission prompts, or non-fallible sidecar errors. The only transport-shape caveat for M21 is ensuring the manifest/compiler path accepts dimensioned `Int<px>` in args/returns and record fields; current metadata validation already treats `Int<...>` as a supported transport type, and current compiled lowering maps dimensioned integers through integer transport.

## 5. Candidate migration strategies

### A. Text/page/save-only Pdf migration

**Migrate in M21:**

- `NewPage`
- `DrawText`
- `DrawTextStyled`
- `Save`
- keep `DefaultTextStyle` pure/local

**Defer:**

- `DrawImage`
- `DrawImageSized`
- compiled meaning of `Pdf.ImageHandle`

**Pros:**

- Fastest path to compiled PDF text reports.
- Uses existing handle transport and record argument transport.
- Avoids the cross-family handle problem completely.
- Preserves current public image APIs and tests for interpreted mode.
- Produces a complete useful subset: page creation, styled text, save-to-file.

**Cons:**

- The full current `Libraries/Pdf/Pdf.Core.octest` includes image facts, so M21 cannot simply claim the whole Pdf test file is compiled-supported unless tests are split/focused.
- Public Pdf image functions remain interpreted-only / compiled-blocked.
- Users may need clear diagnostics/documentation explaining why text Pdf compiles while image Pdf does not.

**Judgment:** Recommended for M21. It is the smallest convergent implementation that materially improves compiled Oct without violating M0 handle policy.

### B. Pdf-owned image handle import

**Future additive API option:**

- `LoadImage(path: String) -> Pdf.ImageHandle ! Error`, or
- `ImportImage(path: String) -> Pdf.ImageHandle ! Error`

Then migrate:

- `DrawImage`
- `DrawImageSized`

**Pros:**

- Keeps image handles sidecar-local to Pdf.
- Preserves the existing conceptual `Pdf.ImageHandle` type.
- Does not share `Image.ImageHandle` across sidecars.
- Lets the Pdf sidecar decode/register images once and reuse them across draw calls.

**Cons:**

- Public API extension required.
- Interpreted mode needs a matching implementation so public behavior remains coherent.
- Existing tests that wrap `ImageLoad` handles should remain interpreted compatibility tests, while new compiled tests should use the Pdf-owned import API.
- Users must understand that `Image.Load` and `Pdf.LoadImage` produce different handle families despite similar record shapes.

**Judgment:** Good future direction if the project wants handle-based Pdf image reuse. Do not include in M21 because it expands public API and tests beyond the text migration.

### C. Path/bytes image drawing helpers

**Future additive API options:**

- `DrawImageFile(page: PdfPage, path: String, x: Int<px>, y: Int<px>) -> Int ! Error`
- `DrawImageFileSized(page: PdfPage, path: String, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`
- `DrawImageBytes(page: PdfPage, bytes: Bytes, format: String, x: Int<px>, y: Int<px>) -> Int ! Error`
- `DrawImageBytesSized(...)`

**Pros:**

- Avoids Pdf image handles entirely.
- Provides an easy user story for drawing images into PDFs.
- Keeps all decoding/validation in the Pdf sidecar for each call.
- Path helpers use transport shapes already proven by other wrappers; bytes helpers align with existing bytes transport if the desired function shape stays simple.

**Cons:**

- Public API extension required.
- Bytes/format support needs careful validation and explicit supported format rules.
- Path helpers may reread/redecode the same file for repeated draws unless implementation adds internal caching.
- Does not make existing `DrawImage` / `DrawImageSized` compiled-supported.

**Judgment:** Also a strong future direction, especially for simple user ergonomics. Choose between B and C in a dedicated Pdf image milestone after M21 proves the text sidecar.

### D. True cross-sidecar handle broker

**Pros:**

- Would allow `Image.ImageHandle` values produced by `octxiliary-image` to flow into `octxiliary-pdf`.
- Could generalize future resource sharing between sidecars.

**Cons:**

- Violates M0 simplicity and the M17/M18/M19 sidecar-local handle policy.
- Requires global broker/lifetime/ownership/error/permission design.
- Requires runtime/package-manager coordination beyond current sidecar launch model.
- Makes numeric public handles even more misleading because the real authority would be broker state.
- Not justified by current Pdf needs.

**Judgment:** Explicitly reject for M21 and for the near-term Pdf image follow-up. Revisit only if multiple independent wrapper families genuinely need shared resources and the project is ready for an M1+ handle model.

## 6. Recommended M21 scope

The exact next implementation milestone should be:

**M21 — migrate Pdf text/page/save to compiled Octxiliary.**

M21 should:

1. Add `cmd/octxiliary-pdf`.
2. Add Pdf wrapper manifest metadata for the text/page/save subset.
3. Declare `Pdf.PdfPage` as a handle transport type.
4. Declare `Pdf.TextStyle` as a record argument transport type.
5. Migrate `NewPage`, `DrawText`, `DrawTextStyled`, and `Save`.
6. Keep `DefaultTextStyle` as pure/local Oct code.
7. Leave `DrawImage`, `DrawImageSized`, and `Pdf.ImageHandle` without compiled support.
8. Document image interop as deferred and rejected for cross-family M0 handles.

M21 should not:

- migrate Pdf image drawing;
- add `Pdf.LoadImage` / `Pdf.ImportImage` yet;
- add path/bytes image drawing helpers yet;
- consume `Image.ImageHandle` in `octxiliary-pdf`;
- add a global handle broker;
- add new protocol types;
- change current interpreted Pdf behavior;
- change or delete existing full Pdf image tests;
- generate lockfiles or package-manager sidecar builds.

## 7. Manifest sketch for the recommended Pdf subset

A future `Libraries/Pdf/manifest.oct` (or equivalent metadata location) should follow the existing wrapper metadata shape used by Image, Xlsx, and Plot. Sketch:

```oct
Wrapper {
    Name: "pdf"
    Family: "Pdf"
    Protocol: "octxiliary.v0"
    SidecarCommand: "octxiliary-pdf"
    GoModuleDir: "octxiliary"
    TransportTypes: [
        WrapperTransportType {
            Name: "Pdf.PdfPage"
            Kind: "handle"
            Fields: [
                WrapperTransportField { Name: "Handle" Type: "Int" }
            ]
        },
        WrapperTransportType {
            Name: "Pdf.TextStyle"
            Kind: "record"
            Fields: [
                WrapperTransportField { Name: "Size" Type: "Int<px>" },
                WrapperTransportField { Name: "ColorR" Type: "Int" },
                WrapperTransportField { Name: "ColorG" Type: "Int" },
                WrapperTransportField { Name: "ColorB" Type: "Int" }
            ]
        }
    ]
    Functions: [
        WrapperFunction { OctName: "NewPage" WireName: "PdfNewPage" Args: ["Int<px>", "Int<px>"] Return: "Pdf.PdfPage" Fallible: true },
        WrapperFunction { OctName: "DrawText" WireName: "PdfDrawText" Args: ["Pdf.PdfPage", "Int<px>", "Int<px>", "String"] Return: "Int" Fallible: true },
        WrapperFunction { OctName: "DrawTextStyled" WireName: "PdfDrawTextStyled" Args: ["Pdf.PdfPage", "Int<px>", "Int<px>", "String", "Pdf.TextStyle"] Return: "Int" Fallible: true },
        WrapperFunction { OctName: "Save" WireName: "PdfSave" Args: ["Pdf.PdfPage", "String"] Return: "Int" Fallible: true }
    ]
}
```

### Dimensioned `Int<px>` support note

M21 should verify, but should not need to invent, dimensioned integer transport. Current wrapper metadata validation accepts `Int<...>` strings, existing Image metadata uses `Int<px>` returns, existing Plot metadata uses `Int<px>` record fields, and compiler lowering treats dimensioned integers as integer payloads. Therefore the Pdf subset should not require a protocol extension for `Int<px>` args or record fields.

### Wrapper shape note for `DrawTextStyled`

The current interpreted public wrapper flattens `TextStyle` into four builtin args. The recommended compiled wrapper metadata treats `TextStyle` as a manifest-declared record argument because M16 already established record argument transport and because it preserves the public source signature. M21 implementation should choose one consistent compiled lowering path:

- Prefer the record-argument manifest shape above if generic wrapper lowering can bind directly to the public `DrawTextStyled` signature.
- If the current wrapper/compiler path only recognizes the flattened builtin call shape, M21 may use a flattened sidecar signature as a tactical compatibility step, but it should document that the public API is a record and migrate to record-argument transport when feasible.

The design recommendation remains record transport for `Pdf.TextStyle` because it is more inspectable and matches third-party wrapper authoring guidance.

## 8. Pdf sidecar design for the recommended subset

### Command and family

- Sidecar command: `cmd/octxiliary-pdf`
- Family: `Pdf`
- Protocol: `octxiliary.v0`

The sidecar should reject unknown families and unknown function names with clear errors, following the Image and Plot sidecar pattern.

### Handle table

- Handle type: `Pdf.PdfPage`
- Wire payload: `OctxiliaryValue { kind: "Handle" handleFamily: "Pdf" handleType: "Pdf.PdfPage" handleID: <positive> }`
- IDs: positive, process-local, monotonically allocated, not reused in M21.
- Lifetime: sidecar process lifetime.
- `Close`: no `Close`/destructor in M21.
- Validation: every `DrawText`, `DrawTextStyled`, and `Save` call must reject unknown page IDs and family/type mismatches.

### Renderer

Implementation options:

1. **Low-risk internal package extraction:** factor the existing interpreter Pdf page/rendering logic into an internal package shared by the interpreter and `cmd/octxiliary-pdf`. This reduces drift but touches production interpreter code and should be done only if the extraction is small and mechanically safe.
2. **Small duplication in sidecar:** duplicate the minimal current rendering behavior in `cmd/octxiliary-pdf` for M21. This avoids destabilizing interpreted Pdf and may be safer for a focused migration, but it creates drift risk.

Both options should use the existing `codeberg.org/go-pdf/fpdf` dependency if it is already present. M21 should preserve observable behavior for the supported subset:

- pixel-native coordinate API;
- `pt = px * (72 / 96)` conversion;
- positive page size validation;
- default font target and Helvetica fallback behavior;
- text color channel validation (`0..255`);
- positive text size validation;
- save path/backend errors mapped to sidecar errors;
- return `Int 0` on successful mutating/save operations.

### Sidecar request handling sketch

- `PdfNewPage(Int, Int) -> Handle(Pdf, Pdf.PdfPage, id)`
  - Validate width/height are positive.
  - Create a one-page PDF document with px-to-pt size.
  - Configure default font/fallback.
  - Allocate and return a positive page handle.
- `PdfDrawText(Handle, Int, Int, String) -> Int`
  - Validate page handle.
  - Draw with default font size/color.
  - Return `0`.
- `PdfDrawTextStyled(Handle, Int, Int, String, Record Pdf.TextStyle) -> Int`
  - Validate page handle and record shape.
  - Validate `Size > 0` and RGB channels in range.
  - Draw with supplied style.
  - Return `0`.
- `PdfSave(Handle, String) -> Int`
  - Validate page handle.
  - Write PDF to path.
  - Return `0`.

No Pdf image table is needed in M21.

## 9. Test plan for recommended M21

M20 does not add these tests. M21 should add them as part of implementation.

### Protocol/manifest/compiler coverage

Existing generic coverage likely already proves:

- scalar/string args;
- `Int<px>` args/returns;
- `String` path args;
- manifest-declared record args;
- handle returns;
- handle args;
- invalid family/type checks;
- sidecar error propagation for fallible functions.

Add Pdf-specific coverage only where it proves Pdf metadata or wrapper integration, not generic transport already covered elsewhere.

### Sidecar tests

Add `cmd/octxiliary-pdf` tests for:

- `NewPage` returns a positive `Pdf.PdfPage` handle.
- `DrawText` succeeds and returns `Int 0`.
- `DrawTextStyled` succeeds and returns `Int 0`.
- `Save` writes a non-empty PDF file.
- Invalid page handle returns a sidecar error.
- Invalid handle family/type returns a sidecar error.
- Invalid style color channel returns a sidecar error.
- Invalid or non-positive style size returns a sidecar error.
- Invalid page size returns a sidecar error.
- Invalid save path returns a sidecar error.

### Compiled Pdf focused tests

Add focused compiled tests for:

- basic page text/save;
- styled text/save;
- invalid page handle;
- invalid save path;
- missing `octxiliary-pdf` diagnostic.

### Regression tests

Run standard wrapper regression areas after M21:

- Image;
- Xlsx;
- Plot;
- Csv;
- Markdown;
- Json;
- Archive;
- Text;
- Time;
- Compression;
- Hash;
- utility wrappers.

## 10. How to handle existing Pdf tests

`Libraries/Pdf/Pdf.Core.octest` currently contains both text-only tests and image tests. M21 should not modify or delete those interpreted compatibility tests just to make compiled Pdf look complete.

Recommended M21 test handling:

1. Add a focused compiled Pdf text test file or test harness that exercises only the M21-supported subset.
2. Keep the existing full Pdf tests unchanged for interpreted mode.
3. Do not claim the entire Pdf package test suite is compiled-supported until image interop is solved.
4. Optionally, later split Pdf tests into text-only and image-focused files if the test runner needs package-level compiled selection. That split should be mechanical and should preserve interpreted coverage.

This avoids confusing users and maintainers: compiled Pdf support after M21 means text/page/save, not image embedding.

## 11. Public API compatibility

M21 should preserve public API compatibility:

- Do not remove `DrawImage`.
- Do not remove `DrawImageSized`.
- Do not change `Pdf.ImageHandle`.
- Do not change the existing interpreted bridge behavior.
- Do not pretend image APIs compile.
- Emit or document clear compiled unsupported behavior for image functions.

Future image APIs should be additive if possible:

- `Pdf.LoadImage(path: String) -> Pdf.ImageHandle ! Error`
- `Pdf.ImportImage(path: String) -> Pdf.ImageHandle ! Error`
- `Pdf.DrawImageFile(...)`
- `Pdf.DrawImageFileSized(...)`
- `Pdf.DrawImageBytes(...)`
- `Pdf.DrawImageBytesSized(...)`

If a future milestone redefines `Pdf.ImageHandle` as Pdf-owned in compiled mode, it must preserve interpreted tests or provide an explicit staged compatibility path. The current raw integer bridge should be treated as legacy interpreter convenience, not as the long-term compiled contract.

## 12. Third-party wrapper implications

Pdf is the clearest warning case for third-party wrapper authors:

- Handle scopes matter. A public record with `Handle: Int` is not just an integer when compiled.
- Sidecar family boundaries matter. A handle from family `Image` is not a handle from family `Pdf`.
- Public records can look identical but still be semantically incompatible handles.
- Wrapper authoring docs should warn against raw `Handle: Int` sharing across wrappers.
- Package-manager and registry rendering should preserve `Family`, handle type, and transport metadata so users can inspect resource ownership.
- A wrapper that needs a resource from another wrapper should prefer explicit import/copy APIs (`Pdf.ImportImage(path)`, bytes transfer, serialized file exchange) over shared handles unless a future global broker exists.
- If third-party authors need handle interop, they should design one sidecar as the owner and expose import/export operations rather than relying on another sidecar's process-local IDs.

This strengthens the M17/M18/M19 model: handle records are public compatibility shells; the compiled capability is the typed wire handle plus sidecar-owned table.

## 13. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Accidentally allowing cross-family handles | Declare only `Pdf.PdfPage` for M21; do not declare `Pdf.ImageHandle`; keep generated handle validation checking family/type/positive ID; sidecar rejects mismatches. |
| Test suite confusion if full Pdf is expected compiled | Add focused compiled text/save tests; keep full image tests interpreted; document M21 scope explicitly. |
| Divergence between interpreted and compiled image behavior | Do not migrate image behavior in M21; preserve interpreted bridge; design additive image APIs later. |
| Font path/resource availability in sidecar | Reuse current font lookup behavior or factor it into a shared package; keep Helvetica fallback; test output creation without assuming Inter is present. |
| Duplicated Pdf rendering logic drifting | Prefer small shared internal renderer extraction if safe; if duplicated, include regression tests and document exact behavior parity requirements. |
| Dimensioned `Int<px>` metadata mismatch | Verify manifest validation and compiled lowering with Pdf metadata; existing Image and Plot metadata indicate support already exists. |
| Record argument mismatch for `DrawTextStyled` | Prefer a declared `Pdf.TextStyle` record arg; if compiler/wrapper call shape requires flattened args, document tactical mismatch and isolate it for cleanup. |
| Save path behavior differs across sidecar/interpreter | Map filesystem/backend errors consistently to fallible sidecar errors; test invalid save paths. |
| Non-fallible sidecar failures | Not a blocker for the recommended subset because sidecar-backed Pdf functions are fallible; `DefaultTextStyle` is pure/local. |
| Public API additions causing fragmentation | Choose exactly one future image API strategy in a dedicated milestone; keep additions additive and document relationships to existing `DrawImage`. |
| Over-scoping M21 into image interop | Treat image support as an explicit non-goal; stop at text/page/save once tests pass. |

## 14. Final recommendation

M21 should be exactly:

**M21 — Pdf text/page/save compiled migration.**

Implement:

- `cmd/octxiliary-pdf`;
- wrapper metadata for family `Pdf` and command `octxiliary-pdf`;
- `Pdf.PdfPage` handle transport;
- `Pdf.TextStyle` record argument transport;
- compiled support for `NewPage`, `DrawText`, `DrawTextStyled`, and `Save`;
- pure/local `DefaultTextStyle` behavior;
- sidecar and compiled focused tests for the supported subset.

M21 non-goals:

- no `DrawImage` / `DrawImageSized` compiled migration;
- no `Pdf.ImageHandle` compiled support;
- no `Image.ImageHandle` consumption in `octxiliary-pdf`;
- no cross-family handles;
- no global handle broker;
- no new protocol types;
- no package-manager sidecar builds;
- no public API removal or interpreted behavior changes;
- no lockfiles or native permission prompts.

Future follow-up:

- Run a dedicated Pdf image milestone after M21.
- Compare `Pdf.LoadImage` / `Pdf.ImportImage` against `DrawImageFile` / `DrawImageBytes` with user ergonomics, caching, validation, and interpreted compatibility as explicit criteria.
- Keep the cross-sidecar broker rejected unless the project intentionally starts a broader post-M0 handle lifecycle/ownership design.

## M21 implementation update

M21 implements the M20 recommended first slice. `Libraries/Pdf/manifest.oct` declares Pdf as an `octxiliary.v0` wrapper package with sidecar command `octxiliary-pdf`, `Pdf.PdfPage` as a handle transport type, and `Pdf.TextStyle` as a record argument transport type.

The compiled-supported public functions are exactly `NewPage`, `DrawText`, `DrawTextStyled`, and `Save`. `DefaultTextStyle()` remains pure/local Oct. `DrawImage`, `DrawImageSized`, and `Pdf.ImageHandle` remain absent from compiled wrapper metadata.

The sidecar keeps a Pdf-local page handle table with positive monotonically increasing IDs. It does not accept Image-family handles and does not implement image drawing. Future compiled image interop should add a Pdf-owned import/path/bytes API rather than sharing `octxiliary-image` handle values.
