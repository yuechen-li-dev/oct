package lower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/consteval"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

func Module(module ast.Module) (vdmir.Module, error) {
	specialized, err := specializeModule(module)
	if err != nil {
		return vdmir.Module{}, err
	}
	expanded, err := expandComptimeModule(specialized)
	if err != nil {
		return vdmir.Module{}, err
	}
	module = expanded
	l := lowering{
		provenance: vdmir.ProvenanceFromFile(module.Source),
		types:      map[string]typeInfo{},
		functions:  map[string]functionInfo{},
	}
	l.seedBuiltins()
	l.collect(module)
	out := vdmir.Module{
		Provenance: l.provenance,
		Namespace:  module.Namespace,
	}
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			out.TypeAliases = append(out.TypeAliases, vdmir.TypeAlias{
				Provenance: l.provenance,
				Name:       d.Name,
				Target:     l.lowerTypeRef(d.Type),
			})
		case ast.RecordDecl:
			out.Records = append(out.Records, l.lowerRecord(d.Name, d.Fields))
		case ast.StreamDecl:
			out.Streams = append(out.Streams, l.lowerStream(d.Name, d.Fields))
		case ast.EnumDecl:
			out.Enums = append(out.Enums, l.lowerEnum(d))
		case ast.FunctionDecl:
			fn, err := l.lowerFunction("", d, nil, nil)
			if err != nil {
				return vdmir.Module{}, err
			}
			out.Functions = append(out.Functions, fn)
		case ast.ShaderDecl:
			if d.Template != nil {
				continue
			}
			resources, bundleName, err := l.resolveShaderResources(d)
			if err != nil {
				return vdmir.Module{}, err
			}
			for i, resource := range resources {
				binding := resolveResourceBinding(resources, i)
				out.Resources = append(out.Resources, vdmir.Resource{
					Provenance:  l.provenance,
					BundleName:  bundleName,
					Name:        resource.Name,
					ElementType: l.lowerResourceElementType(resource.Type),
					Access:      lowerResourceAccess(resource.Access),
					Binding:     binding,
				})
			}
			for _, workgroup := range d.Workgroups {
				typ := l.lowerTypeRef(workgroup.Type)
				elem := vdmir.Type{}
				if typ.Element != nil {
					elem = *typ.Element
				}
				rows := 0
				cols := 0
				isTile := workgroup.Type.Name == "tile"
				length := 0
				if isTile {
					rows = mustConcreteInt(workgroup.Type.TileRows)
					cols = mustConcreteInt(workgroup.Type.TileCols)
					length = rows * cols
				} else {
					length = mustConcreteInt(workgroup.Type.ArraySize)
				}
				out.Workgroups = append(out.Workgroups, vdmir.WorkgroupMemoryDecl{
					Provenance:  l.provenance,
					ShaderName:  d.Name,
					Name:        workgroup.Name,
					Type:        typ,
					ElementType: elem,
					Length:      length,
					Rows:        rows,
					Cols:        cols,
					IsTile:      isTile,
				})
			}
			for _, method := range d.Methods {
				fn, err := l.lowerFunction(d.Name, method, resources, d.Workgroups)
				if err != nil {
					return vdmir.Module{}, err
				}
				out.Functions = append(out.Functions, fn)
				if method.Stage == "compute" {
					out.EntryPoints = append(out.EntryPoints, l.lowerComputeEntryPoint(d, method, resources))
				}
			}
		case ast.ConceptDecl, ast.ConfigDecl, ast.CompileDecl:
			continue
		case ast.UnsupportedDecl:
			return vdmir.Module{}, fmt.Errorf("%s is not implemented in GoOct SDSL-V M0", d.Kind)
		}
	}
	return out, nil
}

type specializeValue struct {
	typ     ast.TypeRef
	int32   int64
	boolVal bool
}

type conceptFieldSpec struct {
	Path         string
	Type         ast.TypeRef
	DefaultValue ast.Expr
	ZeroAllowed  bool
}

func specializeModule(module ast.Module) (ast.Module, error) {
	concepts := map[string]ast.ConceptDecl{}
	configDecls := map[string]ast.ConfigDecl{}
	templates := map[string]ast.ShaderDecl{}
	var compileDecls []ast.CompileDecl
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.ConceptDecl:
			concepts[d.Name] = d
		case ast.ConfigDecl:
			configDecls[d.Name] = d
		case ast.ShaderDecl:
			if d.Template != nil {
				templates[d.Name] = d
			}
		case ast.CompileDecl:
			compileDecls = append(compileDecls, d)
		}
	}
	out := module
	out.Decls = append([]ast.Decl(nil), module.Decls...)
	for _, decl := range compileDecls {
		template, ok := templates[decl.ShaderName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown template shader %s", decl.ShaderName)
		}
		configDecl, ok := configDecls[decl.ConfigName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown config %s", decl.ConfigName)
		}
		concept, ok := concepts[configDecl.ConceptName]
		if !ok {
			return ast.Module{}, fmt.Errorf("unknown concept %s", configDecl.ConceptName)
		}
		config, err := expandConfig(concept, configDecl)
		if err != nil {
			return ast.Module{}, err
		}
		env := map[string]specializeValue{}
		for name, value := range config {
			env[template.Template.Name+"."+name] = value
		}
		instance, err := specializeShader(template, decl.AliasName, env)
		if err != nil {
			return ast.Module{}, err
		}
		out.Decls = append(out.Decls, instance)
	}
	return out, nil
}

func specializeShader(shader ast.ShaderDecl, alias string, env map[string]specializeValue) (ast.ShaderDecl, error) {
	out := shader
	out.Name = alias
	out.Template = nil
	out.SpecializedConfig = lowerSpecializedConfig(env)
	out.Workgroups = make([]ast.WorkgroupDecl, 0, len(shader.Workgroups))
	for _, workgroup := range shader.Workgroups {
		ref, err := specializeTypeRef(workgroup.Type, env)
		if err != nil {
			return ast.ShaderDecl{}, err
		}
		out.Workgroups = append(out.Workgroups, ast.WorkgroupDecl{Name: workgroup.Name, Type: ref})
	}
	out.Methods = make([]ast.FunctionDecl, 0, len(shader.Methods))
	for _, method := range shader.Methods {
		specialized, err := specializeFunction(method, env)
		if err != nil {
			return ast.ShaderDecl{}, err
		}
		out.Methods = append(out.Methods, specialized)
	}
	return out, nil
}

