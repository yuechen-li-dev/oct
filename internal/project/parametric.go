package project

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// elaborateParametrics performs the deliberately bounded PARAMETRICS-M0 pass.
// Templates are authoring declarations only: every application is cloned with
// exact type substitution into an ordinary declaration in the consuming
// package. The existing typechecker, interpreter, FLOW lowering, and Go backend
// never receive an open type parameter or template declaration.
func elaborateParametrics(program Program) (Program, error) {
	started := time.Now()
	e := parametricElaborator{
		program:           &program,
		recordTemplates:   map[string]ast.RecordDecl{},
		functionTemplates: map[string]ast.FunctionDecl{},
		flowTemplates:     map[string]ast.FlowDecl{},
		instantiations:    map[string]string{},
		selectors:         map[string]string{},
	}
	if err := e.collectTemplates(); err != nil {
		return Program{}, err
	}
	if err := e.processConcreteDeclarations(); err != nil {
		return Program{}, err
	}
	program.Parametrics.ElaborationNanoseconds = time.Since(started).Nanoseconds()
	return program, nil
}

type parametricElaborator struct {
	program           *Program
	recordTemplates   map[string]ast.RecordDecl
	functionTemplates map[string]ast.FunctionDecl
	flowTemplates     map[string]ast.FlowDecl
	instantiations    map[string]string
	selectors         map[string]string
}

func templateKey(pkg, name string) string { return pkg + "." + name }

func (e *parametricElaborator) collectTemplates() error {
	for pkgName, pkg := range e.program.Packages {
		records := pkg.Records[:0]
		for _, decl := range pkg.Records {
			if !decl.IsTemplate {
				records = append(records, decl)
				continue
			}
			key := templateKey(pkgName, decl.Name)
			if _, exists := e.recordTemplates[key]; exists {
				return fmt.Errorf("duplicate template record '%s'", key)
			}
			e.recordTemplates[key] = decl
			e.program.Parametrics.TemplateRecords++
		}
		pkg.Records = records
		functions := pkg.Functions[:0]
		for _, decl := range pkg.Functions {
			if !decl.IsTemplate {
				functions = append(functions, decl)
				continue
			}
			key := templateKey(pkgName, decl.Name)
			if _, exists := e.functionTemplates[key]; exists {
				return fmt.Errorf("duplicate template function '%s'", key)
			}
			e.functionTemplates[key] = decl
			e.program.Parametrics.TemplateFunctions++
		}
		pkg.Functions = functions
		flows := pkg.Flows[:0]
		for _, decl := range pkg.Flows {
			if !decl.IsTemplate {
				flows = append(flows, decl)
				continue
			}
			key := templateKey(pkgName, decl.Name)
			if _, exists := e.flowTemplates[key]; exists {
				return fmt.Errorf("duplicate template flow/query '%s'", key)
			}
			e.flowTemplates[key] = decl
			e.program.Parametrics.TemplateFlows++
		}
		pkg.Flows = flows
		e.program.Packages[pkgName] = pkg
	}
	return nil
}

func (e *parametricElaborator) processConcreteDeclarations() error {
	processedRecords, processedFunctions, processedFlows := map[string]int{}, map[string]int{}, map[string]int{}
	for {
		progress := false
		packageNames := make([]string, 0, len(e.program.Packages))
		for name := range e.program.Packages {
			packageNames = append(packageNames, name)
		}
		sort.Strings(packageNames)
		for _, pkgName := range packageNames {
			pkg := e.program.Packages[pkgName]
			for processedRecords[pkgName] < len(pkg.Records) {
				i := processedRecords[pkgName]
				decl, err := e.rewriteRecord(pkgName, pkg.Records[i], nil)
				if err != nil {
					return err
				}
				pkg = e.program.Packages[pkgName]
				pkg.Records[i] = decl
				processedRecords[pkgName]++
				progress = true
			}
			e.program.Packages[pkgName] = pkg
			pkg = e.program.Packages[pkgName]
			for processedFunctions[pkgName] < len(pkg.Functions) {
				i := processedFunctions[pkgName]
				decl, err := e.rewriteFunction(pkgName, pkg.Functions[i], nil)
				if err != nil {
					return err
				}
				pkg = e.program.Packages[pkgName]
				pkg.Functions[i] = decl
				processedFunctions[pkgName]++
				progress = true
			}
			e.program.Packages[pkgName] = pkg
			pkg = e.program.Packages[pkgName]
			for processedFlows[pkgName] < len(pkg.Flows) {
				i := processedFlows[pkgName]
				decl, err := e.rewriteFlow(pkgName, pkg.Flows[i], nil)
				if err != nil {
					return err
				}
				pkg = e.program.Packages[pkgName]
				pkg.Flows[i] = decl
				processedFlows[pkgName]++
				progress = true
			}
			e.program.Packages[pkgName] = pkg
		}
		if !progress {
			return nil
		}
	}
}

