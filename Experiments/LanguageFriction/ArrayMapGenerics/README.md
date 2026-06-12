# Array/Map/Generics friction probes

This directory contains non-normative F1 audit probes for everyday scientific data-shaping friction.
The `.octest` files are runnable probes that demonstrate what works today. F3b replaces the earlier first/drop/window manual-loop probe with `Array.CrossSection`, while map/filter/reduce/zip and matrix row/column probes intentionally keep manual loops as friction evidence.
The `expected_fail/*.oct.disabled` files are intentionally unsupported design probes and are not part of the normal language test corpus.

Run the working probes with:

```sh
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution interpreted
go run ./cmd/oct test Experiments/LanguageFriction/ArrayMapGenerics --execution auto
```

A strict `--execution compiled` run is expected to expose any remaining compiled-support gaps for this audit pack; `Array.CrossSection` itself is implemented in interpreted and compiled paths.

Current unsupported-probe diagnostics recorded during F1:

- `slice_syntax_not_supported.oct.disabled`: `expected ']' after index expression ... near ":"`; use `Array.CrossSection(xs, 1..3)` instead of colon or bracket range slicing.
- `map_literal_not_supported.oct.disabled`: `expected statement ... near "{"`.
- `user_defined_generic_function_not_supported.oct.disabled`: `expected '(' after function name ... near "<"`.
- `string_from_enum_not_supported.oct.disabled`: `String.From<T> supports Int, Float, Bool, and String in M0`.
- `matrix_slice_not_supported.oct.disabled`: same slice parser family as array slices; matrix element access currently accepts concrete indices, not range components.
