# Oct-XML M0 design

> Oct-XML is typed template markup syntax that desugars to ordinary Oct expressions.

## Motivation and boundary

Nested typed construction is readable as calls but visually noisy for document
and tree-shaped authoring. M0 adds an expression surface for that structure
without adding XML semantics or a second execution model. OctCument is the
first substantial consumer; the `UiNode` language fixture is the independent
non-Document proof.

The authoritative language contract is
[`Language/reference/language/21-oct-xml.md`](../../Language/reference/language/21-oct-xml.md).

## Front-end shape

```text
source
  -> lexer tokens with exact source offsets
  -> MarkupElementExpr / MarkupAttribute / MarkupChild
  -> project elaboration and ordinary symbol/signature resolution
  -> CallExpr + ArrayLiteralExpr + StringLiteralExpr
  -> existing typechecker, interpreter, MIR, and backend
```

Markup AST retains the raw source slice as well as structured children. This is
necessary because the parser runs before package-wide callable resolution. A
final `lines: String[]` parameter selects raw capture during elaboration; any
structured parse failure is otherwise reported. No source is concatenated and
reparsed.

## Lowering convention

- A tag names an ordinary function in the current or qualified package.
- Attributes match real parameters case-insensitively and lower in declaration
  order.
- The final unmatched `T[]` parameter receives structured children.
- Plain text for `T[]` lowers through `MarkupTextT(String) -> T` in the target
  package.
- A final unmatched `String` receives normalized text-only content.
- A final `lines: String[]` opts into normalized raw lines.
- All other missing parameters, extra attributes, and unsupported children are
  compile-time errors.

This keeps the protocol finite and inspectable. The only consumer convention is
an ordinary text adapter; raw capture is an explicit signature convention.

## OctCument

`Document.Content(Block[])`, `Document.MarkupTextInline(String)`, and
`Document.MarkupTextBlock(String)` are thin adapters over the existing
`Document.Doc`, `Block`, and `Inline` model. Existing APIs remain unchanged.
The paper specimen uses markup for its content hierarchy and uses
`Document.Code(language, lines)` as a raw-body target, eliminating escaped
line-array source.

## Diagnostics and formatter

Parser diagnostics identify mismatched closing tags and malformed markup
structure. Elaboration diagnostics identify the ordinary callable parameter:
unknown/duplicate attributes, missing required parameters, unsupported
children, or a missing typed text adapter. The formatter indents open/close
markup structure and remains idempotent; raw content is preserved semantically
after common-indent normalization.

## Deferred

M0 does not add generic tag arguments, template aliases as tag targets, XML
comments, namespaces, entities, schemas, DOM APIs, runtime registries, arbitrary
syntax macros, optional interpolation in raw bodies, or a markup-specific MIR.

