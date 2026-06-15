# OFIX1 deferred: board field indexed assignment

`board.ArrayField[i] = value` remains deferred after the OFIX1 hotfix sweep.

Narrow repro:

```oct
package Main

flow F() -> Int {
    board {
        Values: Int[]
    }

    state Start {
        board.Values = [1, 2, 3]
        board.Values[1] = 99
        return board.Values[1]
    }
}
```

Current blocker: ordinary local nested index assignment now has a direct AST target
(`IndexAssignStmt`) and lowering path, but board writes still use the separate
flow field-assignment lowering path (`FieldAssignStmt`) and flow-specific MIR.
Supporting this safely requires a dedicated board-field indexed lvalue rather
than overloading local index assignment or broadening arbitrary record lvalues.

Recommended next task: add a narrow AST/MIR form for direct board field index
assignment only (`board.Field[i] = value`), then typecheck it against the board
field element type and lower it in interpreted and compiled flow state bodies.
