package validate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
)

func Module(module ast.Module) error {
	v := validator{
		types:          map[string]typeInfo{},
		funcs:          map[string]functionInfo{},
		configs:        map[string]configInfo{},
		shaderDecls:    map[string]ast.ShaderDecl{},
		compileAliases: map[string]struct{}{},
	}
	v.seedBuiltins()
	v.collect(module)
	if len(v.errors) == 0 {
		v.validateDecls(module.Decls)
	}
	if len(v.errors) > 0 {
		return errors.New(strings.Join(v.errors, "\n"))
	}
	return nil
}

type fieldInfo struct {
	access string
	typ    ast.TypeRef
}

type typeInfo struct {
	name   string
	kind   string
	fields map[string]fieldInfo
	target ast.TypeRef
}

type functionInfo struct {
	returnType ast.TypeRef
	params     []ast.Parameter
}

type configValue struct {
	typ     ast.TypeRef
	int32   int64
	boolVal bool
}

type configInfo struct {
	conceptName string
	fields      map[string]configValue
}

type varOrigin string

const (
	varLocal     varOrigin = "local"
	varParam     varOrigin = "param"
	varResource  varOrigin = "resource"
	varWorkgroup varOrigin = "workgroup"
	varBuiltin   varOrigin = "builtin"
)

type varInfo struct {
	typ    ast.TypeRef
	origin varOrigin
}

type validator struct {
	errors         []string
	types          map[string]typeInfo
	funcs          map[string]functionInfo
	configs        map[string]configInfo
	shaderDecls    map[string]ast.ShaderDecl
	compileAliases map[string]struct{}
	resources      map[string]ast.ResourceDecl
}

func (v *validator) seedBuiltins() {
	for _, name := range []string{"void", "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4"} {
		v.types[name] = typeInfo{name: name, kind: "builtin"}
	}
	v.types["uint2"] = builtinUintVectorType(2)
	v.types["uint3"] = builtinUintVectorType(3)
	v.types["uint4"] = builtinUintVectorType(4)
}

func (v *validator) collect(module ast.Module) {
	for _, decl := range module.Decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			v.addType(d.Name, typeInfo{name: d.Name, kind: "alias", target: d.Type})
		case ast.RecordDecl:
			v.addFieldType(d.Name, "record", d.Fields, "record")
		case ast.StreamDecl:
			v.addFieldType(d.Name, "stream", d.Fields, "stream")
		case ast.ConceptDecl:
			v.addFieldType(d.Name, "concept", d.Fields, "concept")
		case ast.ConfigDecl:
			if _, exists := v.configs[d.Name]; exists {
				v.errorf("duplicate top-level name %s", d.Name)
				continue
			}
			v.configs[d.Name] = configInfo{conceptName: d.ConceptName, fields: map[string]configValue{}}
		case ast.EnumDecl:
			v.addType(d.Name, typeInfo{name: d.Name, kind: "enum"})
		case ast.FunctionDecl:
			v.addFunc(d.Name, d)
		case ast.ShaderDecl:
			v.addType(d.Name, typeInfo{name: d.Name, kind: "shader"})
			v.shaderDecls[d.Name] = d
			seenMethods := map[string]struct{}{}
			for _, method := range d.Methods {
				if _, exists := seenMethods[method.Name]; exists {
					v.errorf("duplicate shader method %s.%s", d.Name, method.Name)
				}
				seenMethods[method.Name] = struct{}{}
				v.addFunc(d.Name+"_"+method.Name, method)
			}
		case ast.UnsupportedDecl:
			v.errorf("%s is not implemented in GoOct SDSL-V M0", d.Kind)
		}
	}
}

func (v *validator) addFieldType(name, kind string, fields []ast.Field, label string) {
	collected := map[string]fieldInfo{}
	for _, field := range fields {
		if _, exists := collected[field.Name]; exists {
			v.errorf("duplicate %s field %s.%s", label, name, field.Name)
		}
		collected[field.Name] = fieldInfo{access: field.Access, typ: field.Type}
	}
	v.addType(name, typeInfo{name: name, kind: kind, fields: collected})
}

func (v *validator) addType(name string, info typeInfo) {
	if _, exists := v.types[name]; exists {
		v.errorf("duplicate top-level name %s", name)
		return
	}
	v.types[name] = info
}

func (v *validator) addFunc(name string, fn ast.FunctionDecl) {
	if _, exists := v.funcs[name]; exists {
		v.errorf("duplicate function %s", name)
		return
	}
	v.funcs[name] = functionInfo{returnType: fn.ReturnType, params: fn.Parameters}
}

func (v *validator) validateDecls(decls []ast.Decl) {
	for _, decl := range decls {
		switch d := decl.(type) {
		case ast.TypeAliasDecl:
			v.validateType(d.Type)
		case ast.RecordDecl:
			v.validateFields(d.Name, "record", d.Fields, false)
		case ast.StreamDecl:
			v.validateFields(d.Name, "stream", d.Fields, true)
			v.validateComputeThreadStream(d)
		case ast.ConceptDecl:
			v.validateConcept(d)
		case ast.ConfigDecl:
			v.validateConfig(d)
		case ast.ShaderDecl:
			v.validateShader(d)
		case ast.CompileDecl:
			v.validateCompileDecl(d)
		case ast.FunctionDecl:
			v.validateFunction(d, "", "", nil, nil, nil)
		}
	}
}