func substitution(parameters []string, arguments []ast.TypeRef, display string) (map[string]ast.TypeRef, error) {
	if len(parameters) != len(arguments) {
		return nil, fmt.Errorf("%s expects %d type arguments, got %d", display, len(parameters), len(arguments))
	}
	result := make(map[string]ast.TypeRef, len(parameters))
	for i, name := range parameters {
		result[name] = arguments[i]
	}
	return result, nil
}

func (e *parametricElaborator) rewriteRecord(pkgName string, decl ast.RecordDecl, subst map[string]ast.TypeRef) (ast.RecordDecl, error) {
	decl.Fields = append([]ast.RecordField(nil), decl.Fields...)
	for i := range decl.Fields {
		t, err := e.rewriteType(pkgName, decl.Fields[i].Type, subst)
		if err != nil {
			return ast.RecordDecl{}, fmt.Errorf("record '%s' field '%s': %w", decl.Name, decl.Fields[i].Name, err)
		}
		decl.Fields[i].Type = t
	}
	return decl, nil
}

func (e *parametricElaborator) rewriteFunction(pkgName string, decl ast.FunctionDecl, subst map[string]ast.TypeRef) (ast.FunctionDecl, error) {
	decl.Parameters = append([]ast.Parameter(nil), decl.Parameters...)
	for i := range decl.Parameters {
		t, err := e.rewriteType(pkgName, decl.Parameters[i].Type, subst)
		if err != nil {
			return ast.FunctionDecl{}, e.contextError(decl.TemplateOrigin, fmt.Sprintf("parameter %s", decl.Parameters[i].Name), err)
		}
		decl.Parameters[i].Type = t
	}
	var err error
	decl.ReturnType, err = e.rewriteType(pkgName, decl.ReturnType, subst)
	if err != nil {
		return ast.FunctionDecl{}, e.contextError(decl.TemplateOrigin, "return type", err)
	}
	if decl.IsFallible {
		decl.ErrorType, err = e.rewriteType(pkgName, decl.ErrorType, subst)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
	}
	decl.Body, err = e.rewriteBlock(pkgName, decl.Body, decl.ReturnType, subst)
	if err != nil {
		return ast.FunctionDecl{}, e.contextError(decl.TemplateOrigin, "body", err)
	}
	return decl, nil
}

func (e *parametricElaborator) rewriteFlow(pkgName string, decl ast.FlowDecl, subst map[string]ast.TypeRef) (ast.FlowDecl, error) {
	var err error
	decl.Parameters = append([]ast.Parameter(nil), decl.Parameters...)
	decl.Board = append([]ast.BoardField(nil), decl.Board...)
	decl.States = append([]ast.StateDecl(nil), decl.States...)
	if decl.TurnInput != nil {
		value := *decl.TurnInput
		decl.TurnInput = &value
	}
	if decl.YieldType != nil {
		value := *decl.YieldType
		decl.YieldType = &value
	}
	for i := range decl.Parameters {
		decl.Parameters[i].Type, err = e.rewriteType(pkgName, decl.Parameters[i].Type, subst)
		if err != nil {
			return ast.FlowDecl{}, e.contextError(decl.TemplateOrigin, fmt.Sprintf("parameter %s", decl.Parameters[i].Name), err)
		}
	}
	if decl.TurnInput != nil {
		decl.TurnInput.Type, err = e.rewriteType(pkgName, decl.TurnInput.Type, subst)
		if err != nil {
			return ast.FlowDecl{}, err
		}
	}
	if decl.YieldType != nil {
		t, typeErr := e.rewriteType(pkgName, *decl.YieldType, subst)
		if typeErr != nil {
			return ast.FlowDecl{}, typeErr
		}
		decl.YieldType = &t
	}
	decl.ReturnType, err = e.rewriteType(pkgName, decl.ReturnType, subst)
	if err != nil {
		return ast.FlowDecl{}, err
	}
	for i := range decl.Board {
		decl.Board[i].Type, err = e.rewriteType(pkgName, decl.Board[i].Type, subst)
		if err != nil {
			return ast.FlowDecl{}, err
		}
	}
	for i := range decl.States {
		decl.States[i].Body, err = e.rewriteBlock(pkgName, decl.States[i].Body, decl.ReturnType, subst)
		if err != nil {
			return ast.FlowDecl{}, e.contextError(decl.TemplateOrigin, "FLOW/query body", err)
		}
	}
	return decl, nil
}

