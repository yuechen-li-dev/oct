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

// RandomKind classifies a Random builtin by what it needs from the runtime.
type RandomKind string

const (
	// RandomSeed constructs or derives generator state: a v1 Rng from a seed,
	// or a v2 Stream from a seed, a label or an index.
	RandomSeed RandomKind = "seed"
	// RandomDraw is a deterministic draw from explicit generator state.
	RandomDraw RandomKind = "draw"
	// RandomEntropy reads ambient operating-system entropy. It is fallible and
	// is rejected wherever ambient effects are not allowed.
	RandomEntropy RandomKind = "entropy"
)

// RandomParameter is the type of one parameter of a Random builtin.
type RandomParameter string

const (
	RandomParameterInt    RandomParameter = "Int"
	RandomParameterFloat  RandomParameter = "Float"
	RandomParameterString RandomParameter = "String"
	// RandomParameterStream is the record Stream declared in package Random.
	RandomParameterStream RandomParameter = "Stream"
)

// RandomArityCheck records how the typechecker validates the argument count
// of a legacy Random builtin. The three forms preserve the Random v1
// diagnostics exactly; they are retired with v1 in ladder milestone M6.
// Non-legacy builtins always use RandomArityCounted.
type RandomArityCheck string

const (
	// RandomArityCounted reports "expects N arguments, got M".
	RandomArityCounted RandomArityCheck = "counted"
	// RandomArityMismatch reports "arity mismatch" with no counts.
	RandomArityMismatch RandomArityCheck = "mismatch"
	// RandomArityUnchecked performs no argument-count check.
	RandomArityUnchecked RandomArityCheck = "unchecked"
)

// RandomBuiltin is the single description of one compiler-owned Random
// builtin. The typechecker, the interpreter and the compiled backend all
// consult this table; none of them keeps its own list of Random names.
//
// Execution stays in the owning packages, keyed by Implementation(): the
// interpreter's evaluator and the compiled backend's emitter.
type RandomBuiltin struct {
	// Namespace is the Oct package the builtin belongs to.
	Namespace string
	// Symbol is the name inside that package, such as "RandInt".
	Symbol string
	// ImplementedBy names the symbol whose implementation serves this one. It
	// is empty when the builtin has its own implementation.
	ImplementedBy string
	Kind          RandomKind
	// Arguments is the declared argument count.
	Arguments  int
	ArityCheck RandomArityCheck
	// Result is the result type. When ResultInPackage is true it is a record
	// declared in the builtin's package, such as "RandIntResult"; otherwise it
	// is a base type, such as "Int".
	Result          string
	ResultInPackage bool
	Fallible        bool
	// Parameters lists the parameter types of a non-legacy builtin. The
	// typechecker checks every argument against it.
	Parameters []RandomParameter
	// Legacy marks a Random v1 builtin. A legacy builtin keeps the v1 rules
	// until ladder milestone M6 removes it: its unqualified name is reserved
	// in every package, package Random may declare a stub with the same name,
	// and its arguments are not type-checked.
	Legacy bool
}

