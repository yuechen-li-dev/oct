# Query and FLOW evidence

`Filtered<T>` is used for `Job` and `InventoryItem`. The Job fact steps the cursor explicitly and proves source order plus `take 2` early completion. The InventoryItem fact proves the same template specializes independently.

```text
interpreted: 2 passed, 0 failed
compiled:    2 passed, 0 failed; compiled cases: 2; fallback: 0
```

The parser already expresses Query-M0 as a FLOW skeleton. Parametric elaboration substitutes that open skeleton before the ordinary typechecker and compiler see it. The backend test requires each concrete specialization to retain the ordinary single `Scan` state.
