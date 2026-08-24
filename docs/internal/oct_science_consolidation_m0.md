# OCT-SCIENCE-CONSOLIDATION-M0 — Scientific Package, API, and Discovery Consolidation

## 1. Verdict

**Success**

Every major scientific concept now has one documented first place to look, the principal overlaps have explicit ownership rules, the remaining compatibility names delegate rather than compete, shared RF/Wireless constants use `Physics`, a public task- and discipline-oriented guide exists, and four fresh agents selected the canonical packages without human correction. All scientific contracts pass in compiled/auto and interpreted execution with zero unexpected fallback.

## 2. Motivation

M0 and M1 built broad executable-textbook coverage. Their remaining constraint was no longer subject matter but navigation: a user should not need to know whether a calculus, physical law, or radio calculation arrived in an early or late milestone. Consolidation therefore treats the packages as a bookshelf: singular canonical ownership, compatible old entry points, explicit neighboring-package directions, and no `Science.*` re-export layer.

## 3. Post-M1 library inventory

The audit covered the requested science packages plus `Interpolation`, which is scientifically relevant and already present. API family names below are representative rather than an exhaustive symbol dump.

| Package | Canonical responsibility and public API families | Overlap / compatibility | Discovery after M0 |
| --- | --- | --- | --- |
| Algorithms | Generic search, predicates, integer/prime algorithms | Biology keeps biological types; no copied generic algorithms found | README + science guide |
| Analysis | Finite differences, accumulation, and shape features over sampled `Float[]` data | Distinct from function-facing `Numerics`, DSP `Signal`, and summaries `Statistics` | Cross-linked README + improved manifest description |
| Astrophysics | Orbits, stellar radiation, distance, parallax, redshift | Uses `Physics` constants and `Units.Astronomical`; no competing constants | Cross-linked README + guide |
| Chemistry | Solution relations, pH, dilution, Beer-Lambert, kinetics | General chemistry owner; NMR remains specialized | Cross-linked README + guide |
| ChemistryNmr | Larmor/shift/coupling, peaks, multiplicities, spectra, queries | Uses spectroscopy conventions; does not own general chemistry | Cross-linked README + guide |
| Complex | Complex trigonometric/hyperbolic extensions | Core language owns base complex primitives | README + guide |
| ComputationalBiology | DNA/RNA enums, sequence transforms, translation, consensus, population models | Generic statistics/search stay in their generic packages | Cross-linked README + guide |
| ComputationalScience | Executable cross-library lessons | Owns no law or solver; bounded composition examples only | Strengthened README + guide |
| Cooking | Typed culinary conversion/scaling, baker %, brines, mixing, cooling | Composes `Units` and `Thermofluids`; scalar names remain compatible | Cross-linked README + guide |
| DifferentialEquations | Scalar fixed-step Euler, midpoint, RK4 ODE methods | Algorithms, not scenario execution | Cross-linked README + guide |
| Distributions | Deterministic PDF/CDF/PMF evaluation | `Random` owns sampling; `Statistics` owns summaries/inference helpers | README + guide |
| Electromagnetism | Electrostatics, ideal circuits, magnetism, induction | Uses `Physics` constants; RF/Wireless own downstream engineering abstractions | Cross-linked README + guide |
| Geometry | Unit-safe planar, polygon, and solid relations | Mathematical foundation used by applications | README + guide |
| Interpolation | Linear, Lagrange, spline, and bilinear interpolation | Distinct from regression and ODE integration | README + guide |
| LinearAlgebra | Vector/matrix operations, decompositions, eigen/SVD helpers | `Tensor2D` owns pointwise coordinate calculus | README + guide |
| Mathematics | Elementary scalar helpers and complex transforms | Calculus names are compatibility wrappers over `Numerics`; built-in `FFT` is production transform | Existing explicit boundary + guide |
| Mechanics | Stress, failure, fatigue, shafts, beams, buckling, vessels, continuum | `Physics` owns foundational laws; `Thermofluids` owns fluids/heat | Cross-linked README + guide |
| Numerics | Scalar roots, finite differences, quadrature, bounded 1D minimization | Canonical numerical-calculus owner; scalar optimization is deliberately distinct from richer `Optimization` | Existing explicit boundary + guide |
| Optimization | Multivariate gradient methods, line search, Nelder-Mead, nonlinear least squares | `Numerics` retains bounded scalar textbook minimization | Existing explicit boundary + guide |
| Optics | Refraction, thin lenses, interference, diffraction, resolution, photons | Uses `Physics` constants/waves; not generic DSP | Cross-linked README + guide |
| Physics | Shared physical constants, foundational mechanics/dynamics/waves | Specialized packages consume this foundation | Cross-linked README + guide |
| Quantum | Photon/matter-wave, energy-level, uncertainty, Born-probability models | Uses `Physics` constants and `Units`; intentionally bounded | Cross-linked README + guide |
| Random | Seeded RNG, Gaussian/distribution sampling, coin/dice helpers | `Distributions` owns deterministic evaluation | New package README + corrected manifest/registry description |
| RF | Propagation, link primitives, fading, multipath, Doppler/coherence, deterministic MIMO, S-parameters | `Signal` is generic DSP; `Wireless` is communication/link facing; constant names are compatibility wrappers | Cross-linked README + guide |
| Signal | Convolution, correlation, moving average | FFT belongs to built-in/`Mathematics`; RF/Wireless are domain packages | Cross-linked README + corrected manifest/registry description |
| Simulation | Deterministic transition/observation traces, integration of recorded output, sweeps | Does not own ODE algorithms | Cross-linked README + guide |
| Statistics | Descriptive/weighted statistics, summaries, regression, utility fitting | `Distributions` evaluates probability laws; `Uncertainty` propagates measurements | README + guide |
| Tensor2D | Pointwise numerical 2D gradient/Jacobian/divergence/symmetric gradient | Uses vector/matrix values but does not replace `LinearAlgebra` | README + guide |
| Thermofluids | Thermodynamics, fluids, dimensionless groups, heat transfer, process compatibility | Supplies equations; ODE/Simulation supply progression | Cross-linked README + guide |
| Uncertainty | First-order independent measurement-uncertainty propagation | Uses `Statistics`; not a probability distribution package | README + guide |
| Units | American/British, astronomical, atomic, spectroscopy records and SI conversion | Compiler owns SI; `Physics` owns constants and laws | Cross-linked README + guide |
| Wireless | Typed communication bands, link budgets, thermal noise, OFDMA, throughput plus legacy scalar compatibility | RF owns channel/propagation math; `Wireless.Typed` is preferred | Cross-linked README + guide |

