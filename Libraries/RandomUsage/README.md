# RandomUsage

Contracts for using the Random stream builtins from a package other than
`Random`. It plays the same role for `Random` that `ArtifactUsage` plays for
`Artifact`: the library's own tests run inside `package Random`, so this
package is where "what an importing package sees" is checked.

- `Random.Usage.octest`: the qualified builtins and their types, streams
  stored in records and arrays, and the rule that the bare names (`Unit`,
  `Normal`, `Between`, ...) are not reserved outside package Random.
  It also seeds a stream from `Entropy.Seed()` and replays it, with
  `import Entropy`, which is allowed and not required.
- `Random.Usage.invalid.Imported*.octfail`: contracts that import `Random`
  and misuse it, one rejected at compile time and one stopped at run time.
- `Random.Usage.invalid.*.octfail`: a missing `import Random`, and an
  unqualified builtin name used outside package Random, and a Random 0.1.0
  builtin (`Random.RngSeed`) called without an import, which 0.1.0 allowed.

Run it with `oct test Libraries/RandomUsage` in both execution modes.