func (v *validator) validateFields(owner, kind string, fields []ast.Field, allowAccess bool) {
	for _, field := range fields {
		if field.Access != "" && !allowAccess {
			v.errorf("%s field %s.%s must not declare resource access", kind, owner, field.Name)
		}
		v.validateType(field.Type)
	}
}

func (v *validator) validateConcept(concept ast.ConceptDecl) {
	for _, field := range concept.Fields {
		if field.Access != "" {
			v.errorf("concept field %s.%s must not declare resource access", concept.Name, field.Name)
		}
		resolved := v.resolveAlias(field.Type)
		switch resolved.Name {
		case "u32", "i32", "bool", "f32", "float":
		default:
			v.errorf("concept field %s.%s must use a compile-time scalar type in SDSL-V M5", concept.Name, field.Name)
		}
	}
}

func (v *validator) validateConfig(config ast.ConfigDecl) {
	info, ok := v.types[config.ConceptName]
	if !ok || info.kind != "concept" {
		v.errorf("unknown concept %s for config %s", config.ConceptName, config.Name)
		return
	}
	seen := map[string]struct{}{}
	values := map[string]configValue{}
	for _, field := range config.Fields {
		if _, exists := seen[field.Name]; exists {
			v.errorf("duplicate config field %s.%s", config.Name, field.Name)
			continue
		}
		seen[field.Name] = struct{}{}
		conceptField, ok := info.fields[field.Name]
		if !ok {
			v.errorf("unknown config field %s.%s", config.Name, field.Name)
			continue
		}
		value, err := v.evalConstExpr(field.Value, nil)
		if err != nil {
			v.errorf("config %s field %s: %v", config.Name, field.Name, err)
			continue
		}
		if !v.compatible(conceptField.typ, value.typ) {
			v.errorf("config %s field %s expects %s, got %s", config.Name, field.Name, typeName(conceptField.typ), typeName(value.typ))
			continue
		}
		values[field.Name] = value
	}
	for name := range info.fields {
		if _, exists := seen[name]; !exists {
			v.errorf("config field %s missing", name)
		}
	}
	cfg := v.configs[config.Name]
	cfg.fields = values
	v.configs[config.Name] = cfg
}

func (v *validator) validateCompileDecl(decl ast.CompileDecl) {
	if _, exists := v.compileAliases[decl.AliasName]; exists {
		v.errorf("duplicate compile alias %s", decl.AliasName)
		return
	}
	if _, exists := v.types[decl.AliasName]; exists {
		v.errorf("compile alias %s collides with top-level declaration", decl.AliasName)
		return
	}
	shader, ok := v.shaderDecls[decl.ShaderName]
	if !ok {
		v.errorf("unknown shader %s in compile declaration", decl.ShaderName)
		return
	}
	if shader.Template == nil {
		v.errorf("compile target %s must be a template shader", decl.ShaderName)
		return
	}
	config, ok := v.configs[decl.ConfigName]
	if !ok {
		v.errorf("unknown config %s in compile declaration", decl.ConfigName)
		return
	}
	if config.conceptName != shader.Template.ConceptName {
		v.errorf("compile config %s does not satisfy concept %s", decl.ConfigName, shader.Template.ConceptName)
		return
	}
	v.compileAliases[decl.AliasName] = struct{}{}
}

func (v *validator) validateShader(shader ast.ShaderDecl) {
	if shader.ResourceBundleName != "" && len(shader.Resources) > 0 {
		v.errorf("shader %s cannot use both resources %s; and resources { ... }", shader.Name, shader.ResourceBundleName)
	}
	seenShaderNames := map[string]string{}
	resources := v.resolveShaderResources(shader)
	v.resources = map[string]ast.ResourceDecl{}
	for _, resource := range resources {
		if _, exists := v.resources[resource.Name]; exists {
			v.errorf("duplicate shader resource %s.%s", shader.Name, resource.Name)
		}
		if prior, exists := seenShaderNames[resource.Name]; exists {
			v.errorf("shader %s name %s collides with %s", shader.Name, resource.Name, prior)
		}
		seenShaderNames[resource.Name] = "resource"
		if resource.Access != "readonly" && resource.Access != "readwrite" {
			v.errorf("resource %s.%s must be readonly or readwrite", shader.Name, resource.Name)
		}
		if resource.Type.Name != "array" || len(resource.Type.Args) != 1 {
			v.errorf("resource %s.%s must use array<T> in GoOct SDSL-V M3", shader.Name, resource.Name)
		}
		v.validateType(resource.Type)
		v.resources[resource.Name] = resource
	}
	workgroups := map[string]ast.WorkgroupDecl{}
	for _, workgroup := range shader.Workgroups {
		if _, exists := workgroups[workgroup.Name]; exists {
			v.errorf("duplicate shader workgroup %s.%s", shader.Name, workgroup.Name)
			continue
		}
		if prior, exists := seenShaderNames[workgroup.Name]; exists {
			v.errorf("shader %s name %s collides with %s", shader.Name, workgroup.Name, prior)
		}
		seenShaderNames[workgroup.Name] = "workgroup"
		v.validateWorkgroup(shader.Name, workgroup, shader.Template)
		workgroups[workgroup.Name] = workgroup
	}
	for _, method := range shader.Methods {
		if prior, exists := seenShaderNames[method.Name]; exists {
			v.errorf("shader %s name %s collides with %s", shader.Name, method.Name, prior)
		}
		seenShaderNames[method.Name] = "method"
		if method.Stage != "" && method.Stage != "compute" {
			v.errorf("stage %s is not implemented in GoOct SDSL-V M0; only compute is supported", method.Stage)
		}
		if method.Stage == "compute" {
			if method.NumThreads == nil {
				v.errorf("compute method %s.%s requires [numthreads(x, y, z)]", shader.Name, method.Name)
			} else {
				v.validateNumThreads(shader.Name, method, shader.Template)
			}
		}
		v.validateFunction(method, shader.Name, method.Stage, resources, shader.Workgroups, shader.Template)
	}
	v.resources = nil
}

