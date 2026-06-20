# Loop

`Loop` provides explicit immutable loop-state helpers for cases where ordinary
`for`/`while` syntax is not the clearest fit and where hidden loop-control
keywords would obscure state.

M0 supports increasing integer half-open ranges only: `[Start, End)`, positive
steps, explicit `Loop.Advance` rebinding, and explicit `Loop.Stop` early
termination. It does not provide generators, iterators, hidden state machines,
or persistence/checkpointing.

`Loop.Advance` is intentionally not named `Resume`: Octomata `resume` jumps to a
remembered flow state, while `Loop.Advance` returns a new `RangeState` record.

```oct
import Loop

fn Main() -> Int {
    var loop = Loop.Range(0, 5)
    var sum = 0

    while Loop.IsActive(loop) {
        sum = sum + Loop.Current(loop)
        loop = Loop.Advance(loop)
    }

    return sum
}
```
