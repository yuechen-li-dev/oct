# Selector evidence

`Selector<R, F>` is validated against an exact nominal record owner and field result type. Elaboration creates an exact getter `fn(R) -> F`. MIR selector metadata uses the existing `layoutcontract.FieldRef`, including `nominal-record` subject kind, package-qualified identity, ordinal, and field name.

`TestParametricM0ErasesBeforeOrdinaryFlowAndGoLowering` asserts:

```text
ID  -> nominal-record:ParametricsM0Valid.Job
SKU -> nominal-record:ParametricsM0Valid.InventoryItem
```

It also audits generated Go for concrete direct-access getters. No selector string lookup or field enumeration exists.
