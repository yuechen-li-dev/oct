# CM1 vector-array friction audit

CM1 audited collections of mathematical vector and matrix values for scientific workflows.

## Result

`Vector<Float>[]` and `Matrix<Float>[]` are supported as arrays whose elements are mathematical values, not as implicit aliases for `Float[]` or `Float[][]`.

Verified surface:

- parsing type annotations such as `Vector<Float>[]` and `Matrix<Float>[]`;
- typechecking homogeneous array literals of vector or matrix values;
- interpreted and compiled indexing (`dirs[0]`, `transforms[0]`);
- array index assignment with vector/matrix values;
- array copy value semantics for the outer array storage;
- `Append(dirs, vector[...])` for vector arrays.

## Distinctions preserved

- `Float[]` remains a scalar array and is not a `Vector<Float>`.
- `Float[][]` remains an array of arrays and is not a `Matrix<Float>`.
- `Vector<Float>[]` is a collection of vector values.
- `Matrix<Float>[]` is a collection of matrix values.
- Arrays remain the right tool for bulk storage of many samples; vector/matrix values remain the right tool for mathematical tensor operations.

## Fix made

The parser and type-string paths now preserve array suffixes on `Vector<T>` and `Matrix<T>` type references. Before CM1, `Vector<Float>[]` failed during parsing at the array suffix.

## Deferred work

No implicit conversions were added. Vector element mutation remains unsupported by design; mutate by replacing the vector value in the array or by producing a new vector value.
