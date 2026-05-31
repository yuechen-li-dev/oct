# Octxiliary M17 Handle Transport Design Audit

## 1. Executive summary

M17 is a design/audit milestone for making handle-backed standard-library wrappers migratable through compiled Octxiliary without treating handles as plain integers. No production protocol, compiler, package-manager, or library behavior changes are part of this milestone.

**Recommended handle wire shape:** add a first-class Octxiliary value kind named `"Handle"` in M18. Its payload should be explicit, inspectable, and non-pointer-bearing:

```text
OctxiliaryValue { kind: "Handle" handleFamily: "Xlsx" handleType: "IO.Workbook" handleID: 1 }
```

Use the field names `handleFamily`, `handleType`, and `handleID` because they are self-describing in diagnostics, registry renderings, traces, and generated code. `handleID` is intentionally only the sidecar-local numeric identity; `handleFamily` and `handleType` carry the capability metadata that the current public `Handle: Int` field cannot carry by itself.

**Recommended lifecycle model:** M0 handles are sidecar-process-lifetime capabilities. Handle IDs must be positive, sidecar-owned, monotonically allocated, not reused in M0, not serialized across program runs, and invalid after their sidecar exits or restarts. Oct values remain copyable records, but every copy names the same sidecar-owned resource. M0 should not add `Close`, linear/affine ownership, a global broker, host pointers, cross-run persistence, or cross-family transfer.

**Recommended first migration target:** M18 should implement handle transport M0 and migrate `IO.Xlsx` first. Xlsx has one handle type, scalar/string operations, existing invalid-handle and save-path coverage, and no image/PDF interop pressure.

**What remains deferred:** Image migration, Pdf migration, cross-family handles, direct Image-to-Pdf sharing, explicit destructors, leak diagnostics, handle serialization, general record returns, dynamic `Any`, package-manager sidecar builds, native permission prompts, lockfiles, and public API redesigns.

## 2. Current handle-backed API inventory

### A. IO/Xlsx

| Item | Current state |
| --- | --- |
| Public record types | `record Workbook { Handle: Int }` in `Libraries/IO/IO.Xlsx.oct`. |
| Public functions | `CreateWorkbook() -> Workbook`; `AddSheet(workbook: Workbook, name: String) -> Int ! Error`; `SetCellString(workbook: Workbook, sheet: String, cell: String, value: String) -> Int ! Error`; `SetCellFloat(workbook: Workbook, sheet: String, cell: String, value: Float) -> Int ! Error`; `SaveWorkbook(workbook: Workbook, path: String) -> Int ! Error`. |
| Interpreted builtin path | The Oct wrapper calls `XlsxCreateWorkbook`, `XlsxAddSheet`, `XlsxSetCellString`, `XlsxSetCellFloat`, and `XlsxSaveWorkbook`. The interpreter registers those names in `xlsxWrapperBuiltins`; `evalXlsxCreateWorkbookBuiltin` allocates an `*xlsxWorkbook` in `i.workbooks`; the mutating/save helpers recover the workbook through `evalWorkbookAndSheetArgs` / `evalWorkbookSheetCellArgs`. |
| Tests | `Libraries/IO/IO.Xlsx.octest` covers successful workbook creation/write/save, missing sheet writes, invalid workbook handles, invalid save extension, and invalid-handle save. `cmd/oct/io_xlsx_wrapper_test.go` drives the library root through `oct test` and asserts the `.xlsx` artifact exists and is non-empty. |
| Required handle behavior | A created workbook must return a positive sidecar-local workbook handle. Later operations must validate that the ID exists in the same Xlsx sidecar and names an `IO.Workbook`. Unknown IDs return wrapper errors. Saving requires `.xlsx` and at least one sheet. |
| Compiled migration difficulty | Low. One handle type, one family, scalar/string/float arguments, `Int ! Error` mutators, no record arguments beyond the public handle record, no non-fallible handle operations, no cross-family sharing. This is the best M18 target. |

**Verified current shape:** `CreateWorkbook` currently wraps the raw builtin integer as `Workbook { Handle: XlsxCreateWorkbook() }`; all other public helpers pass `workbook.Handle` into the interpreter builtin. This is exactly the public shape M18 should preserve while transporting a first-class handle over Octxiliary.

