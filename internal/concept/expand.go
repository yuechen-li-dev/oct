package concept

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// ExpandFile erases transparent named concepts to their concrete value shape.
// Record-shaped concepts remain nominal records and therefore reuse the
// existing record checker, interpreter, and backend without runtime metadata.
func ExpandFile(file ast.File) (ast.File, error) {
	aliases := make(map[string]ast.TypeRef, len(file.Concepts))
	refinements := make(map[string]ast.ConceptDecl)
	declared := make(map[string]string, len(file.Concepts)+len(file.Records)+len(file.Enums))
	for _, c := range file.Concepts {
		if prior, ok := declared[c.Name]; ok {
			return ast.File{}, fmt.Errorf("duplicate concept declaration '%s' (already declared as %s)", c.Name, prior)
		}
		declared[c.Name] = "concept"
		if len(c.Requirements) == 0 {
			aliases[c.Name] = c.Target
		} else {
			refinements[c.Name] = c
		}
	}
	for _, r := range file.Records {
		kind := "record"
		if r.IsConcept {
			kind = "concept"
		}
		if prior, ok := declared[r.Name]; ok {
			if kind == "concept" || prior == "concept" {
				return ast.File{}, fmt.Errorf("duplicate concept declaration '%s'", r.Name)
			}
			return ast.File{}, fmt.Errorf("duplicate type: %s", r.Name)
		}
		declared[r.Name] = kind
	}
	for _, e := range file.Enums {
		if prior, ok := declared[e.Name]; ok {
			if prior == "concept" {
				return ast.File{}, fmt.Errorf("duplicate concept declaration '%s'", e.Name)
			}
			return ast.File{}, fmt.Errorf("duplicate type: %s", e.Name)
		}
		declared[e.Name] = "enum"
	}

	resolved := make(map[string]ast.TypeRef, len(aliases))
	visiting := make(map[string]bool, len(aliases))
	var resolveAlias func(string) (ast.TypeRef, error)
	resolveAlias = func(name string) (ast.TypeRef, error) {
		if target, ok := resolved[name]; ok {
			return target, nil
		}
		if visiting[name] {
			return ast.TypeRef{}, fmt.Errorf("cyclic concept alias involving '%s'", name)
		}
		visiting[name] = true
		target := aliases[name]
		expanded, err := expandType(target, aliases, resolveAlias)
		if err != nil {
			return ast.TypeRef{}, err
		}
		visiting[name] = false
		resolved[name] = expanded
		return expanded, nil
	}
	for name := range aliases {
		if _, err := resolveAlias(name); err != nil {
			return ast.File{}, err
		}
	}
	aliases = resolved

	for i := range file.Concepts {
		if len(file.Concepts[i].Requirements) == 0 {
			file.Concepts[i].Target = aliases[file.Concepts[i].Name]
			continue
		}
		file.Concepts[i].Target = mustExpand(file.Concepts[i].Target, aliases)
		refinements[file.Concepts[i].Name] = file.Concepts[i]
	}
	for i := range file.Records {
		for j := range file.Records[i].Fields {
			file.Records[i].Fields[j].Type = mustExpand(file.Records[i].Fields[j].Type, aliases)
		}
	}
	if err := rejectRecordConceptCycles(file.Records, aliases); err != nil {
		return ast.File{}, err
	}
	for i := range file.Enums {
		for j := range file.Enums[i].Variants {
			if file.Enums[i].Variants[j].Payload != nil {
				t := mustExpand(*file.Enums[i].Variants[j].Payload, aliases)
				file.Enums[i].Variants[j].Payload = &t
			}
		}
	}
	for i := range file.Functions {
		fn := &file.Functions[i]
		for j := range fn.Parameters {
			fn.Parameters[j].Type = mustExpand(fn.Parameters[j].Type, aliases)
		}
		fn.ReturnType = mustExpand(fn.ReturnType, aliases)
		fn.ErrorType = mustExpand(fn.ErrorType, aliases)
		fn.Body = expandBlock(fn.Body, aliases, refinements)
	}
	for i := range file.Flows {
		flow := &file.Flows[i]
		for j := range flow.Parameters {
			flow.Parameters[j].Type = mustExpand(flow.Parameters[j].Type, aliases)
		}
		if flow.TurnInput != nil {
			flow.TurnInput.Type = mustExpand(flow.TurnInput.Type, aliases)
		}
		if flow.YieldType != nil {
			yieldType := mustExpand(*flow.YieldType, aliases)
			flow.YieldType = &yieldType
		}
		flow.ReturnType = mustExpand(flow.ReturnType, aliases)
		for j := range flow.Board {
			flow.Board[j].Type = mustExpand(flow.Board[j].Type, aliases)
		}
		for j := range flow.States {
			flow.States[j].Body = expandBlock(flow.States[j].Body, aliases, refinements)
		}
	}
	for _, refinement := range refinements {
		file.Functions = append(file.Functions, refinementConstructor(refinement))
	}
	return file, nil
}