func lowerSpecializedConfig(env map[string]specializeValue) map[string]uint32 {
	if len(env) == 0 {
		return nil
	}
	out := map[string]uint32{}
	for key, value := range env {
		field := key
		if dot := strings.IndexByte(field, '.'); dot >= 0 && dot+1 < len(field) {
			field = field[dot+1:]
		}
		if value.int32 < 0 {
			continue
		}
		out[flattenConfigName(field)] = uint32(value.int32)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func specializeFunction(fn ast.FunctionDecl, env map[string]specializeValue) (ast.FunctionDecl, error) {
	out := fn
	if fn.NumThreads != nil {
		x, err := specializeIntExpr(fn.NumThreads.X, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		y, err := specializeIntExpr(fn.NumThreads.Y, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		z, err := specializeIntExpr(fn.NumThreads.Z, env)
		if err != nil {
			return ast.FunctionDecl{}, err
		}
		out.NumThreads = &ast.NumThreads{X: x, Y: y, Z: z}
	}
	body, err := specializeBlock(fn.Body, env)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	out.Body = body
	return out, nil
}

func specializeBlock(block ast.Block, env map[string]specializeValue) (ast.Block, error) {
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		next, err := specializeStmt(stmt, env)
		if err != nil {
			return ast.Block{}, err
		}
		out.Statements = append(out.Statements, next)
	}
	return out, nil
}

func specializeStmt(stmt ast.Stmt, env map[string]specializeValue) (ast.Stmt, error) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		ref, err := specializeLocalTypeRef(s.Type, env)
		if err != nil {
			return nil, err
		}
		var value ast.Expr
		if s.Value != nil {
			value = specializeExpr(s.Value, env)
		}
		return ast.LetStmt{Name: s.Name, Type: ref, Value: value}, nil
	case ast.ComptimeLetStmt:
		ref, err := specializeLocalTypeRef(s.Type, env)
		if err != nil {
			return nil, err
		}
		return ast.ComptimeLetStmt{Name: s.Name, Type: ref, Value: specializeExpr(s.Value, env)}, nil
	case ast.AssignStmt:
		return ast.AssignStmt{Target: specializeExpr(s.Target, env), Value: specializeExpr(s.Value, env)}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return s, nil
		}
		return ast.ReturnStmt{Value: specializeExpr(s.Value, env)}, nil
	case ast.ExprStmt:
		return ast.ExprStmt{Value: specializeExpr(s.Value, env)}, nil
	case ast.IfStmt:
		thenBody, err := specializeBlock(s.ThenBody, env)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.IfStmt{Condition: specializeExpr(s.Condition, env), ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.ComptimeIfStmt:
		thenBody, err := specializeBlock(s.ThenBody, env)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.ComptimeIfStmt{Condition: specializeExpr(s.Condition, env), ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.ComptimeMatchStmt:
		arms := make([]ast.ComptimeMatchArm, 0, len(s.Arms))
		for _, arm := range s.Arms {
			body, err := specializeBlock(arm.Body, env)
			if err != nil {
				return nil, err
			}
			var pattern ast.Expr
			if arm.Pattern != nil {
				pattern = specializeExpr(arm.Pattern, env)
			}
			arms = append(arms, ast.ComptimeMatchArm{Pattern: pattern, IsElse: arm.IsElse, Body: body})
		}
		return ast.ComptimeMatchStmt{Subject: specializeExpr(s.Subject, env), Arms: arms}, nil
	case ast.ComptimeWhenUtilityStmt:
		cases := make([]ast.ComptimeWhenUtilityCase, 0, len(s.Cases))
		for _, c := range s.Cases {
			body, err := specializeBlock(c.Body, env)
			if err != nil {
				return nil, err
			}
			var condition ast.Expr
			if c.Condition != nil {
				condition = specializeExpr(c.Condition, env)
			}
			cases = append(cases, ast.ComptimeWhenUtilityCase{
				Label:     c.Label,
				Condition: condition,
				Score:     specializeExpr(c.Score, env),
				Body:      body,
			})
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := specializeBlock(*s.ElseBody, env)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return ast.ComptimeWhenUtilityStmt{Cases: cases, ElseBody: elseBody}, nil
	case ast.ForStmt:
		body, err := specializeBlock(s.Body, env)
		if err != nil {
			return nil, err
		}
		return ast.ForStmt{Attributes: append([]ast.Attribute(nil), s.Attributes...), Name: s.Name, Start: specializeExpr(s.Start, env), End: specializeExpr(s.End, env), Step: specializeExpr(s.Step, env), Body: body}, nil
	case ast.StaticAssertStmt:
		return ast.StaticAssertStmt{Expr: specializeExpr(s.Expr, env), Text: s.Text}, nil
	default:
		return stmt, nil
	}
}

type comptimeBinding struct {
	typ     ast.TypeRef
	int32   int64
	boolVal bool
}

type runtimeOrigin string

const (
	runtimeParam     runtimeOrigin = "runtime parameter"
	runtimeResource  runtimeOrigin = "resource"
	runtimeWorkgroup runtimeOrigin = "workgroup value"
	runtimeBuiltin   runtimeOrigin = "thread builtin"
	runtimeLocal     runtimeOrigin = "runtime local"
)

type runtimeBinding struct {
	origin runtimeOrigin
}

func expandComptimeModule(module ast.Module) (ast.Module, error) {
	resourceBundles := map[string][]ast.ResourceDecl{}
	for _, decl := range module.Decls {
		stream, ok := decl.(ast.StreamDecl)
		if !ok {
			continue
		}
		for _, field := range stream.Fields {
			if field.Access == "" {
				continue
			}
			resourceBundles[stream.Name] = append(resourceBundles[stream.Name], ast.ResourceDecl{Name: field.Name, Access: field.Access, Type: field.Type, Attributes: field.Attributes})
		}
	}
	out := module
	out.Decls = make([]ast.Decl, 0, len(module.Decls))
	for _, decl := range module.Decls {
		shader, ok := decl.(ast.ShaderDecl)
		if !ok || shader.Template != nil {
			out.Decls = append(out.Decls, decl)
			continue
		}
		expanded, err := expandComptimeShader(shader, resourceBundles)
		if err != nil {
			return ast.Module{}, err
		}
		out.Decls = append(out.Decls, expanded)
	}
	return out, nil
}

func expandComptimeShader(shader ast.ShaderDecl, resourceBundles map[string][]ast.ResourceDecl) (ast.ShaderDecl, error) {
	out := shader
	out.Methods = make([]ast.FunctionDecl, 0, len(shader.Methods))
	resources := shader.Resources
	if shader.ResourceBundleName != "" {
		resources = append([]ast.ResourceDecl(nil), resourceBundles[shader.ResourceBundleName]...)
	}
	for _, method := range shader.Methods {
		expanded, err := expandComptimeFunction(method, resources, shader.Workgroups)
		if err != nil {
			return ast.ShaderDecl{}, fmt.Errorf("shader %s.%s: %w", shader.Name, method.Name, err)
		}
		out.Methods = append(out.Methods, expanded)
	}
	return out, nil
}

func expandComptimeFunction(fn ast.FunctionDecl, resources []ast.ResourceDecl, workgroups []ast.WorkgroupDecl) (ast.FunctionDecl, error) {
	runtime := map[string]runtimeBinding{
		"DispatchThreadID": {origin: runtimeBuiltin},
		"GroupThreadID":    {origin: runtimeBuiltin},
		"GroupID":          {origin: runtimeBuiltin},
		"GroupIndex":       {origin: runtimeBuiltin},
	}
	for _, resource := range resources {
		runtime[resource.Name] = runtimeBinding{origin: runtimeResource}
	}
	for _, workgroup := range workgroups {
		runtime[workgroup.Name] = runtimeBinding{origin: runtimeWorkgroup}
	}
	for _, param := range fn.Parameters {
		runtime[param.Name] = runtimeBinding{origin: runtimeParam}
	}
	body, err := expandComptimeBlock(fn.Body, nil, runtime)
	if err != nil {
		return ast.FunctionDecl{}, err
	}
	out := fn
	out.Body = body
	return out, nil
}

func expandComptimeBlock(block ast.Block, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	ct := cloneComptimeBindings(comptime)
	rt := cloneRuntimeBindings(runtime)
	out := ast.Block{Statements: make([]ast.Stmt, 0, len(block.Statements))}
	for _, stmt := range block.Statements {
		expanded, err := expandComptimeStmt(stmt, ct, rt)
		if err != nil {
			return ast.Block{}, err
		}
		out.Statements = append(out.Statements, expanded...)
	}
	return out, nil
}

func expandComptimeStmt(stmt ast.Stmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) ([]ast.Stmt, error) {
	switch s := stmt.(type) {
	case ast.ComptimeLetStmt:
		value, err := evalComptimeExpr(s.Value, comptime, runtime, "comptime let initializer must be compile-time")
		if err != nil {
			return nil, err
		}
		comptime[s.Name] = comptimeBinding{typ: value.Type, int32: value.Int32, boolVal: value.Bool}
		return nil, nil
	case ast.ComptimeIfStmt:
		value, err := evalComptimeExpr(s.Condition, comptime, runtime, "comptime if condition must be compile-time bool")
		if err != nil {
			return nil, err
		}
		if value.Type.Name != "bool" {
			return nil, fmt.Errorf("comptime if condition must be compile-time bool")
		}
		selected := s.ElseBody
		if value.Bool {
			selected = &s.ThenBody
		}
		if selected == nil {
			return nil, nil
		}
		body, err := expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.ComptimeMatchStmt:
		body, err := expandComptimeMatch(s, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.ComptimeWhenUtilityStmt:
		body, err := expandComptimeWhenUtility(s, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return body.Statements, nil
	case ast.LetStmt:
		out := s
		out.Type = replaceComptimeTypeRef(s.Type, comptime)
		if s.Value != nil {
			out.Value = replaceComptimeExpr(s.Value, comptime)
		}
		runtime[s.Name] = runtimeBinding{origin: runtimeLocal}
		return []ast.Stmt{out}, nil
	case ast.AssignStmt:
		return []ast.Stmt{ast.AssignStmt{Target: replaceComptimeExpr(s.Target, comptime), Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return []ast.Stmt{s}, nil
		}
		return []ast.Stmt{ast.ReturnStmt{Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.ExprStmt:
		return []ast.Stmt{ast.ExprStmt{Value: replaceComptimeExpr(s.Value, comptime)}}, nil
	case ast.IfStmt:
		thenBody, err := expandRuntimeNestedBlock(s.ThenBody, comptime, runtime)
		if err != nil {
			return nil, err
		}
		var elseBody *ast.Block
		if s.ElseBody != nil {
			body, err := expandRuntimeNestedBlock(*s.ElseBody, comptime, runtime)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return []ast.Stmt{ast.IfStmt{Condition: replaceComptimeExpr(s.Condition, comptime), ThenBody: thenBody, ElseBody: elseBody}}, nil
	case ast.ForStmt:
		body, err := expandRuntimeNestedBlock(s.Body, comptime, runtime)
		if err != nil {
			return nil, err
		}
		return []ast.Stmt{ast.ForStmt{Attributes: append([]ast.Attribute(nil), s.Attributes...), Name: s.Name, Start: replaceComptimeExpr(s.Start, comptime), End: replaceComptimeExpr(s.End, comptime), Step: replaceComptimeExpr(s.Step, comptime), Body: body}}, nil
	case ast.StaticAssertStmt:
		value, err := evalComptimeExpr(s.Expr, comptime, runtime, "static assert must be compile-time")
		if err != nil {
			return nil, err
		}
		if value.Type.Name != "bool" {
			return nil, fmt.Errorf("static assert %s must evaluate to bool", s.Text)
		}
		if !value.Bool {
			return nil, fmt.Errorf("failed static assert %s", s.Text)
		}
		return nil, nil
	default:
		return []ast.Stmt{stmt}, nil
	}
}

func expandRuntimeNestedBlock(block ast.Block, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	return expandComptimeBlock(block, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func expandComptimeMatch(stmt ast.ComptimeMatchStmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	subject, err := evalComptimeExpr(stmt.Subject, comptime, runtime, "comptime match scrutinee must be compile-time")
	if err != nil {
		return ast.Block{}, err
	}
	if !consteval.IsInteger(subject.Type) && subject.Type.Name != "bool" {
		return ast.Block{}, fmt.Errorf("comptime match scrutinee must be compile-time")
	}
	seen := map[string]string{}
	hasTrue := false
	hasFalse := false
	var elseBody *ast.Block
	var selected *ast.Block
	for _, arm := range stmt.Arms {
		if arm.IsElse {
			if elseBody != nil {
				return ast.Block{}, fmt.Errorf("duplicate comptime match else arm")
			}
			body := arm.Body
			elseBody = &body
			continue
		}
		pattern, err := evalComptimeMatchPattern(arm.Pattern, comptime, runtime)
		if err != nil {
			return ast.Block{}, err
		}
		if !comptimePatternMatchesSubjectType(pattern, subject) {
			return ast.Block{}, fmt.Errorf("comptime match arm pattern type %s does not match scrutinee type %s", pattern.Type.Name, subject.Type.Name)
		}
		key := comptimeValueKey(pattern)
		label := comptimeValueLabel(pattern)
		if prior, ok := seen[key]; ok {
			return ast.Block{}, fmt.Errorf("duplicate comptime match arm for %s", prior)
		}
		seen[key] = label
		if pattern.Type.Name == "bool" {
			if pattern.Bool {
				hasTrue = true
			} else {
				hasFalse = true
			}
		}
		if selected == nil && comptimeValuesEqual(subject, pattern) {
			body := arm.Body
			selected = &body
		}
	}
	if subject.Type.Name == "bool" {
		if (!hasTrue || !hasFalse) && elseBody == nil {
			return ast.Block{}, fmt.Errorf("bool comptime match requires else arm unless true and false arms are both present")
		}
	} else if consteval.IsInteger(subject.Type) && elseBody == nil {
		return ast.Block{}, fmt.Errorf("comptime match over integer requires else arm")
	}
	if selected == nil {
		selected = elseBody
	}
	if selected == nil {
		return ast.Block{}, fmt.Errorf("no comptime match arm selected and no else arm provided")
	}
	return expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func expandComptimeWhenUtility(stmt ast.ComptimeWhenUtilityStmt, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (ast.Block, error) {
	seenLabels := map[string]struct{}{}
	var selected *ast.Block
	selectedLabel := ""
	selectedScore := int64(0)
	hasSelected := false
	tiedHighestLabel := ""
	for _, c := range stmt.Cases {
		if _, exists := seenLabels[c.Label]; exists {
			return ast.Block{}, fmt.Errorf("duplicate comptime when utility case label %s", c.Label)
		}
		seenLabels[c.Label] = struct{}{}
		eligible := true
		if c.Condition != nil {
			guard, err := evalComptimeExpr(c.Condition, comptime, runtime, "comptime when guard must be compile-time bool")
			if err != nil {
				return ast.Block{}, annotateComptimeWhenExprError(err, "guard")
			}
			if guard.Type.Name != "bool" {
				return ast.Block{}, fmt.Errorf("comptime when guard must be compile-time bool")
			}
			eligible = guard.Bool
		}
		if !eligible {
			continue
		}
		score, err := evalComptimeExpr(c.Score, comptime, runtime, "comptime when score must be compile-time numeric")
		if err != nil {
			return ast.Block{}, annotateComptimeWhenExprError(err, "score")
		}
		if !consteval.IsInteger(score.Type) {
			return ast.Block{}, fmt.Errorf("comptime when score must be compile-time numeric")
		}
		if hasSelected && score.Int32 == selectedScore {
			if tiedHighestLabel == "" {
				tiedHighestLabel = c.Label
			}
			continue
		}
		if !hasSelected || score.Int32 > selectedScore {
			body := c.Body
			selected = &body
			selectedLabel = c.Label
			selectedScore = score.Int32
			hasSelected = true
			tiedHighestLabel = ""
		}
	}
	if hasSelected && tiedHighestLabel != "" {
		return ast.Block{}, fmt.Errorf("ambiguous comptime when utility cases %s and %s have tied score %d", selectedLabel, tiedHighestLabel, selectedScore)
	}
	if !hasSelected {
		selected = stmt.ElseBody
	}
	if selected == nil {
		return ast.Block{}, fmt.Errorf("no comptime when utility case qualified and no else block provided")
	}
	return expandComptimeBlock(*selected, cloneComptimeBindings(comptime), cloneRuntimeBindings(runtime))
}

func annotateComptimeWhenExprError(err error, position string) error {
	msg := err.Error()
	const prefix = "comptime expression cannot reference "
	if strings.HasPrefix(msg, prefix) {
		return fmt.Errorf("comptime when %s cannot reference %s", position, strings.TrimPrefix(msg, prefix))
	}
	return err
}

func evalComptimeMatchPattern(expr ast.Expr, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding) (consteval.Value, error) {
	switch expr.(type) {
	case ast.IntegerLiteral, ast.BoolLiteral:
	default:
		return consteval.Value{}, fmt.Errorf("comptime match arm pattern must be compile-time literal")
	}
	value, err := evalComptimeExpr(expr, comptime, runtime, "comptime match arm pattern must be compile-time literal")
	if err != nil {
		return consteval.Value{}, err
	}
	if !consteval.IsInteger(value.Type) && value.Type.Name != "bool" {
		return consteval.Value{}, fmt.Errorf("comptime match arm pattern must be compile-time literal")
	}
	return value, nil
}

func comptimePatternMatchesSubjectType(pattern, subject consteval.Value) bool {
	if subject.Type.Name == "bool" || pattern.Type.Name == "bool" {
		return subject.Type.Name == pattern.Type.Name
	}
	return consteval.IsInteger(subject.Type) && consteval.IsInteger(pattern.Type)
}

func comptimeValuesEqual(left, right consteval.Value) bool {
	if left.Type.Name == "bool" && right.Type.Name == "bool" {
		return left.Bool == right.Bool
	}
	if consteval.IsInteger(left.Type) && consteval.IsInteger(right.Type) {
		return left.Int32 == right.Int32
	}
	return false
}

func comptimeValueKey(value consteval.Value) string {
	if value.Type.Name == "bool" {
		if value.Bool {
			return "bool:true"
		}
		return "bool:false"
	}
	return "int:" + strconv.FormatInt(value.Int32, 10)
}

func comptimeValueLabel(value consteval.Value) string {
	if value.Type.Name == "bool" {
		if value.Bool {
			return "true"
		}
		return "false"
	}
	text := strconv.FormatInt(value.Int32, 10)
	if value.Type.Name == "u32" {
		text += "u"
	}
	return text
}

func evalComptimeExpr(expr ast.Expr, comptime map[string]comptimeBinding, runtime map[string]runtimeBinding, context string) (consteval.Value, error) {
	if err := rejectRuntimeComptimeRefs(expr, runtime); err != nil {
		return consteval.Value{}, err
	}
	env := map[string]consteval.Value{}
	for name, value := range comptime {
		env[name] = consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true}
	}
	value, err := consteval.Eval(expr, env)
	if err != nil {
		return consteval.Value{}, fmt.Errorf("%s: %w", context, err)
	}
	return value, nil
}

func rejectRuntimeComptimeRefs(expr ast.Expr, runtime map[string]runtimeBinding) error {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if binding, ok := runtime[e.Name]; ok {
			return fmt.Errorf("comptime expression cannot reference %s `%s`", binding.origin, e.Name)
		}
	case ast.FieldAccessExpr:
		if root, ok := rootNameForComptime(e); ok {
			if binding, exists := runtime[root]; exists {
				return fmt.Errorf("comptime expression cannot reference %s `%s`", binding.origin, fieldPathForComptime(e))
			}
		}
		return rejectRuntimeComptimeRefs(e.Target, runtime)
	case ast.IndexExpr:
		if err := rejectRuntimeComptimeRefs(e.Target, runtime); err != nil {
			return err
		}
		if err := rejectRuntimeComptimeRefs(e.Index, runtime); err != nil {
			return err
		}
		if e.HasSecond {
			return rejectRuntimeComptimeRefs(e.Index2, runtime)
		}
	case ast.CallExpr:
		return fmt.Errorf("comptime expression cannot call functions in SDSL-V M13")
	case ast.BinaryExpr:
		if err := rejectRuntimeComptimeRefs(e.Left, runtime); err != nil {
			return err
		}
		return rejectRuntimeComptimeRefs(e.Right, runtime)
	case ast.UnaryExpr:
		return rejectRuntimeComptimeRefs(e.Operand, runtime)
	case ast.ParenExpr:
		return rejectRuntimeComptimeRefs(e.Inner, runtime)
	case ast.ReductionExpr, ast.MatchExpr, ast.WithExpr, ast.EnumConstructExpr:
		return fmt.Errorf("comptime expression cannot use runtime expression forms in SDSL-V M13")
	}
	return nil
}

func replaceComptimeExpr(expr ast.Expr, comptime map[string]comptimeBinding) ast.Expr {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if value, ok := comptime[e.Name]; ok {
			return literalExprForValue(specializeValue{typ: value.typ, int32: value.int32, boolVal: value.boolVal})
		}
		return expr
	case ast.FieldAccessExpr:
		return ast.FieldAccessExpr{Target: replaceComptimeExpr(e.Target, comptime), Field: e.Field}
	case ast.IndexExpr:
		out := ast.IndexExpr{Target: replaceComptimeExpr(e.Target, comptime), Index: replaceComptimeExpr(e.Index, comptime), HasSecond: e.HasSecond}
		if e.HasSecond {
			out.Index2 = replaceComptimeExpr(e.Index2, comptime)
		}
		return out
	case ast.CallExpr:
		args := make([]ast.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, replaceComptimeExpr(arg, comptime))
		}
		return ast.CallExpr{Callee: replaceComptimeExpr(e.Callee, comptime), Arguments: args}
	case ast.BinaryExpr:
		return ast.BinaryExpr{Left: replaceComptimeExpr(e.Left, comptime), Operator: e.Operator, Right: replaceComptimeExpr(e.Right, comptime)}
	case ast.UnaryExpr:
		return ast.UnaryExpr{Operator: e.Operator, Operand: replaceComptimeExpr(e.Operand, comptime)}
	case ast.ParenExpr:
		return ast.ParenExpr{Inner: replaceComptimeExpr(e.Inner, comptime)}
	case ast.WhenUtilityExpr:
		cases := make([]ast.UtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			cases = append(cases, ast.UtilityCase{Value: replaceComptimeExpr(c.Value, comptime), Condition: replaceComptimeExpr(c.Condition, comptime), Score: replaceComptimeExpr(c.Score, comptime)})
		}
		return ast.WhenUtilityExpr{Cases: cases, Else: replaceComptimeExpr(e.Else, comptime)}
	case ast.WithExpr:
		updates := make([]ast.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			updates = append(updates, ast.FieldUpdate{Name: update.Name, Value: replaceComptimeExpr(update.Value, comptime)})
		}
		return ast.WithExpr{Base: replaceComptimeExpr(e.Base, comptime), Updates: updates}
	case ast.EnumConstructExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: replaceComptimeExpr(field.Value, comptime)})
		}
		return ast.EnumConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Fields: fields}
	case ast.MatchExpr:
		arms := make([]ast.MatchArm, 0, len(e.Arms))
		for _, arm := range e.Arms {
			arms = append(arms, ast.MatchArm{EnumName: arm.EnumName, VariantName: arm.VariantName, BindingName: arm.BindingName, Value: replaceComptimeExpr(arm.Value, comptime)})
		}
		return ast.MatchExpr{Subject: replaceComptimeExpr(e.Subject, comptime), Arms: arms}
	case ast.ReductionExpr:
		return ast.ReductionExpr{Attributes: append([]ast.Attribute(nil), e.Attributes...), Op: e.Op, Name: e.Name, Start: replaceComptimeExpr(e.Start, comptime), End: replaceComptimeExpr(e.End, comptime), Step: replaceComptimeExpr(e.Step, comptime), Body: replaceComptimeExpr(e.Body, comptime)}
	default:
		return expr
	}
}

func cloneComptimeBindings(in map[string]comptimeBinding) map[string]comptimeBinding {
	out := make(map[string]comptimeBinding, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneRuntimeBindings(in map[string]runtimeBinding) map[string]runtimeBinding {
	out := make(map[string]runtimeBinding, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func rootNameForComptime(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name, true
	case ast.FieldAccessExpr:
		return rootNameForComptime(e.Target)
	default:
		return "", false
	}
}

func fieldPathForComptime(expr ast.Expr) string {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name
	case ast.FieldAccessExpr:
		return fieldPathForComptime(e.Target) + "." + e.Field
	default:
		return ""
	}
}

const maxRegTileElements = 64

func specializeTypeRef(ref ast.TypeRef, env map[string]specializeValue) (ast.TypeRef, error) {
	out := ref
	if ref.Name == "array" || ref.Name == "tile" || ref.Name == "reg_tile" || ref.Name == "matrix_view" {
		args := make([]ast.TypeRef, len(ref.Args))
		for i, arg := range ref.Args {
			next, err := specializeTypeRef(arg, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			args[i] = next
		}
		out.Args = args
		if ref.HasArraySize {
			sizeExpr, err := specializeIntExpr(ref.ArraySize, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			out.ArraySize = sizeExpr
		}
		if ref.HasTileShape {
			rows, err := specializeIntExpr(ref.TileRows, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			cols, err := specializeIntExpr(ref.TileCols, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			out.TileRows = rows
			out.TileCols = cols
			if ref.Name == "reg_tile" {
				rowsValue := mustConcreteInt(rows)
				colsValue := mustConcreteInt(cols)
				if rowsValue*colsValue > maxRegTileElements {
					return ast.TypeRef{}, fmt.Errorf("reg_tile has %d elements; M15 limit is %d", rowsValue*colsValue, maxRegTileElements)
				}
			}
		}
	}
	return out, nil
}

func specializeLocalTypeRef(ref ast.TypeRef, env map[string]specializeValue) (ast.TypeRef, error) {
	out := ref
	if ref.Name == "array" || ref.Name == "tile" || ref.Name == "reg_tile" || ref.Name == "matrix_view" {
		args := make([]ast.TypeRef, len(ref.Args))
		for i, arg := range ref.Args {
			next, err := specializeLocalTypeRef(arg, env)
			if err != nil {
				return ast.TypeRef{}, err
			}
			args[i] = next
		}
		out.Args = args
		if ref.HasArraySize {
			out.ArraySize = specializeExpr(ref.ArraySize, env)
		}
		if ref.HasTileShape {
			out.TileRows = specializeExpr(ref.TileRows, env)
			out.TileCols = specializeExpr(ref.TileCols, env)
		}
	}
	return out, nil
}

func specializeIntExpr(expr ast.Expr, env map[string]specializeValue) (ast.Expr, error) {
	value, err := evalSpecializeConstExpr(expr, env)
	if err != nil {
		return nil, err
	}
	if value.int32 <= 0 {
		return nil, fmt.Errorf("expected positive integer constant expression")
	}
	return literalExprForValue(value), nil
}

func specializeExpr(expr ast.Expr, env map[string]specializeValue) ast.Expr {
	switch e := expr.(type) {
	case ast.FieldAccessExpr:
		if path, ok := specializeFieldPath(e); ok {
			if value, ok := env[path]; ok {
				return literalExprForValue(value)
			}
		}
		return ast.FieldAccessExpr{Target: specializeExpr(e.Target, env), Field: e.Field}
	case ast.IndexExpr:
		out := ast.IndexExpr{Target: specializeExpr(e.Target, env), Index: specializeExpr(e.Index, env), HasSecond: e.HasSecond}
		if e.HasSecond {
			out.Index2 = specializeExpr(e.Index2, env)
		}
		return out
	case ast.CallExpr:
		args := make([]ast.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, specializeExpr(arg, env))
		}
		return ast.CallExpr{Callee: specializeExpr(e.Callee, env), Arguments: args}
	case ast.BinaryExpr:
		return ast.BinaryExpr{Left: specializeExpr(e.Left, env), Operator: e.Operator, Right: specializeExpr(e.Right, env)}
	case ast.UnaryExpr:
		return ast.UnaryExpr{Operator: e.Operator, Operand: specializeExpr(e.Operand, env)}
	case ast.ParenExpr:
		return ast.ParenExpr{Inner: specializeExpr(e.Inner, env)}
	case ast.WhenUtilityExpr:
		cases := make([]ast.UtilityCase, 0, len(e.Cases))
		for _, c := range e.Cases {
			cases = append(cases, ast.UtilityCase{Value: specializeExpr(c.Value, env), Condition: specializeExpr(c.Condition, env), Score: specializeExpr(c.Score, env)})
		}
		return ast.WhenUtilityExpr{Cases: cases, Else: specializeExpr(e.Else, env)}
	case ast.WithExpr:
		updates := make([]ast.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			updates = append(updates, ast.FieldUpdate{Name: update.Name, Value: specializeExpr(update.Value, env)})
		}
		return ast.WithExpr{Base: specializeExpr(e.Base, env), Updates: updates}
	case ast.EnumConstructExpr:
		fields := make([]ast.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			fields = append(fields, ast.FieldInit{Name: field.Name, Value: specializeExpr(field.Value, env)})
		}
		return ast.EnumConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Fields: fields}
	case ast.MatchExpr:
		arms := make([]ast.MatchArm, 0, len(e.Arms))
		for _, arm := range e.Arms {
			arms = append(arms, ast.MatchArm{EnumName: arm.EnumName, VariantName: arm.VariantName, BindingName: arm.BindingName, Value: specializeExpr(arm.Value, env)})
		}
		return ast.MatchExpr{Subject: specializeExpr(e.Subject, env), Arms: arms}
	case ast.ReductionExpr:
		return ast.ReductionExpr{
			Attributes: append([]ast.Attribute(nil), e.Attributes...),
			Op:         e.Op,
			Name:       e.Name,
			Start:      specializeExpr(e.Start, env),
			End:        specializeExpr(e.End, env),
			Step:       specializeExpr(e.Step, env),
			Body:       specializeExpr(e.Body, env),
		}
	default:
		return expr
	}
}

func evalSpecializeConstExpr(expr ast.Expr, env map[string]specializeValue) (specializeValue, error) {
	ctEnv := map[string]consteval.Value{}
	for key, value := range env {
		ctEnv[key] = consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true}
	}
	value, err := consteval.Eval(expr, ctEnv)
	if err != nil {
		return specializeValue{}, err
	}
	return specializeValue{typ: value.Type, int32: value.Int32, boolVal: value.Bool}, nil
}

func literalExprForValue(value specializeValue) ast.Expr {
	return consteval.LiteralExpr(consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true})
}

func mustConcreteInt(expr ast.Expr) int {
	value, err := evalSpecializeConstExpr(expr, nil)
	if err != nil {
		panic(err)
	}
	return int(value.int32)
}

type fieldInfo struct {
	access     string
	typ        ast.TypeRef
	attributes []ast.Attribute
}

type enumVariantInfo struct {
	name         string
	fields       map[string]fieldInfo
	fieldOrder   []string
	payloadType  string
	hasPayload   bool
	variantIndex int
}

type typeInfo struct {
	kind         string
	target       ast.TypeRef
	fields       map[string]fieldInfo
	enumVariants map[string]enumVariantInfo
}

type functionInfo struct {
	name        string
	emittedName string
	returnType  ast.TypeRef
	params      []ast.Parameter
	shaderName  string
}

type binding struct {
	name   string
	kind   vdmir.VarKind
	typ    ast.TypeRef
	access string
}

type lowering struct {
	provenance vdmir.Provenance
	types      map[string]typeInfo
	functions  map[string]functionInfo
}

func (l *lowering) seedBuiltins() {
	for _, name := range []string{"void", "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4"} {
		l.types[name] = typeInfo{kind: "builtin"}
	}
	l.types["uint2"] = builtinUintVectorType(2)
	l.types["uint3"] = builtinUintVectorType(3)
	l.types["uint4"] = builtinUintVectorType(4)
}

func (l *lowering) collect(module ast.Module) {
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			l.types[d.Name] = typeInfo{kind: "alias", target: d.Type}
		case ast.RecordDecl:
			l.types[d.Name] = typeInfo{kind: "record", fields: collectFields(d.Fields)}
		case ast.StreamDecl:
			l.types[d.Name] = typeInfo{kind: "stream", fields: collectFields(d.Fields)}
		case ast.ConceptDecl:
			l.types[d.Name] = typeInfo{kind: "concept", fields: collectConceptFields(d)}
		case ast.EnumDecl:
			l.collectEnum(d)
		case ast.FunctionDecl:
			l.functions[d.Name] = functionInfo{name: d.Name, emittedName: d.Name, returnType: d.ReturnType, params: d.Parameters}
		case ast.ShaderDecl:
			if d.Template != nil {
				continue
			}
			for _, method := range d.Methods {
				key := d.Name + "_" + method.Name
				l.functions[key] = functionInfo{
					name:        method.Name,
					emittedName: key,
					returnType:  method.ReturnType,
					params:      method.Parameters,
					shaderName:  d.Name,
				}
			}
		case ast.ConfigDecl, ast.CompileDecl:
			continue
		}
	}
}

func (l *lowering) collectEnum(enum ast.EnumDecl) {
	variants := map[string]enumVariantInfo{}
	for i, variant := range enum.Variants {
		fields := collectFields(variant.Fields)
		order := make([]string, 0, len(variant.Fields))
		for _, field := range variant.Fields {
			order = append(order, field.Name)
		}
		info := enumVariantInfo{
			name:         variant.Name,
			fields:       fields,
			fieldOrder:   order,
			hasPayload:   variant.Payload,
			variantIndex: i,
		}
		if variant.Payload {
			info.payloadType = payloadTypeName(enum.Name, variant.Name)
			l.types[info.payloadType] = typeInfo{kind: "record", fields: fields}
		}
		variants[variant.Name] = info
	}
	l.types[enum.Name] = typeInfo{kind: "enum", enumVariants: variants}
}

func (l *lowering) lowerEnum(enum ast.EnumDecl) vdmir.Enum {
	out := vdmir.Enum{Provenance: l.provenance, Name: enum.Name}
	for _, variant := range enum.Variants {
		entry := vdmir.EnumVariant{Name: variant.Name, HasPayload: variant.Payload}
		for _, field := range variant.Fields {
			entry.Payload = append(entry.Payload, vdmir.Field{
				Provenance: l.provenance,
				Name:       field.Name,
				Type:       l.lowerTypeRef(field.Type),
			})
		}
		out.Variants = append(out.Variants, entry)
	}
	return out
}

func (l *lowering) lowerRecord(name string, fields []ast.Field) vdmir.Record {
	record := vdmir.Record{Provenance: l.provenance, Name: name}
	for _, field := range fields {
		record.Fields = append(record.Fields, vdmir.Field{
			Provenance: l.provenance,
			Name:       field.Name,
			Type:       l.lowerTypeRef(field.Type),
		})
	}
	return record
}

func (l *lowering) lowerStream(name string, fields []ast.Field) vdmir.Stream {
	stream := vdmir.Stream{Provenance: l.provenance, Name: name}
	for _, field := range fields {
		if field.Access != "" {
			continue
		}
		stream.Fields = append(stream.Fields, vdmir.Field{
			Provenance: l.provenance,
			Name:       field.Name,
			Type:       l.lowerTypeRef(field.Type),
		})
	}
	return stream
}

func (l *lowering) resolveShaderResources(shader ast.ShaderDecl) ([]ast.ResourceDecl, string, error) {
	if shader.ResourceBundleName == "" {
		return append([]ast.ResourceDecl(nil), shader.Resources...), "", nil
	}
	info, ok := l.types[shader.ResourceBundleName]
	if !ok || info.kind != "stream" {
		return nil, "", fmt.Errorf("unknown stream resource bundle %s", shader.ResourceBundleName)
	}
	names := make([]string, 0, len(info.fields))
	for name := range info.fields {
		names = append(names, name)
	}
	sortStrings(names)
	resources := make([]ast.ResourceDecl, 0, len(names))
	for _, name := range names {
		field := info.fields[name]
		if field.access == "" {
			continue
		}
		resources = append(resources, ast.ResourceDecl{Name: name, Access: field.access, Type: field.typ})
	}
	return resources, shader.ResourceBundleName, nil
}

func (l *lowering) lowerFunction(shaderName string, fn ast.FunctionDecl, resources []ast.ResourceDecl, workgroups []ast.WorkgroupDecl) (vdmir.Function, error) {
	out := vdmir.Function{
		Provenance: l.provenance,
		Name:       fn.Name,
		ShaderName: shaderName,
		ReturnType: l.lowerTypeRef(fn.ReturnType),
	}
	if shaderName != "" {
		out.EmittedName = shaderName + "_" + fn.Name
	} else {
		out.EmittedName = fn.Name
	}

	scope := map[string]binding{}
	addBuiltinBindings(scope)
	for _, resource := range resources {
		scope[resource.Name] = binding{name: resource.Name, kind: vdmir.VarResource, typ: resource.Type, access: resource.Access}
	}
	for _, workgroup := range workgroups {
		scope[workgroup.Name] = binding{name: workgroup.Name, kind: vdmir.VarLocal, typ: workgroup.Type}
	}
	for _, param := range fn.Parameters {
		scope[param.Name] = binding{name: param.Name, kind: vdmir.VarParam, typ: param.Type}
		out.Params = append(out.Params, vdmir.Parameter{
			Provenance: l.provenance,
			Name:       param.Name,
			Type:       l.lowerTypeRef(param.Type),
		})
	}

	locals := map[string]vdmir.Type{}
	body, err := l.lowerBlock(fn.Body, scope, locals, shaderName)
	if err != nil {
		return vdmir.Function{}, err
	}
	out.Body = body
	out.Locals = collectLocals(locals, l.provenance)
	return out, nil
}

func (l *lowering) lowerBlock(block ast.Block, scope map[string]binding, locals map[string]vdmir.Type, shaderName string) (vdmir.Block, error) {
	out := vdmir.Block{}
	for _, stmt := range block.Statements {
		lowered, err := l.lowerStmt(stmt, scope, locals, shaderName)
		if err != nil {
			return vdmir.Block{}, err
		}
		out.Statements = append(out.Statements, lowered)
	}
	return out, nil
}

func (l *lowering) lowerStmt(stmt ast.Stmt, scope map[string]binding, locals map[string]vdmir.Type, shaderName string) (vdmir.Stmt, error) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		loweredType := l.lowerTypeRef(s.Type)
		var value vdmir.Expr
		var err error
		if isRegTileZeroCall(s.Value) {
			value = vdmir.RegTileZeroExpr{Provenance: l.provenance, ExprType: loweredType}
		} else if s.Value != nil {
			value, err = l.lowerExpr(s.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
		}
		access := s.Type.Access
		if rowMajor, ok := value.(vdmir.RowMajorViewExpr); ok {
			loweredType = rowMajor.Type()
			access = string(rowMajor.Access)
		}
		scope[s.Name] = binding{name: s.Name, kind: vdmir.VarLocal, typ: s.Type, access: access}
		locals[s.Name] = loweredType
		return vdmir.LetStmt{Provenance: l.provenance, Name: s.Name, Type: loweredType, Value: value}, nil
	case ast.AssignStmt:
		target, err := l.lowerExpr(s.Target, scope, shaderName)
		if err != nil {
			return nil, err
		}
		value, err := l.lowerExpr(s.Value, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.AssignStmt{Provenance: l.provenance, Target: target, Value: value}, nil
	case ast.ReturnStmt:
		if s.Value == nil {
			return vdmir.ReturnStmt{Provenance: l.provenance}, nil
		}
		value, err := l.lowerExpr(s.Value, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ReturnStmt{Provenance: l.provenance, Value: value}, nil
	case ast.ExprStmt:
		value, err := l.lowerExpr(s.Value, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ExprStmt{Provenance: l.provenance, Value: value}, nil
	case ast.IfStmt:
		cond, err := l.lowerExpr(s.Condition, scope, shaderName)
		if err != nil {
			return nil, err
		}
		thenBody, err := l.lowerBlock(s.ThenBody, cloneScope(scope), locals, shaderName)
		if err != nil {
			return nil, err
		}
		var elseBody *vdmir.Block
		if s.ElseBody != nil {
			body, err := l.lowerBlock(*s.ElseBody, cloneScope(scope), locals, shaderName)
			if err != nil {
				return nil, err
			}
			elseBody = &body
		}
		return vdmir.IfStmt{Provenance: l.provenance, Condition: cond, ThenBody: thenBody, ElseBody: elseBody}, nil
	case ast.ForStmt:
		start, err := l.lowerExpr(s.Start, scope, shaderName)
		if err != nil {
			return nil, err
		}
		end, err := l.lowerExpr(s.End, scope, shaderName)
		if err != nil {
			return nil, err
		}
		step, err := l.lowerExpr(s.Step, scope, shaderName)
		if err != nil {
			return nil, err
		}
		loopType := start.Type()
		loopScope := cloneScope(scope)
		loopScope[s.Name] = binding{name: s.Name, kind: vdmir.VarLocal, typ: astTypeFromVDMIR(loopType)}
		locals[s.Name] = loopType
		body, err := l.lowerBlock(s.Body, loopScope, locals, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ForRangeStmt{
			Provenance: l.provenance,
			LoopHint:   lowerLoopHint(s.Attributes),
			Name:       s.Name,
			Type:       loopType,
			Start:      start,
			End:        end,
			Step:       step,
			Body:       body,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported statement in GoOct SDSL-V M3")
	}
}

func (l *lowering) lowerExpr(expr ast.Expr, scope map[string]binding, shaderName string) (vdmir.Expr, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		typ := vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			typ = vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
		}
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: typ, Kind: vdmir.LiteralInteger, Value: e.Value}, nil
	case ast.FloatLiteral:
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}, Kind: vdmir.LiteralFloat, Value: e.Value}, nil
	case ast.BoolLiteral:
		value := "false"
		if e.Value {
			value = "true"
		}
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}, Kind: vdmir.LiteralBool, Value: value}, nil
	case ast.StringLiteral:
		return vdmir.LiteralExpr{Provenance: l.provenance, ExprType: vdmir.Type{Kind: vdmir.TypeBuiltin, Name: "string"}, Kind: vdmir.LiteralString, Value: fmt.Sprintf("%q", e.Value)}, nil
	case ast.IdentifierExpr:
		if b, ok := scope[e.Name]; ok {
			return vdmir.VarRefExpr{Provenance: l.provenance, ExprType: l.lowerTypeRef(l.resolveAlias(b.typ)), Name: e.Name, Kind: b.kind}, nil
		}
		if fn, ok := l.lookupFunction(shaderName, e.Name); ok {
			return vdmir.VarRefExpr{Provenance: l.provenance, ExprType: l.lowerTypeRef(fn.returnType), Name: fn.emittedName, Kind: vdmir.VarFunction}, nil
		}
		return nil, fmt.Errorf("unknown identifier %s", e.Name)
	case ast.FieldAccessExpr:
		if id, ok := e.Target.(ast.IdentifierExpr); ok {
			if enumInfo, exists := l.types[id.Name]; exists && enumInfo.kind == "enum" {
				return vdmir.EnumConstructExpr{
					Provenance:  l.provenance,
					ExprType:    l.lowerTypeRef(ast.TypeRef{Name: id.Name}),
					EnumName:    id.Name,
					VariantName: e.Field,
				}, nil
			}
		}
		target, err := l.lowerExpr(e.Target, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.FieldAccessExpr{
			Provenance: l.provenance,
			ExprType:   l.lowerTypeRef(l.fieldType(target.Type(), e.Field)),
			Target:     target,
			Field:      e.Field,
		}, nil
	case ast.IndexExpr:
		target, err := l.lowerExpr(e.Target, scope, shaderName)
		if err != nil {
			return nil, err
		}
		index, err := l.lowerExpr(e.Index, scope, shaderName)
		if err != nil {
			return nil, err
		}
		if e.HasSecond {
			index2, err := l.lowerExpr(e.Index2, scope, shaderName)
			if err != nil {
				return nil, err
			}
			return vdmir.Index2DExpr{
				Provenance: l.provenance,
				ExprType:   elementType(target.Type()),
				Target:     target,
				Row:        index,
				Col:        index2,
				Stride:     tileOrViewStride(target.Type()),
			}, nil
		}
		return vdmir.IndexExpr{
			Provenance: l.provenance,
			ExprType:   elementType(target.Type()),
			Target:     target,
			Index:      index,
		}, nil
	case ast.CallExpr:
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && id.Name == "reg_tile_zero" {
			return nil, fmt.Errorf("reg_tile_zero() is only supported as a direct reg_tile local initializer")
		}
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && id.Name == "row_major" {
			args := make([]vdmir.Expr, 0, len(e.Arguments))
			for _, arg := range e.Arguments {
				lowered, err := l.lowerExpr(arg, scope, shaderName)
				if err != nil {
					return nil, err
				}
				args = append(args, lowered)
			}
			access := vdmir.ResourceReadOnly
			if ref, ok := args[0].(vdmir.VarRefExpr); ok {
				if b, exists := scope[ref.Name]; exists && b.access == "readwrite" {
					access = vdmir.ResourceReadWrite
				}
			}
			elem := elementType(args[0].Type())
			viewType := vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: access}
			return vdmir.RowMajorViewExpr{
				Provenance: l.provenance,
				ExprType:   viewType,
				Buffer:     args[0],
				Rows:       args[1],
				Cols:       args[2],
				Access:     access,
			}, nil
		}
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && isBarrierBuiltin(id.Name) {
			args := make([]vdmir.Expr, 0, len(e.Arguments))
			for _, arg := range e.Arguments {
				lowered, err := l.lowerExpr(arg, scope, shaderName)
				if err != nil {
					return nil, err
				}
				args = append(args, lowered)
			}
			return vdmir.IntrinsicCallExpr{
				Provenance: l.provenance,
				ExprType:   vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"},
				Intrinsic:  lowerIntrinsic(id.Name),
				Arguments:  args,
			}, nil
		}
		callee, err := l.lowerExpr(e.Callee, scope, shaderName)
		if err != nil {
			return nil, err
		}
		args := make([]vdmir.Expr, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			lowered, err := l.lowerExpr(arg, scope, shaderName)
			if err != nil {
				return nil, err
			}
			args = append(args, lowered)
		}
		return vdmir.CallExpr{
			Provenance: l.provenance,
			ExprType:   l.callResultType(e, scope, shaderName),
			Callee:     callee,
			Arguments:  args,
		}, nil
	case ast.BinaryExpr:
		left, err := l.lowerExpr(e.Left, scope, shaderName)
		if err != nil {
			return nil, err
		}
		right, err := l.lowerExpr(e.Right, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.BinaryExpr{
			Provenance: l.provenance,
			ExprType:   binaryResultType(left.Type(), e.Operator, right.Type()),
			Left:       left,
			Operator:   e.Operator,
			Right:      right,
		}, nil
	case ast.UnaryExpr:
		operand, err := l.lowerExpr(e.Operand, scope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.UnaryExpr{
			Provenance: l.provenance,
			ExprType:   operand.Type(),
			Operator:   e.Operator,
			Operand:    operand,
		}, nil
	case ast.ParenExpr:
		return l.lowerExpr(e.Inner, scope, shaderName)
	case ast.EnumConstructExpr:
		fields := make([]vdmir.FieldInit, 0, len(e.Fields))
		for _, field := range e.Fields {
			value, err := l.lowerExpr(field.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
			fields = append(fields, vdmir.FieldInit{Name: field.Name, Value: value})
		}
		return vdmir.EnumConstructExpr{
			Provenance:  l.provenance,
			ExprType:    l.lowerTypeRef(ast.TypeRef{Name: e.EnumName}),
			EnumName:    e.EnumName,
			VariantName: e.VariantName,
			Fields:      fields,
		}, nil
	case ast.MatchExpr:
		subject, err := l.lowerExpr(e.Subject, scope, shaderName)
		if err != nil {
			return nil, err
		}
		enumInfo := l.types[subject.Type().Name]
		arms := make([]vdmir.MatchArm, 0, len(e.Arms))
		var resultType vdmir.Type
		for i, arm := range e.Arms {
			variant := enumInfo.enumVariants[arm.VariantName]
			armScope := cloneScope(scope)
			bindingType := vdmir.Type{}
			if arm.BindingName != "" {
				bindingType = l.lowerTypeRef(ast.TypeRef{Name: variant.payloadType})
				armScope[arm.BindingName] = binding{name: arm.BindingName, kind: vdmir.VarLocal, typ: ast.TypeRef{Name: variant.payloadType}}
			}
			value, err := l.lowerExpr(arm.Value, armScope, shaderName)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				resultType = value.Type()
			}
			arms = append(arms, vdmir.MatchArm{
				EnumName:     arm.EnumName,
				VariantName:  arm.VariantName,
				BindingName:  arm.BindingName,
				BindingType:  bindingType,
				VariantIndex: variant.variantIndex,
				Value:        value,
			})
		}
		return vdmir.MatchExpr{
			Provenance: l.provenance,
			ExprType:   resultType,
			Subject:    subject,
			Arms:       arms,
		}, nil
	case ast.WhenUtilityExpr:
		cases := make([]vdmir.WhenUtilityCase, 0, len(e.Cases))
		var resultType vdmir.Type
		for i, c := range e.Cases {
			value, err := l.lowerExpr(c.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
			guard, err := l.lowerExpr(c.Condition, scope, shaderName)
			if err != nil {
				return nil, err
			}
			score, err := l.lowerExpr(c.Score, scope, shaderName)
			if err != nil {
				return nil, err
			}
			if i == 0 {
				resultType = value.Type()
			}
			cases = append(cases, vdmir.WhenUtilityCase{
				Provenance: l.provenance,
				Value:      value,
				Guard:      guard,
				Score:      score,
			})
		}
		elseExpr, err := l.lowerExpr(e.Else, scope, shaderName)
		if err != nil {
			return nil, err
		}
		if resultType.Kind == "" {
			resultType = elseExpr.Type()
		}
		return vdmir.WhenUtilityExpr{
			Provenance: l.provenance,
			ExprType:   resultType,
			Cases:      cases,
			Else:       elseExpr,
		}, nil
	case ast.WithExpr:
		base, err := l.lowerExpr(e.Base, scope, shaderName)
		if err != nil {
			return nil, err
		}
		updates := make([]vdmir.FieldUpdate, 0, len(e.Updates))
		for _, update := range e.Updates {
			value, err := l.lowerExpr(update.Value, scope, shaderName)
			if err != nil {
				return nil, err
			}
			updates = append(updates, vdmir.FieldUpdate{Name: update.Name, Value: value})
		}
		return vdmir.WithExpr{
			Provenance: l.provenance,
			ExprType:   base.Type(),
			Base:       base,
			Updates:    updates,
		}, nil
	case ast.ReductionExpr:
		start, err := l.lowerExpr(e.Start, scope, shaderName)
		if err != nil {
			return nil, err
		}
		end, err := l.lowerExpr(e.End, scope, shaderName)
		if err != nil {
			return nil, err
		}
		step, err := l.lowerExpr(e.Step, scope, shaderName)
		if err != nil {
			return nil, err
		}
		bodyScope := cloneScope(scope)
		bodyScope[e.Name] = binding{name: e.Name, kind: vdmir.VarLocal, typ: astTypeFromVDMIR(start.Type())}
		body, err := l.lowerExpr(e.Body, bodyScope, shaderName)
		if err != nil {
			return nil, err
		}
		return vdmir.ReductionExpr{
			Provenance: l.provenance,
			ExprType:   body.Type(),
			LoopHint:   lowerLoopHint(e.Attributes),
			Op:         lowerReductionOp(e.Op),
			Name:       e.Name,
			IndexType:  start.Type(),
			Start:      start,
			End:        end,
			Step:       step,
			Body:       body,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported expression in GoOct SDSL-V M3")
	}
}

func (l *lowering) lowerComputeEntryPoint(shader ast.ShaderDecl, fn ast.FunctionDecl, resources []ast.ResourceDecl) vdmir.ComputeEntryPoint {
	entry := vdmir.ComputeEntryPoint{
		Provenance:   l.provenance,
		ShaderName:   shader.Name,
		FunctionName: fn.Name,
		EmittedName:  shader.Name + "_" + fn.Name,
		NumThreadsX:  mustConcreteInt(fn.NumThreads.X),
		NumThreadsY:  mustConcreteInt(fn.NumThreads.Y),
		NumThreadsZ:  mustConcreteInt(fn.NumThreads.Z),
		Metadata:     lowerEntryMetadata(shader.SpecializedConfig),
		ConfigValues: lowerConfigValues(shader.SpecializedConfig),
	}
	directBuiltins := referencedBuiltins(fn.Body)
	threadParams := l.computeThreadBindings(fn.Parameters)
	for _, param := range fn.Parameters {
		if binding, ok := computeThreadBindingForParam(param); ok {
			entry.ThreadParams = append(entry.ThreadParams, binding)
			continue
		}
		entry.Params = append(entry.Params, vdmir.Parameter{
			Provenance: l.provenance,
			Name:       param.Name,
			Type:       l.lowerTypeRef(param.Type),
		})
	}
	entry.Builtins = []vdmir.BuiltinParam{
		{Name: "DispatchThreadID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_DispatchThreadID", Builtin: vdmir.BuiltinDispatchThreadID, Available: true, Referenced: directBuiltins["DispatchThreadID"] || threadParams["DispatchThreadID"]},
		{Name: "GroupThreadID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_GroupThreadID", Builtin: vdmir.BuiltinGroupThreadID, Available: true, Referenced: directBuiltins["GroupThreadID"] || threadParams["GroupThreadID"]},
		{Name: "GroupID", Type: vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}, Semantic: "SV_GroupID", Builtin: vdmir.BuiltinGroupID, Available: true, Referenced: directBuiltins["GroupID"] || threadParams["GroupID"]},
		{Name: "GroupIndex", Type: vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}, Semantic: "SV_GroupIndex", Builtin: vdmir.BuiltinGroupIndex, Available: true, Referenced: directBuiltins["GroupIndex"] || threadParams["GroupIndex"]},
	}
	_ = resources
	return entry
}

func lowerEntryMetadata(config map[string]uint32) []vdmir.MetadataField {
	if len(config) == 0 {
		return nil
	}
	names := []string{
		"OUTPUTS_PER_INVOCATION_M",
		"OUTPUTS_PER_INVOCATION_N",
		"TILE_M",
		"TILE_N",
		"TILE_K",
		"UNROLL_K",
	}
	fields := make([]vdmir.MetadataField, 0, len(names))
	for _, name := range names {
		value, ok := config[name]
		if !ok {
			continue
		}
		fields = append(fields, vdmir.MetadataField{Name: name, Value: value})
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func lowerConfigValues(config map[string]uint32) []vdmir.MetadataField {
	if len(config) == 0 {
		return nil
	}
	names := make([]string, 0, len(config))
	for name := range config {
		names = append(names, name)
	}
	slices.Sort(names)
	fields := make([]vdmir.MetadataField, 0, len(names))
	for _, name := range names {
		fields = append(fields, vdmir.MetadataField{Name: name, Value: config[name]})
	}
	return fields
}

func (l *lowering) computeThreadBindings(params []ast.Parameter) map[string]bool {
	out := map[string]bool{}
	for _, param := range params {
		binding, ok := computeThreadBindingForParam(param)
		if !ok {
			continue
		}
		for _, field := range binding.Fields {
			out[field.BuiltinName] = true
		}
	}
	return out
}

func computeThreadBindingForParam(param ast.Parameter) (vdmir.ComputeThreadBinding, bool) {
	if param.Type.Name != "ComputeThread" {
		return vdmir.ComputeThreadBinding{}, false
	}
	return vdmir.ComputeThreadBinding{
		ParamName: param.Name,
		TypeName:  "ComputeThread",
		Fields: []vdmir.ComputeThreadFieldBinding{
			{FieldName: "DispatchId", BuiltinName: "DispatchThreadID"},
			{FieldName: "GroupId", BuiltinName: "GroupID"},
			{FieldName: "GroupThreadId", BuiltinName: "GroupThreadID"},
			{FieldName: "GroupIndex", BuiltinName: "GroupIndex"},
		},
	}, true
}

func (l *lowering) lowerTypeRef(ref ast.TypeRef) vdmir.Type {
	resolved := l.resolveAlias(ref)
	switch resolved.Name {
	case "void":
		return vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}
	case "bool":
		return vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
	case "i32":
		return vdmir.Type{Kind: vdmir.TypeI32, Name: "i32"}
	case "u32":
		return vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
	case "f32", "float":
		return vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
	case "uint2":
		return vdmir.Type{Kind: vdmir.TypeUint2, Name: "uint2"}
	case "uint3":
		return vdmir.Type{Kind: vdmir.TypeUint3, Name: "uint3"}
	case "uint4":
		return vdmir.Type{Kind: vdmir.TypeUint4, Name: "uint4"}
	case "float2":
		return vdmir.Type{Kind: vdmir.TypeFloat2, Name: "float2"}
	case "float3":
		return vdmir.Type{Kind: vdmir.TypeFloat3, Name: "float3"}
	case "float4":
		return vdmir.Type{Kind: vdmir.TypeFloat4, Name: "float4"}
	case "array":
		elem := l.lowerTypeRef(resolved.Args[0])
		if resolved.HasArraySize {
			return vdmir.Type{Kind: vdmir.TypeArray, Name: "array", Element: &elem, ArraySize: mustConcreteInt(resolved.ArraySize), HasArraySize: true}
		}
		return vdmir.Type{Kind: vdmir.TypeRuntimeArray, Name: "array", Element: &elem}
	case "tile":
		elem := l.lowerTypeRef(resolved.Args[0])
		rows := mustConcreteInt(resolved.TileRows)
		cols := mustConcreteInt(resolved.TileCols)
		return vdmir.Type{Kind: vdmir.TypeTile, Name: "tile", Element: &elem, Rows: rows, Cols: cols, ArraySize: rows * cols, HasArraySize: true}
	case "reg_tile":
		elem := l.lowerTypeRef(resolved.Args[0])
		rows := mustConcreteInt(resolved.TileRows)
		cols := mustConcreteInt(resolved.TileCols)
		return vdmir.Type{Kind: vdmir.TypeRegTile, Name: "reg_tile", Element: &elem, Rows: rows, Cols: cols, ArraySize: rows * cols, HasArraySize: true}
	case "matrix_view":
		elem := l.lowerTypeRef(resolved.Args[0])
		return vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: lowerResourceAccess(resolved.Access)}
	default:
		if info, ok := l.types[resolved.Name]; ok {
			switch info.kind {
			case "record":
				return vdmir.Type{Kind: vdmir.TypeRecord, Name: resolved.Name}
			case "stream":
				return vdmir.Type{Kind: vdmir.TypeStream, Name: resolved.Name}
			case "enum":
				return vdmir.Type{Kind: vdmir.TypeEnum, Name: resolved.Name}
			}
		}
		return vdmir.Type{Kind: vdmir.TypeBuiltin, Name: resolved.Name}
	}
}

func (l *lowering) lowerResourceElementType(ref ast.TypeRef) vdmir.Type {
	if len(ref.Args) == 0 {
		return vdmir.Type{}
	}
	return l.lowerTypeRef(ref.Args[0])
}

func (l *lowering) resolveAlias(ref ast.TypeRef) ast.TypeRef {
	info, ok := l.types[ref.Name]
	if !ok || info.kind != "alias" {
		return ref
	}
	return l.resolveAlias(info.target)
}

func (l *lowering) fieldType(target vdmir.Type, field string) ast.TypeRef {
	if target.Name == "uint2" || target.Name == "uint3" || target.Name == "uint4" {
		return ast.TypeRef{Name: "u32"}
	}
	info, ok := l.types[target.Name]
	if !ok || info.fields == nil {
		return ast.TypeRef{Name: "<error>"}
	}
	return l.resolveAlias(info.fields[field].typ)
}

func (l *lowering) lookupFunction(shaderName, name string) (functionInfo, bool) {
	if shaderName != "" {
		if info, ok := l.functions[shaderName+"_"+name]; ok {
			return info, true
		}
	}
	info, ok := l.functions[name]
	return info, ok
}

func (l *lowering) callResultType(call ast.CallExpr, scope map[string]binding, shaderName string) vdmir.Type {
	if id, ok := call.Callee.(ast.IdentifierExpr); ok {
		switch id.Name {
		case "float2", "float3", "float4", "uint2", "uint3", "uint4":
			return l.lowerTypeRef(ast.TypeRef{Name: id.Name})
		case "WorkgroupBarrier", "WorkgroupMemoryBarrier", "WorkgroupMemoryBarrierWithSync":
			return vdmir.Type{Kind: vdmir.TypeVoid, Name: "void"}
		case "row_major":
			if len(call.Arguments) == 3 {
				if first, ok := call.Arguments[0].(ast.IdentifierExpr); ok {
					if b, exists := scope[first.Name]; exists {
						elem := elementType(l.lowerTypeRef(b.typ))
						return vdmir.Type{Kind: vdmir.TypeMatrixView, Name: "matrix_view", Element: &elem, Access: lowerResourceAccess(b.access)}
					}
				}
			}
		}
		if info, ok := l.lookupFunction(shaderName, id.Name); ok {
			return l.lowerTypeRef(info.returnType)
		}
	}
	return vdmir.Type{}
}

func addBuiltinBindings(scope map[string]binding) {
	scope["DispatchThreadID"] = binding{name: "DispatchThreadID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupThreadID"] = binding{name: "GroupThreadID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupID"] = binding{name: "GroupID", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "uint3"}}
	scope["GroupIndex"] = binding{name: "GroupIndex", kind: vdmir.VarBuiltin, typ: ast.TypeRef{Name: "u32"}}
}

func collectLocals(locals map[string]vdmir.Type, provenance vdmir.Provenance) []vdmir.Local {
	names := make([]string, 0, len(locals))
	for name := range locals {
		names = append(names, name)
	}
	sortStrings(names)
	out := make([]vdmir.Local, 0, len(names))
	for _, name := range names {
		out = append(out, vdmir.Local{Provenance: provenance, Name: name, Type: locals[name]})
	}
	return out
}

func referencedBuiltins(block ast.Block) map[string]bool {
	out := map[string]bool{}
	for _, stmt := range block.Statements {
		walkStmt(stmt, out)
	}
	return out
}

func walkStmt(stmt ast.Stmt, builtins map[string]bool) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		if s.Value != nil {
			walkExpr(s.Value, builtins)
		}
	case ast.AssignStmt:
		walkExpr(s.Target, builtins)
		walkExpr(s.Value, builtins)
	case ast.ReturnStmt:
		if s.Value != nil {
			walkExpr(s.Value, builtins)
		}
	case ast.ExprStmt:
		walkExpr(s.Value, builtins)
	case ast.IfStmt:
		walkExpr(s.Condition, builtins)
		for _, nested := range s.ThenBody.Statements {
			walkStmt(nested, builtins)
		}
		if s.ElseBody != nil {
			for _, nested := range s.ElseBody.Statements {
				walkStmt(nested, builtins)
			}
		}
	case ast.ForStmt:
		walkExpr(s.Start, builtins)
		walkExpr(s.End, builtins)
		walkExpr(s.Step, builtins)
		for _, nested := range s.Body.Statements {
			walkStmt(nested, builtins)
		}
	}
}

func walkExpr(expr ast.Expr, builtins map[string]bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		switch e.Name {
		case "DispatchThreadID", "GroupThreadID", "GroupID", "GroupIndex":
			builtins[e.Name] = true
		}
	case ast.FieldAccessExpr:
		walkExpr(e.Target, builtins)
	case ast.IndexExpr:
		walkExpr(e.Target, builtins)
		walkExpr(e.Index, builtins)
		if e.HasSecond {
			walkExpr(e.Index2, builtins)
		}
	case ast.CallExpr:
		walkExpr(e.Callee, builtins)
		for _, arg := range e.Arguments {
			walkExpr(arg, builtins)
		}
	case ast.BinaryExpr:
		walkExpr(e.Left, builtins)
		walkExpr(e.Right, builtins)
	case ast.UnaryExpr:
		walkExpr(e.Operand, builtins)
	case ast.ParenExpr:
		walkExpr(e.Inner, builtins)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			walkExpr(field.Value, builtins)
		}
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			walkExpr(c.Value, builtins)
			walkExpr(c.Condition, builtins)
			walkExpr(c.Score, builtins)
		}
		if e.Else != nil {
			walkExpr(e.Else, builtins)
		}
	case ast.WithExpr:
		walkExpr(e.Base, builtins)
		for _, update := range e.Updates {
			walkExpr(update.Value, builtins)
		}
	case ast.MatchExpr:
		walkExpr(e.Subject, builtins)
		for _, arm := range e.Arms {
			walkExpr(arm.Value, builtins)
		}
	case ast.ReductionExpr:
		walkExpr(e.Start, builtins)
		walkExpr(e.End, builtins)
		walkExpr(e.Step, builtins)
		walkExpr(e.Body, builtins)
	}
}