func (v *validator) resolveShaderResources(shader ast.ShaderDecl) []ast.ResourceDecl {
	if shader.ResourceBundleName == "" {
		return append([]ast.ResourceDecl(nil), shader.Resources...)
	}
	info, ok := v.types[shader.ResourceBundleName]
	if !ok {
		v.errorf("unknown resource bundle %s on shader %s", shader.ResourceBundleName, shader.Name)
		return nil
	}
	if info.kind != "stream" {
		v.errorf("resource bundle %s on shader %s must be a stream", shader.ResourceBundleName, shader.Name)
		return nil
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
			v.errorf("resource bundle %s field %s must declare readonly or readwrite access", shader.ResourceBundleName, name)
			continue
		}
		if field.typ.Name != "array" || len(field.typ.Args) != 1 {
			v.errorf("resource bundle %s field %s must use %s array<T>", shader.ResourceBundleName, name, field.access)
		}
		resources = append(resources, ast.ResourceDecl{Name: name, Access: field.access, Type: field.typ})
	}
	return resources
}

func (v *validator) validateFunction(fn ast.FunctionDecl, shaderName string, stage string, resources []ast.ResourceDecl, workgroups []ast.WorkgroupDecl, templateParam *ast.TemplateParam) {
	v.validateType(fn.ReturnType)
	scope := map[string]varInfo{
		"DispatchThreadID": {typ: ast.TypeRef{Name: "uint3"}, origin: varBuiltin},
		"GroupThreadID":    {typ: ast.TypeRef{Name: "uint3"}, origin: varBuiltin},
		"GroupID":          {typ: ast.TypeRef{Name: "uint3"}, origin: varBuiltin},
		"GroupIndex":       {typ: ast.TypeRef{Name: "u32"}, origin: varBuiltin},
	}
	for _, resource := range resources {
		scope[resource.Name] = varInfo{typ: resource.Type, origin: varResource}
	}
	for _, workgroup := range workgroups {
		scope[workgroup.Name] = varInfo{typ: workgroup.Type, origin: varWorkgroup}
	}
	for _, param := range fn.Parameters {
		if _, exists := scope[param.Name]; exists {
			v.errorf("duplicate parameter or builtin name %s in %s", param.Name, fn.Name)
		}
		v.validateType(param.Type)
		scope[param.Name] = varInfo{typ: param.Type, origin: varParam}
	}
	for _, stmt := range fn.Body.Statements {
		v.validateStmt(stmt, fn.ReturnType, scope, shaderName, stage, templateParam)
	}
}

