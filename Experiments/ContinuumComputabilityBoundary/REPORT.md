# Continuum Computability Boundary

## Overview
This experiment determines whether discretization is actually needed yet for tiny continuum mechanics workflows, identifies the first missing layer after current continuum mechanics support, and distinguishes representation vs problem specification vs computation.

This experiment was migrated from `Language/Mechanics/ContinuumBoundaryM0/...` into its canonical experiment home under `Experiments/ContinuumComputabilityBoundary/M0/`.

## Milestones
- M0: problem-specification boundary probe
- M1: computational lowering probe (finite sites, no mesh/no solver)
- M2: site-local constraint residual relation (no coupling, no solve)
- M3: minimal site-to-site coupling probe (explicit pairs, no mesh/no solve)
- M4: deterministic Cartesian lattice coupling probe
- M5: localized deterministic refinement patch probe (single-level coarse–fine split)
- M6: deterministic graded refinement selection probe (importance-driven, one-step balancing)
- M7: deterministic multi-patch extraction probe (ordering + overlap suppression)
- M8: deterministic inter-patch interface bookkeeping probe (flat interface records + local balance accumulation)

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

## M5

### What refinement mechanism was introduced?
M5 introduces a **single-level localized refinement patch** represented by `RefinementPatch2D`:

- anchored parent cell indices: `ParentI`, `ParentJ`
- fixed refinement factor: `RefineFactor = 2`
- fixed local patch extent: `LocalNx = 2`, `LocalNy = 2`
- deterministic fine spacing: `DxFine = base.Dx / 2`, `DyFine = base.Dy / 2`

No recursion, quadtree, mesh object, or generalized topology graph is added.

### How is it anchored to the base lattice?
The patch is attached explicitly via `RefinedLattice2D`:

- `Base: Lattice2D`
- `Mask: LatticeMask2D`
- `Patch: RefinementPatch2D` (single patch for this milestone)

`AttachPatchToLattice(base, mask, patch)` returns a new refined object without mutating the base lattice. The anchor is purely index-based (`ParentI`, `ParentJ`) and deterministic.

### Exact coarse–fine flux rule used
M5 uses an explicit **conservative split rule** across coarse–fine interfaces:

- for each coarse edge adjacent to the refined parent cell,
- split the coarse edge amount equally into two fine-edge contributions (`amount/2 + amount/2`),
- ensure reverse conservation check: `sum(fine contributions) == coarse amount`.

This is implemented with direct index maps for right/up interface edges, with no interpolation and no hidden neighbor discovery.

### Did this require general topology?
No.

M5 retains deterministic Cartesian indexing plus one explicit local patch record. It does not introduce adjacency graphs, element ownership, basis functions, or solver-level topology machinery.

### Does refinement preserve deterministic structure?
Yes.

Given the same base lattice and parent cell index, patch construction is identical. Coarse–fine routing is fixed by explicit interface index rules and deterministic split math.

### Where does this approach begin to break?
The single-level fixed-rule patch begins to strain when requirements include:

- multiple/overlapping patches,
- non-binary refinement factors,
- anisotropic split rules,
- curved or non-axis-aligned interfaces,
- richer interface transfer rules beyond equal deterministic splitting.

At that point, additional interface bookkeeping may be required; this milestone intentionally stops before any general mesh/topology system.


## M6

### What importance field or demand mechanism was introduced?
M6 introduces `RefinementDemand2D`, an explicit per-cell demand object over the same deterministic 5x3 Cartesian lattice:

- `Importance`: float priority per active cell
- `TargetLevel`: integer requested refinement level per active cell
- deterministic policy: Manhattan distance to a designated hotspot cell
  - distance 0 -> level 2
  - distance 1 -> level 1
  - otherwise -> level 0

This is inspectable, deterministic, and rule-declared; no hidden heuristic scores are used.

### How is refinement selected from demand?
M6 adds `SelectRefinementLevels(...)` and `BalanceRefinementLevels(...)` to produce `RefinementLevelMap2D` directly from demand:

1. copy `TargetLevel` to active cells (explicit selection rule),
2. enforce grading via explicit neighbor balancing,
3. derive a concrete patch anchor for M5-compatible execution by deterministic max-level scan order.

This replaces hand-authored `ParentI/ParentJ` patch placement with rule-based selection from the demand field.

### What balancing/grading rule is enforced?
A strict one-step grading rule is enforced:

- 4-neighbor active cells may differ by at most one refinement level,
- deterministic sweep pass lowers violating cells,
- no recursive trees, no adaptive rebalancing engine, no quadtree object.

This provides the game-LOD-style graded transition behavior without leaving Cartesian indexing.

### M5 vs M6: did arbitrariness actually drop?
Yes, for the tested scope.

- **M5**: patch placement existed but was hand-authored (`ParentI`, `ParentJ`).
- **M6**: placement is selected from an explicit demand map + explicit grading rule.

So the source of placement is now declarative and inspectable rather than manual choice.

### Determinism and Cartesian structure status
Preserved.

