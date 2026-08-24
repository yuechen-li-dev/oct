# Science libraries

Oct's scientific packages are organized by responsibility rather than by implementation history. Start with the scientific task below; compatibility APIs remain available, but each concept has one preferred owner.

## Where do I find X?

| Task or concept | Start here | Boundary |
| --- | --- | --- |
| elementary mathematical helpers | [`Mathematics`](../../Libraries/Mathematics/README.md) | Definitions, scalar helpers, and complex transforms; not numerical solver selection. |
| differentiate a function numerically | [`Numerics`](../../Libraries/Numerics/README.md) | Finite differences with explicit method choice. `Mathematics` calculus names are compatibility wrappers. |
| integrate a function / quadrature | [`Numerics`](../../Libraries/Numerics/README.md) | Trapezoid, Simpson, Gauss-Legendre, and adaptive Simpson. |
| find a scalar root | [`Numerics`](../../Libraries/Numerics/README.md) | Bisection, Newton, secant, and Brent root methods. |
| minimize a bounded scalar function | [`Numerics`](../../Libraries/Numerics/README.md) | Transparent one-dimensional golden-section and Brent methods. |
| gradient descent, Nelder-Mead, or nonlinear least squares | [`Optimization`](../../Libraries/Optimization/README.md) | Rich multivariate optimization workflows and diagnostics. |
| solve an ordinary differential equation | [`DifferentialEquations`](../../Libraries/DifferentialEquations/README.md) | Euler, midpoint, and RK4 integration methods. |
| execute a scenario, collect a trace, or sweep parameters | [`Simulation`](../../Libraries/Simulation/README.md) | Deterministic scenario execution; it does not own ODE algorithms. |
| sampled-data differences or accumulation | [`Analysis`](../../Libraries/Analysis/README.md) | Operations on observed `Float[]` series, not functions. |
| interpolate tabulated data | [`Interpolation`](../../Libraries/Interpolation/README.md) | Interpolation algorithms, separate from statistics and ODEs. |
| descriptive statistics or regression | [`Statistics`](../../Libraries/Statistics/README.md) | Summaries, regression, and fitted utility models. |
| evaluate a probability distribution | [`Distributions`](../../Libraries/Distributions/README.md) | PDF/CDF/PMF evaluation. Use `Random` for sampling. |
| generate reproducible random samples | [`Random`](../../Libraries/Random/README.md) | RNG, sampling, dice, and coin helpers; not distribution evaluation. |
| propagate measurement uncertainty | [`Uncertainty`](../../Libraries/Uncertainty/README.md) | First-order independent uncertainty propagation. |
| complex-valued special functions | [`Complex`](../../Libraries/Complex/README.md) | Extends the language's built-in complex type. |
| vectors and matrices | [`LinearAlgebra`](../../Libraries/LinearAlgebra/README.md) | General linear algebra. |
| pointwise 2D gradient, Jacobian, or divergence | [`Tensor2D`](../../Libraries/Tensor2D/README.md) | Numerical tensor calculus in coordinate dimension two. |
| planar or solid geometry | [`Geometry`](../../Libraries/Geometry/README.md) | Geometric relations and bounded polygon foundations. |
| generic search and discrete algorithms | [`Algorithms`](../../Libraries/Algorithms/README.md) | Reusable algorithms, not domain-specific biology or signal processing. |
| physical constants or foundational laws | [`Physics`](../../Libraries/Physics/README.md) | Shared constants, force/energy/dynamics, and elementary waves. |
| stress, beams, fatigue, or pressure vessels | [`Mechanics`](../../Libraries/Mechanics/README.md) | Engineering mechanics and analysis models. |
| thermodynamics, heat transfer, or fluid flow | [`Thermofluids`](../../Libraries/Thermofluids/README.md) | Thermal and fluid relations; use an ODE/simulation package for time integration. |
| electrostatics, circuits, magnetism, or induction | [`Electromagnetism`](../../Libraries/Electromagnetism/README.md) | Foundational EM models and lumped electrical relations. |
| refraction, lenses, interference, or diffraction | [`Optics`](../../Libraries/Optics/README.md) | Optical models; generic wave relations and constants stay in `Physics`. |
| foundational quantum models | [`Quantum`](../../Libraries/Quantum/README.md) | Bounded textbook quantum relations, not a general simulator. |
| orbits, stellar radiation, or astronomical distance | [`Astrophysics`](../../Libraries/Astrophysics/README.md) | Astronomy applications built on `Physics` and `Units`. |
| solution chemistry or kinetics | [`Chemistry`](../../Libraries/Chemistry/README.md) | General chemistry relations. |
| NMR spectra and chemical shifts | [`ChemistryNmr`](../../Libraries/ChemistryNmr/README.md) | Specialized NMR construction and analysis. |
| DNA/RNA, translation, or population genetics | [`ComputationalBiology`](../../Libraries/ComputationalBiology/README.md) | Biological types and algorithms; generic summaries remain in `Statistics`. |
| convolution, correlation, or moving average | [`Signal`](../../Libraries/Signal/README.md) | Generic finite-sequence DSP. |
| FFT/IFFT | [`Mathematics`](../../Libraries/Mathematics/README.md) | Built-in `FFT` for production; `Mathematics` contains the explicit reference transform and IFFT. |
| RF propagation, fading, S-parameters, or MIMO channel math | [`RF`](../../Libraries/RF/README.md) | First-principles radio-frequency system mathematics. |
| link budget, bands, OFDMA, or throughput | [`Wireless`](../../Libraries/Wireless/README.md) | Communication/link-oriented calculations; prefer its typed `*SI` surface. |
| SI dimensions or non-SI conversion | [`Units`](../../Libraries/Units/README.md) | SI is built into the language; the package owns explicit non-SI/domain conversions. |
| culinary scaling, brines, or ingredient conversions | [`Cooking`](../../Libraries/Cooking/README.md) | Practical culinary calculations composed with `Units` and `Thermofluids`. |
| cross-disciplinary executable lessons | [`ComputationalScience`](../../Libraries/ComputationalScience/README.md) | Composition examples only; it does not own laws or solvers. |

