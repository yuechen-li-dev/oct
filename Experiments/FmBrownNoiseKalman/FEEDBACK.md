# FM Brown-Noise Kalman M1 Attempt Notes

Blocked while implementing M1 artifacts due to unresolved Oct string escape/line-construction behavior for deterministic JSON/markdown emission in this experiment scope.

Meaningful progression completed:
- identified that `WriteOctagon` is stable for structured output
- identified that `IO.WriteLines` is likely the safest path for bounded CSV/report emission without escape syntax ambiguity

Next blocker to isolate:
- minimal language contract under `Language/reference` and/or dedicated `.octest` proving newline and quote escaping semantics in string literals for robust JSON/text artifact composition.