- base lattice remains deterministic Cartesian (`Nx=5`, `Ny=3`, fixed `Dx`,`Dy`),
- refinement remains binary local 2x2 patch structure,
- selection and grading are deterministic from `(lattice, hotspot, rules)`.

No mesh/tessellation structures are introduced.

### Flux/balance compatibility status
Preserved for this pass.

After automatic selection, the resulting refined carrier still executes the M5-style coarse-fine conservative split and balance accumulation checks.

### New boundary exposed by M6
The next pressure point is **structured multi-patch extraction**:

- a level map can request multiple separated high-demand regions,
- this M6 pass still extracts only one concrete patch for downstream M5-compatible execution,
- deterministic multi-patch extraction/ordering and non-overlap policy are now the next smallest justified extension.

### Recommendation for next pass
Run M7 as a narrow deterministic multi-patch probe:

- consume `RefinementLevelMap2D`,
- extract multiple local 2x2 Cartesian patches with explicit stable ordering and non-overlap rules,
- preserve one-step grading and conservative interface routing,
- still avoid AMR/quadtree frameworks and unstructured meshing.

## M7

### Multi-patch extraction rule used
M7 lowers `RefinementLevelMap2D` into multiple explicit 2x2 Cartesian patch candidates using one narrow rule:

- a candidate anchor `(i,j)` is emitted only when:
  - `levelMap.Levels[i,j] > 0`, and
  - the full 2x2 parent-cell footprint `[(i,j),(i+1,j),(i,j+1),(i+1,j+1)]` is active.
- no hand-authored patch coordinates are allowed.

This yields a deterministic candidate list directly from the level map.

### Ordering rule used
M7 uses exactly one ordering policy:

- **descending requested level, then row-major (`j` ascending, then `i` ascending)**.

This is explicit and stable: identical input map always yields identical candidate order.

### Overlap rule used
M7 uses exactly one non-overlap policy:

- **first-wins suppression by candidate order**.
- each accepted patch claims its 2x2 parent-cell footprint.
- any later candidate touching a claimed parent cell is suppressed.
- no merge behavior is permitted.

This keeps overlap handling transparent and deterministic.

### Did flux/balance compatibility hold?
Yes, within M7 scope.

- multiple accepted patches coexist in one explicit multi-patch carrier,
- each accepted patch still uses M5-style coarse-fine conservative split checks,
- each accepted patch still supports explicit incoming/outgoing/net balance totals.

This remains a representation/computational-structure probe; no global solve or mesh assembly is introduced.

### Did this stay smaller/cleaner than adaptive meshing?
Yes.

M7 adds only list-level extraction bookkeeping (candidate list + suppression list + accepted list). It does **not** add tree recursion, parent/child graph traversal, mesh connectivity, basis machinery, or solver infrastructure.

### New complexity that appeared
The real added complexity is now explicit and local:

- deterministic candidate enumeration from a graded level map,
- deterministic overlap suppression bookkeeping,
- per-patch interface accounting repeated over multiple accepted patches.

This is still understandable as plain Cartesian list processing.

### Next boundary surfaced by M7
The next boundary is **inter-patch interaction bookkeeping**:

- when multiple accepted patches are near each other, explicit handling of neighboring coarse-fine interfaces and interaction accounting becomes the dominant pressure point,
- before any justified move to recursive hierarchies or flexible patch shapes.

### Recommendation for next pass
Run M8 as a narrow inter-patch-interface probe:

- keep fixed 2x2 Cartesian patches,
- keep deterministic order + first-wins suppression,
- add explicit bookkeeping for adjacent-patch/coarse-neighbor interface accounting,
- continue forbidding AMR/quadtree/mesh/solver infrastructure.


## M8

### 1) What interface representation was introduced?
M8 introduces `PatchInterface2D` as a **flat interface-record carrier** (not a graph):

- `PatchA`: accepted patch index that owns the record
- `PatchB`: neighboring accepted patch index, or `-1` sentinel for coarse-side interfaces
- `Kind`: encoded interaction type (`0 = patch-coarse`, `1 = patch-patch`)
- `Direction`: encoded side (`0 = Right`, `1 = Up`, `2 = Left`, `3 = Down`)
- `CoarseI`, `CoarseJ`: explicit parent-coarse edge locator
- `FineA0/FineA1`, `FineB0/FineB1`: explicit fine-slot mappings
- `Weight0/Weight1`: explicit split weights

No hidden adjacency graph or mesh connectivity object is introduced.

### 2) How are interfaces detected deterministically?
`BuildPatchInterfaces(...)` performs one fixed index-only sweep over accepted patch anchors:

- iterate accepted patches in deterministic M7 order
- probe neighbors only at fixed offsets `(±2,0)` and `(0,±2)`
- emit patch-patch records for right/up neighbors (`p < q` ownership for de-duplication)
- emit patch-coarse records for sides with no patch neighbor

Detection is explicit, deterministic, inspectable, and graph-free.

### 3) What rules govern coarse-fine and fine-fine interactions?
M8 uses one narrow rule per interface class.

