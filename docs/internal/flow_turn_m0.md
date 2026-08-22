# FLOW-TURN-M0 — typed reactive flows and `yield`

## 1. Verdict

**Meaningful progression**

Typed post-construction input and typed `yield` now work through the interpreter and the existing compiled flow MIR/emitter, with language contracts proving continuation, private-board persistence, input replacement, generator-shaped use, and final completion. The next isolated blocker is the durable/host seam: compiled logical checkpoint/restore and an exported generated Go facade do not yet exist.

## 2. Motivation

External controller dogfood needed to deliver an immutable observation to an already-resident flow and receive one decision without rebuilding or completing that flow. That is a general state-machine requirement; it does not justify shared boards, mutable records, mailboxes, actor syntax, or database-owned semantics inside Octomata.

## 3. Existing Octomata model

An Octomata `flow` is a persistent runtime value. Constructor parameters are lifetime bindings. `state` and `goto` define explicit control progression. A fixed-shape private `board` stores control memory. `Step` runs until `suspend` or `return`; `suspend` preserves continuation while `return` permanently completes. `remember`/`resume` use one flow-local resume slot. The interpreter has a logical checkpoint payload; generated Go already specializes history, resume, utility-policy, and board representation by reachable features.

## 4. Final turn model

Construction creates one persistent instance. An input-bearing turn installs one immutable value, resumes at the current instruction, follows ordinary transitions, and ends at the first `yield`, `suspend`, or final `return`. The input binding is removed/zeroed as the step returns. `yield` leaves the flow incomplete and records one typed turn value. `suspend` records no value. `return` records the final result and completes permanently.

## 5. Syntax

```oct
flow AccountController(id: Int) accepts message: Command yields Decision -> FinalResult {
    board { Count: Int }
    state Active {
        board.Count = board.Count + 1
        yield Decide(message, board.Count)
        goto Active
    }
}
```

`accepts name: Type` and `yields Type` are optional and independent. Plain flows retain their prior syntax.

## 6. Type model

The static flow value carries flow identity plus three distinct contracts: optional input type, optional yield type, and final return type. Yield and final types may differ. All yield sites must produce the declared yield type; heterogeneous values require a common nominal enum or record. A flow may return without yielding.

`Step(flow)` and `Step(flow, input)` return `Void`. `DidYield(flow)` returns `Bool`; `Yielded(flow)` returns the statically declared yield type fallibly. `Result(flow)` continues to return the final type fallibly and remains unavailable after yields until final completion.

## 7. Lowering

The parser adds metadata and `YieldStmt` to the existing flow AST. The existing `lowerFlow` path adds optional turn-input/yield metadata and `MIRFlowYield`; it does not create a coroutine, generator, actor, or agent compiler. Generated Go advances the existing instruction cursor before returning from a yield, so the next step resumes after the yield.

## 8. Yield semantics

The yield expression is evaluated once. Prior board writes are not repeated. `Complete(flow)` remains false. The next step starts at the following instruction. A yield may occur without turn input. A plain `suspend` without a value remains legal.

State-local bindings are transient. Once a statement may yield, bindings declared before it are out of scope afterward; persistent values must be construction parameters or explicit private-board fields. This conservative M0 rule prevents interpreter/generated-Go continuation divergence and avoids serializing arbitrary environments.

## 9. Reactive Step semantics

Input-bearing flows statically reject missing input, and non-input flows reject supplied input. Wrong types are rejected. Each step replaces the binding; interpreter state deletes it at turn end and generated Go zeros the feature-specific input field. Input-bearing steps after completion are runtime errors. A flow may accept input and then suspend or complete without yielding.

## 10. Checkpoint model

The existing interpreter checkpoint remains the only implemented logical checkpoint model. Turn input is excluded after a completed turn, and the local-lifetime rule establishes the main static prerequisite for a canonical yield-safe schema. However, FLOW-TURN-M0 does **not** yet add an explicit yield-safe checkpoint discriminator/schema, yielded-boundary validation, or a compiled materializer/restorer. Therefore checkpoint/restore parity at yield is the isolated next blocker, not claimed complete.

## 11. Generated host ABI

No exported host facade is claimed. Generated flows still use compiler-private Oct builtin plumbing. An intentional `NewFoo`/typed `Step`/`Checkpoint`/`RestoreFoo` surface remains blocked on choosing and implementing the common compiled logical checkpoint schema. Exposing private fields now would freeze the wrong seam.

## 12. Feature specialization