No public science package was proven obsolete. `Cooking` remains a legitimate application-science package. `Interpolation` was added to the conceptual catalog rather than hidden between numerical neighbors.

## 4. Taxonomy

The public taxonomy is:

- Mathematical foundations: Algorithms, Complex, Geometry, LinearAlgebra, Mathematics, Tensor2D.
- Numerical computation: Analysis, DifferentialEquations, Interpolation, Numerics, Optimization, Simulation, ComputationalScience.
- Statistics/probability/uncertainty: Statistics, Distributions, Random, Uncertainty.
- Physical sciences: Physics, Mechanics, Thermofluids, Electromagnetism, Optics, Quantum, Astrophysics.
- Chemistry/life science: Chemistry, ChemistryNmr, ComputationalBiology.
- Signal/communications engineering: Signal, RF, Wireless.
- Scientific foundation/application: Units, Cooking.

This is conceptual documentation only. Existing top-level import identities remain intact.

## 5. Canonical ownership rules

1. Mathematical definitions/helpers and transforms start in `Mathematics`; approximation methods start in `Numerics`.
2. Bounded scalar minimization starts in `Numerics`; multivariate solver workflows start in `Optimization`.
3. Universally reusable constants and foundational physical laws start in `Physics`; domain analysis starts in the specialized package.
4. ODE algorithms start in `DifferentialEquations`; trace/sweep execution starts in `Simulation`.
5. Generic sequence DSP starts in `Signal`; RF channel/system math starts in `RF`; communication/link calculations start in `Wireless`.
6. General chemistry starts in `Chemistry`; NMR starts in `ChemistryNmr`; reusable spectroscopy unit conventions start in `Units.Spectroscopy`.
7. Biological representation/algorithms start in `ComputationalBiology`; generic algorithms/statistics remain generic.
8. SI dimensions are language-owned; non-SI conversion starts in `Units`.

## 6. Mathematics/Numerics consolidation

The memorable rule is: `Mathematics` describes elementary math and transforms; `Numerics` chooses an approximation algorithm and exposes its controls/evidence.

