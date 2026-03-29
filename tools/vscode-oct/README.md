# Oct VSCode Extension (M76b `en-human` View v1)

This directory contains the Oct VSCode extension baseline plus **M76b**:

- canonical editing/formatting/run/test behavior from M76a
- a new **editor-only** `en-human` reading layer

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

## What M76b adds

M76b implements three deterministic view transforms.

### 1) Unit typography (display-only)

Examples:

- `kg*m/s^2` → `kg·m/s²`
- `kg/(m*s^2)` → `kg/(m·s²)`
- `m^2` → `m²`
- `s^2` → `s²`

Rules:

- typography only
- no unit meaning changes
- canonical text unchanged

### 2) Derived-unit collapse (exact whitelist only)

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

### 3) Scientific notation display (display-only)

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

---

## Trust / explanation aid

`en-human` overlays include hover text that shows:

- transform kind
- displayed `en-human` form
- canonical source form

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

## Deferred (intentionally out of M76b)

- repeated-literal compression
- repeated zero-array compression
- doc hover enrichment beyond trust overlays
- localization views (`zh-CN-human`, etc.)
- LSP/server architecture
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

## Validation Evidence (M76b)

Validation run date: **2026-03-29**.

Environment used: VSCode Extension Development Host launched against this repo (`tools/vscode-oct`) and exercised on a real `.oct` file containing M76b transform candidates.

Validation file used during the run (canonical source):

- `m^2`
- `kg*m/s^2`
- `kg/(m*s^2)`
- `2500000Pa`
- `0.000001s`
- `8m/s` (unsupported collapse case)

### 1) Toggle behavior

Observed sequence:

1. Set `oct.enHuman.enabled = false`.
   - Editor showed canonical text only.
2. Ran **Oct: Toggle en-human View** to enable.
   - Overlays appeared immediately (no save/reopen required).
3. Ran **Oct: Toggle en-human View** again to disable.
   - Overlays disappeared immediately.

Observed result:

- refresh is immediate on toggle.
- no stale decorations remained after toggling off.

### 2) Canonical source safety (save + format)

With `en-human` enabled:

- saved the file.
- ran **Format Document**.
- re-read file contents from disk.

Observed result:

- on-disk text remained canonical `en-llm`.
- no `en-human` display forms (e.g. `·`, superscript digits, collapsed unit symbols replacing canonical expressions) were written into source.
- no source mutation into `en-human` occurred.

### 3) Unit typography case

Case A:

- canonical source: `m^2`
- observed rendered view: `m²`
- classification: typography-only transform

Case B:

- canonical source: `kg*m/s^2`
- observed rendered view: `kg·m/s²`
- classification: typography-only rendering when derived collapse is not the selected presentation in that span

### 4) Derived-unit collapse case

Supported whitelist case used:

- canonical source: `kg*m/s^2`
- observed rendered view: `N`

Hover trust text on transformed span showed:

- transform kind (`Oct en-human (derived unit)`)
- display form (`N`)
- canonical source (`kg*m/s^2`)

### 5) Scientific-notation display case

Validated both threshold directions:

- canonical: `2500000Pa` → rendered: `2.5e6Pa` (large-magnitude case)
- canonical: `0.000001s` → rendered: `1e-6s` (small-magnitude case)

Observed threshold behavior matched policy:

- convert when `abs(x) >= 1e4`
- convert when `0 < abs(x) < 1e-3`

### 6) Unsupported / non-collapsing case

Case used:

- canonical source: `8m/s`
- observed rendered view: remains `8m/s` (no derived-unit collapse)

Observed result:

- no unwanted collapse for unsupported/non-whitelisted unit expressions.

### 7) Hover trust text

On transformed spans (derived-unit and scientific notation examples), hover clearly showed:

- transform label (`Oct en-human (...)`)
- displayed form
- canonical source form

Observed result:

- enough context to answer “what am I seeing?” without ambiguity.

### 8) Editing safety

On a transformed line:

- moved cursor into canonical unit text.
- edited canonical expression.
- saved.

Observed result:

- editor editing/selection behavior remained usable.
- source stayed canonical after edit.
- decorations refreshed to match the new canonical text.

### Screenshot status

Screenshots were not captured in this environment during this run.
Validation evidence above is from direct Extension Development Host interaction and on-disk verification.

