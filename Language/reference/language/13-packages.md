# Packages

## Overview

Packages define code boundaries and import resolution.
Imports are explicit and qualified.
Package loading is directory-based.

## Rules

- Package declaration form is `package Name`.
- Import form is `import Name`.
- Imported package members are referenced with qualification (`Name.Symbol()` / `Name.Type.Member`).
- One directory corresponds to one package name.
- Source files in one package directory must declare the same package name.
- Imports resolve as sibling directories under the package root (`<root>/<ImportName>`).
- `manifest.oct` is package metadata, not a regular source file.
- When manifest mode is active for a root, every loaded package directory must include `manifest.oct`.
- When manifest mode is not active, `manifest.oct` is optional.
- Package dependency metadata is declared by `fn Manifest() -> PackageManifest`. See [33 oct pkg](../tooling/33-oct-pkg.md).

## Examples

Valid:

```oct
package Main

import Physics

record SolveConfig {
    Method: Physics.Method
}

fn Main() -> Int {
    let cfg = SolveConfig {
        Method: Physics.Method.Euler
    }
    if cfg.Method == Physics.Method.Euler {
        return 0
    }
    return 1
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    return Distance(3, 4)
}
```
