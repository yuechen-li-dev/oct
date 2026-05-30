# Octxiliary M15 Plot Transport Design Audit

## 1. Executive summary

**Answer to the primary question:** yes. `Libraries/Plot` can be migrated through generic Octxiliary without handles, dynamic `Any`, structured JSON graph transport, or broad nested-array generalization. The inspected public Plot surface is stateless: every public render helper receives numeric data, an output path, a small `Size` record, and a small `Labels` record, then writes a PNG artifact and returns an `Int ! Error`. No public Plot API stores, mutates, reuses, or returns an opaque plotting resource.

**Recommended transport additions:**

1. `Float[]`, represented as `[]float64` in Go and encoded as a first-class Octxiliary value kind.
2. Manifest-declared record **arguments** for small, fixed, non-recursive option records. Plot needs `Plot.Size` and `Plot.Labels`; M16 should not add record returns unless a concrete migrated wrapper requires them.

**Functions that become migratable:**

- `Plot.Line(Float[], Float[], String, Size, Labels) -> Int ! Error`
- `Plot.Scatter(Float[], Float[], String, Size, Labels) -> Int ! Error`
- `Plot.Histogram(Float[], Int, String, Size, Labels) -> Int ! Error`

`DefaultSize() -> Size` and `DefaultLabels() -> Labels` are pure Oct helpers and do not need sidecar transport.

**What remains deferred:** handles for Image/Pdf/XLSX-style resources, dynamic `Any`, structured JSON graph values, arbitrary maps, recursive records, record returns, nested record fields, `Float[][]`, broad generated-Go numeric/type hardening, sidecar build orchestration, native permission prompts, lockfiles, Markdown wrapperization, and Image/Pdf/Xlsx migration.

**M16 recommendation:** **Option 1: implement `Float[]` + record-argument transport + Plot migration**. Plot is small enough and visible enough to justify landing the transport with its motivating package, provided M16 keeps the scope to record arguments and the three Plot render helpers.

## 2. Current Plot API inventory

The public Plot implementation is in `Libraries/Plot/Plot.Core.oct`. The manifest is currently a pure package manifest with no wrapper metadata; the package depends on `OctStd` only.

| Public helper | Signature | Interpreted implementation path / builtin | Tests using it | Required transport shapes | Fits `Float[]` + declared records? |
| --- | --- | --- | --- | --- | --- |
| `Line` | `Line(x: Float[], y: Float[], outputPath: String, size: Size, labels: Labels) -> Int ! Error` | Calls `PlotRenderLine(x, y, outputPath, size.Width, size.Height, labels.Title, labels.X, labels.Y, labels.Legend)?`; interpreted handler is `evalPlotRenderLineBuiltin`, which delegates through `evalPlotRenderXYBuiltin` and `renderPlot`. | `LinePlotWritesConfiguredPng`, `InvalidOutputPathFails`, and indirectly `DefaultSize`/`DefaultLabels` in error-path tests. | `Float[]`, `Float[]`, `String`, `Size`, `Labels`, return `Int ! Error`. The current backend builtin is scalarized to 9 arguments internally. | Yes. No handle required; `Size` and `Labels` are fixed records. |
| `Scatter` | `Scatter(x: Float[], y: Float[], outputPath: String, size: Size, labels: Labels) -> Int ! Error` | Calls `PlotRenderScatter(x, y, outputPath, size.Width, size.Height, labels.Title, labels.X, labels.Y, labels.Legend)?`; interpreted handler is `evalPlotRenderScatterBuiltin`, which delegates through `evalPlotRenderXYBuiltin` and `renderPlot`. | `ScatterPlotWritesConfiguredPng`, `MismatchedDataLengthsFail`. | `Float[]`, `Float[]`, `String`, `Size`, `Labels`, return `Int ! Error`. | Yes. Stateless render-to-file helper. |
| `Histogram` | `Histogram(values: Float[], bins: Int, outputPath: String, size: Size, labels: Labels) -> Int ! Error` | Calls `PlotRenderHistogram(values, bins, outputPath, size.Width, size.Height, labels.Title, labels.X, labels.Y, labels.Legend)?`; interpreted handler is `evalPlotRenderHistogramBuiltin`, which delegates to `renderPlot`. | `HistogramWritesConfiguredPng`, `InvalidHistogramBinsFail`. | `Float[]`, `Int`, `String`, `Size`, `Labels`, return `Int ! Error`. | Yes. Stateless render-to-file helper. |
| `DefaultSize` | `DefaultSize() -> Size` | Pure Oct record literal: `Size { Width: 640px Height: 480px }`. | `ScatterPlotWritesConfiguredPng`, `InvalidOutputPathFails`, `MismatchedDataLengthsFail`, `InvalidHistogramBinsFail`. | No sidecar transport if called locally; generated Go already has record construction support. Its result can become an argument to transported Plot functions. | Yes, as a local record producer feeding record-argument packing. |
| `DefaultLabels` | `DefaultLabels() -> Labels` | Pure Oct record literal with title/legend empty and axes `x`/`y`. | `InvalidOutputPathFails`. | No sidecar transport if called locally; generated Go already has record construction support. Its result can become an argument to transported Plot functions. | Yes, as a local record producer feeding record-argument packing. |

