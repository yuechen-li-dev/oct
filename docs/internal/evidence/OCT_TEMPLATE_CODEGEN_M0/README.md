# OCT-TEMPLATE-CODEGEN-M0 evidence

Verdict: Success.

The focused compiler regression in `internal/build/database_templates_m0_test.go` establishes the requested chain on the real compiler path:

1. `FilteredView<Job>` and the semantically equivalent handwritten `BespokeFilteredView` lower to structurally identical MIR after nominal FLOW-name normalization.
2. Their independently emitted FLOW core and public facade are byte-identical after normalizing the nominal symbol and checkpoint fingerprint.
3. The fingerprint is intentionally not made identical in production: it includes package/FLOW identity for checkpoint compatibility. Template provenance is comment-only and cannot affect execution.
4. Per-package declarations are name-sorted before lowering; five independent reload/lower/emit passes and two W5 generation runs produce identical Go bytes.
5. Go `-gcflags=-m=2` output gives both lanes the same constructor/core/facade inlining and escape decisions, with zero benchmark allocations.

W5 result parity passes for limits 1, 10, 2500, and 5000. The old layout reported template 11.7% slower. The final rerun after moving only benchmark helpers produced forward-order medians of 66,754 ns/op bespoke and 60,708 ns/op template, while reverse order produced 66,398 and 60,630 ns/op. The reversal of the alleged penalty without any generated-code semantic change demonstrates code/benchmark-layout sensitivity; it is not evidence that templates are faster. There is no runtime cost attributable to template codegen.

Reproduce from Oct:

```powershell
go test ./internal/build -run TestDatabaseTemplatesM0GeneratedBoundary -v
go test ./...
go vet ./...
```

Reproduce W5 from OctetDB using the commands in `docs/product/evidence/OCT_DB_TEMPLATES_M0/README.md`.
