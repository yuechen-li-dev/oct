# Packages

## Overview

Packages define code boundaries and import resolution.
Imports are explicit and qualified.
Package loading is directory-based.

## Rules

- Package declaration form is `package Name`.
- Import form is `import Name`.
- Imported package members are referenced with qualification (`Name.Symbol()`).
- One directory corresponds to one package name.
- Source files in one package directory must declare the same package name.
- Imports resolve as sibling directories under the package root (`<root>/<ImportName>`).
- `manifest.oct` is package metadata, not a regular source file.
- When manifest mode is active for a root, every loaded package directory must include `manifest.oct`.
- When manifest mode is not active, `manifest.oct` is optional.
- Package dependency metadata is declared by `fn Manifest() -> PackageManifest`. See [15 oct pkg](../tooling/15-oct-pkg.md).

## Examples

Valid:

```oct
package Main

import Geometry

fn Main() -> Int {
    return Geometry.Distance(3, 4)
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    return Distance(3, 4)
}
```
