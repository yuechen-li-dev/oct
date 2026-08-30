# Atlas Library

Atlas is documentation as code: ordinary immutable Oct records describe stable
Authorities, Citations, Requirements, Interpretations, Claims, and links to
compiler-known symbols, Facts, Theories, and Artifacts.

A package opts in by defining one zero-argument function returning either the
row-oriented `Atlas.Document` or columnar `Atlas.TableDocument`. The table form
uses ordinary `record table` values for homogeneous node/edge batches; tables
support immutable column `with` updates, and the document/policy records support
ordinary record `with`. `oct atlas` evaluates the function only as build-time project data.
Atlas metadata does not affect ordinary runtime behavior or code generation.

Edges use subject-to-object grammar, for example:

```text
Interpretation --Interprets--> Citation
Function       --Implements--> Requirement
Fact           --Verifies----> Requirement
Claim          --DerivedFrom-> Claim
Artifact       --Explains----> Claim
```

Atlas records project claims. A citation does not make a source true, a passing
Fact does not establish legal or scientific truth, and graph consistency is not
proof of real-world correctness. External URI content is mutable; use `Version`,
`Digest`, or a project-vendored source when reproducibility requires it. Atlas
never fetches authority content.