// randomBuiltins is the table. Adding, renaming or removing a Random builtin
// starts here.
var randomBuiltins = []RandomBuiltin{
	{Namespace: RandomNamespace, Symbol: "RngSeed", Kind: RandomSeed, Arguments: 1, ArityCheck: RandomArityCounted, Result: "Rng", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "RandInt", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityCounted, Result: "RandIntResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "RandFloat01", Kind: RandomDraw, Arguments: 1, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "RandFloatRange", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "RandBernoulli", Kind: RandomDraw, Arguments: 2, ArityCheck: RandomArityCounted, Result: "RandBoolResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "Gaussian", ImplementedBy: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "CryptoRandInt", Kind: RandomEntropy, Arguments: 2, ArityCheck: RandomArityUnchecked, Result: "Int", Fallible: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "CryptoRandFloat01", Kind: RandomEntropy, Arguments: 0, ArityCheck: RandomArityUnchecked, Result: "Float", Fallible: true, Legacy: true},
	{Namespace: RandomNamespace, Symbol: "CryptoRandBytes", Kind: RandomEntropy, Arguments: 1, ArityCheck: RandomArityUnchecked, Result: "Bytes", Fallible: true, Legacy: true},

	// Random v2: counter-based streams. Every draw is a pure function of
	// (stream, index, parameters). Specification: internal/random/
	// RANDOM_V2_LADDER.md, section 3.3. Implementation: internal/octrandom.
	stream("Seeded", RandomSeed, "Stream", RandomParameterInt),
	stream("Fork", RandomSeed, "Stream", RandomParameterStream, RandomParameterString),
	stream("Child", RandomSeed, "Stream", RandomParameterStream, RandomParameterInt),
	stream("Unit", RandomDraw, "Float", RandomParameterStream, RandomParameterInt),
	stream("Between", RandomDraw, "Float", RandomParameterStream, RandomParameterInt, RandomParameterFloat, RandomParameterFloat),
	stream("IntBetween", RandomDraw, "Int", RandomParameterStream, RandomParameterInt, RandomParameterInt, RandomParameterInt),
	stream("Normal", RandomDraw, "Float", RandomParameterStream, RandomParameterInt, RandomParameterFloat, RandomParameterFloat),

	// Entropy: draws from the operating system's random source. They are not
	// reproducible, they are fallible, and they are rejected wherever ambient
	// effects are not allowed. Specification: internal/random/
	// RANDOM_V2_LADDER.md, section 3.5. Implementation: internal/octrandom.
	entropy("Seed", "Int"),
	entropy("IntBetween", "Int", RandomParameterInt, RandomParameterInt),
	entropy("Unit", "Float"),
	entropy("Bytes", "Bytes", RandomParameterInt),
}

// stream describes a Random v2 builtin. Its result is the record Stream when
// result names it, and a base type otherwise.
func stream(symbol string, kind RandomKind, result string, parameters ...RandomParameter) RandomBuiltin {
	return RandomBuiltin{
		Namespace:       RandomNamespace,
		Symbol:          symbol,
		Kind:            kind,
		Arguments:       len(parameters),
		ArityCheck:      RandomArityCounted,
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
		Arguments:  len(parameters),
		ArityCheck: RandomArityCounted,
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
// A legacy builtin also reserves its unqualified name, such as "RandInt", in
// every package. No other builtin does: "Unit" and "Normal" stay available to
// other packages, and resolve to a builtin only inside their own package.
func withRandomBuiltinNames(reserved map[string]struct{}) map[string]struct{} {
	for _, entry := range randomBuiltins {
		reserved[entry.Name()] = struct{}{}
		if entry.Legacy {
			reserved[entry.Symbol] = struct{}{}
		}
	}
	return reserved
}

// Name is the qualified builtin name, such as "Random.RandInt". It is the
// spelling recorded in compiled MIR.
func (b RandomBuiltin) Name() string {
	return b.Namespace + "." + b.Symbol
}

// Implementation is the qualified name of the builtin whose implementation
// executes this one. Interpreter and backend dispatch switch on this value.
func (b RandomBuiltin) Implementation() string {
	if b.ImplementedBy != "" {
		return b.Namespace + "." + b.ImplementedBy
	}
	return b.Name()
}

// HasOwnImplementation reports whether the builtin is executed under its own
// name rather than another builtin's.
func (b RandomBuiltin) HasOwnImplementation() bool {
	return b.ImplementedBy == ""
}

// ResultType is the result type as written from another package:
// "Random.RandIntResult" for a record, "Int" for a base type.
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

// LookupRandom resolves a reserved builtin name: any qualified name, or the
// unqualified name of a legacy builtin. It does not resolve the unqualified
// name of any other builtin, because that depends on the calling package; use
// ResolveRandomCall for a call.
func LookupRandom(name string) (RandomBuiltin, bool) {
	if entry, ok := randomByName[name]; ok {
		return entry, true
	}
	entry, ok := randomBySymbol[RandomNamespace][name]
	if !ok || !entry.Legacy {
		return RandomBuiltin{}, false
	}
	return entry, true
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
