package conceptvulkan

import (
	"fmt"
	"strings"
)

type evt1Scope struct {
	parent  *evt1Scope
	values  map[string]evt1ValueBinding
	borrows []evt1RetainedBorrow
}

type evt1ValueBinding struct {
	t                EVT1Type
	mutable          bool
	comptime         bool
	hasValue         bool
	value            EVT1Value
	instanceAutomata string
	batchAutomata    string
	actuatorName     string
}

type evt1AccessPath struct {
	Root   string
	Fields []string
	Span   Span
}

type evt1RetainedBorrow struct {
	InstanceName string
	AutomataName string
	ContextName  string
	Path         evt1AccessPath
	Type         EVT1Type
	Span         Span
}

type evt1LValue struct {
	t          EVT1Type
	mutable    bool
	wholeValue bool
	path       evt1AccessPath
}

func newEVT1Scope(parent *evt1Scope) *evt1Scope {
	return &evt1Scope{parent: parent, values: map[string]evt1ValueBinding{}}
}

func (s *evt1Scope) declare(name string, binding evt1ValueBinding) {
	s.values[name] = binding
}

func (s *evt1Scope) addBorrow(binding evt1RetainedBorrow) {
	s.borrows = append(s.borrows, binding)
}

func (s *evt1Scope) lookup(name string) (evt1ValueBinding, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if t, ok := scope.values[name]; ok {
			return t, true
		}
	}
	return evt1ValueBinding{}, false
}

func (s *evt1Scope) activeBorrows() []evt1RetainedBorrow {
	var out []evt1RetainedBorrow
	for scope := s; scope != nil; scope = scope.parent {
		out = append(out, scope.borrows...)
	}
	return out
}

func (b evt1ValueBinding) isInstance() bool {
	return b.instanceAutomata != ""
}

func (b evt1ValueBinding) isBatch() bool {
	return b.batchAutomata != ""
}

func (b evt1ValueBinding) isActuatorLocal() bool {
	return b.actuatorName != ""
}

func validateEVT1Module(module EVT1Module) error {
	_, err := analyzeEVT1Module(module)
	return err
}

func analyzeEVT1Module(module EVT1Module) (*evt1Env, error) {
	env := newEVT1Env()
	typeNames := map[string]Span{}
	for _, enumDecl := range module.Enums {
		if enumDecl.Name == evt1AutomataDispatchOutcomeTypeName || enumDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", enumDecl.Name), enumDecl.Span)
		}
		if _, exists := env.enums[enumDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4101", fmt.Sprintf("duplicate enum declaration %s", enumDecl.Name), enumDecl.Span)
		}
		if other, exists := typeNames[enumDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4101", fmt.Sprintf("duplicate type declaration %s", enumDecl.Name), other)
		}
		typeNames[enumDecl.Name] = enumDecl.Span
		env.enums[enumDecl.Name] = enumDecl
	}
	for _, structDecl := range module.Structs {
		if structDecl.Name == evt1AutomataDispatchOutcomeTypeName || structDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", structDecl.Name), structDecl.Span)
		}
		if _, exists := env.structs[structDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4122", fmt.Sprintf("duplicate struct declaration %s", structDecl.Name), structDecl.Span)
		}
		if _, exists := typeNames[structDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4122", fmt.Sprintf("duplicate type declaration %s", structDecl.Name), structDecl.Span)
		}
		typeNames[structDecl.Name] = structDecl.Span
		env.structs[structDecl.Name] = structDecl
	}
	for _, effectDecl := range module.Effects {
		if effectDecl.Name == "dispatch" || effectDecl.Name == "actuate" || effectDecl.Name == "discard" {
			return nil, evt1Diagnostic("CV4268", fmt.Sprintf("%s is a compiler-owned operation name and cannot be redeclared", effectDecl.Name), effectDecl.Span)
		}
		if _, exists := env.effects[effectDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4298", fmt.Sprintf("duplicate effect declaration %s", effectDecl.Name), effectDecl.Span)
		}
		if _, exists := env.automata[effectDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4298", fmt.Sprintf("duplicate declaration %s", effectDecl.Name), effectDecl.Span)
		}
		env.effects[effectDecl.Name] = effectDecl
		env.effectOrder = append(env.effectOrder, effectDecl.Name)
	}
	for _, actuatorDecl := range module.Actuators {
		if actuatorDecl.Name == evt1AutomataDispatchOutcomeTypeName || actuatorDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", actuatorDecl.Name), actuatorDecl.Span)
		}
		if _, exists := env.actuators[actuatorDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4313", fmt.Sprintf("duplicate actuator declaration %s", actuatorDecl.Name), actuatorDecl.Span)
		}
		if _, exists := typeNames[actuatorDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4313", fmt.Sprintf("duplicate declaration %s", actuatorDecl.Name), actuatorDecl.Span)
		}
		env.actuators[actuatorDecl.Name] = actuatorDecl
	}
	for _, automataDecl := range module.Automata {
		if automataDecl.Name == evt1AutomataDispatchOutcomeTypeName || automataDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", automataDecl.Name), automataDecl.Span)
		}
		if _, exists := env.automata[automataDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4240", fmt.Sprintf("duplicate automata declaration %s", automataDecl.Name), automataDecl.Span)
		}
		if _, exists := typeNames[automataDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4240", fmt.Sprintf("duplicate declaration %s", automataDecl.Name), automataDecl.Span)
		}
		env.automata[automataDecl.Name] = automataDecl
	}
	for _, conceptDecl := range module.Concepts {
		if conceptDecl.Name == evt1AutomataDispatchOutcomeTypeName || conceptDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", conceptDecl.Name), conceptDecl.Span)
		}
		if _, exists := env.concepts[conceptDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4146", fmt.Sprintf("duplicate concept declaration %s", conceptDecl.Name), conceptDecl.Span)
		}
		if _, exists := env.automata[conceptDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4240", fmt.Sprintf("duplicate declaration %s", conceptDecl.Name), conceptDecl.Span)
		}
		env.concepts[conceptDecl.Name] = conceptDecl
	}
	for _, templateDecl := range module.Templates {
		if templateDecl.Name == evt1AutomataDispatchOutcomeTypeName || templateDecl.Name == evt1ActuationOutcomeTypeName {
			return nil, evt1Diagnostic("CV4267", fmt.Sprintf("%s is a compiler-owned runtime type and cannot be redeclared", templateDecl.Name), templateDecl.Span)
		}
		if templateDecl.Name == "dispatch" || templateDecl.Name == "actuate" || templateDecl.Name == "discard" {
			return nil, evt1Diagnostic("CV4268", fmt.Sprintf("%s is a compiler-owned operation name and cannot be redeclared", templateDecl.Name), templateDecl.Span)
		}
		if env.effects[templateDecl.Name].Name != "" {
			return nil, evt1Diagnostic("CV4298", fmt.Sprintf("duplicate declaration %s", templateDecl.Name), templateDecl.Span)
		}
		if _, exists := env.templates[templateDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4168", fmt.Sprintf("duplicate template declaration %s", templateDecl.Name), templateDecl.Span)
		}
		if _, exists := env.automata[templateDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4240", fmt.Sprintf("duplicate declaration %s", templateDecl.Name), templateDecl.Span)
		}
		env.templates[templateDecl.Name] = templateDecl
	}
	for _, decl := range module.ComptimeDecls {
		if _, exists := env.comptimeDecls[decl.Name]; exists {
			return nil, evt1Diagnostic("CV4203", fmt.Sprintf("duplicate comptime declaration %s", decl.Name), decl.Span)
		}
		if len(env.functions[decl.Name]) > 0 || env.templates[decl.Name].Name != "" || env.comptimeFunctions[decl.Name].Name != "" || env.automata[decl.Name].Name != "" || env.effects[decl.Name].Name != "" || env.actuators[decl.Name].Name != "" {
			return nil, evt1Diagnostic("CV4203", fmt.Sprintf("comptime declaration %s conflicts with an existing symbol", decl.Name), decl.Span)
		}
		env.comptimeDecls[decl.Name] = decl
	}
	for _, fn := range module.Functions {
		if fn.Name == "dispatch" || fn.Name == "actuate" || fn.Name == "discard" {
			return nil, evt1Diagnostic("CV4268", fmt.Sprintf("%s is a compiler-owned operation name and cannot be redeclared", fn.Name), fn.Span)
		}
		if env.effects[fn.Name].Name != "" {
			return nil, evt1Diagnostic("CV4298", fmt.Sprintf("duplicate declaration %s", fn.Name), fn.Span)
		}
		if env.automata[fn.Name].Name != "" || env.actuators[fn.Name].Name != "" {
			return nil, evt1Diagnostic("CV4240", fmt.Sprintf("duplicate declaration %s", fn.Name), fn.Span)
		}
		for _, existing := range env.functions[fn.Name] {
			if evt1FunctionParamSignature(existing) == evt1FunctionParamSignature(fn) {
				return nil, evt1Diagnostic("CV4021", fmt.Sprintf("duplicate function declaration %s", fn.Name), fn.Span)
			}
		}
		env.functions[fn.Name] = append(env.functions[fn.Name], fn)
	}
	for _, fn := range module.ComptimeFns {
		if fn.Name == "dispatch" || fn.Name == "actuate" || fn.Name == "discard" {
			return nil, evt1Diagnostic("CV4268", fmt.Sprintf("%s is a compiler-owned operation name and cannot be redeclared", fn.Name), fn.Span)
		}
		if len(env.functions[fn.Name]) > 0 || env.templates[fn.Name].Name != "" || env.comptimeDecls[fn.Name].Name != "" || env.comptimeFunctions[fn.Name].Name != "" || env.automata[fn.Name].Name != "" || env.effects[fn.Name].Name != "" || env.actuators[fn.Name].Name != "" {
			return nil, evt1Diagnostic("CV4214", fmt.Sprintf("comptime function %s conflicts with an existing symbol", fn.Name), fn.Span)
		}
		env.comptimeFunctions[fn.Name] = fn
	}
	for _, structDecl := range module.Structs {
		fields := map[string]EVT1Type{}
		if len(structDecl.Fields) == 0 {
			return nil, evt1Diagnostic("CV4123", fmt.Sprintf("empty struct %s is not supported", structDecl.Name), structDecl.Span)
		}
		for _, field := range structDecl.Fields {
			if _, exists := fields[field.Name]; exists {
				return nil, evt1Diagnostic("CV4124", fmt.Sprintf("duplicate field %s.%s", structDecl.Name, field.Name), field.Span)
			}
			if err := validateKnownType(env, field.Type, field.Span, "", false); err != nil {
				return nil, err
			}
			resolved, err := evt1ResolveType(env, nil, field.Type.valueType())
			if err != nil {
				return nil, err
			}
			fields[field.Name] = resolved
		}
		env.fieldSets[structDecl.Name] = fields
	}
	for i, enumDecl := range module.Enums {
		seen := map[string]bool{}
		for j, variant := range enumDecl.Variants {
			if seen[variant.Name] {
				return nil, evt1Diagnostic("CV4100", fmt.Sprintf("duplicate variant %s::%s", enumDecl.Name, variant.Name), variant.Span)
			}
			seen[variant.Name] = true
			module.Enums[i].Variants[j].Tag = j
			for _, field := range variant.Payload {
				if err := validateKnownType(env, field.Type, field.Span, "", false); err != nil {
					return nil, err
				}
			}
		}
		env.enums[enumDecl.Name] = module.Enums[i]
	}
	if err := validateValueLayoutCycles(env); err != nil {
		return nil, err
	}
	for _, structDecl := range module.Structs {
		for _, field := range structDecl.Fields {
			if !field.Type.isBorrowLike() && evt1IsImmovableValueType(env, field.Type) {
				return nil, evt1Diagnostic("CV4138", fmt.Sprintf("struct %s cannot embed immovable field %s", structDecl.Name, field.Type.String()), field.Span)
			}
		}
	}
	for _, enumDecl := range module.Enums {
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if evt1IsImmovableValueType(env, field.Type) {
					return nil, evt1Diagnostic("CV4139", fmt.Sprintf("enum payload %s::%s cannot contain immovable type %s", enumDecl.Name, variant.Name, field.Type.String()), field.Span)
				}
				if !evt1TypeCopyable(env, field.Type) {
					return nil, evt1Diagnostic("CV4133", fmt.Sprintf("enum payload %s::%s cannot copy non-copyable type %s", enumDecl.Name, variant.Name, field.Type.String()), field.Span)
				}
			}
		}
	}
	for i, effectDecl := range module.Effects {
		for j, param := range effectDecl.Params {
			if err := validateKnownType(env, param.Type, param.Span, "", false); err != nil {
				return nil, err
			}
			resolved, err := evt1ResolveType(env, nil, param.Type.valueType())
			if err != nil {
				return nil, err
			}
			if err := validateEffectPayloadType(env, resolved, param.Span, effectDecl.Name+"."+param.Name); err != nil {
				return nil, err
			}
			module.Effects[i].Params[j].Type = resolved
		}
		env.effects[effectDecl.Name] = module.Effects[i]
	}
	if err := evt1ValidateAutomataDecls(env, module); err != nil {
		return nil, err
	}
	if err := evt1ValidateActuatorDecls(env, module); err != nil {
		return nil, err
	}
	for _, conceptDecl := range module.Concepts {
		for _, req := range conceptDecl.Requirements {
			switch r := req.(type) {
			case *EVT1OperationRequirement:
				if err := validateKnownType(env, r.ReturnType, r.Span, conceptDecl.TypeParam, false); err != nil {
					return nil, err
				}
				for _, param := range r.Params {
					if err := validateKnownType(env, param.Type, param.Span, conceptDecl.TypeParam, false); err != nil {
						return nil, err
					}
				}
			case *EVT1PrerequisiteRequirement:
				if _, ok := env.concepts[r.ConceptName]; !ok {
					return nil, evt1Diagnostic("CV4152", fmt.Sprintf("unknown prerequisite concept %s", r.ConceptName), r.Span)
				}
				if r.TypeArg.Kind != EVT1TypeConceptParam || r.TypeArg.Name != conceptDecl.TypeParam {
					return nil, evt1Diagnostic("CV4152", fmt.Sprintf("prerequisite %s must use the concept parameter %s", r.ConceptName, conceptDecl.TypeParam), r.Span)
				}
			default:
				return nil, evt1Diagnostic("CV4147", "unsupported concept requirement", req.requirementSpan())
			}
		}
	}
	if err := validateConceptCycles(env); err != nil {
		return nil, err
	}
	for _, templateDecl := range module.Templates {
		if err := validateTemplateSignature(env, templateDecl); err != nil {
			return nil, err
		}
		info, err := buildTemplateInfo(env, templateDecl)
		if err != nil {
			return nil, err
		}
		env.templateInfos[templateDecl.Name] = info
	}
	for _, fn := range module.Functions {
		if err := validateFunctionSignature(env, fn); err != nil {
			return nil, err
		}
	}
	for _, fn := range module.ComptimeFns {
		if err := validateFunctionSignature(env, fn); err != nil {
			return nil, err
		}
		if fn.Body == nil {
			return nil, evt1Diagnostic("CV4215", fmt.Sprintf("comptime function %s requires a body", fn.Name), fn.Span)
		}
		if fn.ReturnType.Name != "void" && !evt1IsComptimeType(env, fn.ReturnType) {
			return nil, evt1Diagnostic("CV4216", fmt.Sprintf("comptime return type %s is not supported", fn.ReturnType.String()), fn.ReturnType.Span)
		}
		for _, param := range fn.Params {
			if !evt1IsComptimeType(env, param.Type) {
				return nil, evt1Diagnostic("CV4216", fmt.Sprintf("comptime parameter type %s is not supported", param.Type.String()), param.Span)
			}
		}
	}
	for _, decl := range module.ComptimeDecls {
		if err := validateKnownType(env, decl.Type, decl.Span, "", false); err != nil {
			return nil, err
		}
		if !evt1IsComptimeType(env, decl.Type) {
			return nil, evt1Diagnostic("CV4216", fmt.Sprintf("comptime declaration type %s is not supported", decl.Type.String()), decl.Span)
		}
	}
	for _, templateDecl := range module.Templates {
		scope := evt1ModuleScope(env)
		resolvedReturn, err := evt1ResolveType(env, nil, templateDecl.ReturnType)
		if err != nil {
			return nil, err
		}
		for _, param := range templateDecl.Params {
			resolvedParam, err := evt1ResolveType(env, nil, param.Type)
			if err != nil {
				return nil, err
			}
			scope.declare(param.Name, evt1ValueBinding{
				t:       resolvedParam,
				mutable: !(param.Type.isBorrowLike() && param.Type.Const),
			})
		}
		if err := validateBlock(env, scope, resolvedReturn, *templateDecl.Body, env.templateInfos[templateDecl.Name], false); err != nil {
			return nil, err
		}
	}
	for _, fn := range module.Functions {
		if fn.Body == nil {
			continue
		}
		scope := evt1ModuleScope(env)
		resolvedReturn, err := evt1ResolveType(env, nil, fn.ReturnType)
		if err != nil {
			return nil, err
		}
		for _, param := range fn.Params {
			resolvedParam, err := evt1ResolveType(env, nil, param.Type)
			if err != nil {
				return nil, err
			}
			scope.declare(param.Name, evt1ValueBinding{
				t:       resolvedParam,
				mutable: !(param.Type.isBorrowLike() && param.Type.Const),
			})
		}
		collectEscapedArmBindings(fn.Body, env)
		if err := validateBlock(env, scope, resolvedReturn, *fn.Body, nil, false); err != nil {
			return nil, err
		}
	}
	for _, fn := range module.ComptimeFns {
		scope := evt1ModuleScope(env)
		resolvedReturn, err := evt1ResolveType(env, nil, fn.ReturnType)
		if err != nil {
			return nil, err
		}
		for _, param := range fn.Params {
			resolvedParam, err := evt1ResolveType(env, nil, param.Type)
			if err != nil {
				return nil, err
			}
			scope.declare(param.Name, evt1ValueBinding{
				t:        resolvedParam,
				mutable:  true,
				comptime: true,
			})
		}
		collectEscapedArmBindings(fn.Body, env)
		if err := validateBlock(env, scope, resolvedReturn, *fn.Body, nil, true); err != nil {
			return nil, err
		}
	}
	if err := validateComptimeFunctionCycles(module, env); err != nil {
		return nil, err
	}
	for _, assertion := range module.Assertions {
		if _, ok := env.concepts[assertion.ConceptName]; !ok {
			return nil, evt1Diagnostic("CV4151", fmt.Sprintf("unknown concept %s", assertion.ConceptName), assertion.Span)
		}
		if err := validateKnownType(env, assertion.ConcreteType, assertion.Span, "", false); err != nil {
			return nil, err
		}
		if err := checkConceptSatisfaction(env, assertion.ConceptName, assertion.ConcreteType, nil, assertion.Span); err != nil {
			return nil, err
		}
	}
	if err := evt1EvaluateModuleComptime(env, module); err != nil {
		return nil, err
	}
	return env, nil
}