func (e *parametricElaborator) rewriteType(pkgName string, t ast.TypeRef, subst map[string]ast.TypeRef) (ast.TypeRef, error) {
	if subst != nil && t.Package == "" && len(t.TypeArguments) == 0 && t.Function == nil && t.VectorOf == nil && t.MatrixOf == nil {
		if replacement, ok := subst[t.Name]; ok {
			depth := t.ArrayDepth
			if t.IsArray && depth == 0 {
				depth = 1
			}
			replacement.ArrayDepth += depth
			replacement.IsArray = replacement.ArrayDepth > 0
			return replacement, nil
		}
	}
	t.TypeArguments = append([]ast.TypeRef(nil), t.TypeArguments...)
	for i := range t.TypeArguments {
		var err error
		t.TypeArguments[i], err = e.rewriteType(pkgName, t.TypeArguments[i], subst)
		if err != nil {
			return ast.TypeRef{}, err
		}
	}
	if t.VectorOf != nil {
		v, err := e.rewriteType(pkgName, *t.VectorOf, subst)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.VectorOf = &v
	}
	if t.MatrixOf != nil {
		v, err := e.rewriteType(pkgName, *t.MatrixOf, subst)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.MatrixOf = &v
	}
	if t.Function != nil {
		f := *t.Function
		f.Parameters = append([]ast.TypeRef(nil), f.Parameters...)
		for i := range f.Parameters {
			v, err := e.rewriteType(pkgName, f.Parameters[i], subst)
			if err != nil {
				return ast.TypeRef{}, err
			}
			f.Parameters[i] = v
		}
		v, err := e.rewriteType(pkgName, f.ReturnType, subst)
		if err != nil {
			return ast.TypeRef{}, err
		}
		f.ReturnType = v
		if f.ErrorType != nil {
			v, err := e.rewriteType(pkgName, *f.ErrorType, subst)
			if err != nil {
				return ast.TypeRef{}, err
			}
			f.ErrorType = &v
		}
		t.Function = &f
	}
	if t.Name == "Selector" && t.Package == "" {
		if len(t.TypeArguments) != 2 {
			return ast.TypeRef{}, fmt.Errorf("Selector expects 2 type arguments, got %d", len(t.TypeArguments))
		}
		owner, result := t.TypeArguments[0], t.TypeArguments[1]
		if owner.IsArray || owner.Function != nil || owner.VectorOf != nil || owner.MatrixOf != nil {
			return ast.TypeRef{}, fmt.Errorf("Selector owner must be a nominal record type, got %s", displayType(owner))
		}
		return ast.TypeRef{Function: &ast.FunctionTypeRef{Parameters: []ast.TypeRef{owner}, ReturnType: result}, SelectorOwner: &owner, SelectorResult: &result}, nil
	}
	if len(t.TypeArguments) > 0 {
		originPkg := t.Package
		if originPkg == "" {
			originPkg = pkgName
		}
		if _, ok := e.recordTemplates[templateKey(originPkg, t.Name)]; !ok {
			return ast.TypeRef{}, fmt.Errorf("%s is not a template record", displayType(t))
		}
		name, err := e.instantiateRecord(pkgName, originPkg, t.Name, t.TypeArguments)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.Package, t.Name, t.TypeArguments = "", name, nil
	}
	return t, nil
}

func (e *parametricElaborator) instantiateRecord(consumerPkg, originPkg, name string, args []ast.TypeRef) (string, error) {
	decl := e.recordTemplates[templateKey(originPkg, name)]
	return e.instantiate(consumerPkg, originPkg, "record", name, decl.TypeParameters, args, func(concrete string, subst map[string]ast.TypeRef, origin *ast.TemplateOrigin) error {
		e.program.Parametrics.RecordInstantiations++
		clone := decl
		clone.Name, clone.TypeParameters, clone.IsTemplate, clone.TemplateOrigin = concrete, nil, false, origin
		var err error
		clone, err = e.rewriteRecord(consumerPkg, clone, subst)
		if err != nil {
			return err
		}
		pkg := e.program.Packages[consumerPkg]
		pkg.Records = append(pkg.Records, clone)
		e.program.Packages[consumerPkg] = pkg
		return nil
	})
}

func (e *parametricElaborator) instantiateFunction(consumerPkg, originPkg, name string, args []ast.TypeRef) (string, error) {
	decl := e.functionTemplates[templateKey(originPkg, name)]
	return e.instantiate(consumerPkg, originPkg, "function", name, decl.TypeParameters, args, func(concrete string, subst map[string]ast.TypeRef, origin *ast.TemplateOrigin) error {
		e.program.Parametrics.FunctionInstantiations++
		clone := decl
		clone.Name, clone.TypeParameters, clone.IsTemplate, clone.TemplateOrigin = concrete, nil, false, origin
		var err error
		clone, err = e.rewriteFunction(consumerPkg, clone, subst)
		if err != nil {
			return err
		}
		pkg := e.program.Packages[consumerPkg]
		pkg.Functions = append(pkg.Functions, clone)
		e.program.Packages[consumerPkg] = pkg
		return nil
	})
}

