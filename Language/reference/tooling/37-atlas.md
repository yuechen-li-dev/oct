# Atlas semantic documentation graphs

Atlas is an opt-in build-time tooling layer for documentation as code. It links
ordinary semantic code and executable evidence without changing either one.

## Authoring and discovery

Import the ordinary `Atlas` library and define exactly one package-local entry:

```oct
import Atlas

fn AtlasDocument() -> Atlas.Document {
    return Atlas.Document {
        Authorities: [...]
        Citations: [...]
        Requirements: [...]
        Interpretations: [...]
        Claims: [...]
        Symbols: [...]
        Evidence: [...]
        Artifacts: [...]
        Links: [...]
        Policy: Atlas.Policy { ... }
    }
}
```

Construction is immutable ordinary Oct evaluation. There are no registration
calls, Atlas attributes, reflection scans, parser modes, or runtime registries.
Projects without `AtlasDocument` remain ordinary Oct projects.

For larger homogeneous collections, `Atlas.TableDocument` is the optional
columnar form. Its fields use `AuthorityTable`, `CitationTable`,
`RequirementTable`, `InterpretationTable`, `ClaimTable`, `SymbolRefTable`,
`EvidenceRefTable`, `ArtifactRefTable`, and `LinkTable`, all ordinary `record
table` values. Table extent/type validation and immutable column `with` updates
are exactly the language rules in [Records](../language/11-records.md). The
regular `Policy` and `TableDocument` values also support ordinary record `with`.
The compiled graph is identical in structure regardless of row or columnar
authoring; source provenance naturally points into the chosen layout.

Columnar form reduces repeated constructors for medium-sized node families.
Row form is often easier to review for short heterogeneous-looking relationship
lists because each edge stays visually together. M0 keeps both and adds no
Atlas-specific table or update semantics.

## Identity and references

Atlas IDs are case-sensitive, dot-separated, project-local semantic identities.
Each segment contains ASCII letters, digits, `_`, or `-`. Identity never uses
file order, source span, map iteration, memory addresses, or generated hashes.

`SymbolRef` supports functions, Concepts, records, enums, and FLOW declarations.
`EvidenceRef` supports existing `[Fact]` and `[Theory]` functions. `ArtifactRef`
supports existing `[Artifact]` functions plus a relative logical output name.
All three resolve through the compiler's package/declaration model; unknown or
wrong-kind references are hard errors. Source spans are provenance, not identity.

## Edge direction

Links use semantic subject-to-object grammar:

```text
Interpretation --Interprets--> Citation or Authority
Symbol         --Implements--> Requirement or Claim
Evidence       --Verifies----> Requirement or Claim
Claim          --DerivedFrom-> Claim
Artifact       --Explains----> semantic node
```

The closed M0 vocabulary is `Supports`, `Interprets`, `Implements`, `Verifies`,
`DerivedFrom`, `DependsOn`, `Supersedes`, `Explains`, `Produces`, and
`Contradicts`. Contradictions are preserved, not resolved. Supersession preserves
old nodes. `Supersedes` and `DerivedFrom` cycles are rejected; `DependsOn` cycles
remain reportable relationships because their domain meaning is not universally
invalid.

## Commands

```text
oct atlas build [project] [--out atlas.octagon]
oct atlas verify [project]
oct atlas show <ID> [project]
oct atlas explain <ID> [project] [--depth N] [--out report.md]
oct atlas affected-by <ID> [project] [--depth N]
oct atlas coverage [project]
```

`build` writes canonical Octagon with nodes sorted by stable ID and edges sorted
by `(From, Relation, To)`. `explain` is deterministic Markdown assembled only
from node text, relationships, compiler-known references, and evidence status.
Traversal defaults to depth four and has no heuristic path ranking.

`verify` checks IDs, endpoints, citation authorities, resolved references,
relation categories, self-supersession, cycles, and optional project policies.
Coverage reports implementation/verifier/support/citation coverage, orphans,
contradictions, and superseded nodes.

## Runtime and trust boundary

Atlas is evaluated only by `oct atlas`. Ordinary `oct run` and `oct build` do not
discover or carry the graph, so Atlas metadata is non-semantic with respect to
ordinary runtime behavior.

Atlas records what a project claims. Citation existence does not establish source
truth; a passing Fact does not establish legal or scientific truth; a consistent
Atlas graph does not prove real-world correctness. Compilation never fetches a
URI. Use `Version`, an explicit digest, or a local vendored source when mutable
external content needs reproducible identity. A relative local Authority URI is
hashed with SHA-256 when available and no digest was authored.

Static graph state and volatile evidence-run state are distinct. M0 resolves
Fact/Theory identities and reports them as `not run`; consuming a separately
produced test result manifest is the deliberately bounded missing integration.

Atlas IDs are project-local in M0. Imported-package and cross-repository graph
federation is future work; M0 never accepts unresolved string references as a
substitute.