## Taxonomy

### Mathematical foundations

- `Algorithms` — reusable discrete/search algorithms.
- `Complex` — complex special functions beyond the built-in complex primitives.
- `Geometry` — scalar geometry and bounded polygon foundations.
- `LinearAlgebra` — vector and matrix algorithms.
- `Mathematics` — elementary definitions/helpers and transforms.
- `Tensor2D` — pointwise numerical tensor calculus.

### Numerical computation

- `Analysis` — observed discrete-series analysis.
- `DifferentialEquations` — mathematical ODE integration methods.
- `Interpolation` — interpolation of tabulated data.
- `Numerics` — scalar approximation algorithms.
- `Optimization` — richer multivariate optimization.
- `Simulation` — trace-oriented scenario execution and sweeps.
- `ComputationalScience` — cross-library executable lessons.

### Statistics, probability, and uncertainty

- `Statistics` — descriptive statistics, regression, and data fitting.
- `Distributions` — deterministic distribution evaluation.
- `Random` — reproducible random generation and sampling.
- `Uncertainty` — first-order measurement-uncertainty propagation.

### Physical sciences

- `Physics` — shared constants and foundational laws.
- `Mechanics` — engineering mechanics.
- `Thermofluids` — thermodynamics, fluids, and heat transfer.
- `Electromagnetism` — electrostatics, circuits, magnetism, and induction.
- `Optics` — optical systems and wave-optics relations.
- `Quantum` — bounded foundational quantum models.
- `Astrophysics` — orbital, stellar, and astronomical applications.

### Chemistry and life science

- `Chemistry` — general solution chemistry and kinetics.
- `ChemistryNmr` — specialized NMR spectroscopy.
- `ComputationalBiology` — typed biological sequences and population models.

### Signal and communications engineering

- `Signal` — domain-neutral finite-sequence DSP.
- `RF` — radio-frequency propagation, channel, and network mathematics.
- `Wireless` — communication bands, link budgets, and throughput.

### Scientific foundations and applications

- `Units` — non-SI/domain records and explicit conversion to compiler-owned SI dimensions.
- `Cooking` — practical culinary science.

## Boundaries users should remember

### Mathematics, Numerics, and Optimization

Use `Mathematics` when the operation is a mathematical helper or transform. Use `Numerics` when choosing a scalar approximation algorithm, tolerance, iteration bound, or convergence record. Use `Optimization` for multivariate solver workflows such as gradient descent, Nelder-Mead, and nonlinear least squares.

`Mathematics.DifferentiateCentral`, `Mathematics.IntegrateTrapezoidal`, and `Mathematics.IntegrateSimpson` are compatibility textbook names that delegate to `Numerics`; new numerical code should import `Numerics`. Bounded one-dimensional minimization remains in `Numerics`; the richer `Optimization` package does not duplicate it.

### DifferentialEquations and Simulation

`DifferentialEquations` owns integration algorithms for a derivative callback. `Simulation` owns transition/observation execution, aligned traces, and parameter sweeps. A physical package supplies the model relation. For example, `Thermofluids` supplies a cooling relation, `DifferentialEquations` can integrate its normalized ODE, and `Simulation` can run repeated scenarios.

### Physics and specialized physical sciences

`Physics` owns universally reused constants and foundational relations. Specialized packages own their scientific interpretation and analysis:

- `Mechanics`: engineering stress, deformation, fatigue, structures, and continuum helpers.
- `Thermofluids`: thermodynamic, fluid, and heat-transfer models.
- `Electromagnetism`: electrostatic, circuit, and magnetic models.
- `Optics`: refraction, lenses, interference, diffraction, and photon-facing optics.
- `Quantum`: non-relativistic textbook quantum models.
- `Astrophysics`: orbital, stellar, and astronomical applications.