func lowerResourceAccess(access string) vdmir.ResourceAccess {
	if access == "readwrite" {
		return vdmir.ResourceReadWrite
	}
	return vdmir.ResourceReadOnly
}

func elementType(t vdmir.Type) vdmir.Type {
	if t.Element == nil {
		return vdmir.Type{}
	}
	return *t.Element
}

func tileOrViewStride(t vdmir.Type) vdmir.Expr {
	if t.Kind == vdmir.TypeTile || t.Kind == vdmir.TypeRegTile {
		return vdmir.LiteralExpr{ExprType: vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}, Kind: vdmir.LiteralInteger, Value: strconv.Itoa(t.Cols)}
	}
	return nil
}

func binaryResultType(left vdmir.Type, op string, right vdmir.Type) vdmir.Type {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return vdmir.Type{Kind: vdmir.TypeBool, Name: "bool"}
	default:
		if left.Kind == vdmir.TypeF32 || right.Kind == vdmir.TypeF32 {
			return vdmir.Type{Kind: vdmir.TypeF32, Name: "f32"}
		}
		if left.Kind == vdmir.TypeU32 || right.Kind == vdmir.TypeU32 {
			return vdmir.Type{Kind: vdmir.TypeU32, Name: "u32"}
		}
		return left
	}
}

