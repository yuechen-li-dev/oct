package builtin

import "strings"

// RandomNamespace is the Oct package whose compiler-owned builtins are
// described by the table in this file.
const RandomNamespace = "Random"

// RandomKind classifies a Random builtin by what it needs from the runtime.
type RandomKind string

const (
	// RandomSeed constructs generator state from a seed.
	RandomSeed RandomKind = "seed"
	// RandomDraw is a deterministic draw from explicit generator state.
	RandomDraw RandomKind = "draw"
	// RandomEntropy reads ambient operating-system entropy. It is fallible and
	// is rejected wherever ambient effects are not allowed.
	RandomEntropy RandomKind = "entropy"
)

// RandomArityCheck records how the typechecker validates the argument count
// of a Random builtin. The three forms preserve the Random v1 diagnostics
// exactly; they are retired with v1 in ladder milestone M6.
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
}

// randomBuiltins is the table. Adding, renaming or removing a Random builtin
// starts here.
var randomBuiltins = []RandomBuiltin{
	{Symbol: "RngSeed", Kind: RandomSeed, Arguments: 1, ArityCheck: RandomArityCounted, Result: "Rng", ResultInPackage: true},
	{Symbol: "RandInt", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityCounted, Result: "RandIntResult", ResultInPackage: true},
	{Symbol: "RandFloat01", Kind: RandomDraw, Arguments: 1, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true},
	{Symbol: "RandFloatRange", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true},
	{Symbol: "RandBernoulli", Kind: RandomDraw, Arguments: 2, ArityCheck: RandomArityCounted, Result: "RandBoolResult", ResultInPackage: true},
	{Symbol: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true},
	{Symbol: "Gaussian", ImplementedBy: "RandNormal", Kind: RandomDraw, Arguments: 3, ArityCheck: RandomArityMismatch, Result: "RandFloatResult", ResultInPackage: true},
	{Symbol: "CryptoRandInt", Kind: RandomEntropy, Arguments: 2, ArityCheck: RandomArityUnchecked, Result: "Int", Fallible: true},
	{Symbol: "CryptoRandFloat01", Kind: RandomEntropy, Arguments: 0, ArityCheck: RandomArityUnchecked, Result: "Float", Fallible: true},
	{Symbol: "CryptoRandBytes", Kind: RandomEntropy, Arguments: 1, ArityCheck: RandomArityUnchecked, Result: "Bytes", Fallible: true},
}

var randomBySymbol = indexRandomBuiltins(randomBuiltins)

func indexRandomBuiltins(table []RandomBuiltin) map[string]RandomBuiltin {
	index := make(map[string]RandomBuiltin, len(table))
	for _, entry := range table {
		index[entry.Symbol] = entry
	}
	return index
}

// withRandomBuiltinNames reserves both spellings of every Random builtin, the
// qualified "Random.RandInt" and the unqualified "RandInt", alongside the
// other reserved builtin names.
func withRandomBuiltinNames(reserved map[string]struct{}) map[string]struct{} {
	for _, entry := range randomBuiltins {
		reserved[entry.Symbol] = struct{}{}
		reserved[entry.Name()] = struct{}{}
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
	return append([]RandomBuiltin(nil), randomBuiltins...)
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