Plain flows do not gain input or yield storage. Reactive flows add exactly one typed transient input field. Yielding flows add one typed last-yield field plus a presence bit. Board/history/resume/utility fields remain controlled by their existing analyses. Interface inspection methods exist uniformly, but unused persistent storage is omitted. Compiled checkpoint helpers are not emitted because that feature is not implemented.

## 13. Iterator interpretation

Iterator syntax is deferred. Explicit `Step(generator)` plus `Yielded(generator)!` proves the architectural model without prematurely overloading `for`. Yield is a resumable state-machine boundary, not construction of a lazy collection.

## 14. Dogfood

The canonical contracts include:

- an input-free counter generator;
- a reactive counter whose private board accumulates new per-turn input;
- a reactive flow that yields a waiting value and later returns a distinct final `Int`.

Nominal command/decision agent-shaped dogfood is not yet added; the compiler already supports nominal records/enums in flow expressions, but the missing host/checkpoint seam is the more important next proof.

## 15. Performance

The required four-way benchmark matrix was not run, because exported host stepping, compiled checkpoint export, and restore do not exist. Reporting checkpoint/restore time or host instance construction bytes would be fabricated. Existing plain-flow specialization is preserved, and targeted generated execution completes without interpreter fallback. A complete performance table belongs with the compiled host/checkpoint implementation.

## 16. Compatibility

Targeted Go packages pass, and all new valid contracts pass interpreted and compiled with zero fallback. Existing `Step(flow)` remains source-compatible for non-input flows; `suspend`, `return`, board, history, and remember/resume behavior are unchanged. `yield` is a new reserved keyword.

## 17. Pressure findings

- Missing general abstraction (removed): typed post-construction turn input.
- Missing general abstraction (removed): typed non-completing yield and continuation.
- Bug (prevented): state locals could otherwise diverge across interpreter and compiled yield resumption.
- Compiler gap: compiled logical checkpoint/restore at canonical yield boundaries.
- API gap: intentional exported generated Go host facade and typed turn outcome.
- Documentation gap (removed): turn/yield/input lifetime is now in the authoritative Octomata reference.
- Not worth solving now: iterator `for` sugar, language mailboxes, actors, shared boards, global flow identity.

## 18. OctetDB Write readiness

The language can now express and compile `Step(agent, message)` followed by typed yielded-decision inspection, with external authoritative state and host-owned mailbox/registry policy. It is **not yet durable-host ready** because a consumer would still need compiler-private generated names and has no supported compiled checkpoint/restore. The motivating external milestone should not resume its durability integration yet.

## 19. Remaining limitations

There is no explicit first-class turn-result enum; callers use `DidYield`, `Yielded`, `Complete`, and `Result`. Plain suspension is distinguishable by `DidYield == false` and `Complete == false`. Runtime errors are not represented in a turn value. Compiled restore, fingerprint validation, schema migration, checkpoint benchmarks, exported host names, and nominal agent-shaped external dogfood remain. Nested local lifetime is conservatively cut at any statement containing a yield.

## 20. Exactly one next recommendation

Implement one versioned logical yield-boundary checkpoint schema with interpreter and compiled materializers, then expose the generated typed Go host facade over that schema.

## Design questions answered

1. A yielding flow is a nominal flow instance carrying optional input, yield, and final types.
2. Yield and final return types are distinct.
3. `Step` returns `Void`; outcome inspection is explicit.
4. `Result` remains fallible/unavailable until final return.
5. One turn ends at yield, suspend, or return after ordinary transitions.
6. Yes, a flow can suspend without yielding.
7. Yes, a flow can yield without input.
8. Yes, an input flow can suspend or return without yielding.
9. Control state, construction parameters, board, resume slot, and utility state may survive yield.
10. No state local declared before a possible yield remains in scope after it.
11. A yield-safe checkpoint excludes turn input and transient locals and is taken after the continuation cursor advances; its versioned compiled schema remains unimplemented.
12. Construction parameters persist; turn input is installed and cleared per step.
13. Interpreter deletion and generated typed zeroing remove the current message.
14. Existing persistent utility-site fields/maps survive ordinary yields; checkpoint parity remains to prove.
15. `remember`/`resume` are unchanged and their slot survives ordinary yield.
16. No generated host ABI promise is made in this progression.
17. Input and yield fields disappear when unused; existing history/resume/utility fields remain feature-driven.
18. Iterator syntax is not yet justified.
19. Yes: parser AST -> existing flow MIR -> existing interpreter/emitter owns generator and reactive cases.
20. Remaining friction is compiled durability plus a supported typed host facade.
