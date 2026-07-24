package conceptvulkan

import (
	"fmt"
	"strings"
)

const (
	evt1ComptimeMaxFuel            = 4096
	evt1ComptimeMaxLoopBound       = 256
	evt1ComptimeMaxCallDepth       = 32
	evt1ComptimeMaxArrayLength     = 64
	evt1ComptimeMaxArrayNesting    = 8
	evt1ComptimeMaxArrayCells      = 512
	evt1ComptimeMaxLiteralElements = 512
)

type evt1EvalBinding struct {
	value    EVT1Value
	mutable  bool
	comptime bool
}

type evt1EvalScope struct {
	parent *evt1EvalScope
	values map[string]evt1EvalBinding
}

func newEVT1EvalScope(parent *evt1EvalScope) *evt1EvalScope {
	return &evt1EvalScope{parent: parent, values: map[string]evt1EvalBinding{}}
}

func (s *evt1EvalScope) declare(name string, binding evt1EvalBinding) {
	s.values[name] = binding
}

func (s *evt1EvalScope) lookup(name string) (evt1EvalBinding, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if binding, ok := scope.values[name]; ok {
			return binding, true
		}
	}
	return evt1EvalBinding{}, false
}

func (s *evt1EvalScope) assign(name string, value EVT1Value) bool {
	for scope := s; scope != nil; scope = scope.parent {
		if binding, ok := scope.values[name]; ok {
			binding.value = value
			scope.values[name] = binding
			return true
		}
	}
	return false
}

type evt1ComptimeState struct {
	env            *evt1Env
	fuel           int
	callDepth      int
	stack          []string
	globalState    map[string]string
	staticMessages map[*EVT1StaticAssert]string
}

func newEVT1ComptimeState(env *evt1Env) *evt1ComptimeState {
	return &evt1ComptimeState{
		env:            env,
		fuel:           evt1ComptimeMaxFuel,
		globalState:    map[string]string{},
		staticMessages: map[*EVT1StaticAssert]string{},
	}
}

func (s *evt1ComptimeState) push(frame string) error {
	s.stack = append(s.stack, frame)
	if len(s.stack) > evt1ComptimeMaxCallDepth {
		return evt1Diagnostic("CV4211", "comptime call depth exceeded at "+strings.Join(s.stack, " -> "), Span{})
	}
	return nil
}

func (s *evt1ComptimeState) pop() {
	if len(s.stack) > 0 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

func (s *evt1ComptimeState) spend(span Span, cost int) error {
	s.fuel -= cost
	if s.fuel < 0 {
		path := strings.Join(s.stack, " -> ")
		if path == "" {
			path = "<root>"
		}
		return evt1Diagnostic("CV4204", "comptime fuel exhausted along "+path, span)
	}
	return nil
}

func evt1IsComptimeType(env *evt1Env, t EVT1Type) bool {
	if t.PointerTo != nil || t.isBorrowLike() || t.isOwned() || len(t.TypeArgs) > 0 {
		return false
	}
	if t.ArrayElem != nil {
		return evt1IsComptimeType(env, *t.ArrayElem)
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		return t.Name == "int" || t.Name == "bool" || t.Name == "string"
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		for _, field := range structDecl.Fields {
			if !evt1IsComptimeType(env, field.Type) {
				return false
			}
		}
		return true
	}
	if enumDecl, ok := env.enums[t.Name]; ok {
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if !evt1IsComptimeType(env, field.Type) {
					return false
				}
			}
		}
		return true
	}
	return false
}

func evt1SeedComptimeScope(env *evt1Env) *evt1EvalScope {
	scope := newEVT1EvalScope(nil)
	for name, value := range env.comptimeValues {
		scope.declare(name, evt1EvalBinding{value: value, mutable: false, comptime: true})
	}
	return scope
}

func evt1EvaluateModuleComptime(env *evt1Env, module EVT1Module) error {
	state := newEVT1ComptimeState(env)
	for _, decl := range module.ComptimeDecls {
		if _, err := evt1EvaluateGlobalComptimeDecl(state, decl); err != nil {
			return err
		}
	}
	for i := range module.StaticAsserts {
		if err := evt1EvaluateStaticAssert(state, evt1SeedComptimeScope(env), &module.StaticAsserts[i]); err != nil {
			return err
		}
	}
	return nil
}