Specialized packages import `Physics`; they do not own competing physical constants.

### Signal, RF, and Wireless

`Signal` owns generic DSP operations on finite sequences. `RF` owns first-principles radio-frequency propagation, fading, multipath, Doppler/coherence, MIMO channel, and S-parameter mathematics. `Wireless` owns communication/link-facing bands, link budgets, OFDMA, and throughput estimates. The packages compose but are not interchangeable.

### Chemistry and NMR

`Chemistry` owns general solution relations and bounded kinetics. `ChemistryNmr` owns NMR-specific frequency, shift, peak, multiplicity, and spectrum operations. `Units.Spectroscopy` owns reusable nominal presentation units such as ppm and reciprocal centimetres; it is not an NMR solver.

## Scientific dependency direction

This is the intended major-package layering, not every manifest edge:

```text
compiler SI dimensions
        |
        +--> Units
        +--> Mathematics / Geometry / Algorithms / Complex
                    |
                    +--> Numerics / LinearAlgebra / Statistics
                              |
                              +--> Optimization / Uncertainty / Tensor2D

Physics (canonical constants and laws)
        |
        +--> Mechanics / Thermofluids / Electromagnetism / Optics / Quantum
        |                                                        |
        +--------------------------------------------------------+--> Astrophysics
        +--> RF / Wireless

DifferentialEquations ----+
Simulation ---------------+--> ComputationalScience examples

Units + Thermofluids -----> Cooking
Chemistry ----------------> domain workflows (without owning generic solvers)
ComputationalBiology -----> domain workflows (without owning generic statistics)
```

The actual manifests remain deliberately sparse. There is no `Science.*` mega-package and no wildcard import.

## Constants and units

`Physics` is the canonical source for the speed of light, Planck and reduced Planck constants, Boltzmann constant, elementary charge, Avogadro constant, gas constant, Stefan-Boltzmann constant, vacuum permittivity/permeability, gravitational constant, Wien displacement constant, standard gravity, and particle rest masses.

- Exact SI-defined: `SpeedOfLight`, `PlanckConstant`, `BoltzmannConstantSI`, `ElementaryChargeSI`, and `AvogadroConstantSI`.
- Exact values derived from SI definitions: `GasConstantSI`.
- Measured or derived after the 2019 SI revision: vacuum permittivity and permeability.
- Measured: gravitational constant and the stored particle masses.
- Conventional rounded approximation: the package's Stefan-Boltzmann value, as documented beside its definition.

Legacy RF and Wireless entry points delegate to the `Physics` definitions. `Units.Spectroscopy.FrequencyToWavenumber` retains the exact speed-of-light numeric value locally to preserve the foundation-first dependency direction: `Units` must not depend upward on `Physics` merely to express an exact conversion scale.

SI units are part of Oct's type system, not library records. Use literal/type expressions such as `Float<m>`, `Float<K>`, `Float<Hz>`, and `Float<kg*m/s^2>`. Use `Units` for explicit conversion from American, British, astronomical, atomic, or spectroscopy conventions. `BaseUnit` strips a dimension; it does not convert or format a value.

## Result and failure conventions

Scientific APIs follow these conventions for new work:

- Invalid mathematical or physical domains return `! Error` when the value is only known at runtime.
- Refined `Concept` values and `Require` express reusable single-value preconditions when the language can prove them statically.
- Nonconvergence returns the best meaningful estimate with explicit evidence such as `Converged`, `Iterations`, `Evaluations`, `Residual`, or `Error`; it is not silently reported as success.
- Domain-specific result records remain domain-specific. Equivalent evidence uses familiar field names, but there is no universal solver-result mega-record.
- Sentinel-returning legacy APIs remain compatibility behavior; their READMEs identify preferred fallible or typed siblings where available.

Names should state the operation without mechanically adding `Calculate` or `Compute`. `Solve*` is reserved for algorithms that solve a system/problem; `Estimate*` indicates an approximation or model estimate. Existing names remain compatible unless a wrapper can clarify ownership without a break.

## Canonical composition examples

- Cooling: `Thermofluids` supplies heat-transfer relations; `DifferentialEquations` supplies an ODE method or `Simulation` supplies scenario traces and sweeps.
- RC decay: `Electromagnetism` supplies the RC time constant and exact discharge relation; `DifferentialEquations` or `Simulation` supplies numerical progression.
- Orbit: `Astrophysics` supplies orbital relations and uses `Physics` constants plus `Units.Astronomical`; `Simulation` is optional for a repeated normalized scenario.
- DNA summary: `ComputationalBiology` supplies typed sequence operations; convert the desired measurements to a `Float[]` and use `Statistics` for generic summaries.

[`ComputationalScience`](../../Libraries/ComputationalScience/README.md) contains small executable examples of these composition boundaries. It is intentionally not a miscellaneous home for laws, solvers, or copied algorithms.