func (e *parametricElaborator) instantiateFlow(consumerPkg, originPkg, name string, args []ast.TypeRef) (string, error) {
	decl := e.flowTemplates[templateKey(originPkg, name)]
	return e.instantiate(consumerPkg, originPkg, "flow/query", name, decl.TypeParameters, args, func(concrete string, subst map[string]ast.TypeRef, origin *ast.TemplateOrigin) error {
		e.program.Parametrics.FlowInstantiations++
		clone := decl
		clone.Name, clone.TypeParameters, clone.IsTemplate, clone.TemplateOrigin = concrete, nil, false, origin
		var err error
		clone, err = e.rewriteFlow(consumerPkg, clone, subst)
		if err != nil {
			return err
		}
		pkg := e.program.Packages[consumerPkg]
		pkg.Flows = append(pkg.Flows, clone)
		e.program.Packages[consumerPkg] = pkg
		return nil
	})
}

func (e *parametricElaborator) instantiate(consumerPkg, originPkg, kind, name string, parameters []string, args []ast.TypeRef, emit func(string, map[string]ast.TypeRef, *ast.TemplateOrigin) error) (string, error) {
	display := originPkg + "." + name
	subst, err := substitution(parameters, args, display)
	if err != nil {
		return "", err
	}
	canonicalArgs := make([]string, len(args))
	for i := range args {
		canonicalArgs[i] = canonicalType(args[i])
	}
	key := consumerPkg + "|" + kind + "|" + display + "<" + strings.Join(canonicalArgs, ",") + ">"
	if concrete, ok := e.instantiations[key]; ok {
		if concrete == "" {
			return "", fmt.Errorf("infinite template instantiation detected while elaborating %s<%s>", display, strings.Join(canonicalArgs, ", "))
		}
		return concrete, nil
	}
	concrete := concreteName(originPkg == consumerPkg, originPkg, name, args)
	e.instantiations[key] = ""
	originArgs := append([]ast.TypeRef(nil), args...)
	origin := &ast.TemplateOrigin{Package: originPkg, Declaration: name, TypeArguments: originArgs}
	if err := emit(concrete, subst, origin); err != nil {
		delete(e.instantiations, key)
		return "", fmt.Errorf("while instantiating %s<%s>: %w", display, joinDisplayTypes(args), err)
	}
	e.instantiations[key] = concrete
	return concrete, nil
}

func (e *parametricElaborator) rewriteBlock(pkgName string, block ast.Block, returnType ast.TypeRef, subst map[string]ast.TypeRef) (ast.Block, error) {
	block.Statements = append([]ast.Stmt(nil), block.Statements...)
	for i, statement := range block.Statements {
		rewritten, err := e.rewriteStmt(pkgName, statement, returnType, subst)
		if err != nil {
			return ast.Block{}, err
		}
		block.Statements[i] = rewritten
	}
	return block, nil
}

func (e *parametricElaborator) rewriteStmt(pkgName string, statement ast.Stmt, returnType ast.TypeRef, subst map[string]ast.TypeRef) (ast.Stmt, error) {
	var err error
	switch s := statement.(type) {
	case ast.LetStmt:
		var expected *ast.TypeRef
		if s.TypeHint != nil {
			t, typeErr := e.rewriteType(pkgName, *s.TypeHint, subst)
			if typeErr != nil {
				return nil, typeErr
			}
			s.TypeHint = &t
			expected = &t
		}
		s.Value, err = e.rewriteExpr(pkgName, s.Value, expected, subst)
		return s, err
	case ast.VarStmt:
		var expected *ast.TypeRef
		if s.TypeHint != nil {
			t, typeErr := e.rewriteType(pkgName, *s.TypeHint, subst)
			if typeErr != nil {
				return nil, typeErr
			}
			s.TypeHint = &t
			expected = &t
		}
		s.Value, err = e.rewriteExpr(pkgName, s.Value, expected, subst)
		return s, err
	case ast.AssignStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.DestructureAssignStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.IndexAssignStmt:
		for i := range s.Indices {
			s.Indices[i], err = e.rewriteExpr(pkgName, s.Indices[i], nil, subst)
			if err != nil {
				return nil, err
			}
		}
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.FieldAssignStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.FieldIndexAssignStmt:
		for i := range s.Indices {
			s.Indices[i], err = e.rewriteExpr(pkgName, s.Indices[i], nil, subst)
			if err != nil {
				return nil, err
			}
		}
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.ReturnStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, &returnType, subst)
		return s, err
	case ast.ExprStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.ForStmt:
		s.Range, err = e.rewriteExpr(pkgName, s.Range, nil, subst)
		if err != nil {
			return nil, err
		}
		s.DescendStep, err = e.rewriteExpr(pkgName, s.DescendStep, nil, subst)
		if err != nil {
			return nil, err
		}
		s.Body, err = e.rewriteBlock(pkgName, s.Body, returnType, subst)
		return s, err
	case ast.MatchStmt:
		s.Subject, err = e.rewriteExpr(pkgName, s.Subject, nil, subst)
		if err != nil {
			return nil, err
		}
		s.OkBody, err = e.rewriteBlock(pkgName, s.OkBody, returnType, subst)
		if err != nil {
			return nil, err
		}
		s.ErrBody, err = e.rewriteBlock(pkgName, s.ErrBody, returnType, subst)
		return s, err
	case ast.IfStmt:
		s.Condition, err = e.rewriteExpr(pkgName, s.Condition, nil, subst)
		if err != nil {
			return nil, err
		}
		s.ThenBody, err = e.rewriteBlock(pkgName, s.ThenBody, returnType, subst)
		if err != nil {
			return nil, err
		}
		if s.ElseBody != nil {
			b, blockErr := e.rewriteBlock(pkgName, *s.ElseBody, returnType, subst)
			if blockErr != nil {
				return nil, blockErr
			}
			s.ElseBody = &b
		}
		return s, nil
	case ast.WhileStmt:
		s.Condition, err = e.rewriteExpr(pkgName, s.Condition, nil, subst)
		if err != nil {
			return nil, err
		}
		s.Body, err = e.rewriteBlock(pkgName, s.Body, returnType, subst)
		return s, err
	case ast.PrometheusStmt:
		s.Body, err = e.rewriteBlock(pkgName, s.Body, returnType, subst)
		return s, err
	case ast.YieldStmt:
		s.Value, err = e.rewriteExpr(pkgName, s.Value, nil, subst)
		return s, err
	case ast.WhenStmt:
		s.Cases = append([]ast.WhenCase(nil), s.Cases...)
		for i := range s.Cases {
			s.Cases[i].Condition, err = e.rewriteExpr(pkgName, s.Cases[i].Condition, nil, subst)
			if err != nil {
				return nil, err
			}
			s.Cases[i].Action, err = e.rewriteAction(pkgName, s.Cases[i].Action, returnType, subst)
			if err != nil {
				return nil, err
			}
		}
		s.Else, err = e.rewriteAction(pkgName, s.Else, returnType, subst)
		return s, err
	default:
		return statement, nil
	}
}

