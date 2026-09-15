package build

import (
	"fmt"
	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/project"
	"sort"
)

type compiledExpressionContext struct {
	program     project.Program
	pkg         project.Package
	flowName    string
	anonymousID int
	functions   []MIRFunction
}

var activeFlowExpressionContext *compiledExpressionContext

func lowerFlow(program project.Program, pkgName string, flow ast.FlowDecl, pkg project.Package) (MIRFlow, []MIRFunction, error) {
	previousExpressionContext := activeFlowExpressionContext
	expressionContext := &compiledExpressionContext{program: program, pkg: pkg, flowName: flow.Name}
	activeFlowExpressionContext = expressionContext
	defer func() { activeFlowExpressionContext = previousExpressionContext }()
	env := map[string]string{}
	locals := map[string]bool{}
	boardFieldTypes := map[string]string{}
	for _, p := range flow.Parameters {
		env[p.Name] = typeRefStringForPackage(pkgName, p.Type)
	}
	if flow.TurnInput != nil {
		env[flow.TurnInput.Name] = typeRefStringForPackage(pkgName, flow.TurnInput.Type)
	}
	out := MIRFlow{
		Package:    pkgName,
		Name:       flow.Name,
		Return:     typeRefStringForPackage(pkgName, flow.ReturnType),
		EntryState: flow.EntryState,
	}
	if flow.TurnInput != nil {
		out.TurnInput = &MIRField{Name: flow.TurnInput.Name, Type: typeRefStringForPackage(pkgName, flow.TurnInput.Type)}
	}
	if flow.YieldType != nil {
		out.YieldType = typeRefStringForPackage(pkgName, *flow.YieldType)
	}
	for _, p := range flow.Parameters {
		out.Parameters = append(out.Parameters, MIRField{Name: p.Name, Type: typeRefStringForPackage(pkgName, p.Type)})
	}
	for _, field := range flow.Board {
		fieldType := typeRefStringForPackage(pkgName, field.Type)
		out.Board = append(out.Board, MIRField{Name: field.Name, Type: fieldType})
		boardFieldTypes[field.Name] = fieldType
	}
	if len(flow.Board) > 0 {
		env["board"] = "__flow_board_" + flow.Name
	}
	for _, st := range flow.States {
		stateEnv := cloneFlowEnv(env)
		stateLocals := cloneFlowLocals(locals)
		lowered, err := lowerFlowBlock(st.Body, stateEnv, stateLocals, pkg.Name, boardFieldTypes)
		if err != nil {
			return MIRFlow{}, nil, fmt.Errorf("state %s: %w", st.Name, err)
		}
		out.States = append(out.States, MIRFlowState{Name: st.Name, Statements: lowered})
	}
	return out, expressionContext.functions, nil
}

func cloneFlowEnv(env map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	return out
}

func cloneFlowLocals(locals map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, v := range locals {
		out[k] = v
	}
	return out
}