Record inventory:

- `Size { Width: Int<px>, Height: Int<px> }`
- `Labels { Title: String, X: String, Y: String, Legend: String }`

Test inventory:

- Three positive tests assert that configured line/scatter/histogram calls create PNG files and then delete them.
- Three negative tests assert invalid output extension, mismatched x/y lengths, and non-positive histogram bins produce errors.

The README confirms the same MVP surface and states that output paths must end in `.png`, line/scatter require equal-length x/y arrays, and histogram bins must be positive.

## 3. Current compiled blockers

Plot failed in the M5g compiled inventory as `unsupported_builtin` / `unsupported_wrapper` for `PlotRenderLine`, `PlotRenderScatter`, and `PlotRenderHistogram`. The M11 wrapper sweep continued to list Plot as blocked by `Float[]` plot data and record arguments.

The concrete blockers are:

- **No compiled lowering for Plot render builtins.** The compiler builtin support path does not include `PlotRenderLine`, `PlotRenderScatter`, or `PlotRenderHistogram`; older compiler tests still expect plot builtins to be unsupported in compiled mode.
- **`Float[]` is not an Octxiliary transport kind.** Current generic transport supports scalar `Float`, not numeric arrays.
- **Records are not transportable across the sidecar boundary.** `Size` and `Labels` compile locally as Go structs, and field access compiles locally, but the generic wrapper metadata and protocol do not describe record shapes or carry record payloads.
- **Artifact/file behavior belongs to the sidecar operation.** The interpreted backend uses `attributedOutputPath`, requires `.png`, validates dimensions/data, and writes via `gonum/plot`. A Plot sidecar must preserve the render-to-file contract and return operational errors rather than panicking.

## 4. Local records vs sidecar record transport

Oct records already compile naturally to Go structs in generated Go. Record literals become Go composite literals, field access becomes Go field access, and record updates are represented in the compiler's local lowering path. That is enough for in-process helpers.

M14 Markdown took advantage of that in-process model. Markdown table helpers can inspect generated-Go record values directly because the implementation lives inside the generated program and does not cross a process boundary. No sidecar ABI has to describe the record.

Plot is different if migrated through generic Octxiliary. Sidecars are separate processes connected by the compact Octxiliary frame protocol. A generated Go struct for `Plot_Size` cannot be reflected inside `octxiliary-plot`; the only shared contract is the manifest metadata plus encoded protocol values. Therefore record transport must be explicit ABI metadata:

- record type name,
- declared field names,
- declared field types,
- deterministic field order,
- validation rules for missing, unknown, duplicate, or unsupported fields.

Do not rely on Go reflection, generated struct names, or implicit field layout across the sidecar boundary.

## 5. Candidate approaches for Plot

### A. Scalarize Plot records in Oct stubs before sidecar calls

Example public shape remains:

```oct
Plot.Line(xs, ys, path, size, labels)
```

but the compiled sidecar wire function receives:

```text
Float[], Float[], String, Int, Int, String, String, String, String
```

for `Line`/`Scatter`, and:

```text
Float[], Int, String, Int, Int, String, String, String, String
```

for `Histogram`.

**Pros:**

- Avoids record transport for a first Plot migration.
- Keeps the immediate sidecar ABI to scalars plus `Float[]`.
- Matches the current interpreted backend builtins, which already scalarize `Size` and `Labels` fields before calling `PlotRender*`.
- Simpler sidecar argument validation for M0 Plot.

**Cons:**

- Special-cases Plot's current option shape rather than adding a reusable typed lane.
- Wrapper manifest metadata would not reflect the public signature cleanly if generic wrapper lowering continues to match public calls to manifest functions.
- It could require private wrapper functions, compiler-recognized stubs, or an API reshaping layer, all of which bend the M6 signature-agreement model.
- It creates pressure to repeat scalarization for every future small option record.