func evt1ModuleScope(env *evt1Env) *evt1Scope {
	scope := newEVT1Scope(nil)
	for _, decl := range env.comptimeDecls {
		resolved := evt1CanonicalType(env, decl.Type)
		if next, err := evt1ResolveType(env, nil, resolved); err == nil {
			resolved = next
		}
		scope.declare(decl.Name, evt1ValueBinding{
			t:        resolved,
			mutable:  false,
			comptime: true,
		})
	}
	return scope
}

func validateComptimeFunctionCycles(module EVT1Module, env *evt1Env) error {
	graph := map[string][]string{}
	for _, fn := range module.ComptimeFns {
		if fn.Body == nil {
			continue
		}
		seen := map[string]bool{}
		for _, callee := range evt1CollectComptimeCallsFromBlock(*fn.Body, env) {
			if !seen[callee] {
				graph[fn.Name] = append(graph[fn.Name], callee)
				seen[callee] = true
			}
		}
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(name string, path []string) error
	dfs = func(name string, path []string) error {
		if visiting[name] {
			cycle := append(path, name)
			return evt1Diagnostic("CV4217", "comptime recursion is not allowed: "+strings.Join(cycle, " -> "), env.comptimeFunctions[name].Span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		for _, callee := range graph[name] {
			if err := dfs(callee, append(path, name)); err != nil {
				return err
			}
		}
		visiting[name] = false
		return nil
	}
	for _, fn := range module.ComptimeFns {
		if err := dfs(fn.Name, nil); err != nil {
			return err
		}
	}
	return nil
}

func evt1CollectComptimeCallsFromBlock(block EVT1Block, env *evt1Env) []string {
	var out []string
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Value, env)...)
		case *EVT1InstanceDecl:
			continue
		case *EVT1AssignStmt:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Target, env)...)
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Value, env)...)
		case *EVT1ReturnStmt:
			if s.Value != nil {
				out = append(out, evt1CollectComptimeCallsFromExpr(s.Value, env)...)
			}
		case *EVT1ExprStmt:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Value, env)...)
		case *EVT1StaticAssertStmt:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Condition, env)...)
			if s.Message != nil {
				out = append(out, evt1CollectComptimeCallsFromExpr(s.Message, env)...)
			}
		case *EVT1MatchStmt:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Subject, env)...)
			for _, arm := range s.Arms {
				out = append(out, evt1CollectComptimeCallsFromBlock(arm.Block, env)...)
			}
		case *EVT1WhileStmt:
			out = append(out, evt1CollectComptimeCallsFromExpr(s.Condition, env)...)
			if s.Bound != nil {
				out = append(out, evt1CollectComptimeCallsFromExpr(s.Bound, env)...)
			}
			out = append(out, evt1CollectComptimeCallsFromBlock(s.Body, env)...)
		case *EVT1Block:
			out = append(out, evt1CollectComptimeCallsFromBlock(*s, env)...)
		}
	}
	return out
}

func evt1CollectComptimeCallsFromExpr(expr EVT1Expr, env *evt1Env) []string {
	switch e := expr.(type) {
	case *EVT1ParenExpr:
		return evt1CollectComptimeCallsFromExpr(e.Value, env)
	case *EVT1UnaryExpr:
		return evt1CollectComptimeCallsFromExpr(e.Value, env)
	case *EVT1FieldExpr:
		return evt1CollectComptimeCallsFromExpr(e.Receiver, env)
	case *EVT1IndexExpr:
		return append(evt1CollectComptimeCallsFromExpr(e.Base, env), evt1CollectComptimeCallsFromExpr(e.Index, env)...)
	case *EVT1BinaryExpr:
		return append(evt1CollectComptimeCallsFromExpr(e.Left, env), evt1CollectComptimeCallsFromExpr(e.Right, env)...)
	case *EVT1CallExpr:
		var out []string
		if _, ok := env.comptimeFunctions[e.Callee]; ok {
			out = append(out, e.Callee)
		}
		for _, arg := range e.Args {
			out = append(out, evt1CollectComptimeCallsFromExpr(arg, env)...)
		}
		return out
	case *EVT1DispatchExpr:
		return evt1CollectComptimeCallsFromExpr(e.Signal, env)
	case *EVT1TemplateCallExpr:
		var out []string
		for _, arg := range e.Args {
			out = append(out, evt1CollectComptimeCallsFromExpr(arg, env)...)
		}
		return out
	case *EVT1ConstructExpr:
		var out []string
		for _, arg := range e.Args {
			out = append(out, evt1CollectComptimeCallsFromExpr(arg, env)...)
		}
		return out
	case *EVT1StructConstructExpr:
		var out []string
		for _, arg := range e.Args {
			out = append(out, evt1CollectComptimeCallsFromExpr(arg, env)...)
		}
		return out
	case *EVT1ArrayLiteralExpr:
		var out []string
		for _, arg := range e.Elements {
			out = append(out, evt1CollectComptimeCallsFromExpr(arg, env)...)
		}
		return out
	case *EVT1MatchExpr:
		out := evt1CollectComptimeCallsFromExpr(e.Subject, env)
		for _, arm := range e.Arms {
			out = append(out, evt1CollectComptimeCallsFromExpr(arm.Value, env)...)
		}
		return out
	case *EVT1IfExpr:
		out := evt1CollectComptimeCallsFromExpr(e.Condition, env)
		out = append(out, evt1CollectComptimeCallsFromExpr(e.Then, env)...)
		out = append(out, evt1CollectComptimeCallsFromExpr(e.Else, env)...)
		return out
	default:
		return nil
	}
}

func validateFunctionSignature(env *evt1Env, fn EVT1FunctionDecl) error {
	if err := validateKnownType(env, fn.ReturnType, fn.Span, "", false); err != nil {
		return err
	}
	if !fn.Comptime && evt1TypeContainsArray(env, fn.ReturnType) {
		return evt1Diagnostic("CV4228", fmt.Sprintf("runtime function return type %s cannot contain fixed compile-time arrays", fn.ReturnType.String()), fn.Span)
	}
	if err := validateByValueBoundary(env, fn.ReturnType, fn.Span, "return"); err != nil {
		return err
	}
	for _, param := range fn.Params {
		if err := validateKnownType(env, param.Type, param.Span, "", false); err != nil {
			return err
		}
		if !fn.Comptime && evt1TypeContainsArray(env, param.Type) {
			return evt1Diagnostic("CV4229", fmt.Sprintf("runtime function parameter type %s cannot contain fixed compile-time arrays", param.Type.String()), param.Span)
		}
		if err := validateByValueBoundary(env, param.Type, param.Span, "parameter"); err != nil {
			return err
		}
	}
	return nil
}

func validateTemplateSignature(env *evt1Env, templateDecl EVT1TemplateDecl) error {
	if _, ok := env.concepts[templateDecl.Constraint.ConceptName]; !ok {
		return evt1Diagnostic("CV4169", fmt.Sprintf("unknown concept %s in template constraint", templateDecl.Constraint.ConceptName), templateDecl.Constraint.Span)
	}
	constraintConcept := env.concepts[templateDecl.Constraint.ConceptName]
	if templateDecl.Constraint.TypeArg.Kind != EVT1TypeConceptParam || templateDecl.Constraint.TypeArg.Name != templateDecl.TypeParam {
		return evt1Diagnostic("CV4170", fmt.Sprintf("template constraint %s must apply to template parameter %s", templateDecl.Constraint.ConceptName, templateDecl.TypeParam), templateDecl.Constraint.Span)
	}
	if err := validateKnownType(env, templateDecl.ReturnType, templateDecl.ReturnType.Span, templateDecl.TypeParam, false); err != nil {
		return err
	}
	if evt1TypeContainsArray(env, templateDecl.ReturnType) {
		return evt1Diagnostic("CV4228", fmt.Sprintf("runtime template return type %s cannot contain fixed compile-time arrays", templateDecl.ReturnType.String()), templateDecl.Span)
	}
	if err := validateTemplateByValueBoundary(env, templateDecl.ReturnType, templateDecl.Span, "return", templateDecl.TypeParam); err != nil {
		return err
	}
	if constraintConcept.TypeParam == "" {
		return evt1Diagnostic("CV4171", fmt.Sprintf("template constraint %s must be a named one-parameter concept", templateDecl.Constraint.ConceptName), templateDecl.Constraint.Span)
	}
	for _, param := range templateDecl.Params {
		if err := validateKnownType(env, param.Type, param.Span, templateDecl.TypeParam, false); err != nil {
			return err
		}
		if evt1TypeContainsArray(env, param.Type) {
			return evt1Diagnostic("CV4229", fmt.Sprintf("runtime template parameter type %s cannot contain fixed compile-time arrays", param.Type.String()), param.Span)
		}
		if err := validateTemplateByValueBoundary(env, param.Type, param.Span, "parameter", templateDecl.TypeParam); err != nil {
			return err
		}
	}
	return nil
}

func validateByValueBoundary(env *evt1Env, t EVT1Type, span Span, context string) error {
	if t.isBorrowLike() {
		return nil
	}
	if evt1IsImmovableValueType(env, t) {
		switch context {
		case "parameter":
			return evt1Diagnostic("CV4136", fmt.Sprintf("immovable type %s cannot be passed by value", t.String()), span)
		case "return":
			return evt1Diagnostic("CV4137", fmt.Sprintf("immovable type %s cannot be returned by value", t.String()), span)
		}
	}
	if !evt1TypeCopyable(env, t) {
		return evt1Diagnostic("CV4133", fmt.Sprintf("non-copyable type %s cannot cross a by-value %s boundary", t.String(), context), span)
	}
	return nil
}

func validateTemplateByValueBoundary(env *evt1Env, t EVT1Type, span Span, context, typeParam string) error {
	if evt1TypeDependsOnParam(t, typeParam) {
		return nil
	}
	return validateByValueBoundary(env, t, span, context)
}

func validateEffectPayloadType(env *evt1Env, t EVT1Type, span Span, label string) error {
	if t.PointerTo != nil || t.ArrayElem != nil || t.Ownership != "" || t.Const || t.Imported || t.Unsafe || len(t.TypeArgs) > 0 {
		return evt1Diagnostic("CV4303", fmt.Sprintf("effect payload %s must use a fixed immutable value type, got %s", label, t.String()), span)
	}
	switch t.Kind {
	case EVT1TypeBuiltin:
		switch t.Name {
		case "int", "bool", "uint64":
			return nil
		default:
			return evt1Diagnostic("CV4303", fmt.Sprintf("effect payload %s must use a fixed immutable value type, got %s", label, t.String()), span)
		}
	case EVT1TypeEnum:
		enumDecl := env.enums[t.Name]
		for _, variant := range enumDecl.Variants {
			if len(variant.Payload) > 0 {
				return evt1Diagnostic("CV4303", fmt.Sprintf("effect payload %s enum %s must use only nullary variants", label, t.Name), span)
			}
		}
		return nil
	case EVT1TypeStruct:
		structDecl := env.structs[t.Name]
		for _, field := range structDecl.Fields {
			fieldType, err := evt1ResolveType(env, nil, field.Type.valueType())
			if err != nil {
				return err
			}
			if err := validateEffectPayloadType(env, fieldType, field.Span, label+"."+field.Name); err != nil {
				return err
			}
		}
		return nil
	default:
		return evt1Diagnostic("CV4303", fmt.Sprintf("effect payload %s must use a fixed immutable value type, got %s", label, t.String()), span)
	}
}

func collectEscapedArmBindings(block *EVT1Block, env *evt1Env) {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1MatchStmt:
			for _, arm := range s.Arms {
				for _, binding := range arm.Pattern.Bindings {
					env.escapedArmBinding[binding] = arm.Pattern.Span
				}
				collectEscapedArmBindings(&arm.Block, env)
			}
		case *EVT1Block:
			collectEscapedArmBindings(s, env)
		}
	}
}

