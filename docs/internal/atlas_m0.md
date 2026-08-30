# Atlas M0 architecture and dogfood record

Atlas M0 adds a domain-neutral semantic documentation graph without changing Oct
runtime semantics. Ordinary Oct records own authored knowledge; the Go tooling
layer owns language identity, validation, serialization, and traversal.

## Frozen M0 surface

- Nodes: Authority, Citation, Requirement, Interpretation, Claim, SymbolRef,
  EvidenceRef, ArtifactRef.
- Edges: Supports, Interprets, Implements, Verifies, DerivedFrom, DependsOn,
  Supersedes, Explains, Produces, Contradicts.
- Discovery: one `AtlasDocument()` returning row-oriented `Atlas.Document` or
  columnar `Atlas.TableDocument` in the target package.
- Product: deterministic `atlas.octagon`, separate from evidence-run state.
- Queries: build, verify, show, explain, affected-by, coverage.
- IDs: project-local, case-sensitive dot-separated ASCII semantic identities.

The library surface survived both policy and mathematical-claim authoring without
domain-specific node or relation kinds. Concepts remain value admission, Facts
and Theories remain executable tests, Artifacts remain output producers, and
Atlas only links them.

Policy Lab dogfoods the columnar form and configures `AdvisoryPolicy()` with an
ordinary immutable record `with`. Library contracts also update a record table
column with `with` and prove the source table remains unchanged in interpreted
and compiled execution. Riemann retains row form. The measured authoring verdict
is mixed but useful: columnar tables materially reduce repeated constructors for
node batches, while row records keep short edge lists easier to review. Both
compile to the same graph model, so no Atlas-specific syntax is justified.

## Policy Lab migration

Policy Lab's handwritten `SourceMapTable`, `BuildSourceMap`, duplicate source-map
Fact, and generated source-map table were removed. `AtlasDocument` now links the
FLSA and FOIA Authorities/Citations through explicit bounded Interpretations and
Requirements to a function, Concept, FLOW, Facts, Theory, and audit Artifact.
Runtime decision provenance and trace values remain because they explain a
specific execution; Atlas replaces project-level provenance plumbing, not useful
runtime result data.

`affected-by Citation.US.FLSA.207a1` reaches the interpretation, requirement,
implementation symbols, verifier Fact/Theory, claims, and audit Artifact. This is
the amendment-impact behavior the handwritten source map could not compute.

## Riemann handoff slice

The M23 local optimizer, M24 outside-window exclusion, M25 B1 control, and named
contact-preserving two-atom component optimum form a useful `DerivedFrom` and
`DependsOn` explanation. The graph explicitly says the component result is not a
proof of the Riemann Hypothesis.

The local Riemann checkout was inspected at commit
`7afc3c35a50610459a63088e88a91ebe86aa7d79`. M23, M24, and M25 reports are
commit-pinned Authorities with exact SHA-256 digests and section Citations. The
Riemann implementation, Go certificates, and separate Oct experiment packages
are not part of the loaded Atlas project. M0 does not fabricate SymbolRef or
EvidenceRef nodes: those would correctly fail compiler resolution. This isolates
package/cross-repository federation as a future need without weakening M0
verification.

## Integration verdicts

- Concepts: ordinary Atlas links to Concept symbols are sufficient (Integration B).
- Facts and Theories: ordinary EvidenceRef links are sufficient for static M0;
  native metadata is not yet justified.
- Artifacts: ordinary ArtifactRef plus logical output is sufficient; automatic
  registration would broaden the Artifact protocol without demonstrated need.
- Templates: author-level template identity can be referenced as a function or
  record; indexing every specialization is deliberately excluded.
- Annotation syntax: not warranted. Optional columnar tables remove most repeated
  constructors; row form remains transparent for short graphs, and both worked
  in dogfood.

## Deliberate limits

Atlas is not a prose generator, graph database, web service, GUI, reflection
system, legal DSL, theorem prover, runtime state, control-flow mechanism, Concept
replacement, Fact/Theory redesign, truth authority, or global ID registry.
Narrative motivation and ambiguity remain prose; Atlas owns addressable identity,
relationships, provenance, implementation linkage, evidence linkage, and change
impact.

The one bounded M0 integration gap is volatile test-result consumption. Static
Fact/Theory identity is compiler-verified and explanations honestly show `not
run`. A later milestone may consume an explicit existing result manifest without
running the suite or embedding timestamps in canonical Atlas output.
