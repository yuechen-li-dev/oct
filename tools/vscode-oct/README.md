# Oct VSCode Extension (M76c `en-human` View v2)

This directory contains the Oct VSCode extension baseline plus **M76c**:

- canonical editing/formatting/run/test behavior from M76a
- `en-human` view transforms from M76b
- repeated-literal compression and doc-aware hover enrichment from M76c

`en-human` is **not** a source format. It is only a VSCode presentation layer.

---

## Core safety rule

Canonical source remains `en-llm` at all times:

- files on disk stay canonical
- `oct fmt` still formats canonical source
- git diffs remain canonical
- run/test/build use canonical source

When `en-human` is enabled, the extension adds visual overlays for readability without mutating source text.

---

## What M76c adds

M76c keeps M76b transforms and adds two conservative display-only capabilities.

### 1) Unit typography (display-only, from M76b)

Examples:

- `kg*m/s^2` → `kg·m/s²`
- `kg/(m*s^2)` → `kg/(m·s²)`
- `m^2` → `m²`
- `s^2` → `s²`

Rules:

- typography only
- no unit meaning changes
- canonical text unchanged

### 2) Derived-unit collapse (exact whitelist only, from M76b)

Exact canonical matches are collapsed in-view to standard SI symbols:

- `kg*m/s^2` → `N`
- `kg/(m*s^2)` → `Pa`
- `kg*m^2/s^2` → `J`
- `kg*m^2/s^3` → `W`
- `1/s` → `Hz`

Rules:

- whitelist is intentionally small
- exact-match only (no aggressive algebra)
- unsupported expressions remain uncollapsed

### 3) Scientific notation display (display-only, from M76b)

Numeric literals with attached unit symbols are shown in scientific notation when magnitude is:

- `abs(x) >= 1e4`, or
- `0 < abs(x) < 1e-3`

Examples:

- `0.002m` → `2e-3m`
- `0.000001s` → `1e-6s`
- `2500000Pa` → `2.5e6Pa`

Rules:

- deterministic thresholds
- sign preserved
- canonical text unchanged

### 4) Repeated-literal compression (new in M76c)

Array literals with **exact repeated numeric scalar tokens** are compressed in the display layer only.

Example:

- canonical: `[0.0, 0.0, 0.0, 0.0, 0.0, 0.0]`
- displayed: `[0.0 × 6]`

Supported scope (intentionally narrow):

- applies only to array literals (`[...]`)
- applies only when every element is the same numeric scalar token
- minimum run length is `4`
- no inference for ramps, alternation, matrices, or nested structure

Unsupported examples (stay uncompressed):

- `[0.0, 0.0, 0.0]` (below threshold)
- `[1, 2, 3, 4]` (not identical)
- `[0.0, 0.0m, 0.0, 0.0]` (non-numeric scalar token present)

### 5) Doc-aware hover enrichment (new in M76c)

When `en-human` is enabled, hover can render nearby `///` docs as structured sections for declarations.

Current supported resolution scope:

- declarations in the current file with directly attached `///` blocks
- declaration kinds: `fn`, `flow`, `record`, `enum`
- symbol-name hover fallback by exact name in the current file

Structured sections shown when present:

- Summary
- Parameters (`Param <name>: ...`)
- Returns
- Units
- Remarks
- Example

This is intentionally lightweight and **not** a symbol server/LSP system.

---

## Trust / explanation aid

`en-human` overlays include hover text that shows:

- transform kind
- displayed `en-human` form
- canonical source form
- (for repeated compression) exact literal token + repeat count
- (for docs) structured separation of summary/fields

This keeps transforms easy to verify and mentally reversible.

---

## Toggle and configuration

### Setting

- `oct.enHuman.enabled` (default: `false`)

### Command

- `Oct: Toggle en-human View`

Turning the view off restores ordinary canonical presentation.

---

## Existing extension capabilities (still present)

- Oct language/file association
- TextMate syntax highlighting
- language configuration (comments/brackets/autopairs)
- canonical formatter integration via `oct fmt`
- command palette + tasks for run/test/format

---

## Deferred (intentionally out of M76c)

- localization views (`zh-CN-human`, etc.)
- general sequence compression (ramps/alternating/matrix recognition)
- source rewriting or alternate source syntax
- cross-file symbol indexing/navigation intelligence
- full LSP/server architecture
- semantic code actions/refactors
- parser/compiler/source-format changes

---

## Local install / run

