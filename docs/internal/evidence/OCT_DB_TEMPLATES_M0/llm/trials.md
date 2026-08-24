# Fresh-agent catalog trials

Four isolated agents received only the public catalog/docs, an application requirement, and a short profile. Scratch programs lived under ignored `.tmp/OCT_DB_TEMPLATES_M0/llm`; this file preserves the durable measurements.

| Trial | Selection/result | Invalid/recovery | Human corrections | LOC (physical/nonblank) | Time | Backend hallucinations |
| --- | --- | --- | ---: | ---: | ---: | ---: |
| jobs | correct JobQueue + bounded identity + finite state + materialized/read-mostly publication + FilteredView; no Inventory | one exact-typed assertion correction | 0 | 112/100 including final contracts import | about 10 min | 0 |
| inventory | correct Inventory + bounded identity + materialized/read-mostly publication + FilteredView; no queue lifecycle | assertion mismatch, then direct call of a configuration-held function was unsupported; recovered through FilteredView | 0 | 93/84 including final contracts import | 2m26s | 0 |
| webhook | correctly selected default OctetDB and authored no specialization | none | 0 | 0 Oct | one turn | 0 |
| invalid owner | Inventory identity deliberately received `fn(Job)->String`; diagnostic named the concrete template field and expected/actual owner | corrected in one pass; interpreted and compiled passed | 0 | 49/43 invalid, 51/46 valid | one turn | 0 |

All executable final artifacts passed interpreted and compiled lanes with zero fallback. After the catalog gained refined Concept fields, the executable trials were revalidated with the documented explicit `DatabaseTemplateContracts` import; selection and application configuration were unchanged. The two recurring friction points are catalog usability findings: generic test assertions require exact types, and configuration-held callbacks cannot always be invoked directly even though they remain exactly typed and work through the selected query. These did not require compiler-internal coaching or invalidate the compositions.