func astTypeFromVDMIR(t vdmir.Type) ast.TypeRef {
	switch t.Kind {
	case vdmir.TypeBool:
		return ast.TypeRef{Name: "bool"}
	case vdmir.TypeI32:
		return ast.TypeRef{Name: "i32"}
	case vdmir.TypeU32:
		return ast.TypeRef{Name: "u32"}
	case vdmir.TypeF32:
		return ast.TypeRef{Name: "f32"}
	case vdmir.TypeRegTile:
		if t.Element == nil {
			return ast.TypeRef{Name: "reg_tile"}
		}
		return ast.TypeRef{
			Name:         "reg_tile",
			Args:         []ast.TypeRef{astTypeFromVDMIR(*t.Element)},
			TileRows:     consteval.LiteralExpr(consteval.Value{Type: ast.TypeRef{Name: "u32"}, Int32: int64(t.Rows), IsKnown: true}),
			TileCols:     consteval.LiteralExpr(consteval.Value{Type: ast.TypeRef{Name: "u32"}, Int32: int64(t.Cols), IsKnown: true}),
			HasTileShape: true,
		}
	default:
		return ast.TypeRef{Name: t.Name}
	}
}

func isRegTileZeroCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Callee.(ast.IdentifierExpr)
	return ok && id.Name == "reg_tile_zero" && len(call.Arguments) == 0
}

