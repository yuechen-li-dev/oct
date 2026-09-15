package builtin

import (
	"fmt"
	"sort"
	"strings"
)

// Definition is backend-neutral semantic metadata for a compiler-owned
// builtin. Execution functions and generated-code templates deliberately stay
// in their owning interpreter and backend packages.
type Definition struct {
	Name                 string
	Aliases              []string
	CallShape            CallShape
	ParameterConstraints []ParameterConstraint
	ReturnRule           ReturnRule
	Fallibility          Fallibility
	Traits               []Trait
}

// CallShape records source-level arity when it is regular enough to be shared.
// Irregular builtins retain their semantic hook in the typechecker.
type CallShape struct {
	Known            bool
	MinimumArguments int
	MaximumArguments int
	MinimumTypeArgs  int
	MaximumTypeArgs  int
}

type Fallibility string

const (
	FallibilityIrregular  Fallibility = "irregular"
	FallibilityInfallible Fallibility = "infallible"
	FallibilityFallible   Fallibility = "fallible"
)

type Trait string

const (
	TraitPure         Trait = "pure"
	TraitRuntime      Trait = "runtime"
	TraitCompilerOnly Trait = "compiler-only"
	TraitFlow         Trait = "flow"
	TraitInterpreted  Trait = "interpreted"
	TraitCompiled     Trait = "compiled"
)

type ParameterConstraint string

const (
	ParameterAnySized          ParameterConstraint = "sized-value"
	ParameterArray             ParameterConstraint = "array"
	ParameterElementOfFirst    ParameterConstraint = "element-of-first-array"
	ParameterRealNumericScalar ParameterConstraint = "real-numeric-scalar"
	ParameterFloatScalar       ParameterConstraint = "float-scalar"
	ParameterIntScalar         ParameterConstraint = "int-scalar"
	ParameterString            ParameterConstraint = "string"
	ParameterStringArray       ParameterConstraint = "string-array"
	ParameterBoolArray         ParameterConstraint = "bool-array"
	ParameterRange             ParameterConstraint = "range"
)

type ReturnRule string

const (
	ReturnSemanticHook ReturnRule = "semantic-hook"
	ReturnFirstType    ReturnRule = "first-argument-type"
	ReturnInt          ReturnRule = "Int"
	ReturnFloat        ReturnRule = "Float"
	ReturnComplex      ReturnRule = "Complex"
	ReturnString       ReturnRule = "String"
)

type semanticRule struct {
	Parameters []ParameterConstraint
	Return     ReturnRule
}

