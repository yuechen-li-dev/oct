# JSON Intent Recovery Lab — M2 Report

## 1) Scope and goal

M2 evaluates **table representation choice** after M1’s detection/classification pass.

Question under test:

> Once JSON is identified as table-shaped, what should be the canonical recovered Oct representation?

Compared representations:

1. **Row-oriented** (`Row[]`, where each row is a record)
2. **Columnar** (record of aligned arrays)
3. **Nested arrays** (`T[][]` / positional rows)

M2 corpus scope follows M1’s relevant table-bearing examples:

- people table
- dispatch/mapping tables
- optional/sparse tickets table
- matrix/grid
- UI/layout-derived tables
- tagged operation tables

`example_02_config` remains a true compositional record from M1 and is out-of-scope for table representation comparison.

## 2) Evaluation rubric (applied uniformly)

Each representation is evaluated on:

1. Readability (human + LLM + diffs)
2. Schema clarity (explicit columns/types/self-description)
3. Optional/sparse handling
4. Deterministic recovery from JSON
5. Transformability (filter/projection/join/reshape)
6. Alignment with Oct philosophy (explicit structure, typed intent, avoid positional ambiguity)

Scale:

- **Strong** = preferred
- **Acceptable** = workable with caveats
- **Weak** = should not be canonical default

---

## 3) Per-example comparison

## 3.1 `example_01_people` (homogeneous person rows)

### Representations

**Row-oriented**

```oct
record Person {
    Id: String
    Name: String
    Role: String
    Active: Bool
}

Person[] {
    Person { Id: "u-100" Name: "Avery Chen" Role: "analyst" Active: true }
    Person { Id: "u-101" Name: "Mina Patel" Role: "designer" Active: false }
    Person { Id: "u-102" Name: "Jon Rivera" Role: "analyst" Active: true }
}
```

**Columnar**

```oct
record PeopleTable {
    Ids: String[]
    Names: String[]
    Roles: String[]
    Active: Bool[]
}

PeopleTable {
    Ids: ["u-100", "u-101", "u-102"]
    Names: ["Avery Chen", "Mina Patel", "Jon Rivera"]
    Roles: ["analyst", "designer", "analyst"]
    Active: [true, false, true]
}
```

**Nested arrays**

```oct
String[][] {
    ["u-100", "Avery Chen", "analyst", "true"]
    ["u-101", "Mina Patel", "designer", "false"]
    ["u-102", "Jon Rivera", "analyst", "true"]
}
```

### Evaluation

- Row: Readability **Strong**, schema clarity **Strong**, optional handling **Acceptable**, deterministic recovery **Strong**, transformability **Strong**, Oct alignment **Strong**.
- Columnar: Readability **Acceptable**, schema clarity **Strong**, optional handling **Strong**, deterministic recovery **Strong** (when keys stable), transformability **Strong** (column ops), Oct alignment **Strong**.
- Nested arrays: Readability **Weak**, schema clarity **Weak**, optional handling **Weak**, deterministic recovery **Acceptable** (only as tuple inference), transformability **Weak**, Oct alignment **Weak**.

### Decision

- **Best:** Row-oriented (for plain entity table readability).
- **Acceptable:** Columnar.
- **Reject as canonical:** Nested arrays (positional ambiguity + type erosion).

---

## 3.2 `example_03_dispatch` (mapping tables)

### Representations

**Row-oriented**

```oct
record HandlerRoute { Event: String Handler: String }
record RetryPolicy { Event: String Retries: Int }

HandlerRoute[] {
    HandlerRoute { Event: "user.created" Handler: "handle_user_created" }
    HandlerRoute { Event: "user.deleted" Handler: "handle_user_deleted" }
    HandlerRoute { Event: "invoice.paid" Handler: "handle_invoice_paid" }
    HandlerRoute { Event: "invoice.failed" Handler: "handle_invoice_failed" }
}

RetryPolicy[] {
    RetryPolicy { Event: "invoice.failed" Retries: 5 }
    RetryPolicy { Event: "user.deleted" Retries: 1 }
}
```

**Columnar**

```oct
record StringDispatchTable { Keys: String[] Values: String[] }
record RetryPolicyTable { Events: String[] Retries: Int[] }
```

**Nested arrays**

