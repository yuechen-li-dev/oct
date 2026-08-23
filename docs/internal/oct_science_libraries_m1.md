# OCT-SCIENCE-LIBRARIES-M1 — Full-Spectrum Executable Science Expansion

Date: 2026-08-23

## 1. Verdict

Success

The real library path now spans mechanics, thermodynamics, fluid mechanics, heat transfer, electromagnetism, optics, astrophysics, quantum fundamentals, computational biology, chemistry/biochemistry, cooking science, and cross-discipline computation. The affected package lane passes 460/460 contracts in both auto and interpreted execution; all 450 runnable auto cases compile with zero fallbacks, and ten package-local dimensional/type failures pass as intended. The repository-wide Go test, vet, canonical-registry, and diff gates are green.

## 2. M0 baseline

M0 left a healthy numerical and foundational shelf: Algorithms, DifferentialEquations, Distributions, Geometry, Interpolation, LinearAlgebra, Numerics, Physics, Simulation, Statistics, and Thermofluids all had useful executable contracts. Mechanics was already substantial in continuum mechanics, stress, failure, fatigue, endurance, notch, and shafts. Units already modeled non-SI quantities as nominal records with explicit SI conversion.

The M0 report also named six unresolved areas. M1 treated them as follows:

| M0 issue | M1 disposition |
| --- | --- |
| `Mathematics.Calculus` vs `Numerics` | Consolidated compatibility-first; Mathematics delegates to canonical Numerics algorithms. |
| `Numerics.Optimization` vs `Optimization` | Documented boundary: scalar reference methods in Numerics, richer multivariate/derivative-free methods in Optimization. |
| Wireless scalar-Hz modernization | Added typed `Float<Hz>`, watt, kelvin, and link-budget APIs; retained legacy wrappers. |
| String/Structures/Time narrowness | Still narrow; explicitly retained as a later organization/usability concern rather than padded with unrelated formulas. |
| Simulation scalar/fixed-step limitation | Still scalar/fixed-step; added a checked runtime Concept-admission constructor and used normalized cross-library examples. |
| `IO.WriteTable` placeholder | Still an explicit external-transport stub; not disguised as science work. |

## 3. M1 philosophy and scope

The implementation follows the executable-textbook rule: connected equations live beside assumptions, domain checks, dimensional signatures, and worked contracts. It favors coherent chapters over formula count. No Oct language feature, heavyweight dependency, property database, or general solver framework was added.

Seven top-level libraries were added, while the strongest existing homes were expanded instead of duplicated. Physics owns foundational laws; Mechanics owns engineering analysis. Thermofluids owns thermodynamics, fluid mechanics, dimensionless analysis, and heat transfer. Chemistry complements rather than replaces ChemistryNmr. ComputationalScience composes existing models and Simulation rather than copying solvers.

## 4. Repository/library audit

The audit started from the 50-library M0 tree and ended with 57 top-level libraries.

| State | Areas |
| --- | --- |
| Healthy before and preserved | Algorithms, DifferentialEquations, Distributions, Geometry, Interpolation, LinearAlgebra, Statistics, existing Mechanics chapters |
| Substantially deepened | Physics, Mechanics, Thermofluids |
| Modernized | Units, Cooking, Simulation, Wireless |
| Consolidated | Mathematics.Calculus/Numerics and Numerics.Optimization/Optimization |
| New coherent chapters | Electromagnetism, Optics, Astrophysics, Quantum, ComputationalBiology, Chemistry, ComputationalScience |
| Still narrow | String, Structures, Time |
| Explicit stub | `IO.WriteTable` |

The canonical registry now includes every new M1 package and the previously absent local prerequisites needed by those dependency graphs, including Units, Wireless, and Tensor2D. The reserved standard namespace `String` was intentionally not registered as a package; ComputationalBiology uses the OctStd `String` namespace and therefore declares only OctStd.

## 5. Scientific API consolidation

`Mathematics.Calculus` remains source-compatible but imports Numerics and delegates differentiation, trapezoidal integration, and Simpson integration to it. Its wrappers preserve established Mathematics behavior for equal or reversed interval endpoints. This makes Numerics the obvious implementation and teaching entry point without deleting old names.