func evt1EvaluateGlobalComptimeDecl(state *evt1ComptimeState, decl EVT1ComptimeDecl) (EVT1Value, error) {
	if value, ok := state.env.comptimeValues[decl.Name]; ok {
		return value, nil
	}
	switch state.globalState[decl.Name] {
	case "evaluating":
		return EVT1Value{}, evt1Diagnostic("CV4209", "comptime declaration cycle involving "+decl.Name, decl.Span)
	case "done":
		return state.env.comptimeValues[decl.Name], nil
	}
	state.globalState[decl.Name] = "evaluating"
	defer func() {
		if state.globalState[decl.Name] == "evaluating" {
			state.globalState[decl.Name] = "done"
		}
	}()
	if err := state.push("comptime " + decl.Name); err != nil {
		return EVT1Value{}, err
	}
	defer state.pop()
	scope := evt1SeedComptimeScope(state.env)
	resolvedType, err := evt1ResolveType(state.env, nil, decl.Type)
	if err != nil {
		return EVT1Value{}, err
	}
	value, err := evt1EvalExprTyped(state, scope, decl.Value, &resolvedType)
	if err != nil {
		return EVT1Value{}, err
	}
	if !evt1CanonicalType(state.env, resolvedType).Equal(evt1CanonicalType(state.env, value.Type)) {
		return EVT1Value{}, evt1Diagnostic("CV4202", fmt.Sprintf("comptime declaration %s expected %s but got %s", decl.Name, resolvedType.String(), value.Type.String()), decl.Span)
	}
	state.env.comptimeValues[decl.Name] = value
	return value, nil
}

func evt1EvaluateStaticAssert(state *evt1ComptimeState, scope *evt1EvalScope, assertion *EVT1StaticAssert) error {
	if err := state.push("static_assert"); err != nil {
		return err
	}
	defer state.pop()
	condition, err := evt1EvalExpr(state, scope, assertion.Condition)
	if err != nil {
		return err
	}
	if condition.Kind != EVT1ValueBool {
		return evt1Diagnostic("CV4207", "static_assert condition must evaluate to bool", assertion.Condition.exprSpan())
	}
	message := ""
	if assertion.Message != nil {
		value, err := evt1EvalExpr(state, scope, assertion.Message)
		if err != nil {
			return err
		}
		if value.Kind != EVT1ValueString {
			return evt1Diagnostic("CV4208", "static_assert message must evaluate to string", assertion.Message.exprSpan())
		}
		message = value.StringValue
	}
	if !condition.BoolValue {
		if message != "" {
			return evt1Diagnostic("CV4207", "static_assert failed: "+message, assertion.Span)
		}
		return evt1Diagnostic("CV4207", "static_assert failed", assertion.Span)
	}
	return nil
}

func evt1EvalExpr(state *evt1ComptimeState, scope *evt1EvalScope, expr EVT1Expr) (EVT1Value, error) {
	return evt1EvalExprTyped(state, scope, expr, nil)
}

