package builtin

// The table in this file describes the compiler-owned randomness builtins of
// two Oct packages.
const (
	// RandomNamespace is deterministic: seeded generators and streams.
	RandomNamespace = "Random"
	// EntropyNamespace reads the operating system's random source. It is a
	// compiler-owned namespace: its builtins need no import.
	EntropyNamespace = "Entropy"
)

// RandomKind classifies a builtin by what it needs from the runtime.
type RandomKind string

const (
	// RandomSeed constructs or derives a Stream: from a seed, a label or an
	// index.
	RandomSeed RandomKind = "seed"
	// RandomDraw is a deterministic draw from a Stream at an index.
	RandomDraw RandomKind = "draw"
	// RandomEntropy reads ambient operating-system entropy. It is fallible and
	// is rejected wherever ambient effects are not allowed.
	RandomEntropy RandomKind = "entropy"
)

// RandomParameter is the type of one parameter of a builtin.
type RandomParameter string

const (
	RandomParameterInt    RandomParameter = "Int"
	RandomParameterFloat  RandomParameter = "Float"
	RandomParameterString RandomParameter = "String"
	// RandomParameterStream is the record Stream declared in package Random.
	RandomParameterStream RandomParameter = "Stream"
)

// RandomBuiltin is the single description of one compiler-owned Random or
// Entropy builtin. The typechecker, the interpreter and the compiled backend
// all consult this table; none of them keeps its own list of names.
//
// Execution stays in the owning packages, keyed by Name(): the interpreter's
// evaluator and the compiled backend's emitter.
type RandomBuiltin struct {
	// Namespace is the Oct package the builtin belongs to.
	Namespace string
	// Symbol is the name inside that package, such as "Unit".
	Symbol string
	Kind   RandomKind
	// Result is the result type. When ResultInPackage is true it is a record
	// declared in the builtin's package, "Stream"; otherwise it is a base
	// type, such as "Int".
	Result          string
	ResultInPackage bool
	Fallible        bool
	// Parameters lists the parameter types. The typechecker checks the
	// argument count and every argument against it.
	Parameters []RandomParameter
}

// randomBuiltins is the table. Adding, renaming or removing a builtin starts
// here.
var randomBuiltins = []RandomBuiltin{
	// Random: counter-based streams. Every draw is a pure function of
	// (stream, index, parameters). Specification: internal/random/Random.md.
	// Implementation: internal/octrandom.
	stream("Seeded", RandomSeed, "Stream", RandomParameterInt),
	stream("Fork", RandomSeed, "Stream", RandomParameterStream, RandomParameterString),
	stream("Child", RandomSeed, "Stream", RandomParameterStream, RandomParameterInt),
	stream("Unit", RandomDraw, "Float", RandomParameterStream, RandomParameterInt),
	stream("Between", RandomDraw, "Float", RandomParameterStream, RandomParameterInt, RandomParameterFloat, RandomParameterFloat),
	stream("IntBetween", RandomDraw, "Int", RandomParameterStream, RandomParameterInt, RandomParameterInt, RandomParameterInt),
	stream("Normal", RandomDraw, "Float", RandomParameterStream, RandomParameterInt, RandomParameterFloat, RandomParameterFloat),

	// Entropy: draws from the operating system's random source. They are not
	// reproducible, they are fallible, and they are rejected wherever ambient
	// effects are not allowed. Specification: internal/random/Random.md.
	// Implementation: internal/octrandom.
	entropy("Seed", "Int"),
	entropy("IntBetween", "Int", RandomParameterInt, RandomParameterInt),
	entropy("Unit", "Float"),
	entropy("Bytes", "Bytes", RandomParameterInt),
}

// stream describes a Random builtin. Its result is the record Stream when
// result names it, and a base type otherwise.
func stream(symbol string, kind RandomKind, result string, parameters ...RandomParameter) RandomBuiltin {
	return RandomBuiltin{
		Namespace:       RandomNamespace,
		Symbol:          symbol,
		Kind:            kind,
		Result:          result,
		ResultInPackage: result == string(RandomParameterStream),
		Parameters:      parameters,
	}
}

