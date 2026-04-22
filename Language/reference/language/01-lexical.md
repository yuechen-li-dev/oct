# Lexical

## Overview

Oct source is tokenized from Unicode text.
Identifiers, literals, keywords, and punctuation define the lexical surface.
Whitespace separates tokens.
`//` starts a line comment.

## Rules

- Identifier start is `_` or any Unicode letter.
- Identifier continuation is identifier-start characters or Unicode digits.
- Keywords are reserved and cannot be used as identifiers.
- Integer literals are base-10 digits.
- Float literals support decimal and scientific notation (`1.0`, `1e3`, `2.5E-2`).
- String literals use double quotes and cannot span lines.
- Bool literals are `true` and `false`.
- Dimension suffixes attach to numeric literals (`5m`, `2.5s`, `90deg`, `22C`).
- `//` comments continue to end of line.
- Arrow tokens accept both `->` and `=>` spellings in arrow positions; parser semantics are identical.

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
