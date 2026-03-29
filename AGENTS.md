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
* Execute tests via `go run ./cmd/oct test ...` (preferred in this environment), or `oct test ...` when the oct binary is available.

#### Exception (narrow and explicit)

Embedded Oct in Go is allowed only for **host-side implementation validation** where Oct cannot be the sole validator without circularity.

This includes:

* parser and lexer validation
* typechecker validation
* runtime/compiler boundary checks

This exception must remain:

* small in scope
* focused on implementation correctness
* not user-facing

It does **not** allow:

* duplicating language semantics already expressed in `Language/`
* writing general program tests in Go

---

### 2. Do not implement Oct in Oct

Oct is **100% implemented in Go and 0% implemented in Oct**.

Do not:

* implement evaluation, typing, or execution logic in Oct
* introduce meta-programming layers to simulate language features
* build helper runtimes in Oct

If behavior belongs to the language, it belongs in Go.

---

### 3. Language semantics belong in Oct tests, not Go

All user-visible behavior must be expressed as:

* `.octest` (valid behavior)
* `.octfail` (invalid/rejected behavior)

These live under:

* `Language/`

❌ Forbidden:

* encoding language semantics in Go test assertions
* using embedded Oct in Go to define behavior

---

### 4. Do not duplicate semantics across Go and Language/

If a language contract exists in `Language/`, it must not be re-expressed in Go tests.

Go tests must not:

* mirror `.octest` or `.octfail` behavior
* act as a second specification of the same semantics

There must be a **single source of truth** for language behavior:

> `Language/`

---

### 5. Go tests orchestrate, not define semantics

Go test code should:

* invoke `oct test`, `oct bench`, `oct artifact`
* validate CLI boundaries and integration behavior

Go test code must not:

* define or reimplement language semantics
* encode evaluation logic that belongs to Oct tests

---

### 6. Migrate legacy semantic tests out of Go

If existing Go tests encode language semantics:

* do not extend them
* migrate the behavior into `.octest` or `.octfail` under `Language/`

Go should not remain the owner of semantic contracts.

---

### 7. Keep the repository roles clean

Each top-level area has a strict purpose:

* `Language/`
  Canonical language contracts (semantic truth)

* `Packages/`
  Reusable Oct packages (user-level code)

* `testdata/`
  Fixtures, synthetic inputs, transitional data (not semantic contracts)

* Go code (`cmd/`, `internal/`)
  Language implementation only

---

### 8. Do not use testdata for language semantics

`testdata/` must not contain language contracts.

Do not:

* place `.octest` or `.octfail` there as canonical behavior
* hide semantic tests as fixtures

Use `testdata/` only for:

* synthetic inputs
* invalid parsing cases
* temporary or transitional data

---

## Placement Rule

When adding code or tests:

* If it defines language behavior → `Language/`
* If it is reusable user code → `Packages/`
* If it is synthetic or temporary → `testdata/`
* If it implements the language → Go

---

## Allowed vs Forbidden Patterns

### Adding a new language behavior

✅ Correct:

* implement behavior in Go
* add `.octest` / `.octfail` under `Language/...`

❌ Forbidden:

* define behavior in Go tests using embedded Oct

---

### Testing behavior

✅ Correct:

* use `Language/<Domain>/<Concept>/valid|invalid`

❌ Forbidden:

* duplicate the same behavior in Go tests

---

### Reusing logic

✅ Correct:

* move reusable logic into `Packages/`

❌ Forbidden:

* simulate reuse via embedded Oct strings in Go
* build meta-systems in Oct to compensate for missing language features

---

## If You Are Unsure

Default to preserving the boundary:

* If it feels like “this could be written in Oct instead of Go” → it probably should NOT be
* If it feels like “this test could live in Language/ instead of Go” → it probably SHOULD
* If something feels like a limitation, friction point, or missing feature → record it in `FEEDBACK.md` instead of working around the rules

When in doubt:

> keep implementation in Go, and semantics in Oct

---

## Summary

* Go defines the language
* Oct expresses the language
* Language contracts live in `Language/`
* Go orchestrates, not specifies

This separation is fundamental to the architecture.
Do not violate it.
