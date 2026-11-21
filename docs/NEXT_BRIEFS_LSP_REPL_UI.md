# Next milestones after LSP-M0 / UI-M0 — Codex briefs

Each brief follows AGENTS.md: Language-reference authority, Oct semantics in
`Language/`, Go tests orchestrate, internal/judgment for multi-signal choices,
and the convergence rule. Each states the value it must deliver **before**
work starts; if a milestone cannot plausibly reach it, stop and report.

Baseline (verified at 7311f35 + LSP-M0/UI-M0):
`go test ./internal/lsp ./internal/machina/... ./internal/cli` passes;
`oct lsp` is clean on 307/307 `Language/**/valid` files and 138/138
`Libraries` files; `oct ui check Examples/MachinaThermostat/main.oct --click Raise`
round-trips through Oct `Update`.

---

## REPL-M0 — `oct repl`

**Goal.** An interactive session where a scientist types Oct and sees
unit-aware results.

**Value it must deliver.** This exact transcript works:

```text
oct> let g = 9.81 m/s^2
oct> g * 2.0 s
19.62 m/s
oct> g + 2.0 s
error: cannot add m/s^2 and s
oct> fn Double(x: Float) -> Float { return x * 2.0 }
oct> Double(4.0)
8.0
```

**Design constraints.**
- Session = a synthetic package. Declarations (`fn`, `record`, `enum`,
  `concept`) accumulate in it; each expression is wrapped and evaluated with
  the interpreter (`interpret.CallFunctionWithArgsAndOptions`).
- **Do not re-execute earlier lines** to rebuild bindings: that repeats
  randomness and file IO. Serialize each committed binding as an `.octagon`
  literal into the session prelude. First verify that octagon round-trips
  every value kind a binding can hold (dimensioned floats, records, arrays,
  options). Report any kind it cannot carry.
- A failed line must leave the session unchanged.
- Results print with units (use the existing Value formatting; do not invent
  a display format).

**First step (spike, ≤ 1 day).** Prove octagon round-trip for the value kinds
above. If it cannot carry them: **Honest stop**, with the failing kinds as
evidence.

**Out of scope.** Multi-line editing beyond brace balancing, history files,
Jupyter.

**Tests.** REPL transcripts as Go orchestration tests that drive stdin/stdout;
no new language semantics.

---

## UI-M1 — make Machina UI a real authoring target

UI-M0 found five structural gaps (`docs/MACHINA_UI_RUN_M0.md`). M1 takes
the three that block real apps, in this order. **Audit before porting.**

1. **Port audit (doc only, first).** Compare `Libraries/UI` against
   MachinaLayout.JS (public) and Machina.UI (in `Copeland/Machina`): list the
   authoring capabilities Oct lacks, ranked by what a small scientific tool
   needs (likely text input, scrolling, labelled numeric input with units).
   Do not port anything not on that list.
2. **Real text metrics.** Layout sizes text at a fixed 8 dp/char, and labels
   overflow. Add a measurement seam: layout asks a `TextMeasurer`; the
   headless default keeps today's metrics so snapshots stay deterministic,
   and Gio supplies real measurement. Value: `oct ui run` shows no clipped or
   overlapping text on the thermostat and storefront fixtures.
3. **Padding and text styles.** M117 wired `UI.Styled` end to end
   (background, border, radius, foreground). Still unapplied: `Style.Padding`
   and `UI.TextStyle`. Both change geometry, so do them with item 2's
   measurement seam. Value: a padded, styled badge in the thermostat has its
   text inside the badge, verified by snapshot.
4. **Text input** (if the audit ranks it first): a `UI.TextField` with
   keyboard focus in the Gio backend.

**Decided, not open.** The WebView/desktop-host path loads a hand-assembled
M98 counter module and is now documented as a harness. Making it show user
programs needs an Oct → wasm backend or a Go-session → WebView bridge; both
were declined in favour of Gio. Revisit only with a concrete reason (e.g. a
browser target).

**Compiled-mode UI builtins** (`compiled mode does not yet support builtin
UIButton`) are their own milestone. Do not force interpreted mode in tests to
hide it. When it lands, add the runtime contract that M117 could not: a
`UI.Styled` call with a color channel of 300 must fail with
`UIStyleNode color channel 0 must be an Int in 0..255, got 300`. Runtime
`.octfail` contracts must fail in both lanes, so it cannot exist before
compiled UI support.

---

## LSP-M1 — positions from the type checker, then hover and definition

**The real blocker.** Type-check errors carry no source position
(`function Main: let z: cannot add m and s`). LSP-M0 compensates with a
judgment-based locator (`internal/lsp/locate.go`, trace in `data.locator`).
That is a workaround, not a fix.

1. **Spans on type-check errors.** Make `typecheck` return
   `diagnostic.Diagnostic` with `Span` for its most common errors first
   (binding mismatch, undefined name, operator dimension mismatch, call
   argument mismatch). Expression nodes need spans; several AST nodes have
   only `Line` today. Value: for those classes, the LSP uses the compiler's
   span directly, and the locator becomes a fallback (keep its trace, so
   remaining heuristic placements stay visible).
2. **Hover**: the type of the identifier under the cursor, including its
   dimension (`g: Float<m/s^2>`). This is the feature that sells units in an
   editor.
3. **Go to definition** for functions, records and package members.

**Value check before step 2.** Measure what share of errors in the
`Language/**/invalid` and `.octfail` corpus get a compiler span after step 1.
If it is under ~70%, finish spans before hover.

**Note.** `typecheck.CheckProgram` stops at the first error. Collecting
several per pass is a separate, larger change. Report it, don't bundle it.

---

## Fixed since these briefs were written

- `0 A` inside a one-line record literal no longer parses as amperes: an
  identifier followed by `:` is a field label, never a unit
  (`Language/Types/UnitLabels/valid/unit_names_as_field_labels.octest`).
- Manifest validation names the rule that failed, e.g.
  `Name is "X" but the package is named "Y"; they must match`.
- `internal/sdslv/test` skips when `dxc` is not installed (`ErrDXCNotFound`);
  a present but failing `dxc` still fails.