func cloneScope(scope map[string]binding) map[string]binding {
	out := make(map[string]binding, len(scope))
	for k, v := range scope {
		out[k] = v
	}
	return out
}

func replaceComptimeTypeRef(ref ast.TypeRef, comptime map[string]comptimeBinding) ast.TypeRef {
	out := ref
	if len(ref.Args) > 0 {
		out.Args = make([]ast.TypeRef, 0, len(ref.Args))
		for _, arg := range ref.Args {
			out.Args = append(out.Args, replaceComptimeTypeRef(arg, comptime))
		}
	}
	if ref.HasArraySize {
		out.ArraySize = replaceComptimeExpr(ref.ArraySize, comptime)
	}
	if ref.HasTileShape {
		out.TileRows = replaceComptimeExpr(ref.TileRows, comptime)
		out.TileCols = replaceComptimeExpr(ref.TileCols, comptime)
	}
	return out
}

func payloadTypeName(enumName, variantName string) string {
	return enumName + "_" + variantName + "Payload"
}

func collectConceptFields(concept ast.ConceptDecl) map[string]fieldInfo {
	out := map[string]fieldInfo{}
	for _, spec := range conceptFieldSpecs(concept) {
		out[spec.Path] = fieldInfo{typ: spec.Type}
	}
	return out
}

