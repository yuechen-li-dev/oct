# Veril-Oct M2: Octomata FLOW as sequential hardware

## Architecture and verdict

Veril-Oct M2 lowers deterministic Octomata FLOW through the existing path:

```text
Oct FLOW -> frontend/typechecker -> MIRModule (MIRFlow + ordinary expression MIR)
         -> CheckSystemVerilogLegal -> direct SystemVerilog emission
```

No generic register/wire source DSL and no RTL LIR were added. `MIRFlow`
already owns named states, board fields, continuation statements, and
controller-bound utility sites. Ordinary values still use ordinary MIR. M2
adds a finite hardware legality analysis and backend view, not a second FLOW
runtime.

## Clock, reset, and turns

Every FLOW module exposes `Clock` and active-high synchronous `Reset`. State
commits in `always_ff @(posedge Clock)`; complete next-state logic lives in
`always_comb`.

Reset loads the source entry state and instruction zero, captures construction
parameter ports, recursively zero/default-initializes board values (including
the first enum tag), and clears resume, utility, yield, result, done, suspended,
and fault state. The first non-reset edge executes the first FLOW turn. A turn
input is named `Turn_<source-name>` and feeds that edge's activation.

One rising edge corresponds to one host `Step`. Within its combinational
activation, ordinary `goto` transitions continue to execute until `suspend`,
`yield`, or final return, matching Octomata's existing turn rule. The legality
pass rejects an activation control cycle that could run indefinitely without a
turn boundary. This is a termination proof, not multi-cycle HLS scheduling.

## FSM, board, and control transfers

- Source states use compact binary `StateT` values in declaration order.
- `State` plus `Instruction` preserve the exact continuation. The instruction
  register is unavoidable because suspend/yield resume after that statement.
- Each admitted board field becomes persistent `Board_<Field>` and
  `NextBoard_<Field>` signals with existing M1 packed layout. Later statements
  read the next value, so source assignment order is preserved.
- `goto T` selects `State_T`, sets instruction zero, and redispatches during the
  current turn. Later statements on the transferred path do not run.
- `suspend` advances continuation, pulses `Suspended`, and ends the turn.
- `yield x` advances continuation, stores `YieldValue`, pulses `YieldValid`, and
  ends the turn without completing.
- `return x` stores stable `Result`, asserts sticky `Done`, and holds until
  reset. Void FLOWs omit the result data port.
- Falling off a state or exhausting an impossible dispatch bound asserts sticky
  `Fault`; completed machines never restart silently.

## Remember and resume

`HasResumeTarget` and typed `ResumeState` form the one-slot continuation store.
`remember` captures the current source state and continues. Later remember
overwrites it. Successful `resume` clears the slot, restores the state at
instruction zero, and redispatches in the same turn. Empty resume is a host
runtime error; hardware exposes it as sticky `Fault`. The slot persists across
suspend and yield.

## Guard and utility arbitration

Guard `when` evaluates cases in source order and selects the first true case;
the required else action runs only if none match. Nested transfer flags prevent
statements after a terminal action from executing.

Utility cases are a fixed source-known linear argmax. Conditions gate
candidates; strict `>` replacement means equal scores retain the earliest
source case. With no eligible candidate, else is selected with score zero.
Controller-bound `when policy` persists exactly the state required by current
Go semantics: has-current, packed current value, current score, and commitment
age. If the current value remains eligible, it is retained while age is less
than `min_commit` or new-best score is at most current score plus `hysteresis`.
Staying increments age; switching starts at age one. If current is no longer
eligible, fallback or another candidate replaces it immediately. Standalone
`when utility` uses the same argmax without persistent state.

## Explicit capture surface

Named outputs expose `StateView`, `InstructionView`, every board field, resume
slot, `Done`/`Result`, `Suspended`, `YieldValid`/`YieldValue`, `Fault`, and each
persistent utility site's current value, score, and age. This is a mechanically
derived inspect/capture surface. M2 does not implement checkpoint JSON, state
loading, UART, PCIe, or another restore protocol; restore/load is M5 work.

## Legality boundary

M2 uses existing M1 hardware values: `Bool`, `Int`/`Int<D>`, immutable nested
records, payload enums, and transparent compile-time Concepts. Pure static calls
remain combinational. Literal-bounded straight-line FLOW `for` loops are
admitted up to 1024 iterations. No arithmetic latency, pipelining, or resource
sharing is inferred.

Rejected constructs include dynamic arrays, Vector/Matrix storage, runtime
collections, String, Float, indexed dynamic board writes, fallibility,
closures/function values, native/Octxiliary or discarded effects, runtime
`while`, recursive within-turn control, unsupported builtins, and
`MIRBackendValue`. Profiled compilation never falls back to Go. M2 FLOW value
expressions are currently limited to one acyclic ordinary MIR block;
statement-level `if`/`when` remain the behavioral branching surface.

## Evidence

`Language/Profiles/VerilogM2` contains compiler-produced goldens for a basic
FSM, remember/resume, and persistent utility arbitration. Its semantic Octest
turn sequences pass interpreted and compiled Go execution. Icarus testbenches
compare board, state, yield, completion, resume-slot, tie-break, hysteresis,
minimum-commit, and fallback observations. The integration lane runs all three
testbenches with Icarus 13.0 and all three FLOW designs through Yosys 0.66
`proc; opt; check; stat`. Qualification reports zero Yosys problems, no latch
inference, combinational cells, and positive synchronous flip-flop cells.

## Arrays, tensors, and roadmap

Container shape is not mathematical tensor shape:

```text
Array<T>       = storage/container; runtime-sized unless semantics say otherwise
Tensor<T,Rank> = mathematical tensor abstraction
Vector<T>      = rank-1 tensor
Matrix<T>      = rank-2 tensor
```

M2 does not reinterpret arrays as hardware vectors or add backend-only extents
to Vector/Matrix. Future static tensor hardware must first define extents in
language type identity. Recorded future work is M3 fixed-shape tensors,
M4 explicit bounded memories, M5 restore/load, M6 operation latency, M7
automatic pipelining, M8 resource sharing, M9 PPA-guided scheduling, and M10
streaming ready/valid interfaces.

No behavior outside `profile Verilog` changes. Interpreter, compiled Go, FLOW
host/checkpoint behavior, and M0/M1 combinational emission remain authoritative.
