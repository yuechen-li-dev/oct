# Document Library (OctCument M0)

`Libraries/Document` is the canonical backend-neutral structured-document model.
Documents are ordinary immutable Oct records, enums, arrays, and functions. Markdown
and DOCX are renderers; neither format is the source of truth.

```text
ordinary Oct -> Document.Doc -> capability validation -> Markdown or DOCX artifact
```

Authoring files may conventionally be named `resume.doc.oct`, `report.doc.oct`, or
`manual.doc.oct`. The suffix has no parser, typechecker, or execution semantics.

Use `Document.Build` for defaults or `Document.BuildWith` for explicit metadata,
styles, and page layout. Refined `concept` values enforce M0 heading levels,
positive font sizes/line spacing, and non-negative spacing/margins at compile time.
The default semantic font is `Inter`, matching the repository-owned
`internal/interpret/assets/fonts/Inter-Regular.ttf`; renderers use normal platform
font substitution when that family is unavailable.
`Document.Title`, `Subtitle`, `H1`, `H2`, `P`, `Section`,
`Subsection`, `Bullets`, `Numbered`, `Table`, `KeyValueTable`, `Code`, and `Callout`
are immutable convenience functions over the canonical values. Application-specific
helpers should return `Document.Block`; the core has no resume/report domain blocks.

Applications may use ordinary Oct `template record` and `template fn` declarations
to specialize reusable document themes around an application-owned parameter type.
The resume acceptance specimen demonstrates this pattern. Specialization produces
the same `Document.Doc`; it is not a macro, runtime generic object model, renderer
template, or alternate document IR. Document M0 deliberately does not wrap this in
an imported generic facade because cross-package sibling-template rebinding is a
current compiler limitation.

`Document.ToMarkdown` deterministically projects the semantic model. Page breaks
become `<!-- pagebreak -->`; page layout and typographic presentation hints are
ignored because Markdown has no equivalent. Code fences harden against the longest
backtick run in the code. `Artifact.Markdown(path, doc)` and
`Artifact.Docx(path, doc)` materialize documents during `oct artifact`. Under the
current language contract, the `[Artifact]` entry point lives in a sibling `.octest`
file (or `Make.oct`), while the conventionally named `*.doc.oct` file contains the
ordinary reusable document value. No suffix is special-cased.

DOCX M0 supports styled headings and paragraphs, inline bold/italic/code, hyperlinks,
lists, tables, callouts, horizontal rules, real page breaks, page size/orientation,
and margins. Its OOXML and ZIP details are renderer-private. Package entries,
relationships, style order, IDs, metadata, and ZIP timestamps are canonicalized;
equal documents produce byte-identical output.

`Document.ValidateFor` returns deterministic diagnostics for invalid M0 heading and
table shapes. All current M0 semantic block kinds are supported by both renderers.

The existing `Markdown.*` string helpers remain compatibility APIs. They are legacy
direct render helpers, not canonical document nodes; no Markdown parsing or reverse
conversion is performed.

Deferred beyond M0: figures, equations/OMML, headers/footers/page numbering, cell
spans, bibliography/citations, shared imported template facades/themes, parsing,
and additional renderers.
