# SDSL-V fixtures

This directory owns SDSL-V language specimens and conformance inputs. It is part of the language contract corpus, rather than a reader-facing example collection. The [SDSL-V language specification](../../docs/SDSL_V_LANGUAGE_SPEC.md) describes the current surface and [workspace guide](../../docs/SDSL_V_WORKSPACE.md) describes the surrounding tooling.

- `M*/` directories retain milestone source, test, benchmark, and canonical artifact fixtures.
- `conformance/` contains the registered valid and invalid cases plus its manifest and pinned graphics artifacts.
- `AttentionSpacePoc/` contains the attention-space comparison inputs.

Fixture paths participate in stable IDs and replay identities. Moving a specimen requires updating its registered path and derived identities, then running the SDSL-V Go test packages.
