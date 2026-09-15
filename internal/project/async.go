package project

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

type asyncSignature struct {
	pkg, name string
	result    ast.TypeRef
}

// lowerAsyncFunctions erases the surface sugar into ordinary FLOW before the
// existing type checker, interpreter, MIR, and compiled backend see it.
func lowerAsyncFunctions(program Program) (Program, error) {
	asyncs := map[string]asyncSignature{}
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if fn.IsAsync {
				asyncs[pkgName+"."+fn.Name] = asyncSignature{pkg: pkgName, name: fn.Name, result: fn.ReturnType}
			}
		}
	}
	if err := rejectAsyncCycles(asyncs, program); err != nil {
		return Program{}, err
	}
	pkgNames := make([]string, 0, len(program.Packages))
	for name := range program.Packages {
		pkgNames = append(pkgNames, name)
	}
	sort.Strings(pkgNames)
	for _, pkgName := range pkgNames {
		pkg := program.Packages[pkgName]
		kept := make([]ast.FunctionDecl, 0, len(pkg.Functions))
		for _, fn := range pkg.Functions {
			if !fn.IsAsync {
				if pos, ok := firstAwaitInBlock(fn.Body); ok {
					return Program{}, fmt.Errorf("%s:%d:%d: await is only valid inside an async fn", fn.SourcePath, pos[0], pos[1])
				}
				kept = append(kept, fn)
				continue
			}
			flow, err := lowerAsyncFunction(pkgName, pkg, fn, asyncs, program)
			if err != nil {
				return Program{}, err
			}
			pkg.Flows = append(pkg.Flows, flow)
		}
		pkg.Functions = kept
		program.Packages[pkgName] = pkg
	}
	return program, nil
}

func lowerAsyncFunction(pkgName string, pkg Package, fn ast.FunctionDecl, asyncs map[string]asyncSignature, program Program) (ast.FlowDecl, error) {
	if fn.IsTemplate || len(fn.TypeParameters) > 0 {
		return ast.FlowDecl{}, asyncError(fn, 0, 0, "generic async functions are deferred in ASYNC-M0")
	}
	if fn.IsFallible {
		return ast.FlowDecl{}, asyncError(fn, 0, 0, "fallible async functions are deferred in ASYNC-M0; async does not change Oct error propagation")
	}
	if fn.IsFact || fn.IsTheory || fn.IsArtifact || fn.IsBenchmark || fn.IsMakePlan {
		return ast.FlowDecl{}, asyncError(fn, 0, 0, "async entry-point attributes are deferred in ASYNC-M0")
	}
	if pos, ok := awaitInsideLoop(fn.Body); ok {
		return ast.FlowDecl{}, asyncError(fn, pos[0], pos[1], "await inside a loop is deferred in ASYNC-M0")
	}
	analysis, err := analyzeAsyncLocals(pkgName, pkg, fn, asyncs, program)
	if err != nil {
		return ast.FlowDecl{}, err
	}
	b := asyncFlowBuilder{pkgName: pkgName, pkg: pkg, program: program, fn: fn, asyncs: asyncs, lifted: analysis.lifted, states: []ast.StateDecl{{Name: "Start"}}}
	for _, name := range analysis.order {
		if analysis.lifted[name] {
			b.board = append(b.board, ast.BoardField{Name: asyncLocalField(name), Type: analysis.types[name]})
			b.info.LiftedLocals = append(b.info.LiftedLocals, name)
		}
	}
	if err := b.lowerStatements(fn.Body.Statements); err != nil {
		return ast.FlowDecl{}, err
	}
	if fn.ReturnType.Name != "Void" && !asyncBlockAlwaysReturns(fn.Body) {
		return ast.FlowDecl{}, asyncError(fn, 0, 0, "async function is missing a return statement")
	}
	if fn.ReturnType.Name == "Void" && !b.terminated {
		b.append(ast.ReturnStmt{})
	}
	return ast.FlowDecl{Name: fn.Name, Parameters: fn.Parameters, ReturnType: fn.ReturnType, Board: b.board, States: b.states, EntryState: "Start", AsyncLowering: &b.info}, nil
}

type asyncAnalysis struct {
	types  map[string]ast.TypeRef
	lifted map[string]bool
	order  []string
}