**Assessment:** viable only if M16 intentionally chooses a Plot-specific shortcut. It is not the best convergence path because declared records solve the real blocker without adding `Any` or handles.

### B. Declared record transport

Example manifest-level ABI declaration:

```oct
WrapperTransportType {
    Name: "Plot.Size"
    Kind: "record"
    Fields: [
        WrapperTransportField { Name: "Width" Type: "Int<px>" },
        WrapperTransportField { Name: "Height" Type: "Int<px>" }
    ]
}
```

Public wrapper signatures can stay close to the API:

```text
Line(Float[], Float[], String, Plot.Size, Plot.Labels) -> Int ! Error
```

**Pros:**

- Preserves the public API and generic wrapper signature agreement model.
- Reusable for other small, fixed option records.
- Keeps the sidecar ABI typed and inspectable.
- Avoids dynamic `Any`, maps, JSON blobs, handles, and broad nested arrays.

**Cons:**

- Needs manifest schema extension and package-manager validation.
- Needs protocol encoding/decoding for records.
- Needs compiler packing of record arguments from generated Go structs.
- Needs sidecar field validation.

**Assessment:** recommended. This is the smallest general typed lane that matches Plot and likely future option records.

### C. Direct compiled Plot helpers without sidecar

**Pros:**

- No Octxiliary protocol change.
- Could reuse Go dependencies directly in generated Go or implement simple SVG/text output if the tests only checked file existence.

**Cons:**

- The current interpreted implementation uses `gonum/plot` and writes PNG files. Inlining that into every generated binary would enlarge generated helpers and introduce backend details into compiled output.
- It does not generalize to future wrapper families.
- It bypasses the wrapper migration strategy established by M6-M13 for host-backed standard-library edges.
- If a simple SVG/text renderer were substituted, it would not preserve the `.png` contract documented and tested today.

**Assessment:** not recommended for Plot M16. Direct compiled helpers were appropriate for deterministic Markdown text construction; Plot is an external rendering operation.

### D. Defer Plot until handles/records/numeric arrays all exist

**Pros:**

- Avoids partial ABI expansion.
- Reduces immediate implementation risk.

**Cons:**

- Delays high-user-visible plotting support.
- Leaves compiled report/artifact workflows incomplete even though Plot itself does not require handles.
- Couples a stateless render-to-file package to the heavier lifecycle design needed for Image/Pdf/XLSX.

**Assessment:** not recommended. Plot can converge independently with `Float[]` and record arguments.

## 6. Proposed declared record transport ABI

### Manifest schema sketch

Add two manifest records for wrapper packages:

```oct
record WrapperTransportType {
    Name: String
    Kind: String
    Fields: WrapperTransportField[]
}

record WrapperTransportField {
    Name: String
    Type: String
}
```

Add transport type declarations **inside `Wrapper`**:

```oct
record Wrapper {
    Name: String
    Family: String
    Protocol: String
    SidecarCommand: String
    GoModuleDir: String
    TransportTypes: WrapperTransportType[]
    Functions: WrapperFunction[]
}
```

Wrapper-level placement is recommended for M16 because transport record types are sidecar-family ABI. They are not yet general package dependency metadata, and keeping them under `Wrapper` makes the sidecar ownership clear. Package-level declarations could become useful later if several wrapper families in one package share record types, but that is not needed for Plot and would blur the first implementation.

### Naming policy

Use fully qualified type strings in `WrapperFunction.Args`:

- `"Plot.Size"`
- `"Plot.Labels"`

and require `WrapperTransportType.Name` to match the same fully qualified string.

Rationale:

- Avoids collisions with other packages' `Size` or `Labels` records.
- Keeps the protocol type tag unambiguous.
- Aligns with existing compiler behavior where package-qualified record types are already represented by dotted names and made Go-safe separately.
- Avoids ad hoc prefixes such as `record:Size`, which would create a second type-string grammar.

Require unique `TransportTypes.Name` values within a wrapper. For M16, a wrapper function may refer only to built-in transport strings plus declared transport type names in the same wrapper metadata. Undeclared custom type strings should be rejected.

### Plot manifest sketch for M16

