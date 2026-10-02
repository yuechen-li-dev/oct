# RandomUsage

Contracts for using the Random v2 stream builtins from a package other than
`Random`. It plays the same role for `Random` that `ArtifactUsage` plays for
`Artifact`: the library's own tests run inside `package Random`, so this
package is where "what an importing package sees" is checked.

- `Random.Usage.octest`: the qualified builtins and their types, streams
  stored in records and arrays, and the rule that the bare names (`Unit`,
  `Normal`, `Between`, ...) are not reserved outside package Random.
- `Random.Usage.invalid.*.octfail`: a missing `import Random`, and an
  unqualified builtin name used outside package Random.

Run it with `oct test Libraries/RandomUsage` in both execution modes.
