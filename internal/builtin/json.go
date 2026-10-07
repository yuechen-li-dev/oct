package builtin

// JsonNamespace is the package of the Json builtins. They are implemented in
// internal/octjson, for both lanes. Specification:
// internal/json/JSON_V2_LADDER.md.
const JsonNamespace = "Json"

// JsonSource is where a Json builtin takes its JSON text from.
type JsonSource string

const (
	// JsonFromFile reads the file its argument names.
	JsonFromFile JsonSource = "file"
	// JsonFromText reads its argument.
	JsonFromText JsonSource = "text"
)

// JsonBuiltin is the single description of one Json builtin. The typechecker,
// the interpreter and the compiled backend consult this table; none of them
// keeps its own list of names.
//
// Each takes one type argument, the type the JSON is read as, and one String
// argument, and is fallible.
type JsonBuiltin struct {
	// Symbol is the name inside the namespace, such as "Load".
	Symbol string
	Source JsonSource
}

// jsonBuiltins is the table. Adding, renaming or removing a builtin starts
// here.
var jsonBuiltins = []JsonBuiltin{
	{Symbol: "Load", Source: JsonFromFile},
	{Symbol: "Parse", Source: JsonFromText},
}

// Name is the qualified name, such as "Json.Load". It is the spelling
// recorded in compiled MIR and the name a message gives the function.
func (b JsonBuiltin) Name() string {
	return JsonNamespace + "." + b.Symbol
}

// JsonBuiltins returns the table in declaration order.
func JsonBuiltins() []JsonBuiltin {
	return append([]JsonBuiltin(nil), jsonBuiltins...)
}

// LookupJson resolves a qualified name, such as "Json.Load".
func LookupJson(name string) (JsonBuiltin, bool) {
	for _, entry := range jsonBuiltins {
		if entry.Name() == name {
			return entry, true
		}
	}
	return JsonBuiltin{}, false
}

// ResolveJsonCall resolves a call written as callee with typeArguments type
// arguments. The Json builtins need no import.
//
// While the first Json library is still present (it leaves in milestone M5
// of the ladder), `Json.Load(path)` without a type argument is its function
// and not this table's.
func ResolveJsonCall(callee string, typeArguments int) (JsonBuiltin, bool) {
	entry, ok := LookupJson(callee)
	if !ok {
		return JsonBuiltin{}, false
	}
	if _, firstLibrary := ResolveNamespacedAlias(JsonNamespace, entry.Symbol); firstLibrary && typeArguments == 0 {
		return JsonBuiltin{}, false
	}
	return entry, true
}
