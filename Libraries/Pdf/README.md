# Pdf

## Role

`Pdf.Core` is Oct's pixel-native PDF output layer.

The user-facing coordinate system is always pixel space (`Int<px>`):

- page size is defined in px
- text coordinates are px
- image coordinates/sizes are px

PDF units are an internal backend detail.

## Current MVP surface

- `record PdfPage { Handle: Int }`
- `record TextStyle { Size: Int<px>, ColorR: Int, ColorG: Int, ColorB: Int }`
- `NewPage(width: Int<px>, height: Int<px>) -> PdfPage ! Error`
- `DrawText(page: PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`
- `DrawTextStyled(page: PdfPage, x: Int<px>, y: Int<px>, text: String, style: TextStyle) -> Int ! Error`
- `record ImageHandle { Handle: Int }` (bridges to the existing `ImageLoad` handle identity)
- `DrawImage(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error`
- `DrawImageSized(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`
- `Save(page: PdfPage, path: String) -> Int ! Error`
- `DefaultTextStyle() -> TextStyle`

## Font behavior (MVP)

- Default Latin font target: **Inter**.
- Inter is loaded from the repository-controlled path `internal/interpret/assets/fonts/Inter-Regular.ttf`.
- In this authoring flow, the binary TTF is added manually due binary submission limitations; once present at that path, no code changes are required.
- If Inter is missing or registration fails, backend fallback is `Helvetica`.
- Non-Latin glyph coverage is not a solved goal in this MVP; behavior depends on glyph availability in the bundled Inter font.

## Coordinate mapping detail

Internally, the PDF backend runs in points (`pt`) with conversion:

- `pt = px * (72 / 96)`

This keeps the public API pixel-native while preserving deterministic PDF output scaling.

## Example

```oct
package Example

import Pdf

fn Main() -> Int ! Error {
    let page = Pdf.NewPage(640px, 480px)?
    Pdf.DrawText(page, 24px, 28px, "Pixel-native PDF")?

    let raw = ImageLoad("logo.png")?
    let logo = Pdf.ImageHandle { Handle: raw }
    Pdf.DrawImageSized(page, logo, 24px, 56px, 192px, 96px)?

    Pdf.Save(page, "example.pdf")?
    return 0
}
```

## Intentional exclusions in this milestone

- flowing paragraph layout
- rich text shaping/advanced typography controls
- tables/report builders
- forms/annotations/interactivity
- multi-page framework abstractions