func evt1EvalExprTyped(state *evt1ComptimeState, scope *evt1EvalScope, expr EVT1Expr, expected *EVT1Type) (EVT1Value, error) {
	if err := state.spend(expr.exprSpan(), 1); err != nil {
		return EVT1Value{}, err
	}
	switch e := expr.(type) {
	case *EVT1ParenExpr:
		return evt1EvalExprTyped(state, scope, e.Value, expected)
	case *EVT1IntLiteral:
		t, _ := evt1BuiltinType("int", e.Span)
		return EVT1Value{Kind: EVT1ValueInt, Type: t, IntValue: e.Value}, nil
	case *EVT1BoolLiteral:
		t, _ := evt1BuiltinType("bool", e.Span)
		return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: e.Value}, nil
	case *EVT1StringLiteral:
		t, _ := evt1BuiltinType("string", e.Span)
		return EVT1Value{Kind: EVT1ValueString, Type: t, StringValue: e.Value}, nil
	case *EVT1NameExpr:
		if binding, ok := scope.lookup(e.Name); ok && binding.comptime {
			return binding.value, nil
		}
		if decl, ok := state.env.comptimeDecls[e.Name]; ok {
			return evt1EvaluateGlobalComptimeDecl(state, decl)
		}
		return EVT1Value{}, evt1Diagnostic("CV4200", fmt.Sprintf("name %s is not available in comptime evaluation", e.Name), e.Span)
	case *EVT1UnaryExpr:
		value, err := evt1EvalExpr(state, scope, e.Value)
		if err != nil {
			return EVT1Value{}, err
		}
		switch e.Op {
		case "-":
			if value.Kind != EVT1ValueInt {
				return EVT1Value{}, evt1Diagnostic("CV4201", "unary - requires int", e.Span)
			}
			return EVT1Value{Kind: EVT1ValueInt, Type: value.Type, IntValue: -value.IntValue}, nil
		case "not":
			if value.Kind != EVT1ValueBool {
				return EVT1Value{}, evt1Diagnostic("CV4201", "not requires bool", e.Span)
			}
			return EVT1Value{Kind: EVT1ValueBool, Type: value.Type, BoolValue: !value.BoolValue}, nil
		default:
			return EVT1Value{}, evt1Diagnostic("CV4201", "unsupported comptime unary operator "+e.Op, e.Span)
		}
	case *EVT1BinaryExpr:
		return evt1EvalBinaryExpr(state, scope, *e)
	case *EVT1FieldExpr:
		receiver, err := evt1EvalExpr(state, scope, e.Receiver)
		if err != nil {
			return EVT1Value{}, err
		}
		value, ok := receiver.Fields[e.Field]
		if !ok {
			return EVT1Value{}, evt1Diagnostic("CV4201", fmt.Sprintf("field %s is not available in comptime value", e.Field), e.Span)
		}
		return value, nil
	case *EVT1ArrayLiteralExpr:
		return evt1EvalArrayLiteral(state, scope, *e, expected)
	case *EVT1IndexExpr:
		base, err := evt1EvalExpr(state, scope, e.Base)
		if err != nil {
			return EVT1Value{}, err
		}
		if base.Kind != EVT1ValueArray {
			return EVT1Value{}, evt1Diagnostic("CV4231", "indexing requires a fixed compile-time array", e.Base.exprSpan())
		}
		index, err := evt1EvalExpr(state, scope, e.Index)
		if err != nil {
			return EVT1Value{}, err
		}
		if index.Kind != EVT1ValueInt {
			return EVT1Value{}, evt1Diagnostic("CV4232", "array index must evaluate to int", e.Index.exprSpan())
		}
		if index.IntValue < 0 || index.IntValue >= len(base.Elements) {
			return EVT1Value{}, evt1Diagnostic("CV4233", fmt.Sprintf("array index %d is out of range for length %d", index.IntValue, len(base.Elements)), e.Index.exprSpan())
		}
		return base.Elements[index.IntValue], nil
	case *EVT1StructConstructExpr:
		structDecl := state.env.structs[e.StructName]
		fields := map[string]EVT1Value{}
		for i, arg := range e.Args {
			value, err := evt1EvalExpr(state, scope, arg)
			if err != nil {
				return EVT1Value{}, err
			}
			fields[structDecl.Fields[i].Name] = value
		}
		return EVT1Value{
			Kind:       EVT1ValueStruct,
			Type:       EVT1Type{Name: e.StructName, Kind: EVT1TypeStruct, Span: e.Span},
			StructName: e.StructName,
			Fields:     fields,
		}, nil
	case *EVT1ConstructExpr:
		var payload []EVT1Value
		for _, arg := range e.Args {
			value, err := evt1EvalExpr(state, scope, arg)
			if err != nil {
				return EVT1Value{}, err
			}
			payload = append(payload, value)
		}
		return EVT1Value{
			Kind:     EVT1ValueEnum,
			Type:     EVT1Type{Name: e.EnumName, Kind: EVT1TypeEnum, Span: e.Span},
			EnumName: e.EnumName,
			Variant:  e.VariantName,
			Payload:  payload,
		}, nil
	case *EVT1IfExpr:
		condition, err := evt1EvalExpr(state, scope, e.Condition)
		if err != nil {
			return EVT1Value{}, err
		}
		if condition.Kind != EVT1ValueBool {
			return EVT1Value{}, evt1Diagnostic("CV4201", "comptime if condition must evaluate to bool", e.Condition.exprSpan())
		}
		if condition.BoolValue {
			return evt1EvalExpr(state, scope, e.Then)
		}
		return evt1EvalExpr(state, scope, e.Else)
	case *EVT1MatchExpr:
		return evt1EvalMatchExpr(state, scope, *e)
	case *EVT1CallExpr:
		if e.Callee == "Len" {
			if len(e.Args) != 1 {
				return EVT1Value{}, evt1Diagnostic("CV4234", fmt.Sprintf("Len expects exactly one argument, got %d", len(e.Args)), e.Span)
			}
			value, err := evt1EvalExpr(state, scope, e.Args[0])
			if err != nil {
				return EVT1Value{}, err
			}
			if value.Kind != EVT1ValueArray {
				return EVT1Value{}, evt1Diagnostic("CV4235", "Len requires a fixed compile-time array", e.Args[0].exprSpan())
			}
			t, _ := evt1BuiltinType("int", e.Span)
			return EVT1Value{Kind: EVT1ValueInt, Type: t, IntValue: len(value.Elements)}, nil
		}
		return evt1EvalComptimeCall(state, scope, e.Callee, e.Args, e.Span)
	case *EVT1TemplateCallExpr:
		return EVT1Value{}, evt1Diagnostic("CV4201", "templates are not available during comptime evaluation", e.Span)
	default:
		return EVT1Value{}, evt1Diagnostic("CV4201", "unsupported comptime expression", expr.exprSpan())
	}
}