func conceptFieldSpecs(concept ast.ConceptDecl) []conceptFieldSpec {
	var out []conceptFieldSpec
	var walk func([]ast.ConceptMember, []string)
	walk = func(members []ast.ConceptMember, prefix []string) {
		for _, member := range members {
			switch m := member.(type) {
			case ast.ConceptField:
				out = append(out, conceptFieldSpec{
					Path:         joinConceptPath(prefix, m.Name),
					Type:         m.Type,
					DefaultValue: m.DefaultValue,
					ZeroAllowed:  m.Type.ZeroAllowed,
				})
			case ast.ConceptGroup:
				walk(m.Members, append(append([]string(nil), prefix...), m.Name))
			}
		}
	}
	walk(concept.Members, nil)
	return out
}

func joinConceptPath(prefix []string, name string) string {
	if len(prefix) == 0 {
		return name
	}
	return strings.Join(append(append([]string(nil), prefix...), name), ".")
}

func expandConfig(concept ast.ConceptDecl, config ast.ConfigDecl) (map[string]specializeValue, error) {
	assignments := map[string]ast.Expr{}
	for _, field := range config.Fields {
		assignments[field.Path] = field.Value
	}
	values := map[string]specializeValue{}
	for _, spec := range conceptFieldSpecs(concept) {
		if expr, ok := assignments[spec.Path]; ok {
			value, err := evalSpecializeConstExpr(expr, values)
			if err != nil {
				return nil, fmt.Errorf("config %s field %s: %w", config.Name, spec.Path, err)
			}
			if spec.Type.Name == "u32" && !spec.ZeroAllowed && value.int32 == 0 {
				return nil, fmt.Errorf("config field %s is nonzero by default; use u32! if zero is intentional", spec.Path)
			}
			values[spec.Path] = value
			continue
		}
		if spec.DefaultValue == nil {
			return nil, fmt.Errorf("config field %s missing", spec.Path)
		}
		value, err := evalSpecializeConstExpr(spec.DefaultValue, values)
		if err != nil {
			return nil, fmt.Errorf("config %s field %s default: %w", config.Name, spec.Path, err)
		}
		if spec.Type.Name == "u32" && !spec.ZeroAllowed && value.int32 == 0 {
			return nil, fmt.Errorf("config field %s is nonzero by default; use u32! if zero is intentional", spec.Path)
		}
		values[spec.Path] = value
	}
	return values, nil
}

