# Pdf.Core MVP Report (Mx105)

## 1) Why `Pdf.Core` is pixel-native

`Pdf.Core` exposes page size, text placement, and image placement exclusively in `Int<px>` so users compose in screen-like pixel space rather than print/PDF-native units.

## 2) Backend dependency used

The runtime wrapper uses `codeberg.org/go-pdf/fpdf` for PDF generation.

## 3) Internal coordinate mapping

Wrapper internals convert pixel inputs to PDF points using:

- `pt = px * (72 / 96)`

The user-facing API never requires points/inches.

## 4) Implemented API surface

- `NewPage(width: Int<px>, height: Int<px>) -> PdfPage ! Error`
- `DrawText(page: PdfPage, x: Int<px>, y: Int<px>, text: String) -> Int ! Error`
- `DrawTextStyled(page: PdfPage, x: Int<px>, y: Int<px>, text: String, style: TextStyle) -> Int ! Error`
- `DrawImage(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>) -> Int ! Error`
- `DrawImageSized(page: PdfPage, image: ImageHandle, x: Int<px>, y: Int<px>, width: Int<px>, height: Int<px>) -> Int ! Error`
- `Save(page: PdfPage, path: String) -> Int ! Error`

## 5) How Inter is defaulted

Inter is expected at the repository-controlled font path:

- `internal/interpret/assets/fonts/Inter-Regular.ttf` (added manually in this workflow due binary submission limits)

The PDF wrapper attempts to load this file directly from disk and sets Inter as default for text rendering when present.

Fallback behavior:

- If Inter is missing or registration fails, fallback is backend core font `Helvetica`.

Current non-Latin limitation:

- This MVP does not implement script-aware fallback chains; glyph coverage is limited to what bundled Inter provides.

## 6) Tests proving MVP

- `Libraries/Pdf/Pdf.Core.octest` covers page creation, text, image, sized image, invalid save path, invalid page handle, invalid image handle.
- `cmd/oct/m105_pdf_core_wrappers_test.go` orchestrates library tests and verifies emitted PDF bytes include either Inter (when file is present) or Helvetica fallback markers.
- `internal/typecheck/typecheck_test.go` adds Pdf builtin signature acceptance and rejection checks.

## 7) Deliberately excluded

- text flow/layout engine
- tables and report builders
- forms/annotations/interactivity
- broader font management UI
- multi-page composition abstractions

## Documentation/behavior gap surfaced explicitly

- `Language/reference` does not currently document cross-library import/dependency behavior for running isolated library roots under `oct test`.
- To keep `Libraries/Pdf` tests deterministic as a standalone root, image interop uses the shared underlying image-handle identity (`ImageLoad` builtin handle wrapped into `Pdf.ImageHandle`) rather than a direct `Image.Core` type import.
- This is a wrapper-surface ergonomics gap to address in future reference/docs updates.
