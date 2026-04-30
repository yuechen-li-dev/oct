# Oct Standard Libraries — M6 Enum + Switch/Match Domain Modeling

Date: 2026-04-30

## Search patterns used

- `rg -n "Kind: String|Mode: String|Status: String|Type: String" Libraries -g "*.oct"`
- `rg -n '"absolute"|"anchored"|"null"|"array"|"object"|"number"|"string"|"bool"' Libraries -g "*.oct" -g "*.octest" -g "*.md"`
- `rg -n "kind|mode|status|type" Libraries -i -g "*.oct" -g "*.octest" -g "*.md"`

## Candidates found

- `Libraries/UI/UI.Layout.oct`: `UIBox.Kind: String` with closed alternatives `"absolute"` / `"anchored"` (**high confidence internal domain**).
- `Libraries/IO/IO.Json.oct`: `JsonRawGraphNode.Kind: String` with JSON-structural tags (`"null"`, `"array"`, `"object"`, `"number"`, `"string"`, `"bool"`) (**boundary sensitive**).

## Migrations performed

1. Tests-first shape update for UI layout kind domain:
   - updated/added tests to assert enum-based box kind behavior via switch consumption.
2. `UIBox.Kind` migrated from `String` to `BoxKind` enum in `UI.Layout`.
3. `Place(...)` now dispatches via `switch box.Kind` rather than string comparison.
4. Added `.octfail` contract to reject legacy string tags for `UIBox.Kind`.

## Candidates intentionally deferred

- `IO.Json` raw graph kind tags were intentionally deferred in M6.
- Reason: this is an interchange/compatibility boundary and broad migration risk is medium; preserving existing boundary strings avoids format/API breakage in this pass.

## Boundary compatibility notes

- JSON parse/lower/import behavior remains string-compatible in M6.
- `Libraries/IO/README.md` now explicitly records the deferred enum-adapter plan.

## Language/reference consistency note

- M6 uses `enum` + `switch` as documented in `Language/reference/language/04-control-flow.md` and record usage in `Language/reference/language/11-records.md`.
- Documentation gap observed: enum-specific reference content location was not found at `Language/reference/language/08-enums.md` during this pass; this should be tracked separately as reference organization gap.

## Validation results

- `go test ./...`
- `go run ./cmd/oct test Libraries/UI`
- `go run ./cmd/oct test Libraries/IO`
- `go run ./cmd/oct test Libraries`