`Mathematics.DifferentiateCentral`, `IntegrateTrapezoidal`, and `IntegrateSimpson` remain compatibility textbook names. They already delegate to `Numerics.CentralDiff`, `Trapezoid`, and `Simpson`, preserving historical reversed/zero-width interval behavior. The public guide now directs new numerical calculus code to `Numerics`. No implementation duplication remains in this overlap.

Observed-data finite differences remain in `Analysis`, because that package accepts sampled arrays rather than a mathematical function.

## 7. Optimization consolidation

`Optimization` is canonical for gradient descent, line search, momentum, Nelder-Mead, Gauss-Newton, Levenberg-Marquardt, and curve fitting. `Numerics` is canonical for transparent bounded one-dimensional golden-section and Brent minimization. These are different abstraction levels, not equal owners. No API moved and no duplicate implementation was introduced.

## 8. Physics/specialized-domain boundaries

`Physics` owns constants, force/momentum/energy, general dynamics, and elementary wave relations. `Mechanics`, `Thermofluids`, `Electromagnetism`, `Optics`, `Quantum`, and `Astrophysics` own the engineering/scientific models named by their disciplines. Each README now says where adjacent work belongs.

Astrophysical gravitational force remains an application-facing convenience over the canonical `Physics.GravitationalConstant`; it is not a competing definition of the constant or a replacement for foundational mechanics.

## 9. Simulation/ODE boundary

`DifferentialEquations` owns derivative-driven ODE algorithms. `Simulation` owns callback-defined transitions, observations, aligned traces, recorded-output integration, and deterministic sweeps. `ComputationalScience` demonstrates how normalized scenario execution composes with typed domain boundaries. Neither package was merged or renamed.

## 10. Signal/RF/Wireless boundary

- `Signal`: domain-neutral convolution, correlation, and moving averages.
- `RF`: propagation/channel/network mathematics, fading/multipath, Doppler/coherence, deterministic MIMO, and S-parameters.
- `Wireless`: communication bands, link budgets, OFDMA, and throughput.

M1's Wireless modernization is complete: `Wireless.Typed` is the preferred `Float<Hz>`/SI surface, the old scalar-Hz surface is explicitly compatibility-only, and no stale scalar-modernization TODO remains in public science documentation.

## 11. Chemistry/NMR boundary

`Chemistry` owns solution chemistry and kinetics. `ChemistryNmr` owns NMR frequency, shift, coupling, peaks, multiplicities, and spectra. `Units.Spectroscopy` owns nominal presentation/conversion records such as ppm and reciprocal centimetres. No spectroscopy constant was moved upward into NMR.

## 12. ComputationalScience role

`ComputationalScience` is a bounded cross-disciplinary executable chapter. It may demonstrate composition, error, and convergence, but it must not own physical laws, generic statistics, ODE methods, or simulation engines. The current cooling, RC, population, and orbit lessons fit this role; no miscellaneous expansion was added.

## 13. Units organization

SI dimensions (`m`, `kg`, `s`, `A`, `K`, `mol`, `cd`, `Hz`) are compiler-owned. `Units.Core` provides small typed helpers. `Units.American` and `Units.British` provide display/convenience records; `Units.Astronomical`, `Units.Atomic`, and `Units.Spectroscopy` own their respective domain conventions and explicit conversion to SI.

`BaseUnit` strips a dimension and is not a converter. The public guide and Units README state this directly. Units remains U1-strength; it was not redesigned.

## 14. Cooking placement

`Cooking` owns practical culinary calculations: typed ingredients, recipe/yield scaling, baker's percentages, brines, mixing, and resting/cooling applications. It imports `Units` and `Thermofluids`; `CoolingRestTemperature` delegates to the canonical lumped thermal relation. Foundational heat equations remain in `Thermofluids`, and no chemistry or thermodynamics was copied into Cooking.

## 15. Shared constants/provenance

`Physics` remains the canonical owner of `c`, `h`, `hbar`, `k_B`, `e`, `N_A`, `R`, Stefan-Boltzmann, vacuum permittivity/permeability, `G`, Wien displacement, standard gravity, and stored particle masses.

This milestone removed proven hard-coded duplication:

- `RF.SpeedOfLightMetersPerSecond` now delegates to `Physics.SpeedOfLight`.
- `RF.BoltzmannConstantWattsPerHzKelvin` now delegates to `Physics.BoltzmannConstantSI`.
- legacy `Wireless.ThermalNoiseFloorDbm` now obtains `k_B` from `Physics.BoltzmannConstantSI`.