func analyzeAsyncLocals(pkgName string, pkg Package, fn ast.FunctionDecl, asyncs map[string]asyncSignature, program Program) (asyncAnalysis, error) {
	decl, lastUse := map[string]int{}, map[string]int{}
	awaits, order, position := []int{}, []string{}, 0
	var walkExpr func(ast.Expr)
	walkExpr = func(expr ast.Expr) {
		switch x := expr.(type) {
		case ast.IdentifierExpr:
			lastUse[x.Name] = position * 2
		case ast.AwaitExpr:
			awaits = append(awaits, position*2)
			walkExpr(x.Inner)
		case ast.CallExpr:
			walkExpr(x.Callee)
			for _, a := range x.Arguments {
				walkExpr(a)
			}
		case ast.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case ast.UnaryExpr:
			walkExpr(x.Operand)
		case ast.ParenExpr:
			walkExpr(x.Inner)
		case ast.PropagateExpr:
			walkExpr(x.Inner)
		case ast.UnwrapExpr:
			walkExpr(x.Inner)
		case ast.FieldAccessExpr:
			walkExpr(x.Target)
		case ast.IndexExpr:
			walkExpr(x.Target)
			for _, i := range x.Indices {
				walkExpr(i)
			}
		case ast.IfExpr:
			walkExpr(x.Condition)
			walkExpr(x.ThenExpr)
			walkExpr(x.ElseExpr)
		}
	}
	var walkBlock func(ast.Block) error
	walkBlock = func(block ast.Block) error {
		for _, stmt := range block.Statements {
			position++
			switch x := stmt.(type) {
			case ast.LetStmt:
				if _, exists := decl[x.Name]; exists {
					return asyncError(fn, 0, 0, fmt.Sprintf("async local %q shadows another local; shadowing is deferred in ASYNC-M0", x.Name))
				}
				walkExpr(x.Value)
				decl[x.Name] = position*2 + 1
				order = append(order, x.Name)
			case ast.VarStmt:
				if _, exists := decl[x.Name]; exists {
					return asyncError(fn, 0, 0, fmt.Sprintf("async local %q shadows another local; shadowing is deferred in ASYNC-M0", x.Name))
				}
				walkExpr(x.Value)
				decl[x.Name] = position*2 + 1
				order = append(order, x.Name)
			case ast.AssignStmt:
				walkExpr(x.Value)
				lastUse[x.Name] = position * 2
			case ast.ReturnStmt:
				walkExpr(x.Value)
			case ast.ExprStmt:
				walkExpr(x.Value)
			case ast.IfStmt:
				walkExpr(x.Condition)
				if err := walkBlock(x.ThenBody); err != nil {
					return err
				}
				if x.ElseBody != nil {
					if err := walkBlock(*x.ElseBody); err != nil {
						return err
					}
				}
			case ast.WhileStmt:
				walkExpr(x.Condition)
				if err := walkBlock(x.Body); err != nil {
					return err
				}
			case ast.ForStmt:
				walkExpr(x.Range)
				if err := walkBlock(x.Body); err != nil {
					return err
				}
			default:
				return asyncError(fn, 0, 0, fmt.Sprintf("statement %T is not supported in async fn in ASYNC-M0", stmt))
			}
		}
		return nil
	}
	if err := walkBlock(fn.Body); err != nil {
		return asyncAnalysis{}, err
	}
	lifted := map[string]bool{}
	for name, d := range decl {
		for _, a := range awaits {
			if d <= a && a < lastUse[name] {
				lifted[name] = true
				break
			}
		}
	}
	env, types := map[string]ast.TypeRef{}, map[string]ast.TypeRef{}
	for _, p := range fn.Parameters {
		env[p.Name] = p.Type
	}
	var inferBlock func(ast.Block) error
	inferBlock = func(block ast.Block) error {
		for _, stmt := range block.Statements {
			switch x := stmt.(type) {
			case ast.LetStmt:
				t, err := inferAsyncExprType(pkgName, pkg, x.Value, env, asyncs, program)
				if err != nil {
					return asyncError(fn, 0, 0, fmt.Sprintf("let %s: %v", x.Name, err))
				}
				if x.TypeHint != nil {
					t = *x.TypeHint
				}
				env[x.Name], types[x.Name] = t, t
			case ast.VarStmt:
				t, err := inferAsyncExprType(pkgName, pkg, x.Value, env, asyncs, program)
				if err != nil {
					return asyncError(fn, 0, 0, fmt.Sprintf("var %s: %v", x.Name, err))
				}
				if x.TypeHint != nil {
					t = *x.TypeHint
				}
				env[x.Name], types[x.Name] = t, t
			case ast.IfStmt:
				if err := inferBlock(x.ThenBody); err != nil {
					return err
				}
				if x.ElseBody != nil {
					if err := inferBlock(*x.ElseBody); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := inferBlock(fn.Body); err != nil {
		return asyncAnalysis{}, err
	}
	return asyncAnalysis{types: types, lifted: lifted, order: order}, nil
}

func inferAsyncExprType(pkgName string, pkg Package, expr ast.Expr, env map[string]ast.TypeRef, asyncs map[string]asyncSignature, program Program) (ast.TypeRef, error) {
	switch x := expr.(type) {
	case ast.AwaitExpr:
		call, ok := x.Inner.(ast.CallExpr)
		if !ok {
			if _, stored := x.Inner.(ast.IdentifierExpr); !stored {
				return ast.TypeRef{}, fmt.Errorf("await requires an async function or FLOW call; this expression is not awaitable")
			}
			return ast.TypeRef{}, fmt.Errorf("await expects a direct async function call; stored awaitable handles are deferred in ASYNC-M0")
		}
		result, _, _, ok := resolveAwaitCall(pkgName, pkg, call, asyncs, program)
		if !ok {
			return ast.TypeRef{}, fmt.Errorf("await requires an async function or FLOW call; this expression is not awaitable")
		}
		return result, nil
	case ast.IntegerLiteral:
		return ast.TypeRef{Name: "Int", Dimension: x.Dimension, HasUnit: x.HasUnit}, nil
	case ast.FloatLiteral:
		return ast.TypeRef{Name: "Float", Dimension: x.Dimension, HasUnit: x.HasUnit}, nil
	case ast.BoolLiteral:
		return ast.TypeRef{Name: "Bool"}, nil
	case ast.StringLiteralExpr:
		return ast.TypeRef{Name: "String"}, nil
	case ast.IdentifierExpr:
		if t, ok := env[x.Name]; ok {
			return t, nil
		}
		return ast.TypeRef{}, fmt.Errorf("unknown binding %q", x.Name)
	case ast.ParenExpr:
		return inferAsyncExprType(pkgName, pkg, x.Inner, env, asyncs, program)
	case ast.UnaryExpr:
		return inferAsyncExprType(pkgName, pkg, x.Operand, env, asyncs, program)
	case ast.BinaryExpr:
		if strings.Contains(" == != < <= > >= and or ", " "+x.Operator+" ") {
			return ast.TypeRef{Name: "Bool"}, nil
		}
		return inferAsyncExprType(pkgName, pkg, x.Left, env, asyncs, program)
	case ast.CallExpr:
		if sig, _, ok := resolveAsyncCall(pkgName, x, asyncs); ok {
			r := sig.result
			return ast.TypeRef{FlowInstanceOf: &r, FlowIdentity: sig.pkg + "." + sig.name}, nil
		}
		name := directCallName(x.Callee)
		for _, f := range pkg.Functions {
			if f.Name == name {
				return f.ReturnType, nil
			}
		}
		for _, f := range pkg.Flows {
			if f.Name == name {
				r := f.ReturnType
				return ast.TypeRef{FlowInstanceOf: &r, FlowIdentity: pkgName + "." + f.Name}, nil
			}
		}
		return ast.TypeRef{}, fmt.Errorf("cannot infer result type of call %q; add an explicit local type annotation", name)
	default:
		return ast.TypeRef{}, fmt.Errorf("cannot infer async local type from %T; add an explicit type annotation", expr)
	}
}

type asyncFlowBuilder struct {
	pkgName                    string
	pkg                        Package
	program                    Program
	fn                         ast.FunctionDecl
	asyncs                     map[string]asyncSignature
	lifted                     map[string]bool
	board                      []ast.BoardField
	states                     []ast.StateDecl
	current, awaitID, branchID int
	terminated                 bool
	info                       ast.AsyncLoweringInfo
}

func (b *asyncFlowBuilder) append(stmt ast.Stmt) {
	b.states[b.current].Body.Statements = append(b.states[b.current].Body.Statements, stmt)
}
func (b *asyncFlowBuilder) addState(name string) int {
	b.states = append(b.states, ast.StateDecl{Name: name})
	return len(b.states) - 1
}

func (b *asyncFlowBuilder) lowerStatements(statements []ast.Stmt) error {
	for _, stmt := range statements {
		if b.terminated {
			break
		}
		switch x := stmt.(type) {
		case ast.LetStmt:
			if aw, ok := x.Value.(ast.AwaitExpr); ok {
				if err := b.lowerAwaitBinding(x.Name, x.TypeHint, false, aw); err != nil {
					return err
				}
				continue
			}
			value, err := b.rewriteExpr(x.Value)
			if err != nil {
				return err
			}
			if b.lifted[x.Name] {
				b.append(ast.FieldAssignStmt{Target: "board", Field: asyncLocalField(x.Name), Value: value})
			} else {
				x.Value = value
				b.append(x)
			}
		case ast.VarStmt:
			if aw, ok := x.Value.(ast.AwaitExpr); ok {
				if err := b.lowerAwaitBinding(x.Name, x.TypeHint, true, aw); err != nil {
					return err
				}
				continue
			}
			value, err := b.rewriteExpr(x.Value)
			if err != nil {
				return err
			}
			if b.lifted[x.Name] {
				b.append(ast.FieldAssignStmt{Target: "board", Field: asyncLocalField(x.Name), Value: value})
			} else {
				x.Value = value
				b.append(x)
			}
		case ast.AssignStmt:
			if aw, ok := x.Value.(ast.AwaitExpr); ok {
				return asyncError(b.fn, aw.Line, aw.Column, "await assignment is deferred in ASYNC-M0; bind the result with let")
			}
			value, err := b.rewriteExpr(x.Value)
			if err != nil {
				return err
			}
			if b.lifted[x.Name] {
				b.append(ast.FieldAssignStmt{Target: "board", Field: asyncLocalField(x.Name), Value: value})
			} else {
				x.Value = value
				b.append(x)
			}
		case ast.ReturnStmt:
			if _, ok := x.Value.(ast.AwaitExpr); ok {
				return asyncError(b.fn, 0, 0, "return await is deferred in ASYNC-M0; bind the awaited result first")
			}
			value, err := b.rewriteExpr(x.Value)
			if err != nil {
				return err
			}
			x.Value = value
			b.append(x)
			b.terminated = true
		case ast.ExprStmt:
			value, err := b.rewriteExpr(x.Value)
			if err != nil {
				return err
			}
			x.Value = value
			b.append(x)
		case ast.IfStmt:
			if blockContainsAwait(x.ThenBody) || (x.ElseBody != nil && blockContainsAwait(*x.ElseBody)) {
				if err := b.lowerAwaitIf(x); err != nil {
					return err
				}
			} else {
				y, err := b.rewriteIf(x)
				if err != nil {
					return err
				}
				b.append(y)
			}
		case ast.WhileStmt:
			condition, err := b.rewriteExpr(x.Condition)
			if err != nil {
				return err
			}
			body, err := b.rewriteBlock(x.Body)
			if err != nil {
				return err
			}
			x.Condition, x.Body = condition, body
			b.append(x)
		case ast.ForStmt:
			rangeExpr, err := b.rewriteExpr(x.Range)
			if err != nil {
				return err
			}
			body, err := b.rewriteBlock(x.Body)
			if err != nil {
				return err
			}
			x.Range, x.Body = rangeExpr, body
			b.append(x)
		default:
			return asyncError(b.fn, 0, 0, fmt.Sprintf("statement %T is not supported in async fn in ASYNC-M0", stmt))
		}
	}
	return nil
}

func (b *asyncFlowBuilder) lowerAwaitBinding(name string, hint *ast.TypeRef, mutable bool, aw ast.AwaitExpr) error {
	call, ok := aw.Inner.(ast.CallExpr)
	if !ok {
		return asyncError(b.fn, aw.Line, aw.Column, "await expects a direct async function call; stored awaitable handles are deferred in ASYNC-M0")
	}
	result, identity, rewrittenCall, ok := resolveAwaitCall(b.pkgName, b.pkg, call, b.asyncs, b.program)
	if !ok {
		return asyncError(b.fn, aw.Line, aw.Column, "await requires an async function or FLOW call; this expression is not awaitable")
	}
	for i, arg := range rewrittenCall.Arguments {
		v, err := b.rewriteExpr(arg)
		if err != nil {
			return err
		}
		rewrittenCall.Arguments[i] = v
	}
	id := b.awaitID
	b.awaitID++
	handle, awaitState := fmt.Sprintf("Await%d", id), fmt.Sprintf("Await%d", id)
	suspendState, continueState := fmt.Sprintf("SuspendAwait%d", id), fmt.Sprintf("ContinueAfterAwait%d", id)
	b.board = append(b.board, ast.BoardField{Name: handle, Type: ast.TypeRef{FlowInstanceOf: &result, FlowIdentity: identity}, AsyncHandle: true})
	b.append(ast.FieldAssignStmt{Target: "board", Field: handle, Value: rewrittenCall})
	b.append(ast.GotoStmt{Target: awaitState})
	b.current = b.addState(awaitState)
	field := ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "board"}, Field: handle}
	b.append(ast.ExprStmt{Value: ast.CallExpr{Callee: ast.IdentifierExpr{Name: "Step"}, Arguments: []ast.Expr{field}}})
	b.append(ast.IfStmt{Condition: ast.CallExpr{Callee: ast.IdentifierExpr{Name: "Complete"}, Arguments: []ast.Expr{field}}, ThenBody: ast.Block{Statements: []ast.Stmt{ast.GotoStmt{Target: continueState}}}})
	b.append(ast.GotoStmt{Target: suspendState})
	b.current = b.addState(suspendState)
	b.append(ast.SuspendStmt{})
	b.append(ast.GotoStmt{Target: awaitState})
	b.current = b.addState(continueState)
	resultExpr := ast.UnwrapExpr{Inner: ast.CallExpr{Callee: ast.IdentifierExpr{Name: "Result"}, Arguments: []ast.Expr{field}}}
	if b.lifted[name] {
		b.append(ast.FieldAssignStmt{Target: "board", Field: asyncLocalField(name), Value: resultExpr})
	} else if mutable {
		b.append(ast.VarStmt{Name: name, TypeHint: hint, Value: resultExpr})
	} else {
		b.append(ast.LetStmt{Name: name, TypeHint: hint, Value: resultExpr})
	}
	b.info.Continuations = append(b.info.Continuations, awaitState, continueState)
	return nil
}

func (b *asyncFlowBuilder) lowerAwaitIf(x ast.IfStmt) error {
	condition, err := b.rewriteExpr(x.Condition)
	if err != nil {
		return err
	}
	id := b.branchID
	b.branchID++
	thenName, elseName, joinName := fmt.Sprintf("If%dThen", id), fmt.Sprintf("If%dElse", id), fmt.Sprintf("ContinueAfterIf%d", id)
	b.append(ast.IfStmt{Condition: condition, ThenBody: ast.Block{Statements: []ast.Stmt{ast.GotoStmt{Target: thenName}}}, ElseBody: &ast.Block{Statements: []ast.Stmt{ast.GotoStmt{Target: elseName}}}})
	b.current = b.addState(thenName)
	b.terminated = false
	if err := b.lowerStatements(x.ThenBody.Statements); err != nil {
		return err
	}
	thenTerm := b.terminated
	if !thenTerm {
		b.append(ast.GotoStmt{Target: joinName})
	}
	b.current = b.addState(elseName)
	b.terminated = false
	if x.ElseBody != nil {
		if err := b.lowerStatements(x.ElseBody.Statements); err != nil {
			return err
		}
	}
	elseTerm := b.terminated
	if !elseTerm {
		b.append(ast.GotoStmt{Target: joinName})
	}
	if thenTerm && elseTerm {
		b.terminated = true
		return nil
	}
	b.current = b.addState(joinName)
	b.terminated = false
	return nil
}

func (b *asyncFlowBuilder) rewriteIf(x ast.IfStmt) (ast.IfStmt, error) {
	var err error
	x.Condition, err = b.rewriteExpr(x.Condition)
	if err != nil {
		return x, err
	}
	x.ThenBody, err = b.rewriteBlock(x.ThenBody)
	if err != nil {
		return x, err
	}
	if x.ElseBody != nil {
		v, e := b.rewriteBlock(*x.ElseBody)
		if e != nil {
			return x, e
		}
		x.ElseBody = &v
	}
	return x, nil
}
func (b *asyncFlowBuilder) rewriteBlock(block ast.Block) (ast.Block, error) {
	for i, stmt := range block.Statements {
		switch x := stmt.(type) {
		case ast.LetStmt:
			v, e := b.rewriteExpr(x.Value)
			if e != nil {
				return block, e
			}
			x.Value = v
			block.Statements[i] = x
		case ast.VarStmt:
			v, e := b.rewriteExpr(x.Value)
			if e != nil {
				return block, e
			}
			x.Value = v
			block.Statements[i] = x
		case ast.AssignStmt:
			v, e := b.rewriteExpr(x.Value)
			if e != nil {
				return block, e
			}
			x.Value = v
			block.Statements[i] = x
		case ast.ReturnStmt:
			v, e := b.rewriteExpr(x.Value)
			if e != nil {
				return block, e
			}
			x.Value = v
			block.Statements[i] = x
		case ast.ExprStmt:
			v, e := b.rewriteExpr(x.Value)
			if e != nil {
				return block, e
			}
			x.Value = v
			block.Statements[i] = x
		case ast.IfStmt:
			v, e := b.rewriteIf(x)
			if e != nil {
				return block, e
			}
			block.Statements[i] = v
		case ast.WhileStmt:
			v, e := b.rewriteExpr(x.Condition)
			if e != nil {
				return block, e
			}
			body, e := b.rewriteBlock(x.Body)
			if e != nil {
				return block, e
			}
			x.Condition, x.Body = v, body
			block.Statements[i] = x
		case ast.ForStmt:
			v, e := b.rewriteExpr(x.Range)
			if e != nil {
				return block, e
			}
			body, e := b.rewriteBlock(x.Body)
			if e != nil {
				return block, e
			}
			x.Range, x.Body = v, body
			block.Statements[i] = x
		}
	}
	return block, nil
}

func (b *asyncFlowBuilder) rewriteExpr(expr ast.Expr) (ast.Expr, error) {
	if expr == nil {
		return nil, nil
	}
	switch x := expr.(type) {
	case ast.AwaitExpr:
		return nil, asyncError(b.fn, x.Line, x.Column, "await is supported only as the complete right-hand side of a let/var binding in ASYNC-M0")
	case ast.IdentifierExpr:
		if b.lifted[x.Name] {
			return ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "board"}, Field: asyncLocalField(x.Name)}, nil
		}
		return x, nil
	case ast.CallExpr:
		v, e := b.rewriteExpr(x.Callee)
		if e != nil {
			return nil, e
		}
		x.Callee = v
		for i, a := range x.Arguments {
			v, e = b.rewriteExpr(a)
			if e != nil {
				return nil, e
			}
			x.Arguments[i] = v
		}
		return x, nil
	case ast.BinaryExpr:
		l, e := b.rewriteExpr(x.Left)
		if e != nil {
			return nil, e
		}
		r, e := b.rewriteExpr(x.Right)
		if e != nil {
			return nil, e
		}
		x.Left, x.Right = l, r
		return x, nil
	case ast.UnaryExpr:
		v, e := b.rewriteExpr(x.Operand)
		x.Operand = v
		return x, e
	case ast.ParenExpr:
		v, e := b.rewriteExpr(x.Inner)
		x.Inner = v
		return x, e
	case ast.PropagateExpr:
		return nil, asyncError(b.fn, 0, 0, "async error propagation is deferred in ASYNC-M0")
	case ast.UnwrapExpr:
		v, e := b.rewriteExpr(x.Inner)
		x.Inner = v
		return x, e
	case ast.FieldAccessExpr:
		v, e := b.rewriteExpr(x.Target)
		x.Target = v
		return x, e
	case ast.IndexExpr:
		v, e := b.rewriteExpr(x.Target)
		if e != nil {
			return nil, e
		}
		x.Target = v
		for i, a := range x.Indices {
			v, e = b.rewriteExpr(a)
			if e != nil {
				return nil, e
			}
			x.Indices[i] = v
		}
		return x, nil
	case ast.IfExpr:
		c, e := b.rewriteExpr(x.Condition)
		if e != nil {
			return nil, e
		}
		t, e := b.rewriteExpr(x.ThenExpr)
		if e != nil {
			return nil, e
		}
		f, e := b.rewriteExpr(x.ElseExpr)
		x.Condition, x.ThenExpr, x.ElseExpr = c, t, f
		return x, e
	default:
		return expr, nil
	}
}