func validateBlock(env *evt1Env, scope *evt1Scope, returnType EVT1Type, block EVT1Block, templateInfo *evt1TemplateInfo, inComptimeFn bool) error {
	local := newEVT1Scope(scope)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if err := validateKnownType(env, s.Type, s.Span, typeParam, false); err != nil {
				return err
			}
			resolvedType, err := evt1ResolveType(env, local, s.Type)
			if err != nil {
				return err
			}
			if evt1TypeContainsArray(env, resolvedType) && !inComptimeFn && !s.Comptime {
				return evt1Diagnostic("CV4230", fmt.Sprintf("runtime local %s cannot use fixed compile-time array type %s", s.Name, resolvedType.String()), s.Span)
			}
			valueType, err := validateExprAgainstExpected(env, local, s.Value, resolvedType, templateInfo, inComptimeFn || s.Comptime)
			if err != nil {
				return err
			}
			if !evt1TypesCompatible(env, resolvedType, valueType, typeParam) {
				return evt1Diagnostic("CV4106", fmt.Sprintf("constructor or initializer for %s expected %s but got %s", s.Name, resolvedType.String(), valueType.String()), s.Value.exprSpan())
			}
			if !s.Comptime && !evt1CanDirectInitialize(env, resolvedType, s.Value) && !evt1TypeCopyable(env, resolvedType) && !evt1TypeDependsOnParam(resolvedType, typeParam) {
				if evt1IsImmovableValueType(env, resolvedType) {
					return evt1Diagnostic("CV4134", fmt.Sprintf("immovable value %s must be constructed directly in final storage", resolvedType.String()), s.Span)
				}
				return evt1Diagnostic("CV4133", fmt.Sprintf("copy of non-copyable type %s is not allowed", resolvedType.String()), s.Span)
			}
			if s.Comptime {
				if !evt1IsComptimeType(env, resolvedType) {
					return evt1Diagnostic("CV4216", fmt.Sprintf("comptime declaration type %s is not supported", resolvedType.String()), s.Span)
				}
				value, err := evt1EvalExprTyped(newEVT1ComptimeState(env), evt1EvalScopeFromValidation(local, env), s.Value, &resolvedType)
				if err != nil {
					return err
				}
				local.declare(s.Name, evt1ValueBinding{t: resolvedType, mutable: false, comptime: true, hasValue: true, value: value})
				continue
			}
				local.declare(s.Name, evt1ValueBinding{t: resolvedType, mutable: true, comptime: inComptimeFn})
		case *EVT1EffectsDecl:
			if inComptimeFn {
				return evt1Diagnostic("CV4304", fmt.Sprintf("effects batch %s cannot be declared in comptime code", s.Name), s.Span)
			}
			info, ok := env.automataInfo[s.AutomataName]
			if !ok {
				return evt1Diagnostic("CV4300", fmt.Sprintf("effects declaration requires an automata name, got %s", s.AutomataName), s.Span)
			}
			local.declare(s.Name, evt1ValueBinding{
				mutable:       true,
				batchAutomata: info.Decl.Name,
			})
		case *EVT1ActuatorLocalDecl:
			if inComptimeFn {
				return evt1Diagnostic("CV4319", fmt.Sprintf("actuator local %s cannot be declared in comptime code", s.Name), s.Span)
			}
			info, ok := env.actuatorInfo[s.ActuatorName]
			if !ok {
				return evt1Diagnostic("CV4319", fmt.Sprintf("unknown actuator %s", s.ActuatorName), s.Span)
			}
			mechanismType, err := validateExpr(env, local, s.Mechanism, templateInfo, false)
			if err != nil {
				return err
			}
			if err := validateCallArgument(env, local, info.MechanismType, s.Mechanism, mechanismType, templateInfo); err != nil {
				return err
			}
			local.declare(s.Name, evt1ValueBinding{
				mutable:      false,
				actuatorName: info.Decl.Name,
			})
		case *EVT1InstanceDecl:
			if inComptimeFn {
				return evt1Diagnostic("CV4271", fmt.Sprintf("instance %s cannot be declared in comptime code", s.Name), s.Span)
			}
			info, ok := env.automataInfo[s.AutomataName]
			if !ok {
				return evt1Diagnostic("CV4270", fmt.Sprintf("instance declaration requires an automata name, got %s", s.AutomataName), s.Span)
			}
			if info.Decl.Context != nil {
				if s.Context == nil {
					return evt1Diagnostic("CV4283", fmt.Sprintf("instance %s of automata %s requires a context argument", s.Name, s.AutomataName), s.Span)
				}
				contextType := info.Decl.Context.Type
				argType, err := validateExpr(env, local, s.Context, templateInfo, false)
				if err != nil {
					return err
				}
				paramType := contextType
				paramType.Ownership = "borrow"
				paramType.Const = true
				if err := validateCallArgument(env, local, paramType, s.Context, argType, templateInfo); err != nil {
					return err
				}
				lvalue, err := validateAssignable(env, local, s.Context, templateInfo)
				if err != nil {
					return evt1Diagnostic("CV4285", fmt.Sprintf("context binding for instance %s requires an assignable access path", s.Name), s.Context.exprSpan())
				}
				local.addBorrow(evt1RetainedBorrow{
					InstanceName: s.Name,
					AutomataName: s.AutomataName,
					ContextName:  info.Decl.Context.Name,
					Path:         lvalue.path,
					Type:         contextType,
					Span:         s.Span,
				})
			} else if s.Context != nil {
				return evt1Diagnostic("CV4284", fmt.Sprintf("contextless automata %s does not accept a context argument", s.AutomataName), s.Context.exprSpan())
			}
			local.declare(s.Name, evt1ValueBinding{
				mutable:          true,
				instanceAutomata: info.Decl.Name,
			})
		case *EVT1ActuationDecl:
			info, ok := env.actuatorInfo[s.ActuatorName]
			if !ok {
				return evt1Diagnostic("CV4320", fmt.Sprintf("unknown actuator %s in actuation", s.ActuatorName), s.Span)
			}
			batchBinding, ok := local.lookup(s.BatchName)
			if !ok || !batchBinding.isBatch() {
				return evt1Diagnostic("CV4321", fmt.Sprintf("actuate requires a local effects batch, but %s is not one", s.BatchName), s.Span)
			}
			if batchBinding.batchAutomata != info.Automata.Decl.Name {
				return evt1Diagnostic("CV4321", fmt.Sprintf("actuate requires a batch for automata %s, but %s belongs to %s", info.Automata.Decl.Name, s.BatchName, batchBinding.batchAutomata), s.Span)
			}
			executorBinding, ok := local.lookup(s.ExecutorName)
			if !ok || !executorBinding.isActuatorLocal() {
				return evt1Diagnostic("CV4321", fmt.Sprintf("actuate requires a local actuator executor, but %s is not one", s.ExecutorName), s.Span)
			}
			if executorBinding.actuatorName != info.Decl.Name {
				return evt1Diagnostic("CV4321", fmt.Sprintf("actuation %s expects an executor for actuator %s, but %s belongs to %s", s.Name, info.Decl.Name, s.ExecutorName, executorBinding.actuatorName), s.Span)
			}
			local.declare(s.Name, evt1ValueBinding{
				t:       EVT1Type{Name: info.ResultTypeName, Kind: EVT1TypeStruct, Span: s.Span},
				mutable: false,
			})
		case *EVT1AssignStmt:
			target, err := validateAssignable(env, local, s.Target, templateInfo)
			if err != nil {
				return err
			}
			if !target.mutable {
				return evt1Diagnostic("CV4128", "mutation through a const access path is not allowed", s.Target.exprSpan())
			}
			if borrow, ok := evt1FindOverlappingBorrow(local.activeBorrows(), target.path); ok {
				return evt1Diagnostic("CV4291", fmt.Sprintf("assignment to %s overlaps retained immutable automata context for instance %s of %s", exprLabel(s.Target), borrow.InstanceName, borrow.AutomataName), s.Target.exprSpan())
			}
			valueType, err := validateExprAgainstExpected(env, local, s.Value, target.t, templateInfo, inComptimeFn)
			if err != nil {
				return err
			}
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if !evt1TypesCompatible(env, target.t, valueType, typeParam) {
				return evt1Diagnostic("CV4107", fmt.Sprintf("assignment to %s expected %s but got %s", exprLabel(s.Target), target.t.String(), valueType.String()), s.Value.exprSpan())
			}
			if !evt1TypeCopyable(env, target.t) && !evt1TypeDependsOnParam(target.t, typeParam) {
				if evt1IsImmovableValueType(env, target.t) && target.wholeValue {
					return evt1Diagnostic("CV4135", fmt.Sprintf("immovable value %s cannot be assigned as a whole", target.t.String()), s.Span)
				}
				return evt1Diagnostic("CV4133", fmt.Sprintf("assignment copies non-copyable type %s", target.t.String()), s.Span)
			}
		case *EVT1ReturnStmt:
			if s.Value == nil {
				if returnType.Name != "void" {
					return evt1Diagnostic("CV4022", fmt.Sprintf("return requires a %s value", returnType.String()), s.Span)
				}
				continue
			}
			valueType, err := validateExprAgainstExpected(env, local, s.Value, returnType, templateInfo, inComptimeFn)
			if err != nil {
				return err
			}
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if !evt1TypesCompatible(env, returnType, valueType, typeParam) {
				return evt1Diagnostic("CV4116", fmt.Sprintf("expression result type mismatch: expected %s but got %s", returnType.String(), valueType.String()), s.Value.exprSpan())
			}
		case *EVT1ExprStmt:
			if _, err := validateExpr(env, local, s.Value, templateInfo, inComptimeFn); err != nil {
				return err
			}
		case *EVT1StaticAssertStmt:
			if _, err := validateExpr(env, local, s.Condition, templateInfo, true); err != nil {
				return err
			}
			if s.Message != nil {
				if _, err := validateExpr(env, local, s.Message, templateInfo, true); err != nil {
					return err
				}
			}
			if err := evt1EvaluateStaticAssert(newEVT1ComptimeState(env), evt1EvalScopeFromValidation(local, env), &EVT1StaticAssert{Condition: s.Condition, Message: s.Message, Span: s.Span}); err != nil {
				return err
			}
		case *EVT1MatchStmt:
			if err := validateMatchStmt(env, local, *s, returnType, templateInfo, inComptimeFn); err != nil {
				return err
			}
		case *EVT1WhileStmt:
			if err := validateWhileStmt(env, local, *s, templateInfo, inComptimeFn); err != nil {
				return err
			}
		case *EVT1Block:
			if err := validateBlock(env, local, returnType, *s, templateInfo, inComptimeFn); err != nil {
				return err
			}
		default:
			return evt1Diagnostic("CV4023", "unsupported statement", stmt.statementSpan())
		}
	}
	return nil
}

func evt1EvalScopeFromValidation(scope *evt1Scope, env *evt1Env) *evt1EvalScope {
	var root *evt1EvalScope
	if scope.parent != nil {
		root = evt1EvalScopeFromValidation(scope.parent, env)
	} else {
		root = evt1SeedComptimeScope(env)
	}
	current := newEVT1EvalScope(root)
	for name, binding := range scope.values {
		if !binding.comptime {
			continue
		}
		if binding.hasValue {
			current.declare(name, evt1EvalBinding{value: binding.value, mutable: binding.mutable, comptime: true})
			continue
		}
		if value, ok := env.comptimeValues[name]; ok {
			current.declare(name, evt1EvalBinding{value: value, mutable: false, comptime: true})
		}
	}
	return current
}

func validateExprAgainstExpected(env *evt1Env, scope *evt1Scope, expr EVT1Expr, expected EVT1Type, templateInfo *evt1TemplateInfo, inComptimeFn bool) (EVT1Type, error) {
	if lit, ok := expr.(*EVT1ArrayLiteralExpr); ok && expected.ArrayElem != nil {
		return validateArrayLiteralExpr(env, scope, *lit, &expected, templateInfo, inComptimeFn)
	}
	return validateExpr(env, scope, expr, templateInfo, inComptimeFn)
}

func evt1ResolveType(env *evt1Env, scope *evt1Scope, t EVT1Type) (EVT1Type, error) {
	if t.PointerTo != nil {
		base, err := evt1ResolveType(env, scope, *t.PointerTo)
		if err != nil {
			return EVT1Type{}, err
		}
		t.PointerTo = &base
		return evt1CanonicalType(env, t), nil
	}
	for i := range t.TypeArgs {
		resolved, err := evt1ResolveType(env, scope, t.TypeArgs[i])
		if err != nil {
			return EVT1Type{}, err
		}
		t.TypeArgs[i] = resolved
	}
	if t.ArrayElem != nil {
		elem, err := evt1ResolveType(env, scope, *t.ArrayElem)
		if err != nil {
			return EVT1Type{}, err
		}
		length, err := evt1ResolveArrayLength(env, scope, t.ArrayLengthExpr, t.Span)
		if err != nil {
			return EVT1Type{}, err
		}
		resolved := EVT1Type{
			Name:        elem.String() + "[]",
			Kind:        EVT1TypeArray,
			ArrayElem:   &elem,
			ArrayLength: length,
			Span:        t.Span,
		}
		if depth := evt1ArrayDepth(resolved); depth > evt1ComptimeMaxArrayNesting {
			return EVT1Type{}, evt1Diagnostic("CV4223", fmt.Sprintf("array nesting depth %d exceeds limit %d", depth, evt1ComptimeMaxArrayNesting), t.Span)
		}
		if cells, err := evt1ComptimeTypeCellCount(env, resolved); err != nil {
			return EVT1Type{}, err
		} else if cells > evt1ComptimeMaxArrayCells {
			return EVT1Type{}, evt1Diagnostic("CV4224", fmt.Sprintf("array cell count %d exceeds limit %d", cells, evt1ComptimeMaxArrayCells), t.Span)
		}
		return resolved, nil
	}
	return evt1CanonicalType(env, t), nil
}

func evt1ResolveArrayLength(env *evt1Env, scope *evt1Scope, expr EVT1Expr, span Span) (int, error) {
	if expr == nil {
		return 0, evt1Diagnostic("CV4220", "fixed-array types require an explicit length expression", span)
	}
	evalScope := evt1SeedComptimeScope(env)
	if scope != nil {
		evalScope = evt1EvalScopeFromValidation(scope, env)
	}
	value, err := evt1EvalExpr(newEVT1ComptimeState(env), evalScope, expr)
	if err != nil {
		return 0, err
	}
	if value.Kind != EVT1ValueInt {
		return 0, evt1Diagnostic("CV4221", "fixed-array length must evaluate to int", expr.exprSpan())
	}
	if value.IntValue < 0 {
		return 0, evt1Diagnostic("CV4222", fmt.Sprintf("fixed-array length %d must be non-negative", value.IntValue), expr.exprSpan())
	}
	if value.IntValue > evt1ComptimeMaxArrayLength {
		return 0, evt1Diagnostic("CV4223", fmt.Sprintf("fixed-array length %d exceeds limit %d", value.IntValue, evt1ComptimeMaxArrayLength), expr.exprSpan())
	}
	return value.IntValue, nil
}

func evt1ArrayDepth(t EVT1Type) int {
	if t.ArrayElem == nil {
		return 0
	}
	return 1 + evt1ArrayDepth(*t.ArrayElem)
}

func evt1TypeContainsArray(env *evt1Env, t EVT1Type) bool {
	if t.ArrayElem != nil {
		return true
	}
	if t.PointerTo != nil && evt1TypeContainsArray(env, *t.PointerTo) {
		return true
	}
	for _, arg := range t.TypeArgs {
		if evt1TypeContainsArray(env, arg) {
			return true
		}
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		for _, field := range structDecl.Fields {
			if evt1TypeContainsArray(env, field.Type) {
				return true
			}
		}
	}
	if enumDecl, ok := env.enums[t.Name]; ok {
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if evt1TypeContainsArray(env, field.Type) {
					return true
				}
			}
		}
	}
	return false
}

func evt1ComptimeTypeCellCount(env *evt1Env, t EVT1Type) (int, error) {
	if t.ArrayElem != nil {
		elemCells, err := evt1ComptimeTypeCellCount(env, *t.ArrayElem)
		if err != nil {
			return 0, err
		}
		if elemCells == 0 || t.ArrayLength == 0 {
			return 0, nil
		}
		if elemCells > evt1ComptimeMaxArrayCells/t.ArrayLength {
			return 0, evt1Diagnostic("CV4224", fmt.Sprintf("array cell count overflows the limit %d", evt1ComptimeMaxArrayCells), t.Span)
		}
		return elemCells * t.ArrayLength, nil
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		total := 0
		for _, field := range structDecl.Fields {
			fieldType, err := evt1ResolveType(env, nil, field.Type)
			if err != nil {
				return 0, err
			}
			cells, err := evt1ComptimeTypeCellCount(env, fieldType)
			if err != nil {
				return 0, err
			}
			if cells < 1 {
				cells = 1
			}
			total += cells
		}
		return total, nil
	}
	return 1, nil
}

