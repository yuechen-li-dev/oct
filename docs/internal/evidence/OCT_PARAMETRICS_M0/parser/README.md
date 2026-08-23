# Parser evidence

The accepted public surface is `template record/fn/flow/query Name<T, ...>`, explicit applications `Name<A, ...>`, and context-typed `.Field` selector expressions. `template` remains a contextual top-level identifier rather than a lexer keyword, preserving existing identifier behavior. Built-in dimensional types retain their existing angle-bracket parsing; named types use angles for type applications.

Verification:

```text
go test ./internal/parse ./internal/ocfmt
PASS
go run ./cmd/oct fmt ./Language/Types/ParametricsM0 --check
PASS
```

The formatter regression covers `Keyed<Job, String>` so type arguments are not reformatted as comparisons.