The optimization boundary is now explicit in both READMEs and registry descriptions:

- Numerics owns deterministic scalar reference methods, including golden-section and Brent minimization.
- Optimization owns the larger surface: gradient methods, line searches, least squares, Nelder-Mead, and related multivariate algorithms.

No working API was removed.

## 6. Mechanics

The deep audit found that continuum, fatigue, failure, notch, shaft, and stress work was already real and should not be cloned. The missing engineering chapter became `Mechanics.Structures`:

- rectangular area and centroidal second moment;
- axial strain, scalar Hooke law, and thermal strain/expansion;
- uniform prismatic-bar deformation;
- simply supported center-load moment and deflection;
- cantilever end-load moment and deflection;
- Euler ideal-column buckling;
- thin-wall closed-cylinder hoop and longitudinal stress;
- Saint-Venant shaft angle of twist;
- a record-shaped `MohrCirclePlaneStress` Concept.

The README states small-strain, Euler–Bernoulli, ideal-column, thin-wall, and linear-elastic limits. Singular geometry is fallible, and a dimensional failure proves that energy cannot stand in for beam force.

## 7. Thermodynamics

`Thermofluids.Thermodynamics` now forms a connected ideal-gas and energy chapter:

- specific gas constant from the canonical universal gas constant;
- ideal-gas pressure and volume;
- sensible heat, ideal-gas internal-energy change, and enthalpy change;
- the closed-system first-law sign convention `Q_in = ΔU + W_out`;
- heat-capacity ratio and reversible adiabatic temperature relation;
- thermal efficiency, Carnot efficiency, refrigerator COP, and heat-pump COP;
- supplied-property latent heat.

Tests cover constant-volume/constant-pressure heating interpretations, first-law composition, isentropic pressure change, Carnot bounds, and COP identities. The API says “ideal gas” in its names and comments and does not imply real-fluid property support.

## 8. Fluid mechanics

The new foundation includes density, specific weight, hydrostatic pressure, Archimedes buoyancy, volume and mass flow, incompressible continuity, dynamic pressure, the Bernoulli pressure sum, pressure head, hydraulic diameter, laminar Darcy factor, bounded Haaland factor, and Darcy–Weisbach pressure loss.

Bernoulli documents steady, incompressible, inviscid streamline assumptions with no shaft work or loss. Haaland rejects transition flow, Reynolds below 4000, and relative roughness outside `[0, 0.05]`; the laminar relation is bounded to `0 < Re <= 2300`. The tests connect hydrostatics to buoyancy, continuity to area change, Bernoulli terms to identical pressure dimensions, and friction factor to pressure drop.

## 9. Dimensionless analysis

Fourteen typed groups are present: Reynolds, Mach, Froude, Euler, Prandtl, Nusselt, Grashof, Rayleigh, Biot, Fourier, Peclet, Weber, Schmidt, and Sherwood. Each source comment states the ratio's physical meaning and formula. Inputs use dimensional types, return types are `Float`, and compiled facts assign/use the results as dimensionless values.

Representative lessons include pipe-air Reynolds scale, gravity-wave versus sound-speed ratios, thermal diffusivity pairs, buoyancy-driven Rayleigh composition, lumped-transient Biot/Fourier interpretation, and mass-transfer Peclet/Schmidt/Sherwood relations. Singular denominators are rejected.

## 10. Heat transfer

The bounded chapter covers plane-wall steady conduction, wall resistance, convective resistance, series resistance, heat rate from resistance, typed Newton convection, diffuse-gray radiation to large surroundings, lumped thermal time constant, exact lumped temperature, and the conventional `Bi < 0.1` screening helper.

The README now includes a qualified external example composing two convective films with wall conduction and explains the base-SI spellings for W/(m·K), W/(m²·K), K/W, and W. The sign convention is explicit. Tests make resistance and direct conduction agree, compose layered resistance, check radiation at equal temperatures, and recover the one-time-constant exponential.

## 11. Electromagnetism

The new package progresses through:

- vacuum point-charge force, field, potential, and potential energy;
- ideal parallel-plate capacitance and capacitor energy;
- Ohm law, electric power, series/parallel resistance, RC time constant, and discharge;
- magnetic flux, charge/wire force magnitudes, and the ideal long-wire field;
- one-turn Faraday induction, inductor energy, and RL time constant.

Charge, voltage, resistance, capacitance, magnetic field/flux, and inductance use ordinary SI base expressions with `A`; no string pseudo-units were invented. The README states point-charge, fringing-free plate, lumped-element, uniform-field, and infinite-wire assumptions. A fresh-agent correction now distinguishes voltage/charge/current half-time `τ ln 2` from energy half-time `τ ln 2 / 2`.

## 12. Optics/waves

Physics gained the reusable wave relations `v=fλ`, angular frequency, wave number, beats, and fixed-string harmonics. Optics builds on that foundation with a refined positive `RefractiveIndex`, Snell refraction, critical angle/TIR, a record-shaped `OpticalImage` Concept, thin-lens results, lens power, two-slit spacing, single-slit minima, grating maxima, Rayleigh resolution, and the photon wavelength/frequency/energy bridge.

Angles are documented as dimensionless radians. Thin-lens and two-slit approximations are explicit. Non-propagating diffraction orders and impossible transmitted rays return errors instead of NaN.

## 13. Units modernization

The Core, American, British, Astronomical, Atomic, and Spectroscopy families were audited. They already follow the current policy: non-SI units are nominal records and conversions expose typed SI values. M1 added US tablespoon and teaspoon records/converters with exact customary relationships to the existing cup/fluid-ounce definitions, plus a package README explaining exact/conventional status.

The seven SI base dimensions already let the new libraries express Pa, J, W, C, V, ohm, farad, henry, tesla, weber, Hz, density, viscosity, conductivity, diffusivity, and heat-transfer coefficients directly. Named derived aliases were not added because ordinary expressions remained readable and compiler checked.

## 14. Cooking modernization/science

Legacy scalar conversion functions remain callable, but their arithmetic now routes through typed Units conversions where compatible. `Cooking.Typed` adds:

- typed SI-to-ounce/pound/cup/fluid-ounce/tablespoon/teaspoon presentation;
- typed kelvin-to-Fahrenheit presentation;
- baker percentage and inverse mass calculation;
- density-backed `IngredientQuantity` construction;
- trim yield and `BrineComposition` mass fraction;
- a documented equal-specific-heat dough mixing estimate;
- cooling/resting through the canonical Thermofluids lumped model.

The README corrects obsolete error-handling examples and frames doneness/safety and leavening helpers as compatibility references, not universal regulatory or biochemical laws.

## 15. Astrophysics/orbits

The new package covers Newtonian gravitation, gravitational parameter, circular speed and period, escape speed, specific orbital energy, vis-viva, Hill radius, blackbody luminosity, inverse-square radiant flux, Wien displacement, distance modulus, parallax, and redshift. `CircularOrbit` and `DistanceObservation` are domain Concepts with public builders.

Worked cases recover Earth escape speed, a 400-km low-Earth-orbit scale, the `sqrt(2)` escape/circular ratio, Earth's Hill radius, solar luminosity/one-AU flux, the wavelength-form blackbody peak near 500 nm, ten-parsec distance modulus, and redshift scaling. The public docs now say radius is from the central body's center and specific energy is J/kg. Units.Astronomical owns AU, parsec, masses, radius, and luminosity conversions.

## 16. Quantum fundamentals

The deliberately small chapter includes `E=hf`, energy-frequency inversion, non-relativistic de Broglie wavelength, the infinite one-dimensional box, a non-relativistic hydrogenic Z=1 level formula, uncertainty product and `ħ/2`, Born weights, finite discrete-state normalization, and a diagonal dimensionless expectation value.

`QuantumLevel` is a refined positive-integer Concept with runtime admission. `DiscreteProbabilityState` keeps normalized probabilities and the original norm together. Tests cover visible-photon/electron-wavelength scales, `n²` box scaling, hydrogen levels, non-normalizable states, complex phase invariance, and probability normalization. No Schrödinger solver or quantum-computing framework is implied.