```oct
Wrapper {
    Name: "plot"
    Family: "Plot"
    Protocol: "octxiliary.v0"
    SidecarCommand: "octxiliary-plot"
    GoModuleDir: "octxiliary"
    TransportTypes: [
        WrapperTransportType {
            Name: "Plot.Size"
            Kind: "record"
            Fields: [
                WrapperTransportField { Name: "Width" Type: "Int<px>" },
                WrapperTransportField { Name: "Height" Type: "Int<px>" }
            ]
        },
        WrapperTransportType {
            Name: "Plot.Labels"
            Kind: "record"
            Fields: [
                WrapperTransportField { Name: "Title" Type: "String" },
                WrapperTransportField { Name: "X" Type: "String" },
                WrapperTransportField { Name: "Y" Type: "String" },
                WrapperTransportField { Name: "Legend" Type: "String" }
            ]
        }
    ]
    Functions: [
        WrapperFunction { OctName: "Line" WireName: "PlotRenderLine" Args: ["Float[]", "Float[]", "String", "Plot.Size", "Plot.Labels"] Return: "Int" Fallible: true },
        WrapperFunction { OctName: "Scatter" WireName: "PlotRenderScatter" Args: ["Float[]", "Float[]", "String", "Plot.Size", "Plot.Labels"] Return: "Int" Fallible: true },
        WrapperFunction { OctName: "Histogram" WireName: "PlotRenderHistogram" Args: ["Float[]", "Int", "String", "Plot.Size", "Plot.Labels"] Return: "Int" Fallible: true }
    ]
}
```

For dimensioned fields, record field metadata should either preserve `Int<px>` exactly or normalize it to `Int` plus a dimension tag. For M16 Plot, preserving the type string is safer because `Size.Width` and `Size.Height` are explicitly `Int<px>` in the public API and the interpreted path validates pixel dimensions.

## 7. Protocol design for record values

Current Octxiliary values are compact textual envelopes such as:

```text
OctxiliaryValue { kind: "String" string: "hello" }
```

Keep the same one-line style. Add a record kind with an explicit type tag and ordered fields:

```text
OctxiliaryValue { kind: "Record" recordType: "Plot.Size" fields: [
  OctxiliaryField { name: "Width" value: OctxiliaryValue { kind: "Int" int: 800 } }
  OctxiliaryField { name: "Height" value: OctxiliaryValue { kind: "Int" int: 600 } }
] }
```

The actual encoder can emit this on one line; the layout above is illustrative.

Requirements:

- Preserve declared field order when encoding.
- Parse in order but validate by declared field names.
- Reject unknown fields.
- Reject missing fields.
- Reject duplicate fields.
- Reject unsupported field transport types.
- Reject record type names not declared in manifest metadata for the wrapper family.
- Reject recursive records.
- Reject maps and arbitrary dynamic fields.
- No `Any`.
- No handles.
- No arbitrary nested records in M16.

Allowed field types for M16 should be deliberately narrow:

- Scalars: `Int`, dimensioned `Int<...>` where the source Oct field has a dimension, `Float`, `Bool`, `String`.
- Existing buffers/lists only if a real first user appears: `String[]`, `String[][]`, `Bytes`.
- `Float[]` once added.

For Plot specifically, only `Int<px>` and `String` fields are required. Record fields that are other records should be deferred. Record return values should be deferred unless a concrete wrapper requires them.

## 8. Protocol design for `Float[]`

Add a new `ValueKind` string:

```text
"Float[]"
```

Go payload:

```go
[]float64
```

Encoding shape:

```text
OctxiliaryValue { kind: "Float[]" floats: [ 1 2.5 -3 ] }
```

Formatting:

- Use `strconv.FormatFloat(v, 'g', -1, 64)` for each element, matching scalar `Float` style.
- Parse with `strconv.ParseFloat(..., 64)`.
- Empty arrays encode as `floats: [  ]` or the existing list convention's equivalent empty payload; parser should round-trip empty slices even though Plot will reject empty data operationally.

NaN/Inf policy:

- **Reject NaN and +/-Inf in M16** when encoding and parsing `Float[]` unless the language specification explicitly requires transporting them.
- Rationale: text protocol portability and downstream rendering semantics are ambiguous, while Plot data should be finite for deterministic artifacts.

Parser behavior:

- Malformed float tokens produce protocol parse errors.
- Non-finite values produce protocol validation errors before sidecar execution.
- Plot-specific checks such as non-empty x/y, equal line/scatter lengths, positive bins, positive pixel sizes, and `.png` path remain sidecar operational validation.

`Float[][]` is not necessary for current Plot. Do not add it in M16.

## 9. Compiler lowering design

### `Float[]`

M16 needs these compiler/protocol touchpoints:

