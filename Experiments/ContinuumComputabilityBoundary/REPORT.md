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

### Probe framing
M3 is a comparative lowering probe between:

- **point-only coupling over finite sites**, and
- **directed pairwise flux-like transfer carriers**.

It does not add a solver, mesh, FEM/FV/FDM, or hidden topology. The pass only asks whether a directed transfer primitive is a more honest first interaction bytecode for balance-shaped computation.

### Flux-like carrier introduced (exact)
M3 introduces `SiteFluxRelation2D` with explicit finite arrays:

- `I: Int[]` (source site index)
- `J: Int[]` (target site index)
- `Dx: Float[]` (direction x)
- `Dy: Float[]` (direction y)
- `Amount: Float[]` (scalar transfer amount)

Interpretation used in this probe:

- each row is a **directed transfer** `i -> j`
- `Amount[p]` is scalar transfer magnitude
- `(Dx[p], Dy[p])` is the explicit transfer direction
- outgoing contribution is added at `i`; incoming contribution is added at `j`

This remains a plain explicit pair list, not a topology system.

### Balance-style computation unlocked
M3 adds one accumulation function: `AccumulateDirectedFluxBalance(lowered, flux)`.

From only pair entries, it computes per-site:

- `IncomingAmount`
- `OutgoingAmount`
- `NetAmount = outgoing - incoming`
- `NetVectorX`, `NetVectorY` from signed directed transfer vectors

This is the key new capability: **site-level balance-like quantities are directly computable from directed transfer carriers** without solve/assembly.

### Concrete comparison: point-only vs directed flux carrier

#### What point-only site coupling can express
- finite locations and unknown slots
- candidate site values
- local site residuals (as shown in M2)

#### What it cannot express honestly by itself
- directed transfer meaning (`who -> whom`)
- explicit transfer amount separate from unknown differences
- incoming vs outgoing bookkeeping required by balance-style accumulation

#### What directed flux carriers add
- first-class direction, transfer amount, and source/target semantics
- straightforward accumulation into balance-shaped site quantities
- less arbitrary meaning than pure pairwise unknown differences when probing transfer-dominated relations

### Topology requirement outcome
M3 did **not** require topology to run this probe.

- Pair policy stayed explicit and finite.
- No hidden adjacency graph, elements, cells, faces, or ownership machinery was introduced.

However, the next pressure is clear: principled pair/interface selection quality. If pair policy can no longer be justified manually, minimal topology becomes the next explicit boundary.

### Required M3 answers
1. **What exact flux-like carrier was introduced?** `SiteFluxRelation2D(I, J, Dx, Dy, Amount)`.
2. **What does it encode that bare point sites do not?** Directed source/target transfer semantics, explicit transfer amount, and transfer orientation.
3. **What balance-like quantity became computable?** Per-site incoming/outgoing/net scalar transfer and signed net transfer vectors.
4. **Did it require topology?** No, not for this explicit pair-list probe.
5. **Is it a more honest first interaction primitive than point-only coupling?** For balance/transfer-shaped relations in this experiment, yes.
6. **What is the next boundary?** Pair/interface selection quality and whether that forces a minimal topology commitment.

### Blunt conclusion
For this experiment, directed flux-like pair carriers are a more honest first coupling primitive than point-only site coupling for balance-dominated interaction structure. This is not a solver claim and not a replacement for FEM/FV/FDM; it is a cleaner pre-topology interaction bytecode for the next probe.

### Recommendation for next pass
Run an M4 **pair-selection policy probe**:

- keep directed flux carriers
- compare at least two explicit pair policies (e.g., hand-listed vs distance-threshold)
- measure effect on balance accumulation structure
- decide if minimal explicit topology is now unavoidable

## M4

### Probe framing
M4 is a topology-commitment probe that compares three interaction carriers:

- **M2 point-only sites** (no transfer semantics),
- **M3 directed pairwise flux** (explicit transfer, but pair-policy-dependent), and
- **M4 deterministic Cartesian lattice flux** (explicit transfer, deterministic local structure).

This pass does not introduce FEM/FV/FDM, mesh/tessellation, global assembly, or solves.

### Lattice representation introduced
M4 introduces `Lattice2D`:

- `OriginX`, `OriginY`
- `Dx`, `Dy`
- `Nx`, `Ny`

and embedding mask `LatticeMask2D`:

- `IsActive`
- `IsBoundary`

The lattice is built deterministically from fixed spacing and extents, then the body is embedded onto lattice sites with no pair-list policy step.

### How pair-selection arbitrariness is removed
M3 used explicit finite pair lists (`I`, `J`) and therefore required a hand-authored (or otherwise policy-chosen) pair inclusion decision.

M4 removes this by deriving interaction structure exclusively from Cartesian offsets:

- `(i, j) -> (i+1, j)`
- `(i, j) -> (i, j+1)`

restricted to active sites. This makes neighborhood generation deterministic from lattice + mask.

### Flux carrier on lattice
M4 keeps directed transfer semantics from M3, but relocates storage from arbitrary pair rows to deterministic edge slots:

- `FluxRight[idx]` for +x transfer from site `idx`
- `FluxUp[idx]` for +y transfer from site `idx`

This remains a transfer carrier, not a finite-volume method or mesh operator.

### Balance-style computation enabled
`AccumulateLatticeFluxBalance(lattice, mask, flux)` computes per site:

- `IncomingAmount`
- `OutgoingAmount`
- `NetAmount`
- `NetVectorX`
- `NetVectorY`

using only deterministic right/up adjacency and directed edge amounts.

### Explicit comparison (M2 vs M3 vs M4)
- **M2 point-only**: minimal unknown-slot representation, no transfer semantics.
- **M3 pair-flux**: transfer semantics unlocked, but physical interpretation can vary with pair policy.
- **M4 lattice-flux**: transfer semantics preserved and structure becomes deterministic from topology parameters.

Least-arbitrary carrier in this probe: **M4 lattice-flux**.

### Is topology now committed?
Yes.

A Cartesian lattice with deterministic neighbor offsets is already an explicit topology commitment. It is the first unavoidable topology step in this probe, because interaction structure is no longer policy-defined per pair and is now encoded by a fixed neighborhood relation.

### Is this more honest than mesh/tessellation at this stage?
Yes.

For this stage, the lattice is a smaller and cleaner topology commitment than full mesh/tessellation:

- no element shapes
- no basis functions
- no unstructured connectivity machinery
- no solver stack

Yet interaction structure is explicit, deterministic, and computable.

### Next boundary
The next boundary is topology flexibility:

- curved boundaries,
- anisotropy/non-axis-aligned structure,
- heterogeneous local connectivity needs.

These pressures may require a topology carrier beyond Cartesian offsets.

### Recommendation for M5
Run an M5 **topology-flexibility probe**:

- keep directed flux carriers,
- compare Cartesian lattice against one minimally more flexible topology representation,
- keep no-solver/no-mesh constraints,
- determine the smallest justified extension beyond deterministic Cartesian adjacency.