## 17. Computational biology

`DNABase` and `RNABase` enums are the canonical alphabets. Exhaustive `match` owns symbol rendering, complement, and transcription. Consensus uses enum-targeted `when utility DNABase` with named candidate scores and stable A/C/G/T tie order. This is a scientifically natural use of the user's requested enum/match/judgment pattern rather than a nested selection ladder.

The sequence chapter includes validation, composition/GC, reverse complement, coding-strand transcription, Hamming distance, identity, consensus, overlapping k-mer counts, the standard nuclear genetic code, and in-frame translation. `TranslateTypedDNA` avoids forcing enum callers back to strings. Documentation distinguishes a 5′→3′ coding strand from a template strand and treats `"Stop"` as a termination token, not an amino acid. Complexity notes label the O(n), O(n·k), and small educational algorithms.

Population helpers add Hardy–Weinberg genotype frequencies plus exponential and logistic growth, with equilibrium/model assumptions stated.

## 18. Chemistry/biochemistry additions

The new Chemistry package complements ChemistryNmr. It provides typed molarity and solution state, conservation-based dilution, ideal concentration pH and inverse, Henderson–Hasselbalch, Beer–Lambert absorbance, first-order reaction progress/half-life, Arrhenius rate, Michaelis–Menten rate, and Hill occupancy.

The source states activity, homogeneous-sample, quasi-steady single-substrate, constant-activation-energy, and phenomenological-cooperativity assumptions. Reference facts prove concentration round trips, equal acid/base pH behavior, dimensionless absorbance, half-life, Arrhenius monotonicity, and half-max kinetic cases.

## 19. Other discretionary science

Physics gained a compact waves chapter and foundational particle/rotational dynamics rather than spawning more top-level packages. Wireless gained a parallel canonical SI surface for free-space loss, thermal noise, power conversion, link margin, and throughput. These additions were bounded and reused Physics constants.

No optional acoustics, geoscience, relativity, epidemiology, or chaotic-dynamics package was added; the required shelf already reached broad coherent coverage.

## 20. Simulation/cross-library examples

ComputationalScience contains executable mini-lessons rather than another solver:

- normalized Euler decay with exact error and step-size convergence;
- the shared exponential shape of lumped cooling and RC discharge;
- logistic population growth bounded by carrying capacity;
- one-AU period from solar mass and astronomical units.

`DecayExperiment` is a record-shaped Concept joining trace, exact result, numerical result, and error. Simulation remains honestly scalar and fixed-step. `NewFixedStepConfig` is the single checked runtime admission boundary for dynamic `dt` and step-count inputs.

## 21. Modern Oct usage

Features were used only where they clarified the science:

| Feature | Natural use |
| --- | --- |
| Refined Concept + `Require` | positive mass/duration/length, refractive index, quantum level, fixed-step configuration |
| Record-shaped Concept | orbit, distance observation, optical image, probability state, solution/reaction state, ingredient/brine state, wireless models, decay experiment, Mohr circle |
| `match` | exhaustive DNA/RNA alphabet transforms |
| `when utility` | deterministic consensus-base selection from four bounded candidates |
| Fallible functions | singular geometry, impossible optical orders, invalid temperatures/concentrations/sequences, non-normalizable states |
| Units | physical meaning retained in signatures and expression algebra |
| `with` and `batch` | retained in Simulation for scenario variants and deterministic independent sweeps |

Templates, FLOW, query, and additional batch use were not added merely for decoration. Existing canonical Numerics FLOW solvers remain the right home for those algorithms.

## 22. Physical constants/provenance

Physics is the canonical source. M1 added fully typed SI forms for Boltzmann, elementary charge, Avogadro, gas, Stefan–Boltzmann, vacuum permittivity, vacuum permeability, gravitational, and Wien constants while preserving older scalar/partially dimensional compatibility functions.

Comments distinguish:

