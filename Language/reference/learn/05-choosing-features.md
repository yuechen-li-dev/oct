# 5. Choosing Oct's unusual features

[Previous](04-collections.md) · [Reference home](../README.md)

Oct's less familiar constructs each own a specific kind of work. Start from
the result you need, then pick the construct that makes it explicit.

| Need | Construct | What it owns |
| --- | --- | --- |
| One result from typed inputs | `fn` | Computation and explicit failure |
| Reuse one definition for several exact types | `template` | Concrete specialization before ordinary checking |
| Refer to a field without naming it in a string | `Selector<R, T>` and `.Field` | Exact owner and field type |
| Select among eligible ranked outcomes | `when utility` | One-shot scoring with stable source-order ties |
| Filter and yield source values incrementally | `query` | One bounded FLOW-backed source scan |
| Map independent inputs to ordered results | `batch` | Implicit join and deterministic failure selection |
| Keep behavior across turns | `flow` / `state` / `board` | Explicit continuation and state-owned memory |
| Select a transition by guard order | State-local `when` | First true action |
| Keep a ranked choice stable over time | State-local `when policy` | Commitment, hysteresis, and minimum hold age |
| Suspend a sequential computation for a child | `async fn` / `await` | Compiler-generated FLOW continuation |
| Author typed document-like trees | Oct-XML | Ordinary function calls and typed child values |
| Publish data or documents during a build phase | `[Artifact]` and `StaticAssert` | Checked generation and confined publication |

Here are templates, a query, one-shot judgment, and a reactive FLOW together.
Each still works with ordinary typed values:

<!-- verified-example: ../../ReferenceExamples/valid/unusual_features.octest -->
```oct
package ReferenceExamples

template record ReadingBox<T> { Value: T }

query PositiveReadings(source: Int[], limit: Int) yields Int {
    filter { item > 0 }
    take limit
}

enum ReadingAction { Wait Review Urgent }

fn ChooseReadingAction(level: Int) -> ReadingAction {
    return when utility ReadingAction {
        case ReadingAction.Urgent when level > 90 score 100
        case ReadingAction.Review when level > 50 score 60
        else ReadingAction.Wait
    }
}

flow ReadingCounter() accepts amount: Int yields Int -> Int {
    board { Total: Int }
    state Count {
        board.Total = board.Total + amount
        yield board.Total
        goto Count
    }
}

[Fact]
fn UnusualFeaturesUseOrdinaryTypedValues() -> Void ! Error {
    let boxed = ReadingBox<Int> { Value: 7 }
    Assert.Equal(7, boxed.Value, "templates specialize to concrete records")
    Assert.Equal(2, Query.Count(PositiveReadings([-1, 2, 0, 3, 4], 2)), "queries stop at take")
    Assert.Equal(ReadingAction.Urgent, ChooseReadingAction(95), "the highest eligible score wins")

    let counter = ReadingCounter()
    Step(counter, 2)
    Assert.Equal(2, Yielded(counter)?, "the first turn yields a value")
    Step(counter, 5)
    Assert.Equal(7, Yielded(counter)?, "the board persists between turns")
    Assert.False(Complete(counter), "yield does not complete the flow")
}
```

Run the [source](../../ReferenceExamples/valid/unusual_features.octest) with
`oct test <source> --execution compiled`.

## Notice the different kinds of time

The template specializes before execution. `when utility` makes a fresh choice
on each evaluation. A query yields as it is consumed; `Query.Count` consumes
that instance, so construct a new instance for another scan.

The FLOW is constructed without running its body. Each `Step` supplies one
turn input and runs until `yield`, `suspend`, or final `return`. `goto` runs
the next state within that same turn. In this example, `yield` preserves the
continuation, and the following `goto Count` begins the next activation on the
next `Step`. Only the board keeps `Total` between turns; state locals do not
persist across a boundary.

`async fn` offers sequential spelling for awaiting a child FLOW, using this
same execution model. It does not introduce a scheduler, background task,
thread pool, or implicit I/O concurrency. `batch` is the distinct construct
for independent array work.

## Pick your next reference

- [Templates and selectors](../language/templates-and-selectors.md): exact specialization and field references.
- [Utility selection](../language/utility-selection.md): eligibility, scores, ties, and selected-only evaluation.
- [Queries](../runtime/queries.md): sources, stage order, and consuming terminals.
- [Octomata](../runtime/flows-and-state-machines.md): transitions, suspension, boards, and explicit resume slots.
- [Turns and checkpoints](../runtime/turns-and-checkpoints.md): input/yield and durable continuation boundaries.
- [Controller policy](../runtime/utility-policy.md): repeated decisions with memory.
- [Async](../runtime/async.md) and [batch](../runtime/batch.md): two distinct execution needs.
- [Oct-XML](../language/typed-markup.md): typed markup with no separate runtime.
- [Artifacts and benchmarks](../tooling/artifacts-and-benchmarks.md): executable output and measurement examples.

For specialized backends, native interoperability, and experimental tools,
use the [support map](../support.md). Their availability is explicit rather
than a prerequisite for learning the language.
