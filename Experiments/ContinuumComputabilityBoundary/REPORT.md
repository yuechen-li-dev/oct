# Continuum Computability Boundary

## Overview
This experiment determines whether discretization is actually needed yet for tiny continuum mechanics workflows, identifies the first missing layer after current continuum mechanics support, and distinguishes representation vs problem specification vs computation.

This experiment was migrated from `Language/Mechanics/ContinuumBoundaryM0/...` into its canonical experiment home under `Experiments/ContinuumComputabilityBoundary/M0/`.

## Milestones
- M0: problem-specification boundary probe
- M1: computational lowering probe (finite sites, no mesh/no solver)
- M2: site-local constraint residual relation (no coupling, no solve)
- M3: minimal site-to-site coupling probe (explicit pairs, no mesh/no solve)

## M0

### Tiny problem chosen
A minimal 2D small-strain solid mechanics problem was chosen:

- rectangular 2D body (`Width = 2.0`, `Height = 1.0`)
- isotropic linear elastic material from (`E`, `nu`)
- plane stress assumption
- one unknown displacement field `u`
- constant body force `(0, -1000)`
- one displacement constraint (left edge fixed)
- governing relation intent: linear momentum balance
- query intents:
  - `ResidualAtCandidateField`
  - `SolveForField`

This is a cantilever-like boundary-value statement, intentionally without any finite lowering.

### Problem objects introduced
Using ordinary Oct records/enums/arrays only:

- root problem object: `ContinuumProblem2D`
- supporting records:
  - `Body2D`
  - `MaterialModel`
  - `UnknownField2D`
  - `Load2D`
  - `Constraint2D`
- probe result type: `BoundaryProbeResult`
- explicit tags are represented as constrained strings in this M0 probe (`Assumption`, `LoadKind`, `ConstraintKind`, `GoverningRelation`, `Query`, `Category`, `Missing`) because enum-in-record fields are not currently supported in this layer.

No parser, DSL, mesh, basis, assembly, solver, or numerical integration machinery was added.

### Layer split

#### A) Representation layer
Still directly expressible in existing continuum substrate:

- `SymGrad(u)`
- isotropic constitutive map `StressFromMaterial(...)`
- strong-form residual slice `Div(stress) + bodyForce`

#### B) Problem-specification layer
Now made explicit as `ContinuumProblem2D`:

- body/domain
- material + assumption
- unknown fields
- loads
- constraints
- governing relation intent
- query intent

This demonstrates a real tiny continuum problem can be stated honestly without discretization.

#### C) Computational layer
Boundary appears only at the solve query:

- `Query = "SolveForField"` requires finite computational lowering to realize field values over the body.

The probe reports:

- `Category = "Computational"`
- `Missing = "MissingFiniteLowering"`

### Required questions answered
1. **What is already expressible?**
   Constitutive algebra, strong-form residual composition, and typed model components.
2. **Is a structured problem-specification layer missing before discretization?**
   Yes; this pass adds it explicitly as `ContinuumProblem2D` using regular language structures.
3. **First exact missing capability?**
   Finite lowering for `SolveForField` queries.
4. **Boundary type?**
   Computational.
5. **Is discretization actually next?**
   Only after explicit problem/query specification exists; then yes, as the realization layer.
6. **If discretization is needed, what role should it play?**
   Pure computational lowering of an already-specified continuum problem, not a replacement for model semantics.

### Blunt conclusion
The first missing layer after current field-form continuum mechanics is an explicit problem/query object.

After that object exists, the first hard boundary is computational: numerical field realization is impossible without finite lowering. That is where discretization becomes honestly justified.

### Next-step recommendation
Run a narrow **Computational Lowering M1** experiment that:

- consumes `ContinuumProblem2D`
- supports one tiny lowering path for one query (`SolveForField`)
- keeps constitutive and governing semantics in the continuum/problem layer
- emits solved outputs as a separate computational artifact

Do not broaden into general FE/FV/FD infrastructure yet.

## M1