### B. Image

| Item | Current state |
| --- | --- |
| Public record types | `record ImageHandle { Handle: Int }` in `Libraries/Image/Image.Core.oct`. |
| Public functions | `Load(path: String) -> ImageHandle ! Error`; `Save(image: ImageHandle, path: String) -> Int ! Error`; `Width(image: ImageHandle) -> Int<px>`; `Height(image: ImageHandle) -> Int<px>`; `Format(image: ImageHandle) -> String`. |
| Interpreted builtin path | The Oct wrapper calls `ImageLoad`, `ImageSave`, `ImageWidth`, `ImageHeight`, and `ImageFormat`. The interpreter registers those names in `imageWrapperBuiltins`; `evalImageLoadBuiltin` decodes an image file and stores `*wrapperImage` in `i.images`; save/metadata helpers look up the image handle in that store. |
| Tests | `Libraries/Image/Image.Core.octest` covers PNG/JPEG load, metadata, save/reload round trip, missing file, corrupt image, and unsupported save extension. `Libraries/Image/README.md` documents invalid image handles as an expected failure case. |
| Required handle behavior | Loaded images must return positive image handles tied to the Image sidecar. Save/metadata calls must reject stale or unknown image handles. Width/Height return `Int<px>` and Format returns `String`. |
| Compiled migration difficulty | Medium. There is still only one handle type, but codec behavior, metadata correctness, fixture paths, and non-fallible `Width`/`Height`/`Format` mean sidecar errors need a documented compiled runtime-panic path for non-fallible public calls. Image is useful, but should follow Xlsx after handle M0 is proven. |

**Important non-fallible API note:** `Width`, `Height`, and `Format` are non-fallible public helpers even though an invalid/stale handle can fail at runtime. M18 should not redesign this API. When later migrated, compiled non-fallible generic lowering should convert sidecar errors into runtime panics using the existing non-fallible wrapper behavior.

### C. Pdf

| Item | Current state |
| --- | --- |
| Public record types | `record PdfPage { Handle: Int }`; `record TextStyle { Size: Int<px>, ColorR: Int, ColorG: Int, ColorB: Int }`; `record ImageHandle { Handle: Int }` in `Libraries/Pdf/Pdf.Core.oct`. |
| Public functions | `NewPage(width: Int<px>, height: Int<px>) -> PdfPage ! Error`; `DrawText(page: PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`; `DrawTextStyled(page: PdfPage, x: Int<px>, y: Int<px>, text: String, style: TextStyle) -> Int ! Error`; `DrawImage(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error`; `DrawImageSized(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`; `Save(page: PdfPage, path: String) -> Int ! Error`; `DefaultTextStyle() -> TextStyle`. |
| Interpreted builtin path | The Oct wrapper calls `PdfNewPage`, `PdfDrawText`, `PdfDrawTextStyled`, `PdfDrawImage`, `PdfDrawImageSized`, and `PdfSave`. The interpreter registers those names in `pdfWrapperBuiltins`; page handles are stored in `i.pdfPages`; image drawing currently looks up the image ID in `i.images`, so the interpreted runtime shares one in-process image store between Image and Pdf builtins. |
| Tests | `Libraries/Pdf/Pdf.Core.octest` covers text PDF creation/save, image drawing, sized image drawing, styled text, invalid save path, invalid page handle, and invalid image handle. `Libraries/Pdf/README.md` documents the API and explicitly notes that `Pdf.ImageHandle` bridges to `ImageLoad` handle identity. |
| Required handle behavior | Page handles must be positive Pdf-sidecar `Pdf.PdfPage` capabilities. Image handles are the open design issue: current interpreted Pdf accepts an image handle identity produced by the Image builtin store, but M0 compiled Octxiliary should not share direct handles across sidecars. |
| Compiled migration difficulty | High. Pdf combines page handles, an image handle, `TextStyle` record arguments, pixel dimensions, image interop, font/backend behavior, and a larger sidecar surface. It should follow Image or get a dedicated Pdf design/migration pass. |