- SI-defined exact values: `c`, `h`, `e`, `k_B`, and `N_A`;
- exact values derived from those definitions, such as the universal gas constant;
- post-2019 measured/derived values for `epsilon_0` and `mu_0`;
- measured values such as `G` and measured/derived Wien displacement;
- a documented conventional rounded Stefan–Boltzmann value.

Science packages import Physics rather than redeclaring constants. Standard equations are named in comments. The bounded Haaland correlation is identified by name and validity range; no citation was fabricated.

## 23. Numerical stability/model assumptions

| Field | Dimensional/formula review | Assumptions and stability |
| --- | --- | --- |
| Mechanics | Analytic units reduce correctly through beams, buckling, vessels, torsion, and Mohr circle. | Linear elasticity, small deflection/strain, slender column, thin wall; singular geometry rejected. |
| Thermofluids | Energy, pressure, flow, resistance, and all fourteen ratios were checked algebraically and by compiled contracts. | Ideal gas, constant properties, Bernoulli restrictions, bounded friction regimes, lumped `Bi < 0.1` screen. |
| Electromagnetism | Electrical/magnetic SI base expressions compose to force, energy, time, and power. | Vacuum/ideal geometry/lumped elements; zero denominators rejected. |
| Optics | Length/frequency/energy and dimensionless angles/magnification compose correctly. | Paraxial/small-angle/ideal aperture; inverse-trig domains checked before evaluation. |
| Astrophysics | Newtonian orbit and radiant-flux dimensions match analytic scales. | Point/spherical masses, negligible test-body mass, no perturbations/relativity; blackbody approximation. |
| Quantum | Action, wavelength, energy, and probabilities reduce correctly. | Non-relativistic textbook models; finite arrays use explicit normalization and tolerance. |
| Biology | Discrete types prevent alphabet mixing; probabilities/growth remain dimensionless or Hz×s. | Small sequences, standard nuclear code, explicit strand orientation, ideal population models. |
| Chemistry | mol/m³, J/mol, Hz, and dimensionless log/exponential arguments are explicit. | Ideal concentrations and bounded textbook kinetic models. |

Analytic functions remain analytic. Iterative work reuses existing deterministic solvers. Empirical correlations reject unsupported ranges instead of extrapolating silently.

## 24. Error/failure contracts

Fallible APIs cover non-positive denominators, invalid absolute temperatures, non-physical efficiencies, unsupported friction regimes, impossible inverse-trig arguments, coincident charges, zero orbit radii, invalid quantum levels/states, malformed DNA/RNA, unequal Hamming lengths, out-of-range probabilities, and invalid chemistry concentrations/rates.

Errors are used for mathematical/model-domain failures. They are not replaced with NaN, silent clamping, or compiler fallback. The legacy Thermofluids tank-level clamp remains its documented compatibility policy.

## 25. Dimensional-invalid corpus

Ten affected package-local `.octfail` contracts teach the relevant mistake directly:

- Physics: time passed as velocity;
- Mechanics: energy passed as beam force;
- Thermofluids: length passed as dynamic viscosity;
- Electromagnetism: mass passed as charge;
- Optics: time passed as focal length;
- Astrophysics: duration passed as orbit radius;
- Quantum: mass passed as frequency;
- Chemistry: energy passed as temperature;
- Wireless: time passed as frequency;
- ComputationalBiology: an RNA enum passed to a DNA function.

The cross-library invalid lane discovered and passed all 24 repository `.octfail` cases.

## 26. Test coverage

Affected package auto results:

| Package | Passed | Runnable compiled | Fallback |
| --- | ---: | ---: | ---: |
| Physics | 20/20 | 19 | 0 |
| Mechanics | 79/79 | 78 | 0 |
| Thermofluids | 44/44 | 43 | 0 |
| Electromagnetism | 17/17 | 16 | 0 |
| Optics | 12/12 | 11 | 0 |
| Units | 44/44 | 44 | 0 |
| Cooking | 49/49 | 49 | 0 |
| Astrophysics | 11/11 | 10 | 0 |
| Quantum | 12/12 | 11 | 0 |
| ComputationalBiology | 17/17 | 16 | 0 |
| Chemistry | 11/11 | 10 | 0 |
| ComputationalScience | 5/5 | 5 | 0 |
| Mathematics | 21/21 | 21 | 0 |
| Numerics | 45/45 | 45 | 0 |
| Optimization | 40/40 | 40 | 0 |
| Simulation | 12/12 | 12 | 0 |
| Wireless | 21/21 | 20 | 0 |
| **Total** | **460/460** | **450** | **0** |