- Add `Float[]` to wrapper manifest type-string validation.
- Add `Float[]` to compiler `isOctxiliaryTransportType`.
- Add `octxiliary.ValueFloatArray` or equivalent value kind constant.
- Add `[]float64` payload storage to `octxiliary.Value`.
- Add `octxiliaryValueExpr("Float[]", expr)` packing.
- Add `octxiliaryValueExtractExpr("Float[]", value)` extraction if a wrapper ever returns `Float[]`; Plot needs argument packing only, but symmetric extraction is cheap and keeps the transport complete.
- Confirm `goType("Float[]")` already maps to `[]float64` through the generic array suffix path.
- Confirm `goSafeName("Float[]")` produces stable result/helper names such as `FloatSlice`.

### Records

The compiler must know record field types and declared order from wrapper metadata. When lowering a generic wrapper call:

1. If an argument transport type is a declared transport record, look up the declaration by fully qualified type name.
2. Confirm the Oct argument type corresponds to that record type.
3. Emit an `octxiliary.Value` with `kind: Record`, `recordType`, and fields in manifest-declared order.
4. For each field, read the generated Go struct field and pack it using the field transport type.
5. Use the same scalar/dimension/list validation rules that the manifest accepted.

For M16, support **record arguments only**. Plot does not require record returns; `DefaultSize` and `DefaultLabels` are local pure Oct helpers. Deferring record returns avoids early decisions about extracting protocol records into generated Go structs, record update aliasing, and cross-package type identity.

If record returns are added later, they should be a separate milestone with extraction into the correct generated Go struct type, exact field validation, and tests for missing/extra/duplicate fields.

## 10. Sidecar validation design

`octxiliary-plot` should never trust generated clients. It should validate:

- request family is exactly `Plot`;
- function is one of `PlotRenderLine`, `PlotRenderScatter`, `PlotRenderHistogram`;
- argument count matches the manifest signature;
- argument kinds match expected values;
- `Float[]` payloads contain finite numbers;
- record type tags are exactly `Plot.Size` and `Plot.Labels` where expected;
- record fields are present once, in the declared set, and have the expected kinds;
- `Size.Width` and `Size.Height` are positive pixel integers;
- output path ends in `.png`;
- line/scatter data arrays are non-empty and equal length;
- histogram values are non-empty and bins are positive.

Operational failures should return sidecar errors (`ok: false`) or fallible wrapper errors as appropriate for the generic call path. Malformed protocol payloads should be handled as validation errors, never panics.

## 11. Plot sidecar feasibility

The current interpreted renderer uses `gonum.org/v1/plot`, `plotter`, and `vg`; it writes PNG output via `p.Save(width, height, filepath.Clean(outputPath))`. It already implements line, scatter, and histogram in one shared `renderPlot` path. Convenience builtins use default dimensions and labels; Plot.Core wrappers supply size and labels through scalarized fields.

Feasible M16 implementation options:

1. **Reuse the existing rendering logic in a sidecar package.** Move or duplicate the small renderer into a sidecar command using the existing module dependency. This best preserves current PNG behavior.
2. **Factor shared Plot rendering into an internal package** used by both interpreter and sidecar. This avoids duplication but touches production interpreter organization, so M16 should do it only if the refactor remains small and well-tested.
3. **Write a minimal SVG/text renderer.** Not recommended: current docs/tests say `.png`, and interpreted behavior uses `gonum/plot` PNG output.

No new external Go dependency appears necessary because the repo already uses `gonum/plot` for interpreted Plot. The current Plot tests check file existence and error behavior, not golden image pixels, so a sidecar that preserves PNG creation and validations should be enough. Avoid golden pixel tests in M16; PNG rendering can vary with backend/library details.

Artifact behavior note: the interpreter calls `attributedOutputPath` before saving. The sidecar migration must preserve the observable output-path/artifact behavior expected by `oct test` and report workflows. If attribution logic is not available to a sidecar, M16 should explicitly test the real `oct test Libraries/Plot --compiled` path and document any path-attribution gap rather than silently changing artifact placement.

## 12. Recommended M16 scope

Choose exactly one: **Option 1 — M16 implements `Float[]` + record-argument transport + Plot migration.**

Rationale:

- Plot is stateless and does not need handles.
- The public API is small: three sidecar-backed render helpers plus two local defaults.
- `Float[]` is a narrow numeric lane, not a broad nested-array generalization.
- Declared record arguments are a bounded ABI addition that preserves the public API and avoids scalarization hacks.
- Landing transport with Plot tests proves the real motivating case, not only a fixture.

