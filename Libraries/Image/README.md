# Image

## Role

`Image.Core` is the Oct image foundation layer: loading, saving, and basic metadata inspection.

This module intentionally stays narrow so higher-level wrappers (`Plot.Core`, `Pdf.Core`, and future image processing helpers) can build on a stable image artifact substrate.

## Current MVP surface

- `Load(path: String) -> ImageHandle ! Error`
- `Save(image: ImageHandle, path: String) -> Int ! Error`
- `Width(image: ImageHandle) -> Int<px>`
- `Height(image: ImageHandle) -> Int<px>`
- `Format(image: ImageHandle) -> String`

`ImageHandle` is a thin handle-backed wrapper record (`Handle: Int`) following established wrapper patterns already used by `IO.Xlsx`.
The handle remains an opaque resource identity (not a geometric measurement).
Image dimensions are returned as `Int<px>` so geometry aligns with existing pixel-unit typing used by UI/layout surfaces.

## Supported formats (Mx103d MVP)

- load: PNG, JPEG
- save: PNG (`.png`), JPEG (`.jpg` / `.jpeg`)

## Example

```oct
package Example

fn Main() -> Int ! Error {
    let image = Image.Load("input.png")?
    let width = Image.Width(image)
    let height = Image.Height(image)
    Print(ToString(width) + "x" + ToString(height))
    let _saved = Image.Save(image, "copy.jpg")?
    return 0
}
```

## Common failure cases

- file path does not exist (`NotFound`)
- input is not a valid/supported image (`InvalidData`)
- save path extension unsupported (`InvalidArgument`)
- invalid image handle (`InvalidHandle`)