func refinementConstructor(c ast.ConceptDecl) ast.FunctionDecl {
	name := "__oct_refine_" + c.Name
	statements := make([]ast.Stmt, 0, len(c.Requirements)+1)
	for _, requirement := range c.Requirements {
		message := requirement.Explanation
		if message == "" {
			message = "requirement " + formatRequirement(requirement.Condition) + " was not satisfied"
		}
		message = "refined concept " + c.Name + ": " + message
		statements = append(statements, ast.IfStmt{
			Condition: ast.UnaryExpr{Operator: "not", Operand: requirement.Condition},
			ThenBody: ast.Block{Statements: []ast.Stmt{ast.ReturnStmt{Value: ast.CallExpr{
				Callee: ast.IdentifierExpr{Name: "error"}, Arguments: []ast.Expr{ast.StringLiteralExpr{Value: message}},
			}}}},
		})
	}
	statements = append(statements, ast.ReturnStmt{Value: ast.IdentifierExpr{Name: "Self"}})
	return ast.FunctionDecl{
		Name: name, SourcePath: "<refined concept " + c.Name + ">",
		Parameters: []ast.Parameter{{Name: "Self", Type: c.Target}},
		ReturnType: ast.TypeRef{Name: c.Name}, IsFallible: true,
		ErrorType: ast.TypeRef{Name: "Error"}, Body: ast.Block{Statements: statements},
		IsRefinementConstructor: true,
	}
}

func formatRequirement(expr ast.Expr) string {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name
	case ast.IntegerLiteral:
		return node.Value
	case ast.FloatLiteral:
		return node.Value
	case ast.BoolLiteral:
		if node.Value {
			return "true"
		}
		return "false"
	case ast.StringLiteralExpr:
		return fmt.Sprintf("%q", node.Value)
	case ast.ParenExpr:
		return "(" + formatRequirement(node.Inner) + ")"
	case ast.UnaryExpr:
		return node.Operator + " " + formatRequirement(node.Operand)
	case ast.BinaryExpr:
		return formatRequirement(node.Left) + " " + node.Operator + " " + formatRequirement(node.Right)
	case ast.CallExpr:
		callee, _ := node.Callee.(ast.IdentifierExpr)
		parts := make([]string, 0, len(node.Arguments))
		for _, arg := range node.Arguments {
			parts = append(parts, formatRequirement(arg))
		}
		return callee.Name + "(" + fmt.Sprint(parts) + ")"
	default:
		return "<requirement>"
	}
}

func mustExpand(t ast.TypeRef, aliases map[string]ast.TypeRef) ast.TypeRef {
	expanded, _ := expandType(t, aliases, func(name string) (ast.TypeRef, error) { return aliases[name], nil })
	return expanded
}

func expandType(t ast.TypeRef, aliases map[string]ast.TypeRef, resolve func(string) (ast.TypeRef, error)) (ast.TypeRef, error) {
	if t.Package == "" && t.Name != "" {
		if _, ok := aliases[t.Name]; ok {
			if t.HasUnit {
				return ast.TypeRef{}, fmt.Errorf("concept '%s' cannot be dimension-qualified at its use site", t.Name)
			}
			target, err := resolve(t.Name)
			if err != nil {
				return ast.TypeRef{}, err
			}
			depth := t.ArrayDepth
			if t.IsArray && depth == 0 {
				depth = 1
			}
			target.ArrayDepth += depth
			target.IsArray = target.ArrayDepth > 0
			return target, nil
		}
	}
	if t.VectorOf != nil {
		e, err := expandType(*t.VectorOf, aliases, resolve)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.VectorOf = &e
	}
	for i := range t.TypeArguments {
		e, err := expandType(t.TypeArguments[i], aliases, resolve)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.TypeArguments[i] = e
	}
	if t.MatrixOf != nil {
		e, err := expandType(*t.MatrixOf, aliases, resolve)
		if err != nil {
			return ast.TypeRef{}, err
		}
		t.MatrixOf = &e
	}
	if t.Function != nil {
		f := *t.Function
		for i := range f.Parameters {
			f.Parameters[i] = mustExpand(f.Parameters[i], aliases)
		}
		f.ReturnType = mustExpand(f.ReturnType, aliases)
		if f.ErrorType != nil {
			e := mustExpand(*f.ErrorType, aliases)
			f.ErrorType = &e
		}
		t.Function = &f
	}
	return t, nil
}

