# Tensor2D

`Tensor2D` provides explicit numerical finite-difference tensor calculus helpers for pointwise 2D evaluation.

CM2 uses the `Tensor2D.*` API rather than `Tensor.D2.*` because the current Oct package surface supports two-segment qualified calls (`Package.Function(...)`) and does not support nested package namespaces such as `Tensor.D2.Gradient(...)`.

## API

- `Gradient(f: fn(Vector<Float>) -> Float, point: Vector<Float>, h: Float) -> Vector<Float> ! Error`
- `Jacobian(u: fn(Vector<Float>) -> Vector<Float>, point: Vector<Float>, h: Float) -> Matrix<Float> ! Error`
- `Divergence(u: fn(Vector<Float>) -> Vector<Float>, point: Vector<Float>, h: Float) -> Float ! Error`
- `SymmetricGradient(u: fn(Vector<Float>) -> Vector<Float>, point: Vector<Float>, h: Float) -> Matrix<Float> ! Error`

All operators use central differences. `point` must be a `Vector<Float>` of length 2, vector field outputs must be `Vector<Float>` values of length 2, and `h` must be positive.

`Vector<Float>` and `Matrix<Float>` are mathematical values. `Float[]` and `Float[][]` are collections and are not accepted as substitutes.

These functions are numerical pointwise operators only. They do not implement symbolic differentiation, grid/PDE boundary conditions, 3D operators, rank-polymorphic dispatch, overloads, or named arguments.
