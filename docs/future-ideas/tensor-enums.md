Algebraic data types and tensor algebra are both “algebraic,” but not the same algebra. The cool part is not that enums magically become tensors. The cool part is that **enums can express tensor *structure, regimes, boundary conditions, material models, and coordinate choices***, while tensor notation expresses the math inside each case.

So Oct can combine:

```text
enum/match:
  choose the mathematical regime / variant / model

tensor notation:
  express the equations inside that regime
```

That is very powerful for continuum mechanics, RF, control, PDEs, simulation, and geometric modeling.

## The simple example: material law variants

Imagine:

```oct
enum MaterialModel {
    LinearElastic { Lambda: Float, Mu: Float }
    NeoHookean { Mu: Float, Kappa: Float }
    Rigid
}
```

Then:

```oct
fn Stress(model: MaterialModel, F: Matrix<Float>) -> Matrix<Float> {
    let i = Idx("i")
    let j = Idx("j")
    let k = Idx("k")

    match model {
        LinearElastic params => {
            let C = F[k, i] * F[k, j]
            let E = 0.5 * (C - Identity2())
            return (params.Lambda * Trace(E)) * Identity2() + (2.0 * params.Mu) * E
        }

        NeoHookean params => {
            let B = F[i, k] * F[j, k]
            return params.Mu * (B - Identity2()) + params.Kappa * Log(Det(F)) * Identity2()
        }

        Rigid => {
            return Zero2()
        }
    }
}
```

That’s the good stuff. The enum says “which constitutive law is this?” The tensor notation says “what is the law?”

In Python/NumPy, this becomes a soup of classes/functions/strings/branching and `einsum`. In Oct, it could look like the paper.

## Enums are great for tensorial “kinds”

Tensor math often has variants:

```text
coordinate system:
  Cartesian | Cylindrical | Spherical

boundary condition:
  Dirichlet | Neumann | Robin | Periodic

element type:
  Triangle3 | Quad4 | Tetra4 | Hex8

strain model:
  SmallStrain | GreenLagrange | Almansi

stress measure:
  Cauchy | FirstPiola | SecondPiola

material:
  LinearElastic | NeoHookean | MooneyRivlin | Viscoelastic

solver status:
  Converged | Diverged | MaxIterations | SingularJacobian
```

Those are algebraic data types begging to exist.

Then `match` gives you exhaustive handling:

```oct
fn ApplyBoundary(bc: BoundaryCondition, u: Vector<Float>, normal: Vector<Float>) -> Vector<Float> {
    let i = Idx("i")

    match bc {
        Dirichlet target => {
            return target.Value
        }

        Neumann traction => {
            return traction.Value
        }

        Robin params => {
            return params.Alpha * u + params.Beta * normal
        }

        Periodic => {
            return u
        }
    }
}
```

The point is not just elegance. It is **correctness**. If you add a new boundary condition later, the compiler can force you to handle it.

## The really cool part: tensor expressions inside match arms preserve meaning

A `match` over enums can select mathematical formulas while each arm still uses index notation:

```oct
enum StrainMeasure {
    Small
    GreenLagrange
    LeftCauchyGreen
}

fn Strain(measure: StrainMeasure, F: Matrix<Float>, gradU: Matrix<Float>) -> Matrix<Float> {
    let i = Idx("i")
    let j = Idx("j")
    let k = Idx("k")

    match measure {
        Small => {
            return 0.5 * (gradU[i, j] + gradU[j, i])
        }

        GreenLagrange => {
            let C = F[k, i] * F[k, j]
            return 0.5 * (C - Identity2())
        }

        LeftCauchyGreen => {
            return F[i, k] * F[j, k]
        }
    }
}
```

This is where Oct’s enums and tensor notation reinforce each other:

```text
enum/match prevents regime bugs
tensor notation prevents index bugs
dimensions prevent unit bugs
records prevent parameter soup
```

That’s a nasty little correctness stack. In the good way.

## But enums are not tensor indices

Important boundary: do **not** confuse enum variants with tensor index symbols.

Tensor index symbols:

```oct
let i = Idx("i")
let j = Idx("j")
```

mean axis slots and contractions.

Enums:

```oct
enum CoordinateSystem { Cartesian Cylindrical Spherical }
```

mean symbolic variants.

They can interact, but they are not the same thing.

Bad idea:

```oct
A[Coordinate.X, Coordinate.Y]
```