**Current API inconsistency to surface:** `Libraries/Pdf/README.md` says `Pdf.ImageHandle` bridges to the existing `ImageLoad` handle identity, and the interpreted implementation indeed looks up `DrawImage` image IDs in `i.images`. That is an interpreted in-process convenience, not a valid M0 compiled Octxiliary cross-sidecar policy. M18 must explicitly forbid using an `Image.ImageHandle` from an Image sidecar as a direct Pdf-sidecar handle. Pdf can preserve `Pdf.ImageHandle` as a separate type later, but its import mechanism needs a separate design.

## 3. Why handles are not plain `Int`

A public record field named `Handle: Int` is an API compatibility shell, not the transport truth. Treating handles as plain integers in Octxiliary would lose the properties that make them safe opaque capabilities:

- **Owner sidecar is lost.** The integer `1` could mean the first workbook in Xlsx, the first image in Image, or the first page in Pdf.
- **Handle type is lost.** `IO.Workbook`, `Image.ImageHandle`, `Pdf.PdfPage`, and `Pdf.ImageHandle` are not interchangeable even if their public record field has the same integer type.
- **Cross-family misuse cannot be detected early.** Passing an Image-created ID to Pdf or Xlsx can look syntactically valid if the transport is only `Int`.
- **Stale/expired identities are indistinguishable from arbitrary numbers.** A sidecar restart can make `1` refer to nothing or to a new resource. The transport must preserve enough context for useful diagnostics.
- **Process-lifetime ownership is hidden.** Handles name sidecar-owned resources with sidecar-process lifetime, not durable data values.
- **Package/runtime diagnostics degrade.** The runtime cannot say “expected Xlsx IO.Workbook handle, got Image Image.ImageHandle” if only an integer crossed the wire.
- **Security posture is weaker.** A capability-like transport can reject missing family/type/invalid ID payloads before a sidecar accidentally treats a number as an authority.

## 4. Proposed handle wire shape

Add a protocol kind:

```go
const ValueHandle ValueKind = "Handle"
```

Extend `octxiliary.Value` with explicit handle payload fields:

```go
type Value struct {
    Kind ValueKind
    // existing scalar/list/record fields...
    HandleFamily string
    HandleType   string
    HandleID     int
}
```

Canonical text encoding:

```text
OctxiliaryValue { kind: "Handle" handleFamily: "Xlsx" handleType: "IO.Workbook" handleID: 1 }
```

Choose `handleFamily`, `handleType`, and `handleID` rather than shorter `family`, `type`, and `id` because:

1. they avoid confusion with request-level `family` and record `recordType` fields;
2. they make malformed-frame diagnostics precise;
3. they are grep-friendly in generated code and protocol tests;
4. they keep the handle payload visibly separate from host pointers or record payloads.

Protocol rules for M18:

- `HandleID` must be positive (`> 0`).
- `HandleFamily` must be non-empty.
- `HandleType` must be non-empty.
- The parser must reject missing handle payload fields.
- Validation must reject `HandleID <= 0` even if parsing succeeded.
- Validation should reject empty family/type in both requests and responses.
- No host pointer values are exposed on the wire.
- No cross-run or cross-process serialization guarantee exists.
- A handle value is not a record value; record reconstruction happens only in generated compiled code at the Oct boundary.

## 5. Manifest schema design

Extend `WrapperTransportType.Kind` to support `"handle"` in addition to the existing `"record"` kind:

```oct
WrapperTransportType {
    Name: "IO.Workbook"
    Kind: "handle"
    Fields: [
        WrapperTransportField { Name: "Handle" Type: "Int" }
    ]
}
```

M18 manifest rules:

- `WrapperTransportType.Kind == "record"` continues to mean the existing M16 declared record argument transport.
- `WrapperTransportType.Kind == "handle"` declares a public Oct record that is transported as `ValueHandle`, not as `ValueRecord`.
- Handle transport types must have exactly one field in M0.
- The field name must be exactly `Handle` in M0.
- The field type must be exactly `Int` in M0.
- `WrapperFunction.Args` may use declared handle types.
- `WrapperFunction.Return` may use declared handle types when `Kind == "handle"`.
- `WrapperFunction.Return` must still reject declared `Kind == "record"` types in M0; handle return reconstruction is a narrow special case, not general record returns.
- Declared handle type names should be fully qualified Oct type names, such as `IO.Workbook`, `Image.ImageHandle`, `Pdf.PdfPage`, and `Pdf.ImageHandle`.
- A wrapper function may use only transport types declared by the same wrapper metadata entry. This keeps cross-wrapper/cross-family use explicit and rejected by default.
- Existing record transport behavior must remain unchanged for Plot-style records.