func (v *validator) validateStmt(stmt ast.Stmt, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam) {
	switch s := stmt.(type) {
	case ast.LetStmt:
		v.validateType(s.Type)
		if s.Value != nil {
			v.validateWithPlacement(s.Value, true)
			v.validateBarrierUsage(s.Value, false, shaderName, stage)
			valueType := v.exprType(s.Value, scope, shaderName, templateParam)
			if !v.compatible(s.Type, valueType) {
				v.errorf("cannot assign %s to local %s of type %s", typeName(valueType), s.Name, typeName(s.Type))
			}
		}
		if _, exists := scope[s.Name]; exists {
			v.errorf("duplicate local name %s", s.Name)
		}
		scope[s.Name] = varInfo{typ: s.Type, origin: varLocal}
	case ast.AssignStmt:
		v.validateWithPlacement(s.Target, false)
		v.validateWithPlacement(s.Value, true)
		v.validateBarrierUsage(s.Target, false, shaderName, stage)
		v.validateBarrierUsage(s.Value, false, shaderName, stage)
		targetType := v.exprType(s.Target, scope, shaderName, templateParam)
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		if !isAssignableTarget(s.Target) {
			v.errorf("assignment target is not assignable")
		}
		v.validateImmutableAssignmentTarget(s.Target, scope)
		if !v.compatible(targetType, valueType) {
			v.errorf("assignment type mismatch: %s = %s", typeName(targetType), typeName(valueType))
		}
	case ast.ReturnStmt:
		if s.Value == nil {
			if returnType.Name != "void" {
				v.errorf("return without value in function returning %s", typeName(returnType))
			}
			return
		}
		v.validateWithPlacement(s.Value, true)
		v.validateBarrierUsage(s.Value, false, shaderName, stage)
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		if !v.compatible(returnType, valueType) {
			v.errorf("return type mismatch: expected %s, got %s", typeName(returnType), typeName(valueType))
		}
	case ast.ExprStmt:
		v.validateWithPlacement(s.Value, false)
		v.validateBarrierUsage(s.Value, true, shaderName, stage)
		v.exprType(s.Value, scope, shaderName, templateParam)
	case ast.IfStmt:
		v.validateWithPlacement(s.Condition, false)
		v.validateBarrierUsage(s.Condition, false, shaderName, stage)
		cond := v.exprType(s.Condition, scope, shaderName, templateParam)
		if cond.Name != "bool" {
			v.errorf("if condition must be bool, got %s", typeName(cond))
		}
		v.validateBlock(s.ThenBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.ForStmt:
		v.validateWithPlacement(s.Start, false)
		v.validateWithPlacement(s.End, false)
		v.validateWithPlacement(s.Step, false)
		v.validateBarrierUsage(s.Start, false, shaderName, stage)
		v.validateBarrierUsage(s.End, false, shaderName, stage)
		v.validateBarrierUsage(s.Step, false, shaderName, stage)
		startType := v.exprType(s.Start, scope, shaderName, templateParam)
		endType := v.exprType(s.End, scope, shaderName, templateParam)
		if !isInteger(startType) || !isInteger(endType) {
			v.errorf("for bounds must be integer")
		}
		if !positiveIntegerLiteral(s.Step) {
			v.errorf("for step must be a positive integer literal")
		}
		loopScope := cloneScope(scope)
		loopScope[s.Name] = varInfo{typ: startType, origin: varLocal}
		v.validateBlock(s.Body, returnType, loopScope, shaderName, stage, templateParam)
	}
}

func (v *validator) validateBlock(block ast.Block, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam) {
	for _, stmt := range block.Statements {
		v.validateStmt(stmt, returnType, scope, shaderName, stage, templateParam)
	}
}

func (v *validator) validateType(ref ast.TypeRef) {
	if ref.Name == "array" {
		if len(ref.Args) != 1 {
			v.errorf("array type requires one element type")
			return
		}
		v.validateType(ref.Args[0])
		if ref.HasArraySize && ref.ArraySize != nil {
			value, err := v.evalConstExpr(ref.ArraySize, nil)
			if err == nil && !isInteger(value.typ) {
				v.errorf("array size must be an integer constant expression")
			}
		}
		return
	}
	if _, ok := v.types[ref.Name]; !ok {
		v.errorf("unknown type %s", ref.Name)
	}
}

func (v *validator) exprType(expr ast.Expr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			return ast.TypeRef{Name: "u32"}
		}
		return ast.TypeRef{Name: "i32"}
	case ast.FloatLiteral:
		return ast.TypeRef{Name: "f32"}
	case ast.BoolLiteral:
		return ast.TypeRef{Name: "bool"}
	case ast.StringLiteral:
		return ast.TypeRef{Name: "string"}
	case ast.IdentifierExpr:
		if t, ok := scope[e.Name]; ok {
			return v.resolveAlias(t.typ)
		}
		v.errorf("unknown identifier %s", e.Name)
		return ast.TypeRef{Name: "<error>"}
	case ast.FieldAccessExpr:
		if id, ok := e.Target.(ast.IdentifierExpr); ok && templateParam != nil && id.Name == templateParam.Name {
			concept, ok := v.types[templateParam.ConceptName]
			if !ok || concept.kind != "concept" {
				v.errorf("unknown concept %s on template shader %s", templateParam.ConceptName, shaderName)
				return ast.TypeRef{Name: "<error>"}
			}
			if fieldType, ok := concept.fields[e.Field]; ok {
				return v.resolveAlias(fieldType.typ)
			}
			v.errorf("unknown template field %s on concept %s", e.Field, templateParam.ConceptName)
			return ast.TypeRef{Name: "<error>"}
		}
		if id, ok := e.Target.(ast.IdentifierExpr); ok && templateParam == nil {
			if _, inScope := scope[id.Name]; !inScope {
				v.errorf("%s.%s is only valid inside template shader %s", id.Name, e.Field, shaderName)
				return ast.TypeRef{Name: "<error>"}
			}
		}
		target := v.exprType(e.Target, scope, shaderName, templateParam)
		if info, ok := v.types[target.Name]; ok && info.fields != nil {
			if fieldType, ok := info.fields[e.Field]; ok {
				return v.resolveAlias(fieldType.typ)
			}
		}
		v.errorf("unknown field %s on %s", e.Field, typeName(target))
		return ast.TypeRef{Name: "<error>"}
	case ast.IndexExpr:
		target := v.exprType(e.Target, scope, shaderName, templateParam)
		index := v.exprType(e.Index, scope, shaderName, templateParam)
		if !isInteger(index) {
			v.errorf("array index must be integer")
		}
		if target.Name == "array" && len(target.Args) == 1 {
			return v.resolveAlias(target.Args[0])
		}
		v.errorf("cannot index non-array type %s", typeName(target))
		return ast.TypeRef{Name: "<error>"}
	case ast.CallExpr:
		return v.callType(e, scope, shaderName, templateParam)
	case ast.BinaryExpr:
		left := v.exprType(e.Left, scope, shaderName, templateParam)
		right := v.exprType(e.Right, scope, shaderName, templateParam)
		switch e.Operator {
		case "&&", "||":
			if left.Name != "bool" || right.Name != "bool" {
				v.errorf("logical operands must be bool")
			}
			return ast.TypeRef{Name: "bool"}
		case "==", "!=", "<", "<=", ">", ">=":
			if !v.compatible(left, right) && !(isNumeric(left) && isNumeric(right)) {
				v.errorf("comparison type mismatch: %s %s %s", typeName(left), e.Operator, typeName(right))
			}
			return ast.TypeRef{Name: "bool"}
		default:
			if !isNumeric(left) || !isNumeric(right) {
				v.errorf("arithmetic operands must be numeric")
				return ast.TypeRef{Name: "<error>"}
			}
			if isFloat(left) || isFloat(right) {
				return ast.TypeRef{Name: "f32"}
			}
			if left.Name == "u32" || right.Name == "u32" {
				return ast.TypeRef{Name: "u32"}
			}
			return ast.TypeRef{Name: "i32"}
		}
	case ast.UnaryExpr:
		operand := v.exprType(e.Operand, scope, shaderName, templateParam)
		if !isNumeric(operand) {
			v.errorf("unary %s requires numeric operand", e.Operator)
		}
		return operand
	case ast.ParenExpr:
		return v.exprType(e.Inner, scope, shaderName, templateParam)
	case ast.WhenUtilityExpr:
		var result ast.TypeRef
		for i, c := range e.Cases {
			value := v.exprType(c.Value, scope, shaderName, templateParam)
			cond := v.exprType(c.Condition, scope, shaderName, templateParam)
			score := v.exprType(c.Score, scope, shaderName, templateParam)
			if cond.Name != "bool" {
				v.errorf("when utility guard must be bool")
			}
			if !isNumeric(score) {
				v.errorf("when utility score must be numeric")
			}
			if i == 0 {
				result = value
			} else if !v.compatible(result, value) {
				v.errorf("when utility case values must share a type")
			}
		}
		elseType := v.exprType(e.Else, scope, shaderName, templateParam)
		if result.Name == "" {
			result = elseType
		}
		if !v.compatible(result, elseType) {
			v.errorf("when utility else value must match case value type")
		}
		return result
	case ast.WithExpr:
		baseType := v.exprType(e.Base, scope, shaderName, templateParam)
		kind := v.typeKind(baseType)
		if kind != "record" && kind != "stream" {
			v.errorf("with base must be a record or stream, got %s", typeName(baseType))
			return ast.TypeRef{Name: "<error>"}
		}
		info := v.types[baseType.Name]
		seen := map[string]struct{}{}
		for _, update := range e.Updates {
			if _, exists := seen[update.Name]; exists {
				v.errorf("duplicate with field %s", update.Name)
				continue
			}
			seen[update.Name] = struct{}{}
			field, ok := info.fields[update.Name]
			if !ok {
				v.errorf("unknown with field %s on %s", update.Name, typeName(baseType))
				continue
			}
			valueType := v.exprType(update.Value, scope, shaderName, templateParam)
			if !v.compatible(field.typ, valueType) {
				v.errorf("with field %s expects %s, got %s", update.Name, typeName(field.typ), typeName(valueType))
			}
		}
		return baseType
	default:
		v.errorf("unsupported expression in GoOct SDSL-V M3")
		return ast.TypeRef{Name: "<error>"}
	}
}

