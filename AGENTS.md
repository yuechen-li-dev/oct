# AGENTS.md

## Core Principle

Oct has a strict, non-negotiable separation of concerns:

> Go implements the Oct language.  
> Oct expresses programs, tests, and language contracts.

Do not blur this boundary.

---

## Mental Model

Think of the repository as two layers:

- **Go = compiler/runtime (implementation layer)**
- **Oct = user space (programs and contracts)**

Go defines how Oct works.  
Oct is never used to define how Oct works.

---

## Hard Rules

### 1. Do not embed Oct inside Go

Writing Oct programs as strings inside Go code is almost always a bug.

❌ Forbidden:
```go
src := `
fn main() {
    return 42
}
`
````

✅ Correct:

* Place code in `.octest` or `.octfail`
* Execute via `oct test` or CLI integration

Exception:

* Extremely small, parser-level or lexer-level unit tests only
* Must not represent user-visible semantics

---

### 2. Do not implement Oct in Oct

Oct is **100% implemented in Go and 0% implemented in Oct**.

Do not:

* add meta-programming layers in Oct to simulate language features
* implement evaluation, typing, or execution logic in Oct
* create "helper" Oct layers that act like a runtime

If behavior belongs to the language, it belongs in Go.

---

### 3. Language semantics belong in Oct tests, not Go

All user-visible behavior must be expressed as:

* `.octest` (valid behavior)
* `.octfail` (invalid/rejected behavior)

❌ Forbidden:

* encoding language semantics in Go test assertions using string programs

✅ Correct:

* express behavior in `Language/` suites
* use Go only to orchestrate execution

---

### 4. Go tests orchestrate, not define semantics

Go test code should:

* invoke `oct test`, `oct bench`, `oct artifact`
* validate CLI boundaries and integration behavior

Go test code should NOT:

* duplicate language semantics
* reimplement logic that already exists in Oct tests
* act as a second specification of the language

---

### 5. Keep the repository roles clean

Each top-level area has a specific purpose:

* `Language/`
  Canonical language contracts (semantic truth)

* `Packages/`
  Reusable Oct packages (user-level code)

* `testdata/`
  Fixtures, synthetic inputs, transitional data

* Go code (`cmd/`, `internal/`)
  Language implementation only

Do not mix these roles.

---

## Allowed vs Forbidden Patterns

### Adding a new language behavior

✅ Correct:

* implement behavior in Go
* add `.octest` and/or `.octfail` under `Language/...`

❌ Forbidden:

* write Go tests with embedded Oct strings to define behavior

---

### Testing a feature

✅ Correct:

* place tests in `Language/<Domain>/<Concept>/valid` or `invalid`

❌ Forbidden:

* re-express the same behavior in Go test files

---

### Reusing logic

✅ Correct:

* move reusable logic into `Packages/`

❌ Forbidden:

* simulate reuse via copied Oct strings inside Go
* build meta-systems in Oct to compensate for missing language features

---

## If You Are Unsure

Default to preserving the boundary:

* If it feels like “this could be written in Oct instead of Go” → it probably should NOT be
* If it feels like “this test could live in Language/ instead of Go” → it probably SHOULD

When in doubt:

> keep implementation in Go, and semantics in Oct

---

## Summary

* Go defines the language
* Oct expresses the language
* Tests live in Oct
* Go orchestrates, not specifies

This separation is fundamental to the architecture.
Do not violate it.