### Finite representation chosen
M1 uses a **finite set of explicit 2D evaluation sites** inside the body (`FiniteSites2D`) as the minimal carrier.

- sites are concrete positions `(x, y)`
- no connectivity or adjacency is present
- no element/face/cell ownership is present
- no basis functions or quadrature are present

For the tiny cantilever probe, four fixed interior sites are used.

### Why this is the minimal honest lowering
`SolveForField(u)` cannot even be stated computationally until there are finitely many locations where unknowns can live. A bare site set is the smallest step that:

- is finite
- is explicit
- remains downstream from `ContinuumProblem2D`
- does not force FE/FV/FD commitments

Anything less (pure continuum only) cannot host computable unknown instances. Anything more (topology/connectivity/operators) over-commits before the seam is proven.

### Lowered object introduced
M1 adds:

- `LoweredProblem2D`
  - `Source: ContinuumProblem2D`
  - `Sites: FiniteSites2D` (`X: Float[]`, `Y: Float[]`)
  - `Unknowns: SiteUnknowns2D` (`Ux: Float[]`, `Uy: Float[]`, `IsSolved: Bool[]`)
  - `QueryTarget: String`

Lowering is explicit via `LowerContinuumProblem2D(problem)`.

### What `u` becomes after lowering
`u` becomes finite site-scoped unknown components:

- per-site unknown components: `ux`, `uy`
- representation: `SiteUnknowns2D` with one `Ux[i]`, `Uy[i]` pair per site index
- `IsSolved[i]` remains `false` in M1 for every site

These are placeholders for a future solve, not solved values.

### One computational capability unlocked
M1 defines exactly one computationally meaningful operation:

- `AttachUnknownsToSites(sites)` → creates explicit unknown instances at every site.

This makes the query target concrete (`"SiteUnknownDisplacement"`): the system can now point to *what would be solved for*.

### Boundary after M1 (explicit)
**Now possible**

- finite computational carrier exists
- unknowns are explicitly attached to finite sites
- the continuum source problem is preserved and referenced downstream

**Still impossible**

- no discrete operator is defined
- no residual/equilibrium system is built
- no global consistency enforcement exists
- no solve for `u` is possible
- no continuous field reconstruction is possible

The boundary is reported as: `MissingDiscreteOperatorWithoutTopology`.

### Does M1 force mesh-based discretization yet?
No.

M1 proves mesh topology is not yet required to cross the first computability seam. The first seam only requires finite unknown placement, not elements/connectivity.

### Recommendation for next pass
Run M2 as a **minimal operator-definition pass**:

- keep the same site carrier (still no mesh connectivity)
- define one tiny discrete relation that can be evaluated at sites
- keep solve out-of-scope unless/until a concrete global algebra object is introduced

Do not jump directly to full FEM/FV/FD stacks.

## M2

### Discrete relation chosen
M2 introduces a **site-scoped constraint residual relation**:

- attachment object: `SiteConstraintAttachment2D`
- evaluation object: `SiteConstraintResidual2D`
- evaluator: `EvaluateConstraintResidualAtSites(...)`

This relation maps a constrained site unknown slot to a residual-like mismatch:

- `uxResidual[i] = ux[i] - prescribedUx[i]` when `ux` is constrained at site `i`
- `uyResidual[i] = uy[i] - prescribedUy[i]` when `uy` is constrained at site `i`

No solve, assembly, or neighborhood coupling is introduced.

### Why this is the smallest honest relation
It is strictly downstream from `LoweredProblem2D` and uses only:

- explicit finite sites (`FiniteSites2D`)
- explicit site unknown slots (`SiteUnknowns2D`)
- explicit per-site constraint masks/values

It introduces one computable relation (`unknown - prescribed`) without introducing site-to-site stencils, elements, or adjacency.

### Objects introduced
- `SiteConstraintAttachment2D`
  - `IsUxConstrained: Bool[]`
  - `IsUyConstrained: Bool[]`
  - `PrescribedUx: Float[]`
  - `PrescribedUy: Float[]`