func (v *validator) callType(call ast.CallExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	if id, ok := call.Callee.(ast.IdentifierExpr); ok {
		if isVectorConstructor(id.Name) {
			return ast.TypeRef{Name: id.Name}
		}
		if isBarrierBuiltin(id.Name) {
			return ast.TypeRef{Name: "void"}
		}
		names := []string{id.Name}
		if shaderName != "" {
			names = append([]string{shaderName + "_" + id.Name}, names...)
		}
		for _, name := range names {
			if info, ok := v.funcs[name]; ok {
				if len(info.params) != len(call.Arguments) {
					v.errorf("function %s expects %d arguments, got %d", id.Name, len(info.params), len(call.Arguments))
					return info.returnType
				}
				for i, arg := range call.Arguments {
					argType := v.exprType(arg, scope, shaderName, templateParam)
					if !v.compatible(info.params[i].Type, argType) {
						v.errorf("function %s argument %d expects %s, got %s", id.Name, i+1, typeName(info.params[i].Type), typeName(argType))
					}
				}
				return info.returnType
			}
		}
	}
	v.errorf("unsupported or unknown function call")
	return ast.TypeRef{Name: "<error>"}
}

func (v *validator) compatible(left, right ast.TypeRef) bool {
	left = v.resolveAlias(left)
	right = v.resolveAlias(right)
	if left.Name == right.Name {
		if left.Name != "array" {
			return true
		}
		return len(left.Args) == len(right.Args) && (len(left.Args) == 0 || v.compatible(left.Args[0], right.Args[0]))
	}
	return isFloat(left) && isFloat(right)
}

