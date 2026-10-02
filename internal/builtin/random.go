package builtin

import "strings"

// RandomNamespace is the Oct package whose compiler-owned builtins are
// described by the table in this file.
const RandomNamespace = "Random"

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
	// Symbol is the name inside package Random, such as "RandInt".
	Symbol string
	// ImplementedBy names the symbol whose implementation serves this one. It
	// is empty when the builtin has its own implementation.
	ImplementedBy string
	Kind          RandomKind
	// Arguments is the declared argument count.
	Arguments  int
	ArityCheck RandomArityCheck
	// Result is the result type. When ResultInPackage is true it is a record
	// declared in package Random, such as "RandIntResult"; otherwise it is a
	// base type, such as "Int".
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
	{Symbol: "RngSeed", Kind: RandomSeed, Arguments: 1, ArityCheck: RandomArityCounted, Result: "Rng", ResultInPackage: true, Legacy: true},
	{Symbol: "RandInt", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityCounted, Result: "RandIntResult", ResultInPackage: true, Legacy: true},
	{Symbol: "RandFloat01", Kind: RandomDraw, Arguments: 1, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Symbol: "RandFloatRange", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Symbol: "RandBernoulli", Kind: RandomDraw, Arguments: 2, ArityCheck: RandomArityCounted, Result: "RandBoolResult", ResultInPackage: true, Legacy: true},
	{Symbol: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Symbol: "Gaussian", ImplementedBy: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true, Legacy: true},
	{Symbol: "CryptoRandInt", Kind: RandomEntropy, Arguments: 2, ArityCheck: RandomArityUnchecked, Result: "Int", Fallible: true, Legacy: true},
	{Symbol: "CryptoRandFloat01", Kind: RandomEntropy, Arguments: 0, ArityCheck: RandomArityUnchecked, Result: "Float", Fallible: true, Legacy: true},
	{Symbol: "CryptoRandBytes", Kind: RandomEntropy, Arguments: 1, ArityCheck: RandomArityUnchecked, Result: "Bytes", Fallible: true, Legacy: true},

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
}

// stream describes a Random v2 builtin. Its result is the record Stream when
// result names it, and a base type otherwise.
func stream(symbol string, kind RandomKind, result string, parameters ...RandomParameter) RandomBuiltin {
	return RandomBuiltin{
		Symbol:          symbol,
		Kind:            kind,
		Arguments:       len(parameters),
		ArityCheck:      RandomArityCounted,
		Result:          result,
		ResultInPackage: result == string(RandomParameterStream),
		Parameters:      parameters,
	}
}

var randomBySymbol = indexRandomBuiltins(randomBuiltins)

func indexRandomBuiltins(table []RandomBuiltin) map[string]RandomBuiltin {
	index := make(map[string]RandomBuiltin, len(table))
	for _, entry := range table {
		index[entry.Symbol] = entry
	}
	return index
}

// withRandomBuiltinNames reserves the qualified name of every Random builtin,
// such as "Random.Unit", alongside the other reserved builtin names. A legacy
// builtin also reserves its unqualified name, such as "RandInt", in every
// package. A v2 builtin does not: "Unit" and "Normal" stay available to other
// packages, and resolve to the builtin only inside package Random.
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
	return RandomNamespace + "." + b.Symbol
}

// Implementation is the qualified name of the builtin whose implementation
// executes this one. Interpreter and backend dispatch switch on this value.
func (b RandomBuiltin) Implementation() string {
	if b.ImplementedBy != "" {
		return RandomNamespace + "." + b.ImplementedBy
	}
	return b.Name()
}

// HasOwnImplementation reports whether the builtin is executed under its own
// name rather than another builtin's.
func (b RandomBuiltin) HasOwnImplementation() bool {
	return b.ImplementedBy == ""
}

// ResultType is the result type as written from a package other than Random:
// "Random.RandIntResult" for a record, "Int" for a base type.
func (b RandomBuiltin) ResultType() string {
	if b.ResultInPackage {
		return RandomNamespace + "." + b.Result
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

// IsRandomSymbol reports whether symbol is the unqualified name of a Random
// builtin.
func IsRandomSymbol(symbol string) bool {
	_, ok := randomBySymbol[symbol]
	return ok
}

// LookupRandom resolves either spelling of a Random builtin name.
func LookupRandom(name string) (RandomBuiltin, bool) {
	if symbol, qualified := strings.CutPrefix(name, RandomNamespace+"."); qualified {
		entry, ok := randomBySymbol[symbol]
		return entry, ok
	}
	entry, ok := randomBySymbol[name]
	return entry, ok
}

// LookupRandomQualified resolves only the qualified spelling.
func LookupRandomQualified(name string) (RandomBuiltin, bool) {
	symbol, qualified := strings.CutPrefix(name, RandomNamespace+".")
	if !qualified {
		return RandomBuiltin{}, false
	}
	entry, ok := randomBySymbol[symbol]
	return entry, ok
}

// ResolveRandomCall resolves a call as the execution lanes see it: the
// qualified spelling resolves from any package, and the unqualified spelling
// resolves only for code inside package Random itself.
func ResolveRandomCall(callee string, callerPackage string) (RandomBuiltin, bool) {
	if entry, ok := LookupRandomQualified(callee); ok {
		return entry, true
	}
	if callerPackage != RandomNamespace {
		return RandomBuiltin{}, false
	}
	entry, ok := randomBySymbol[callee]
	return entry, ok
}
