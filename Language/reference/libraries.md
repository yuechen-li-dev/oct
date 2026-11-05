# Library directory

[Reference home](README.md) · [Library execution and common APIs](language/standard-libraries.md)

Start from the [science task guide](../../docs/science/README.md) for numerical
work. This directory includes every current `Libraries/` package, including
usage specimens and experimental tools. A listed package is not automatically
a stable API; consult [support](support.md) and its own contracts.

The linked README is the package's practical guide when present; otherwise
the source directory contains its typed declarations and tests. `Library`
APIs use ordinary Oct functions. Compiler-known namespaces such as `Json`,
`Random`, `Entropy`, and `String.From<T>` also have exact contracts in the
[standard-library chapter](language/standard-libraries.md).

## Mathematics and numerical methods

- [Analysis](../../Libraries/Analysis/README.md)
- [Complex](../../Libraries/Complex/README.md)
- [DifferentialEquations](../../Libraries/DifferentialEquations/README.md)
- [Distributions](../../Libraries/Distributions/README.md)
- [Interpolation](../../Libraries/Interpolation/README.md)
- [LinearAlgebra](../../Libraries/LinearAlgebra/README.md)
- [Mathematics](../../Libraries/Mathematics/README.md)
- [Numerics](../../Libraries/Numerics/README.md)
- [Optimization](../../Libraries/Optimization/README.md)
- [Statistics](../../Libraries/Statistics/README.md)
- [Tensor2D](../../Libraries/Tensor2D/README.md)
- [Uncertainty](../../Libraries/Uncertainty/README.md)

## Scientific domains

- [Astrophysics](../../Libraries/Astrophysics/README.md)
- [Chemistry](../../Libraries/Chemistry/README.md)
- [ChemistryNmr](../../Libraries/ChemistryNmr/README.md)
- [ComputationalBiology](../../Libraries/ComputationalBiology/README.md)
- [ComputationalScience](../../Libraries/ComputationalScience/README.md)
- [Electromagnetism](../../Libraries/Electromagnetism/README.md)
- [Geometry](../../Libraries/Geometry/README.md)
- [Mechanics](../../Libraries/Mechanics/README.md)
- [Optics](../../Libraries/Optics/README.md)
- [Physics](../../Libraries/Physics/README.md)
- [Quantum](../../Libraries/Quantum/README.md)
- [RF](../../Libraries/RF/README.md)
- [Signal](../../Libraries/Signal/README.md)
- [Simulation](../../Libraries/Simulation/README.md)
- [SymbolicRegression](../../Libraries/SymbolicRegression/)
- [Thermofluids](../../Libraries/Thermofluids/README.md)
- [Units](../../Libraries/Units/README.md)
- [Wireless](../../Libraries/Wireless/README.md)

## Data and general programming

- [Algorithms](../../Libraries/Algorithms/README.md)
- [DatabaseTemplateContracts](../../Libraries/DatabaseTemplateContracts/README.md)
- [DatabaseTemplates](../../Libraries/DatabaseTemplates/README.md)
- [Entropy](../../Libraries/Entropy/README.md)
- [IfErrNotEqualNil](../../Libraries/IfErrNotEqualNil/)
- [Loop](../../Libraries/Loop/README.md)
- [Octomata](../../Libraries/Octomata/README.md)
- [Random](../../Libraries/Random/README.md)
- [String](../../Libraries/String/README.md)
- [Structures](../../Libraries/Structures/)
- [Time](../../Libraries/Time/README.md)

## Files, interchange, and native-backed output

- [Archive](../../Libraries/Archive/README.md)
- [Compression](../../Libraries/Compression/README.md)
- [Csv](../../Libraries/Csv/)
- [Hash](../../Libraries/Hash/README.md)
- [Image](../../Libraries/Image/README.md)
- [IO](../../Libraries/IO/README.md)
- [Json](../../Libraries/Json/README.md)
- [Pdf](../../Libraries/Pdf/README.md)
- [Plot](../../Libraries/Plot/README.md)
- [Text](../../Libraries/Text/README.md)

## Documents, artifacts, and semantic graphs

- [Artifact](../../Libraries/Artifact/README.md)
- [Atlas](../../Libraries/Atlas/README.md)
- [Document](../../Libraries/Document/README.md)
- [Markdown](../../Libraries/Markdown/README.md)

## Experimental build and UI integration

- [Make](../../Libraries/Make/README.md)
- [MakeHostPrivileged](../../Libraries/MakeHostPrivileged/)
- [MakeOctestPlan](../../Libraries/MakeOctestPlan/)
- [UI](../../Libraries/UI/README.md)

## Usage specimens and reference packages

- [ArtifactUsage](../../Libraries/ArtifactUsage/)
- [Cooking](../../Libraries/Cooking/README.md)
- [Deployment](../../Libraries/Deployment/README.md)
- [HelloScience](../../Libraries/HelloScience/README.md)
- [RandomUsage](../../Libraries/RandomUsage/README.md)

## Pick one owner

Use `Mathematics` for mathematical helpers, `Numerics` for differentiation,
quadrature, and scalar root/minimum methods, `Optimization` for richer
multivariate optimization, and `DifferentialEquations` for ODE evolution.
`Tensor2D` evaluates numerical derivatives at a point; language `Grad`/`Div`/
`SymGrad` are representational operators. The science guide records preferred
owners and compatibility wrappers.

Use `Random` with an explicit seed and draw index for reproducibility. `Entropy`
is ambient randomness with a different contract. Use `Document` for document
semantics and `Markdown` for Markdown output helpers. Artifact entry points
own explicit publication; they do not replace normal runtime APIs.