func (v *validator) resolveAlias(ref ast.TypeRef) ast.TypeRef {
	info, ok := v.types[ref.Name]
	if !ok || info.kind != "alias" {
		return ref
	}
	return v.resolveAlias(info.target)
}

func (v *validator) typeKind(ref ast.TypeRef) string {
	resolved := v.resolveAlias(ref)
	info, ok := v.types[resolved.Name]
	if !ok {
		if resolved.Name == "array" {
			return "array"
		}
		return ""
	}
	return info.kind
}

func (v *validator) validateImmutableAssignmentTarget(expr ast.Expr, scope map[string]varInfo) {
	root, ok := rootIdentifier(expr)
	if !ok {
		return
	}
	info, ok := scope[root]
	if !ok {
		return
	}
	switch info.origin {
	case varBuiltin:
		v.errorf("cannot assign to compute builtin %s", root)
	case varParam:
		kind := v.typeKind(info.typ)
		switch {
		case kind == "record" || kind == "stream":
			if !isDirectIdentifier(expr) {
				v.errorf("cannot assign through immutable %s parameter %s; use with instead", kind, root)
			}
		case kind == "array":
			if !isDirectIdentifier(expr) {
				v.errorf("cannot assign through immutable array parameter %s", root)
			}
		}
	}
}

func (v *validator) validateWithPlacement(expr ast.Expr, topLevelAllowed bool) {
	switch e := expr.(type) {
	case ast.WithExpr:
		if !topLevelAllowed {
			v.errorf("with expression is only supported as a direct let initializer, assignment RHS, or return value in GoOct SDSL-V M3")
			return
		}
		v.validateWithPlacement(e.Base, false)
		for _, update := range e.Updates {
			v.validateWithPlacement(update.Value, false)
		}
	case ast.FieldAccessExpr:
		v.validateWithPlacement(e.Target, false)
	case ast.IndexExpr:
		v.validateWithPlacement(e.Target, false)
		v.validateWithPlacement(e.Index, false)
	case ast.CallExpr:
		v.validateWithPlacement(e.Callee, false)
		for _, arg := range e.Arguments {
			v.validateWithPlacement(arg, false)
		}
	case ast.BinaryExpr:
		v.validateWithPlacement(e.Left, false)
		v.validateWithPlacement(e.Right, false)
	case ast.UnaryExpr:
		v.validateWithPlacement(e.Operand, false)
	case ast.ParenExpr:
		v.validateWithPlacement(e.Inner, false)
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			v.validateWithPlacement(c.Value, false)
			v.validateWithPlacement(c.Condition, false)
			v.validateWithPlacement(c.Score, false)
		}
		if e.Else != nil {
			v.validateWithPlacement(e.Else, false)
		}
	}
}

func (v *validator) validateComputeThreadStream(stream ast.StreamDecl) {
	if stream.Name != "ComputeThread" {
		return
	}
	expected := map[string]string{
		"DispatchId":    "uint3",
		"GroupId":       "uint3",
		"GroupThreadId": "uint3",
		"GroupIndex":    "u32",
	}
	for _, field := range stream.Fields {
		want, ok := expected[field.Name]
		if !ok {
			continue
		}
		got := v.resolveAlias(field.Type)
		if got.Name != want {
			v.errorf("ComputeThread.%s must be %s, got %s", field.Name, want, typeName(got))
		}
	}
}

func (v *validator) validateNumThreads(shaderName string, method ast.FunctionDecl, templateParam *ast.TemplateParam) {
	exprs := []ast.Expr{method.NumThreads.X, method.NumThreads.Y, method.NumThreads.Z}
	for _, expr := range exprs {
		if expr == nil {
			v.errorf("compute method %s.%s requires [numthreads(x, y, z)]", shaderName, method.Name)
			return
		}
		typ := v.exprType(expr, map[string]varInfo{}, shaderName, templateParam)
		if !isInteger(typ) {
			v.errorf("compute method %s.%s numthreads values must be positive integer literals or template constants", shaderName, method.Name)
			return
		}
		if templateParam == nil {
			value, err := v.evalConstExpr(expr, nil)
			if err != nil || !isInteger(value.typ) || value.int32 <= 0 {
				v.errorf("compute method %s.%s numthreads values must be positive integer literals", shaderName, method.Name)
				return
			}
		}
	}
}

