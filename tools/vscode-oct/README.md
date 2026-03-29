# Oct VSCode Extension (M76a Foundation)

This directory contains the **M76a** baseline VSCode extension for Oct.

Scope is intentionally narrow:

- language/file association
- TextMate syntax highlighting
- language configuration (comments/brackets/autopairs)
- canonical formatter integration via `oct fmt`
- command palette + task integration for run/test/format

No hover/LSP/autocomplete/refactor/alternate source views are included in this milestone.

---

## Location

- Extension root: `tools/vscode-oct/`

---

## Local install / run

1. Open this repository in VSCode.
2. Open `tools/vscode-oct/`.
3. Run:

   ```bash
   npm install
   ```

   (No runtime dependencies are required; this keeps VSCode extension tooling conventions happy.)

4. Press `F5` in VSCode to launch an Extension Development Host.

---

## Supported file types

The extension registers Oct language mode for:

- `.oct`
- `.octest`
- `.octfail`
- `.octagon` (included as an optional convenience)

---

## Formatter wiring

Document formatting is wired to the canonical formatter command:

- `<oct.cli.command> fmt <current-file>`

Default command:

- `go run ./cmd/oct`

This default is **repo-local** for this Oct repository setup (so the extension works without requiring a globally installed `oct` binary).

So by default formatting runs as:

- `go run ./cmd/oct fmt <current-file>`

You can override the command via VSCode setting:

- `oct.cli.command`

Example override:

- `oct`

---

## Commands

Command palette entries:

- `Oct: Run Current File`
- `Oct: Test Current File`
- `Oct: Format Current File`
- `Oct: Run Workspace`
- `Oct: Test Workspace`
- `Oct: Format Workspace`

These shell out to the configured CLI command in the integrated terminal.

The same current-file actions are also available from the editor right-click context menu when editing Oct files.

---

## Tasks

The extension contributes a basic `oct` task provider with workspace tasks:

- `Oct: Run Workspace`
- `Oct: Test Workspace`
- `Oct: Format Workspace`

Run these from **Terminal → Run Task…**.

---

## Intentionally deferred (out of M76a)

- language server / LSP
- hovers and semantic docs rendering
- intelligent completion
- code actions and refactors
- semantic diagnostics engine
- alternate source views (`en-human`, localization, display transforms)

M76a is the stable editor baseline only.

---

## Manual validation checklist (M76a hardening)

1. Open any `.oct`, `.octest`, or `.octfail` file.
   - Confirm VSCode language mode shows **Oct** and syntax highlighting is active.
2. Run **Format Document**.
   - Confirm the extension invokes canonical `oct fmt` for the current file.
   - Confirm comments (`//`) and doc comments (`///`) remain intact.
3. Run **Oct: Run Current File**.
   - Confirm command executes in the integrated terminal.
4. Run **Oct: Test Current File** (or **Oct: Test Workspace**).
   - Confirm test execution runs in the integrated terminal.
5. (Optional) Open an `.octagon` file and confirm language association to **Oct**.


## Validation Evidence

Validation run date: **2026-03-29**.

Executed in a real VSCode Extension Development Host (headless Electron via `xvfb-run`) against this repository workspace.

1. **Language activation**
   - Opened: `tmp validation/format target with spaces.oct`.
   - Observed: language mode resolved to **Oct** (`languageId === "oct"`) ✔.
   - Observed: TextMate grammar binding active for Oct files (highlighting active in editor) ✔.

2. **Syntax highlighting**
   - Checked a file containing `//`, `///`, and declarations.
   - Observed: comments highlighted, doc comments highlighted distinctly from regular comments, and keywords/types tokenized under Oct grammar ✔.
   - Status: **works**.

3. **Format Document (critical)**
   - Invoked VSCode **Format Document** on the active `.oct` file.
   - Observed: file content changed ✔.
   - Observed: output matched canonical `oct fmt` result (idempotence check against `go run ./cmd/oct fmt`) ✔.
   - Observed: `///` doc comments preserved exactly ✔.
   - Observed: single final replacement result (no partial/fragmented edits) ✔.

4. **Run command**
   - Invoked **Oct: Run Current File**.
   - Observed: integrated terminal opened (`Oct`) ✔.
   - Observed: run command invoked configured CLI with `run` subcommand ✔.
   - Observed: current file path with spaces passed correctly as one argument ✔.

5. **Test command**
   - Invoked **Oct: Test Workspace** from the same session.
   - Observed: integrated terminal execution triggered via configured CLI with `test` subcommand ✔.
   - Status: **works**.

6. **Failure handling**
   - Set `oct.cli.command` to a non-existent executable (`missing-oct-cli-command`).
   - Invoked **Format Document**.
   - Observed: failure surfaced clearly (`Oct format failed: ... not found`) ✔.
   - Observed: no silent success and target file remained unchanged ✔.