func evt1TypeEqualityAvailable(env *evt1Env, t EVT1Type) bool {
	if t.ArrayElem != nil {
		return evt1TypeEqualityAvailable(env, *t.ArrayElem)
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		return t.Name == "int" || t.Name == "bool" || t.Name == "string"
	}
	if _, ok := env.enums[t.Name]; ok {
		return true
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		for _, field := range structDecl.Fields {
			fieldType, err := evt1ResolveType(env, nil, field.Type)
			if err != nil || !evt1TypeEqualityAvailable(env, fieldType) {
				return false
			}
		}
		return true
	}
	return false
}

func validateArrayLiteralExpr(env *evt1Env, scope *evt1Scope, expr EVT1ArrayLiteralExpr, expected *EVT1Type, templateInfo *evt1TemplateInfo, inComptimeFn bool) (EVT1Type, error) {
	if len(expr.Elements) > evt1ComptimeMaxLiteralElements {
		return EVT1Type{}, evt1Diagnostic("CV4224", fmt.Sprintf("array literal element count %d exceeds limit %d", len(expr.Elements), evt1ComptimeMaxLiteralElements), expr.Span)
	}
	var arrayType EVT1Type
	if expected != nil && expected.ArrayElem != nil {
		arrayType = evt1CanonicalType(env, *expected)
		if len(expr.Elements) != arrayType.ArrayLength {
			return EVT1Type{}, evt1Diagnostic("CV4226", fmt.Sprintf("array literal expected %d elements but got %d", arrayType.ArrayLength, len(expr.Elements)), expr.Span)
		}
	} else if len(expr.Elements) == 0 {
		return EVT1Type{}, evt1Diagnostic("CV4225", "empty array literal requires an explicit fixed-array type", expr.Span)
	}
	for i, element := range expr.Elements {
		var elemExpected *EVT1Type
		if arrayType.ArrayElem != nil {
			elemExpected = arrayType.ArrayElem
		}
		elementType, err := validateExpr(env, scope, element, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if i == 0 && arrayType.ArrayElem == nil {
			elem := evt1CanonicalType(env, elementType.valueType())
			arrayType = EVT1Type{
				Name:        elem.String() + "[]",
				Kind:        EVT1TypeArray,
				ArrayElem:   &elem,
				ArrayLength: len(expr.Elements),
				Span:        expr.Span,
			}
		}
		if elemExpected != nil && !elemExpected.valueType().Equal(elementType.valueType()) {
			return EVT1Type{}, evt1Diagnostic("CV4227", fmt.Sprintf("array literal element %d expected %s but got %s", i+1, elemExpected.String(), elementType.String()), element.exprSpan())
		}
		if elemExpected == nil && arrayType.ArrayElem != nil && !arrayType.ArrayElem.valueType().Equal(elementType.valueType()) {
			return EVT1Type{}, evt1Diagnostic("CV4227", fmt.Sprintf("array literal element %d expected %s but got %s", i+1, arrayType.ArrayElem.String(), elementType.String()), element.exprSpan())
		}
	}
	if arrayType.ArrayElem == nil {
		return EVT1Type{}, evt1Diagnostic("CV4225", "empty array literal requires an explicit fixed-array type", expr.Span)
	}
	return arrayType, nil
}

func validateKnownType(env *evt1Env, t EVT1Type, span Span, conceptParam string, allowConceptApp bool) error {
	if t.PointerTo != nil {
		return validateKnownType(env, *t.PointerTo, span, conceptParam, allowConceptApp)
	}
	if t.ArrayElem != nil {
		if err := validateKnownType(env, *t.ArrayElem, span, conceptParam, allowConceptApp); err != nil {
			return err
		}
		_, err := evt1ResolveArrayLength(env, nil, t.ArrayLengthExpr, span)
		return err
	}
	if t.Kind == EVT1TypeConceptParam {
		if conceptParam != "" && t.Name == conceptParam {
			return nil
		}
		return evt1Diagnostic("CV4148", fmt.Sprintf("unknown concept parameter %s", t.Name), span)
	}
	if len(t.TypeArgs) > 0 {
		if t.Name == "Result" {
			if len(t.TypeArgs) != 2 {
				return evt1Diagnostic("CV4322", fmt.Sprintf("Result requires exactly two type arguments, got %d", len(t.TypeArgs)), span)
			}
			if err := validateKnownType(env, t.TypeArgs[0], span, conceptParam, false); err != nil {
				return err
			}
			if err := validateKnownType(env, t.TypeArgs[1], span, conceptParam, false); err != nil {
				return err
			}
			if t.TypeArgs[0].Name != "void" {
				return evt1Diagnostic("CV4322", fmt.Sprintf("EVT1 M4 supports only Result<void, Error>, got %s", t.String()), span)
			}
			if !evt1ActuatorErrorTypeAllowed(env, evt1CanonicalType(env, t.TypeArgs[1])) {
				return evt1Diagnostic("CV4322", fmt.Sprintf("Result error type must be a fixed copyable runtime value, got %s", t.TypeArgs[1].String()), span)
			}
			return nil
		}
		if _, ok := env.concepts[t.Name]; ok {
			if !allowConceptApp {
				return evt1Diagnostic("CV4164", fmt.Sprintf("concept %s cannot be used as a runtime type", t.String()), span)
			}
			if len(t.TypeArgs) != 1 {
				return evt1Diagnostic("CV4149", fmt.Sprintf("concept %s requires exactly one type argument", t.Name), span)
			}
			return validateKnownType(env, t.TypeArgs[0], span, conceptParam, false)
		}
		return evt1Diagnostic("CV4102", fmt.Sprintf("unknown type application %s", t.String()), span)
	}
	if _, ok := evt1BuiltinType(t.Name, span); ok {
		return nil
	}
	if _, ok := env.enums[t.Name]; ok {
		return nil
	}
	if _, ok := env.structs[t.Name]; ok {
		return nil
	}
	if _, ok := env.automata[t.Name]; ok {
		return evt1Diagnostic("CV4263", fmt.Sprintf("automata %s cannot be used as a runtime type", t.Name), span)
	}
	return evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum or type %s", t.Name), span)
}

func validateExpr(env *evt1Env, scope *evt1Scope, expr EVT1Expr, templateInfo *evt1TemplateInfo, inComptimeFn bool) (EVT1Type, error) {
	switch e := expr.(type) {
	case *EVT1IntLiteral:
		t, _ := evt1BuiltinType("int", e.Span)
		return t, nil
	case *EVT1StringLiteral:
		t, _ := evt1BuiltinType("string", e.Span)
		return t, nil
	case *EVT1BoolLiteral:
		t, _ := evt1BuiltinType("bool", e.Span)
		return t, nil
	case *EVT1ArrayLiteralExpr:
		return validateArrayLiteralExpr(env, scope, *e, nil, templateInfo, inComptimeFn)
	case *EVT1ParenExpr:
		return validateExpr(env, scope, e.Value, templateInfo, inComptimeFn)
	case *EVT1NameExpr:
		if binding, ok := scope.lookup(e.Name); ok {
			if binding.isInstance() {
				return EVT1Type{}, evt1Diagnostic("CV4272", fmt.Sprintf("instance %s of automata %s cannot be used as an ordinary value; use dispatch(%s, signal)", e.Name, binding.instanceAutomata, e.Name), e.Span)
			}
			if binding.isBatch() {
				return EVT1Type{}, evt1Diagnostic("CV4305", fmt.Sprintf("effects batch %s for automata %s cannot be used as an ordinary value; use dispatch(instance, signal, %s)", e.Name, binding.batchAutomata, e.Name), e.Span)
			}
			if binding.isActuatorLocal() {
				return EVT1Type{}, evt1Diagnostic("CV4319", fmt.Sprintf("actuator local %s of actuator %s cannot be used as an ordinary value; use actuation ... = actuate(batch, %s)", e.Name, binding.actuatorName, e.Name), e.Span)
			}
			return evt1CanonicalType(env, binding.t), nil
		}
		if _, ok := env.automata[e.Name]; ok {
			return EVT1Type{}, evt1Diagnostic("CV4263", fmt.Sprintf("automata %s cannot be used as a runtime expression", e.Name), e.Span)
		}
		if bindingSpan, ok := env.escapedArmBinding[e.Name]; ok {
			return EVT1Type{}, evt1Diagnostic("CV4114", fmt.Sprintf("payload binding %s is scoped to its match arm", e.Name), bindingSpan)
		}
		return EVT1Type{}, evt1Diagnostic("CV4024", fmt.Sprintf("unknown name %s", e.Name), e.Span)
	case *EVT1FieldExpr:
		receiverType, err := validateExpr(env, scope, e.Receiver, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if templateInfo != nil && evt1TypeDependsOnParam(receiverType, templateInfo.Decl.TypeParam) {
			return EVT1Type{}, evt1Diagnostic("CV4172", "dependent field access is not allowed in EVT1 M1B-B templates", e.Span)
		}
		fields, baseName, err := evt1FieldSet(env, receiverType)
		if err != nil {
			return EVT1Type{}, evt1Diagnostic("CV4025", err.Error(), e.Span)
		}
		fieldType, ok := fields[e.Field]
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4026", fmt.Sprintf("unknown field %s on %s", e.Field, baseName), e.Span)
		}
		return evt1CanonicalType(env, fieldType), nil
	case *EVT1CallExpr:
		if e.Callee == "discard" {
			if len(e.Args) != 1 {
				return EVT1Type{}, evt1Diagnostic("CV4323", fmt.Sprintf("discard requires exactly one batch argument, got %d", len(e.Args)), e.Span)
			}
			nameExpr, ok := e.Args[0].(*EVT1NameExpr)
			if !ok {
				return EVT1Type{}, evt1Diagnostic("CV4323", "discard requires a local effects batch name", e.Args[0].exprSpan())
			}
			binding, ok := scope.lookup(nameExpr.Name)
			if !ok || !binding.isBatch() {
				return EVT1Type{}, evt1Diagnostic("CV4323", fmt.Sprintf("discard requires a local effects batch, but %s is not one", nameExpr.Name), e.Args[0].exprSpan())
			}
			return EVT1Type{Name: "void", Kind: EVT1TypeBuiltin, Span: e.Span}, nil
		}
		if e.Callee == "Len" {
			if len(e.Args) != 1 {
				return EVT1Type{}, evt1Diagnostic("CV4234", fmt.Sprintf("Len expects exactly one argument, got %d", len(e.Args)), e.Span)
			}
			argType, err := validateExpr(env, scope, e.Args[0], templateInfo, inComptimeFn)
			if err != nil {
				return EVT1Type{}, err
			}
			if argType.ArrayElem == nil {
				return EVT1Type{}, evt1Diagnostic("CV4235", "Len requires a fixed compile-time array argument", e.Args[0].exprSpan())
			}
			out, _ := evt1BuiltinType("int", e.Span)
			return out, nil
		}
		if templateInfo != nil {
			return validateTemplateCallExpr(env, scope, *e, templateInfo)
		}
		if _, exists := env.comptimeFunctions[e.Callee]; exists && !inComptimeFn {
			return EVT1Type{}, evt1Diagnostic("CV4210", fmt.Sprintf("comptime function %s cannot be called from runtime code", e.Callee), e.Span)
		}
		argTypes := make([]EVT1Type, 0, len(e.Args))
		for _, arg := range e.Args {
			argType, err := validateExpr(env, scope, arg, templateInfo, inComptimeFn)
			if err != nil {
				return EVT1Type{}, err
			}
			argTypes = append(argTypes, argType)
		}
		if inComptimeFn {
			if comptimeFn, ok := env.comptimeFunctions[e.Callee]; ok {
				if len(comptimeFn.Params) != len(e.Args) {
					return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor or call payload count for %s: expected %d but got %d", e.Callee, len(comptimeFn.Params), len(e.Args)), e.Span)
				}
				for i, arg := range e.Args {
					if err := validateCallArgument(env, scope, comptimeFn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
						return EVT1Type{}, err
					}
				}
				return evt1CanonicalType(env, comptimeFn.ReturnType), nil
			}
		}
		fn, err := evt1ResolveOrdinaryCall(env, scope, e.Callee, e.Args, argTypes, templateInfo, e.Span)
		if err != nil {
			if _, exists := env.templates[e.Callee]; exists {
				return EVT1Type{}, evt1Diagnostic("CV4173", fmt.Sprintf("template call %s requires an explicit concrete type argument", e.Callee), e.Span)
			}
			return EVT1Type{}, err
		}
		for i, arg := range e.Args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				return EVT1Type{}, err
			}
		}
		return evt1CanonicalType(env, fn.ReturnType), nil
	case *EVT1DispatchExpr:
		if inComptimeFn {
			return EVT1Type{}, evt1Diagnostic("CV4275", "dispatch is not available during comptime evaluation", e.Span)
		}
		binding, ok := scope.lookup(e.InstanceName)
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4273", fmt.Sprintf("dispatch requires a local instance, but %s is unknown", e.InstanceName), e.Span)
		}
		if !binding.isInstance() {
			return EVT1Type{}, evt1Diagnostic("CV4273", fmt.Sprintf("dispatch requires a local instance as its first operand, but %s is not an instance", e.InstanceName), e.Span)
		}
		info := env.automataInfo[binding.instanceAutomata]
		expected := evt1CanonicalType(env, info.Decl.SignalType)
		signalType, err := validateExprAgainstExpected(env, scope, e.Signal, expected, templateInfo, false)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanonicalType(env, signalType.valueType()).Equal(evt1CanonicalType(env, expected.valueType())) {
			return EVT1Type{}, evt1Diagnostic("CV4274", fmt.Sprintf("dispatch(%s, ...) expects signal type %s but got %s", e.InstanceName, expected.String(), signalType.String()), e.Signal.exprSpan())
		}
		if len(info.EffectSet) > 0 {
			if e.BatchName == "" {
				return EVT1Type{}, evt1Diagnostic("CV4306", fmt.Sprintf("effectful automata %s requires dispatch(%s, signal, batch)", info.Decl.Name, e.InstanceName), e.Span)
			}
			batchBinding, ok := scope.lookup(e.BatchName)
			if !ok || !batchBinding.isBatch() {
				return EVT1Type{}, evt1Diagnostic("CV4307", fmt.Sprintf("dispatch(%s, ...) requires a local effects batch as its third operand, but %s is not one", e.InstanceName, e.BatchName), e.Span)
			}
			if batchBinding.batchAutomata != info.Decl.Name {
				return EVT1Type{}, evt1Diagnostic("CV4308", fmt.Sprintf("dispatch(%s, ...) requires an effects batch for automata %s, but %s belongs to %s", e.InstanceName, info.Decl.Name, e.BatchName, batchBinding.batchAutomata), e.Span)
			}
		} else if e.BatchName != "" {
			return EVT1Type{}, evt1Diagnostic("CV4309", fmt.Sprintf("effect-free automata %s does not accept a third dispatch operand", info.Decl.Name), e.Span)
		}
		return EVT1Type{Name: evt1AutomataDispatchOutcomeTypeName, Kind: EVT1TypeEnum, Span: e.Span}, nil
	case *EVT1TemplateCallExpr:
		if inComptimeFn {
			return EVT1Type{}, evt1Diagnostic("CV4201", "templates are not available during comptime evaluation", e.Span)
		}
		if templateInfo != nil {
			return EVT1Type{}, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", e.Span)
		}
		instance, err := instantiateTemplate(env, e.Callee, e.TypeArg, e.Span)
		if err != nil {
			return EVT1Type{}, err
		}
		if len(instance.Function.Params) != len(e.Args) {
			return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor or call payload count for %s: expected %d but got %d", e.Callee, len(instance.Function.Params), len(e.Args)), e.Span)
		}
		for i, arg := range e.Args {
			argType, err := validateExpr(env, scope, arg, nil, false)
			if err != nil {
				return EVT1Type{}, err
			}
			if err := validateCallArgument(env, scope, instance.Function.Params[i].Type, arg, argType, nil); err != nil {
				return EVT1Type{}, err
			}
		}
		instance.InvocationSpans = append(instance.InvocationSpans, e.Span)
		return evt1CanonicalType(env, instance.Function.ReturnType), nil
	case *EVT1IndexExpr:
		baseType, err := validateExpr(env, scope, e.Base, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if baseType.ArrayElem == nil {
			return EVT1Type{}, evt1Diagnostic("CV4231", fmt.Sprintf("index target %s is not a fixed compile-time array", baseType.String()), e.Base.exprSpan())
		}
		indexType, err := validateExpr(env, scope, e.Index, templateInfo, true)
		if err != nil {
			return EVT1Type{}, err
		}
		if indexType.Name != "int" {
			return EVT1Type{}, evt1Diagnostic("CV4232", "array index must be int", e.Index.exprSpan())
		}
		indexValue, err := evt1EvalExpr(newEVT1ComptimeState(env), evt1EvalScopeFromValidation(scope, env), e.Index)
		if err == nil {
			if indexValue.Kind != EVT1ValueInt {
				return EVT1Type{}, evt1Diagnostic("CV4232", "array index must evaluate to int", e.Index.exprSpan())
			}
			if indexValue.IntValue < 0 || indexValue.IntValue >= baseType.ArrayLength {
				return EVT1Type{}, evt1Diagnostic("CV4233", fmt.Sprintf("array index %d is out of range for length %d", indexValue.IntValue, baseType.ArrayLength), e.Index.exprSpan())
			}
		} else if !inComptimeFn {
			return EVT1Type{}, err
		}
		return evt1CanonicalType(env, *baseType.ArrayElem), nil
	case *EVT1UnaryExpr:
		valueType, err := validateExpr(env, scope, e.Value, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		switch e.Op {
		case "-":
			if valueType.Name != "int" {
				return EVT1Type{}, evt1Diagnostic("CV4028", "unary - requires int", e.Span)
			}
			return valueType, nil
		case "not":
			if valueType.Name != "bool" {
				return EVT1Type{}, evt1Diagnostic("CV4028", "not requires bool", e.Span)
			}
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		default:
			return EVT1Type{}, evt1Diagnostic("CV4028", "unsupported unary operator "+e.Op, e.Span)
		}
	case *EVT1BinaryExpr:
		leftType, err := validateExpr(env, scope, e.Left, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		rightType, err := validateExpr(env, scope, e.Right, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if templateInfo != nil && (evt1TypeDependsOnParam(leftType, templateInfo.Decl.TypeParam) || evt1TypeDependsOnParam(rightType, templateInfo.Decl.TypeParam)) {
			return EVT1Type{}, evt1Diagnostic("CV4175", "dependent operators are not allowed in EVT1 M1B-B templates", e.Span)
		}
		if leftType.Name == "bool" && rightType.Name == "bool" && (e.Op == "and" || e.Op == "or" || e.Op == "==" || e.Op == "!=") {
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		}
		if leftType.Name == "string" && rightType.Name == "string" && (e.Op == "==" || e.Op == "!=") {
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		}
		if (leftType.ArrayElem != nil || rightType.ArrayElem != nil) && (e.Op == "<" || e.Op == ">" || e.Op == "<=" || e.Op == ">=") {
			return EVT1Type{}, evt1Diagnostic("CV4236", "array ordering comparisons are not supported", e.Span)
		}
		if leftType.Name == "int" && rightType.Name == "int" {
			if e.Op == "<" || e.Op == ">" || e.Op == "<=" || e.Op == ">=" || e.Op == "==" || e.Op == "!=" {
				out, _ := evt1BuiltinType("bool", e.Span)
				return out, nil
			}
			if e.Op == "+" || e.Op == "-" || e.Op == "*" {
				return leftType, nil
			}
		}
		if leftType.Name == "bool" && rightType.Name == "bool" {
			if e.Op == "==" || e.Op == "!=" {
				out, _ := evt1BuiltinType("bool", e.Span)
				return out, nil
			}
		}
		if leftType.Name == rightType.Name && leftType.Kind == EVT1TypeEnum && (e.Op == "==" || e.Op == "!=") {
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		}
		if leftType.Name == rightType.Name && leftType.Kind == EVT1TypeStruct && (e.Op == "==" || e.Op == "!=") && evt1IsComptimeType(env, leftType) {
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		}
		if leftType.ArrayElem != nil || rightType.ArrayElem != nil {
			if e.Op != "==" && e.Op != "!=" {
				return EVT1Type{}, evt1Diagnostic("CV4236", "fixed compile-time arrays only support == and !=", e.Span)
			}
			if !evt1CanonicalType(env, leftType).Equal(evt1CanonicalType(env, rightType)) {
				return EVT1Type{}, evt1Diagnostic("CV4238", fmt.Sprintf("array equality requires identical fixed-array types, got %s and %s", leftType.String(), rightType.String()), e.Span)
			}
			if !evt1TypeEqualityAvailable(env, leftType) {
				return EVT1Type{}, evt1Diagnostic("CV4239", fmt.Sprintf("array equality is unavailable for element type %s", leftType.ArrayElem.String()), e.Span)
			}
			out, _ := evt1BuiltinType("bool", e.Span)
			return out, nil
		}
		if leftType.Name == "uint64" && rightType.Name == "uint64" {
			if e.Op == "<" || e.Op == ">" || e.Op == "<=" || e.Op == ">=" {
				out, _ := evt1BuiltinType("bool", e.Span)
				return out, nil
			}
			return leftType, nil
		}
		return EVT1Type{}, evt1Diagnostic("CV4028", "only int/uint64 additive and comparison expressions are supported in EVT1", e.Span)
	case *EVT1ConstructExpr:
		return validateConstructExpr(env, scope, *e)
	case *EVT1StructConstructExpr:
		return validateStructConstructExpr(env, scope, *e)
	case *EVT1IfExpr:
		conditionType, err := validateExpr(env, scope, e.Condition, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if conditionType.Name != "bool" {
			return EVT1Type{}, evt1Diagnostic("CV4186", "if expression condition must be bool", e.Condition.exprSpan())
		}
		thenType, err := validateExpr(env, scope, e.Then, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		elseType, err := validateExpr(env, scope, e.Else, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1TypesCompatible(env, thenType, elseType, "") {
			return EVT1Type{}, evt1Diagnostic("CV4116", fmt.Sprintf("if expression arms must have the same type: got %s and %s", thenType.String(), elseType.String()), e.Span)
		}
		return evt1CanonicalType(env, thenType), nil
	case *EVT1MatchExpr:
		return validateMatchExpr(env, scope, *e, templateInfo, inComptimeFn)
	default:
		return EVT1Type{}, evt1Diagnostic("CV4029", fmt.Sprintf("unsupported expression %s", evt1Unexpected(expr)), expr.exprSpan())
	}
}

func validateCallArgument(env *evt1Env, scope *evt1Scope, paramType EVT1Type, arg EVT1Expr, argType EVT1Type, templateInfo *evt1TemplateInfo) error {
	typeParam := ""
	if templateInfo != nil {
		typeParam = templateInfo.Decl.TypeParam
	}
	if lit, ok := arg.(*EVT1ArrayLiteralExpr); ok && paramType.ArrayElem != nil {
		validatedType, err := validateArrayLiteralExpr(env, scope, *lit, &paramType, templateInfo, false)
		if err != nil {
			return err
		}
		argType = validatedType
	}
	if paramType.isBorrow() {
		required := paramType.borrowBase()
		if argType.isBorrowLike() {
			if !evt1TypesCompatible(env, required, argType.borrowBase(), typeParam) {
				return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
			}
			if !paramType.Const && argType.Const {
				return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot accept const %s", required.String(), argType.String()), arg.exprSpan())
			}
			return nil
		}
		if !evt1TypesCompatible(env, required, argType.valueType(), typeParam) {
			return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
		}
		lvalue, err := validateAssignable(env, scope, arg, templateInfo)
		if err != nil {
			return evt1Diagnostic("CV4127", fmt.Sprintf("borrow argument for %s requires an assignable access path", required.String()), arg.exprSpan())
		}
		if !paramType.Const && !lvalue.mutable {
			return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot bind a const access path", required.String()), arg.exprSpan())
		}
		return nil
	}
	if !evt1TypesCompatible(env, paramType, argType, typeParam) {
		return evt1Diagnostic("CV4107", fmt.Sprintf("wrong payload type for call argument: expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
	}
	if !evt1TypeCopyable(env, argType) && !evt1TypeDependsOnParam(argType, typeParam) {
		return evt1Diagnostic("CV4133", fmt.Sprintf("call copies non-copyable type %s", argType.String()), arg.exprSpan())
	}
	return nil
}

func validateAssignable(env *evt1Env, scope *evt1Scope, expr EVT1Expr, templateInfo *evt1TemplateInfo) (evt1LValue, error) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		binding, ok := scope.lookup(e.Name)
		if !ok {
			return evt1LValue{}, evt1Diagnostic("CV4127", fmt.Sprintf("unknown assignable target %s", e.Name), e.Span)
		}
		if binding.isInstance() {
			return evt1LValue{}, evt1Diagnostic("CV4272", fmt.Sprintf("instance %s of automata %s cannot be assigned or copied as a value", e.Name, binding.instanceAutomata), e.Span)
		}
		if binding.isBatch() {
			return evt1LValue{}, evt1Diagnostic("CV4305", fmt.Sprintf("effects batch %s of automata %s cannot be assigned or copied as a value", e.Name, binding.batchAutomata), e.Span)
		}
		if binding.isActuatorLocal() {
			return evt1LValue{}, evt1Diagnostic("CV4319", fmt.Sprintf("actuator local %s of actuator %s cannot be assigned or copied as a value", e.Name, binding.actuatorName), e.Span)
		}
		return evt1LValue{
			t:          evt1CanonicalType(env, binding.t),
			mutable:    binding.mutable,
			wholeValue: true,
			path: evt1AccessPath{
				Root: e.Name,
				Span: e.Span,
			},
		}, nil
	case *EVT1FieldExpr:
		receiver, err := validateAssignable(env, scope, e.Receiver, templateInfo)
		if err != nil {
			return evt1LValue{}, err
		}
		if templateInfo != nil && evt1TypeDependsOnParam(receiver.t, templateInfo.Decl.TypeParam) {
			return evt1LValue{}, evt1Diagnostic("CV4172", "dependent field access is not allowed in EVT1 M1B-B templates", e.Span)
		}
		fields, baseName, err := evt1FieldSet(env, receiver.t)
		if err != nil {
			return evt1LValue{}, evt1Diagnostic("CV4025", err.Error(), e.Span)
		}
		fieldType, ok := fields[e.Field]
		if !ok {
			return evt1LValue{}, evt1Diagnostic("CV4026", fmt.Sprintf("unknown field %s on %s", e.Field, baseName), e.Span)
		}
		path := receiver.path
		path.Fields = append(append([]string{}, receiver.path.Fields...), e.Field)
		path.Span = e.Span
		return evt1LValue{t: evt1CanonicalType(env, fieldType), mutable: receiver.mutable, wholeValue: false, path: path}, nil
	case *EVT1IndexExpr:
		return evt1LValue{}, evt1Diagnostic("CV4231", "fixed compile-time array elements are immutable in EVT1 M1B-D", expr.exprSpan())
	default:
		return evt1LValue{}, evt1Diagnostic("CV4127", "assignment requires a local or field access target", expr.exprSpan())
	}
}

func evt1FindOverlappingBorrow(borrows []evt1RetainedBorrow, path evt1AccessPath) (evt1RetainedBorrow, bool) {
	for _, borrow := range borrows {
		if evt1AccessPathsOverlap(borrow.Path, path) {
			return borrow, true
		}
	}
	return evt1RetainedBorrow{}, false
}

func evt1AccessPathsOverlap(a, b evt1AccessPath) bool {
	if a.Root == "" || b.Root == "" || a.Root != b.Root {
		return false
	}
	shared := len(a.Fields)
	if len(b.Fields) < shared {
		shared = len(b.Fields)
	}
	for i := 0; i < shared; i++ {
		if a.Fields[i] != b.Fields[i] {
			return false
		}
	}
	return true
}

func evt1FieldSet(env *evt1Env, t EVT1Type) (map[string]EVT1Type, string, error) {
	base := t
	if t.isBorrowLike() {
		base = t.borrowBase()
	}
	fields, ok := env.fieldSets[base.Name]
	if !ok {
		return nil, base.Name, fmt.Errorf("type %s has no fields", base.Name)
	}
	return fields, base.Name, nil
}

func validateConstructExpr(env *evt1Env, scope *evt1Scope, expr EVT1ConstructExpr) (EVT1Type, error) {
	enumDecl, ok := env.enums[expr.EnumName]
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum type %s in qualified construction", expr.EnumName), expr.Span)
	}
	variant, ok := evt1LookupVariant(enumDecl, expr.VariantName)
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4103", fmt.Sprintf("unknown variant %s::%s", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) == 0 && len(expr.Args) > 0 {
		return EVT1Type{}, evt1Diagnostic("CV4104", fmt.Sprintf("unit variant %s::%s does not accept payload arguments", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) > 0 && len(expr.Args) == 0 {
		return EVT1Type{}, evt1Diagnostic("CV4104", fmt.Sprintf("payload variant %s::%s requires construction arguments", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) != len(expr.Args) {
		return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor payload count for %s::%s: expected %d but got %d", expr.EnumName, expr.VariantName, len(variant.Payload), len(expr.Args)), expr.Span)
	}
	for i, arg := range expr.Args {
		argType, err := validateExprAgainstExpected(env, scope, arg, variant.Payload[i].Type, nil, false)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanInitializeStoredType(env, variant.Payload[i].Type, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong constructor payload type for %s::%s position %d: expected %s but got %s", expr.EnumName, expr.VariantName, i+1, variant.Payload[i].Type.String(), argType.String()), arg.exprSpan())
		}
		if !evt1TypeCopyable(env, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4133", fmt.Sprintf("enum construction copies non-copyable type %s", argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: enumDecl.Name, Kind: EVT1TypeEnum, Span: expr.Span}, nil
}

func validateStructConstructExpr(env *evt1Env, scope *evt1Scope, expr EVT1StructConstructExpr) (EVT1Type, error) {
	structDecl, ok := env.structs[expr.StructName]
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4125", fmt.Sprintf("unknown struct type %s", expr.StructName), expr.Span)
	}
	if len(structDecl.Fields) != len(expr.Args) {
		return EVT1Type{}, evt1Diagnostic("CV4126", fmt.Sprintf("wrong initializer count for %s: expected %d but got %d", expr.StructName, len(structDecl.Fields), len(expr.Args)), expr.Span)
	}
	for i, arg := range expr.Args {
		argType, err := validateExprAgainstExpected(env, scope, arg, structDecl.Fields[i].Type, nil, false)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanInitializeStoredType(env, structDecl.Fields[i].Type, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong initializer type for %s field %s: expected %s but got %s", expr.StructName, structDecl.Fields[i].Name, structDecl.Fields[i].Type.String(), argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: structDecl.Name, Kind: EVT1TypeStruct, Span: expr.Span}, nil
}

func validateMatchExpr(env *evt1Env, scope *evt1Scope, expr EVT1MatchExpr, templateInfo *evt1TemplateInfo, inComptimeFn bool) (EVT1Type, error) {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, expr.Subject, templateInfo, inComptimeFn)
	if err != nil {
		return EVT1Type{}, err
	}
	seen := map[string]bool{}
	var resultType EVT1Type
	for index, arm := range expr.Arms {
		armScope, _, err := validatePattern(env, scope, subjectType, enumDecl, arm.Pattern, seen)
		if err != nil {
			return EVT1Type{}, err
		}
		valueType, err := validateExpr(env, armScope, arm.Value, templateInfo, inComptimeFn)
		if err != nil {
			return EVT1Type{}, err
		}
		if index == 0 {
			resultType = evt1CanonicalType(env, valueType)
		} else if !evt1CanonicalType(env, resultType).Equal(evt1CanonicalType(env, valueType)) {
			return EVT1Type{}, evt1Diagnostic("CV4116", fmt.Sprintf("incompatible expression-arm result types: expected %s but got %s", resultType.String(), valueType.String()), arm.Value.exprSpan())
		}
	}
	if missing := evt1MissingVariants(enumDecl, seen); len(missing) > 0 {
		return EVT1Type{}, evt1Diagnostic("CV4115", "non-exhaustive match, missing variants: "+strings.Join(missing, ", "), expr.Span)
	}
	return resultType, nil
}

func validateMatchStmt(env *evt1Env, scope *evt1Scope, stmt EVT1MatchStmt, returnType EVT1Type, templateInfo *evt1TemplateInfo, inComptimeFn bool) error {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, stmt.Subject, templateInfo, inComptimeFn)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, arm := range stmt.Arms {
		armScope, _, err := validatePattern(env, scope, subjectType, enumDecl, arm.Pattern, seen)
		if err != nil {
			return err
		}
		if err := validateBlock(env, armScope, returnType, arm.Block, templateInfo, inComptimeFn); err != nil {
			return err
		}
	}
	if missing := evt1MissingVariants(enumDecl, seen); len(missing) > 0 {
		return evt1Diagnostic("CV4115", "non-exhaustive match, missing variants: "+strings.Join(missing, ", "), stmt.Span)
	}
	return nil
}

func validateMatchSubject(env *evt1Env, scope *evt1Scope, subject EVT1Expr, templateInfo *evt1TemplateInfo, inComptimeFn bool) (EVT1Type, EVT1EnumDecl, error) {
	subjectType, err := validateExpr(env, scope, subject, templateInfo, inComptimeFn)
	if err != nil {
		return EVT1Type{}, EVT1EnumDecl{}, err
	}
	if subjectType.Kind != EVT1TypeEnum {
		enumDecl, ok := env.enums[subjectType.Name]
		if !ok {
			return EVT1Type{}, EVT1EnumDecl{}, evt1Diagnostic("CV4108", fmt.Sprintf("match subject must be an enum, got %s", subjectType.String()), subject.exprSpan())
		}
		return subjectType, enumDecl, nil
	}
	return subjectType, env.enums[subjectType.Name], nil
}

func validatePattern(env *evt1Env, scope *evt1Scope, subjectType EVT1Type, enumDecl EVT1EnumDecl, pattern EVT1Pattern, seen map[string]bool) (*evt1Scope, EVT1VariantDecl, error) {
	if pattern.EnumName != enumDecl.Name {
		if _, ok := env.enums[pattern.EnumName]; ok {
			return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4109", fmt.Sprintf("match arm pattern uses %s on subject of type %s", pattern.EnumName, subjectType.Name), pattern.Span)
		}
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum type %s in match arm", pattern.EnumName), pattern.Span)
	}
	variant, ok := evt1LookupVariant(enumDecl, pattern.VariantName)
	if !ok {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4110", fmt.Sprintf("unknown match variant %s::%s", pattern.EnumName, pattern.VariantName), pattern.Span)
	}
	key := pattern.EnumName + "::" + pattern.VariantName
	if seen[key] {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4113", fmt.Sprintf("duplicate match arm %s", key), pattern.Span)
	}
	seen[key] = true
	if len(variant.Payload) == 0 && len(pattern.Bindings) > 0 {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4112", fmt.Sprintf("unit variant %s::%s cannot bind payload names", pattern.EnumName, pattern.VariantName), pattern.Span)
	}
	if len(variant.Payload) != len(pattern.Bindings) {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4111", fmt.Sprintf("payload binding count mismatch for %s::%s: expected %d but got %d", pattern.EnumName, pattern.VariantName, len(variant.Payload), len(pattern.Bindings)), pattern.Span)
	}
	armScope := newEVT1Scope(scope)
	seenBindings := map[string]bool{}
	for i, binding := range pattern.Bindings {
		if seenBindings[binding] {
			return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4112", fmt.Sprintf("duplicate payload binding name %s", binding), pattern.Span)
		}
		seenBindings[binding] = true
		armScope.declare(binding, evt1ValueBinding{t: evt1CanonicalType(env, variant.Payload[i].Type), mutable: true})
	}
	return armScope, variant, nil
}

func validateWhileStmt(env *evt1Env, scope *evt1Scope, stmt EVT1WhileStmt, templateInfo *evt1TemplateInfo, inComptimeFn bool) error {
	conditionType, err := validateExpr(env, scope, stmt.Condition, templateInfo, inComptimeFn)
	if err != nil {
		return err
	}
	if conditionType.Name != "bool" {
		return evt1Diagnostic("CV4187", "while condition must be bool", stmt.Condition.exprSpan())
	}
	if inComptimeFn && stmt.Bound == nil {
		return evt1Diagnostic("CV4205", "comptime while requires an explicit bounded(limit) clause", stmt.Span)
	}
	if stmt.Bound != nil {
		boundType, err := validateExpr(env, scope, stmt.Bound, templateInfo, true)
		if err != nil {
			return err
		}
		if boundType.Name != "int" {
			return evt1Diagnostic("CV4205", "bounded while requires a compile-time int bound", stmt.Bound.exprSpan())
		}
		value, err := evt1EvalExpr(newEVT1ComptimeState(env), evt1EvalScopeFromValidation(scope, env), stmt.Bound)
		if err != nil {
			return err
		}
		if value.Kind != EVT1ValueInt || value.IntValue < 0 {
			return evt1Diagnostic("CV4205", "bounded while requires a non-negative compile-time int bound", stmt.Bound.exprSpan())
		}
		if inComptimeFn && value.IntValue > evt1ComptimeMaxLoopBound {
			return evt1Diagnostic("CV4206", fmt.Sprintf("comptime loop bound %d exceeds limit %d", value.IntValue, evt1ComptimeMaxLoopBound), stmt.Bound.exprSpan())
		}
	}
	return validateBlock(env, scope, EVT1Type{Name: "void", Kind: EVT1TypeBuiltin}, stmt.Body, templateInfo, inComptimeFn)
}

const (
	evt1AutomataMaxGuardExprNodes      = 128
	evt1AutomataMaxGuardCallDepth      = 8
	evt1AutomataMaxGuardCallGraphNodes = 16
	evt1AutomataMaxGuardCallGraphEdges = 32
)

type evt1GuardCheckState struct {
	checked map[string]bool
	visiting map[string]bool
	nodes   int
	edges   int
}

func evt1ValidateAutomataGuard(env *evt1Env, info *evt1AutomataInfo, expr EVT1Expr) error {
	scope := evt1ModuleScope(env)
	if info.Decl.Context != nil {
		contextType := info.Decl.Context.Type
		contextType.Ownership = "borrow"
		contextType.Const = true
		scope.declare(info.Decl.Context.Name, evt1ValueBinding{
			t:        contextType,
			mutable:  false,
			comptime: false,
		})
	}
	guardType, err := validateExpr(env, scope, expr, nil, false)
	if err != nil {
		return err
	}
	if guardType.Name != "bool" {
		return evt1Diagnostic("CV4290", fmt.Sprintf("guard expression must have exact type bool, got %s", guardType.String()), expr.exprSpan())
	}
	if nodes := evt1GuardExprNodeCount(expr); nodes > evt1AutomataMaxGuardExprNodes {
		return evt1Diagnostic("CV4297", fmt.Sprintf("guard expression node count %d exceeds limit %d", nodes, evt1AutomataMaxGuardExprNodes), expr.exprSpan())
	}
	state := &evt1GuardCheckState{
		checked:  map[string]bool{},
		visiting: map[string]bool{},
	}
	return evt1ValidateGuardExpr(env, scope, expr, state, 0)
}

func evt1ValidateGuardExpr(env *evt1Env, scope *evt1Scope, expr EVT1Expr, state *evt1GuardCheckState, depth int) error {
	switch e := expr.(type) {
	case *EVT1NameExpr, *EVT1IntLiteral, *EVT1StringLiteral, *EVT1BoolLiteral:
		return nil
	case *EVT1FieldExpr:
		return evt1ValidateGuardExpr(env, scope, e.Receiver, state, depth)
	case *EVT1ParenExpr:
		return evt1ValidateGuardExpr(env, scope, e.Value, state, depth)
	case *EVT1UnaryExpr:
		return evt1ValidateGuardExpr(env, scope, e.Value, state, depth)
	case *EVT1BinaryExpr:
		if err := evt1ValidateGuardExpr(env, scope, e.Left, state, depth); err != nil {
			return err
		}
		return evt1ValidateGuardExpr(env, scope, e.Right, state, depth)
	case *EVT1IfExpr:
		if err := evt1ValidateGuardExpr(env, scope, e.Condition, state, depth); err != nil {
			return err
		}
		if err := evt1ValidateGuardExpr(env, scope, e.Then, state, depth); err != nil {
			return err
		}
		return evt1ValidateGuardExpr(env, scope, e.Else, state, depth)
	case *EVT1MatchExpr:
		if err := evt1ValidateGuardExpr(env, scope, e.Subject, state, depth); err != nil {
			return err
		}
		for _, arm := range e.Arms {
			if err := evt1ValidateGuardExpr(env, scope, arm.Value, state, depth); err != nil {
				return err
			}
		}
		return nil
	case *EVT1ConstructExpr:
		for _, arg := range e.Args {
			if err := evt1ValidateGuardExpr(env, scope, arg, state, depth); err != nil {
				return err
			}
		}
		return nil
	case *EVT1StructConstructExpr:
		for _, arg := range e.Args {
			if err := evt1ValidateGuardExpr(env, scope, arg, state, depth); err != nil {
				return err
			}
		}
		return nil
	case *EVT1ArrayLiteralExpr:
		for _, element := range e.Elements {
			if err := evt1ValidateGuardExpr(env, scope, element, state, depth); err != nil {
				return err
			}
		}
		return nil
	case *EVT1IndexExpr:
		if err := evt1ValidateGuardExpr(env, scope, e.Base, state, depth); err != nil {
			return err
		}
		return evt1ValidateGuardExpr(env, scope, e.Index, state, depth)
	case *EVT1DispatchExpr:
		return evt1Diagnostic("CV4292", "dispatch is not allowed in automata guards", e.Span)
	case *EVT1TemplateCallExpr:
		return evt1Diagnostic("CV4293", "template calls are not allowed in automata guards", e.Span)
	case *EVT1CallExpr:
		if e.Callee == "Len" {
			for _, arg := range e.Args {
				if err := evt1ValidateGuardExpr(env, scope, arg, state, depth); err != nil {
					return err
				}
			}
			return nil
		}
		argTypes := make([]EVT1Type, 0, len(e.Args))
		for _, arg := range e.Args {
			if err := evt1ValidateGuardExpr(env, scope, arg, state, depth); err != nil {
				return err
			}
			argType, err := validateExpr(env, scope, arg, nil, false)
			if err != nil {
				return err
			}
			argTypes = append(argTypes, argType)
		}
		fn, err := evt1ResolveOrdinaryCall(env, scope, e.Callee, e.Args, argTypes, nil, e.Span)
		if err != nil {
			return err
		}
		return evt1ValidateGuardFunction(env, fn, state, depth+1)
	default:
		return evt1Diagnostic("CV4294", "unsupported guard expression form", expr.exprSpan())
	}
}

func evt1ValidateGuardFunction(env *evt1Env, fn EVT1FunctionDecl, state *evt1GuardCheckState, depth int) error {
	if depth > evt1AutomataMaxGuardCallDepth {
		return evt1Diagnostic("CV4298", fmt.Sprintf("guard call depth %d exceeds limit %d", depth, evt1AutomataMaxGuardCallDepth), fn.Span)
	}
	key := fn.Name + "|" + evt1FunctionParamSignature(fn)
	if state.visiting[key] {
		return evt1Diagnostic("CV4296", "recursive guard call graph is not allowed: "+key, fn.Span)
	}
	if state.checked[key] {
		return nil
	}
	state.nodes++
	if state.nodes > evt1AutomataMaxGuardCallGraphNodes {
		return evt1Diagnostic("CV4298", fmt.Sprintf("guard call graph node count %d exceeds limit %d", state.nodes, evt1AutomataMaxGuardCallGraphNodes), fn.Span)
	}
	if fn.Body == nil {
		return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s requires a local function body so purity can be verified", fn.Name), fn.Span)
	}
	if fn.ReturnType.Name == "void" {
		return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s must return a runtime value", fn.Name), fn.Span)
	}
	for _, param := range fn.Params {
		if param.Type.isOwned() {
			return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot take owned parameter %s", fn.Name, param.Type.String()), param.Span)
		}
		if param.Type.isBorrow() && !param.Type.Const {
			return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot take mutable borrow parameter %s", fn.Name, param.Type.String()), param.Span)
		}
		if param.Type.PointerTo != nil {
			return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot take pointer parameter %s", fn.Name, param.Type.String()), param.Span)
		}
		if !param.Type.isBorrowLike() && !evt1TypeCopyable(env, param.Type) {
			return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot take non-copyable parameter %s", fn.Name, param.Type.String()), param.Span)
		}
	}
	state.visiting[key] = true
	guardScope := newEVT1Scope(evt1ModuleScope(env))
	for _, param := range fn.Params {
		guardScope.declare(param.Name, evt1ValueBinding{
			t:        evt1CanonicalType(env, param.Type),
			mutable:  false,
			comptime: false,
		})
	}
	if err := evt1ValidateGuardFunctionBlock(env, guardScope, *fn.Body, fn, state, depth); err != nil {
		delete(state.visiting, key)
		return err
	}
	delete(state.visiting, key)
	state.checked[key] = true
	return nil
}

func evt1ValidateGuardFunctionBlock(env *evt1Env, scope *evt1Scope, block EVT1Block, fn EVT1FunctionDecl, state *evt1GuardCheckState, depth int) error {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			if s.Comptime {
				return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot use comptime locals", fn.Name), s.Span)
			}
			if _, err := validateExpr(env, scope, s.Value, nil, false); err != nil {
				return err
			}
			if err := evt1ValidateGuardExpr(env, scope, s.Value, state, depth); err != nil {
				return err
			}
			resolvedType, err := evt1ResolveType(env, scope, s.Type)
			if err != nil {
				return err
			}
			scope.declare(s.Name, evt1ValueBinding{t: evt1CanonicalType(env, resolvedType), mutable: false})
		case *EVT1ReturnStmt:
			if s.Value != nil {
				if _, err := validateExpr(env, scope, s.Value, nil, false); err != nil {
					return err
				}
				if err := evt1ValidateGuardExpr(env, scope, s.Value, state, depth); err != nil {
					return err
				}
			}
		case *EVT1ExprStmt:
			if _, err := validateExpr(env, scope, s.Value, nil, false); err != nil {
				return err
			}
			if err := evt1ValidateGuardExpr(env, scope, s.Value, state, depth); err != nil {
				return err
			}
		case *EVT1Block:
			child := newEVT1Scope(scope)
			if err := evt1ValidateGuardFunctionBlock(env, child, *s, fn, state, depth); err != nil {
				return err
			}
		default:
			return evt1Diagnostic("CV4295", fmt.Sprintf("guard call target %s cannot use %s", fn.Name, evt1GuardStatementLabel(stmt)), stmt.statementSpan())
		}
	}
	return nil
}

func evt1GuardStatementLabel(stmt EVT1Statement) string {
	switch stmt.(type) {
	case *EVT1AssignStmt:
		return "assignment"
	case *EVT1InstanceDecl:
		return "instance declarations"
	case *EVT1MatchStmt:
		return "statement-form match"
	case *EVT1WhileStmt:
		return "while loops"
	case *EVT1StaticAssertStmt:
		return "static_assert"
	default:
		return "that statement form"
	}
}

func evt1GuardExprNodeCount(expr EVT1Expr) int {
	count := 1
	switch e := expr.(type) {
	case *EVT1FieldExpr:
		count += evt1GuardExprNodeCount(e.Receiver)
	case *EVT1CallExpr:
		for _, arg := range e.Args {
			count += evt1GuardExprNodeCount(arg)
		}
	case *EVT1DispatchExpr:
		count += evt1GuardExprNodeCount(e.Signal)
	case *EVT1TemplateCallExpr:
		for _, arg := range e.Args {
			count += evt1GuardExprNodeCount(arg)
		}
	case *EVT1BinaryExpr:
		count += evt1GuardExprNodeCount(e.Left)
		count += evt1GuardExprNodeCount(e.Right)
	case *EVT1UnaryExpr:
		count += evt1GuardExprNodeCount(e.Value)
	case *EVT1ConstructExpr:
		for _, arg := range e.Args {
			count += evt1GuardExprNodeCount(arg)
		}
	case *EVT1StructConstructExpr:
		for _, arg := range e.Args {
			count += evt1GuardExprNodeCount(arg)
		}
	case *EVT1ArrayLiteralExpr:
		for _, element := range e.Elements {
			count += evt1GuardExprNodeCount(element)
		}
	case *EVT1IndexExpr:
		count += evt1GuardExprNodeCount(e.Base)
		count += evt1GuardExprNodeCount(e.Index)
	case *EVT1MatchExpr:
		count += evt1GuardExprNodeCount(e.Subject)
		for _, arm := range e.Arms {
			count += evt1GuardExprNodeCount(arm.Value)
		}
	case *EVT1IfExpr:
		count += evt1GuardExprNodeCount(e.Condition)
		count += evt1GuardExprNodeCount(e.Then)
		count += evt1GuardExprNodeCount(e.Else)
	case *EVT1ParenExpr:
		count += evt1GuardExprNodeCount(e.Value)
	}
	return count
}

func evt1ExprIdentity(expr EVT1Expr) string {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		return e.Name
	case *EVT1IntLiteral:
		return fmt.Sprintf("%d", e.Value)
	case *EVT1StringLiteral:
		return fmt.Sprintf("%q", e.Value)
	case *EVT1BoolLiteral:
		if e.Value {
			return "true"
		}
		return "false"
	case *EVT1FieldExpr:
		return evt1ExprIdentity(e.Receiver) + "." + e.Field
	case *EVT1CallExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, evt1ExprIdentity(arg))
		}
		return e.Callee + "(" + strings.Join(args, ",") + ")"
	case *EVT1DispatchExpr:
		if e.BatchName != "" {
			return "dispatch(" + e.InstanceName + "," + evt1ExprIdentity(e.Signal) + "," + e.BatchName + ")"
		}
		return "dispatch(" + e.InstanceName + "," + evt1ExprIdentity(e.Signal) + ")"
	case *EVT1TemplateCallExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, evt1ExprIdentity(arg))
		}
		return e.Callee + "<" + e.TypeArg.String() + ">(" + strings.Join(args, ",") + ")"
	case *EVT1BinaryExpr:
		return "(" + evt1ExprIdentity(e.Left) + " " + e.Op + " " + evt1ExprIdentity(e.Right) + ")"
	case *EVT1UnaryExpr:
		return "(" + e.Op + " " + evt1ExprIdentity(e.Value) + ")"
	case *EVT1ConstructExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, evt1ExprIdentity(arg))
		}
		return e.EnumName + "::" + e.VariantName + "(" + strings.Join(args, ",") + ")"
	case *EVT1StructConstructExpr:
		var args []string
		for _, arg := range e.Args {
			args = append(args, evt1ExprIdentity(arg))
		}
		return e.StructName + "{" + strings.Join(args, ",") + "}"
	case *EVT1ArrayLiteralExpr:
		var parts []string
		for _, element := range e.Elements {
			parts = append(parts, evt1ExprIdentity(element))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case *EVT1IndexExpr:
		return evt1ExprIdentity(e.Base) + "[" + evt1ExprIdentity(e.Index) + "]"
	case *EVT1MatchExpr:
		var arms []string
		for _, arm := range e.Arms {
			arms = append(arms, arm.Pattern.EnumName+"::"+arm.Pattern.VariantName+"=>"+evt1ExprIdentity(arm.Value))
		}
		return "match(" + evt1ExprIdentity(e.Subject) + "){" + strings.Join(arms, ",") + "}"
	case *EVT1IfExpr:
		return "if(" + evt1ExprIdentity(e.Condition) + ") " + evt1ExprIdentity(e.Then) + " else " + evt1ExprIdentity(e.Else)
	case *EVT1ParenExpr:
		return "(" + evt1ExprIdentity(e.Value) + ")"
	default:
		return "<expr>"
	}
}

