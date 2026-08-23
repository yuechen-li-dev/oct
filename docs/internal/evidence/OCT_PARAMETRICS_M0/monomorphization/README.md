# Monomorphization evidence

`internal/project/parametric.go` performs consumer-side, project-level AST elaboration. The canonical key combines declaration package/name, kind, consumer package, and canonical concrete type arguments. Identical applications reuse one specialization. Open templates are removed from the executable program and concrete declarations retain `TemplateOrigin` provenance.

The backend-boundary test observes two record, two function, and two FLOW instantiations for the database proof even though applications repeat. It also rejects generated output containing `TemplateRuntime`, `GenericDictionary`, or `TypeArguments`.

Imported declaration proof:

```text
go run ./cmd/oct test ./Language/Types/ParametricsM0/packages --execution interpreted
Result: 1 passed
go run ./cmd/oct test ./Language/Types/ParametricsM0/packages --execution compiled
Result: 1 passed; compiled: 1; fallback: 0
```