func (e *parametricElaborator) rewriteAction(pkgName string, action ast.WhenAction, returnType ast.TypeRef, subst map[string]ast.TypeRef) (ast.WhenAction, error) {
	switch a := action.(type) {
	case ast.WhenReturnAction:
		value, err := e.rewriteExpr(pkgName, a.Value, &returnType, subst)
		a.Value = value
		return a, err
	case ast.WhenBlockAction:
		a.Statements = append([]ast.Stmt(nil), a.Statements...)
		for i := range a.Statements {
			statement, err := e.rewriteStmt(pkgName, a.Statements[i], returnType, subst)
			if err != nil {
				return nil, err
			}
			a.Statements[i] = statement
		}
		return a, nil
	default:
		return action, nil
	}
}

func (e *parametricElaborator) rewriteExpr(pkgName string, expr ast.Expr, expected *ast.TypeRef, subst map[string]ast.TypeRef) (ast.Expr, error) {
	if expr == nil {
		return nil, nil
	}
	var err error
	switch x := expr.(type) {
	case ast.SelectorExpr:
		if expected == nil || expected.SelectorOwner == nil || expected.SelectorResult == nil {
			return nil, fmt.Errorf("selector .%s requires contextual type Selector<Record, FieldType>", x.Field)
		}
		name, err := e.ensureSelector(pkgName, *expected.SelectorOwner, *expected.SelectorResult, x.Field)
		if err != nil {
			return nil, err
		}
		return ast.IdentifierExpr{Name: name}, nil
	case ast.ArrayLiteralExpr:
		x.Elements = append([]ast.Expr(nil), x.Elements...)
		for i := range x.Elements {
			x.Elements[i], err = e.rewriteExpr(pkgName, x.Elements[i], nil, subst)
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	case ast.VectorLiteralExpr:
		x.Elements = append([]ast.Expr(nil), x.Elements...)
		for i := range x.Elements {
			x.Elements[i], err = e.rewriteExpr(pkgName, x.Elements[i], nil, subst)
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	case ast.MatrixLiteralExpr:
		x.Rows = append([][]ast.Expr(nil), x.Rows...)
		for i := range x.Rows {
			x.Rows[i] = append([]ast.Expr(nil), x.Rows[i]...)
			for j := range x.Rows[i] {
				x.Rows[i][j], err = e.rewriteExpr(pkgName, x.Rows[i][j], nil, subst)
				if err != nil {
					return nil, err
				}
			}
		}
		return x, nil
	case ast.CallExpr:
		x.TypeArguments = append([]ast.TypeRef(nil), x.TypeArguments...)
		x.Arguments = append([]ast.Expr(nil), x.Arguments...)
		for i := range x.TypeArguments {
			x.TypeArguments[i], err = e.rewriteType(pkgName, x.TypeArguments[i], subst)
			if err != nil {
				return nil, err
			}
		}
		calleePkg, calleeName, qualified := directCallee(pkgName, x.Callee)
		if len(x.TypeArguments) > 0 {
			if _, ok := e.functionTemplates[templateKey(calleePkg, calleeName)]; ok {
				name, instErr := e.instantiateFunction(pkgName, calleePkg, calleeName, x.TypeArguments)
				if instErr != nil {
					return nil, instErr
				}
				x.Callee, x.TypeArguments = ast.IdentifierExpr{Name: name}, nil
				calleePkg, calleeName, qualified = pkgName, name, false
			} else if _, ok := e.flowTemplates[templateKey(calleePkg, calleeName)]; ok {
				name, instErr := e.instantiateFlow(pkgName, calleePkg, calleeName, x.TypeArguments)
				if instErr != nil {
					return nil, instErr
				}
				x.Callee, x.TypeArguments = ast.IdentifierExpr{Name: name}, nil
				calleePkg, calleeName, qualified = pkgName, name, false
			}
		}
		parameterTypes := e.callParameterTypes(calleePkg, calleeName)
		for i := range x.Arguments {
			var argExpected *ast.TypeRef
			if i < len(parameterTypes) {
				argExpected = &parameterTypes[i]
			}
			x.Arguments[i], err = e.rewriteExpr(pkgName, x.Arguments[i], argExpected, subst)
			if err != nil {
				return nil, err
			}
		}
		if qualified {
			x.Callee, err = e.rewriteExpr(pkgName, x.Callee, nil, subst)
		}
		return x, err
	case ast.RecordLiteralExpr:
		x.TypeArguments = append([]ast.TypeRef(nil), x.TypeArguments...)
		x.Fields = append([]ast.RecordLiteralField(nil), x.Fields...)
		if len(x.TypeArguments) > 0 {
			for i := range x.TypeArguments {
				x.TypeArguments[i], err = e.rewriteType(pkgName, x.TypeArguments[i], subst)
				if err != nil {
					return nil, err
				}
			}
			originPkg, name := splitSurfaceName(pkgName, x.TypeName)
			concrete, instErr := e.instantiateRecord(pkgName, originPkg, name, x.TypeArguments)
			if instErr != nil {
				return nil, instErr
			}
			x.TypeName, x.TypeArguments = concrete, nil
		}
		fieldTypes := e.recordFieldTypes(pkgName, x.TypeName)
		for i := range x.Fields {
			fieldExpected := fieldTypes[x.Fields[i].Name]
			x.Fields[i].Value, err = e.rewriteExpr(pkgName, x.Fields[i].Value, fieldExpected, subst)
			if err != nil {
				return nil, fmt.Errorf("record literal %s field %s: %w", x.TypeName, x.Fields[i].Name, err)
			}
		}
		return x, nil
	case ast.RecordUpdateExpr:
		x.Fields = append([]ast.RecordLiteralField(nil), x.Fields...)
		x.Source, err = e.rewriteExpr(pkgName, x.Source, nil, subst)
		if err != nil {
			return nil, err
		}
		for i := range x.Fields {
			x.Fields[i].Value, err = e.rewriteExpr(pkgName, x.Fields[i].Value, nil, subst)
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	case ast.IndexExpr:
		x.Indices = append([]ast.Expr(nil), x.Indices...)
		x.Target, err = e.rewriteExpr(pkgName, x.Target, nil, subst)
		if err != nil {
			return nil, err
		}
		for i := range x.Indices {
			x.Indices[i], err = e.rewriteExpr(pkgName, x.Indices[i], nil, subst)
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	case ast.FieldAccessExpr:
		x.Target, err = e.rewriteExpr(pkgName, x.Target, nil, subst)
		return x, err
	case ast.BinaryExpr:
		x.Left, err = e.rewriteExpr(pkgName, x.Left, nil, subst)
		if err != nil {
			return nil, err
		}
		x.Right, err = e.rewriteExpr(pkgName, x.Right, nil, subst)
		return x, err
	case ast.UnaryExpr:
		x.Operand, err = e.rewriteExpr(pkgName, x.Operand, nil, subst)
		return x, err
	case ast.RangeExpr:
		x.Start, err = e.rewriteExpr(pkgName, x.Start, nil, subst)
		if err != nil {
			return nil, err
		}
		x.End, err = e.rewriteExpr(pkgName, x.End, nil, subst)
		if err != nil {
			return nil, err
		}
		x.Step, err = e.rewriteExpr(pkgName, x.Step, nil, subst)
		return x, err
	case ast.ParenExpr:
		x.Inner, err = e.rewriteExpr(pkgName, x.Inner, expected, subst)
		return x, err
	case ast.PropagateExpr:
		x.Inner, err = e.rewriteExpr(pkgName, x.Inner, expected, subst)
		return x, err
	case ast.UnwrapExpr:
		x.Inner, err = e.rewriteExpr(pkgName, x.Inner, expected, subst)
		return x, err
	case ast.SwitchExpr:
		x.Cases = append([]ast.SwitchCase(nil), x.Cases...)
		x.Subject, err = e.rewriteExpr(pkgName, x.Subject, nil, subst)
		if err != nil {
			return nil, err
		}
		for i := range x.Cases {
			x.Cases[i].Match, err = e.rewriteExpr(pkgName, x.Cases[i].Match, nil, subst)
			if err != nil {
				return nil, err
			}
			x.Cases[i].Value, err = e.rewriteExpr(pkgName, x.Cases[i].Value, expected, subst)
			if err != nil {
				return nil, err
			}
		}
		x.Else, err = e.rewriteExpr(pkgName, x.Else, expected, subst)
		return x, err
	case ast.MatchExpr:
		x.Cases = append([]ast.MatchCase(nil), x.Cases...)
		x.Subject, err = e.rewriteExpr(pkgName, x.Subject, nil, subst)
		if err != nil {
			return nil, err
		}
		for i := range x.Cases {
			x.Cases[i].Value, err = e.rewriteExpr(pkgName, x.Cases[i].Value, expected, subst)
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	case ast.IfExpr:
		x.Condition, err = e.rewriteExpr(pkgName, x.Condition, nil, subst)
		if err != nil {
			return nil, err
		}
		x.ThenExpr, err = e.rewriteExpr(pkgName, x.ThenExpr, expected, subst)
		if err != nil {
			return nil, err
		}
		x.ElseExpr, err = e.rewriteExpr(pkgName, x.ElseExpr, expected, subst)
		return x, err
	case ast.UtilityWhenExpr:
		x.Cases = append([]ast.UtilityWhenCase(nil), x.Cases...)
		if x.EnumTarget != nil {
			t, typeErr := e.rewriteType(pkgName, *x.EnumTarget, subst)
			if typeErr != nil {
				return nil, typeErr
			}
			x.EnumTarget = &t
		}
		for i := range x.Cases {
			x.Cases[i].Value, err = e.rewriteExpr(pkgName, x.Cases[i].Value, expected, subst)
			if err != nil {
				return nil, err
			}
			x.Cases[i].Condition, err = e.rewriteExpr(pkgName, x.Cases[i].Condition, nil, subst)
			if err != nil {
				return nil, err
			}
			x.Cases[i].Score, err = e.rewriteExpr(pkgName, x.Cases[i].Score, nil, subst)
			if err != nil {
				return nil, err
			}
		}
		x.Else, err = e.rewriteExpr(pkgName, x.Else, expected, subst)
		return x, err
	case ast.BatchExpr:
		x.Input, err = e.rewriteExpr(pkgName, x.Input, nil, subst)
		if err != nil {
			return nil, err
		}
		x.Body, err = e.rewriteBlock(pkgName, x.Body, ast.TypeRef{Name: "Void"}, subst)
		return x, err
	default:
		return expr, nil
	}
}

func (e *parametricElaborator) ensureSelector(pkgName string, owner, result ast.TypeRef, field string) (string, error) {
	ownerPkg, ownerName := owner.Package, owner.Name
	if ownerPkg == "" {
		ownerPkg = pkgName
	}
	record, ok := e.findRecord(ownerPkg, ownerName)
	if !ok {
		return "", fmt.Errorf("Selector .%s owner %s is not a known nominal record", field, displayType(owner))
	}
	var fieldType *ast.TypeRef
	for _, candidate := range record.Fields {
		if candidate.Name == field {
			value := candidate.Type
			fieldType = &value
			break
		}
	}
	if fieldType == nil {
		return "", fmt.Errorf("Selector .%s does not exist on %s", field, displayType(owner))
	}
	if canonicalType(*fieldType) != canonicalType(result) {
		return "", fmt.Errorf("selector .%s on %s has type %s, but Selector<%s, %s> requires %s", field, displayType(owner), displayType(*fieldType), displayType(owner), displayType(result), displayType(result))
	}
	key := pkgName + "|" + canonicalType(owner) + "." + field + "->" + canonicalType(result)
	if name, ok := e.selectors[key]; ok {
		return name, nil
	}
	name := "__oct_selector__" + mangleType(owner) + "__" + sanitizeIdentifier(field)
	getterOwner := owner
	decl := ast.FunctionDecl{
		Name: name, SourcePath: "<selector ." + field + ">", SelectorOwner: &getterOwner, SelectorField: field,
		Parameters: []ast.Parameter{{Name: "value", Type: owner}}, ReturnType: result,
		Body: ast.Block{Statements: []ast.Stmt{ast.ReturnStmt{Value: ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "value"}, Field: field}}}},
	}
	pkg := e.program.Packages[pkgName]
	pkg.Functions = append(pkg.Functions, decl)
	e.program.Packages[pkgName] = pkg
	e.selectors[key] = name
	e.program.Parametrics.SelectorsResolved++
	return name, nil
}

func (e *parametricElaborator) findRecord(pkgName, name string) (ast.RecordDecl, bool) {
	pkg, ok := e.program.Packages[pkgName]
	if !ok {
		return ast.RecordDecl{}, false
	}
	for _, record := range pkg.Records {
		if record.Name == name {
			return record, true
		}
	}
	return ast.RecordDecl{}, false
}

func (e *parametricElaborator) recordFieldTypes(pkgName, surface string) map[string]*ast.TypeRef {
	recordPkg, name := splitSurfaceName(pkgName, surface)
	record, ok := e.findRecord(recordPkg, name)
	if !ok {
		return nil
	}
	result := make(map[string]*ast.TypeRef, len(record.Fields))
	for _, field := range record.Fields {
		t := field.Type
		result[field.Name] = &t
	}
	return result
}

func (e *parametricElaborator) callParameterTypes(pkgName, name string) []ast.TypeRef {
	pkg, ok := e.program.Packages[pkgName]
	if !ok {
		return nil
	}
	for _, fn := range pkg.Functions {
		if fn.Name == name {
			out := make([]ast.TypeRef, len(fn.Parameters))
			for i := range fn.Parameters {
				out[i] = fn.Parameters[i].Type
			}
			return out
		}
	}
	for _, flow := range pkg.Flows {
		if flow.Name == name {
			out := make([]ast.TypeRef, len(flow.Parameters))
			for i := range flow.Parameters {
				out[i] = flow.Parameters[i].Type
			}
			return out
		}
	}
	return nil
}

func directCallee(currentPkg string, expr ast.Expr) (string, string, bool) {
	switch x := expr.(type) {
	case ast.IdentifierExpr:
		return currentPkg, x.Name, false
	case ast.FieldAccessExpr:
		if id, ok := x.Target.(ast.IdentifierExpr); ok {
			return id.Name, x.Field, true
		}
	}
	return currentPkg, "", false
}

func splitSurfaceName(currentPkg, name string) (string, string) {
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		return name[:dot], name[dot+1:]
	}
	return currentPkg, name
}

func (e *parametricElaborator) contextError(origin *ast.TemplateOrigin, part string, err error) error {
	if origin == nil {
		return fmt.Errorf("%s: %w", part, err)
	}
	return fmt.Errorf("%s<%s> %s: %w", origin.Declaration, joinDisplayTypes(origin.TypeArguments), part, err)
}

var nonIdentifier = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func sanitizeIdentifier(value string) string {
	value = nonIdentifier.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "Type"
	}
	return value
}

func concreteName(local bool, originPkg, name string, args []ast.TypeRef) string {
	prefix := ""
	if !local {
		prefix = sanitizeIdentifier(originPkg) + "__"
	}
	parts := make([]string, len(args))
	for i := range args {
		parts[i] = mangleType(args[i])
	}
	candidate := prefix + name + "__" + strings.Join(parts, "__")
	if len(candidate) <= 120 {
		return candidate
	}
	hash := sha256.Sum256([]byte(candidate))
	return candidate[:96] + fmt.Sprintf("__%x", hash[:8])
}

func mangleType(t ast.TypeRef) string { return sanitizeIdentifier(canonicalType(t)) }

func canonicalType(t ast.TypeRef) string {
	if t.Function != nil {
		parts := make([]string, len(t.Function.Parameters))
		for i := range t.Function.Parameters {
			parts[i] = canonicalType(t.Function.Parameters[i])
		}
		value := "fn(" + strings.Join(parts, ",") + ")->" + canonicalType(t.Function.ReturnType)
		if t.Function.IsFallible && t.Function.ErrorType != nil {
			value += "!" + canonicalType(*t.Function.ErrorType)
		}
		return value + strings.Repeat("[]", t.ArrayDepth)
	}
	if t.VectorOf != nil {
		return "Vector<" + canonicalType(*t.VectorOf) + ">" + strings.Repeat("[]", t.ArrayDepth)
	}
	if t.MatrixOf != nil {
		return "Matrix<" + canonicalType(*t.MatrixOf) + ">" + strings.Repeat("[]", t.ArrayDepth)
	}
	name := t.Name
	if t.Package != "" {
		name = t.Package + "." + name
	}
	if len(t.TypeArguments) > 0 {
		parts := make([]string, len(t.TypeArguments))
		for i := range t.TypeArguments {
			parts[i] = canonicalType(t.TypeArguments[i])
		}
		name += "<" + strings.Join(parts, ",") + ">"
	}
	if t.HasUnit {
		name += "<" + t.Dimension.String() + ">"
	}
	depth := t.ArrayDepth
	if t.IsArray && depth == 0 {
		depth = 1
	}
	return name + strings.Repeat("[]", depth)
}

func displayType(t ast.TypeRef) string { return canonicalType(t) }
func joinDisplayTypes(types []ast.TypeRef) string {
	parts := make([]string, len(types))
	for i := range types {
		parts[i] = displayType(types[i])
	}
	return strings.Join(parts, ", ")
}
