# Lexical

## Overview

Oct source is tokenized from Unicode text. Identifiers, literals, keywords, and punctuation form the lexical surface. Whitespace separates tokens but is otherwise insignificant. `//` starts a line comment.

## Rules

- Identifier start: `_` or any Unicode letter.
- Identifier continuation: identifier start characters or Unicode digits.
- Keywords are reserved and are not identifiers.
- Integer literals are base-10 digits.
- Float literals support decimal form and scientific notation (`1.0`, `1e3`, `2.5E-2`).
- String literals use double quotes and cannot span lines.
- Bool literals are `true` and `false`.
- Dimension suffixes attach to numeric literals (for example `5m`, `2.5s`, `90deg`).
- `//` comments run to end of line.

## Examples

Valid:

```oct
fn Main() -> Bool {
    let _x1 = 1e3
    let name = "ok"
    return true
}
```

Invalid:

```oct
fn Main() -> String {
    return "unterminated
}
```