func validateValueLayoutCycles(env *evt1Env) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(name string, span Span) error
	dfs = func(name string, span Span) error {
		if visiting[name] {
			return evt1Diagnostic("CV4129", fmt.Sprintf("recursive by-value type cycle involving %s", name), span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		if structDecl, ok := env.structs[name]; ok {
			for _, field := range structDecl.Fields {
				if target, ok := evt1ByValueTypeName(field.Type); ok {
					if err := dfs(target, field.Span); err != nil {
						return err
					}
				}
			}
		}
		if enumDecl, ok := env.enums[name]; ok {
			for _, variant := range enumDecl.Variants {
				for _, field := range variant.Payload {
					if target, ok := evt1ByValueTypeName(field.Type); ok {
						if err := dfs(target, field.Span); err != nil {
							return err
						}
					}
				}
			}
		}
		visiting[name] = false
		return nil
	}
	for _, structDecl := range env.structs {
		if err := dfs(structDecl.Name, structDecl.Span); err != nil {
			return err
		}
	}
	return nil
}

func evt1ByValueTypeName(t EVT1Type) (string, bool) {
	if t.ArrayElem != nil {
		return evt1ByValueTypeName(*t.ArrayElem)
	}
	if t.isBorrowLike() {
		return "", false
	}
	if len(t.TypeArgs) > 0 {
		return "", false
	}
	if t.Kind == EVT1TypeBuiltin || t.Kind == EVT1TypeConceptParam {
		return "", false
	}
	return t.Name, true
}

func evt1TypeCopyable(env *evt1Env, t EVT1Type) bool {
	if t.isBorrowLike() {
		return true
	}
	if t.ArrayElem != nil {
		return evt1TypeCopyable(env, *t.ArrayElem)
	}
	if t.isOwned() {
		return false
	}
	if t.Name == "Result" && len(t.TypeArgs) == 2 {
		return evt1CanonicalType(env, t.TypeArgs[0]).Name == "void" && evt1TypeCopyable(env, t.TypeArgs[1])
	}
	if len(t.TypeArgs) > 0 {
		return false
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		return true
	}
	if cached, ok := env.copyableCache[t.Name]; ok {
		return cached
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		if structDecl.Immovable {
			env.copyableCache[t.Name] = false
			return false
		}
		env.copyableCache[t.Name] = true
		for _, field := range structDecl.Fields {
			if !evt1TypeCopyable(env, field.Type) {
				env.copyableCache[t.Name] = false
				return false
			}
		}
		return true
	}
	if enumDecl, ok := env.enums[t.Name]; ok {
		env.copyableCache[t.Name] = true
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if !evt1TypeCopyable(env, field.Type) {
					env.copyableCache[t.Name] = false
					return false
				}
			}
		}
		return true
	}
	return true
}

