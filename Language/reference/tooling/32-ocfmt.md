# ocfmt

## Overview

`oct fmt` is Oct's canonical formatter.
It supports explicit modes for human-readable source and compact LLM payload generation.
Formatting is syntax-validated (parse-backed), deterministic, and idempotent per mode.

The formatter decides two things: how far each line is indented, and whether a
space separates two neighbouring tokens. It never moves a token to another
line, never changes a token, and never joins or splits lines. Line wrapping is
not enabled.

## Rules

- Command form is `oct fmt <file-or-directory> [--mode en-llm|en-llm-compact] [--arrows keep|thin|fat] [--check]`.
- `en-llm` is the committed/review style and the default.
- `en-llm-compact` is a dense style for prompt payloads and handoff snippets. It keeps the lines and indentation of `en-llm` and drops every space that is not needed to keep two tokens apart.
- `readable` and `compact` are accepted as legacy names for `en-llm` and `en-llm-compact`. The command's help does not advertise them.
- Running the formatter twice in the same mode produces no further changes. Formatting `en-llm-compact` output as `en-llm` gives the same text as formatting the original as `en-llm`.
- The formatted program has the same tokens on the same lines as the source; the only token whose spelling can change is an arrow, and only under `--arrows thin` or `--arrows fat`. If the formatter cannot guarantee that, it reports an internal error and writes nothing.
- `->` and `=>` are one token, and the formatter has no opinion about which one is written. By default each arrow stays as its author wrote it. `--arrows thin` writes every arrow as `->` and `--arrows fat` writes every arrow as `=>`; `--arrows keep` names the default.
- Formatter preserves comments; source that fails parse is refused.
- Line endings are written as LF. Trailing white space is removed. Blank lines are kept.

### Directories

- Every `.oct`, `.octest` and `.octfail` file under the directory is attempted. A file that is refused does not stop the others; every refusal is reported and the command fails.
- An `.octfail` whose body does not lex or parse is a contract for that error. It cannot be formatted, so it is left unchanged and is not a failure, in `--check` mode as well.

### Indentation

- Indentation is four spaces per level. Tabs are replaced.
- A line sits one level inside the line that holds its innermost open `(`, `[` or `{`.
- A line that starts with `)`, `]` or `}` sits level with the line that opened it.
- A line that continues an expression without a bracket is indented one further level: the line before it ends with a binary operator or `=`, or it starts with a binary operator or with the `.` of a member access. Inside `(` or `[` the contents are already one level in and are not indented again.

### Spacing

- One space surrounds binary operators, `=`, `->`, comparisons, `and` and `or`.
- A sign is attached to its operand: `-x`, `a - -b`, `[1.0, -2.0]`.
- No space precedes `,`, `:`, `)`, `]`, `;`, a postfix `!` or `?`, or a member `.`. One space follows `,`, `:` and `;`.
- No space follows `(`, `[` or a member `.`.
- A call, index, array type or type argument list is attached to what it applies to: `Sqrt(x)`, `rows[0]`, `Float<m>[]`, `Identity<Int>(1)`. A `(` or `[` that begins an operand is spaced like one: `a * (b + c)`, `return [1, 2]`, `Authors: ["A"]`.
- Braces are padded on one line and preceded by a space: `Vec2 { X: 1.0 Y: 2.0 }`, `if ok { 1 } else { 0 }`. Empty braces are written `{}`.
- Ranges are tight: `0..Len(xs)`, `..n`, `100..`.
- Type argument lists and unit expressions are tight: `Float<kg*m^-3>`, `Keyed<Job, String>`, `Tensor<Float<m>>[]`. A comparison keeps its spaces: `state.Altitude < limit`.
- A literal's unit suffix is tight: `9.81m/s^2`, `0.5kg*m^-3`. Whether a unit name touches its number is never changed, because the parser reads it.
- The `!` of a fallible type is spaced: `-> Int ! Error`.
- `a[i][j]` and the matrix rows `[1, 2] [3, 4]` have the same tokens. A `]` followed by `[` is written as the author spaced it.

### Oct-XML

A raw body's value is its lines with their common leading white space removed, and the formatter cannot tell which bodies are raw. It therefore treats every element as if its text mattered.

- A markup element is copied as written. Nothing inside it is respaced.
- The lines of a multi-line element move as one block. The white space that all of its lines share is replaced so that the block sits one level inside the line that opens the element. Each line keeps everything after that shared part, byte for byte, including tabs and trailing white space.
- A line that starts with the element's own closing tag sits level with the line that opens it.
- A line inside an element that holds only white space is written empty.
- If a body starts on the same line as its opening tag and continues on later lines, those lines share no leading white space. The block is then left exactly where it is.

## Examples

```text
oct fmt Language/reference --mode en-llm
oct fmt Experiments/OctErgonomicsLab/M0 --mode en-llm-compact
oct fmt Language/reference --mode en-llm --check
```

Before:

```oct
fn DragForce(
cd:Float,
area:Float<m^2>,
v:Float<m/s>
)->Float<kg*m*s^-2>{
let rho=0.5kg * m ^ -3
return 0.5*cd*area*rho*v*v
}
```

After `oct fmt`:

```oct
fn DragForce(
    cd: Float,
    area: Float<m^2>,
    v: Float<m/s>
) -> Float<kg*m*s^-2> {
    let rho = 0.5kg*m^-3
    return 0.5 * cd * area * rho * v * v
}
```