func rejectRecordConceptCycles(records []ast.RecordDecl, aliases map[string]ast.TypeRef) error {
	concepts := map[string]ast.RecordDecl{}
	for _, r := range records {
		if r.IsConcept {
			concepts[r.Name] = r
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("cyclic record-shaped concept involving '%s'", name)
		}
		if done[name] {
			return nil
		}
		visiting[name] = true
		for _, field := range concepts[name].Fields {
			t := field.Type
			if t.IsArray || t.ArrayDepth > 0 {
				continue
			}
			if _, ok := concepts[t.Name]; ok {
				if err := visit(t.Name); err != nil {
					return err
				}
			}
		}
		visiting[name] = false
		done[name] = true
		return nil
	}
	for name := range concepts {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func expandBlock(block ast.Block, aliases map[string]ast.TypeRef, refinements map[string]ast.ConceptDecl) ast.Block {
	for i, stmt := range block.Statements {
		block.Statements[i] = expandStmt(stmt, aliases, refinements)
	}
	return block
}

func expandStmt(stmt ast.Stmt, aliases map[string]ast.TypeRef, refinements map[string]ast.ConceptDecl) ast.Stmt {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if s.TypeHint != nil {
			t := mustExpand(*s.TypeHint, aliases)
			s.TypeHint = &t
		}
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.VarStmt:
		if s.TypeHint != nil {
			t := mustExpand(*s.TypeHint, aliases)
			s.TypeHint = &t
		}
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.AssignStmt:
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.DestructureAssignStmt:
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.IndexAssignStmt:
		for i := range s.Indices {
			s.Indices[i] = expandExpr(s.Indices[i], aliases, refinements)
		}
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.FieldAssignStmt:
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.FieldIndexAssignStmt:
		for i := range s.Indices {
			s.Indices[i] = expandExpr(s.Indices[i], aliases, refinements)
		}
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.ReturnStmt:
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.ExprStmt:
		s.Value = expandExpr(s.Value, aliases, refinements)
		return s
	case ast.ForStmt:
		s.Range = expandExpr(s.Range, aliases, refinements)
		s.DescendStep = expandExpr(s.DescendStep, aliases, refinements)
		s.Body = expandBlock(s.Body, aliases, refinements)
		return s
	case ast.MatchStmt:
		s.Subject = expandExpr(s.Subject, aliases, refinements)
		s.OkBody = expandBlock(s.OkBody, aliases, refinements)
		s.ErrBody = expandBlock(s.ErrBody, aliases, refinements)
		return s
	case ast.IfStmt:
		s.Condition = expandExpr(s.Condition, aliases, refinements)
		s.ThenBody = expandBlock(s.ThenBody, aliases, refinements)
		if s.ElseBody != nil {
			b := expandBlock(*s.ElseBody, aliases, refinements)
			s.ElseBody = &b
		}
		return s
	case ast.WhileStmt:
		s.Condition = expandExpr(s.Condition, aliases, refinements)
		s.Body = expandBlock(s.Body, aliases, refinements)
		return s
	case ast.PrometheusStmt:
		s.Body = expandBlock(s.Body, aliases, refinements)
		return s
	case ast.WhenStmt:
		for i := range s.Cases {
			s.Cases[i].Condition = expandExpr(s.Cases[i].Condition, aliases, refinements)
			s.Cases[i].Action = expandAction(s.Cases[i].Action, aliases, refinements)
		}
		s.Else = expandAction(s.Else, aliases, refinements)
		return s
	default:
		return stmt
	}
}

func expandAction(action ast.WhenAction, aliases map[string]ast.TypeRef, refinements map[string]ast.ConceptDecl) ast.WhenAction {
	switch a := action.(type) {
	case ast.WhenReturnAction:
		a.Value = expandExpr(a.Value, aliases, refinements)
		return a
	case ast.WhenBlockAction:
		for i, s := range a.Statements {
			a.Statements[i] = expandStmt(s, aliases, refinements)
		}
		return a
	default:
		return action
	}
}

func expandExpr(expr ast.Expr, aliases map[string]ast.TypeRef, refinements map[string]ast.ConceptDecl) ast.Expr {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case ast.ArrayLiteralExpr:
		for i := range e.Elements {
			e.Elements[i] = expandExpr(e.Elements[i], aliases, refinements)
		}
		return e
	case ast.VectorLiteralExpr:
		for i := range e.Elements {
			e.Elements[i] = expandExpr(e.Elements[i], aliases, refinements)
		}
		return e
	case ast.MatrixLiteralExpr:
		for i := range e.Rows {
			for j := range e.Rows[i] {
				e.Rows[i][j] = expandExpr(e.Rows[i][j], aliases, refinements)
			}
		}
		return e
	case ast.CallExpr:
		e.Callee = expandExpr(e.Callee, aliases, refinements)
		if callee, ok := e.Callee.(ast.IdentifierExpr); ok {
			if _, refined := refinements[callee.Name]; refined {
				e.Callee = ast.IdentifierExpr{Name: "__oct_refine_" + callee.Name}
			}
		}
		for i := range e.TypeArguments {
			e.TypeArguments[i] = mustExpand(e.TypeArguments[i], aliases)
		}
		for i := range e.Arguments {
			e.Arguments[i] = expandExpr(e.Arguments[i], aliases, refinements)
		}
		return e
	case ast.IndexExpr:
		e.Target = expandExpr(e.Target, aliases, refinements)
		for i := range e.Indices {
			e.Indices[i] = expandExpr(e.Indices[i], aliases, refinements)
		}
		return e
	case ast.FieldAccessExpr:
		e.Target = expandExpr(e.Target, aliases, refinements)
		return e
	case ast.BinaryExpr:
		e.Left = expandExpr(e.Left, aliases, refinements)
		e.Right = expandExpr(e.Right, aliases, refinements)
		return e
	case ast.UnaryExpr:
		e.Operand = expandExpr(e.Operand, aliases, refinements)
		return e
	case ast.RangeExpr:
		e.Start = expandExpr(e.Start, aliases, refinements)
		e.End = expandExpr(e.End, aliases, refinements)
		e.Step = expandExpr(e.Step, aliases, refinements)
		return e
	case ast.ParenExpr:
		e.Inner = expandExpr(e.Inner, aliases, refinements)
		return e
	case ast.PropagateExpr:
		e.Inner = expandExpr(e.Inner, aliases, refinements)
		return e
	case ast.UnwrapExpr:
		e.Inner = expandExpr(e.Inner, aliases, refinements)
		return e
	case ast.SwitchExpr:
		e.Subject = expandExpr(e.Subject, aliases, refinements)
		for i := range e.Cases {
			e.Cases[i].Match = expandExpr(e.Cases[i].Match, aliases, refinements)
			e.Cases[i].Value = expandExpr(e.Cases[i].Value, aliases, refinements)
		}
		e.Else = expandExpr(e.Else, aliases, refinements)
		return e
	case ast.MatchExpr:
		e.Subject = expandExpr(e.Subject, aliases, refinements)
		for i := range e.Cases {
			e.Cases[i].Value = expandExpr(e.Cases[i].Value, aliases, refinements)
		}
		return e
	case ast.IfExpr:
		e.Condition = expandExpr(e.Condition, aliases, refinements)
		e.ThenExpr = expandExpr(e.ThenExpr, aliases, refinements)
		e.ElseExpr = expandExpr(e.ElseExpr, aliases, refinements)
		return e
	case ast.UtilityWhenExpr:
		if e.EnumTarget != nil {
			t := mustExpand(*e.EnumTarget, aliases)
			e.EnumTarget = &t
		}
		for i := range e.Cases {
			e.Cases[i].Value = expandExpr(e.Cases[i].Value, aliases, refinements)
			e.Cases[i].Condition = expandExpr(e.Cases[i].Condition, aliases, refinements)
			e.Cases[i].Score = expandExpr(e.Cases[i].Score, aliases, refinements)
		}
		e.Else = expandExpr(e.Else, aliases, refinements)
		return e
	case ast.BatchExpr:
		e.Input = expandExpr(e.Input, aliases, refinements)
		e.Body = expandBlock(e.Body, aliases, refinements)
		return e
	case ast.RecordLiteralExpr:
		for i := range e.TypeArguments {
			e.TypeArguments[i] = mustExpand(e.TypeArguments[i], aliases)
		}
		for i := range e.Fields {
			e.Fields[i].Value = expandExpr(e.Fields[i].Value, aliases, refinements)
		}
		return e
	case ast.RecordUpdateExpr:
		e.Source = expandExpr(e.Source, aliases, refinements)
		for i := range e.Fields {
			e.Fields[i].Value = expandExpr(e.Fields[i].Value, aliases, refinements)
		}
		return e
	default:
		return expr
	}
}