func evt1IsImmovableValueType(env *evt1Env, t EVT1Type) bool {
	if t.isBorrowLike() || len(t.TypeArgs) > 0 {
		return false
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		return structDecl.Immovable
	}
	return false
}

func evt1CanDirectInitialize(env *evt1Env, t EVT1Type, expr EVT1Expr) bool {
	if lit, ok := expr.(*EVT1ArrayLiteralExpr); ok {
		return t.ArrayElem != nil && len(lit.Elements) == t.ArrayLength
	}
	construct, ok := expr.(*EVT1StructConstructExpr)
	if !ok {
		return false
	}
	return construct.StructName == t.Name && evt1LookupStruct(env, t.Name)
}

func evt1LookupStruct(env *evt1Env, name string) bool {
	_, ok := env.structs[name]
	return ok
}

func validateConceptCycles(env *evt1Env) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(name string, span Span) error
	dfs = func(name string, span Span) error {
		if visiting[name] {
			return evt1Diagnostic("CV4162", fmt.Sprintf("concept dependency cycle involving %s", name), span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		for _, req := range env.concepts[name].Requirements {
			ref, ok := req.(*EVT1PrerequisiteRequirement)
			if !ok {
				continue
			}
			if err := dfs(ref.ConceptName, ref.Span); err != nil {
				return err
			}
		}
		visiting[name] = false
		return nil
	}
	for _, conceptDecl := range env.concepts {
		if err := dfs(conceptDecl.Name, conceptDecl.Span); err != nil {
			return err
		}
	}
	return nil
}

func evt1ResolveOrdinaryCall(env *evt1Env, scope *evt1Scope, name string, args []EVT1Expr, argTypes []EVT1Type, templateInfo *evt1TemplateInfo, span Span) (EVT1FunctionDecl, error) {
	candidates, ok := env.functions[name]
	if !ok || len(candidates) == 0 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4027", fmt.Sprintf("unknown function %s", name), span)
	}
	var matches []EVT1FunctionDecl
	for _, fn := range candidates {
		if len(fn.Params) != len(args) {
			continue
		}
		match := true
		for i, arg := range args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, fn)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4182", fmt.Sprintf("call %s is ambiguous under exact-signature matching", name), span)
	}
	return EVT1FunctionDecl{}, evt1Diagnostic("CV4107", fmt.Sprintf("no exact call target matched %s", name), span)
}