func specializeFieldPath(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.FieldAccessExpr:
		if id, ok := e.Target.(ast.IdentifierExpr); ok {
			return id.Name + "." + e.Field, true
		}
		prefix, ok := specializeFieldPath(e.Target)
		if !ok {
			return "", false
		}
		return prefix + "." + e.Field, true
	default:
		return "", false
	}
}

func flattenConfigName(path string) string {
	parts := strings.Split(path, ".")
	words := make([]string, 0, len(parts)*2)
	for _, part := range parts {
		words = append(words, splitConfigWords(part)...)
	}
	return strings.Join(words, "_")
}

func splitConfigWords(part string) []string {
	part = strings.ReplaceAll(part, "-", "_")
	if part == "" {
		return nil
	}
	var words []string
	start := 0
	for i := 1; i < len(part); i++ {
		prev := part[i-1]
		curr := part[i]
		next := byte(0)
		if i+1 < len(part) {
			next = part[i+1]
		}
		if curr == '_' {
			if start < i {
				words = append(words, strings.ToUpper(part[start:i]))
			}
			start = i + 1
			continue
		}
		if isLowerASCII(prev) && isUpperASCII(curr) || isUpperASCII(prev) && isUpperASCII(curr) && next != 0 && isLowerASCII(next) {
			words = append(words, strings.ToUpper(part[start:i]))
			start = i
		}
	}
	if start < len(part) {
		words = append(words, strings.ToUpper(part[start:]))
	}
	return words
}

