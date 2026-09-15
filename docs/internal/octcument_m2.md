# OctCument M2: figures and anchored publishing

Status: implemented 2026-09-15.

## Authority pipeline

M2 preserves the existing direction:

```text
ordinary Oct
  -> Document.Doc
  -> semantic numbering and reference resolution
  -> capability validation
  -> Markdown or DOCX
```

The public model contains file-backed figures, typed document lengths, semantic
placement, captions, identifiers/references, and page chrome. DrawingML elements,
relationship IDs, drawing IDs, media names, EMUs, and Word fields exist only under
`internal/document`.

## Manual geometry contract

X increases rightward and Y increases downward from the declared anchor reference.
Coordinates and z-order must be non-negative. `FigureAnchor.Page`, `Margin`,
`Paragraph`, and `Character` map to supported DrawingML page/page-margin,
column/paragraph, and character/line references. Supported floating wraps are
Square, TopBottom, BehindText, and InFrontOfText.

The engineering acceptance specimen uses:

```oct
Document.PageAnchor(20.0, 30.0, 120.0)
```

The exact DOCX geometry is mechanically asserted as:

```text
X      20 mm * 360000 EMU/mm =  7,200,000 EMU
Y      30 mm * 360000 EMU/mm = 10,800,000 EMU
Width 120 mm * 360000 EMU/mm = 43,200,000 EMU
```

The focused DOCX and artifact integration tests inspect `word/document.xml` for
those exact values. Point conversion is separately asserted at 12,700 EMU/pt.

## Determinism

Media filenames use a SHA-256 prefix and the decoded PNG/JPEG format. Identical
image bytes reuse one media part and one image relationship. Relationships and
drawing IDs follow deterministic semantic encounter order; ZIP entries are sorted
and retain the existing fixed 1980 timestamp. Unit and artifact integration tests
render twice and require byte identity.

## Markdown degradation

Markdown renders every figure in flow with semantic alt text and a numbered caption.
For anchored figures it also emits a deterministic HTML comment stating that the
absolute geometry was omitted. Header/footer page chrome is omitted because Markdown
has no stable page model.

## Deliberate M2 boundary

SVG, tight contour wrapping, arbitrary positioned blocks/text boxes, section and
equation references, image editing, DOCX parsing, and additional renderers remain
out of scope.
