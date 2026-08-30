# Parametric templates

## Overview

Oct supports a deliberately bounded typed substitution surface for reusable records, functions, flows, and queries. A `template` declaration is authoring material: each explicit application is monomorphized into an ordinary concrete Oct declaration before the existing typechecker, interpreter, FLOW lowering, or Go backend runs.

This is not compile-time execution. There is no template VM, macro expansion language, AST reflection, token emission, runtime type dictionary, or runtime generic object model.

## Declaration and application syntax

```oct
template record Box<T> {
    Value: T
}

template fn Identity<T>(value: T) -> T {
    return value
}

let boxed = Box<Job> { Value: job }
let same = Identity<Job>(boxed.Value)
```

- A template declares one or more explicit type parameters with `<T>` or `<T, U>`.
- M0 requires the `template` keyword; ordinary declarations do not accept type parameters.
- Applications provide all type arguments explicitly. General type inference is not part of M0.
- Type parameters may occur in record fields, function parameters/results, arrays, exact function-value types, FLOW signatures, and query source/yield types.
- A repeated identical application in one consumer package reuses one concrete specialization.
- A runtime-recursive template function specializes once per semantic key and may call that concrete function recursively.
- Specialization that keeps creating new semantic keys (for example `A<T>` -> `A<Box<T>>`) is rejected by a defensive depth/count budget with the instantiation chain.
- Concrete names shown by low-level diagnostics are deterministic but are not a promised source-level ABI.

## Package declaration authority

The package, not an individual source file, is the template declaration universe. Template collection completes across loaded package sources before any concrete declaration is rewritten, so sibling-file forward references and multi-file template chains behave like co-located declarations. Explicit `.octest` selection still controls which test entry points execute; same-package template-bearing and declaration-only support sources remain available to compilation. This preserves isolated standalone fixture programs that intentionally reuse a package name while preventing test selection from hiding template authority.

## Typed selectors

`Selector<Record, FieldType>` is a typed field-reference value:

```oct
record Job { ID: String Status: String }

let jobID: Selector<Job, String> = .ID
```

- `.Field` requires contextual `Selector<Record, FieldType>` typing.
- The owner must be one exact nominal record.
- The named field must exist and its type must exactly match `FieldType`.
- Selectors for unrelated record owners are not interchangeable.
- A selector is callable as an exact getter: `jobID(job)`.
- When a selector is stored in a record field, bind the field value before calling it: `let keyOf = keyed.KeyOf` followed by `keyOf(job)`. Direct `keyed.KeyOf(job)` is currently parsed as a package-qualified call.
- Elaboration emits an ordinary exact-signature getter such as `fn(Job) -> String`; generated Go performs direct field access.
- Compiler semantic provenance lowers to the existing exact-subject `FieldRef` with the owner identity, field ordinal, and field name. Selectors are never encoded as source strings and do not enumerate fields.

Concise `.Field` is preferred when a record field, function argument, or explicit type annotation supplies unambiguous selector context.

## Parametric functions and predicates

```oct
template fn FirstWhere<T>(values: T[], predicate: fn(T) -> Bool) -> T ! Error {
    for i in 0..Len(values) {
        if predicate(values[i]) { return values[i] }
    }
    return error("FirstWhere found no matching value")
}
```

Substitution preserves exact function typing. `FirstWhere<Job>` requires `fn(Job) -> Bool`; a predicate over another nominal record is rejected rather than erased to a dynamic type.

## Parametric queries and FLOW

```oct
template query Filtered<T>(source: T[], predicate: fn(T) -> Bool, limit: Int) yields T {
    filter predicate
    take limit
}
```

The parser represents query syntax with the established Query-M0 FLOW skeleton. `Filtered<Job>(jobs, IsReady, 10)` substitutes every open type in that skeleton before ordinary typechecking and backend lowering. The resulting FLOW declaration is concrete; FLOW execution, the interpreter, and generated Go have no type parameters to handle.

## Records, values, and `with`

Type parameters customize reusable structure. Ordinary fields remain value configuration:

```oct
template record Bounded<Record> {
    MaxRecords: Int
}

let base = Bounded<Job> { MaxRecords: 1000 }
let larger = base with { MaxRecords: 5000 }
```

`with` remains immutable and same-type. M0 has no const generics and does not derive a new nominal type from an update.

## Packages and identity

Imported templates are instantiated deterministically in the consumer compilation. The semantic specialization key includes the declaration package/name, concrete type arguments, declaration kind, and consumer package. This permits consumer-owned nominal type arguments without creating a reverse package dependency. M0 template bodies intended for cross-package reuse should be self-contained or use operations/types already visible in the consumer; automatic qualification of arbitrary template-package-private dependencies is deferred.

## Concepts and requirements

Existing transparent Concept aliases compose with type arguments and are expanded before specialization. Existing `Require` expressions inside a template are checked on each concrete declaration and erase through the ordinary proof path. M0 does not add trait bounds, typeclasses, implicit implementation search, or a `where` clause.

Refined Concepts can therefore be used as concrete type arguments and retain admission and dimensional identity. They cannot currently constrain an arbitrary type parameter by required operators or structure. A template such as `SquareSameType<T>` is deliberately checked only after specialization: numeric dimensionless instantiations can succeed, dimensioned or nonnumeric instantiations can fail, and dormant unused declarations are tolerated. Behavioral/operator Concepts are a separate future design question.

## Elaboration and diagnostics

Template declarations receive syntax/basic declaration validation when parsed. Template bodies receive ordinary semantic checking only after a concrete application exists. Specialization diagnostics retain the outer-to-inner instantiation chain, concrete type arguments, and the template function source path. Before ordinary downstream phases run, the project elaborator structurally rejects any remaining template declaration, type-parameter list, open parameter reference in a specialization, or unresolved call to a known template.

## Explicit limits

M0 does not provide template enums, higher-kinded types, variance, associated types, specialization ordering, const/value parameters, lifetimes, dependent types, type-level arithmetic, generic transformed-dimension expressions such as `Square<T>`, behavioral/operator bounds, arbitrary compile-time loops/functions, filesystem/network/process access, quote/unquote, eval, AST/token macros, field enumeration, reflection over `T`, runtime dictionaries, or a template registry.

## Catalog convention

`*.template.oct` is an ordinary-source naming convention for discovery and documentation. It has no separate parser or execution semantics. `oct templates list <root>` and `oct templates describe <Name> <root>` derive catalog metadata from real declarations and adjacent `///` documentation; `--json` provides the same deterministic data to tools. Catalog Category names are actual String-refining Concepts with `Require`. Requirements are classified by their executable enforcement boundary: `Require`, exact `Type`, structural composition, or an explicit application lifecycle obligation. Template choice remains explicit author intent.

The database catalog demonstrates existing Concept constraints on template configuration. Consumers explicitly import `DatabaseTemplateContracts`; its package-qualified refined value types survive concrete specialization and erase to ordinary representations after admission. This uses the existing Concept/Require path rather than a template-specific bounds system.