func isLowerASCII(b byte) bool { return b >= 'a' && b <= 'z' }
func isUpperASCII(b byte) bool { return b >= 'A' && b <= 'Z' }

func collectFields(fields []ast.Field) map[string]fieldInfo {
	out := make(map[string]fieldInfo, len(fields))
	for _, field := range fields {
		out[field.Name] = fieldInfo{access: field.Access, typ: field.Type, attributes: field.Attributes}
	}
	return out
}

func lowerLoopHint(attributes []ast.Attribute) vdmir.LoopHint {
	for _, attr := range attributes {
		switch attr.Name {
		case "unroll":
			return vdmir.LoopHintUnroll
		case "loop":
			return vdmir.LoopHintLoop
		}
	}
	return vdmir.LoopHintNone
}

func lowerReductionOp(op ast.ReductionOp) vdmir.ReductionOp {
	switch op {
	case ast.ReductionSum:
		return vdmir.ReductionSum
	case ast.ReductionProduct:
		return vdmir.ReductionProduct
	case ast.ReductionMax:
		return vdmir.ReductionMax
	case ast.ReductionMin:
		return vdmir.ReductionMin
	default:
		return ""
	}
}

func resolveResourceBinding(resources []ast.ResourceDecl, index int) vdmir.Binding {
	used := map[int]struct{}{}
	for _, resource := range resources {
		if binding, ok := explicitBinding(resource.Attributes); ok {
			used[binding] = struct{}{}
		}
	}
	resource := resources[index]
	if binding, ok := explicitBinding(resource.Attributes); ok {
		return vdmir.Binding{Set: 0, Binding: binding, Explicit: true}
	}
	next := 0
	for i := 0; i < index; i++ {
		if binding, ok := explicitBinding(resources[i].Attributes); ok {
			used[binding] = struct{}{}
			continue
		}
		for {
			if _, exists := used[next]; !exists {
				used[next] = struct{}{}
				break
			}
			next++
		}
	}
	for {
		if _, exists := used[next]; !exists {
			return vdmir.Binding{Set: 0, Binding: next}
		}
		next++
	}
}

func explicitBinding(attributes []ast.Attribute) (int, bool) {
	for _, attr := range attributes {
		if attr.Name != "binding" || len(attr.Arguments) != 1 {
			continue
		}
		lit, ok := attr.Arguments[0].(ast.IntegerLiteral)
		if !ok {
			return 0, false
		}
		value, err := strconv.Atoi(strings.TrimRight(lit.Value, "uU"))
		if err != nil {
			return 0, false
		}
		return value, true
	}
	return 0, false
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func builtinUintVectorType(dim int) typeInfo {
	fields := map[string]fieldInfo{
		"x": {typ: ast.TypeRef{Name: "u32"}},
		"y": {typ: ast.TypeRef{Name: "u32"}},
		"z": {typ: ast.TypeRef{Name: "u32"}},
		"w": {typ: ast.TypeRef{Name: "u32"}},
	}
	trimmed := map[string]fieldInfo{}
	for _, name := range []string{"x", "y", "z", "w"}[:dim] {
		trimmed[name] = fields[name]
	}
	return typeInfo{kind: "builtin", fields: trimmed}
}

func isBarrierBuiltin(name string) bool {
	return name == "WorkgroupBarrier" || name == "WorkgroupMemoryBarrier" || name == "WorkgroupMemoryBarrierWithSync"
}

func lowerIntrinsic(name string) vdmir.Intrinsic {
	switch name {
	case "WorkgroupBarrier":
		return vdmir.IntrinsicWorkgroupBarrier
	case "WorkgroupMemoryBarrier":
		return vdmir.IntrinsicWorkgroupMemoryBarrier
	default:
		return vdmir.IntrinsicWorkgroupMemoryBarrierWithSync
	}
}