func resolveAsyncCall(pkgName string, call ast.CallExpr, asyncs map[string]asyncSignature) (asyncSignature, ast.CallExpr, bool) {
	name := directCallName(call.Callee)
	key := pkgName + "." + name
	if strings.Contains(name, ".") {
		key = name
	}
	sig, ok := asyncs[key]
	return sig, call, ok
}

func resolveAwaitCall(pkgName string, pkg Package, call ast.CallExpr, asyncs map[string]asyncSignature, program Program) (ast.TypeRef, string, ast.CallExpr, bool) {
	if sig, rewritten, ok := resolveAsyncCall(pkgName, call, asyncs); ok {
		return sig.result, sig.pkg + "." + sig.name, rewritten, true
	}
	name := directCallName(call.Callee)
	if !strings.Contains(name, ".") {
		for _, flow := range pkg.Flows {
			if flow.Name == name {
				return flow.ReturnType, pkgName + "." + flow.Name, call, true
			}
		}
	} else {
		parts := strings.SplitN(name, ".", 2)
		if imported, ok := program.Packages[parts[0]]; ok {
			for _, flow := range imported.Flows {
				if flow.Name == parts[1] {
					return flow.ReturnType, name, call, true
				}
			}
		}
	}
	return ast.TypeRef{}, "", call, false
}
func directCallName(expr ast.Expr) string {
	switch x := expr.(type) {
	case ast.IdentifierExpr:
		return x.Name
	case ast.FieldAccessExpr:
		if p, ok := x.Target.(ast.IdentifierExpr); ok {
			return p.Name + "." + x.Field
		}
	}
	return ""
}
func asyncLocalField(name string) string { return "Local_" + name }
func asyncError(fn ast.FunctionDecl, line, column int, message string) error {
	if line > 0 {
		return fmt.Errorf("%s:%d:%d: async fn %s: %s", fn.SourcePath, line, column, fn.Name, message)
	}
	return fmt.Errorf("async fn %s: %s", fn.Name, message)
}

