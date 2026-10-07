# JsonIntentRecoveryLab (closed)

Closed 2026-10-07.

This lab asked whether Oct could import JSON by recovering the intent of a
document: lowering it to a raw node graph and scoring candidate shapes
(table, mapping, grid, nested record, tagged array). Its reports, M0 to M4,
are the record of how far that went and why it stopped.

The answer Oct took is the opposite one. The program declares the type, and
`Json.Load<T>(path)` reads the document as that type or says where it does
not fit. Nothing is recovered and nothing is scored. The design and its
reasons are in `internal/json/JSON_V2_LADDER.md`.

What remains here:

- `M0/corpus/`: the seven documents. They are the acceptance corpus of the
  Json library: `Language/Builtins/Json/corpus` loads each one into the
  declarations a person would write for it.
- `M0/recovery/`, `M1/recovery/`: the Oct sources the recovery passes
  produced, as they were.
- `M0` to `M4` `REPORT.md`, and `M0/CI_REPAIR_NOTE.md`: unchanged.

What was removed: `M0/corpus_validation.octest`, which called the builtins
`JsonLoad`, `JsonNormalize` and `JsonLoadStructured<JsonRawGraph>`. Those
builtins, the `IO` JSON functions and the raw-graph records no longer exist.
