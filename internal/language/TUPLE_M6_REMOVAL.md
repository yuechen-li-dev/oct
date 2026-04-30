# Tuple M6 — Public Tuple API Removal

Tuple M1–M5 were experimental and are superseded by M6.

## Why public tuple API was rejected

Tuple return/destructuring syntax was introduced to support Go-style random API experiments. That inverted Oct architecture: user-facing language design was being pulled by implementation convenience.

## Why records are the correct Oct surface

For heterogeneous values, Oct uses named records. Records provide explicit field meaning and stable evolution without positional ambiguity. Arrays remain the homogeneous sequence abstraction.

## Go/internal multi-value vs Oct language

Go helpers may continue using native multi-value returns (`value, err`, `[]byte, error`) strictly as implementation details. This does not imply an Oct tuple surface.

## Removed/disabled user-facing tuple features

- Tuple return type syntax (`-> (A, B)`) is parser-rejected.
- Destructuring assignment (`a, b = expr`) is parser-rejected.
- Tuple probe builtins (`TupleProbe`, `BoolIntProbe`) are no longer public builtins.
- Tuple language tests were replaced with negative tests that lock rejection behavior.

## What remains internally

Internal Go code may still return multiple Go values where needed (e.g., runtime helpers, crypto/random plumbing). These are not constructible/assignable from Oct syntax.

## Random API direction

Random APIs should converge on record-return shapes for heterogeneous results (for example `{ Next, Value }`) rather than tuple threading syntax.