The ten-case difference is intentional compile-time `.octfail` coverage. Major equations have analytic identities, known physical scales, boundary/domain cases, dimensional failures, and cross-library examples rather than arbitrary numeric spot checks.

Repository gates:

- `go test ./...` — pass;
- `go vet ./...` — pass;
- canonical registry/package-manager tests — pass;
- `git diff --check` — pass.

## 27. Interpreter/compiler parity

All 17 affected packages pass the same 460 contracts under `--execution interpreted`. Under `--execution auto`, every one of the 450 runnable cases uses the compiled backend and reports zero interpreted fallback. Package manifests and qualified imports exercise the real dependency path. Representative compiled lessons exist in every major new package.

## 28. Fresh-agent science trial

Four fresh read-only agents received only public package source, README, manifests, contracts, and normal language reference when needed. They did not receive compiler-internal documentation.

| Trial | What the agent accomplished | Misunderstanding/friction found | Correction applied |
| --- | --- | --- | --- |
| Heat transfer | Composed inside film + wall + outside film and explained ~61 W outward heat loss. | Could confuse total resistance with conduction alone; external qualification and derived SI forms were not obvious. | Added a qualified README mini-lesson, sign convention, and W/K-derived-unit map. |
| Electromagnetism | Safely proposed RC half-voltage time from the public time constant. | “Half-life” differs for voltage/charge/current versus stored energy; zero `tau` composition needed care. | Added qualified quick start and the exact half-time/zero-time distinction. |
| Astrophysics | Built a 400-km Earth circular-orbit example using Units. | Radius could be mistaken for altitude; specific energy could be mistaken for total energy; distance Concept was unused. | Clarified radius/energy, added quick start, renamed the misleading “green Sun” fact, and added `DistanceObservationAt`. |
| Computational biology | Used enum DNA, exhaustive `match`, `when utility`, coding-strand transcription, and translation. | Coding versus template strand and `"Stop"` semantics needed stating; typed callers had to render to strings to translate. | Documented orientation/termination/tie behavior, added quick start, and added compiled `TranslateTypedDNA`. |

No trial found a dimensional error in the implemented equations. Trial snippets were read-only proposals; their component paths and the applied corrections were subsequently verified in the real compiled package contracts. The trial improved API discoverability rather than requiring compiler knowledge.

## 29. API compatibility

Compatibility was additive:

- old Physics constant functions remain wrappers around or companions to typed canonical forms;
- old Cooking scalar conversions remain, with typed APIs preferred;
- old Thermofluids scalar-temperature functions remain, with typed Kelvin APIs preferred;
- old Wireless scalar-Hz records/functions remain because changing the historical `TxPowerW` dimension would be breaking; `*SI` APIs are canonical;
- Mathematics calculus names remain wrappers over Numerics;
- existing Mechanics chapters and package versions remain intact.

READMEs and registry descriptions identify the preferred modern paths. No gratuitous deletion or rename was made.

## 30. Library inventory diff

Top-level libraries added:

- Astrophysics
- Chemistry
- ComputationalBiology
- ComputationalScience
- Electromagnetism
- Optics
- Quantum

Existing libraries substantially expanded:

- Mechanics
- Physics
- Thermofluids

Libraries modernized:

- Cooking
- Units
- Simulation
- Wireless

Libraries consolidated:

- Mathematics.Calculus → Numerics canonical implementations
- Numerics.Optimization / Optimization documented scalar-reference versus richer-solver boundary

Stubs/limitations remaining:

- `IO.WriteTable` external transport placeholder;
- String, Structures, and Time remain narrow;
- Simulation remains scalar and fixed-step;
- Wireless retains legacy scalar APIs beside the typed canonical surface.