func blockContainsAwait(block ast.Block) bool { _, ok := firstAwaitInBlock(block); return ok }
func asyncBlockAlwaysReturns(block ast.Block) bool {
	for _, stmt := range block.Statements {
		switch x := stmt.(type) {
		case ast.ReturnStmt:
			return true
		case ast.IfStmt:
			if x.ElseBody != nil && asyncBlockAlwaysReturns(x.ThenBody) && asyncBlockAlwaysReturns(*x.ElseBody) {
				return true
			}
		}
	}
	return false
}
func firstAwaitInBlock(block ast.Block) ([2]int, bool) {
	for _, s := range block.Statements {
		if p, ok := firstAwaitInStmt(s); ok {
			return p, true
		}
	}
	return [2]int{}, false
}
func firstAwaitInStmt(stmt ast.Stmt) ([2]int, bool) {
	switch x := stmt.(type) {
	case ast.LetStmt:
		return firstAwaitInExpr(x.Value)
	case ast.VarStmt:
		return firstAwaitInExpr(x.Value)
	case ast.AssignStmt:
		return firstAwaitInExpr(x.Value)
	case ast.ReturnStmt:
		return firstAwaitInExpr(x.Value)
	case ast.ExprStmt:
		return firstAwaitInExpr(x.Value)
	case ast.IfStmt:
		if p, ok := firstAwaitInExpr(x.Condition); ok {
			return p, true
		}
		if p, ok := firstAwaitInBlock(x.ThenBody); ok {
			return p, true
		}
		if x.ElseBody != nil {
			return firstAwaitInBlock(*x.ElseBody)
		}
	case ast.WhileStmt:
		if p, ok := firstAwaitInExpr(x.Condition); ok {
			return p, true
		}
		return firstAwaitInBlock(x.Body)
	case ast.ForStmt:
		if p, ok := firstAwaitInExpr(x.Range); ok {
			return p, true
		}
		return firstAwaitInBlock(x.Body)
	}
	return [2]int{}, false
}
func firstAwaitInExpr(expr ast.Expr) ([2]int, bool) {
	switch x := expr.(type) {
	case ast.AwaitExpr:
		return [2]int{x.Line, x.Column}, true
	case ast.CallExpr:
		for _, a := range x.Arguments {
			if p, ok := firstAwaitInExpr(a); ok {
				return p, true
			}
		}
	case ast.BinaryExpr:
		if p, ok := firstAwaitInExpr(x.Left); ok {
			return p, true
		}
		return firstAwaitInExpr(x.Right)
	case ast.UnaryExpr:
		return firstAwaitInExpr(x.Operand)
	case ast.ParenExpr:
		return firstAwaitInExpr(x.Inner)
	case ast.PropagateExpr:
		return firstAwaitInExpr(x.Inner)
	case ast.UnwrapExpr:
		return firstAwaitInExpr(x.Inner)
	case ast.IfExpr:
		if p, ok := firstAwaitInExpr(x.Condition); ok {
			return p, true
		}
		if p, ok := firstAwaitInExpr(x.ThenExpr); ok {
			return p, true
		}
		return firstAwaitInExpr(x.ElseExpr)
	}
	return [2]int{}, false
}
func awaitInsideLoop(block ast.Block) ([2]int, bool) {
	for _, s := range block.Statements {
		switch x := s.(type) {
		case ast.WhileStmt:
			if p, ok := firstAwaitInBlock(x.Body); ok {
				return p, true
			}
		case ast.ForStmt:
			if p, ok := firstAwaitInBlock(x.Body); ok {
				return p, true
			}
		case ast.IfStmt:
			if p, ok := awaitInsideLoop(x.ThenBody); ok {
				return p, true
			}
			if x.ElseBody != nil {
				if p, ok := awaitInsideLoop(*x.ElseBody); ok {
					return p, true
				}
			}
		}
	}
	return [2]int{}, false
}