- **Coarse-fine rule:** read coarse edge amount at explicit `(CoarseI, CoarseJ)` indices and split 50/50 across the two aligned fine slots.
- **Fine-fine (patch-patch) rule:** direct aligned slot matching (two slot pairs) using shared parent coarse edge amounts; no interpolation is introduced.

### 4) Does flux/balance accounting remain coherent?
Yes, within M8 scope.

- `EvaluateInterfaceContributions(...)` computes per-interface amounts and explicit per-patch incoming/outgoing/net totals.
- `AccumulateInterfaceBalance(...)` aggregates interface totals and preserves traceability.
- With symmetric bookkeeping rules, each patch keeps zero net (`outgoing - incoming`) in this probe.

No global system is assembled or solved.

### 5) Does M8 still avoid topology/graph machinery?
Yes. M8 stays within:

- Cartesian base lattice
- fixed 2x2 patches
- flat arrays and interface records

M8 still avoids AMR trees, quadtree traversal, mesh connectivity graphs, and solver infrastructure.

### 6) What new complexity appeared?
New complexity is explicit and local:

- interface list size grows faster than patch count
- directional coarse-index bookkeeping is verbose
- ownership/de-dup policy (`p < q`) must remain stable

This is still smaller than mesh/topology systems, but bookkeeping strain becomes visible.

### 7) What is the next boundary?
The next boundary is junction policy:

- corner/T-junction ownership when several patches crowd one coarse neighborhood
- deterministic tie-breaking for multi-interface attribution

### M7 vs M8
- **M7:** patches are extracted deterministically, but do not explicitly interact.
- **M8:** patches explicitly interact through `PatchInterface2D` records (patch-patch and patch-coarse), and interface contributions are accumulated explicitly.

### Recommendation for next pass
Run a narrow junction-bookkeeping probe:

- keep fixed 2x2 Cartesian patches
- keep flat explicit interface records
- add explicit corner/T-junction ownership tables
- continue forbidding AMR/quadtree/mesh/solver infrastructure

## M9

### 1) What junction cases were tested?
M9 probes two narrow ambiguous local configurations only:

- **corner-like competition** at coarse neighborhood `(1,1)` where multiple nearby interface-side claims can own the junction,
- **T-junction-like competition** at coarse neighborhood `(2,1)` where patch-side and coarse fallback attribution can both appear.

No general junction engine was introduced.

### 2) What candidate representation was used?
M9 adds `JunctionCandidate2D` as a flat, explicit candidate table:

- `OwnerKind` (`1 = patch`, `0 = coarse fallback`)
- `PatchIndex` (`-1` sentinel for coarse fallback)
- `RefinementLevel`
- `ExtractionOrder`
- `Direction`
- `ContextKind`
- `Directness`
- `SourceInterface`

Candidate generation is handled by `BuildJunctionCandidates(...)` via one deterministic interface sweep at a specific coarse `(i,j)`.

### 3) What scoring factors were used?
M9 now uses **real Octomata utility policy syntax** for primary owner choice:

- `flow SelectJunctionOwnerPolicyFlow(...)`
- single `state Decide`
- `let winner = when policy { hysteresis: 0, min_commit: 0 } { ... }`

Each candidate case uses the same explicit integer score:

- `1000 * RefinementLevel`
- `100 * Directness`
- `10 * OwnerKind`
- direction-match bonus (`4` if direction matches context, else `1`)
- early extraction bonus (`50 - ExtractionOrder`)

### 4) How was tie-breaking handled?
`SelectJunctionOwner(...)` now executes two explicit deterministic phases:

1. primary candidate choice via Octomata `when policy` (in `flow/state`) with `hysteresis: 0` and `min_commit: 0`,
2. explicit exact-score tie-break by lower `ExtractionOrder`, then lower `PatchIndex`, then patch owner over coarse fallback.

`flow/state` is present only because this pass intentionally uses real Octomata utility vocabulary; this remains a one-shot local decision, not a temporal controller.

### 5) Was utility-style ranking cleaner than hardcoded precedence?
For the two tested cases, Octomata utility policy and naive precedence produced the **same winners**.

Bluntly: for this narrow one-shot probe, the Octomata version is **not cleaner** than the earlier plain helper loop. It is still inspectable, but introduces extra flow/state ceremony solely to stay in the real Octomata control model.

### 6) What complexity remained?
M9 introduces two explicit complexities:

- score-weight calibration must stay disciplined to avoid disguised policy sprawl,
- mandatory `flow/state + when policy` scaffolding is additional ceremony when `hysteresis` and `min_commit` are intentionally set to zero.

So while deterministic and inspectable, this formulation is heavier than plain scoring for one-shot ownership.

### 7) What boundary appears next?
Based on probe evidence, the next boundary is:

- harder **multi-claimant** junction classes (3+ viable patch claimants),
- and eventually non-axis-aligned/curved ownership regions.

That is still distinct from adopting full topology infrastructure.

### Recommendation for next pass
Run one additional deterministic probe with one harder three-claimant junction case while preserving:

- fixed 2x2 Cartesian patches,
- flat interface + candidate records,
- one-shot explicit ranking,
- no graph/mesh/AMR/solver machinery.