## 6. Compiler lowering design

### Handle returns

For a handle-returning generic wrapper function:

1. The sidecar returns `octxiliary.Value{Kind: ValueHandle, HandleFamily: <family>, HandleType: <declared type>, HandleID: <positive id>}`.
2. The generated compiled call requests/validates `ValueHandle` as the expected kind.
3. Generated code validates:
   - response kind is `Handle`;
   - `HandleFamily` matches the wrapper family declared in the manifest;
   - `HandleType` matches the declared return type;
   - `HandleID > 0`.
4. Generated code reconstructs the public Oct record from the ID:
   - `IO_Workbook{Handle: __value.HandleID}`
   - `Image_ImageHandle{Handle: __value.HandleID}`
   - `Pdf_PdfPage{Handle: __value.HandleID}`

This is a narrow handle-return exception. It must not unlock arbitrary declared record returns.

### Handle arguments

For a handle argument:

1. Generated code reads the public record field, e.g. `workbook.Handle`.
2. It packs the argument as:

```go
octxiliary.Value{
    Kind:         octxiliary.ValueHandle,
    HandleFamily: "Xlsx",
    HandleType:   "IO.Workbook",
    HandleID:     workbook.Handle,
}
```

3. Generated code should reject or diagnose zero/negative IDs before the request when easy.
4. The sidecar must still validate the handle payload and table membership; compiler-side checks are diagnostics, not trust boundaries.

### Non-fallible handle operations

Image currently has non-fallible handle operations (`Width`, `Height`, `Format`). M17/M18 should not change their public signatures. For later Image migration, sidecar errors from non-fallible calls should become runtime panics through the existing generic non-fallible lowering path. Fallible APIs continue to propagate `Error` values normally.

## 7. Sidecar handle table design

M18 should use a reusable sidecar helper pattern. It can start package-local and become an internal sidecar helper later if several sidecars need it:

```go
type handleTable[T any] struct {
    next   int
    values map[int]T
}

func (t *handleTable[T]) add(v T) int
func (t *handleTable[T]) get(id int) (T, bool)
```

Rules:

- ID `0` is invalid.
- IDs are positive.
- IDs monotonically increase.
- IDs are not reused in M0.
- The sidecar owns all values.
- Sidecar process exit releases all values.
- Handles are not persisted.
- Handles cannot move across sidecars.
- There is no shared global handle broker.
- Host pointers are never exposed on the wire.
- Invalid/missing IDs return a sidecar error with the wrapper's existing error vocabulary, such as `InvalidHandle` where applicable.

The interpreter already has a `wrapperHandleStore[T]` with maps and invalid-handle errors. M18 sidecars should not blindly copy its current allocation behavior if reuse becomes possible through release; instead, M0 should prefer explicit monotonic `next` and no reuse.

## 8. Lifecycle model

M0 lifecycle:

- Handles live for the sidecar process lifetime.
- There is no explicit `Close` or destructor.
- Handles are copyable Oct values, but every copy names the same sidecar-owned resource.
- Resources are released when the sidecar exits.
- Invalid/stale handles produce sidecar errors.
- If a public function is non-fallible, the compiled runtime converts sidecar errors into runtime panics per existing non-fallible generic lowering behavior.
- Handles do not survive compiled binary restart.
- Handles do not survive sidecar restart.
- Handles are not serializable as durable data.

Future lifecycle options, explicitly not M18:

- `Close(handle) -> Int ! Error` for resource-heavy wrappers.
- Leak diagnostics at sidecar shutdown.
- Package-manager/runtime cleanup hooks.
- More precise ownership/borrowing rules if the language later needs them.

## 9. Cross-family / cross-sidecar policy

M0 rule: handles are valid only for the wrapper family and sidecar that created them.

Consequences:

- Passing `Image.ImageHandle` to `Pdf.DrawImage` as direct cross-sidecar sharing is not supported in M0.
- Pdf should preserve a separate `Pdf.ImageHandle` type if the public API keeps that shape, but that handle must be owned by the Pdf sidecar.
- Pdf image import should be designed as a separate operation, such as importing from path/bytes or creating a Pdf-sidecar image handle from file data.
- A shared global handle registry or broker is explicitly deferred.

Options for Pdf after Xlsx/Image:

1. **Keep Pdf image handles separate.** Add Pdf-sidecar image import helpers while retaining `Pdf.ImageHandle` as the public type.
2. **Add path/bytes import helpers.** Let Pdf draw from a path or byte payload without sharing Image sidecar handles.
3. **True cross-sidecar handle broker.** Defer until there is a strong reason; it expands lifetime, ownership, build/runtime, and permission scope.

## 10. First migration target recommendation

| Candidate | Pros | Cons | Recommendation |
| --- | --- | --- | --- |
| `IO.Xlsx` | One handle type (`IO.Workbook`); simple process-lifetime resource; scalar/string/float operations; existing invalid handle and save-path tests; no cross-family handles; no non-fallible handle operations. | Requires handle returns, which generic Octxiliary has intentionally deferred until now. | **Migrate first in M18.** This is the cleanest motivating case for handle M0. |
| `Image` | One handle type; useful standalone capability; good tests around codecs and metadata. | Load/save codec behavior and fixture paths are more operationally rich; metadata calls are non-fallible despite possible invalid-handle runtime failures; future Pdf interop pressure could tempt cross-family shortcuts. | Migrate after Xlsx once handle M0 is proven. |
| `Pdf` | High-value report/document output; already uses records and handles. | Multiple handle-shaped records, `TextStyle` record args, image interop, font/backend behavior, and current interpreted Image/Pdf shared-handle identity conflict with M0 cross-sidecar policy. | Do not migrate first. Handle in a later dedicated design/migration pass or after Image. |

Exact next milestone: **M18 — implement handle transport M0 and migrate IO.Xlsx.**

## 11. Third-party wrapper authoring implications

Handle transport should make future wrapper-package authoring more explicit and safer:

- Wrapper packages can declare handle types in manifests using `Kind: "handle"`.
- The package manager and registry can inspect native resource-bearing APIs instead of inferring them from `Int` fields.
- Registry output should include handle transport metadata unchanged, just as it already carries declared structured transport metadata.
- Wrapper functions can advertise handle args/returns to the compiler without broad dynamic typing.
- Sidecar build lifecycle remains future package-manager work; M18 should not build sidecars during install/sync.
- Native sidecars require explicit build/permission policy later.
- No automatic postinstall scripts should be introduced.
- Handle transport does not require dynamic `Any`, pointer serialization, or global runtime brokers.

This design keeps third-party ergonomics declarative: authors describe the public record shape and ownership family in the manifest, while the compiler/sidecar protocol enforces the capability payload at runtime.

## 12. Tests required for M18

If M18 migrates `IO.Xlsx`, add tests in these categories:

### Protocol tests

- Encode/parse a `Handle` value.
- Validate `HandleFamily` is required.
- Validate `HandleType` is required.
- Validate `HandleID > 0`.
- Reject missing/invalid handle payloads in requests and responses.

### Manifest validation tests

- Accept a valid handle type declaration.
- Reject handle declarations without exactly one field.
- Reject handle declarations whose only field is not `Handle`.
- Reject handle declarations whose field type is not `Int`.
- Allow handle args.
- Allow handle returns.
- Continue rejecting declared `record` returns.
- Reject undeclared handle type use.
- Reject cross-wrapper handle type use when not declared by the wrapper.

### Compiler fixture tests

- Sidecar returns handle; compiled code reconstructs the public record.
- Compiled code passes a handle record back as `ValueHandle`.
- Invalid handle argument returns a fallible error.
- Handle response family/type mismatches are diagnosed.
- Zero/negative handle ID is rejected or converted to a clear wrapper/runtime error.

### Sidecar tests

- Create workbook returns a positive handle.
- Add sheet works.
- Set string cell works.
- Set float cell works.
- Save writes a non-empty `.xlsx` file.
- Invalid workbook handle returns sidecar error.

### Compiled IO.Xlsx tests

