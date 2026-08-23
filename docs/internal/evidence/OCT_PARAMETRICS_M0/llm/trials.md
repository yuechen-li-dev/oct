# Fresh-agent authoring trials

Each agent was restricted to repository guidance, public/reference documentation, and an isolated ignored scratch directory. No trial modified production files.

## A: reusable predicate/query helper

The first source passed unchanged for `Job` and `Sensor`: interpreted 1/1 in 13 ms and compiled 1/1 in 688 ms, with no diagnostics or fallback. Six tool turns and a few minutes. No API hallucinations. The agent judged explicit type arguments slightly verbose but the shared structure clear.

## B: keyed dataset and selectors

The final source passed for `Job.ID: String` and `InventoryItem.SKU: Int`: interpreted 1/1 in 8 ms and compiled 1/1 in 669 ms, with no fallback. About four minutes and one correction. The initial `jobPattern.KeySelector(job)` was parsed as package qualification; binding `jobPattern.KeySelector` to a local and calling the local worked. This gap is now documented publicly.

## C: diagnostic-led invalid reuse

Cross-owner selector and predicate attempts produced exact nominal diagnostics, including `expects fn(InventoryItem) -> String, got fn(Job) -> String` and the analogous `Bool` signature. The corrected proof passed 1/1 in both lanes with no fallback. Three corrections over four feedback rounds and about seven minutes: selector owner, predicate owner, then the same record-field call syntax issue.

## Finding

All three agents reached correct, readable programs using only public material. Exact owner/function diagnostics led directly to fixes. Two agents independently found the selector-valued record-field invocation limitation; it is an ordinary call-parser ergonomics issue, not a selector typing failure.
