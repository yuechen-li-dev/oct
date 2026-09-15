# Oct-XML typed template markup

## Overview

Oct-XML is typed template markup syntax that desugars to ordinary Oct
expressions. Despite the nickname, it does not implement XML semantics. There
is no DOM, runtime tag lookup, attribute map, component registry, namespace,
schema, XML entity layer, markup MIR, or backend support.

Markup is accepted only where an expression may begin:

```oct
<Panel Enabled={true} Count={4}>
    Hello <Label Text="world" />
</Panel>
```

The tag is resolved through ordinary package and callable lookup. Qualified
names use the normal `Package.Symbol` form. M0 tags resolve ordinary functions;
explicit generic tag application is deferred. Specialize through ordinary Oct
outside markup when a generic function is required.

## Attributes and calls

Attributes are named arguments to the resolved function. Attribute matching is
case-insensitive so author-facing `Title` maps to an ordinary `title`
parameter. Names are still statically checked: unknown, duplicate, and missing
parameters are errors. `Name="value"` is shorthand for `Name={"value"}`; all
other values use `{ expression }` and retain normal Oct typing.

Arguments are emitted in declared parameter order. Parameters supplied by
attributes precede one optional body parameter, which must be the final
parameter. A self-closing element supplies no body.

## Typed bodies

If the final unmatched parameter is `T[]`, structured children form an ordinary
array of `T`. Element children and `{ expression }` children are checked as
ordinary expressions of type `T`.

Plain text for `T[]` calls the ordinary package function
`MarkupTextT(value: String) -> T`. For example, a package with `Inline` children
declares `MarkupTextInline`. This is the complete M0 text adapter protocol: it
is an ordinary statically resolved function, not implicit conversion or
runtime dispatch.

If the final unmatched parameter is `String`, the body must contain text only
and becomes that string. If it is named `lines: String[]`, the target opts into
raw-body capture. Raw bodies do not parse nested tags or `{}` interpolation;
line endings become LF, leading/trailing blank lines are removed, and common
source indentation is stripped while internal indentation is preserved.

## Whitespace and escaping

Structured text normalizes each run of whitespace to one space. Indentation-
only content between elements is ignored. A trailing whitespace run before an
adjacent expression or element contributes one trailing space. Thus two prose
lines become one space-separated text node and mixed prose retains word
boundaries deterministically.

`&` and `>` have no XML entity meaning in text. `<` begins a nested tag only
when immediately followed by a qualified-name start, and `{` begins an embedded
Oct expression. Use an embedded string expression such as `{"<"}` or `{"{"}`
when those reserved forms are required literally in structured text. Raw bodies
preserve them directly.

Opening and closing names must match exactly. XML comments are not part of M0;
ordinary Oct comments remain available outside markup.

## Ambiguity and lowering

Markup starts only in expression-prefix position with `<QualifiedName`.
Comparison expressions such as `a < b` and ordinary template applications such
as `Identity<Int>(1)` retain their existing parse. The parser produces explicit
markup element, attribute, text, and expression-child AST nodes. Project
elaboration resolves the callable and adapters, then replaces the markup with
ordinary calls, arrays, and literals before type checking. Interpreter, MIR,
Go, and profile backends never receive markup nodes.

