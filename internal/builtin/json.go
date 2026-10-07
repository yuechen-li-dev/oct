package builtin

// JsonNamespace is the package of the Json builtins. They are implemented in
// internal/octjson, for both lanes. Specification:
// internal/json/JSON_V2_LADDER.md.
const JsonNamespace = "Json"

// JsonAction is what a Json builtin does.
type JsonAction string

const (
	// JsonReadFile is `Json.Load<T>(path) -> T ! Error`.
	JsonReadFile JsonAction = "read file"
	// JsonReadText is `Json.Parse<T>(text) -> T ! Error`.
	JsonReadText JsonAction = "read text"
	// JsonWriteFile is `Json.Save(path, value) -> Void ! Error`.
	JsonWriteFile JsonAction = "write file"
	// JsonWriteText is `Json.Text(value) -> String`.
	JsonWriteText JsonAction = "write text"
	// JsonWriteArtifact is `Artifact.WriteJson(path, value)`, which exists
	// during `oct artifact` evaluation.
	JsonWriteArtifact JsonAction = "write artifact"
)

// JsonBuiltin is the single description of one Json builtin. The parser, the
// typechecker, the interpreter and the compiled backend consult this table;
// none of them keeps its own list of names.
//
// Every call is made at a type T, which is the call's one type argument. A
// reader's is written, `Json.Load<Ticket>(path)`. A writer's is the type of
// the value it is given: the parser leaves the type argument as an empty
// slot and the typechecker writes T into it, so that the lanes read T there
// for a writer as they do for a reader.
type JsonBuiltin struct {
	// Namespace and Symbol are the name as written, `Json` and `Load`.
	Namespace string
	Symbol    string
	Action    JsonAction
}

// jsonBuiltins is the table. Adding, renaming or removing a builtin starts
// here.
var jsonBuiltins = []JsonBuiltin{
	{Namespace: JsonNamespace, Symbol: "Load", Action: JsonReadFile},
	{Namespace: JsonNamespace, Symbol: "Parse", Action: JsonReadText},
	{Namespace: JsonNamespace, Symbol: "Save", Action: JsonWriteFile},
	{Namespace: JsonNamespace, Symbol: "Text", Action: JsonWriteText},
	{Namespace: "Artifact", Symbol: "WriteJson", Action: JsonWriteArtifact},
}

// Name is the qualified name, such as "Json.Load". It is the spelling
// recorded in compiled MIR and the name a message gives the function.
func (b JsonBuiltin) Name() string {
	return b.Namespace + "." + b.Symbol
}

// Writes reports whether the builtin writes a value as JSON; the others read
// one.
func (b JsonBuiltin) Writes() bool {
	return b.Action == JsonWriteFile || b.Action == JsonWriteText || b.Action == JsonWriteArtifact
}

// HasFirstLibraryForm reports whether the first Json library has a function
// of this name, which a call may still mean until that library leaves in
// milestone M5 of the ladder: `Json.Load(path)` with no type argument, and
// `Json.Save(path, text)` and `Artifact.WriteJson(path, text)` given a String
// of JSON text. The typechecker decides which a call is, and a call of the
// first library's form carries no type.
func (b JsonBuiltin) HasFirstLibraryForm() bool {
	_, has := ResolveNamespacedAlias(b.Namespace, b.Symbol)
	return has
}

// JsonBuiltins returns the table in declaration order.
func JsonBuiltins() []JsonBuiltin {
	return append([]JsonBuiltin(nil), jsonBuiltins...)
}

// LookupJson resolves a qualified name, such as "Json.Load". The Json
// builtins need no import.
func LookupJson(name string) (JsonBuiltin, bool) {
	for _, entry := range jsonBuiltins {
		if entry.Name() == name {
			return entry, true
		}
	}
	return JsonBuiltin{}, false
}
