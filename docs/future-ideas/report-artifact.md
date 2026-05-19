# Oct Report Artifact Roadmap: Markdown, LaTeX, and Structured Reports

## Purpose

Oct needs a clean report-artifact story for scientific experiments.

The current artifact stack has useful pieces:

* `.octagon` as Oct’s native structured artifact format.
* CSV for compact tabular interchange.
* JSON for compact machine-readable summaries.
* Markdown for lightweight human-readable reports.

However, Markdown should not become the core document model. Markdown is convenient as an output format, but it is structurally weak for serious nested documents, code examples, figures, tables, equations, and future PDF/UI rendering.

The long-term direction should be:

```text
Structured Oct report model
    -> Markdown renderer
    -> LaTeX renderer
    -> PDF renderer
    -> HTML/UI renderer later
```

Markdown remains useful, but it should be one rendering target, not the foundation.

## Why Markdown Is Not Enough

Markdown is pleasant for simple notes, but it is fundamentally a flat line-oriented format pretending to be a document tree.

This becomes painful when reports need:

* Nested code blocks.
* Code examples that contain Markdown.
* Tables with metadata.
* Figures and captions.
* Callouts.
* Equations.
* Cross-references.
* PDF-quality layout.
* Future UI rendering.

The code-fence problem is a concrete example. Standard Markdown code fences require delimiter hardening, such as choosing longer backtick fences when the content itself contains triple backticks. This works as a defensive measure, but the need for it proves the underlying model is fragile.

HTML/XML solved the document-tree problem with explicit nested tags. Markdown solved casual authoring ergonomics, but threw away too much structure. Oct should eventually take the useful middle path: structured document blocks with humane authoring and deterministic renderers.

## Current Markdown Role

Markdown M0/M1 should stay scoped as a deterministic report-writing helper.

It is useful for generating `.md` artifacts from Oct experiment code:

* `Markdown.H1`
* `Markdown.Section`
* `Markdown.Paragraph`
* `Markdown.Table`
* `Markdown.KeyValueTable`
* `Markdown.Callout`
* `Markdown.Figure`
* `Markdown.CodeBlock`
* `Markdown.Report`
* `Artifact.WriteMarkdown`

This is good for lightweight reports and quick human inspection.

Markdown should not claim CommonMark compliance, should not parse arbitrary Markdown, and should not become the canonical internal report representation.

## LaTeX as the Serious Scientific Backend

For scientific reports, LaTeX is a better final rendering target than Markdown.

LaTeX supports:

* Mathematical notation.
* Equation blocks.
* Figure captions.
* Tables.
* Cross-references.
* Bibliography paths later.
* Stable PDF output.
* Print-oriented layout.

LaTeX has its own escaping problems, but those are more appropriate for scientific typesetting than Markdown’s structural limitations.

The first LaTeX milestone should emit deterministic `.tex` files, not PDF directly. PDF generation introduces environmental dependencies such as `pdflatex`, `xelatex`, fonts, packages, image paths, and CI setup. Exact `.tex` output is easier to test and should come first.

## Do Not Lower Markdown Directly to LaTeX

The long-term architecture should not be:

```text
Markdown string -> LaTeX
```

That requires parsing Markdown and recreates the same ambiguity problems.

Instead, Oct should use:

```text
Structured report value -> Markdown
Structured report value -> LaTeX
Structured report value -> PDF later
```

Markdown is then just a renderer.

## Proposed Long-Term Model

Introduce a `Report` or `Document` layer.

Possible API shape:

```text
Report.Document(...)
Report.Title(...)
Report.Section(...)
Report.Subsection(...)
Report.Paragraph(...)
Report.Callout(...)
Report.CodeBlock(...)
Report.Table(...)
Report.KeyValueTable(...)
Report.Figure(...)
Report.Equation(...)
Report.Bullets(...)
Report.Numbered(...)
```

Then provide renderers:

```text
Report.ToMarkdown(report) -> [String]
Report.ToLatex(report) -> [String]
```

Artifact helpers can then write those outputs:

```text
Artifact.WriteMarkdown("report.md", Report.ToMarkdown(report))
Artifact.WriteLatex("report.tex", Report.ToLatex(report))
```

Later:

```text
Artifact.WritePdf("report.pdf", report)
```

The important idea is that the report is structured first. Markdown and LaTeX are output forms.

