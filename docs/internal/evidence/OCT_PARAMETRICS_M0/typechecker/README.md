# Typechecker evidence

Project elaboration replaces every template application with an ordinary concrete declaration before the existing typechecker. A guard in the ordinary checker rejects any unelaborated type argument that crosses that boundary.

The authoritative invalid suite covers missing fields, wrong selector result types, cross-owner selectors, cross-owner predicates, nominally distinct instantiations, and recursive specialization:

```text
go run ./cmd/oct test ./Language/Types/ParametricsM0/invalid --execution interpreted
Result: 6 passed, 0 failed, 0 skipped
go run ./cmd/oct test ./Language/Types/ParametricsM0/invalid --execution compiled
Result: 6 passed, 0 failed, 0 skipped
```

Representative diagnostics include `Selector .SKU does not exist on Job`, `expects fn(InventoryItem) -> Bool, got fn(Job) -> Bool`, and `infinite template instantiation detected`.