Unless you deliberately design finite named axes later. That’s possible, but not M32/M33 territory. Tiny dragon with a jeweled collar.

## A future cool idea: typed axes

Later, Oct could have named finite axis enums:

```oct
enum Axis2 {
    X
    Y
}
```

and maybe matrices/vectors with axis metadata:

```oct
Vector<Float, Axis2>
Matrix<Float, Axis2, Axis2>
```

Then you could prevent mixing coordinate axes incorrectly. But that is a bigger typed-tensor design. Powerful, but not now.

For now:

```text
Index values:
  Einstein symbolic labels

Enums:
  model variants and domain choices
```

Keep those separate.

## Another cool pattern: result enums for tensor algorithms

Tensor algorithms often fail in typed ways:

```oct
enum SolveResult {
    Converged { Stress: Matrix<Float>, Iterations: Int }
    Diverged { Residual: Float }
    Singular { Step: Int }
}
```

Then:

```oct
fn UpdateStress(...) -> SolveResult {
    ...
}
```

And:

```oct
match result {
    Converged ok => {
        let sigma = ok.Stress
        ...
    }

    Diverged err => {
        ...
    }

    Singular err => {
        ...
    }
}
```

This is much better than returning `Matrix<Float> ! Error` when the failure modes matter structurally. Fallible errors are good for ordinary “file not found / invalid input” style paths. Enums are better when the algorithm has meaningful scientific states.

## Algebraic data types + tensor notation = model algebra

This is the non-stupid insight:

```text
Tensor algebra:
  algebra of quantities and index contractions

Algebraic data types:
  algebra of cases/products/sums

Together:
  a language for scientific model structure
```

A record is a product type:

```text
LinearElastic = Lambda × Mu
```

An enum is a sum type:

```text
MaterialModel = LinearElastic + NeoHookean + Rigid
```

A tensor expression is algebra over mathematical fields/matrices/vectors.

So a model can be expressed as:

```text
Material model variant
× parameters
× tensor equations
× boundary conditions
× solver result states
```

That’s not gimmicky. That’s exactly how scientific modeling actually works.

## Where this should influence design

For tensor compiled parity, we should make sure:

1. `match` arms can contain indexed tensor expressions.
2. tensor expressions can return enum payload fields when appropriate.
3. diagnostics remain good inside match arms.
4. compiled lowering preserves labels through branch/merge if each arm returns a compatible matrix type.

Example future test:

```oct
enum StrainKind {
    Small
    Green
}

fn Compute(kind: StrainKind, F: Matrix<Float>, gradU: Matrix<Float>) -> Matrix<Float> {
    let i = Idx("i")
    let j = Idx("j")
    let k = Idx("k")

    match kind {
        Small => 0.5 * (gradU[i, j] + gradU[j, i])
        Green => {
            let C = F[k, i] * F[k, j]
            0.5 * (C - Identity2())
        }
    }
}
```

That’s a great friction test later.

## But don’t overgeneralize too fast

Things I would **not** do immediately:

```text
enum variants as tensor axes
dependent tensor shapes from enum variants
automatic coordinate transforms from enum matches
rank-N typed tensor algebra
symbolic algebra simplifier
pattern matching on tensor shapes
```

Those are cool, yes. Also huge. A glorious swamp with stained-glass windows.

For now, the sweet spot is:

```text
Enums choose the case.
Records hold parameters.
Tensor notation expresses the math.
Match ensures all cases are handled.
```

## The Oct killer example

Something like this should eventually be in the docs:

```oct
enum StrainModel {
    Small
    GreenLagrange
}

fn Strain(model: StrainModel, F: Matrix<Float>, gradU: Matrix<Float>) -> Matrix<Float> {
    let i = Idx("i")
    let j = Idx("j")
    let k = Idx("k")

    match model {
        Small => {
            return 0.5 * (gradU[i, j] + gradU[j, i])
        }

        GreenLagrange => {
            let C = F[k, i] * F[k, j]
            return 0.5 * (C - Identity2())
        }
    }
}
```

This is the kind of example where someone looks at Oct and goes:

> Oh. This is not NumPy. This is a different animal.

Exactly. NumPy is storage algebra. Oct is trying to be scientific model algebra.

So yes — fun idea, not stupid. But keep the roles clear:

```text
Tensor notation:
  mathematical contraction/expression structure

Enums/match:
  model/case/regime/result structure

Records:
  parameter/config/product structure

Octomata:
  temporal/control/process structure
```

That stack is very, very strong.
