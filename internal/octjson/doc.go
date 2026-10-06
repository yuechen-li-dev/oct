// Package octjson is the one implementation of Oct's JSON rules. The
// interpreter and generated programs both import it, so the two lanes cannot
// read or write JSON differently.
//
// It has four parts, in the order a load uses them:
//
//   - Parse turns text into a Document: a tree that keeps member order, the
//     text of every number and the position of every value.
//   - A Schema describes the Oct type the text is read as. Each lane builds
//     one from its own type information.
//   - Decode reads a Document as a Schema and gives a Data value, which is
//     Octagon data: the lane hands it to the materialiser it uses for
//     LoadOctagon.
//   - Encode writes a Data value of a Schema as text.
//
// Every rule of the Json v2 specification that looks at a document or decides
// how a value is written is in this package (internal/json/JSON_V2_LADDER.md,
// sections 3.3 to 3.8). Neither lane contains one.
//
// The parser is written here, and encoding/json is not used: positions,
// repeated keys, member order and exact integers need a tree that the
// standard library does not give.
package octjson
