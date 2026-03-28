# M57 — Single-Slot Resume for Octomata (Implementation Report)

## What was added

- Added explicit Octomata state-only statements:
  - `remember`
  - `resume`
- Added runtime single-slot resume state per `FlowInstance`.
- Added minimal observability helper:
  - `ResumeTarget(flow) -> String`

## Slot overwrite behavior

- `remember` stores the **current active state name** into the flow instance resume slot.
- If a value already exists in the slot, `remember` **overwrites** it.
- This keeps resume bounded to one remembered target (no stack depth).

## Clear-on-resume behavior

- `resume` checks whether a remembered target exists.
- On success, runtime transitions to that remembered target.
- The slot is then **cleared immediately** (`ResumeTarget` becomes `""`).

## Empty-slot failure behavior

- If `resume` executes while the slot is empty, runtime fails deterministically with:
  - `runtime error: resume called with empty resume slot`

## What was intentionally excluded

- no stack semantics (`push` / `pop` / `replace`)
- no arbitrary-depth nesting
- no `waitUntil` or waiting primitive additions
- no coroutine/async scheduling behavior
- no hidden ambient control hierarchy

M57 remains a narrow, explicit one-slot interruption/resume mechanism.