- Existing interpreted workflow runs in compiled mode through `octxiliary-xlsx`.
- Missing `octxiliary-xlsx` produces a clear diagnostic.
- Existing invalid-handle and invalid-save-path facts still pass.

### Regression tests

- Existing Hash, Compression, Time, Text, Archive, Json, Csv, Plot, Markdown, and IO sidecar tests keep passing.
- Existing manifest/registry/planning tests keep passing and render handle transport metadata without executing sidecars.

## 13. Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Treating handles as `Int` accidentally | Add `ValueHandle`, manifest `Kind: "handle"`, compiler packing/unpacking, and protocol validation. Keep `Handle: Int` only as public record compatibility shell. |
| Stale handles after sidecar restart | Document sidecar-process lifetime; validate every handle use in sidecar tables; return errors for unknown IDs. |
| Non-fallible handle operations panic | Preserve current public APIs; document that non-fallible generic calls convert sidecar errors to runtime panics; prefer fallible APIs for new resource operations. |
| Cross-sidecar handle confusion | Require family/type in every handle payload; generated code checks expected family/type; M0 forbids cross-family handles. |
| Leaks without `Close` | Keep M0 process-lifetime simple; defer `Close` and leak diagnostics until actual pressure appears. |
| Registry misses handle metadata | Preserve `TransportTypes` in package-manager plans/registry and render `Kind: "handle"` exactly like declared records. |
| Third-party native code builds become implicit | Keep sidecar build lifecycle out of M18; no postinstall scripts, no permission prompts, no lockfile generation. |
| Temptation to use dynamic `Any` or host pointers | Use fixed handle fields; reject host pointer exposure and dynamic payloads. |
| Public API mismatch across IO.Xlsx/Image/Pdf and handle ABI | Keep public records unchanged, but require fully qualified declared type names and family/type checks in transport. Surface Pdf image interop as a follow-up design issue. |

## 14. Final recommendation

The exact next implementation milestone should be: **M18 — implement handle transport M0 and migrate IO.Xlsx**.

Exact M18 non-goals:

- no Image migration;
- no Pdf migration;
- no cross-family handles;
- no `Close`/explicit destructor semantics;
- no handle serialization across program runs;
- no package-manager native build system;
- no dynamic `Any`;
- no shared global handle broker;
- no general record returns beyond the handle-return special case;
- no public API redesign unless an implementation blocker is found and documented.

Rationale: `IO.Xlsx` is the smallest real handle-backed standard-library blocker and exercises the essential missing mechanics: first-class handle wire values, manifest-declared handle types, handle argument packing, handle return reconstruction, positive-ID validation, sidecar table ownership, and process-lifetime semantics. Starting with Xlsx proves the capability model without image codec complexity or Pdf cross-family interop. Image should follow once M0 is stable; Pdf should follow Image or receive its own design/migration pass because page handles, text style records, and image interop need separate policy decisions.

## M18 update

M18 implements the recommended M0 handle transport and migrates `IO.Xlsx` first. `IO.Workbook` is declared as a manifest `handle` transport type with the public `Handle: Int` field, while the wire value carries `handleFamily`, `handleType`, and positive sidecar-local `handleID` fields. The lifecycle remains sidecar-process lifetime only: no Close/destructor, no cross-family handles, no serialization across runs, and no Image/Pdf migration were added.

## M19 Image realization note

M19 applies the M17/M18 handle model to `Libraries/Image`. `Image.ImageHandle` is declared as a manifest handle transport with exactly `Handle: Int`, while the wire payload remains a typed `Handle` with family `Image`, type `Image.ImageHandle`, and a positive sidecar-local ID. The public `ImageHandle` record is reconstructed only at the Oct boundary; the sidecar never treats it as a plain integer handle without family/type validation.

`cmd/octxiliary-image` owns decoded images and format metadata for the sidecar process lifetime. It supports PNG/JPEG load, PNG/JPEG save by extension, width/height integer payloads for public `Int<px>` returns, and format strings. Invalid handles produce sidecar errors; for non-fallible `Width`, `Height`, and `Format`, compiled lowering converts those sidecar errors into runtime failures. M19 deliberately leaves Pdf unmigrated and does not introduce cross-family Image/Pdf handle sharing or Close/destructor semantics.