func lowerFlowBlock(block ast.Block, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) ([]MIRFlowStmt, error) {
	out := make([]MIRFlowStmt, 0, len(block.Statements))
	for _, stmt := range block.Statements {
		s, err := lowerFlowStmt(stmt, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
		if astStmtContainsYield(stmt) {
			for name := range locals {
				delete(env, name)
			}
			clear(locals)
		}
	}
	return out, nil
}

func astStmtContainsYield(stmt ast.Stmt) bool {
	switch node := stmt.(type) {
	case ast.YieldStmt:
		return true
	case ast.IfStmt:
		if astBlockContainsYield(node.ThenBody) {
			return true
		}
		return node.ElseBody != nil && astBlockContainsYield(*node.ElseBody)
	case ast.WhenStmt:
		for _, c := range node.Cases {
			if astWhenActionContainsYield(c.Action) {
				return true
			}
		}
		return astWhenActionContainsYield(node.Else)
	default:
		return false
	}
}

func astBlockContainsYield(block ast.Block) bool {
	for _, stmt := range block.Statements {
		if astStmtContainsYield(stmt) {
			return true
		}
	}
	return false
}

func astWhenActionContainsYield(action ast.WhenAction) bool {
	block, ok := action.(ast.WhenBlockAction)
	return ok && astBlockContainsYield(ast.Block{Statements: block.Statements})
}

func lowerFlowStmt(stmt ast.Stmt, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowStmt, error) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if _, exists := env[s.Name]; exists {
			return nil, fmt.Errorf("flow local '%s' conflicts with existing binding", s.Name)
		}
		v, t, fallible, err := lowerFlowExprTyped(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		if fallible {
			return nil, fmt.Errorf("fallible calls are not supported in compiled flow let bindings; handle outside the flow or use non-fallible helper")
		}
		if s.TypeHint != nil {
			hint := typeRefStringForPackage(pkg, *s.TypeHint)
			if hint != t {
				return nil, fmt.Errorf("flow let '%s' expected %s, got %s", s.Name, hint, t)
			}
		}
		env[s.Name] = t
		locals[s.Name] = true
		return MIRFlowLetStmt{Name: s.Name, Type: t, Value: v}, nil
	case ast.VarStmt:
		if _, exists := env[s.Name]; exists {
			return nil, fmt.Errorf("flow local '%s' conflicts with existing binding", s.Name)
		}
		v, t, fallible, err := lowerFlowExprTyped(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		if fallible {
			return nil, fmt.Errorf("fallible calls are not supported in compiled flow var bindings; handle with match or use '!'")
		}
		if s.TypeHint != nil {
			hint := typeRefStringForPackage(pkg, *s.TypeHint)
			if hint != t {
				return nil, fmt.Errorf("flow var '%s' expected %s, got %s", s.Name, hint, t)
			}
		}
		env[s.Name] = t
		locals[s.Name] = true
		return MIRFlowLetStmt{Name: s.Name, Type: t, Value: v}, nil
	case ast.AssignStmt:
		if !locals[s.Name] {
			return nil, fmt.Errorf("flow assignment target '%s' is not a state local", s.Name)
		}
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowLocalAssign{Name: s.Name, Value: v}, nil
	case ast.GotoStmt:
		return MIRFlowGoto{Target: s.Target}, nil
	case ast.SuspendStmt:
		return MIRFlowSuspend{}, nil
	case ast.YieldStmt:
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowYield{Value: v}, nil
	case ast.RememberStmt:
		return MIRFlowRemember{}, nil
	case ast.ResumeStmt:
		return MIRFlowResume{}, nil
	case ast.FieldAssignStmt:
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowFieldAssign{Target: s.Target, Field: s.Field, Value: v}, nil
	case ast.FieldIndexAssignStmt:
		indices := make([]MIRFlowExpr, 0, len(s.Indices))
		for _, index := range s.Indices {
			lowered, err := lowerFlowExpr(index, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			indices = append(indices, lowered)
		}
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowFieldIndexAssign{Target: s.Target, Field: s.Field, Indices: indices, Value: v}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return MIRFlowReturn{}, nil
		}
		v, err := lowerFlowExpr(s.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowReturn{Value: v}, nil
	case ast.IfStmt:
		cond, err := lowerFlowExpr(s.Condition, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		thenBody, err := lowerFlowBlock(s.ThenBody, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		var elseBody []MIRFlowStmt
		if s.ElseBody != nil {
			elseBody, err = lowerFlowBlock(*s.ElseBody, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
		}
		return MIRFlowIf{Condition: cond, Then: thenBody, Else: elseBody}, nil
	case ast.WhileStmt:
		cond, err := lowerFlowExpr(s.Condition, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		body, err := lowerFlowBlock(s.Body, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhile{Condition: cond, Body: body}, nil
	case ast.ForStmt:
		rangeExpr, ok := s.Range.(ast.RangeExpr)
		if !ok || rangeExpr.End == nil {
			return nil, unsupported("compiled flow for loops require a bounded range")
		}
		startExpr := rangeExpr.Start
		if startExpr == nil {
			startExpr = ast.IntegerLiteral{Value: "0"}
		}
		stepExpr := rangeExpr.Step
		if s.Direction == ast.ForDirectionDesc && s.DescendStep != nil {
			stepExpr = s.DescendStep
		}
		if stepExpr == nil {
			stepExpr = ast.IntegerLiteral{Value: "1"}
		}
		start, err := lowerFlowExpr(startExpr, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		end, err := lowerFlowExpr(rangeExpr.End, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		step, err := lowerFlowExpr(stepExpr, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		bodyEnv := cloneFlowEnv(env)
		bodyLocals := cloneFlowLocals(locals)
		bodyEnv[s.Name] = "Int"
		bodyLocals[s.Name] = true
		body, err := lowerFlowBlock(s.Body, bodyEnv, bodyLocals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowFor{Name: s.Name, Start: start, End: end, Step: step, Descending: s.Direction == ast.ForDirectionDesc, Body: body}, nil
	case ast.MatchStmt:
		call, ok := s.Subject.(ast.CallExpr)
		if !ok {
			return nil, unsupported("compiled FLOW fallible match currently requires a direct fallible call")
		}
		subject, resultType, err := lowerFallibleFlowCall(call, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		okEnv, okLocals := cloneFlowEnv(env), cloneFlowLocals(locals)
		okEnv[s.OkName], okLocals[s.OkName] = resultType, true
		okBody, err := lowerFlowBlock(s.OkBody, okEnv, okLocals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		errEnv, errLocals := cloneFlowEnv(env), cloneFlowLocals(locals)
		errEnv[s.ErrName], errLocals[s.ErrName] = "Error", true
		errBody, err := lowerFlowBlock(s.ErrBody, errEnv, errLocals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowFallibleMatch{Subject: subject, ResultType: resultType, OkName: s.OkName, OkBody: okBody, ErrName: s.ErrName, ErrBody: errBody}, nil
	case ast.WhenStmt:
		cases := make([]MIRFlowWhenCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			cond, err := lowerFlowExpr(c.Condition, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			action, err := lowerFlowWhenAction(c.Action, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowWhenCase{Condition: cond, Action: action})
		}
		elseAction, err := lowerFlowWhenAction(s.Else, cloneFlowEnv(env), cloneFlowLocals(locals), pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhen{Cases: cases, Else: elseAction}, nil
	case ast.ExprStmt:
		if activeFlowExpressionContext != nil && activeFlowExpressionContext.program.Profile == "Verilog" {
			return nil, unsupported("effectful/discarded FLOW expression statements; native/Octxiliary, filesystem, network, and process effects are not hardware-admissible")
		}
		return nil, unsupported(fmt.Sprintf("flow statement %T", stmt))
	default:
		return nil, unsupported(fmt.Sprintf("flow statement %T", stmt))
	}
}

func lowerFlowWhenAction(action ast.WhenAction, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowWhenAction, error) {
	switch a := action.(type) {
	case ast.WhenGotoAction:
		return MIRFlowWhenGoto{Target: a.Target}, nil
	case ast.WhenSuspendAction:
		return MIRFlowWhenSuspend{}, nil
	case ast.WhenReturnAction:
		v, err := lowerFlowExpr(a.Value, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		return MIRFlowWhenReturn{Value: v}, nil
	case ast.WhenBlockAction:
		statements := make([]MIRFlowStmt, 0, len(a.Statements))
		for _, statement := range a.Statements {
			lowered, err := lowerFlowStmt(statement, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			statements = append(statements, lowered)
		}
		return MIRFlowWhenBlock{Statements: statements}, nil
	default:
		return nil, unsupported(fmt.Sprintf("flow when action %T", action))
	}
}

func flowExpressionType(expr MIRFlowExpr) (string, bool, error) {
	switch value := expr.(type) {
	case MIRFlowSharedExpr:
		return value.Type, value.Fallible, nil
	case MIRFlowUtilityWhenExpr:
		return value.ResultType, false, nil
	default:
		return "", false, fmt.Errorf("internal error: unsupported FLOW expression representation %T", expr)
	}
}

func lowerFlowExprTyped(expr ast.Expr, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowExpr, string, bool, error) {
	value, err := lowerFlowExpr(expr, env, locals, pkg, boardFieldTypes)
	if err != nil {
		return nil, "", false, err
	}
	typ, fallible, err := flowExpressionType(value)
	if err != nil {
		return nil, "", false, err
	}
	return value, typ, fallible, nil
}

func lowerFallibleFlowCall(call ast.CallExpr, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowExpr, string, error) {
	sharedExpr, err := lowerSharedFlowExpression(call, env, locals)
	if err != nil {
		return nil, "", err
	}
	shared := sharedExpr.(MIRFlowSharedExpr)
	if !shared.Fallible {
		return nil, "", fmt.Errorf("operator '!' requires fallible expression")
	}
	return shared, shared.Type, nil
}

func lowerFlowExpr(expr ast.Expr, env map[string]string, locals map[string]bool, pkg string, boardFieldTypes map[string]string) (MIRFlowExpr, error) {
	switch expression := expr.(type) {
	case ast.UtilityWhenExpr:
		if expression.EnumTarget != nil && utilityWhenHasPayloadCandidate(expression) {
			return nil, unsupported("compiled enum-targeted utility payload candidates require delayed payload lowering")
		}
		hysteresis, err := lowerFlowExpr(expression.Policy.Hysteresis, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		minCommit, err := lowerFlowExpr(expression.Policy.MinCommit, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		cases := make([]MIRFlowUtilityCase, 0, len(expression.Cases))
		for _, candidate := range expression.Cases {
			value, err := lowerFlowExpr(candidate.Value, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			condition, err := lowerFlowExpr(candidate.Condition, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			score, err := lowerFlowExpr(candidate.Score, env, locals, pkg, boardFieldTypes)
			if err != nil {
				return nil, err
			}
			cases = append(cases, MIRFlowUtilityCase{Value: value, Condition: condition, Score: score})
		}
		elseExpr, err := lowerFlowExpr(expression.Else, env, locals, pkg, boardFieldTypes)
		if err != nil {
			return nil, err
		}
		resultType, fallible, err := flowExpressionType(elseExpr)
		if err != nil {
			return nil, err
		}
		if fallible {
			return nil, fmt.Errorf("utility when result cannot be fallible")
		}
		return MIRFlowUtilityWhenExpr{
			SiteID:          expression.SiteID,
			ControllerBound: expression.ControllerBound,
			ResultType:      resultType,
			Hysteresis:      hysteresis,
			MinCommit:       minCommit,
			Cases:           cases,
			Else:            elseExpr,
		}, nil
	case ast.PropagateExpr:
		return nil, fmt.Errorf("error propagation with '?' requires a fallible FLOW result contract; handle the error with match or use '!'")
	default:
		return lowerSharedFlowExpression(expr, env, locals)
	}
}

func lowerSharedFlowExpression(expr ast.Expr, env map[string]string, flowLocals map[string]bool) (MIRFlowExpr, error) {
	shared := activeFlowExpressionContext
	if shared == nil {
		return nil, fmt.Errorf("internal error: missing compiled FLOW expression context")
	}
	ordinaryLocals := make(map[string]string, len(env))
	goNames := make(map[string]string, len(env))
	for name, typ := range env {
		ordinaryLocals[name] = typ
		switch {
		case flowLocals[name]:
			goNames[name] = goIdent(name)
		case name == "board":
			goNames[name] = "f.board"
		default:
			goNames[name] = "f." + goIdent(name)
		}
	}
	ctx := &lowerCtx{
		pkg:         shared.pkg,
		program:     shared.program,
		locals:      ordinaryLocals,
		goNames:     goNames,
		blocks:      []MIRBlock{{Label: "entry"}},
		cur:         0,
		retType:     "Void",
		fn:          ast.FunctionDecl{Name: shared.flowName + "_state_expression"},
		anonymousID: shared.anonymousID,
		einTerms:    map[string]einsteinTermMeta{},
	}
	value, typ, fallible, err := ctx.lowerExpr(expr)
	if err != nil {
		return nil, err
	}
	ctx.blocks[ctx.cur].Terminator = MIRReturn{Value: lowerMIRValue(value, typ)}
	shared.anonymousID = ctx.anonymousID
	shared.functions = append(shared.functions, ctx.extra...)

	resultLocals := make([]MIRField, 0)
	for name, localType := range ctx.locals {
		if _, external := env[name]; external {
			continue
		}
		resultLocals = append(resultLocals, MIRField{Name: ctx.goLocalName(name), Type: localType})
	}
	sort.Slice(resultLocals, func(i, j int) bool { return resultLocals[i].Name < resultLocals[j].Name })
	return MIRFlowSharedExpr{Type: typ, Fallible: fallible, Locals: resultLocals, Blocks: ctx.blocks}, nil
}
