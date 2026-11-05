# Source and lexical structure

[Reference home](../README.md) · [Coverage](../coverage.md)

## Overview

Oct source is tokenized from Unicode text.
Identifiers, literals, keywords, and punctuation define the lexical surface.
Whitespace separates tokens. Semicolons are optional separators and are never required.
`//` starts a line comment.

## Rules

- Identifier start is `_` or any Unicode letter.
- Identifier continuation is identifier-start characters or Unicode digits.
- Reserved keywords cannot be used as ordinary identifiers. `flow`, `state`, and `step` are contextual and remain usable as names in the documented unambiguous binding/reference positions. See [FLOW](../runtime/flows-and-state-machines.md).
- Integer literals are base-10 digits.
- Float literals support decimal and scientific notation (`1.0`, `1e3`, `2.5E-2`).
- String literals use double quotes and cannot span lines.
- Bool literals are `true` and `false`.
- Dimension suffixes attach to numeric literals (`5m`, `2.5s`, `90deg`, `22C`).
- `//` comments continue to end of line.
- Arrow tokens accept both `->` and `=>` spellings in arrow positions; parser semantics are identical.
- `...` is one token, the ellipsis. It repeats an element of an array, vector or matrix literal and has no other use; see [Arrays](arrays.md#repeated-elements). `..` is a different token, the range operator.
- Semicolons are treated like whitespace. Canonical style remains one declaration or statement per line; semicolons are useful only for deliberately compact source.

## Examples

Valid:

```oct
package Main

fn Main() -> Bool {
    let _x1 = 1e3
    let name = "ok"
    return true
}
```

Invalid:

```oct
package Main

fn Main() -> String {
    return "unterminated
}
```

## String escapes (M0 contract)

Supported escapes in string literals: `\n`, `\"`, `\\`.
Unsupported escapes are rejected by the lexer.

## Documentation comments and authoring forms

`///` immediately before a declaration attaches documentation. Ordinary `//`
comments remain comments. The [comments contract](../../Functions/CommentsAndDocs/valid/comments_and_doc_comments_parse.octest)
includes `Param x:` and `Returns:` sections. Template discovery and Atlas read
the declarations and their documentation; comments do not execute code.

`template`, `query`, `profile`, `board`, `accepts`, `yields`, `filter`, `map`,
`take`, `utility`, `policy`, and policy fields have meanings in their specific
authoring contexts. They do not introduce a second general keyword grammar.
Both `->` and `=>` are accepted arrows; formatter arrow policy can retain or
normalize spelling. See [formatting](../tooling/formatting.md).