## Suggested Artifact Roles

Oct should treat artifact formats by intent:

### `.octagon`

Primary native structured artifact.

Use for:

* Full experiment reports.
* Structured metrics.
* Tables.
* Configuration snapshots.
* Diagnostic summaries.
* Data that Oct should round-trip natively.

### `.csv`

Compact tabular interchange.

Use for:

* Small metrics tables.
* Row-major exported CSV.
* Compatibility with spreadsheet tools.

CSV import should be explicit about shape:

* `Csv.ReadRows` for raw row-major string grids.
* `Csv.ReadTable` for header-based columnar records.
* `Csv.ReadMatrix` for numeric grids.

### `.json`

Compact machine-readable summary.

Use for:

* Summaries.
* Settings.
* Small structured data.
* External tool compatibility.

### `.md`

Lightweight human-readable report.

Use for:

* Quick inspection.
* GitHub/GitLab-friendly reports.
* Human-readable lab notes.

Markdown should be generated by helper functions, not hand-built string soup.

### `.tex`

Serious scientific report source.

Use for:

* PDF-ready reports.
* Math-heavy output.
* Figure/table/caption-heavy documents.
* Publication-style artifacts.

### `.pdf`

Future final report output.

Use for:

* Stable rendered reports.
* Distribution.
* Visual inspection.
* Archival scientific reports.

PDF generation should come after deterministic `.tex` generation is stable.

## Table Shape Principle

Oct’s native table convention should be columnar records.

Example:

```text
{
    mode: [...],
    method: [...],
    outputSNRDb: [...],
    nrmse: [...],
    label: [...]
}
```

This is easier to read and author in scientific code than row-major arrays. Most experiment data is naturally columnar: each variable is a named vector.

Display formats such as Markdown tables and CSV files are row-oriented, but exporters should handle that transpose internally.

Rule:

```text
Named table data -> columnar record
Numeric grid/matrix data -> nested arrays or Matrix
CSV raw storage -> row-major rows
```

## Future Oct Markdown Dialect

Eventually, Oct may support an Oct-owned `.md` dialect with structured blocks.

This could support syntax conceptually like:

```text
<Code language="oct">
...
</Code>

<Figure path="plot.png" caption="Recovered signal" />

<Callout kind="warning">
...
</Callout>
```

This should not be treated as CommonMark or MDX compatibility. It would be Oct’s own document dialect, parsed by Oct’s own lexer/parser/renderer when the project is ready for serious PDF/UI report generation.

This is future work. The current Markdown library should remain programmatic and line-oriented.

## Implementation Roadmap

### Stage 1: Markdown report helpers

Status: in progress / mostly implemented.

Goal:

* Deterministic `[String]` report generation.
* Tables from columnar records.
* Key-value tables.
* Callouts.
* Figures.
* Code blocks with dynamic fence hardening.
* Artifact emission via `Artifact.WriteMarkdown`.

### Stage 2: Structured `Report` model

Introduce a structured report/document value independent of Markdown.

Goal:

* One report definition.
* Multiple renderers.
* Clear document block model.
* Avoid locking report semantics to Markdown strings.

### Stage 3: Markdown renderer for `Report`

Render structured reports to Markdown lines.

Goal:

* Preserve current `.md` artifact usefulness.
* Make Markdown a renderer, not the document model.

### Stage 4: LaTeX renderer for `Report`

Render structured reports to deterministic `.tex`.

Goal:

* Exact-content tests.
* No PDF dependency yet.
* Support sections, paragraphs, lists, tables, figures, code blocks, callouts, and equations.

### Stage 5: Optional PDF generation

Compile LaTeX to PDF if a TeX engine is available.

Goal:

* Optional integration path.
* Clear diagnostics if toolchain missing.
* CI-safe behavior.

### Stage 6: Future UI/HTML renderer

Render structured reports to UI or HTML if needed.

Goal:

* Same report model.
* Multiple output targets.
* No Markdown parsing dependency.

## Design Principle

Markdown is a useful output format, but not the source of truth.

The source of truth should become a structured Oct report model.

Octagon remains the native machine-readable artifact. Markdown remains the lightweight human-readable artifact. LaTeX becomes the serious scientific/reporting backend. PDF comes later as a rendered product.

That gives Oct a clean scientific reporting story without inheriting Markdown’s structural limitations or turning LaTeX into the authoring surface.