var regularSemanticRules = map[string]semanticRule{
	"Len":               {Parameters: []ParameterConstraint{ParameterAnySized}, Return: ReturnInt},
	"Append":            {Parameters: []ParameterConstraint{ParameterArray, ParameterElementOfFirst}, Return: ReturnFirstType},
	"Complex":           {Parameters: []ParameterConstraint{ParameterRealNumericScalar, ParameterRealNumericScalar}, Return: ReturnComplex},
	"ComplexPolar":      {Parameters: []ParameterConstraint{ParameterRealNumericScalar, ParameterRealNumericScalar}, Return: ReturnComplex},
	"I":                 {Return: ReturnComplex},
	"Real":              {Return: ReturnFloat},
	"Imag":              {Return: ReturnFloat},
	"Arg":               {Return: ReturnFloat},
	"Conj":              {Return: ReturnComplex},
	"Pi":                {Return: ReturnFloat},
	"E":                 {Return: ReturnFloat},
	"Float":             {Parameters: []ParameterConstraint{ParameterIntScalar}, Return: ReturnFloat},
	"FloorToInt":        {Parameters: []ParameterConstraint{ParameterFloatScalar}, Return: ReturnInt},
	"CeilToInt":         {Parameters: []ParameterConstraint{ParameterFloatScalar}, Return: ReturnInt},
	"RoundToInt":        {Parameters: []ParameterConstraint{ParameterFloatScalar}, Return: ReturnInt},
	"BaseValue":         {Parameters: []ParameterConstraint{ParameterFloatScalar}, Return: ReturnFloat},
	"BaseUnit":          {Parameters: []ParameterConstraint{ParameterFloatScalar}, Return: ReturnFloat},
	"FormatFloat":       {Parameters: []ParameterConstraint{ParameterFloatScalar, ParameterIntScalar}, Return: ReturnString},
	"Contains":          {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"StartsWith":        {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"EndsWith":          {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"Trim":              {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
	"Lower":             {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
	"Upper":             {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
	"Join":              {Parameters: []ParameterConstraint{ParameterStringArray, ParameterString}, Return: ReturnString},
	"ArrayCrossSection": {Parameters: []ParameterConstraint{ParameterArray, ParameterRange}, Return: ReturnFirstType},
	"ArrayWhere":        {Parameters: []ParameterConstraint{ParameterArray, ParameterBoolArray}, Return: ReturnFirstType},
	"StringByteLength":  {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnInt},
	"StringRuneCount":   {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnInt},
	"StringConcat":      {Parameters: []ParameterConstraint{ParameterStringArray}, Return: ReturnString},
	"StringFrom":        {Return: ReturnString},
	"StringJoin":        {Parameters: []ParameterConstraint{ParameterStringArray, ParameterString}, Return: ReturnString},
	"StringReplaceAll":  {Parameters: []ParameterConstraint{ParameterString, ParameterString, ParameterString}, Return: ReturnString},
	"StringContains":    {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"StringStartsWith":  {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"StringEndsWith":    {Parameters: []ParameterConstraint{ParameterString, ParameterString}, Return: "Bool"},
	"StringTrim":        {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
	"StringSplitLines":  {Parameters: []ParameterConstraint{ParameterString}, Return: "String[]"},
	"StringEscapeJSON":  {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
	"StringQuoteJSON":   {Parameters: []ParameterConstraint{ParameterString}, Return: ReturnString},
}

var regularCallShapes = map[string]CallShape{
	"Len":               exactShape(1),
	"Append":            exactShape(2),
	"Abs":               exactShape(1),
	"Sqrt":              exactShape(1),
	"Sin":               exactShape(1),
	"Cos":               exactShape(1),
	"Tan":               exactShape(1),
	"Asin":              exactShape(1),
	"Acos":              exactShape(1),
	"Atan":              exactShape(1),
	"Atan2":             exactShape(2),
	"Exp":               exactShape(1),
	"Ln":                exactShape(1),
	"Pow":               exactShape(2),
	"Log10":             exactShape(1),
	"Sinh":              exactShape(1),
	"Cosh":              exactShape(1),
	"Tanh":              exactShape(1),
	"Complex":           exactShape(2),
	"ComplexPolar":      exactShape(2),
	"I":                 exactShape(0),
	"Real":              exactShape(1),
	"Imag":              exactShape(1),
	"Conj":              exactShape(1),
	"Arg":               exactShape(1),
	"Pi":                exactShape(0),
	"E":                 exactShape(0),
	"Float":             exactShape(1),
	"FloorToInt":        exactShape(1),
	"CeilToInt":         exactShape(1),
	"RoundToInt":        exactShape(1),
	"BaseValue":         exactShape(1),
	"BaseUnit":          exactShape(1),
	"FormatFloat":       exactShape(2),
	"Contains":          exactShape(2),
	"StartsWith":        exactShape(2),
	"EndsWith":          exactShape(2),
	"Trim":              exactShape(1),
	"Lower":             exactShape(1),
	"Upper":             exactShape(1),
	"Join":              exactShape(2),
	"ArrayCrossSection": exactShape(2),
	"ArrayWhere":        exactShape(2),
	"StringByteLength":  exactShape(1),
	"StringRuneCount":   exactShape(1),
	"StringConcat":      exactShape(1),
	"StringFrom": {
		Known: true, MinimumArguments: 1, MaximumArguments: 1,
		MinimumTypeArgs: 1, MaximumTypeArgs: 1,
	},
	"StringJoin":       exactShape(2),
	"StringReplaceAll": exactShape(3),
	"StringContains":   exactShape(2),
	"StringStartsWith": exactShape(2),
	"StringEndsWith":   exactShape(2),
	"StringTrim":       exactShape(1),
	"StringSplitLines": exactShape(1),
	"StringEscapeJSON": exactShape(1),
	"StringQuoteJSON":  exactShape(1),
}

var definitions = buildDefinitions()

func exactShape(arguments int) CallShape {
	return CallShape{Known: true, MinimumArguments: arguments, MaximumArguments: arguments}
}

func buildDefinitions() map[string]Definition {
	canonical := make(map[string]Definition, len(names))
	aliasToCanonical := make(map[string]string)
	for namespace, aliases := range namespaceAliases {
		for symbol, name := range aliases {
			aliasToCanonical[namespace+"."+symbol] = name
		}
	}

	for name := range names {
		canonicalName := name
		if resolved, ok := aliasToCanonical[name]; ok {
			canonicalName = resolved
		}
		definition := canonical[canonicalName]
		definition.Name = canonicalName
		canonical[canonicalName] = definition
	}
	for alias, canonicalName := range aliasToCanonical {
		definition := canonical[canonicalName]
		definition.Name = canonicalName
		definition.Aliases = append(definition.Aliases, alias)
		canonical[canonicalName] = definition
	}
	for name, definition := range canonical {
		definition.ReturnRule = ReturnSemanticHook
		if shape, ok := regularCallShapes[name]; ok {
			definition.CallShape = shape
			definition.Fallibility = FallibilityInfallible
			definition.Traits = []Trait{TraitPure, TraitRuntime, TraitInterpreted, TraitCompiled}
		} else {
			definition.Fallibility = FallibilityIrregular
			definition.Traits = []Trait{TraitRuntime}
		}
		if rule, ok := regularSemanticRules[name]; ok {
			definition.ParameterConstraints = append([]ParameterConstraint(nil), rule.Parameters...)
			definition.ReturnRule = rule.Return
		}
		sort.Strings(definition.Aliases)
		canonical[name] = definition
	}
	return canonical
}

// CanonicalName resolves compiler-owned namespace aliases to one definition.
// It does not canonicalize ordinary package functions that happen to lower to
// a builtin implementation.
func CanonicalName(name string) string {
	namespace, symbol, ok := strings.Cut(name, ".")
	if !ok {
		return name
	}
	if resolved, ok := ResolveNamespacedAlias(namespace, symbol); ok {
		return resolved
	}
	return name
}

func Lookup(name string) (Definition, bool) {
	definition, ok := definitions[CanonicalName(name)]
	return definition, ok
}

func Definitions() []Definition {
	result := make([]Definition, 0, len(definitions))
	for _, definition := range definitions {
		copyOfDefinition := definition
		copyOfDefinition.Aliases = append([]string(nil), definition.Aliases...)
		copyOfDefinition.ParameterConstraints = append([]ParameterConstraint(nil), definition.ParameterConstraints...)
		copyOfDefinition.Traits = append([]Trait(nil), definition.Traits...)
		result = append(result, copyOfDefinition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func ValidateCallShape(name string, argumentCount, typeArgumentCount int) error {
	definition, ok := Lookup(name)
	if !ok || !definition.CallShape.Known {
		return nil
	}
	shape := definition.CallShape
	if typeArgumentCount < shape.MinimumTypeArgs || typeArgumentCount > shape.MaximumTypeArgs {
		if shape.MinimumTypeArgs == 0 && shape.MaximumTypeArgs == 0 {
			return fmt.Errorf("function '%s' does not accept type arguments", name)
		}
		if shape.MinimumTypeArgs == shape.MaximumTypeArgs {
			return fmt.Errorf("function '%s' expects %d %s, got %d", name, shape.MinimumTypeArgs, plural(shape.MinimumTypeArgs, "type argument"), typeArgumentCount)
		}
		return fmt.Errorf("function '%s' expects %d to %d type arguments, got %d", name, shape.MinimumTypeArgs, shape.MaximumTypeArgs, typeArgumentCount)
	}
	return ValidateArgumentCount(name, argumentCount)
}

func ValidateArgumentCount(name string, argumentCount int) error {
	definition, ok := Lookup(name)
	if !ok || !definition.CallShape.Known {
		return nil
	}
	shape := definition.CallShape
	if argumentCount >= shape.MinimumArguments && argumentCount <= shape.MaximumArguments {
		return nil
	}
	if shape.MinimumArguments == shape.MaximumArguments {
		return fmt.Errorf("function '%s' expects %d %s, got %d", name, shape.MinimumArguments, plural(shape.MinimumArguments, "argument"), argumentCount)
	}
	return fmt.Errorf("function '%s' expects %d to %d arguments, got %d", name, shape.MinimumArguments, shape.MaximumArguments, argumentCount)
}

func plural(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}