func evt1EvalBinaryExpr(state *evt1ComptimeState, scope *evt1EvalScope, expr EVT1BinaryExpr) (EVT1Value, error) {
	if expr.Op == "and" || expr.Op == "or" {
		left, err := evt1EvalExpr(state, scope, expr.Left)
		if err != nil {
			return EVT1Value{}, err
		}
		if left.Kind != EVT1ValueBool {
			return EVT1Value{}, evt1Diagnostic("CV4201", "logical operators require bool operands", expr.Left.exprSpan())
		}
		if expr.Op == "and" && !left.BoolValue {
			t, _ := evt1BuiltinType("bool", expr.Span)
			return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: false}, nil
		}
		if expr.Op == "or" && left.BoolValue {
			t, _ := evt1BuiltinType("bool", expr.Span)
			return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: true}, nil
		}
		right, err := evt1EvalExpr(state, scope, expr.Right)
		if err != nil {
			return EVT1Value{}, err
		}
		if right.Kind != EVT1ValueBool {
			return EVT1Value{}, evt1Diagnostic("CV4201", "logical operators require bool operands", expr.Right.exprSpan())
		}
		t, _ := evt1BuiltinType("bool", expr.Span)
		if expr.Op == "and" {
			return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: left.BoolValue && right.BoolValue}, nil
		}
		return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: left.BoolValue || right.BoolValue}, nil
	}
	left, err := evt1EvalExpr(state, scope, expr.Left)
	if err != nil {
		return EVT1Value{}, err
	}
	right, err := evt1EvalExpr(state, scope, expr.Right)
	if err != nil {
		return EVT1Value{}, err
	}
	if (left.Kind == EVT1ValueArray || right.Kind == EVT1ValueArray) && (expr.Op == "<" || expr.Op == ">" || expr.Op == "<=" || expr.Op == ">=") {
		return EVT1Value{}, evt1Diagnostic("CV4236", "array ordering comparisons are not supported", expr.Span)
	}
	switch expr.Op {
	case "+", "-", "*", "<", ">", "<=", ">=":
		if left.Kind != EVT1ValueInt || right.Kind != EVT1ValueInt {
			return EVT1Value{}, evt1Diagnostic("CV4201", "integer operator requires int operands", expr.Span)
		}
		switch expr.Op {
		case "+":
			return EVT1Value{Kind: EVT1ValueInt, Type: left.Type, IntValue: left.IntValue + right.IntValue}, nil
		case "-":
			return EVT1Value{Kind: EVT1ValueInt, Type: left.Type, IntValue: left.IntValue - right.IntValue}, nil
		case "*":
			return EVT1Value{Kind: EVT1ValueInt, Type: left.Type, IntValue: left.IntValue * right.IntValue}, nil
		default:
			t, _ := evt1BuiltinType("bool", expr.Span)
			return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: evt1CompareInts(left.IntValue, right.IntValue, expr.Op)}, nil
		}
	case "==", "!=":
		if left.Kind == EVT1ValueArray || right.Kind == EVT1ValueArray {
			if !evt1ValueEqual(left, right) && expr.Op == "==" {
				t, _ := evt1BuiltinType("bool", expr.Span)
				return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: false}, nil
			}
		}
		equal := evt1ValueEqual(left, right)
		t, _ := evt1BuiltinType("bool", expr.Span)
		if expr.Op == "==" {
			return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: equal}, nil
		}
		return EVT1Value{Kind: EVT1ValueBool, Type: t, BoolValue: !equal}, nil
	default:
		return EVT1Value{}, evt1Diagnostic("CV4201", "unsupported comptime operator "+expr.Op, expr.Span)
	}
}