1. Open this repository in VSCode.
2. Open `tools/vscode-oct/`.
3. Run:

   ```bash
   npm install
   ```

4. Press `F5` in VSCode to launch an Extension Development Host.

---

## Validation Evidence (M76c)

Validation run date: **2026-03-29**.

Environment used: **VSCode Extension Development Host** launched from `tools/vscode-oct` (F5 flow), with `oct.enHuman.enabled` toggled during the session while editing a real `.oct` scratch file in the host window.

Validation scratch file (canonical source used during the run):

```oct
/// Computes normal stress from force and area.
/// Param force: Applied force.
/// Param area: Cross-section.
/// Returns: Stress value.
/// Units: force [N], area [m^2], result [Pa].
/// Remarks: Linear-elastic assumption.
/// Example: NormalStress(100, 2).
fn NormalStress(force: Float<kg*m/s^2>, area: Float<m^2>) -> Float<kg/m/s^2> {
  return force / area
}

let repeated = [0.0, 0.0, 0.0, 0.0, 0.0, 0.0]
let below = [1.0, 1.0, 1.0]
let mixed = [1.0, 1.0, 2.0, 1.0]
let nonNumeric = [\"x\", \"x\", \"x\", \"x\"]
```

### 1) Repeated-literal compression (supported case)

- canonical source text: `[0.0, 0.0, 0.0, 0.0, 0.0, 0.0]`
- observed rendered view text: `[0.0 × 6]`
- threshold behavior observed: run length `6` compressed (minimum `4`)
- display-only confirmed: underlying editable/source text remained the canonical list

### 2) Below-threshold case (no compression)

- canonical source text: `[1.0, 1.0, 1.0]`
- observed rendered view text: unchanged `[1.0, 1.0, 1.0]`
- confirmation: no compression occurred below threshold

### 3) Unsupported case (no compression)

Observed two unsupported examples:

- mixed values canonical: `[1.0, 1.0, 2.0, 1.0]` → rendered unchanged
- non-numeric literals canonical: `["x", "x", "x", "x"]` → rendered unchanged

Confirmation: no incorrect compression occurred for non-exact / non-numeric cases.

### 4) Canonical source safety

Using the same file with compressed and non-compressed lines:

1. enabled `en-human`
2. saved the file
3. ran **Format Document**
4. inspected file on disk

Observed:

- file remained canonical `en-llm`
- no `[value × N]` text appeared in source on disk
- formatting continued to operate correctly on canonical text

### 5) Hover trust for compression

Hovering the compressed span showed:

- transform label: `Oct en-human (repeated literal compression)`
- display-only value: `[0.0 × 6]`
- canonical source list (full list form)
- exact literal token (`0.0`)
- repeat count (`6`)

Trust result: hover made the canonical value/count explicit and clearly indicated display-only behavior.

### 6) Doc-aware hover (real example)

Hovered `NormalStress` in the same file (with attached `///` docs).

- summary present: ✔
- parameters present: ✔ (`force`, `area`)
- returns present: ✔
- units present: ✔
- remarks present: ✔
- example present: ✔

Overall readability: concise and useful; sections were clearly separated and easy to scan.

Unsupported-scope check:

- hovering an unrelated identifier without an attached in-file `///` declaration produced no special doc block (graceful fallback to normal hover behavior).

### 7) Editing safety

On the compressed-array line:

1. placed cursor inside the canonical array
2. changed one literal (`0.0` → `3.0`) to break exact repetition
3. saved file

Observed:

- cursor/edit behavior remained normal (no cursor jumps or blocked edits)
- canonical edit applied exactly where typed
- decoration refreshed immediately (compression removed once repetition was broken)
- no editing confusion observed

### 8) Toggle behavior

Executed OFF → ON → OFF:

- OFF: no `en-human` compression/overlays visible
- ON: compression and other `en-human` overlays appeared
- OFF: overlays disappeared cleanly

Observed:

- compression appeared/disappeared correctly with toggle state
- doc-aware hover appeared only when enabled
- no stale decorations remained after disabling

### Existing M76b regressions check

Spot-checked previously shipped transforms during the same host session:

- `m^2` → `m²` (typography)
- `kg*m/s^2` → `N` (derived-unit whitelist collapse)
- `2500000Pa` → `2.5e6Pa` (scientific notation)

Observed: no M76b regressions.

### Screenshot status

Screenshot capture tooling was not available in this execution environment, so this section records precise observed host-session behavior textually.
