# Document Library (OctCument M1)

`Libraries/Document` is the canonical backend-neutral structured-document model.
Documents are ordinary immutable Oct records, enums, arrays, and functions. Markdown
and DOCX are renderers; neither format is the source of truth.

```text
ordinary Oct -> Document.Doc -> capability validation -> Markdown or DOCX artifact
```

Authoring files may conventionally be named `resume.doc.oct`, `report.doc.oct`, or
`manual.doc.oct`. The suffix has no parser, typechecker, or execution semantics.

Use `Document.Build` for defaults or `Document.BuildWith` for explicit metadata,
styles, and page layout. Refined `concept` values enforce bounded heading levels,
positive font sizes/line spacing, and non-negative spacing/margins at compile time.
The default semantic font is `Inter`, matching the repository-owned
`internal/interpret/assets/fonts/Inter-Regular.ttf`; renderers use normal platform
font substitution when that family is unavailable.
M1 adds small immutable style transforms: `WithTextSize`, `WithColor`, `WithFont`,
`WithWeight`, `WithItalic`, `WithSpacing`, `WithAlignment`, and `WithLineSpacing`.
`ProfessionalStyle` and `ScientificStyle` are ordinary `StyleSheet`-returning
functions, so callers can continue composing them with `with`; there is no theme
inheritance or second style model.

`Document.Title`, `Subtitle`, `H1`, `H2`, `P`, `Section`,
`Subsection`, `Bullets`, `Numbered`, `Table`, `KeyValueTable`, `Code`, and `Callout`
are immutable convenience functions over the canonical values. Application-specific
helpers should return `Document.Block`; the core has no resume/report domain blocks.
`Texts` and `JoinInline` help compose explicit typed inline fragments without parsing
Markdown-like strings.

`Document.Template<Parameters>` and `Document.Instantiate<Parameters>` provide the
shared reusable template facade. Their function fields produce metadata, styles,
layout, and semantic blocks from an application-owned parameter record. Instantiation
produces the same `Document.Doc`; it is not a macro, runtime generic object model,
renderer template, or alternate document IR. The resume acceptance specimen proves
the facade across the `Document` package boundary.

`Document.ToMarkdown` deterministically projects the semantic model. Page breaks
become `<!-- pagebreak -->`; page layout and typographic presentation hints are
ignored because Markdown has no equivalent. Code fences harden against the longest
backtick run in the code. `Artifact.Markdown(path, doc)` and
`Artifact.Docx(path, doc)` materialize documents during `oct artifact`. Under the
current language contract, the `[Artifact]` entry point lives in a sibling `.octest`
file (or `Make.oct`), while the conventionally named `*.doc.oct` file contains the
ordinary reusable document value. No suffix is special-cased.

DOCX supports styled headings and paragraphs, inline bold/italic/code, hyperlinks,
lists, tables, callouts, horizontal rules, real page breaks, page size/orientation,
and margins. Its OOXML and ZIP details are renderer-private. Package entries,
relationships, style order, IDs, metadata, and ZIP timestamps are canonicalized;
equal documents produce byte-identical output.

`Document.ValidateFor` returns deterministic diagnostics for invalid heading and
table shapes. All current semantic block kinds are supported by both renderers.

The existing `Markdown.*` string helpers remain compatibility APIs. They are legacy
direct render helpers, not canonical document nodes; no Markdown parsing or reverse
conversion is performed.

Deferred beyond M1: figures, equations/OMML, headers/footers/page numbering, cell
spans, bibliography/citations, parsing, and additional renderers.