// entropy describes an Entropy builtin. Its result is a base type and it is
// fallible: the operating system's random source can fail.
func entropy(symbol string, result string, parameters ...RandomParameter) RandomBuiltin {
	return RandomBuiltin{
		Namespace:  EntropyNamespace,
		Symbol:     symbol,
		Kind:       RandomEntropy,
		Result:     result,
		Fallible:   true,
		Parameters: parameters,
	}
}

// randomByName indexes the table by qualified name, such as "Random.Unit".
var randomByName = func() map[string]RandomBuiltin {
	index := make(map[string]RandomBuiltin, len(randomBuiltins))
	for _, entry := range randomBuiltins {
		index[entry.Name()] = entry
	}
	return index
}()

// randomBySymbol indexes the table by namespace and then by unqualified
// symbol. The two namespaces share symbols, such as "Unit" and "IntBetween",
// so an unqualified symbol means nothing without its package.
var randomBySymbol = func() map[string]map[string]RandomBuiltin {
	index := map[string]map[string]RandomBuiltin{}
	for _, entry := range randomBuiltins {
		if index[entry.Namespace] == nil {
			index[entry.Namespace] = map[string]RandomBuiltin{}
		}
		index[entry.Namespace][entry.Symbol] = entry
	}
	return index
}()

// withRandomBuiltinNames reserves the qualified name of every randomness
// builtin, such as "Random.Unit", alongside the other reserved builtin names.
// The unqualified names are not reserved: "Unit" and "Normal" stay available
// to other packages, and resolve to a builtin only inside their own package.
func withRandomBuiltinNames(reserved map[string]struct{}) map[string]struct{} {
	for _, entry := range randomBuiltins {
		reserved[entry.Name()] = struct{}{}
	}
	return reserved
}

// Name is the qualified builtin name, such as "Random.Unit". It is the
// spelling recorded in compiled MIR and the key the execution lanes dispatch
// on.
func (b RandomBuiltin) Name() string {
	return b.Namespace + "." + b.Symbol
}

// ResultType is the result type as written from another package:
// "Random.Stream" for the record, "Int" for a base type.
func (b RandomBuiltin) ResultType() string {
	if b.ResultInPackage {
		return b.Namespace + "." + b.Result
	}
	return b.Result
}

// RandomBuiltins returns the table in declaration order.
func RandomBuiltins() []RandomBuiltin {
	table := make([]RandomBuiltin, len(randomBuiltins))
	for i, entry := range randomBuiltins {
		entry.Parameters = append([]RandomParameter(nil), entry.Parameters...)
		table[i] = entry
	}
	return table
}

// IsRandomNamespace reports whether name is a package whose builtins this
// table describes.
func IsRandomNamespace(name string) bool {
	return name == RandomNamespace || name == EntropyNamespace
}

// LookupRandomQualified resolves a qualified name, such as "Random.Unit".
func LookupRandomQualified(name string) (RandomBuiltin, bool) {
	entry, ok := randomByName[name]
	return entry, ok
}

// LookupRandomIn resolves an unqualified symbol as code inside the package
// namespace writes it.
func LookupRandomIn(namespace string, symbol string) (RandomBuiltin, bool) {
	if !IsRandomNamespace(namespace) {
		return RandomBuiltin{}, false
	}
	entry, ok := randomBySymbol[namespace][symbol]
	return entry, ok
}

// ResolveRandomCall resolves a call from callerPackage: the qualified spelling
// resolves from any package, and the unqualified spelling resolves only for
// code inside the builtin's own package.
func ResolveRandomCall(callee string, callerPackage string) (RandomBuiltin, bool) {
	if entry, ok := randomByName[callee]; ok {
		return entry, true
	}
	return LookupRandomIn(callerPackage, callee)
}
