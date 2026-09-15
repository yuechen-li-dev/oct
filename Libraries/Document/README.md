# Document Library (OctCument M3)

`Libraries/Document` is the canonical backend-neutral structured-document model.
Documents are ordinary immutable Oct records, enums, arrays, and functions. Markdown
and DOCX are renderers; neither format is the source of truth.

```text
ordinary Oct -> Document.Doc -> capability validation -> Markdown, DOCX, or LaTeX -> PDF
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

M2 adds first-class PNG/JPEG figures. `Figure(source, caption, placement)` is the
concise flow API; `FigureIdentified` adds a stable cross-reference target.
`WidthMM`, `WidthPt`, and `FigureAuto` create centered inline placements whose
width preserves the source aspect ratio. `ExactMM` requests intentional exact
width and height. Image paths are artifact-backed and resolved relative to the
artifact entry source; the document library references images and does not process
or rewrite them.

`AnchoredFigure` and `AnchoredFigureIdentified` are the manual authority path.
`PageAnchor(xMM, yMM, widthMM)` is the concise form, while `Anchor` accepts the
public `FigureAnchor`, `Length`, `FigureSize`, `FigureWrap`, and z-order values.
The coordinate contract is explicit: X increases rightward, Y increases downward,
and both are measured from the declared reference. Page is the page edge, Margin
is the page margin, Paragraph maps horizontal position to the containing column
and vertical position to the paragraph, and Character maps to character/line.
Coordinates and z-order are non-negative. Public lengths are only `Pt(Float)` and
`Mm(Float)`; Word-native EMUs, twips, and half-points remain renderer-private.
Supported anchored wraps are `Square`, `TopBottom`, `BehindText`, and
`InFrontOfText`. The DOCX renderer honors the declared geometry or fails; it does
not silently reinterpret it.

Figure captions and `TableWithCaption` remain semantic inline content and use the
existing Caption paragraph style. `FigureRef` and `TableRef` create typed semantic
references. One document-order numbering pass walks nested `Group` blocks, numbers
figures and labeled tables, resolves reference text identically for both renderers,
and diagnoses duplicate IDs, missing targets, and wrong kinds before materialization.

`HeaderFooter` adds modest page chrome without changing the M0/M1 `Doc` record.
Headers and footers accept ordinary rich inline content plus `DocumentTitle`,
`DocumentAuthor`, and the semantic `PageNumber` token. DOCX emits a real page-number
field; Markdown omits page chrome because pages have no stable meaning there.

`Document.Template<Parameters>` and `Document.Instantiate<Parameters>` provide the
shared reusable template facade. Their function fields produce metadata, styles,
layout, and semantic blocks from an application-owned parameter record. Instantiation
produces the same `Document.Doc`; it is not a macro, runtime generic object model,
renderer template, or alternate document IR. The resume acceptance specimen proves
the facade across the `Document` package boundary.

`Document.ToMarkdown` deterministically projects the resolved semantic model. Page breaks
become `<!-- pagebreak -->`; page layout and typographic presentation hints are
ignored because Markdown has no equivalent. Anchored figures remain visible in
normal flow and carry `<!-- anchored figure geometry omitted in Markdown -->`, so
the lost geometry is explicit. Code fences harden against the longest
backtick run in the code. `Artifact.Markdown(path, doc)` and
`Artifact.Docx(path, doc)` materialize documents during `oct artifact`. Under the
current language contract, the `[Artifact]` entry point lives in a sibling `.octest`
file (or `Make.oct`), while the conventionally named `*.doc.oct` file contains the
ordinary reusable document value. No suffix is special-cased.

DOCX supports styled headings and paragraphs, inline bold/italic/code, hyperlinks,
lists, tables, callouts, horizontal rules, real page breaks, page size/orientation,
and margins. M2 adds DrawingML inline/anchored figures, content-addressed media,
header/footer parts, and semantic page numbering. Its OOXML and ZIP details are renderer-private. Package entries,
relationships, style order, IDs, metadata, and ZIP timestamps are canonicalized;
equal documents produce byte-identical output.

`Document.ValidateFor` returns deterministic diagnostics for invalid heading and
table shapes, invalid figure dimensions/coordinates, duplicate identifiers, and
missing or wrong-kind references. Missing or invalid image files are diagnosed
during artifact materialization, where filesystem authority exists.

The existing `Markdown.*` string helpers remain compatibility APIs. They are legacy
direct render helpers, not canonical document nodes; no Markdown parsing or reverse
conversion is performed.

Deferred beyond M2: SVG, tight wrapping, section/equation references, equations/OMML,
cell spans, bibliography/citations, parsing, and additional renderers. Absolute
positioning remains figure-only; M2 is not a general page-layout engine.

M3 adds `Format.Latex` and `Format.Pdf`, `Artifact.Latex`, and `Artifact.Pdf`.
Both artifact APIs use the same renderer-private textual lowering. PDF generation
compiles the preserved sibling `.tex`; it does not use `Libraries/Pdf` and does
not introduce a PDF-native semantic IR. Generated figures and `.bib` files are
copied to stable content-addressed paths under `assets/`, and the source contains
only relative slash-separated references.

Academic additions are bounded semantic `Section`, `Abstract`, `Equation`,
`Citation`, and `Bibliography` values. `LabeledEquation`, `SectionIdentified`,
`EquationRef`, and `SectionRef` participate in the same deterministic numbering
pass as figures and tables. Equation payloads are LaTeX math notation only;
document-level commands are rejected by the backend. Citation keys remain style
neutral and lower through BibTeX's stable `plain` style.

LaTeX renders general blocks and rich inline content, uses `booktabs` tables,
non-shell-escape `verbatim` code, bounded quote-style callouts, `hyperref` links,
`fancyhdr` page chrome, and fixed-order figures/tables. Letter/A4, orientation,
and margins map directly. Body point size and line spacing are honored; named
DOCX theme fonts such as Inter and Cascadia Mono normalize to the selected TeX
engine's dependable built-in fonts rather than making PDF generation depend on
system font installation.

Page- and margin-relative anchored figures lower through typed `textpos`
coordinates. Paragraph/character anchors and z-orders above the baseline 0/1 are
rejected for LaTeX/PDF instead of being approximated. DOCX still owns its full M2
DrawingML anchor contract. Equations and bibliographies remain explicitly
unsupported by DOCX in M3; Markdown projects equations as display math and keeps
citations visible without pretending to format a bibliography.

`Artifact.Pdf` discovers `OCT_LATEX_ENGINE` first, then `pdflatex`, `xelatex`, or
`lualatex`. It runs with shell escape disabled, a fixed job name, UTC, and a fixed
`SOURCE_DATE_EPOCH`, invoking BibTeX only when the generated auxiliary file
requires it. Engine failures report the engine, exit code, concise diagnostic,
and `.tex` artifact path. Reproducibility is toolchain-dependent; the generated
`.tex` is always the authoritative deterministic representation.

Deferred beyond M3: raw LaTeX blocks, a full math IR, CSL, Biber, OMML, paragraph/
character LaTeX anchors, arbitrary z-order, tagged-PDF alt text, journal classes,
font-file bundling, TeX import/round-trip, and custom PDF layout.
