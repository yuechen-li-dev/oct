// Package jsoninfer proposes the Oct declarations a JSON document loads
// into. It is what `oct json infer` runs. Specification: section 3.11 of
// internal/json/JSON_V2_LADDER.md.
//
// Inference is tooling. A program never infers: it declares a type and
// `Json.Load<T>` reads the document as that type (decisions D1, D7 and D10 of
// the ladder). This package reads a document the other way round, once, so
// that a person has declarations to start from.
//
// It works in three steps:
//
//   - shape.go reads the document into shapes: what the values seen at one
//     place have in common. The rows of an array and the instances of an
//     object are folded into one shape, so a member that is null or absent
//     somewhere is known to be optional.
//   - decide.go makes the two choices that have several signals and no
//     single one that decides: whether an object is a record or a keyed
//     table, and whether an array of objects is a table or a tagged array.
//     Both go through internal/judgment and leave a trace.
//   - declare.go turns shapes into named declarations, and print.go writes
//     them.
//
// Everything else is a rule with one answer and is written as one. The
// rules about what a declaration reads are not here: they are in
// internal/octjson, and the output of this package is checked against them
// by loading every document with the declarations inferred for it
// (Language/Tooling/JsonInfer).
package jsoninfer