func rejectAsyncCycles(asyncs map[string]asyncSignature, program Program) error {
	edges := map[string][]string{}
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if !fn.IsAsync {
				continue
			}
			from := pkgName + "." + fn.Name
			collectAsyncCalls(fn.Body, pkgName, asyncs, func(to string) { edges[from] = append(edges[from], to) })
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(n string) error {
		if visiting[n] {
			return fmt.Errorf("async recursion cycle involving %s is not supported in ASYNC-M0", n)
		}
		if done[n] {
			return nil
		}
		visiting[n] = true
		for _, m := range edges[n] {
			if err := visit(m); err != nil {
				return err
			}
		}
		delete(visiting, n)
		done[n] = true
		return nil
	}
	keys := make([]string, 0, len(asyncs))
	for k := range asyncs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := visit(k); err != nil {
			return err
		}
	}
	return nil
}
func collectAsyncCalls(block ast.Block, pkg string, asyncs map[string]asyncSignature, add func(string)) {
	var expr func(ast.Expr)
	expr = func(e ast.Expr) {
		switch x := e.(type) {
		case ast.AwaitExpr:
			expr(x.Inner)
		case ast.CallExpr:
			if sig, _, ok := resolveAsyncCall(pkg, x, asyncs); ok {
				add(sig.pkg + "." + sig.name)
			}
			for _, a := range x.Arguments {
				expr(a)
			}
		case ast.BinaryExpr:
			expr(x.Left)
			expr(x.Right)
		case ast.UnaryExpr:
			expr(x.Operand)
		case ast.ParenExpr:
			expr(x.Inner)
		}
	}
	for _, s := range block.Statements {
		switch x := s.(type) {
		case ast.LetStmt:
			expr(x.Value)
		case ast.VarStmt:
			expr(x.Value)
		case ast.AssignStmt:
			expr(x.Value)
		case ast.ReturnStmt:
			expr(x.Value)
		case ast.ExprStmt:
			expr(x.Value)
		case ast.IfStmt:
			expr(x.Condition)
			collectAsyncCalls(x.ThenBody, pkg, asyncs, add)
			if x.ElseBody != nil {
				collectAsyncCalls(*x.ElseBody, pkg, asyncs, add)
			}
		case ast.WhileStmt:
			collectAsyncCalls(x.Body, pkg, asyncs, add)
		case ast.ForStmt:
			collectAsyncCalls(x.Body, pkg, asyncs, add)
		}
	}
}