## 31. Modernization table

| Area | Before | After | Modern Oct feature used | Why better |
| --- | --- | --- | --- | --- |
| Mechanics | Strong specialist stress/fatigue/continuum chapters; missing basic structures | Beams, axial/thermal deformation, buckling, vessels, torsion, Mohr circle | Units, fallible functions, record-shaped Concept | Fills the engineering progression without duplicating foundational dynamics |
| Thermofluids | Lumped thermal and tank/process helpers | Thermodynamics, fluid foundations, 14 dimensionless groups, heat transfer | Units, fallible APIs, records/composition | Equations expose dimensions, assumptions, correlations, and lessons |
| Electromagnetism | N/A | Charge through induction and lumped RC/RL | Typed `A` dimensions, fallible functions | Compiler checks electrical/magnetic algebra instead of relying on unit names |
| Optics | N/A | Refraction, lenses, interference, diffraction, resolution, photons | Refined and record-shaped Concepts, units | Positive indices and invalid propagation domains are explicit |
| Units | Modern nominal wrappers but missing culinary spoons/docs | Audited families plus tablespoon/teaspoon and clear SI policy | Records, `Float<Unit>` conversions | Practical units remain typed and exact relationships are tested |
| Cooking | Many scalar conversion/scaling APIs | Typed mass/volume/temperature, baker %, density, brine, mixing, cooling | Concepts, units, cross-package fallibility | Practical compatibility remains while new work is dimension-safe |
| Astrophysics | N/A | Orbits, radiation, distances, parallax, redshift | Concepts, units, imported constants | Known scales are readable and astronomical units are not redeclared |
| Quantum | N/A | Photon/matter waves, levels, uncertainty, Born probabilities | Refined/record Concepts, Complex, units | Small textbook models state approximations and reject invalid states |
| Computational biology | N/A | Typed sequences, genetic code, population genetics/growth | Enums, exhaustive `match`, `when utility`, Concepts | Alphabet transforms are closed and consensus decisions are deterministic/inspectable |

## 32. What was deliberately not built

M1 did not build CFD, FEA, a property/steam-table database, industrial ray tracing, a general electromagnetic field/circuit solver, a quantum simulator, quantum chemistry, molecular dynamics, a genome database/aligner, an N-body engine, a climate model, a symbolic CAS, or a SciPy clone. No new compiler feature or external heavyweight dependency was introduced.

## 33. Remaining weak scientific areas

- Package discovery is now the main breadth cost: 57 top-level libraries need a subject index and clearer bookshelf navigation.
- String, Structures, and Time remain intentionally narrow.
- Simulation is still scalar/fixed-step; vector state and adaptive stepping need a separate contract, not nested-array improvisation.
- `IO.WriteTable` remains an honest transport placeholder.
- Legacy Cooking, Thermofluids, and Wireless scalar APIs remain visible for compatibility, although the modern path is documented.
- Named derived SI aliases are absent. This was not painful in implementation, but README unit maps remain important for newcomers.

## 34. Architecture decision

**B. It scales, but package discovery/organization now needs a dedicated consolidation pass.**

The executable-textbook model remained coherent across all new domains. The recurring fresh-agent friction was finding/importing the right surface and reading base-SI derived forms, not missing scientific or compiler capability.

## 35. Units decision

**U1. Modern typed units now cover practical scientific/engineering authoring well.**

The seven base dimensions expressed every M1 physical equation cleanly. No repeated unit-system blocker appeared. Explicit base expressions are verbose in electrical and heat-transfer APIs, but they are correct, composable, and teachable with compact unit maps.

## 36. Next-science decision

**S3. Scientific package/API consolidation should happen before more expansion.**

The shelf is broad enough to validate the model. The next constraint is navigation and public-surface coherence, not a missing scientific domain or numerical primitive.

## 37. Exactly one next recommendation

Run one dedicated scientific-library navigation and API-consolidation milestone that produces a subject-oriented bookshelf index, compact public API/unit maps, and canonical-versus-compatibility guidance across the 57-library tree, without adding another scientific field.
