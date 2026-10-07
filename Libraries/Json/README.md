# Json

`Json` reads JSON as a type the program declares, and writes a value of such
a type as JSON.

```oct
record table Ticket {
    Id:       String
    Assignee: Option<String>
    Points:   Int
}

fn OpenPoints(path: String) -> Int ! Error {
    let tickets = Json.Load<Ticket>(path)?
    var points = 0
    for row in 0..Len(tickets) {
        points = points + tickets[row].Points
    }
    return points
}
```

## API

- `Json.Load<T>(path: String) -> T ! Error`: reads a file as a `T`.
- `Json.Parse<T>(text: String) -> T ! Error`: reads text as a `T`.
- `Json.Save(path: String, value: T) -> Void ! Error`: writes a value to a file.
- `Json.Text(value: T) -> String`: the JSON text of a value.
- `Artifact.WriteJson(path: String, value: T)`: publishes a value during
  `oct artifact` evaluation.

`Json` is a compiler-owned namespace, like `Artifact` and `Entropy`. The
functions are builtins implemented in Go (`internal/octjson`) for both
execution lanes, with no sidecar, and they need no `import`. `import Json` is
allowed. No package may declare a function named `Load`, `Parse`, `Save` or
`Text` inside package `Json`.

## Rules

- The declared type says how a document is read and how a value is written.
  Nothing is guessed from the document, and there is no untyped value: a
  JSON object is a `record`, an array of objects is a `record table`.
- A type must have a JSON form in every part, or the call is a compile error
  that names the part.
- Reading is strict: a member that names no field, a missing member that is
  not an `Option`, and a value of the wrong kind are each an `Error` that
  gives the file, the path into the document, the line and the column.
- A key and a field are one name with `_`, `-`, `.`, spaces and case ignored.
  Writing uses the field name as declared.
- The two lanes read equal values, write equal bytes and give equal error
  text.

The full rules, with the table of types, are in
`Language/reference/language/17-standard-libraries.md` under "Json". The
contracts are in `Language/Builtins/Json`. The specification is
`internal/json/JSON_V2_LADDER.md`.
