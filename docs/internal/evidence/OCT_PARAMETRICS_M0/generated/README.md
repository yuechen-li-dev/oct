# Generated Go evidence

The backend-boundary test requires concrete names and shapes including:

```text
type ParametricQueryValid_MaterializedFilter__Job struct
type ParametricQueryValid_MaterializedFilter__InventoryItem struct
type __octFlow_ParametricQueryValid_Filtered__Job struct
fn_ParametricQueryValid_FirstWhere__Job
fn_ParametricQueryValid_FirstWhere__InventoryItem
fn_ParametricsM0Valid___oct_selector__Job__ID
fn_ParametricsM0Valid___oct_selector__InventoryItem__SKU
```

`TestParametricM0QueryMIRMatchesHandwrittenConcreteShape` compares the parametric and handwritten FLOW MIR after normalizing only the declaration name; `reflect.DeepEqual` passes. Generated code compiles and runs in the compiled language contracts without manual repair.