func checkConceptSatisfaction(env *evt1Env, conceptName string, concreteType EVT1Type, path []string, span Span) error {
	path = append(path, fmt.Sprintf("%s<%s>", conceptName, concreteType.String()))
	conceptDecl := env.concepts[conceptName]
	for _, req := range conceptDecl.Requirements {
		switch r := req.(type) {
		case *EVT1PrerequisiteRequirement:
			if err := checkConceptSatisfaction(env, r.ConceptName, concreteType, path, span); err != nil {
				return err
			}
		case *EVT1OperationRequirement:
			required := evt1SubstituteRequirement(*r, conceptDecl.TypeParam, concreteType)
			if _, err := evt1LookupRequiredOperation(env, required, span, strings.Join(path, " -> ")); err != nil {
				return err
			}
		}
	}
	return nil
}

func evt1LookupRequiredOperation(env *evt1Env, required EVT1OperationRequirement, span Span, prefix string) (EVT1FunctionDecl, error) {
	candidates := env.functions[required.Name]
	if len(candidates) == 0 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4153", fmt.Sprintf("%s is missing required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
	}
	var exact []EVT1FunctionDecl
	for _, fn := range candidates {
		if len(fn.Params) != len(required.Params) {
			continue
		}
		paramsMatch := true
		qualifierMismatch := false
		for i := range fn.Params {
			if !evt1CanonicalType(env, fn.Params[i].Type).Equal(evt1CanonicalType(env, required.Params[i].Type)) {
				if evt1CanonicalType(env, fn.Params[i].Type.valueType()).Equal(evt1CanonicalType(env, required.Params[i].Type.valueType())) {
					qualifierMismatch = true
				}
				paramsMatch = false
				break
			}
		}
		if !paramsMatch {
			if qualifierMismatch {
				return EVT1FunctionDecl{}, evt1Diagnostic("CV4155", fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
			}
			continue
		}
		if !evt1CanonicalType(env, fn.ReturnType).Equal(evt1CanonicalType(env, required.ReturnType)) {
			return EVT1FunctionDecl{}, evt1Diagnostic("CV4156", fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
		}
		exact = append(exact, fn)
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4182", fmt.Sprintf("%s has ambiguous required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
	}
	for _, fn := range candidates {
		if len(fn.Params) == len(required.Params) {
			code := "CV4154"
			for i := range fn.Params {
				if i < len(required.Params) && evt1CanonicalType(env, fn.Params[i].Type.valueType()).Equal(evt1CanonicalType(env, required.Params[i].Type.valueType())) && !evt1CanonicalType(env, fn.Params[i].Type).Equal(evt1CanonicalType(env, required.Params[i].Type)) {
					code = "CV4155"
					break
				}
			}
			return EVT1FunctionDecl{}, evt1Diagnostic(code, fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
		}
	}
	return EVT1FunctionDecl{}, evt1Diagnostic("CV4153", fmt.Sprintf("%s is missing required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
}

func evt1SubstituteRequirement(req EVT1OperationRequirement, typeParam string, concreteType EVT1Type) EVT1OperationRequirement {
	out := req
	out.ReturnType = evt1SubstituteType(req.ReturnType, typeParam, concreteType)
	out.Params = make([]EVT1Param, 0, len(req.Params))
	for _, param := range req.Params {
		out.Params = append(out.Params, EVT1Param{
			Type: evt1SubstituteType(param.Type, typeParam, concreteType),
			Name: param.Name,
			Span: param.Span,
		})
	}
	return out
}

func evt1SubstituteType(t EVT1Type, typeParam string, concreteType EVT1Type) EVT1Type {
	if t.Kind == EVT1TypeConceptParam && t.Name == typeParam {
		out := concreteType
		out.Ownership = t.Ownership
		out.Const = t.Const
		out.Imported = t.Imported
		out.Unsafe = t.Unsafe
		return out
	}
	if t.PointerTo != nil {
		base := evt1SubstituteType(*t.PointerTo, typeParam, concreteType)
		t.PointerTo = &base
		return t
	}
	for i := range t.TypeArgs {
		t.TypeArgs[i] = evt1SubstituteType(t.TypeArgs[i], typeParam, concreteType)
	}
	return t
}

func evt1Signature(retType EVT1Type, name string, params []EVT1Param) string {
	var parts []string
	for _, param := range params {
		parts = append(parts, param.Type.String())
	}
	return fmt.Sprintf("%s %s(%s)", retType.String(), name, strings.Join(parts, ", "))
}

func evt1FunctionSignature(fn EVT1FunctionDecl) string {
	return evt1Signature(fn.ReturnType, fn.Name, fn.Params)
}

func evt1FunctionParamSignature(fn EVT1FunctionDecl) string {
	var parts []string
	for _, param := range fn.Params {
		parts = append(parts, param.Type.String())
	}
	return fmt.Sprintf("%s(%s)", fn.Name, strings.Join(parts, ", "))
}

func exprLabel(expr EVT1Expr) string {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		return e.Name
	case *EVT1FieldExpr:
		return exprLabel(e.Receiver) + "." + e.Field
	case *EVT1IndexExpr:
		return exprLabel(e.Base) + "[index]"
	case *EVT1ArrayLiteralExpr:
		return "array_literal"
	case *EVT1DispatchExpr:
		return "dispatch(" + e.InstanceName + ", ...)"
	default:
		return "expression"
	}
}

func evt1CanonicalType(env *evt1Env, t EVT1Type) EVT1Type {
	if t.PointerTo != nil {
		base := evt1CanonicalType(env, *t.PointerTo)
		t.PointerTo = &base
		return t
	}
	if t.ArrayElem != nil {
		elem := evt1CanonicalType(env, *t.ArrayElem)
		t.ArrayElem = &elem
		t.Kind = EVT1TypeArray
		return t
	}
	for i := range t.TypeArgs {
		t.TypeArgs[i] = evt1CanonicalType(env, t.TypeArgs[i])
	}
	if t.Kind == EVT1TypeConceptParam || len(t.TypeArgs) > 0 {
		return t
	}
	if env == nil {
		return t
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		t.Kind = EVT1TypeBuiltin
		return t
	}
	if _, ok := env.enums[t.Name]; ok {
		t.Kind = EVT1TypeEnum
		return t
	}
	if _, ok := env.structs[t.Name]; ok {
		t.Kind = EVT1TypeStruct
		return t
	}
	return t
}

func evt1CanInitializeStoredType(env *evt1Env, expected EVT1Type, actual EVT1Type) bool {
	return evt1CanonicalType(env, expected.valueType()).Equal(evt1CanonicalType(env, actual.valueType()))
}

func evt1TypeDependsOnParam(t EVT1Type, typeParam string) bool {
	if typeParam == "" {
		return false
	}
	if t.Kind == EVT1TypeConceptParam && t.Name == typeParam {
		return true
	}
	if t.PointerTo != nil && evt1TypeDependsOnParam(*t.PointerTo, typeParam) {
		return true
	}
	if t.ArrayElem != nil && evt1TypeDependsOnParam(*t.ArrayElem, typeParam) {
		return true
	}
	for _, arg := range t.TypeArgs {
		if evt1TypeDependsOnParam(arg, typeParam) {
			return true
		}
	}
	return false
}

func evt1TypesCompatible(env *evt1Env, expected EVT1Type, actual EVT1Type, typeParam string) bool {
	expected = evt1CanonicalType(env, expected.valueType())
	actual = evt1CanonicalType(env, actual.valueType())
	if !evt1TypeDependsOnParam(expected, typeParam) && !evt1TypeDependsOnParam(actual, typeParam) {
		return expected.Equal(actual)
	}
	return evt1SymbolicTypeEqual(expected, actual, typeParam)
}

func evt1SymbolicTypeEqual(a EVT1Type, b EVT1Type, typeParam string) bool {
	if a.Kind == EVT1TypeConceptParam || b.Kind == EVT1TypeConceptParam {
		return a.Kind == b.Kind && a.Name == b.Name
	}
	if (a.PointerTo == nil) != (b.PointerTo == nil) {
		return false
	}
	if a.PointerTo != nil {
		return a.Const == b.Const && evt1SymbolicTypeEqual(*a.PointerTo, *b.PointerTo, typeParam)
	}
	if (a.ArrayElem == nil) != (b.ArrayElem == nil) {
		return false
	}
	if a.ArrayElem != nil {
		return a.ArrayLength == b.ArrayLength && evt1SymbolicTypeEqual(*a.ArrayElem, *b.ArrayElem, typeParam)
	}
	if a.Name != b.Name || a.Kind != b.Kind || a.Ownership != b.Ownership || a.Const != b.Const || a.Imported != b.Imported || a.Unsafe != b.Unsafe || len(a.TypeArgs) != len(b.TypeArgs) {
		return false
	}
	for i := range a.TypeArgs {
		if !evt1SymbolicTypeEqual(a.TypeArgs[i], b.TypeArgs[i], typeParam) {
			return false
		}
	}
	return true
}

func buildTemplateInfo(env *evt1Env, templateDecl EVT1TemplateDecl) (*evt1TemplateInfo, error) {
	info := &evt1TemplateInfo{
		Decl:         templateDecl,
		CallBindings: map[string]evt1TemplateCallBinding{},
	}
	seenConcepts := map[string]bool{}
	seenRequirements := map[string]bool{}
	typeParamType := EVT1Type{Name: templateDecl.TypeParam, Kind: EVT1TypeConceptParam, Span: templateDecl.TypeParamSpan}
	var walk func(conceptName string, path []string) error
	walk = func(conceptName string, path []string) error {
		if !seenConcepts[conceptName] {
			info.Closure = append(info.Closure, evt1TemplateClosureEntry{
				Concept: conceptName,
				Path:    append([]string{}, path...),
			})
			seenConcepts[conceptName] = true
		}
		conceptDecl := env.concepts[conceptName]
		for _, rawReq := range conceptDecl.Requirements {
			switch req := rawReq.(type) {
			case *EVT1PrerequisiteRequirement:
				nextPath := append(append([]string{}, path...), req.ConceptName)
				if err := walk(req.ConceptName, nextPath); err != nil {
					return err
				}
			case *EVT1OperationRequirement:
				substituted := evt1SubstituteRequirement(*req, conceptDecl.TypeParam, typeParamType)
				key := evt1RequirementKey(substituted)
				if seenRequirements[key] {
					continue
				}
				seenRequirements[key] = true
				info.Requirements = append(info.Requirements, evt1TemplateRequirement{
					ID:        fmt.Sprintf("%s.req.%02d", templateDecl.Name, len(info.Requirements)+1),
					Path:      append([]string{}, path...),
					Concept:   conceptName,
					Operation: substituted,
				})
			}
		}
		return nil
	}
	rootPath := []string{templateDecl.Constraint.ConceptName}
	if err := walk(templateDecl.Constraint.ConceptName, rootPath); err != nil {
		return nil, err
	}
	return info, nil
}

func evt1RequirementKey(req EVT1OperationRequirement) string {
	return evt1Signature(req.ReturnType, req.Name, req.Params)
}

func evt1SpanKey(span Span) string {
	return fmt.Sprintf("%d:%d", span.Line, span.Column)
}

func validateTemplateCallExpr(env *evt1Env, scope *evt1Scope, call EVT1CallExpr, templateInfo *evt1TemplateInfo) (EVT1Type, error) {
	argTypes := make([]EVT1Type, 0, len(call.Args))
	dependent := false
	for _, arg := range call.Args {
		argType, err := validateExpr(env, scope, arg, templateInfo, false)
		if err != nil {
			return EVT1Type{}, err
		}
		if evt1TypeDependsOnParam(argType, templateInfo.Decl.TypeParam) {
			dependent = true
		}
		argTypes = append(argTypes, argType)
	}
	if !dependent {
		fn, err := evt1ResolveOrdinaryCall(env, scope, call.Callee, call.Args, argTypes, templateInfo, call.Span)
		if err != nil {
			if _, exists := env.templates[call.Callee]; exists {
				return EVT1Type{}, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", call.Span)
			}
			return EVT1Type{}, err
		}
		for i, arg := range call.Args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				return EVT1Type{}, err
			}
		}
		return evt1CanonicalType(env, fn.ReturnType), nil
	}
	var matches []evt1TemplateRequirement
	for _, req := range templateInfo.Requirements {
		if req.Operation.Name != call.Callee || len(req.Operation.Params) != len(call.Args) {
			continue
		}
		ok := true
		for i, arg := range call.Args {
			if err := validateCallArgument(env, scope, req.Operation.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, req)
		}
	}
	if len(matches) == 0 {
		return EVT1Type{}, evt1Diagnostic("CV4176", fmt.Sprintf("template body call %s is not guaranteed by constraint %s", call.Callee, templateInfo.Decl.Constraint.ConceptName), call.Span)
	}
	if len(matches) > 1 {
		return EVT1Type{}, evt1Diagnostic("CV4177", fmt.Sprintf("template body call %s is ambiguously guaranteed by constraint %s", call.Callee, templateInfo.Decl.Constraint.ConceptName), call.Span)
	}
	templateInfo.CallBindings[evt1SpanKey(call.Span)] = evt1TemplateCallBinding{
		CallSpan:    call.Span,
		Requirement: matches[0],
	}
	return evt1CanonicalType(env, matches[0].Operation.ReturnType), nil
}

func instantiateTemplate(env *evt1Env, templateName string, concreteType EVT1Type, span Span) (*evt1TemplateInstance, error) {
	templateDecl, ok := env.templates[templateName]
	if !ok {
		return nil, evt1Diagnostic("CV4178", fmt.Sprintf("unknown template %s", templateName), span)
	}
	if err := validateTemplateTypeArgument(env, concreteType, span); err != nil {
		return nil, err
	}
	concreteType = evt1CanonicalType(env, concreteType)
	key := templateName + "|" + evt1TypeIdentity(concreteType)
	if instance, ok := env.templateInstances[key]; ok {
		return instance, nil
	}
	if err := checkConceptSatisfaction(env, templateDecl.Constraint.ConceptName, concreteType, nil, span); err != nil {
		return nil, err
	}
	info := env.templateInfos[templateName]
	var bindings []evt1InstanceRequirementBinding
	for _, req := range info.Requirements {
		concreteReq := evt1SubstituteRequirement(req.Operation, templateDecl.TypeParam, concreteType)
		fn, err := evt1LookupRequiredOperation(env, concreteReq, span, templateName+"<"+concreteType.String()+">")
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, evt1InstanceRequirementBinding{
			Requirement: req,
			Function:    fn,
		})
	}
	instFn, err := evt1InstantiateTemplateFunction(templateDecl, concreteType)
	if err != nil {
		return nil, err
	}
	if err := validateFunctionSignature(env, instFn); err != nil {
		return nil, err
	}
	scope := newEVT1Scope(nil)
	for _, param := range instFn.Params {
		scope.declare(param.Name, evt1ValueBinding{
			t:       evt1CanonicalType(env, param.Type),
			mutable: !(param.Type.isBorrowLike() && param.Type.Const),
		})
	}
	if err := validateBlock(env, scope, instFn.ReturnType, *instFn.Body, nil, false); err != nil {
		return nil, err
	}
	instance := &evt1TemplateInstance{
		Key:                 key,
		TemplateName:        templateName,
		ConcreteType:        concreteType,
		TypeIdentity:        evt1TypeIdentity(concreteType),
		GeneratedSymbol:     evt1TemplateInstanceSymbol(templateName, concreteType),
		ConstraintConcept:   templateDecl.Constraint.ConceptName,
		Closure:             append([]evt1TemplateClosureEntry{}, info.Closure...),
		RequirementBindings: bindings,
		Function:            instFn,
		SourceSpan:          templateDecl.Span,
	}
	env.templateInstances[key] = instance
	return instance, nil
}