M16 implementation should remain constrained:

- Add only `Float[]`, not `Float[][]`.
- Add declared non-recursive record **arguments**, not record returns.
- Add `cmd/octxiliary-plot` only for `Plot` family functions.
- Migrate only `Plot.Line`, `Plot.Scatter`, and `Plot.Histogram` through wrapper metadata.
- Keep `DefaultSize` and `DefaultLabels` pure/local.
- Do not migrate Image, Pdf, Xlsx, structured JSON, or Markdown.

## 13. Tests required for M16/M17

If M16 follows the recommendation, required tests include:

- Protocol tests for `Float[]` encode/decode, including empty arrays, representative finite floats, malformed tokens, and NaN/Inf rejection.
- Protocol tests for record argument payload parsing/encoding, including exact field order and type tag preservation.
- Manifest/package-manager validation tests for declared transport records: unknown type, duplicate record name, duplicate field, missing field, unsupported field type, undeclared `WrapperFunction.Args` type.
- Compiler generic fixture for `Float[]` arguments.
- Compiler generic fixture for record arguments into a small test sidecar.
- Invalid record field/missing/duplicate tests at sidecar validation level.
- Plot sidecar fixture validation for family/function/argument count/kinds.
- Compiled Plot tests for `Line`, `Scatter`, and `Histogram` positive paths.
- Compiled Plot tests for invalid extension, mismatched line/scatter lengths, non-positive bins, and non-positive pixel sizes.
- Missing `octxiliary-plot` diagnostic coverage analogous to existing missing sidecar diagnostics.
- Regression tests for existing wrapper packages: Hash, Compression, Time, Text, Archive, Json, Csv, IO wrapper paths.
- Existing Markdown compiled tests to ensure direct Markdown support remains independent of Octxiliary records.

For M15 itself, only documentation changes are made; production behavior should not change.

## 14. Risk map

| Risk | Mitigation |
| --- | --- |
| Record ABI becomes dynamic JSON | Require manifest-declared record names and fields; reject unknown/missing/duplicate fields; no maps; no `Any`. |
| Field ordering drift | Preserve manifest-declared field order for encoding and tests; sidecar validates by name and can also enforce order if desired. |
| Record type name collision | Use fully qualified names such as `Plot.Size` in declarations and function signatures. |
| Go struct aliasing/immutability confusion | M16 supports record arguments only; no record returns or mutable sidecar-owned records. |
| NaN/Inf float encoding ambiguity | Reject non-finite values in `Float[]` protocol for M16. |
| Plot rendering golden tests brittle | Test file existence, error behavior, and possibly PNG signature only; avoid pixel goldens. |
| External dependency temptation | Reuse existing `gonum/plot`; do not add new renderer dependencies unless a concrete blocker appears. |
| Scope creep into handles/Image/Pdf | Explicitly defer handles and lifecycle semantics. |
| Compiler type-string mismatch | Use fully qualified transport record names and require manifest declarations to match wrapper function type strings exactly. |
| Result naming mismatch for `Float[]` | Confirm `goSafeName("Float[]")` and `goResultTypeName` produce stable helper/result names before adding fixtures. |
| Dimension loss for `Int<px>` fields | Preserve or explicitly model dimensioned field type strings in manifest validation; do not silently normalize away `px` if sidecar validation depends on it. |
| Artifact path attribution changes | Test real compiled Plot artifact paths; factor shared attribution behavior if needed. |

## 15. Final recommendation

**Recommended next implementation milestone:** M16 should implement `Float[]` transport, manifest-declared non-recursive record **argument** transport, and migrate `Libraries/Plot` render helpers (`Line`, `Scatter`, `Histogram`) to a dedicated `octxiliary-plot` sidecar.

**M16 non-goals:** record returns, nested records, recursive records, maps, dynamic `Any`, handles, `Float[][]`, structured JSON graph transport, Image migration, Pdf migration, Xlsx migration, Markdown wrapperization, package-manager sidecar builds, native permission prompts, lockfiles, broad generated-Go hardening, and unrelated numeric/type bug fixes.

**Expected packages/functions unlocked:** `Libraries/Plot.Line`, `Libraries/Plot.Scatter`, and `Libraries/Plot.Histogram` in compiled mode, while `DefaultSize` and `DefaultLabels` remain local pure Oct helpers.

**Deferred blockers:** lifecycle/ownership semantics for handles, structured graph typing for JSON-like values, nested numeric matrices, and broader compiler-generated Go robustness outside the focused Plot transport path.
