# W5 `oct new` package scaffolding implementation

W5 implements deterministic package scaffolding for the no-flag command family designed in W4:

```sh
oct new experiment <Name>
oct new library <Name>
oct new wrapper-library <Name>
```

## Implemented commands

- `oct new library <Name>` creates a pure library package in `./<Name>`.
- `oct new experiment <Name>` creates an experiment package in `./<Name>` with an `M0/` entry milestone and `REPORT.md` placeholder.
- `oct new wrapper-library <Name>` creates a wrapper package in `./<Name>` with package-local sidecar reference files under `sidecars/octxiliary-<kebab>/`.

W5 intentionally adds no flags. There is no `--dir`, `--force`, `--family`, `--sidecar`, lockfile generation, registry publishing, sidecar build lifecycle, package-manager sync lifecycle, interpreted generic wrapper dispatch, `@extern`, or `EXTERNAL { ... }` support in this milestone.

## Name validation

`<Name>` is accepted only when it matches strict M0 PascalCase:

```text
[A-Z][A-Za-z0-9]*
```

Additional rejected names include empty names, names longer than 80 bytes, whitespace, hyphens, underscores, slashes or path separators, dots, colons, `Manifest`, `Main`, built-in scalar/type family names, and top-level command family names.

Invalid names are rejected rather than normalized. For example, `oct-opencv`, `signal_tools`, and `openCV` fail; `OpenCV` succeeds. Derived scaffold stems are deterministic, such as `OpenCV` -> `open_cv` and `open-cv`.

## Filesystem safety

The target is always the current working directory plus the exact package name: `./<Name>`. W5 does not auto-write under `Libraries/` or `Experiments/`.

The target directory must not already exist, even if empty. Scaffolding does not overwrite files. Generated file paths are fixed relative paths under the target directory; file contents use LF endings, final newlines, no timestamps, no absolute paths, and no machine-specific content.

If a write fails after the target directory is created, the scaffolder rolls back the newly-created target directory.

## Wrapper-library sidecar scope

The generated wrapper manifest includes current-compatible wrapper metadata so `oct pkg wrappers` can inspect the package and render deterministic `.octagon` registry output.

The generated raw wrapper function is manifest metadata only in W5. It is not called by generated Oct tests, and `oct new wrapper-library` does not build or run the sidecar. Native sidecar build, dispatch, download, and permission lifecycles remain future work.

The generated sidecar Go module is author reference scaffolding. Authors should update its module path and implementation before publishing.