```oct
String[][] {
    ["user.created", "handle_user_created"]
    ["user.deleted", "handle_user_deleted"]
    ["invoice.paid", "handle_invoice_paid"]
    ["invoice.failed", "handle_invoice_failed"]
}

Int[][] { [5], [1] } // retries lose event linkage unless parallel table maintained
```

### Evaluation

- Row: **Strong** readability/clarity, **Strong** deterministic recovery from key-value maps (after map-to-pairs), **Strong** transformability.
- Columnar: **Strong** for exact-key declarations and direct Storefront-style dispatch/mapping tables; **Strong** sparse handling (if policy columns added).
- Nested arrays: **Weak** because key/value meaning is positional and cross-table linkage becomes brittle.

### Decision

- **Best:** Columnar for canonical dispatch/mapping declaration.
- **Acceptable:** Row-oriented key/value pairs.
- **Reject:** Nested arrays.

### Special-case verdict (A: mapping tables)

- `record-of-arrays` and `row pairs` are both valid.
- Plain map literal recovery is useful intermediate form, but canonical recovered table should be explicit typed table form.

---

## 3.3 `example_05_optional` (sparse ticket table)

### Representations

**Row-oriented**

```oct
record TicketRow {
    Id: String
    Title: String
    Priority: String
    AssigneePresent: Bool
    Assignee: String
    DueDatePresent: Bool
    DueDateIso: String
    SprintPresent: Bool
    Sprint: String
}
```

**Columnar**

```oct
record TicketTable { Ids: String[] Titles: String[] Priorities: String[] }
record TicketOptionalColumns {
    AssigneePresent: Bool[]
    AssigneeValue: String[]
    DueDatePresent: Bool[]
    DueDateIso: String[]
    SprintPresent: Bool[]
    SprintValue: String[]
}
```

**Nested arrays**

```oct
String[][] {
    ["T-9001", "Login page timeout", "high", "sam", "", "S24"]
    ["T-9002", "Update FAQ links", "low", "", "", ""]
    ["T-9003", "Billing alert copy", "medium", "nora", "2026-05-10", ""]
}
```

### Evaluation

- Row: **Acceptable**, but verbose and repeats sparse semantics per row.
- Columnar: **Strong**; sparse columns are explicit and deterministic without null-type coupling.
- Nested arrays: **Weak**; missing-value intent is ambiguous and column meaning implicit.

### Decision

- **Best:** Columnar split base+optional tables.
- **Acceptable:** Row form with explicit presence flags.
- **Reject:** Nested arrays.

### Special-case verdict (C: optional/sparse)

- Sparse data should default to columnar with explicit presence/value columns.
- Row form is second-best when downstream operations are row-centric.

---

## 3.4 `example_04_matrix` (rectangular numeric grid)

### Representations

**Row-oriented**

```oct
record SensorCell { Row: Int Col: Int Value: Float }
SensorCell[] { /* 12 rows */ }
```

**Columnar**

```oct
record SensorGridTable { Rows: Int Cols: Int Values: Float[] }
record SensorPlacementTable { Row: Int[] Col: Int[] }
```

**Nested arrays**

```oct
Float[][] {
    [21.5, 21.8, 22.0, 21.9]
    [21.7, 21.9, 22.1, 22.0]
    [21.6, 21.7, 21.9, 21.8]
}
```

### Evaluation

- Row: **Acceptable** for relational transforms, but noisy for inherently rectangular data.
- Columnar: **Strong** when paired with explicit placement arrays and dimensions.
- Nested arrays: **Strong** when matrix semantics are primary and positional meaning is intrinsic (row/col indices).

### Decision

- **Best:** Nested arrays for canonical numeric grid literal, with explicit dimension metadata when needed.
- **Acceptable:** Columnar flattened + placement for tabular tooling.
- **Conditionally acceptable:** Row cells for join-heavy workflows.

### Special-case verdict (B: rectangular numeric grids)

- Grids are the one major case where positional arrays are naturally semantic, not accidental.

---

## 3.5 `example_06_ui_like` (layout-derived tables)

### Representations

**Row-oriented**

```oct
record SectionPlacementRow { Kind: String Slot: String }
SectionPlacementRow[] {
    SectionPlacementRow { Kind: "hero" Slot: "top" }
    SectionPlacementRow { Kind: "product_grid" Slot: "main" }
    SectionPlacementRow { Kind: "banner" Slot: "footer" }
}
```

**Columnar**