func validateTemplateTypeArgument(env *evt1Env, concreteType EVT1Type, span Span) error {
	if concreteType.PointerTo != nil || concreteType.ArrayElem != nil || concreteType.Ownership != "" || concreteType.Const || concreteType.Imported || concreteType.Unsafe || len(concreteType.TypeArgs) > 0 || concreteType.Kind == EVT1TypeConceptParam {
		return evt1Diagnostic("CV4179", "template calls require one concrete non-template type argument", span)
	}
	return validateKnownType(env, concreteType, span, "", false)
}

func evt1TypeIdentity(t EVT1Type) string {
	if t.PointerTo != nil {
		return "ptr_" + evt1TypeIdentity(*t.PointerTo)
	}
	if t.ArrayElem != nil {
		return fmt.Sprintf("array_%d_%s", t.ArrayLength, evt1TypeIdentity(*t.ArrayElem))
	}
	if len(t.TypeArgs) > 0 {
		var parts []string
		for _, arg := range t.TypeArgs {
			parts = append(parts, evt1TypeIdentity(arg))
		}
		return evt1CName(t.Name)[len("concept_vulkan_"):] + "_" + strings.Join(parts, "_")
	}
	return evt1CName(t.Name)[len("concept_vulkan_"):]
}

func evt1TemplateInstanceSymbol(templateName string, concreteType EVT1Type) string {
	return "concept_vulkan_template_" + evt1CName(templateName)[len("concept_vulkan_"):] + "__" + evt1TypeIdentity(concreteType)
}

func evt1InstantiateTemplateFunction(templateDecl EVT1TemplateDecl, concreteType EVT1Type) (EVT1FunctionDecl, error) {
	body, err := evt1SubstituteBlock(*templateDecl.Body, templateDecl.TypeParam, concreteType)
	if err != nil {
		return EVT1FunctionDecl{}, err
	}
	fn := EVT1FunctionDecl{
		Name:       templateDecl.Name,
		ReturnType: evt1SubstituteType(templateDecl.ReturnType, templateDecl.TypeParam, concreteType),
		Span:       templateDecl.Span,
		Body:       &body,
	}
	for _, param := range templateDecl.Params {
		fn.Params = append(fn.Params, EVT1Param{
			Type: evt1SubstituteType(param.Type, templateDecl.TypeParam, concreteType),
			Name: param.Name,
			Span: param.Span,
		})
	}
	return fn, nil
}

func evt1SubstituteBlock(block EVT1Block, typeParam string, concreteType EVT1Type) (EVT1Block, error) {
	out := EVT1Block{Span: block.Span}
	for _, stmt := range block.Statements {
		sub, err := evt1SubstituteStatement(stmt, typeParam, concreteType)
		if err != nil {
			return EVT1Block{}, err
		}
		out.Statements = append(out.Statements, sub)
	}
	return out, nil
}

func evt1SubstituteStatement(stmt EVT1Statement, typeParam string, concreteType EVT1Type) (EVT1Statement, error) {
	switch s := stmt.(type) {
	case *EVT1VarDecl:
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1VarDecl{Type: evt1SubstituteType(s.Type, typeParam, concreteType), Name: s.Name, Value: value, Span: s.Span}, nil
	case *EVT1EffectsDecl:
		return &EVT1EffectsDecl{AutomataName: s.AutomataName, Name: s.Name, Span: s.Span}, nil
	case *EVT1InstanceDecl:
		return &EVT1InstanceDecl{AutomataName: s.AutomataName, Name: s.Name, Span: s.Span}, nil
	case *EVT1AssignStmt:
		target, err := evt1SubstituteExpr(s.Target, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1AssignStmt{Target: target, Value: value, Span: s.Span}, nil
	case *EVT1ReturnStmt:
		if s.Value == nil {
			return &EVT1ReturnStmt{Span: s.Span}, nil
		}
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1ReturnStmt{Value: value, Span: s.Span}, nil
	case *EVT1ExprStmt:
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1ExprStmt{Value: value, Span: s.Span}, nil
	case *EVT1MatchStmt:
		subject, err := evt1SubstituteExpr(s.Subject, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		out := &EVT1MatchStmt{Subject: subject, Span: s.Span}
		for _, arm := range s.Arms {
			block, err := evt1SubstituteBlock(arm.Block, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Arms = append(out.Arms, EVT1StatementArm{Pattern: arm.Pattern, Block: block, Span: arm.Span})
		}
		return out, nil
	case *EVT1Block:
		block, err := evt1SubstituteBlock(*s, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &block, nil
	default:
		return nil, evt1Diagnostic("CV4180", "unsupported template statement during instantiation", stmt.statementSpan())
	}
}

func evt1SubstituteExpr(expr EVT1Expr, typeParam string, concreteType EVT1Type) (EVT1Expr, error) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		return &EVT1NameExpr{Name: e.Name, Span: e.Span}, nil
	case *EVT1IntLiteral:
		return &EVT1IntLiteral{Value: e.Value, Span: e.Span}, nil
	case *EVT1BoolLiteral:
		return &EVT1BoolLiteral{Value: e.Value, Span: e.Span}, nil
	case *EVT1FieldExpr:
		receiver, err := evt1SubstituteExpr(e.Receiver, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1FieldExpr{Receiver: receiver, Field: e.Field, Span: e.Span}, nil
	case *EVT1CallExpr:
		out := &EVT1CallExpr{Callee: e.Callee, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1DispatchExpr:
		signal, err := evt1SubstituteExpr(e.Signal, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1DispatchExpr{InstanceName: e.InstanceName, Signal: signal, BatchName: e.BatchName, Span: e.Span}, nil
	case *EVT1TemplateCallExpr:
		return nil, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", e.Span)
	case *EVT1BinaryExpr:
		left, err := evt1SubstituteExpr(e.Left, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		right, err := evt1SubstituteExpr(e.Right, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1BinaryExpr{Op: e.Op, Left: left, Right: right, Span: e.Span}, nil
	case *EVT1ConstructExpr:
		out := &EVT1ConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1StructConstructExpr:
		out := &EVT1StructConstructExpr{StructName: e.StructName, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1MatchExpr:
		subject, err := evt1SubstituteExpr(e.Subject, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		out := &EVT1MatchExpr{Subject: subject, Span: e.Span}
		for _, arm := range e.Arms {
			value, err := evt1SubstituteExpr(arm.Value, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Arms = append(out.Arms, EVT1ExprArm{Pattern: arm.Pattern, Value: value, Span: arm.Span})
		}
		return out, nil
	default:
		return nil, evt1Diagnostic("CV4181", "unsupported template expression during instantiation", expr.exprSpan())
	}
}