func (v *validator) validateWorkgroup(shaderName string, decl ast.WorkgroupDecl, templateParam *ast.TemplateParam) {
	if decl.Type.Name != "array" {
		v.errorf("workgroup %s.%s must use array<T, N>", shaderName, decl.Name)
		return
	}
	if len(decl.Type.Args) != 1 || !decl.Type.HasArraySize {
		v.errorf("workgroup %s.%s must use fixed-size array<T, N>", shaderName, decl.Name)
		return
	}
	if decl.Type.ArraySize == nil {
		v.errorf("workgroup %s.%s must use fixed-size array<T, N>", shaderName, decl.Name)
	} else {
		typ := v.exprType(decl.Type.ArraySize, map[string]varInfo{}, shaderName, templateParam)
		if !isInteger(typ) {
			v.errorf("workgroup %s.%s size must be an integer constant expression", shaderName, decl.Name)
		} else if templateParam == nil {
			value, err := v.evalConstExpr(decl.Type.ArraySize, nil)
			if err != nil || value.int32 <= 0 {
				v.errorf("workgroup %s.%s size must be positive", shaderName, decl.Name)
			}
		}
	}
	v.validateType(decl.Type)
	if !isWorkgroupElementType(v.resolveAlias(decl.Type.Args[0])) {
		v.errorf("workgroup %s.%s element type %s is not supported in GoOct SDSL-V M4", shaderName, decl.Name, typeName(v.resolveAlias(decl.Type.Args[0])))
	}
}

func (v *validator) validateBarrierUsage(expr ast.Expr, topLevelExprStmt bool, shaderName string, stage string) {
	switch e := expr.(type) {
	case ast.CallExpr:
		if id, ok := e.Callee.(ast.IdentifierExpr); ok && isBarrierBuiltin(id.Name) {
			if !topLevelExprStmt {
				v.errorf("barrier builtin %s may only be used as an expression statement", id.Name)
			}
			if len(e.Arguments) != 0 {
				v.errorf("barrier builtin %s expects 0 arguments, got %d", id.Name, len(e.Arguments))
			}
			if stage != "compute" && shaderName == "" {
				v.errorf("barrier builtin %s is only valid in compute functions", id.Name)
			}
		}
		v.validateBarrierUsage(e.Callee, false, shaderName, stage)
		for _, arg := range e.Arguments {
			v.validateBarrierUsage(arg, false, shaderName, stage)
		}
	case ast.FieldAccessExpr:
		v.validateBarrierUsage(e.Target, false, shaderName, stage)
	case ast.IndexExpr:
		v.validateBarrierUsage(e.Target, false, shaderName, stage)
		v.validateBarrierUsage(e.Index, false, shaderName, stage)
	case ast.BinaryExpr:
		v.validateBarrierUsage(e.Left, false, shaderName, stage)
		v.validateBarrierUsage(e.Right, false, shaderName, stage)
	case ast.UnaryExpr:
		v.validateBarrierUsage(e.Operand, false, shaderName, stage)
	case ast.ParenExpr:
		v.validateBarrierUsage(e.Inner, false, shaderName, stage)
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			v.validateBarrierUsage(c.Value, false, shaderName, stage)
			v.validateBarrierUsage(c.Condition, false, shaderName, stage)
			v.validateBarrierUsage(c.Score, false, shaderName, stage)
		}
		if e.Else != nil {
			v.validateBarrierUsage(e.Else, false, shaderName, stage)
		}
	case ast.WithExpr:
		v.validateBarrierUsage(e.Base, false, shaderName, stage)
		for _, update := range e.Updates {
			v.validateBarrierUsage(update.Value, false, shaderName, stage)
		}
	}
}

func (v *validator) errorf(format string, args ...any) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

func typeName(ref ast.TypeRef) string {
	if ref.Name != "array" || len(ref.Args) == 0 {
		return ref.Name
	}
	if ref.HasArraySize {
		return fmt.Sprintf("array<%s,N>", typeName(ref.Args[0]))
	}
	return fmt.Sprintf("array<%s>", typeName(ref.Args[0]))
}