```oct
record SectionPlacementTable { Kind: String[] Slot: String[] }
record ProductCatalogTable { Sku: String[] Label: String[] Price: Float[] }
```

**Nested arrays**

```oct
String[][] {
    ["hero", "top"]
    ["product_grid", "main"]
    ["banner", "footer"]
}
```

### Evaluation

- Row: **Strong** readability, especially for authored UI placement.
- Columnar: **Strong** when aligning with Storefront table-first data segments (`Placement`, `Data`).
- Nested arrays: **Weak** because slot/kind semantics are critical and should be named.

### Decision

- **Best:** Columnar for canonical recovered storage, with row view acceptable for authored readability.
- **Acceptable:** Row-oriented placement rows.
- **Reject:** Nested arrays.

### Special-case verdict (D: UI/layout)

- Placement and catalog data should keep explicit named columns; positional arrays obscure intent and hurt maintainability.

---

## 3.6 `example_07_tagged` (tagged operation tables)

### Representations

**Row-oriented**

```oct
record CreditRow { AccountId: String Amount: Float Currency: String }
record DebitRow { AccountId: String Amount: Float Currency: String Merchant: String }
record TransferRow { FromAccount: String ToAccount: String Amount: Float Currency: String }
```

**Columnar**

```oct
record CreditTable { AccountId: String[] Amount: Float[] Currency: String[] }
record DebitTable { AccountId: String[] Amount: Float[] Currency: String[] Merchant: String[] }
record TransferTable { FromAccount: String[] ToAccount: String[] Amount: Float[] Currency: String[] }
```

**Nested arrays**

```oct
String[][] {
    ["credit", "A-100", "250.0", "USD"]
    ["debit", "A-100", "49.5", "USD", "Metro Cafe"]
    ["transfer", "A-100", "A-210", "30.0", "USD"]
}
```

### Evaluation

- Row: **Strong** for variant-specific readability.
- Columnar: **Strong** for deterministic per-tag subtable recovery and typed alignment.
- Nested arrays: **Weak** due to variant-dependent positional schemas and mixed arity.

### Decision

- **Best:** Columnar per-tag subtables (M1 choice remains strongest for canonical recovery).
- **Acceptable:** Row form per variant.
- **Reject:** Single positional nested-array supertable.

---

## 4) Cross-example synthesis

## 4.1 Canonical representation rules

1. **Use columnar tables by default** for recovered JSON tables with named fields and stable columns.
2. **Use row tables when author readability/editability dominates** and sparsity is low/moderate.
3. **Use nested arrays only when positional semantics are intrinsic** (rectangular numeric grids / matrix-like data).
4. **Avoid nested arrays for business/entity tables**, mapping tables, sparse corpora, and UI placement data.

## 4.2 Deterministic decision rules for importer (M3-ready)

1. If JSON source has named object keys per row and stable key set -> **recover columnar**.
2. If table is sparse/optional-heavy -> **recover columnar with presence/value split**.
3. If source is object map (`key -> scalar/simple payload`) -> **recover mapping table** (columnar `Keys/Values` or explicit row pair).
4. If source is rectangular numeric nested arrays with consistent row length -> **recover nested arrays** (optionally alongside dimension metadata).
5. If discriminator/tag splits row schema -> **recover per-tag subtables** (prefer columnar).
6. If structure is semantically compositional singleton object -> **do not table-normalize** (stay record).

## 4.3 Storefront alignment answer

**Was Storefront’s final columnar representation already correct?**

**Mostly yes, with one refinement.**

- Yes: For dispatch, placement, and catalog-style authored corpora, Storefront’s columnar/table-first split remains the best canonical recovered form.
- Refinement: Rectangular numeric grids are a valid exception where nested arrays can be canonical due to intrinsic positional semantics.

## 4.4 Final recommendation for M3/importer default

> **M3 should default to columnar recovered tables for table-shaped JSON, with a deterministic exception for true rectangular numeric grids where nested arrays are canonical.**

Operationally:

- default: columnar
- acceptable alternate view: row-oriented
- constrained exception: nested arrays for intrinsic positional grids only

---

## 5) Language/reference consistency note

`Language/reference/language/07-arrays.md` explicitly documents nested arrays (`T[][]` and deeper), so M2’s constrained nested-array use for matrix/grid cases is language-consistent.

No `Dynamic` or null-based type semantics were introduced in M2; sparse handling remains explicit via presence/value columns.
