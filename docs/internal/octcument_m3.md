# OctCument M3: LaTeX backend and PDF materialization

Status: implemented 2026-09-15.

## Authority and reused infrastructure

M3 preserves the authority chain:

```text
ordinary Oct -> Document.Doc -> semantic Resolve/ValidateFor
             -> internal/document LaTeX lowering -> portable .tex bundle
             -> existing artifact staging/publishing -> external LaTeX engine -> PDF
```

The repository contained two relevant facilities. The transactional artifact
publisher already provides relative-path validation, staging, atomic publishing,
content hashing, and unchanged detection; M3 reuses it for `.tex`, `.pdf`, images,
and BibTeX files. `Libraries/Pdf` and `octxiliary-pdf` are a direct drawing API and
therefore are deliberately not reused as document authority. No pre-existing TeX
renderer or engine wrapper existed, so M3 adds one narrow process adapter that
discovers `OCT_LATEX_ENGINE`, `pdflatex`, `xelatex`, or `lualatex` and never enables
shell escape.

## Stable textual publishing IR

`internal/document/latex.go` is the only LaTeX emitter. `Artifact.Latex` publishes
its exact source and bundle. `Artifact.Pdf` calls the same function, publishes the
same sibling source/bundle, and sends those bytes to `internal/document/pdf.go`.
The academic integration proof compares the two public APIs byte for byte.

Package order, labels, whitespace, numeric formatting, content-addressed asset
names, and bibliography invocation are deterministic. PNG/JPEG and `.bib` sources
are read relative to the artifact entry file and emitted as `assets/<kind>-<hash>`;
generated LaTeX never includes temporary or absolute host paths.

## Semantic coverage

M3 renders headings, paragraphs, lists, tables, code, callouts, rules, page breaks,
groups, rich inlines, hyperlinks, figures, page chrome, sections, abstracts,
equations, citations, and bibliography references. Figure/table/equation/section
numbers and visible reference text come from `Document.Resolve`; explicit LaTeX
tags agree with those semantic counters.

Equation payloads are bounded math notation. Empty payloads and document-level
commands such as `input`, `include`, `begin`, `end`, macro definitions, writes, and
package/class declarations are rejected. Arbitrary raw LaTeX is not exposed.

Letter/A4, orientation, margins, body point size, and line spacing map directly.
Named semantic fonts normalize to scalable Latin Modern under pdfTeX instead of
depending on host-installed Inter or Cascadia Mono. Abstracts use semantic body
size/leading with hyphenation-aware ragged composition. Page/margin anchored figures
use `textpos` with mechanical point/millimeter conversion. Paragraph/character
anchors and nonbaseline z-order are capability errors.

## PDF reproducibility and failure behavior

The compiler uses a stable `paper` job name, `SOURCE_DATE_EPOCH=946684800`,
`FORCE_SOURCE_DATE=1`, UTC, and three LaTeX passes; BibTeX runs between passes only
when required. Diagnostics retain the engine, exit code, a bounded relevant log
slice, and the published source path. A missing tool is an error, not a panic or a
fallback to custom PDF drawing.

On the Windows M3 qualification host, MiKTeX-pdfTeX 4.23 (MiKTeX 25.12) produced
byte-identical PDFs across repeated runs. This is evidence for that exact toolchain,
not a cross-engine guarantee. Deterministic `.tex` remains the portable contract.

## Intentional boundary

M3 does not add a PDF semantic IR, custom PDF layout, raw document-level TeX, a TeX
parser, CSL, Biber, journal classes, Beamer, a full math IR, OMML, paragraph/
character TeX anchors, arbitrary overlay z-order, tagged-PDF alt text, or bundled
font files.
