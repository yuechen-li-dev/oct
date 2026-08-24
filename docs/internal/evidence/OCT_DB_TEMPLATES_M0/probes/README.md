# Original-blocker rerun

The previous OCT-DB-TEMPLATES-M0 probe output is retained in the OctetDB repository at `docs/product/evidence/OCT_DB_TEMPLATES_M0/probe-results.txt`. It established the control: the suffix and concrete `with` worked, while generic declarations, cross-record predicate reuse, and typed selectors did not.

| Semantic case | Prior result | Parametrics-M0 rerun |
| --- | --- | --- |
| generic record over `Record, Key` | parser rejected `<` | `StableIdentity<Record,Key>` and `BoundedKeyedDataset<Record,Key>` elaborate for both Job/String and InventoryItem/String |
| predicate retargeting Job → Item | exact nominal function mismatch | `MaterializedFilter<Record>` substitutes `fn(Record) -> Bool`; wrong-owner predicates still fail |
| typed `.ID` / `.SKU` selectors | selector expression rejected | both resolve as `Selector<Record,String>` with concrete `FieldRef` owner/ordinal provenance |
| parametric query source/result | unavailable | `FilteredView<Record>` lowers to concrete FLOW for Job and InventoryItem |
| cross-application monomorphization | unavailable | the core catalog test creates distinct Job and Inventory specializations without fact or selector leakage |

Commands verified on 2026-08-23:

```text
go run ./cmd/oct test Libraries/DatabaseTemplates --execution interpreted
go run ./cmd/oct test Libraries/DatabaseTemplates --execution compiled
```

Both lanes passed 3 valid facts and 11 compile-fail contracts; compiled execution reported 3 compiled cases and zero interpreted fallbacks. Four of the final failures specifically exercise refined Concepts: non-positive bound, non-positive limit, empty publication identity, and negative publication version.