Exact SI-defined constants are identified in `Physics.Constants.oct`; gas constant is exact-derived; vacuum electromagnetic constants are post-2019 measured/derived; `G` is measured; the stored Stefan-Boltzmann value is documented as conventional rounded. `Units.Spectroscopy.FrequencyToWavenumber` retains the exact `c` numeric scale locally so foundation-level `Units` does not depend upward on `Physics` solely for a conversion factor.

## 16. Dependency direction

| Layer | Major packages | May depend toward |
| --- | --- | --- |
| Language foundation | compiler SI dimensions, OctStd | nothing scientific |
| Mathematical/unit foundation | Units, Algorithms, Mathematics, Geometry, Complex | language foundation |
| General numerical/data foundation | Numerics, LinearAlgebra, Statistics, Distributions, Random, Analysis, Interpolation | mathematical foundation as needed |
| Higher numerical methods | Optimization, Uncertainty, Tensor2D, DifferentialEquations, Simulation | general foundation |
| Physical foundation | Physics | language foundation |
| Physical domains | Mechanics, Thermofluids, Electromagnetism, Optics, Quantum | Physics and mathematical/unit foundations |
| Applied physical science | Astrophysics, RF, Wireless, Cooking | physical/domain foundations and Units |
| Biological/chemical domains | Chemistry, ChemistryNmr, ComputationalBiology | Physics or generic foundations only when real code needs them |
| Composition lessons | ComputationalScience | domain packages plus Simulation/Units |

No scientific dependency cycle was introduced. The only manifest dependency added is `RF -> Physics`, matching the documented direction.

## 17. Compatibility aliases

Compatibility paths remain source-compatible and are documented as such:

- Mathematics calculus textbook names delegate to `Numerics`.
- Physics scalar/partially dimensional constant functions remain beside preferred SI forms.
- Cooking scalar conversions remain beside typed APIs.
- Thermofluids scalar-temperature helpers remain beside typed relations.
- Wireless legacy scalar-Hz records/functions remain beside preferred `*SI` APIs.
- RF constant names remain public and now delegate to `Physics`.

No deprecation mechanism or runtime registry was invented.

## 18. Implementation deduplication

The audit confirmed Mathematics/Numerics calculus delegation and Cooking/Thermofluids cooling delegation already exist. This milestone collapsed the three proven RF/Wireless constant copies described in section 15. Similar-looking formulas with domain-specific assumptions—such as generic wave relations, optical photon relations, and RF propagation—were not abstracted merely because they share algebraic pieces.

## 19. Naming/error-semantics consistency

The current numerical result families use recognizable evidence (`Value`/`Root`/`XMin`, `Converged`, `Iterations`, and evaluation counts where applicable). Domain records remain appropriately named; no universal result record was forced.

Guidance now says:

- runtime mathematical/domain invalidity: `! Error`;
- statically expressible single-value precondition: refined `Concept`/`Require`;
- meaningful nonconverged estimate: explicit result with convergence evidence;
- historical sentinel behavior: preserve for compatibility and point to preferred fallible/typed siblings.

No mass `CalculateX`/`ComputeX` rename was justified. Existing readable verb/noun names remain stable.

## 20. Package README improvements

Boundary/neighbor/starting-example guidance was added to Analysis, DifferentialEquations, Simulation, Physics, Mechanics, Thermofluids, Electromagnetism, Optics, Quantum, Astrophysics, Chemistry, ChemistryNmr, ComputationalBiology, Signal, RF, Wireless, Units, Cooking, and ComputationalScience. `Random` gained its missing root README. Existing Mathematics, Numerics, and Optimization READMEs already stated their canonical distinctions.

Manifest and registry descriptions for Analysis, Random, and Signal were replaced with searchable responsibilities. Explicit Authors arrays were preserved; touched manifest dates use ISO `2026-08-23`.

## 21. Top-level science guide

Public navigation now lives at [`docs/science/README.md`](../science/README.md) and is linked from the root README. It supports task-first and discipline-first browsing, names package boundaries, explains constants/units and error/result conventions, gives a scientific dependency map, and shows canonical composition examples. It adds no registry, search service, mega-package, or wildcard import.

## 22. Concept ownership table