func evt1CompareInts(left, right int, op string) bool {
	switch op {
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	default:
		return false
	}
}

func evt1ValueEqual(left, right EVT1Value) bool {
	if !evt1CanonicalType(nil, left.Type).Equal(evt1CanonicalType(nil, right.Type)) && left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case EVT1ValueInt:
		return right.Kind == EVT1ValueInt && left.IntValue == right.IntValue
	case EVT1ValueBool:
		return right.Kind == EVT1ValueBool && left.BoolValue == right.BoolValue
	case EVT1ValueString:
		return right.Kind == EVT1ValueString && left.StringValue == right.StringValue
	case EVT1ValueStruct:
		if right.Kind != EVT1ValueStruct || left.StructName != right.StructName || len(left.Fields) != len(right.Fields) {
			return false
		}
		for name, value := range left.Fields {
			other, ok := right.Fields[name]
			if !ok || !evt1ValueEqual(value, other) {
				return false
			}
		}
		return true
	case EVT1ValueEnum:
		if right.Kind != EVT1ValueEnum || left.EnumName != right.EnumName || left.Variant != right.Variant || len(left.Payload) != len(right.Payload) {
			return false
		}
		for i := range left.Payload {
			if !evt1ValueEqual(left.Payload[i], right.Payload[i]) {
				return false
			}
		}
		return true
	case EVT1ValueArray:
		if right.Kind != EVT1ValueArray || len(left.Elements) != len(right.Elements) {
			return false
		}
		for i := range left.Elements {
			if !evt1ValueEqual(left.Elements[i], right.Elements[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func evt1EvalMatchExpr(state *evt1ComptimeState, scope *evt1EvalScope, expr EVT1MatchExpr) (EVT1Value, error) {
	subject, err := evt1EvalExpr(state, scope, expr.Subject)
	if err != nil {
		return EVT1Value{}, err
	}
	if subject.Kind != EVT1ValueEnum {
		return EVT1Value{}, evt1Diagnostic("CV4201", "comptime match subject must be an enum value", expr.Subject.exprSpan())
	}
	for _, arm := range expr.Arms {
		if arm.Pattern.EnumName != subject.EnumName || arm.Pattern.VariantName != subject.Variant {
			continue
		}
		armScope := newEVT1EvalScope(scope)
		for i, binding := range arm.Pattern.Bindings {
			armScope.declare(binding, evt1EvalBinding{value: subject.Payload[i], mutable: false, comptime: true})
		}
		return evt1EvalExpr(state, armScope, arm.Value)
	}
	return EVT1Value{}, evt1Diagnostic("CV4201", "comptime match found no selected arm", expr.Span)
}

func evt1EvalComptimeCall(state *evt1ComptimeState, scope *evt1EvalScope, name string, args []EVT1Expr, span Span) (EVT1Value, error) {
	fn, ok := state.env.comptimeFunctions[name]
	if !ok {
		return EVT1Value{}, evt1Diagnostic("CV4210", fmt.Sprintf("runtime function %s cannot be called during comptime evaluation", name), span)
	}
	if len(fn.Params) != len(args) {
		return EVT1Value{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor or call payload count for %s: expected %d but got %d", name, len(fn.Params), len(args)), span)
	}
	if err := state.push("comptime fn " + name); err != nil {
		return EVT1Value{}, err
	}
	defer state.pop()
	callScope := evt1SeedComptimeScope(state.env)
	for i, arg := range args {
		paramType, err := evt1ResolveType(state.env, nil, fn.Params[i].Type)
		if err != nil {
			return EVT1Value{}, err
		}
		value, err := evt1EvalExprTyped(state, scope, arg, &paramType)
		if err != nil {
			return EVT1Value{}, err
		}
		callScope.declare(fn.Params[i].Name, evt1EvalBinding{value: value, mutable: true, comptime: true})
	}
	resolvedReturn, err := evt1ResolveType(state.env, nil, fn.ReturnType)
	if err != nil {
		return EVT1Value{}, err
	}
	result, err := evt1ExecComptimeBlock(state, callScope, *fn.Body, resolvedReturn)
	if err != nil {
		return EVT1Value{}, err
	}
	if result == nil {
		if fn.ReturnType.Name == "void" {
			return EVT1Value{Type: fn.ReturnType}, nil
		}
		return EVT1Value{}, evt1Diagnostic("CV4212", fmt.Sprintf("comptime function %s did not return a value", name), fn.Span)
	}
	return *result, nil
}

func evt1ExecComptimeBlock(state *evt1ComptimeState, scope *evt1EvalScope, block EVT1Block, returnType EVT1Type) (*EVT1Value, error) {
	local := newEVT1EvalScope(scope)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			value, err := evt1EvalExprTyped(state, local, s.Value, &s.Type)
			if err != nil {
				return nil, err
			}
			local.declare(s.Name, evt1EvalBinding{value: value, mutable: !s.Comptime, comptime: true})
		case *EVT1AssignStmt:
			nameExpr, ok := s.Target.(*EVT1NameExpr)
			if !ok {
				return nil, evt1Diagnostic("CV4213", "comptime assignment requires a named local", s.Target.exprSpan())
			}
			binding, ok := local.lookup(nameExpr.Name)
			if !ok || !binding.mutable {
				return nil, evt1Diagnostic("CV4213", fmt.Sprintf("comptime local %s is not mutable", nameExpr.Name), s.Span)
			}
			value, err := evt1EvalExpr(state, local, s.Value)
			if err != nil {
				return nil, err
			}
			local.assign(nameExpr.Name, value)
		case *EVT1ReturnStmt:
			if s.Value == nil {
				return nil, nil
			}
			value, err := evt1EvalExprTyped(state, local, s.Value, &returnType)
			if err != nil {
				return nil, err
			}
			return &value, nil
		case *EVT1ExprStmt:
			if _, err := evt1EvalExpr(state, local, s.Value); err != nil {
				return nil, err
			}
		case *EVT1StaticAssertStmt:
			if err := evt1EvaluateStaticAssert(state, local, &EVT1StaticAssert{Condition: s.Condition, Message: s.Message, Span: s.Span}); err != nil {
				return nil, err
			}
		case *EVT1WhileStmt:
			if s.Bound == nil {
				return nil, evt1Diagnostic("CV4205", "comptime while requires an explicit bounded(limit) clause", s.Span)
			}
			bound, err := evt1EvalExpr(state, local, s.Bound)
			if err != nil {
				return nil, err
			}
			if bound.Kind != EVT1ValueInt || bound.IntValue < 0 {
				return nil, evt1Diagnostic("CV4205", "bounded while requires a non-negative compile-time int bound", s.Bound.exprSpan())
			}
			if bound.IntValue > evt1ComptimeMaxLoopBound {
				return nil, evt1Diagnostic("CV4206", fmt.Sprintf("comptime loop bound %d exceeds limit %d", bound.IntValue, evt1ComptimeMaxLoopBound), s.Bound.exprSpan())
			}
			if bound.IntValue == 0 {
				continue
			}
			if err := state.push(fmt.Sprintf("while[%d]", bound.IntValue)); err != nil {
				return nil, err
			}
			for i := 0; i < bound.IntValue; i++ {
				condition, err := evt1EvalExpr(state, local, s.Condition)
				if err != nil {
					state.pop()
					return nil, err
				}
				if condition.Kind != EVT1ValueBool {
					state.pop()
					return nil, evt1Diagnostic("CV4201", "while condition must evaluate to bool", s.Condition.exprSpan())
				}
				if !condition.BoolValue {
					break
				}
				result, err := evt1ExecComptimeBlock(state, local, s.Body, returnType)
				if err != nil {
					state.pop()
					return nil, err
				}
				if result != nil {
					state.pop()
					return result, nil
				}
			}
			state.pop()
		case *EVT1MatchStmt:
			subject, err := evt1EvalExpr(state, local, s.Subject)
			if err != nil {
				return nil, err
			}
			if subject.Kind != EVT1ValueEnum {
				return nil, evt1Diagnostic("CV4201", "comptime match subject must be an enum value", s.Subject.exprSpan())
			}
			matched := false
			for _, arm := range s.Arms {
				if arm.Pattern.EnumName != subject.EnumName || arm.Pattern.VariantName != subject.Variant {
					continue
				}
				matched = true
				armScope := newEVT1EvalScope(local)
				for i, binding := range arm.Pattern.Bindings {
					armScope.declare(binding, evt1EvalBinding{value: subject.Payload[i], mutable: false, comptime: true})
				}
				result, err := evt1ExecComptimeBlock(state, armScope, arm.Block, returnType)
				if err != nil {
					return nil, err
				}
				if result != nil {
					return result, nil
				}
				break
			}
			if !matched {
				return nil, evt1Diagnostic("CV4201", "comptime match found no selected arm", s.Span)
			}
		case *EVT1Block:
			result, err := evt1ExecComptimeBlock(state, local, *s, returnType)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		default:
			return nil, evt1Diagnostic("CV4201", "unsupported comptime statement", stmt.statementSpan())
		}
	}
	return nil, nil
}

func evt1EvalArrayLiteral(state *evt1ComptimeState, scope *evt1EvalScope, expr EVT1ArrayLiteralExpr, expected *EVT1Type) (EVT1Value, error) {
	var arrayType EVT1Type
	if expected != nil && expected.ArrayElem != nil {
		arrayType = expected.valueType()
		if len(expr.Elements) != arrayType.ArrayLength {
			return EVT1Value{}, evt1Diagnostic("CV4226", fmt.Sprintf("array literal expected %d elements but got %d", arrayType.ArrayLength, len(expr.Elements)), expr.Span)
		}
	} else if len(expr.Elements) == 0 {
		return EVT1Value{}, evt1Diagnostic("CV4225", "empty array literal requires an explicit fixed-array type", expr.Span)
	}
	if len(expr.Elements) > evt1ComptimeMaxLiteralElements {
		return EVT1Value{}, evt1Diagnostic("CV4224", fmt.Sprintf("array literal element count %d exceeds limit %d", len(expr.Elements), evt1ComptimeMaxLiteralElements), expr.Span)
	}
	elements := make([]EVT1Value, 0, len(expr.Elements))
	for i, element := range expr.Elements {
		var elemExpected *EVT1Type
		if arrayType.ArrayElem != nil {
			elemExpected = arrayType.ArrayElem
		}
		value, err := evt1EvalExprTyped(state, scope, element, elemExpected)
		if err != nil {
			return EVT1Value{}, err
		}
		if i == 0 && arrayType.ArrayElem == nil {
			elemType := value.Type.valueType()
			arrayType = EVT1Type{
				Name:        elemType.String() + "[]",
				Kind:        EVT1TypeArray,
				ArrayElem:   &elemType,
				ArrayLength: len(expr.Elements),
				Span:        expr.Span,
			}
		}
		if arrayType.ArrayElem != nil && !arrayType.ArrayElem.valueType().Equal(value.Type.valueType()) {
			return EVT1Value{}, evt1Diagnostic("CV4227", fmt.Sprintf("array literal element %d expected %s but got %s", i+1, arrayType.ArrayElem.String(), value.Type.String()), element.exprSpan())
		}
		elements = append(elements, value)
	}
	return EVT1Value{Kind: EVT1ValueArray, Type: arrayType, Elements: elements}, nil
}