func (v *validator) evalConstExpr(expr ast.Expr, env map[string]configValue) (configValue, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		value, err := strconv.ParseInt(strings.TrimRight(e.Value, "uU"), 10, 32)
		if err != nil {
			return configValue{}, fmt.Errorf("invalid integer literal %s", e.Value)
		}
		typ := ast.TypeRef{Name: "i32"}
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			typ = ast.TypeRef{Name: "u32"}
		}
		return configValue{typ: typ, int32: value}, nil
	case ast.BoolLiteral:
		return configValue{typ: ast.TypeRef{Name: "bool"}, boolVal: e.Value}, nil
	case ast.FloatLiteral:
		return configValue{}, fmt.Errorf("float constant expressions are not implemented in SDSL-V M5")
	case ast.ParenExpr:
		return v.evalConstExpr(e.Inner, env)
	case ast.UnaryExpr:
		value, err := v.evalConstExpr(e.Operand, env)
		if err != nil {
			return configValue{}, err
		}
		if e.Operator != "-" || !isInteger(value.typ) {
			return configValue{}, fmt.Errorf("unsupported unary constant expression")
		}
		return configValue{typ: value.typ, int32: -value.int32}, nil
	case ast.FieldAccessExpr:
		id, ok := e.Target.(ast.IdentifierExpr)
		if !ok || env == nil {
			return configValue{}, fmt.Errorf("template config field references are not available here")
		}
		value, ok := env[id.Name+"."+e.Field]
		if !ok {
			return configValue{}, fmt.Errorf("unknown template field %s.%s", id.Name, e.Field)
		}
		return value, nil
	case ast.BinaryExpr:
		left, err := v.evalConstExpr(e.Left, env)
		if err != nil {
			return configValue{}, err
		}
		right, err := v.evalConstExpr(e.Right, env)
		if err != nil {
			return configValue{}, err
		}
		switch e.Operator {
		case "+", "-", "*", "/", "%":
			if !isInteger(left.typ) || !isInteger(right.typ) {
				return configValue{}, fmt.Errorf("arithmetic constant expressions require integer operands")
			}
			if (e.Operator == "/" || e.Operator == "%") && right.int32 == 0 {
				return configValue{}, fmt.Errorf("division by zero in constant expression")
			}
			out := configValue{typ: left.typ, int32: left.int32}
			if left.typ.Name == "u32" || right.typ.Name == "u32" {
				out.typ = ast.TypeRef{Name: "u32"}
			}
			switch e.Operator {
			case "+":
				out.int32 = left.int32 + right.int32
			case "-":
				out.int32 = left.int32 - right.int32
			case "*":
				out.int32 = left.int32 * right.int32
			case "/":
				out.int32 = left.int32 / right.int32
			case "%":
				out.int32 = left.int32 % right.int32
			}
			return out, nil
		case "==", "!=", "<", "<=", ">", ">=":
			if !isInteger(left.typ) || !isInteger(right.typ) {
				return configValue{}, fmt.Errorf("comparison constant expressions require integer operands")
			}
			result := false
			switch e.Operator {
			case "==":
				result = left.int32 == right.int32
			case "!=":
				result = left.int32 != right.int32
			case "<":
				result = left.int32 < right.int32
			case "<=":
				result = left.int32 <= right.int32
			case ">":
				result = left.int32 > right.int32
			case ">=":
				result = left.int32 >= right.int32
			}
			return configValue{typ: ast.TypeRef{Name: "bool"}, boolVal: result}, nil
		case "&&", "||":
			if left.typ.Name != "bool" || right.typ.Name != "bool" {
				return configValue{}, fmt.Errorf("logical constant expressions require bool operands")
			}
			result := left.boolVal && right.boolVal
			if e.Operator == "||" {
				result = left.boolVal || right.boolVal
			}
			return configValue{typ: ast.TypeRef{Name: "bool"}, boolVal: result}, nil
		default:
			return configValue{}, fmt.Errorf("unsupported constant operator %s", e.Operator)
		}
	default:
		return configValue{}, fmt.Errorf("expression is not a valid SDSL-V M5 constant expression")
	}
}

func cloneScope(scope map[string]varInfo) map[string]varInfo {
	next := make(map[string]varInfo, len(scope))
	for k, v := range scope {
		next[k] = v
	}
	return next
}

func isAssignableTarget(expr ast.Expr) bool {
	switch expr.(type) {
	case ast.IdentifierExpr, ast.FieldAccessExpr, ast.IndexExpr:
		return true
	default:
		return false
	}
}

func rootIdentifier(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name, true
	case ast.FieldAccessExpr:
		return rootIdentifier(e.Target)
	case ast.IndexExpr:
		return rootIdentifier(e.Target)
	default:
		return "", false
	}
}

func isDirectIdentifier(expr ast.Expr) bool {
	_, ok := expr.(ast.IdentifierExpr)
	return ok
}

func isFloat(ref ast.TypeRef) bool { return ref.Name == "f32" || ref.Name == "float" }

func isInteger(ref ast.TypeRef) bool {
	return ref.Name == "i32" || ref.Name == "u32"
}

func isNumeric(ref ast.TypeRef) bool { return isInteger(ref) || isFloat(ref) }

func isVectorConstructor(name string) bool {
	return name == "float2" || name == "float3" || name == "float4" || name == "uint2" || name == "uint3" || name == "uint4"
}

func positiveIntegerLiteral(expr ast.Expr) bool {
	lit, ok := expr.(ast.IntegerLiteral)
	if !ok {
		return false
	}
	value, err := strconv.Atoi(strings.TrimRight(lit.Value, "uU"))
	return err == nil && value > 0
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
	return typeInfo{name: fmt.Sprintf("uint%d", dim), kind: "builtin", fields: trimmed}
}

func isWorkgroupElementType(ref ast.TypeRef) bool {
	switch ref.Name {
	case "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4", "uint2", "uint3", "uint4":
		return true
	default:
		return false
	}
}

func isBarrierBuiltin(name string) bool {
	return name == "WorkgroupBarrier" || name == "WorkgroupMemoryBarrier" || name == "WorkgroupMemoryBarrierWithSync"
}