| Domain/concept | Canonical owner | Compatibility owner(s) | Rationale |
| --- | --- | --- | --- |
| calculus-facing numerical methods | Numerics | Mathematics textbook wrappers | Differentiation/quadrature are approximation methods; Mathematics retains historical textbook names. |
| quadrature | Numerics | Mathematics textbook wrappers | Multiple rules and convergence diagnostics live together. |
| root finding | Numerics | none | Scalar reference numerical algorithms. |
| bounded scalar optimization | Numerics | none | Transparent one-dimensional methods. |
| multivariate optimization | Optimization | none | Gradient/simplex/least-squares workflows and diagnostics. |
| ODEs | DifferentialEquations | none | Mathematical integration algorithms. |
| simulation | Simulation | none | Scenario transitions, traces, and sweeps. |
| statistics | Statistics | none | Generic summaries, regression, fitting. |
| probability evaluation | Distributions | none | Deterministic PDF/CDF/PMF surface. |
| random sampling | Random | none | Seeded stochastic generation. |
| signal transforms | Mathematics / built-in `FFT` | `FastFourierTransform` is reference path | Production transform is built-in; explicit IFFT/reference implementation remains discoverable in Mathematics. |
| finite-sequence DSP | Signal | none | Domain-neutral convolution/correlation/moving average. |
| mechanics | Mechanics | foundational Physics relations | Engineering analysis is distinct from universal laws. |
| thermodynamics | Thermofluids | scalar thermal compatibility helpers | Typed thermodynamic relations share assumptions and units. |
| fluids | Thermofluids | legacy process helpers in same package | Hydrostatics/flow/dimensionless/correlation family. |
| heat transfer | Thermofluids | Cooking application wrappers | Foundational conduction/convection/radiation/transients. |
| electromagnetism | Electromagnetism | foundational Physics constants | Electrostatic/circuit/magnetic domain. |
| optics | Optics | Physics waves/constants | Optical assumptions and result records. |
| quantum | Quantum | Physics constants | Bounded quantum models. |
| astrophysics | Astrophysics | Physics foundational gravity/radiation | Astronomy/orbit applications. |
| chemistry | Chemistry | none | General solution and kinetics relations. |
| NMR | ChemistryNmr | Units.Spectroscopy conventions | Specialized spectral models; units remain reusable. |
| computational biology | ComputationalBiology | none | Biological alphabets and operations. |
| units | compiler SI + Units for non-SI conversion | historical scalar APIs in domain packages | SI is static language truth; conversions are explicit. |
| cooking | Cooking | none | Legitimate practical culinary science. |
| RF | RF | RF constant-name wrappers | Propagation/channel/system mathematics. |
| wireless | Wireless typed `*SI` surface | Wireless legacy scalar-Hz surface | Communication/link-facing models with compatibility. |

## 23. Migration table

| Old path/API | Preferred path/API | Compatibility status | Breaking? |
| --- | --- | --- | --- |
| `Mathematics.DifferentiateCentral` | `Numerics.CentralDiff` (or chosen finite-difference method) | Delegating wrapper retained | No |
| `Mathematics.IntegrateTrapezoidal` | `Numerics.Trapezoid` | Delegating wrapper retained with historical interval behavior | No |
| `Mathematics.IntegrateSimpson` | `Numerics.Simpson` | Delegating wrapper retained with historical interval behavior | No |
| RF-local speed-of-light value behind `RF.SpeedOfLightMetersPerSecond` | `Physics.SpeedOfLight` | RF name retained as delegating wrapper | No |
| RF-local Boltzmann value behind `RF.BoltzmannConstantWattsPerHzKelvin` | `Physics.BoltzmannConstantSI` | RF name retained as delegating wrapper | No |
| hard-coded `k_B` inside legacy `Wireless.ThermalNoiseFloorDbm` | `Physics.BoltzmannConstantSI` internally; `Wireless.*SI` for new callers | Legacy function unchanged externally | No |
| Wireless scalar-Hz records/functions | `WirelessBandSI`, `WirelessLinkBudgetSI`, `WirelessThroughputSI`, `*SI` functions | Original surface retained | No |
| scalar Cooking/Thermofluids temperature/conversion helpers | typed Cooking/Thermofluids APIs | Original surface retained | No |

No C3 package/source movement or C4 public break occurred.

## 24. Cross-library examples

The existing bounded examples remain sufficient:

- Cooling: `ComputationalScience` compares normalized simulation decay with `Thermofluids.LumpedTemperature`.
- RC decay: it compares the same decay shape with `Electromagnetism.RCDischargeVoltage`.
- Orbit: it composes `Astrophysics` with astronomical units.
- Biology: it demonstrates bounded population growth; public docs separately direct DNA measurements to `ComputationalBiology` plus generic `Statistics`.