- `SiteConstraintResidual2D`
  - `UxResidual: Float[]`
  - `UyResidual: Float[]`
  - `ActiveUx: Bool[]`
  - `ActiveUy: Bool[]`
- `DiscreteRelationBoundaryReport`

Helper functions:
- `AttachCantileverConstraintToSites(lowered)`
- `EvaluateConstraintResidualAtSites(lowered, attachment)`
- `LowerWithSiteUnknowns(lowered, ux, uy)`
- `ProbeDiscreteRelationBoundary(lowered)`

### Capability unlocked
M2 unlocks one concrete computational concept:

- a typed **site-level residual-like value** for displacement constraints can be evaluated directly from a candidate site unknown state.

This is enough to computationally detect a constrained-slot mismatch without solving the continuum problem.

### Did this require topology?
No.

The relation is purely site-local and did not require:

- adjacency tables
- element ownership
- face/edge/cell connectivity
- hidden neighbor maps

### What remains impossible after M2
Still impossible (and intentionally out of scope):

- global coupling/equilibrium enforcement
- solve for `u`
- matrix/global assembly
- continuous field reconstruction
- operator application that fundamentally needs neighborhood interaction

### Topology pressure outcome
M2 succeeds without topology for a local constraint residual relation.

The next blocked capability is **site-to-site coupling** (for example, any equilibrium-style interaction). If the next pass targets that capability, it must introduce the first explicit coupling commitment and then test whether topology is truly required.

### Recommendation for next pass
Run one more narrow pass focused only on the first coupling relation (not a solver). At that point, decide if minimal explicit topology is unavoidable. Do not jump to FEM/FV/FD infrastructure yet.

## M3

### Coupling relation chosen
M3 introduces exactly one site-to-site relation: **pairwise difference coupling** over explicit site pairs:

- for each listed pair `(i, j)`
- `Δux_ij = ux[j] - ux[i]`
- `Δuy_ij = uy[j] - uy[i]`

This is evaluated by `EvaluateSiteCoupling(lowered, pairs)` and returned as typed per-pair outputs (`SiteCouplingResult2D`).

### Why this is minimal
This is the smallest honest interaction step beyond M2:

- coupling exists between different sites
- no solve or global assembly is performed
- no method commitment (no FE shape functions, no FDM stencil assumptions, no FVM control volumes)

It upgrades capability from site-local residual checks (M2) to explicit site-to-site interaction while keeping all global commitments out of scope.

### Coupling structure introduced
M3 adds `SitePairRelation2D`:

- `I: Int[]`
- `J: Int[]`
- `Weight: Float[]`

Pairs are explicit and finite for the tiny carrier:

- `(0,1), (1,3), (0,2), (2,3)`
- all weights are `1.0`

No neighbor graph, element ownership, adjacency table, or hidden mesh construction is introduced.

### Topology pressure outcome
**Did defining pairs require topology?**
No for this pass: explicit pair listing is sufficient to compute coupling deltas.

**Did pair selection feel arbitrary?**
Yes, intentionally. This is the pressure signal M3 is meant to expose.

**Did correctness depend on structured neighbor selection?**
Only weakly in M3 (deterministic relation evaluation works either way), but physical fidelity and operator consistency clearly begin to depend on principled pair selection.

**Does this naturally push toward nearest-neighbor/connectivity/partitioning?**
Yes. Once pair selection must be principled rather than hand-listed, the design is pressured toward minimal topology.

### Boundary after M3
**Now possible**

- site-to-site interaction exists
- typed coupling residual-like quantities exist per pair
- deterministic coupling evaluation can run over finite site unknown slots

**Still impossible**

- no consistent global operator
- no physical correctness guarantee
- no solve
- no continuum-consistent discretization

### Next missing piece decision
M3 indicates the immediate next choice is:

- **A) better pair selection (still topology-free)** as one more constrained probe, or
- **B) minimal topology commitment** if principled neighbor definition is required for correctness expectations

Current recommendation: attempt one narrow A-pass first, and if pair policy cannot be justified without hidden structure, make B explicit.
