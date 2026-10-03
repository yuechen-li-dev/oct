# ocfmt: layout from the lexer's tokens

Date: 2026-10-03
Base commit: `c325f94`
Trigger: `Experiments/OrbitalDecay` was committed in the formatter's output and its indentation drifts to the right for the whole file.

## Verdict

**SUCCESS.** `oct fmt` now indents and spaces from the lexer's tokens and the
parser's markup extents. `Experiments/OrbitalDecay` is reformatted and its
tests are unchanged in both lanes.

## What was wrong

The old formatter worked line by line on text. Four faults, in order of damage:

1. **Type arguments were read as Oct-XML tags.** A regular expression for
   `<Name ...>` matched `Float<m>`, `String.From<Int>` and `Float<kg * m ^ - 3>`.
   Each match added one indent level that nothing removed, and switched the
   formatter into "inside markup" for the rest of the file, where it stopped
   normalizing spacing. That is the OrbitalDecay drift. It affected every file
   that uses a unit type or a template.
2. **Indentation came from the first and last character of a line.** Only a
   trailing `{` indented and only a leading `}` dedented, so the contents of a
   multi-line `(...)` or `[...]` were flattened to the surrounding level.
3. **Spacing came from a rough private tokenizer.** It wrote `- 0.0004` for a
   sign (54,704 lines in the repository), `1e - 12` for a float literal, which
   does not lex, `9.81m / s ^ 2` for a unit, `x =[1, 2]`, `return(a)`,
   `E /(2.0 *(1.0 + nu))`, `Parse(raw) !`, `0 .. n`, and `state.Altitude<limit`
   for a comparison whose left side is capitalized.
4. **Raw Oct-XML bodies were re-indented line by line.** A raw body's value is
   its lines with their common indentation removed, so this changed program
   values. `testdata/ocfmt_raw_markup` fails all four of its facts after the
   old formatter has run on it.

Nothing checked the output. The formatter validated its input by parsing it,
then wrote whatever the text rules produced.

## What it does now

`internal/ocfmt/layout.go`. The rules are in
`Language/reference/tooling/32-ocfmt.md`.

- **Indentation** follows bracket nesting over the real tokens: one level
  inside the line that holds the innermost open bracket; a line that starts
  with a closer sits level with the opener's line. A bracketless continuation
  (`... and` / `+ ...` / `.Method()`) is one level in, at block level only.
- **Spacing** is decided between neighbouring tokens from their kinds and from
  three things worked out once per file: which `<`/`>` are type argument
  lists, which tokens are a literal's unit suffix (using the parser's own rule
  for where a suffix starts and ends), and which `-`, `+`, `!`, `?` are prefix
  or postfix.
- **Oct-XML** extents come from the parser (`ast.File.MarkupSpans`, new). An
  element is copied verbatim. A multi-line element moves as one block by
  replacing only the white space all its lines share, which is exactly the
  part a raw body discards.
- **The output is checked.** The formatter lexes what it produced and refuses
  to return it unless the tokens, their lines, and every number-to-unit
  junction are the ones it was given. Two tokens are never written together
  if they would lex differently joined (`!` before `==`, `>` before `=`).
- **Directories.** One refused file no longer stops the walk. An `.octfail`
  that is a contract for a lexer or parser error is left unchanged instead of
  failing the run; before this, `oct fmt Language` stopped at the first one.

Automatic line wrapping stays off. The dormant call-layout judgment for it is
kept, unchanged, in `autowrap.go`; it still uses the old private tokenizer and
must be moved to real tokens before it is enabled.

## Decisions a reviewer may want to reverse

1. **Type arguments against comparison is decided by shape, not by the
   parser.** `A < B` and `Float<m>` are the same tokens. The rule: the `<`
   follows a name; a matching `>` is on the same line; only type-like tokens
   lie between; and the token after is not a literal or a sign. A lower-case
   name must also be applied with `(` and name a capitalized type. Every
   condition is necessary, none is weighed against another, so this is written
   as direct rules and not with `internal/judgment`. A wrong answer costs
   spacing only. The parser could report type-argument extents the way it now
   reports markup extents; that would remove the guess.
2. **`]` followed by `[` is written as the author spaced it.** `rows[0][1]`
   and the matrix rows `[1, 2] [3, 4]` are the same tokens.
3. **Brace padding.** `Vec2 { X: 1.0 }`, not `Vec2 {X: 1.0}`. The repository
   writes it padded about 9 times in 10.
4. **Markup is not tidied.** Nested elements keep the indentation they were
   written with. Tidying them would need to know which bodies are raw, which
   depends on the callee's last parameter being named `lines`, which is not
   known at parse time.
5. **A line of only white space inside an element is written empty.** In a
   raw body such a line can carry stray spaces into the value. This is the one
   place the formatter can change a value: a blank line stays blank but may
   lose spaces.

## Evidence

Linux/amd64, Go 1.25.0.

Every `.oct`, `.octest` and `.octfail` in the repository (1,703 files) was
copied, formatted, and compared with the original:

| | Old formatter | New formatter |
|---|---|---|
| Already in the formatter's style | 401 | 1,266 |
| Would be changed | 1,251 | 427 |
| Refused | 51 | 10 |
| Lines in the diffs | 74,132 | 11,012 |

The 10 refused files do not parse and are neither `.octfail` nor new; they are
listed in `FEEDBACK.md`. The other 41 the old formatter refused are `.octfail`
contracts for parser errors, now left alone.

The 427 are files written in a style the repository uses in a minority of
places: one-line compact experiments (`let n=FloorToInt(a*b)`), unpadded
braces, and files previously written by the old formatter (`Float < K >`,
`0 .. n`, flattened argument lists).

| Check | Result |
|---|---|
| `go test ./internal/ocfmt` | pass: 6 golden fixtures in both modes, the earlier tests unchanged |
| `go test -tags=integration ./internal/ocfmt` | 1,567 repository sources formatted in memory in both modes: no failure, stable on a second pass, compact output re-formats to the readable output, tokens unchanged |
| `cmd/oct` `TestFmtKeepsRawMarkupValues` | the raw-body fixture's four facts pass before and after formatting |
| `Experiments/OrbitalDecay` M0 and M1, interpreted and compiled | 5 and 3 passed before; 5 and 3 passed after reformatting |
| Fault injection | 28 faults in the layout engine. The first pass missed three (brackets inside a multi-line tag, tab width in a raw body, the output check switched off); tests were added and all 28 are caught |

## Found on the way, not fixed

Recorded in `FEEDBACK.md`:

- The repository is not formatted, and nothing checks that it is.
- The reference and the CLI disagreed about the mode names. The CLI and its
  tests treat `en-llm` and `en-llm-compact` as canonical and `readable` and
  `compact` as legacy; `32-ocfmt.md` and `35-cli.md` said the reverse. Both
  pages now state what the CLI does.
- The formatter rewrites `=>` as `->` everywhere, which `32-ocfmt.md`
  documents, while the reference's own examples write match arms with `=>`.
- `let w: Float<m>=x` does not parse: the lexer reads `>=`.

## Not done

- The repository was not reformatted, apart from `Experiments/OrbitalDecay`.
- Windows was not run. The formatter writes LF line endings, as before.