Fresh-agent trials produced human-readable representative imports without adding a giant tutorial suite.

## 25. Fresh-agent trials

Each trial agent was restricted to the root/public READMEs, `docs/science/README.md`, relevant public package source/contracts/manifests, and language reference. No agent received internal consolidation notes.

| Trial | Intended package(s) | Agent choice | Corrections | Result |
| --- | --- | --- | ---: | --- |
| A — integrate `x^2`; solve `y'=y` | Numerics; DifferentialEquations | `Numerics.Simpson`; `DifferentialEquations.EulerSolve` | 0 | Pass; avoided Mathematics compatibility names and invented generic solvers |
| B — wall heat transfer with films | Thermofluids | `ConvectiveThermalResistance`, `PlaneWallThermalResistance`, `SeriesThermalResistance`, `HeatRateFromResistance` | 0 | Pass; dimensionally derived ~61.3 W and did not substitute conduction-only/legacy convection APIs |
| C — RC decay simulation | Electromagnetism + Simulation | domain exact relation through `Electromagnetism`, execution/trace through `Simulation` | 0 | Pass; rejected restating an RC derivative merely to force DifferentialEquations |
| D — DNA translation/GC statistic | ComputationalBiology + Statistics | typed parse/translate/GC plus `Statistics.Mean` | 0 | Pass; avoided manual GC counting, `Statistics.Average` hallucination, and unsupported slice syntax |

Common docs searched were the public science guide and owning package README, followed by source/contracts to confirm exact names. Total human corrections: zero. Trial D's sketch was source-validated rather than executed by the isolated agent; the authoritative package suites were subsequently verified by the main task's real CLI lane.

## 26. API compatibility

All changes are C0 (docs/discovery/metadata), C1 (compatibility delegation), or C2 (internal constant deduplication). All existing public names, package identities, signatures, unit types, sentinel policies, and invalid-domain contracts remain available. C3: none. C4: none.

## 27. Runtime/compile impact

There is no runtime registry or discovery machinery. RF compatibility functions add a trivial direct function delegation that compiles normally; Wireless reads the same constant through `Physics` and `BaseUnit`. These paths have no meaningful runtime impact. Documentation and manifest-description changes do not affect execution.

## 28. Test verification

| Verification | Result |
| --- | --- |
| 32 scientific packages, `--execution auto` | 842/842 pass; 825 runnable compiled; 0 interpreted fallback |
| same packages, `--execution interpreted` | 842/842 pass |
| repository `Libraries` invalid corpus | 24/24 `.octfail` pass, including dimensional and biological type failures |
| RF focused auto/interpreted | 59/59 in each mode; 59 runnable compiled; 0 fallback |
| Wireless focused auto/interpreted | 22/22 in each mode; 21 runnable compiled; 0 fallback |
| `go test ./...` | pass |
| `go vet ./...` | pass |
| `git diff --check` | recorded after final report edit; pass required for verdict |

The science auto/interpreted totals include all package-local invalid contracts. The separate Libraries root lane is the repository's cross-library invalid discovery lane. No compiled support was forced to interpreted mode.

## 29. What was deliberately NOT reorganized

No package directory moved. No import identity changed. No `Science.*`, `OctSci`, wildcard import, runtime API registry, or nested mega-directory was added. No broad API rename, universal result record, Units redesign, new scientific discipline, new solver family, or mass test-import rewrite was attempted.

## 30. Remaining ambiguity

The built-in production `FFT` and the `Mathematics` reference transform are two visible transform surfaces, but they are not equal owners: production code should use the built-in, while `Mathematics.FastFourierTransform` is an explicit pure-Oct reference/oracle and `Mathematics.IFFT` supplies the inverse. This is a documentation distinction, not a package-boundary blocker.

`Simulation` and `DifferentialEquations` remain scalar/fixed-step. That is a capability limit, not an ownership ambiguity and is outside consolidation scope.

## 31. Architecture decision

**A. Scientific package organization is now coherent and scales cleanly.**

## 32. Database-return decision

**D1. Science surface is consolidated enough; return to OctetDB/application work.**

## 33. Exactly one next recommendation

Return to the OctetDB/application roadmap and use this science guide as the stable navigation contract; do not schedule another science-library milestone until an application exposes one specific missing boundary.
