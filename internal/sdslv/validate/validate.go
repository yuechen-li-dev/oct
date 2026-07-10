package validate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/consteval"
)

func Module(module ast.Module) error {
	v := validator{
		types:          map[string]typeInfo{},
		funcs:          map[string]functionInfo{},
		configs:        map[string]configInfo{},
		concepts:       map[string]ast.ConceptDecl{},
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
	access     string
	typ        ast.TypeRef
	attributes []ast.Attribute
}

type enumVariantInfo struct {
	name        string
	fields      map[string]fieldInfo
	fieldOrder  []string
	payloadType string
	hasPayload  bool
}

type typeInfo struct {
	name         string
	kind         string
	fields       map[string]fieldInfo
	enumVariants map[string]enumVariantInfo
	target       ast.TypeRef
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

type conceptFieldSpec struct {
	Path         string
	Type         ast.TypeRef
	DefaultValue ast.Expr
	ZeroAllowed  bool
}

type varOrigin string

const (
	varLocal     varOrigin = "local"
	varParam     varOrigin = "param"
	varResource  varOrigin = "resource"
	varWorkgroup varOrigin = "workgroup"
	varBuiltin   varOrigin = "builtin"
	varComptime  varOrigin = "comptime"
)

type varInfo struct {
	typ    ast.TypeRef
	origin varOrigin
	access string
	value  *configValue
}

const maxRegTileElements = 64

type validator struct {
	errors         []string
	types          map[string]typeInfo
	funcs          map[string]functionInfo
	configs        map[string]configInfo
	concepts       map[string]ast.ConceptDecl
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
		case ast.BoardDecl:
			v.addFieldType(d.Name, "board", d.Fields, "board")
		case ast.StreamDecl:
			v.addFieldType(d.Name, "stream", d.Fields, "stream")
		case ast.ConceptDecl:
			v.addType(d.Name, typeInfo{name: d.Name, kind: "concept", fields: v.collectConceptFieldMap(d)})
			v.concepts[d.Name] = d
		case ast.ConfigDecl:
			if _, exists := v.configs[d.Name]; exists {
				v.errorf("duplicate top-level name %s", d.Name)
				continue
			}
			v.configs[d.Name] = configInfo{conceptName: d.ConceptName, fields: map[string]configValue{}}
		case ast.EnumDecl:
			v.addEnumType(d)
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
			if d.Kind == "flow" {
				v.errorf("SDSL-V flow/state controllers are planned but not supported in M21")
			} else {
				v.errorf("%s is not implemented in GoOct SDSL-V M0", d.Kind)
			}
		}
	}
}

func (v *validator) addFieldType(name, kind string, fields []ast.Field, label string) {
	collected := map[string]fieldInfo{}
	for _, field := range fields {
		if _, exists := collected[field.Name]; exists {
			v.errorf("duplicate %s field %s.%s", label, name, field.Name)
		}
		collected[field.Name] = fieldInfo{access: field.Access, typ: field.Type, attributes: field.Attributes}
	}
	v.addType(name, typeInfo{name: name, kind: kind, fields: collected})
}

func (v *validator) collectConceptFieldMap(concept ast.ConceptDecl) map[string]fieldInfo {
	collected := map[string]fieldInfo{}
	for _, spec := range v.conceptFieldSpecs(concept) {
		if _, exists := collected[spec.Path]; exists {
			continue
		}
		collected[spec.Path] = fieldInfo{typ: spec.Type}
	}
	return collected
}

func (v *validator) conceptFieldSpecs(concept ast.ConceptDecl) []conceptFieldSpec {
	var out []conceptFieldSpec
	var walk func([]ast.ConceptMember, []string)
	walk = func(members []ast.ConceptMember, prefix []string) {
		seen := map[string]string{}
		for _, member := range members {
			switch m := member.(type) {
			case ast.ConceptField:
				if prior, exists := seen[m.Name]; exists {
					if prior == "group" {
						v.errorf("concept %s path %s cannot be both a group and field", concept.Name, joinConceptPath(prefix, m.Name))
					} else {
						v.errorf("duplicate concept field %s.%s", concept.Name, joinConceptPath(prefix, m.Name))
					}
					continue
				}
				seen[m.Name] = "field"
				out = append(out, conceptFieldSpec{
					Path:         joinConceptPath(prefix, m.Name),
					Type:         m.Type,
					DefaultValue: m.DefaultValue,
					ZeroAllowed:  m.Type.ZeroAllowed,
				})
			case ast.ConceptGroup:
				if prior, exists := seen[m.Name]; exists {
					if prior == "field" {
						v.errorf("concept %s path %s cannot be both a field and group", concept.Name, joinConceptPath(prefix, m.Name))
					} else {
						v.errorf("duplicate concept group %s.%s", concept.Name, joinConceptPath(prefix, m.Name))
					}
					continue
				}
				seen[m.Name] = "group"
				walk(m.Members, append(append([]string(nil), prefix...), m.Name))
			}
		}
	}
	walk(concept.Members, nil)
	return out
}

func (v *validator) addEnumType(enum ast.EnumDecl) {
	variants := map[string]enumVariantInfo{}
	for _, variant := range enum.Variants {
		if _, exists := variants[variant.Name]; exists {
			v.errorf("duplicate enum variant %s.%s", enum.Name, variant.Name)
			continue
		}
		fields := map[string]fieldInfo{}
		order := make([]string, 0, len(variant.Fields))
		for _, field := range variant.Fields {
			if _, exists := fields[field.Name]; exists {
				v.errorf("duplicate enum payload field %s.%s.%s", enum.Name, variant.Name, field.Name)
				continue
			}
			fields[field.Name] = fieldInfo{typ: field.Type}
			order = append(order, field.Name)
		}
		info := enumVariantInfo{name: variant.Name, fields: fields, fieldOrder: order, hasPayload: variant.Payload}
		if variant.Payload {
			info.payloadType = payloadTypeName(enum.Name, variant.Name)
			v.addType(info.payloadType, typeInfo{name: info.payloadType, kind: "record", fields: fields})
		}
		variants[variant.Name] = info
	}
	v.addType(enum.Name, typeInfo{name: enum.Name, kind: "enum", enumVariants: variants})
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
		case ast.BoardDecl:
			v.validateFields(d.Name, "board", d.Fields, false)
			v.validateBoardFields(d)
		case ast.StreamDecl:
			v.validateFields(d.Name, "stream", d.Fields, true)
			v.validateComputeThreadStream(d)
		case ast.ConceptDecl:
			v.validateConcept(d)
		case ast.ConfigDecl:
			v.validateConfig(d)
		case ast.EnumDecl:
			v.validateEnum(d)
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
		if len(field.Attributes) > 0 && (kind != "stream" || field.Access == "") {
			v.errorf("%s field %s.%s does not support attributes in SDSL-V M6", kind, owner, field.Name)
		}
		v.validateType(field.Type)
		if field.Type.Name == "reg_tile" {
			v.errorf("%s field %s.%s cannot use reg_tile<T, Rows, Cols> in SDSL-V M15", kind, owner, field.Name)
		}
	}
}

func (v *validator) validateBoardFields(board ast.BoardDecl) {
	for _, field := range board.Fields {
		if !v.isAllowedBoardFieldType(field.Type) {
			v.errorf("board field %s.%s type %s is not supported in SDSL-V M21; board fields must be bool, i32, u32, f32, float, or supported scalar vector types", board.Name, field.Name, typeName(field.Type))
		}
	}
}

func (v *validator) isAllowedBoardFieldType(ref ast.TypeRef) bool {
	resolved := v.resolveAlias(ref)
	switch resolved.Name {
	case "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4", "uint2", "uint3", "uint4":
		return true
	default:
		return false
	}
}

func (v *validator) validateConcept(concept ast.ConceptDecl) {
	specs := v.conceptFieldSpecs(concept)
	env := map[string]configValue{}
	for _, field := range specs {
		resolved := v.resolveAlias(field.Type)
		switch resolved.Name {
		case "u32", "i32", "bool", "f32", "float":
		default:
			v.errorf("concept field %s.%s must use a compile-time scalar type in SDSL-V M5", concept.Name, field.Path)
		}
		if field.ZeroAllowed && resolved.Name != "u32" {
			v.errorf("concept field %s.%s may only use u32! in SDSL-V M11", concept.Name, field.Path)
		}
		if field.DefaultValue != nil {
			value, err := v.evalConstExpr(field.DefaultValue, env)
			if err != nil {
				v.errorf("concept field %s.%s default: %v", concept.Name, field.Path, err)
			} else if !v.compatible(field.Type, value.typ) {
				v.errorf("concept field %s.%s default expects %s, got %s", concept.Name, field.Path, typeName(field.Type), typeName(value.typ))
			} else if resolved.Name == "u32" && !field.ZeroAllowed && value.int32 == 0 {
				v.errorf("config field %s is nonzero by default; use u32! if zero is intentional", field.Path)
			} else {
				env[field.Path] = value
				continue
			}
		}
		env[field.Path] = placeholderConfigValue(resolved)
	}
	for _, requirement := range concept.Requirements {
		value, err := v.evalConstExpr(requirement.Expr, env)
		if err != nil {
			v.errorf("concept %s require %s: %v", concept.Name, requirement.Text, err)
			continue
		}
		if value.typ.Name != "bool" {
			v.errorf("concept %s require %s must evaluate to bool", concept.Name, requirement.Text)
		}
	}
}

func (v *validator) validateConfig(config ast.ConfigDecl) {
	info, ok := v.types[config.ConceptName]
	if !ok || info.kind != "concept" {
		v.errorf("unknown concept %s for config %s", config.ConceptName, config.Name)
		return
	}
	concept := v.concepts[config.ConceptName]
	specs := v.conceptFieldSpecs(concept)
	seen := map[string]struct{}{}
	assignments := map[string]ast.ConfigField{}
	for _, field := range config.Fields {
		if _, exists := seen[field.Path]; exists {
			v.errorf("duplicate config field %s.%s", config.Name, field.Path)
			continue
		}
		seen[field.Path] = struct{}{}
		conceptField, ok := info.fields[field.Path]
		if !ok {
			v.errorf("unknown config field %s.%s", config.Name, field.Path)
			continue
		}
		assignments[field.Path] = field
		_ = conceptField
	}
	values := map[string]configValue{}
	for _, spec := range specs {
		field := spec.Path
		if assignment, exists := assignments[field]; exists {
			value, err := v.evalConstExpr(assignment.Value, values)
			if err != nil {
				v.errorf("config %s field %s: %v", config.Name, field, err)
				values[field] = placeholderConfigValue(v.resolveAlias(spec.Type))
				continue
			}
			if !v.compatible(spec.Type, value.typ) {
				v.errorf("config %s field %s expects %s, got %s", config.Name, field, typeName(spec.Type), typeName(value.typ))
				values[field] = placeholderConfigValue(v.resolveAlias(spec.Type))
				continue
			}
			if err := v.validateNonZeroConfigField(field, spec, value); err != nil {
				v.errorf("%s", err.Error())
			}
			values[field] = value
			continue
		}
		if spec.DefaultValue != nil {
			value, err := v.evalConstExpr(spec.DefaultValue, values)
			if err != nil {
				v.errorf("config %s field %s default: %v", config.Name, field, err)
				values[field] = placeholderConfigValue(v.resolveAlias(spec.Type))
				continue
			}
			if !v.compatible(spec.Type, value.typ) {
				v.errorf("config %s field %s default expects %s, got %s", config.Name, field, typeName(spec.Type), typeName(value.typ))
				values[field] = placeholderConfigValue(v.resolveAlias(spec.Type))
				continue
			}
			if err := v.validateNonZeroConfigField(field, spec, value); err != nil {
				v.errorf("%s", err.Error())
			}
			values[field] = value
			continue
		}
		v.errorf("config field %s missing", field)
		values[field] = placeholderConfigValue(v.resolveAlias(spec.Type))
	}
	cfg := v.configs[config.Name]
	cfg.fields = values
	v.configs[config.Name] = cfg
	for _, requirement := range concept.Requirements {
		value, err := v.evalConstExpr(requirement.Expr, values)
		if err != nil {
			v.errorf("config %s require %s: %v", config.Name, requirement.Text, err)
			continue
		}
		if value.typ.Name != "bool" {
			v.errorf("config %s require %s must evaluate to bool", config.Name, requirement.Text)
			continue
		}
		if !value.boolVal {
			v.errorf("config %s failed requirement %s", config.Name, requirement.Text)
		}
	}
	for _, requirement := range config.Requirements {
		value, err := v.evalConstExpr(requirement.Expr, values)
		if err != nil {
			v.errorf("config %s require %s: %v", config.Name, requirement.Text, err)
			continue
		}
		if value.typ.Name != "bool" {
			v.errorf("config %s require %s must evaluate to bool", config.Name, requirement.Text)
			continue
		}
		if !value.boolVal {
			v.errorf("config %s failed requirement %s", config.Name, requirement.Text)
		}
	}
}

func (v *validator) validateEnum(enum ast.EnumDecl) {
	for _, variant := range enum.Variants {
		for _, field := range variant.Fields {
			if field.Access != "" {
				v.errorf("enum payload field %s.%s.%s must not declare resource access", enum.Name, variant.Name, field.Name)
			}
			if len(field.Attributes) > 0 {
				v.errorf("enum payload field %s.%s.%s does not support attributes", enum.Name, variant.Name, field.Name)
			}
			v.validateType(field.Type)
			if !v.isAllowedEnumPayloadType(field.Type) {
				v.errorf("enum payload field %s.%s.%s type %s is not supported in GoOct SDSL-V M9", enum.Name, variant.Name, field.Name, typeName(field.Type))
			}
		}
	}
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
	env := map[string]configValue{}
	for name, value := range config.fields {
		env[shader.Template.Name+"."+name] = value
	}
	for _, staticAssert := range shader.StaticAsserts {
		value, err := v.evalConstExpr(staticAssert.Expr, env)
		if err != nil {
			v.errorf("compile %s as %s static assert %s: %v", decl.ShaderName, decl.AliasName, staticAssert.Text, err)
			continue
		}
		if value.typ.Name != "bool" {
			v.errorf("compile %s as %s static assert %s must evaluate to bool", decl.ShaderName, decl.AliasName, staticAssert.Text)
			continue
		}
		if !value.boolVal {
			v.errorf("compile %s as %s failed static assert %s", decl.ShaderName, decl.AliasName, staticAssert.Text)
		}
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
		} else if v.typeKind(resource.Type.Args[0]) == "board" {
			v.errorf("resource %s.%s cannot use board element type %s; SDSL-V M21 boards are shader-local values and do not affect resource bindings", shader.Name, resource.Name, typeName(resource.Type.Args[0]))
		}
		v.validateResourceAttributes(shader.Name, resource)
		v.validateType(resource.Type)
		v.resources[resource.Name] = resource
	}
	v.validateResourceBindings(shader.Name, resources)
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
	for _, staticAssert := range shader.StaticAsserts {
		typ := v.exprType(staticAssert.Expr, map[string]varInfo{}, shader.Name, shader.Template)
		if typ.Name != "bool" && typ.Name != "<error>" {
			v.errorf("shader %s static assert %s must evaluate to bool", shader.Name, staticAssert.Text)
		}
	}
	if shader.Template == nil {
		for _, staticAssert := range shader.StaticAsserts {
			value, err := v.evalConstExpr(staticAssert.Expr, nil)
			if err != nil {
				v.errorf("shader %s static assert %s: %v", shader.Name, staticAssert.Text, err)
				continue
			}
			if value.typ.Name != "bool" {
				v.errorf("shader %s static assert %s must evaluate to bool", shader.Name, staticAssert.Text)
				continue
			}
			if !value.boolVal {
				v.errorf("shader %s failed static assert %s", shader.Name, staticAssert.Text)
			}
		}
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
		resources = append(resources, ast.ResourceDecl{Name: name, Access: field.access, Type: field.typ, Attributes: field.attributes})
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
		scope[resource.Name] = varInfo{typ: resource.Type, origin: varResource, access: resource.Access}
	}
	for _, workgroup := range workgroups {
		scope[workgroup.Name] = varInfo{typ: workgroup.Type, origin: varWorkgroup}
	}
	for _, param := range fn.Parameters {
		if _, exists := scope[param.Name]; exists {
			v.errorf("duplicate parameter or builtin name %s in %s", param.Name, fn.Name)
		}
		v.validateType(param.Type)
		if stage != "" && v.typeKind(param.Type) == "board" {
			v.errorf("stage parameter %s cannot use board type %s; SDSL-V M21 boards are shader-local values, not push constants or interface payloads", param.Name, typeName(param.Type))
		}
		if param.Type.Name == "tile" || param.Type.Name == "matrix_view" {
			v.errorf("%s parameters are not supported in SDSL-V M12", param.Type.Name)
		}
		if param.Type.Name == "reg_tile" {
			v.errorf("reg_tile parameters are not supported in SDSL-V M15")
		}
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
		if s.Type.Name == "tile" {
			v.errorf("tile<T,R,C> is only valid for workgroup declarations in SDSL-V M12")
		}
		if s.Type.Name == "reg_tile" {
			v.validateLocalRegTileType(s.Type, s.Name, scope, shaderName, templateParam)
		}
		if s.Type.Name == "matrix_view" && s.Value == nil {
			v.errorf("matrix_view locals must be initialized with row_major(...) in SDSL-V M12")
		}
		if s.Type.Name == "matrix_view" && s.Value != nil && !isRowMajorCall(s.Value) {
			v.errorf("matrix_view locals must be initialized with row_major(...) in SDSL-V M12")
		}
		if s.Type.Name == "reg_tile" && s.Value == nil {
			v.errorf("reg_tile locals must be initialized with reg_tile_zero() in SDSL-V M15")
		}
		if s.Type.Name == "reg_tile" && s.Value != nil && !isRegTileZeroCall(s.Value) {
			v.errorf("reg_tile locals must be initialized with reg_tile_zero() in SDSL-V M15")
		}
		valueType := ast.TypeRef{}
		if s.Value != nil {
			v.validateWithPlacement(s.Value, true)
			v.validateGuardedReadPlacement(s.Value, true)
			v.validateMatchPlacement(s.Value, true)
			v.validateReductionPlacement(s.Value, true)
			v.validateBarrierUsage(s.Value, false, shaderName, stage)
			if s.Type.Name == "reg_tile" && isRegTileZeroCall(s.Value) {
				valueType = s.Type
			} else {
				valueType = v.exprType(s.Value, scope, shaderName, templateParam)
			}
			if !v.compatible(s.Type, valueType) {
				v.errorf("cannot assign %s to local %s of type %s", typeName(valueType), s.Name, typeName(s.Type))
			}
		}
		if _, exists := scope[s.Name]; exists {
			v.errorf("duplicate local name %s", s.Name)
		}
		access := s.Type.Access
		if s.Value != nil && valueType.Name == "matrix_view" {
			access = valueType.Access
		}
		localType := s.Type
		localType.Access = access
		var localValue *configValue
		if s.Type.Name == "reg_tile" {
			localValue = nil
		}
		scope[s.Name] = varInfo{typ: localType, origin: varLocal, access: access, value: localValue}
	case ast.ComptimeLetStmt:
		v.validateType(s.Type)
		if v.typeKind(s.Type) == "board" {
			v.errorf("comptime let %s cannot use board type %s in SDSL-V M21; structured consteval boards are not supported", s.Name, typeName(s.Type))
		}
		v.validateWithPlacement(s.Value, false)
		v.validateGuardedReadPlacement(s.Value, true)
		v.validateMatchPlacement(s.Value, false)
		v.validateReductionPlacement(s.Value, false)
		if containsGuardedReadExpr(s.Value) {
			v.errorf("guarded read is not a compile-time expression in SDSL-V M16a")
		}
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		if !v.compatible(s.Type, valueType) {
			v.errorf("cannot assign %s to comptime local %s of type %s", typeName(valueType), s.Name, typeName(s.Type))
		}
		if _, exists := scope[s.Name]; exists {
			v.errorf("duplicate local name %s", s.Name)
		}
		value, err := v.evalConstExpr(s.Value, v.constEnv(scope, templateParam))
		if err != nil {
			value = placeholderConfigValue(v.resolveAlias(s.Type))
		}
		scope[s.Name] = varInfo{typ: s.Type, origin: varComptime, value: &value}
	case ast.AssignStmt:
		v.validateWithPlacement(s.Target, false)
		v.validateWithPlacement(s.Value, true)
		v.validateGuardedReadPlacement(s.Target, false)
		v.validateGuardedReadPlacement(s.Value, true)
		v.validateMatchPlacement(s.Target, false)
		v.validateMatchPlacement(s.Value, true)
		v.validateReductionPlacement(s.Target, false)
		v.validateReductionPlacement(s.Value, true)
		v.validateBarrierUsage(s.Target, false, shaderName, stage)
		v.validateBarrierUsage(s.Value, false, shaderName, stage)
		targetType := v.exprType(s.Target, scope, shaderName, templateParam)
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		if !isAssignableTarget(s.Target) {
			v.errorf("assignment target is not assignable")
		}
		v.validateImmutableAssignmentTarget(s.Target, scope)
		if targetType.Name == "reg_tile" || valueType.Name == "reg_tile" {
			v.errorf("whole reg_tile assignment is not supported in SDSL-V M15")
		}
		if v.typeKind(targetType) == "board" || v.assignmentTouchesBoardField(s.Target, scope) {
			v.errorf("board values are immutable in SDSL-V M21; board field assignment is reserved for flow-bound mutable board state")
		}
		if !v.compatible(targetType, valueType) {
			v.errorf("assignment type mismatch: %s = %s", typeName(targetType), typeName(valueType))
		}
	case ast.GuardedWriteStmt:
		v.validateWithPlacement(s.Target, false)
		v.validateWithPlacement(s.Value, true)
		v.validateWithPlacement(s.Condition, false)
		v.validateGuardedReadPlacement(s.Target, false)
		v.validateGuardedReadPlacement(s.Value, true)
		v.validateGuardedReadPlacement(s.Condition, false)
		v.validateMatchPlacement(s.Target, false)
		v.validateMatchPlacement(s.Value, true)
		v.validateMatchPlacement(s.Condition, false)
		v.validateReductionPlacement(s.Target, false)
		v.validateReductionPlacement(s.Value, true)
		v.validateReductionPlacement(s.Condition, false)
		v.validateBarrierUsage(s.Target, false, shaderName, stage)
		v.validateBarrierUsage(s.Value, false, shaderName, stage)
		v.validateBarrierUsage(s.Condition, false, shaderName, stage)
		if !isGuardedMemoryReadTarget(s.Target) || !isGuardedMemoryWriteTarget(s.Target, scope) {
			v.errorf("guarded write target must be a writable indexed memory expression")
		}
		if isReadonlyMatrixViewIndex(s.Target, scope) {
			v.errorf("cannot guarded-write to readonly matrix view")
		}
		targetType := v.exprType(s.Target, scope, shaderName, templateParam)
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		guardType := v.exprType(s.Condition, scope, shaderName, templateParam)
		if guardType.Name != "bool" {
			v.errorf("guarded write condition must be bool")
		}
		if !v.compatible(targetType, valueType) {
			v.errorf("guarded write value type does not match target element type")
		}
	case ast.ReturnStmt:
		if s.Value == nil {
			if returnType.Name != "void" {
				v.errorf("return without value in function returning %s", typeName(returnType))
			}
			return
		}
		v.validateWithPlacement(s.Value, true)
		v.validateGuardedReadPlacement(s.Value, true)
		v.validateMatchPlacement(s.Value, true)
		v.validateReductionPlacement(s.Value, true)
		v.validateBarrierUsage(s.Value, false, shaderName, stage)
		valueType := v.exprType(s.Value, scope, shaderName, templateParam)
		if valueType.Name == "reg_tile" || returnType.Name == "reg_tile" {
			v.errorf("reg_tile return values are not supported in SDSL-V M15")
		}
		if !v.compatible(returnType, valueType) {
			v.errorf("return type mismatch: expected %s, got %s", typeName(returnType), typeName(valueType))
		}
	case ast.ExprStmt:
		v.validateWithPlacement(s.Value, false)
		v.validateGuardedReadPlacement(s.Value, false)
		v.validateMatchPlacement(s.Value, false)
		v.validateReductionPlacement(s.Value, false)
		v.validateBarrierUsage(s.Value, true, shaderName, stage)
		v.exprType(s.Value, scope, shaderName, templateParam)
	case ast.IfStmt:
		v.validateWithPlacement(s.Condition, false)
		v.validateGuardedReadPlacement(s.Condition, false)
		v.validateMatchPlacement(s.Condition, false)
		v.validateReductionPlacement(s.Condition, false)
		v.validateBarrierUsage(s.Condition, false, shaderName, stage)
		cond := v.exprType(s.Condition, scope, shaderName, templateParam)
		if cond.Name != "bool" {
			v.errorf("if condition must be bool, got %s", typeName(cond))
		}
		v.validateBlock(s.ThenBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.GuardWhenStmt:
		for _, c := range s.Cases {
			v.validateWithPlacement(c.Condition, false)
			v.validateGuardedReadPlacement(c.Condition, false)
			v.validateMatchPlacement(c.Condition, false)
			v.validateReductionPlacement(c.Condition, false)
			v.validateBarrierUsage(c.Condition, false, shaderName, stage)
			guardType := v.exprType(c.Condition, scope, shaderName, templateParam)
			if guardType.Name != "bool" && guardType.Name != "<error>" {
				v.errorf("guard when case condition must be bool")
			}
			v.validateBlock(c.Body, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.ComptimeIfStmt:
		v.validateWithPlacement(s.Condition, false)
		v.validateGuardedReadPlacement(s.Condition, false)
		v.validateMatchPlacement(s.Condition, false)
		v.validateReductionPlacement(s.Condition, false)
		cond := v.exprType(s.Condition, scope, shaderName, templateParam)
		if cond.Name != "bool" {
			v.errorf("comptime if condition must be compile-time bool")
		}
		v.validateBlock(s.ThenBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.ComptimeMatchStmt:
		v.validateWithPlacement(s.Subject, false)
		v.validateMatchPlacement(s.Subject, false)
		v.validateReductionPlacement(s.Subject, false)
		subjectType := v.exprType(s.Subject, scope, shaderName, templateParam)
		if !isInteger(subjectType) && subjectType.Name != "bool" && subjectType.Name != "<error>" {
			v.errorf("comptime match scrutinee must be compile-time")
		}
		seenElse := false
		for _, arm := range s.Arms {
			if arm.IsElse {
				if seenElse {
					v.errorf("duplicate comptime match else arm")
				}
				seenElse = true
			} else {
				if !isComptimeMatchLiteralPattern(arm.Pattern) {
					v.errorf("comptime match arm pattern must be compile-time literal")
				}
				v.validateWithPlacement(arm.Pattern, false)
				v.validateMatchPlacement(arm.Pattern, false)
				v.validateReductionPlacement(arm.Pattern, false)
				patternType := v.exprType(arm.Pattern, scope, shaderName, templateParam)
				if subjectType.Name != "<error>" && patternType.Name != "<error>" && !v.compatible(subjectType, patternType) {
					v.errorf("comptime match arm pattern type %s does not match scrutinee type %s", typeName(patternType), typeName(subjectType))
				}
			}
			v.validateBlock(arm.Body, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.ComptimeWhenUtilityStmt:
		seenLabels := map[string]struct{}{}
		for _, c := range s.Cases {
			if _, exists := seenLabels[c.Label]; exists {
				v.errorf("duplicate comptime when utility case label %s", c.Label)
			}
			seenLabels[c.Label] = struct{}{}
			if c.Condition != nil {
				v.validateWithPlacement(c.Condition, false)
				v.validateGuardedReadPlacement(c.Condition, false)
				v.validateMatchPlacement(c.Condition, false)
				v.validateReductionPlacement(c.Condition, false)
				guardType := v.exprType(c.Condition, scope, shaderName, templateParam)
				if guardType.Name != "bool" && guardType.Name != "<error>" {
					v.errorf("comptime when guard must be compile-time bool")
				}
			}
			v.validateWithPlacement(c.Score, false)
			v.validateGuardedReadPlacement(c.Score, false)
			v.validateMatchPlacement(c.Score, false)
			v.validateReductionPlacement(c.Score, false)
			scoreType := v.exprType(c.Score, scope, shaderName, templateParam)
			if !isNumeric(scoreType) && scoreType.Name != "<error>" {
				v.errorf("comptime when score must be compile-time numeric")
			}
			v.validateBlock(c.Body, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam)
		}
	case ast.ComptimeForStmt:
		v.validateWithPlacement(s.Start, false)
		v.validateWithPlacement(s.End, false)
		v.validateGuardedReadPlacement(s.Start, false)
		v.validateGuardedReadPlacement(s.End, false)
		v.validateMatchPlacement(s.Start, false)
		v.validateMatchPlacement(s.End, false)
		v.validateReductionPlacement(s.Start, false)
		v.validateReductionPlacement(s.End, false)
		startType := v.exprType(s.Start, scope, shaderName, templateParam)
		endType := v.exprType(s.End, scope, shaderName, templateParam)
		if !isInteger(startType) || !isInteger(endType) {
			v.errorf("comptime for bounds must be compile-time integers")
		}
		env := v.constEnv(scope, templateParam)
		startValue, startErr := v.evalConstExpr(s.Start, env)
		endValue, endErr := v.evalConstExpr(s.End, env)
		if startErr != nil || endErr != nil {
			v.errorf("comptime for bounds must be compile-time integers")
		} else {
			if !isInteger(startValue.typ) || !isInteger(endValue.typ) {
				v.errorf("comptime for bounds must be compile-time integers")
			}
			if startValue.int32 < 0 || endValue.int32 < 0 {
				v.errorf("comptime for bounds must be non-negative in SDSL-V M16")
			}
			if startValue.int32 > endValue.int32 {
				v.errorf("comptime for range start must be <= end")
			}
		}
		loopScope := cloneScope(scope)
		loopScope[s.Name] = varInfo{
			typ:    ast.TypeRef{Name: "u32"},
			origin: varComptime,
			value:  &configValue{typ: ast.TypeRef{Name: "u32"}, int32: 0},
		}
		v.validateBlock(s.Body, returnType, loopScope, shaderName, stage, templateParam)
	case ast.ForStmt:
		v.validateLoopAttributes(s.Attributes)
		v.validateWithPlacement(s.Start, false)
		v.validateWithPlacement(s.End, false)
		v.validateWithPlacement(s.Step, false)
		v.validateGuardedReadPlacement(s.Start, false)
		v.validateGuardedReadPlacement(s.End, false)
		v.validateGuardedReadPlacement(s.Step, false)
		v.validateMatchPlacement(s.Start, false)
		v.validateMatchPlacement(s.End, false)
		v.validateMatchPlacement(s.Step, false)
		v.validateReductionPlacement(s.Start, false)
		v.validateReductionPlacement(s.End, false)
		v.validateReductionPlacement(s.Step, false)
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
	case ast.StaticAssertStmt:
		v.validateGuardedReadPlacement(s.Expr, false)
		typ := v.exprType(s.Expr, scope, shaderName, templateParam)
		if typ.Name != "bool" && typ.Name != "<error>" {
			v.errorf("static assert %s must evaluate to bool", s.Text)
		}
	}
}

func (v *validator) validateBlock(block ast.Block, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam) {
	for _, stmt := range block.Statements {
		v.validateStmt(stmt, returnType, scope, shaderName, stage, templateParam)
	}
}

func (v *validator) validateType(ref ast.TypeRef) {
	if ref.ZeroAllowed {
		v.errorf("u32! is only valid for concept/config fields in SDSL-V M11")
		return
	}
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
	if ref.Name == "tile" {
		if len(ref.Args) != 1 || !ref.HasTileShape {
			v.errorf("tile type requires tile<T, Rows, Cols>")
			return
		}
		v.validateType(ref.Args[0])
		return
	}
	if ref.Name == "reg_tile" {
		if len(ref.Args) != 1 || !ref.HasTileShape {
			v.errorf("reg_tile type requires reg_tile<T, Rows, Cols>")
			return
		}
		v.validateType(ref.Args[0])
		return
	}
	if ref.Name == "matrix_view" {
		if len(ref.Args) != 1 {
			v.errorf("matrix_view type requires one element type")
			return
		}
		v.validateType(ref.Args[0])
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
		if id, ok := e.Target.(ast.IdentifierExpr); ok {
			if enumInfo, exists := v.types[id.Name]; exists && enumInfo.kind == "enum" {
				variant, ok := enumInfo.enumVariants[e.Field]
				if !ok {
					v.errorf("unknown enum variant %s.%s", id.Name, e.Field)
					return ast.TypeRef{Name: "<error>"}
				}
				if variant.hasPayload {
					v.errorf("payload variant %s.%s requires payload construction", id.Name, e.Field)
					return ast.TypeRef{Name: id.Name}
				}
				return ast.TypeRef{Name: id.Name}
			}
		}
		if templateParam != nil {
			if path, ok := templateFieldPath(e, templateParam.Name); ok {
				concept, ok := v.types[templateParam.ConceptName]
				if !ok || concept.kind != "concept" {
					v.errorf("unknown concept %s on template shader %s", templateParam.ConceptName, shaderName)
					return ast.TypeRef{Name: "<error>"}
				}
				if fieldType, ok := concept.fields[path]; ok {
					return v.resolveAlias(fieldType.typ)
				}
				v.errorf("unknown template field %s on concept %s", path, templateParam.ConceptName)
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
		if e.HasSecond {
			index2 := v.exprType(e.Index2, scope, shaderName, templateParam)
			if !isInteger(index2) {
				v.errorf("2D index second operand must be integer")
			}
			switch target.Name {
			case "tile":
				if len(target.Args) == 1 {
					return v.resolveAlias(target.Args[0])
				}
			case "reg_tile":
				if len(target.Args) == 1 {
					return v.resolveAlias(target.Args[0])
				}
			case "matrix_view":
				if len(target.Args) == 1 {
					return v.resolveAlias(target.Args[0])
				}
			}
			v.errorf("2D indexing is only valid on tile<T,R,C>, reg_tile<T,R,C>, or matrix_view<T>, got %s", typeName(target))
			return ast.TypeRef{Name: "<error>"}
		}
		if target.Name == "tile" {
			v.errorf("tile values require explicit 2D indexing")
			return ast.TypeRef{Name: "<error>"}
		}
		if target.Name == "reg_tile" {
			v.errorf("reg_tile values require explicit 2D indexing")
			return ast.TypeRef{Name: "<error>"}
		}
		if target.Name == "matrix_view" {
			v.errorf("matrix_view values require explicit 2D indexing")
			return ast.TypeRef{Name: "<error>"}
		}
		if target.Name == "array" && len(target.Args) == 1 {
			return v.resolveAlias(target.Args[0])
		}
		v.errorf("cannot index non-array type %s", typeName(target))
		return ast.TypeRef{Name: "<error>"}
	case ast.GuardedReadExpr:
		if !isGuardedMemoryReadTarget(e.Target) {
			v.errorf("guarded read target must be an indexed memory expression")
		}
		targetType := v.exprType(e.Target, scope, shaderName, templateParam)
		guardType := v.exprType(e.Condition, scope, shaderName, templateParam)
		fallbackType := v.exprType(e.Fallback, scope, shaderName, templateParam)
		if guardType.Name != "bool" {
			v.errorf("guarded read condition must be bool")
		}
		if !v.compatible(targetType, fallbackType) {
			v.errorf("guarded read fallback type does not match target element type")
		}
		return targetType
	case ast.CallExpr:
		return v.callType(e, scope, shaderName, templateParam)
	case ast.BinaryExpr:
		left := v.exprType(e.Left, scope, shaderName, templateParam)
		right := v.exprType(e.Right, scope, shaderName, templateParam)
		switch e.Operator {
		case "and", "or":
			if left.Name != "bool" || right.Name != "bool" {
				v.errorf("operator `%s` requires bool operands", e.Operator)
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
		switch e.Operator {
		case "not":
			if operand.Name != "bool" {
				v.errorf("operator `not` requires bool operand")
			}
			return ast.TypeRef{Name: "bool"}
		default:
			if !isNumeric(operand) {
				v.errorf("unary %s requires numeric operand", e.Operator)
			}
		}
		return operand
	case ast.ParenExpr:
		return v.exprType(e.Inner, scope, shaderName, templateParam)
	case ast.EnumConstructExpr:
		enumInfo, ok := v.types[e.EnumName]
		if !ok || enumInfo.kind != "enum" {
			v.errorf("unknown enum %s", e.EnumName)
			return ast.TypeRef{Name: "<error>"}
		}
		variant, ok := enumInfo.enumVariants[e.VariantName]
		if !ok {
			v.errorf("unknown enum variant %s.%s", e.EnumName, e.VariantName)
			return ast.TypeRef{Name: "<error>"}
		}
		if !variant.hasPayload {
			if len(e.Fields) > 0 {
				v.errorf("simple variant %s.%s must not be constructed with payload", e.EnumName, e.VariantName)
			}
			return ast.TypeRef{Name: e.EnumName}
		}
		seen := map[string]struct{}{}
		for _, field := range e.Fields {
			if _, exists := seen[field.Name]; exists {
				v.errorf("duplicate payload field %s on %s.%s", field.Name, e.EnumName, e.VariantName)
				continue
			}
			seen[field.Name] = struct{}{}
			decl, ok := variant.fields[field.Name]
			if !ok {
				v.errorf("unknown payload field %s on %s.%s", field.Name, e.EnumName, e.VariantName)
				continue
			}
			valueType := v.exprType(field.Value, scope, shaderName, templateParam)
			if !v.compatible(decl.typ, valueType) {
				v.errorf("payload field %s on %s.%s expects %s, got %s", field.Name, e.EnumName, e.VariantName, typeName(decl.typ), typeName(valueType))
			}
		}
		for _, fieldName := range variant.fieldOrder {
			if _, exists := seen[fieldName]; !exists {
				v.errorf("missing payload field %s on %s.%s", fieldName, e.EnumName, e.VariantName)
			}
		}
		return ast.TypeRef{Name: e.EnumName}
	case ast.BoardLiteralExpr:
		info, ok := v.types[e.TypeName]
		if !ok || info.kind != "board" {
			v.errorf("unknown board type %s", e.TypeName)
			return ast.TypeRef{Name: "<error>"}
		}
		seen := map[string]struct{}{}
		for _, field := range e.Fields {
			if _, exists := seen[field.Name]; exists {
				v.errorf("duplicate board literal field %s on %s", field.Name, e.TypeName)
				continue
			}
			seen[field.Name] = struct{}{}
			decl, ok := info.fields[field.Name]
			if !ok {
				v.errorf("unknown board literal field %s on %s", field.Name, e.TypeName)
				continue
			}
			valueType := v.exprType(field.Value, scope, shaderName, templateParam)
			if !v.compatible(decl.typ, valueType) {
				v.errorf("board literal field %s on %s expects %s, got %s", field.Name, e.TypeName, typeName(decl.typ), typeName(valueType))
			}
		}
		for fieldName := range info.fields {
			if _, exists := seen[fieldName]; !exists {
				v.errorf("missing board literal field %s on %s", fieldName, e.TypeName)
			}
		}
		return ast.TypeRef{Name: e.TypeName}
	case ast.MatchExpr:
		subjectType := v.resolveAlias(v.exprType(e.Subject, scope, shaderName, templateParam))
		if v.typeKind(subjectType) != "enum" {
			v.errorf("match subject must be enum, got %s", typeName(subjectType))
			return ast.TypeRef{Name: "<error>"}
		}
		enumInfo := v.types[subjectType.Name]
		seen := map[string]struct{}{}
		var result ast.TypeRef
		for i, arm := range e.Arms {
			if arm.EnumName != subjectType.Name {
				v.errorf("match arm %s.%s does not match subject enum %s", arm.EnumName, arm.VariantName, subjectType.Name)
			}
			variant, ok := enumInfo.enumVariants[arm.VariantName]
			if !ok {
				v.errorf("unknown enum variant %s.%s", arm.EnumName, arm.VariantName)
				continue
			}
			if _, exists := seen[arm.VariantName]; exists {
				v.errorf("duplicate match arm %s.%s", arm.EnumName, arm.VariantName)
			}
			seen[arm.VariantName] = struct{}{}
			armScope := cloneScope(scope)
			if variant.hasPayload {
				if arm.BindingName == "" {
					v.errorf("payload variant %s.%s must bind payload in match", arm.EnumName, arm.VariantName)
				} else {
					armScope[arm.BindingName] = varInfo{typ: ast.TypeRef{Name: variant.payloadType}, origin: varLocal}
				}
			} else if arm.BindingName != "" {
				v.errorf("simple variant %s.%s must not bind payload", arm.EnumName, arm.VariantName)
			}
			valueType := v.exprType(arm.Value, armScope, shaderName, templateParam)
			if i == 0 {
				result = valueType
			} else if !v.compatible(result, valueType) {
				v.errorf("match arms must return a uniform type")
			}
		}
		for variantName := range enumInfo.enumVariants {
			if _, exists := seen[variantName]; !exists {
				v.errorf("match missing arm for %s.%s", subjectType.Name, variantName)
			}
		}
		if result.Name == "" {
			return ast.TypeRef{Name: "<error>"}
		}
		return result
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
		if kind == "board" {
			v.errorf("board values are immutable in SDSL-V M21; use a new board literal instead of with-update")
			return ast.TypeRef{Name: "<error>"}
		}
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
	case ast.ReductionExpr:
		v.validateReductionAttributes(e.Attributes)
		startType := v.exprType(e.Start, scope, shaderName, templateParam)
		endType := v.exprType(e.End, scope, shaderName, templateParam)
		if !isInteger(startType) || !isInteger(endType) {
			v.errorf("%s reduction bounds must be integer", e.Op)
		}
		if !positiveIntegerLiteral(e.Step) {
			v.errorf("%s reduction step must be a positive integer literal", e.Op)
		}
		bodyScope := cloneScope(scope)
		bodyScope[e.Name] = varInfo{typ: startType, origin: varLocal}
		bodyType := v.exprType(e.Body, bodyScope, shaderName, templateParam)
		switch e.Op {
		case ast.ReductionSum, ast.ReductionProduct:
			if !isNumeric(bodyType) {
				v.errorf("%s reduction body must be numeric", e.Op)
				return ast.TypeRef{Name: "<error>"}
			}
			return bodyType
		case ast.ReductionMax, ast.ReductionMin:
			v.errorf("%s reduction is reserved but not yet implemented in GoOct SDSL-V M10; use an explicit loop for now", e.Op)
			if !isNumeric(bodyType) {
				v.errorf("%s reduction body must be numeric", e.Op)
				return ast.TypeRef{Name: "<error>"}
			}
			return bodyType
		default:
			v.errorf("unknown reduction operator %s", e.Op)
			return ast.TypeRef{Name: "<error>"}
		}
	default:
		v.errorf("unsupported expression in GoOct SDSL-V M3")
		return ast.TypeRef{Name: "<error>"}
	}
}

func (v *validator) callType(call ast.CallExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	if id, ok := call.Callee.(ast.IdentifierExpr); ok {
		if id.Name == "reg_tile_zero" {
			if len(call.Arguments) != 0 {
				v.errorf("reg_tile_zero expects 0 arguments, got %d", len(call.Arguments))
			}
			v.errorf("reg_tile_zero() is only valid as a direct reg_tile local initializer in SDSL-V M15")
			return ast.TypeRef{Name: "<error>"}
		}
		if id.Name == "row_major" {
			if len(call.Arguments) != 3 {
				v.errorf("row_major expects 3 arguments, got %d", len(call.Arguments))
				return ast.TypeRef{Name: "<error>"}
			}
			bufferID, ok := call.Arguments[0].(ast.IdentifierExpr)
			if !ok {
				v.errorf("row_major first argument must be a resource array")
				return ast.TypeRef{Name: "<error>"}
			}
			info, ok := scope[bufferID.Name]
			if !ok || info.origin != varResource || info.typ.Name != "array" || len(info.typ.Args) != 1 {
				v.errorf("row_major first argument must be a resource array")
				return ast.TypeRef{Name: "<error>"}
			}
			rows := v.exprType(call.Arguments[1], scope, shaderName, templateParam)
			cols := v.exprType(call.Arguments[2], scope, shaderName, templateParam)
			if !isInteger(rows) || !isInteger(cols) {
				v.errorf("row_major rows and cols must be integer expressions")
			}
			return ast.TypeRef{Name: "matrix_view", Args: []ast.TypeRef{v.resolveAlias(info.typ.Args[0])}, Access: info.access}
		}
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
		if left.Name == "matrix_view" || left.Name == "tile" || left.Name == "reg_tile" {
			return len(left.Args) == len(right.Args) && (len(left.Args) == 0 || v.compatible(left.Args[0], right.Args[0]))
		}
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
		if resolved.Name == "tile" {
			return "tile"
		}
		if resolved.Name == "reg_tile" {
			return "reg_tile"
		}
		if resolved.Name == "matrix_view" {
			return "matrix_view"
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
	case varComptime:
		v.errorf("cannot assign to comptime binding %s", root)
	case varParam:
		kind := v.typeKind(info.typ)
		switch {
		case kind == "record" || kind == "stream":
			if !isDirectIdentifier(expr) {
				v.errorf("cannot assign through immutable %s parameter %s; use with instead", kind, root)
			}
		case kind == "board":
			if !isDirectIdentifier(expr) {
				v.errorf("board values are immutable in SDSL-V M21; board field assignment is reserved for flow-bound mutable board state")
			}
		case kind == "array":
			if !isDirectIdentifier(expr) {
				v.errorf("cannot assign through immutable array parameter %s", root)
			}
		}
	}
	if info.typ.Name == "matrix_view" && info.access != "readwrite" && !isDirectIdentifier(expr) {
		v.errorf("cannot assign through readonly matrix_view %s", root)
	}
}

func (v *validator) assignmentTouchesBoardField(expr ast.Expr, scope map[string]varInfo) bool {
	if _, ok := expr.(ast.FieldAccessExpr); !ok {
		return false
	}
	root, ok := rootIdentifier(expr)
	if !ok {
		return false
	}
	info, ok := scope[root]
	if !ok {
		return false
	}
	return v.typeKind(info.typ) == "board"
}

func (v *validator) validateWithPlacement(expr ast.Expr, topLevelAllowed bool) {
	switch e := expr.(type) {
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			v.validateWithPlacement(field.Value, false)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			v.validateWithPlacement(field.Value, false)
		}
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
		if e.HasSecond {
			v.validateWithPlacement(e.Index2, false)
		}
	case ast.GuardedReadExpr:
		v.validateWithPlacement(e.Target, false)
		v.validateWithPlacement(e.Condition, false)
		v.validateWithPlacement(e.Fallback, false)
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
	case ast.MatchExpr:
		v.validateWithPlacement(e.Subject, false)
		for _, arm := range e.Arms {
			v.validateWithPlacement(arm.Value, false)
		}
	}
}

func (v *validator) validateMatchPlacement(expr ast.Expr, topLevelAllowed bool) {
	switch e := expr.(type) {
	case ast.MatchExpr:
		if !topLevelAllowed {
			v.errorf("match expression is only supported as a direct let initializer, assignment RHS, or return value in GoOct SDSL-V M9")
			return
		}
		v.validateMatchPlacement(e.Subject, false)
		for _, arm := range e.Arms {
			v.validateMatchPlacement(arm.Value, false)
		}
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			v.validateMatchPlacement(field.Value, false)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			v.validateMatchPlacement(field.Value, false)
		}
	case ast.FieldAccessExpr:
		v.validateMatchPlacement(e.Target, false)
	case ast.IndexExpr:
		v.validateMatchPlacement(e.Target, false)
		v.validateMatchPlacement(e.Index, false)
		if e.HasSecond {
			v.validateMatchPlacement(e.Index2, false)
		}
	case ast.GuardedReadExpr:
		v.validateMatchPlacement(e.Target, false)
		v.validateMatchPlacement(e.Condition, false)
		v.validateMatchPlacement(e.Fallback, false)
	case ast.CallExpr:
		v.validateMatchPlacement(e.Callee, false)
		for _, arg := range e.Arguments {
			v.validateMatchPlacement(arg, false)
		}
	case ast.BinaryExpr:
		v.validateMatchPlacement(e.Left, false)
		v.validateMatchPlacement(e.Right, false)
	case ast.UnaryExpr:
		v.validateMatchPlacement(e.Operand, false)
	case ast.ParenExpr:
		v.validateMatchPlacement(e.Inner, false)
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			v.validateMatchPlacement(c.Value, false)
			v.validateMatchPlacement(c.Condition, false)
			v.validateMatchPlacement(c.Score, false)
		}
		if e.Else != nil {
			v.validateMatchPlacement(e.Else, false)
		}
	case ast.WithExpr:
		v.validateMatchPlacement(e.Base, false)
		for _, update := range e.Updates {
			v.validateMatchPlacement(update.Value, false)
		}
	}
}

func (v *validator) validateReductionPlacement(expr ast.Expr, topLevelAllowed bool) {
	switch e := expr.(type) {
	case ast.ReductionExpr:
		if !topLevelAllowed {
			v.errorf("reduction expression is only supported as a direct let initializer, assignment RHS, or return value in GoOct SDSL-V M10")
			return
		}
		v.validateReductionPlacement(e.Start, false)
		v.validateReductionPlacement(e.End, false)
		v.validateReductionPlacement(e.Step, false)
		v.validateReductionPlacement(e.Body, false)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			v.validateReductionPlacement(field.Value, false)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			v.validateReductionPlacement(field.Value, false)
		}
	case ast.FieldAccessExpr:
		v.validateReductionPlacement(e.Target, false)
	case ast.IndexExpr:
		v.validateReductionPlacement(e.Target, false)
		v.validateReductionPlacement(e.Index, false)
		if e.HasSecond {
			v.validateReductionPlacement(e.Index2, false)
		}
	case ast.GuardedReadExpr:
		v.validateReductionPlacement(e.Target, false)
		v.validateReductionPlacement(e.Condition, false)
		v.validateReductionPlacement(e.Fallback, false)
	case ast.CallExpr:
		v.validateReductionPlacement(e.Callee, false)
		for _, arg := range e.Arguments {
			v.validateReductionPlacement(arg, false)
		}
	case ast.BinaryExpr:
		v.validateReductionPlacement(e.Left, false)
		v.validateReductionPlacement(e.Right, false)
	case ast.UnaryExpr:
		v.validateReductionPlacement(e.Operand, false)
	case ast.ParenExpr:
		v.validateReductionPlacement(e.Inner, false)
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			v.validateReductionPlacement(c.Value, false)
			v.validateReductionPlacement(c.Condition, false)
			v.validateReductionPlacement(c.Score, false)
		}
		if e.Else != nil {
			v.validateReductionPlacement(e.Else, false)
		}
	case ast.WithExpr:
		v.validateReductionPlacement(e.Base, false)
		for _, update := range e.Updates {
			v.validateReductionPlacement(update.Value, false)
		}
	case ast.MatchExpr:
		v.validateReductionPlacement(e.Subject, false)
		for _, arm := range e.Arms {
			v.validateReductionPlacement(arm.Value, false)
		}
	}
}

func (v *validator) validateGuardedReadPlacement(expr ast.Expr, topLevelAllowed bool) {
	switch e := expr.(type) {
	case ast.GuardedReadExpr:
		if !topLevelAllowed {
			v.errorf("guarded read expression is only supported as a direct let initializer, assignment RHS, or return value in SDSL-V M16a")
			return
		}
		v.validateGuardedReadPlacement(e.Target, false)
		v.validateGuardedReadPlacement(e.Condition, false)
		v.validateGuardedReadPlacement(e.Fallback, false)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			v.validateGuardedReadPlacement(field.Value, false)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			v.validateGuardedReadPlacement(field.Value, false)
		}
	case ast.FieldAccessExpr:
		v.validateGuardedReadPlacement(e.Target, false)
	case ast.IndexExpr:
		v.validateGuardedReadPlacement(e.Target, false)
		v.validateGuardedReadPlacement(e.Index, false)
		if e.HasSecond {
			v.validateGuardedReadPlacement(e.Index2, false)
		}
	case ast.CallExpr:
		v.validateGuardedReadPlacement(e.Callee, false)
		for _, arg := range e.Arguments {
			v.validateGuardedReadPlacement(arg, false)
		}
	case ast.BinaryExpr:
		v.validateGuardedReadPlacement(e.Left, false)
		v.validateGuardedReadPlacement(e.Right, false)
	case ast.UnaryExpr:
		v.validateGuardedReadPlacement(e.Operand, false)
	case ast.ParenExpr:
		v.validateGuardedReadPlacement(e.Inner, false)
	case ast.WhenUtilityExpr:
		for _, c := range e.Cases {
			v.validateGuardedReadPlacement(c.Value, false)
			v.validateGuardedReadPlacement(c.Condition, false)
			v.validateGuardedReadPlacement(c.Score, false)
		}
		if e.Else != nil {
			v.validateGuardedReadPlacement(e.Else, false)
		}
	case ast.WithExpr:
		v.validateGuardedReadPlacement(e.Base, false)
		for _, update := range e.Updates {
			v.validateGuardedReadPlacement(update.Value, false)
		}
	case ast.MatchExpr:
		v.validateGuardedReadPlacement(e.Subject, false)
		for _, arm := range e.Arms {
			v.validateGuardedReadPlacement(arm.Value, false)
		}
	case ast.ReductionExpr:
		v.validateGuardedReadPlacement(e.Start, false)
		v.validateGuardedReadPlacement(e.End, false)
		v.validateGuardedReadPlacement(e.Step, false)
		v.validateGuardedReadPlacement(e.Body, false)
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
	if decl.Type.Name == "tile" {
		if len(decl.Type.Args) != 1 || !decl.Type.HasTileShape {
			v.errorf("workgroup %s.%s must use tile<T, Rows, Cols>", shaderName, decl.Name)
			return
		}
		for _, dim := range []struct {
			name string
			expr ast.Expr
		}{
			{"rows", decl.Type.TileRows},
			{"cols", decl.Type.TileCols},
		} {
			typ := v.exprType(dim.expr, map[string]varInfo{}, shaderName, templateParam)
			if !isInteger(typ) {
				v.errorf("workgroup %s.%s tile %s must be an integer constant expression", shaderName, decl.Name, dim.name)
			} else if templateParam == nil {
				value, err := v.evalConstExpr(dim.expr, nil)
				if err != nil || value.int32 <= 0 {
					v.errorf("workgroup %s.%s tile %s must be positive", shaderName, decl.Name, dim.name)
				}
			}
		}
		v.validateType(decl.Type)
		if !isWorkgroupElementType(v.resolveAlias(decl.Type.Args[0])) {
			v.errorf("workgroup %s.%s element type %s is not supported in GoOct SDSL-V M12", shaderName, decl.Name, typeName(v.resolveAlias(decl.Type.Args[0])))
		}
		return
	}
	if decl.Type.Name == "reg_tile" {
		v.errorf("workgroup %s.%s cannot use reg_tile<T, Rows, Cols>; reg_tile is local per-thread storage in SDSL-V M15", shaderName, decl.Name)
		return
	}
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
		if e.HasSecond {
			v.validateBarrierUsage(e.Index2, false, shaderName, stage)
		}
	case ast.GuardedReadExpr:
		v.validateBarrierUsage(e.Target, false, shaderName, stage)
		v.validateBarrierUsage(e.Condition, false, shaderName, stage)
		v.validateBarrierUsage(e.Fallback, false, shaderName, stage)
	case ast.BinaryExpr:
		v.validateBarrierUsage(e.Left, false, shaderName, stage)
		v.validateBarrierUsage(e.Right, false, shaderName, stage)
	case ast.UnaryExpr:
		v.validateBarrierUsage(e.Operand, false, shaderName, stage)
	case ast.ParenExpr:
		v.validateBarrierUsage(e.Inner, false, shaderName, stage)
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			v.validateBarrierUsage(field.Value, false, shaderName, stage)
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			v.validateBarrierUsage(field.Value, false, shaderName, stage)
		}
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
	case ast.MatchExpr:
		v.validateBarrierUsage(e.Subject, false, shaderName, stage)
		for _, arm := range e.Arms {
			v.validateBarrierUsage(arm.Value, false, shaderName, stage)
		}
	case ast.ReductionExpr:
		v.validateBarrierUsage(e.Start, false, shaderName, stage)
		v.validateBarrierUsage(e.End, false, shaderName, stage)
		v.validateBarrierUsage(e.Step, false, shaderName, stage)
		v.validateBarrierUsage(e.Body, false, shaderName, stage)
	}
}

func (v *validator) errorf(format string, args ...any) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

func typeName(ref ast.TypeRef) string {
	if ref.Name == "matrix_view" && len(ref.Args) == 1 {
		if ref.Access != "" {
			return ref.Access + " matrix_view<" + typeName(ref.Args[0]) + ">"
		}
		return "matrix_view<" + typeName(ref.Args[0]) + ">"
	}
	if ref.Name == "tile" && len(ref.Args) == 1 {
		return "tile<" + typeName(ref.Args[0]) + ",R,C>"
	}
	if ref.Name == "reg_tile" && len(ref.Args) == 1 {
		return "reg_tile<" + typeName(ref.Args[0]) + ",R,C>"
	}
	if ref.Name != "array" || len(ref.Args) == 0 {
		return ref.Name
	}
	if ref.HasArraySize {
		return fmt.Sprintf("array<%s,N>", typeName(ref.Args[0]))
	}
	return fmt.Sprintf("array<%s>", typeName(ref.Args[0]))
}

func isRegTileZeroCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Callee.(ast.IdentifierExpr)
	return ok && id.Name == "reg_tile_zero" && len(call.Arguments) == 0
}

func (v *validator) validateLocalRegTileType(ref ast.TypeRef, name string, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) {
	if len(ref.Args) != 1 || !ref.HasTileShape {
		return
	}
	elem := v.resolveAlias(ref.Args[0])
	if elem.Name != "f32" && elem.Name != "float" {
		v.errorf("local reg_tile %s element type %s is not supported in SDSL-V M15", name, typeName(elem))
	}
	env := v.constEnv(scope, templateParam)
	rows, rowsErr := v.evalConstExpr(ref.TileRows, env)
	cols, colsErr := v.evalConstExpr(ref.TileCols, env)
	if rowsErr != nil {
		v.errorf("local reg_tile %s rows must be a compile-time positive integer expression", name)
	} else if !isInteger(rows.typ) || rows.int32 <= 0 {
		v.errorf("local reg_tile %s rows must be a compile-time positive integer expression", name)
	}
	if colsErr != nil {
		v.errorf("local reg_tile %s cols must be a compile-time positive integer expression", name)
	} else if !isInteger(cols.typ) || cols.int32 <= 0 {
		v.errorf("local reg_tile %s cols must be a compile-time positive integer expression", name)
	}
	if rowsErr == nil && colsErr == nil && isInteger(rows.typ) && isInteger(cols.typ) {
		if rows.int32*cols.int32 > maxRegTileElements {
			v.errorf("reg_tile has %d elements; M15 limit is %d", rows.int32*cols.int32, maxRegTileElements)
		}
	}
}

func (v *validator) constEnv(scope map[string]varInfo, templateParam *ast.TemplateParam) map[string]configValue {
	env := map[string]configValue{}
	if templateParam != nil {
		if concept, ok := v.concepts[templateParam.ConceptName]; ok {
			for _, spec := range v.conceptFieldSpecs(concept) {
				env[templateParam.Name+"."+spec.Path] = placeholderConfigValue(v.resolveAlias(spec.Type))
			}
		}
	}
	for name, info := range scope {
		if info.origin == varComptime && info.value != nil {
			env[name] = *info.value
		}
	}
	return env
}

func (v *validator) evalConstExpr(expr ast.Expr, env map[string]configValue) (configValue, error) {
	ctEnv := map[string]consteval.Value{}
	for key, value := range env {
		ctEnv[key] = consteval.Value{Type: value.typ, Int32: value.int32, Bool: value.boolVal, IsKnown: true}
	}
	value, err := consteval.Eval(expr, ctEnv)
	if err != nil {
		return configValue{}, err
	}
	return configValue{typ: value.Type, int32: value.Int32, boolVal: value.Bool}, nil
}

func (v *validator) conceptRequirementEnv(concept ast.ConceptDecl) map[string]configValue {
	env := map[string]configValue{}
	for _, field := range v.conceptFieldSpecs(concept) {
		env[field.Path] = placeholderConfigValue(v.resolveAlias(field.Type))
	}
	return env
}

func placeholderConfigValue(ref ast.TypeRef) configValue {
	value := configValue{typ: ref}
	if ref.Name == "bool" {
		value.boolVal = true
	} else {
		value.int32 = 1
	}
	return value
}

func (v *validator) validateNonZeroConfigField(path string, spec conceptFieldSpec, value configValue) error {
	resolved := v.resolveAlias(spec.Type)
	if resolved.Name == "u32" && !spec.ZeroAllowed && value.int32 == 0 {
		return fmt.Errorf("config field %s is nonzero by default; use u32! if zero is intentional", path)
	}
	return nil
}

func (v *validator) validateLoopAttributes(attributes []ast.Attribute) {
	v.validateLoopHintAttributes("loop", attributes)
}

func (v *validator) validateReductionAttributes(attributes []ast.Attribute) {
	v.validateLoopHintAttributes("reduction", attributes)
}

func (v *validator) validateLoopHintAttributes(subject string, attributes []ast.Attribute) {
	seenUnroll := false
	seenLoop := false
	for _, attr := range attributes {
		if len(attr.Arguments) != 0 {
			v.errorf("attribute [%s] does not take arguments", attr.Name)
			continue
		}
		switch attr.Name {
		case "unroll":
			seenUnroll = true
		case "loop":
			seenLoop = true
		default:
			v.errorf("unknown attribute [%s]", attr.Name)
		}
	}
	if seenUnroll && seenLoop {
		v.errorf("%s cannot declare both [unroll] and [loop]", subject)
	}
}

func (v *validator) validateResourceAttributes(shaderName string, resource ast.ResourceDecl) {
	for _, attr := range resource.Attributes {
		switch attr.Name {
		case "binding":
			if len(attr.Arguments) != 1 {
				v.errorf("resource %s.%s attribute [binding] expects exactly 1 argument", shaderName, resource.Name)
				continue
			}
			lit, ok := attr.Arguments[0].(ast.IntegerLiteral)
			if !ok {
				v.errorf("resource %s.%s attribute [binding] requires a non-negative integer literal", shaderName, resource.Name)
				continue
			}
			value, err := strconv.Atoi(strings.TrimRight(lit.Value, "uU"))
			if err != nil || value < 0 {
				v.errorf("resource %s.%s attribute [binding] requires a non-negative integer literal", shaderName, resource.Name)
			}
		default:
			v.errorf("unknown attribute [%s] on resource %s.%s", attr.Name, shaderName, resource.Name)
		}
	}
}

func (v *validator) validateResourceBindings(shaderName string, resources []ast.ResourceDecl) {
	seen := map[int]string{}
	for _, resource := range resources {
		binding, ok := explicitBinding(resource.Attributes)
		if !ok {
			continue
		}
		if prior, exists := seen[binding]; exists {
			v.errorf("shader %s duplicate explicit binding %d on resources %s and %s", shaderName, binding, prior, resource.Name)
			continue
		}
		seen[binding] = resource.Name
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
	case ast.GuardedReadExpr:
		return rootIdentifier(e.Target)
	default:
		return "", false
	}
}

func isGuardedMemoryReadTarget(expr ast.Expr) bool {
	switch e := expr.(type) {
	case ast.IndexExpr:
		switch target := e.Target.(type) {
		case ast.IdentifierExpr, ast.FieldAccessExpr, ast.IndexExpr:
			_ = target
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isGuardedMemoryWriteTarget(expr ast.Expr, scope map[string]varInfo) bool {
	index, ok := expr.(ast.IndexExpr)
	if !ok {
		return false
	}
	root, ok := rootIdentifier(index)
	if !ok {
		return false
	}
	info, ok := scope[root]
	if !ok {
		return false
	}
	switch info.origin {
	case varResource, varWorkgroup:
		return true
	case varLocal:
		return info.typ.Name == "tile" || info.typ.Name == "matrix_view"
	default:
		return false
	}
}

func isReadonlyMatrixViewIndex(expr ast.Expr, scope map[string]varInfo) bool {
	index, ok := expr.(ast.IndexExpr)
	if !ok {
		return false
	}
	root, ok := rootIdentifier(index)
	if !ok {
		return false
	}
	info, ok := scope[root]
	if !ok {
		return false
	}
	return info.typ.Name == "matrix_view" && info.access != "readwrite"
}

func isDirectIdentifier(expr ast.Expr) bool {
	_, ok := expr.(ast.IdentifierExpr)
	return ok
}

func isRowMajorCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Callee.(ast.IdentifierExpr)
	return ok && id.Name == "row_major"
}

func containsGuardedReadExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case ast.GuardedReadExpr:
		return true
	case ast.FieldAccessExpr:
		return containsGuardedReadExpr(e.Target)
	case ast.IndexExpr:
		return containsGuardedReadExpr(e.Target) || containsGuardedReadExpr(e.Index) || (e.HasSecond && containsGuardedReadExpr(e.Index2))
	case ast.CallExpr:
		if containsGuardedReadExpr(e.Callee) {
			return true
		}
		for _, arg := range e.Arguments {
			if containsGuardedReadExpr(arg) {
				return true
			}
		}
	case ast.BinaryExpr:
		return containsGuardedReadExpr(e.Left) || containsGuardedReadExpr(e.Right)
	case ast.UnaryExpr:
		return containsGuardedReadExpr(e.Operand)
	case ast.ParenExpr:
		return containsGuardedReadExpr(e.Inner)
	case ast.WhenUtilityExpr:
		if containsGuardedReadExpr(e.Else) {
			return true
		}
		for _, c := range e.Cases {
			if containsGuardedReadExpr(c.Value) || containsGuardedReadExpr(c.Condition) || containsGuardedReadExpr(c.Score) {
				return true
			}
		}
	case ast.WithExpr:
		if containsGuardedReadExpr(e.Base) {
			return true
		}
		for _, update := range e.Updates {
			if containsGuardedReadExpr(update.Value) {
				return true
			}
		}
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			if containsGuardedReadExpr(field.Value) {
				return true
			}
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			if containsGuardedReadExpr(field.Value) {
				return true
			}
		}
	case ast.MatchExpr:
		if containsGuardedReadExpr(e.Subject) {
			return true
		}
		for _, arm := range e.Arms {
			if containsGuardedReadExpr(arm.Value) {
				return true
			}
		}
	case ast.ReductionExpr:
		return containsGuardedReadExpr(e.Start) || containsGuardedReadExpr(e.End) || containsGuardedReadExpr(e.Step) || containsGuardedReadExpr(e.Body)
	}
	return false
}

func isFloat(ref ast.TypeRef) bool { return ref.Name == "f32" || ref.Name == "float" }

func isInteger(ref ast.TypeRef) bool {
	return ref.Name == "i32" || ref.Name == "u32"
}

func isComptimeMatchLiteralPattern(expr ast.Expr) bool {
	switch expr.(type) {
	case ast.IntegerLiteral, ast.BoolLiteral:
		return true
	default:
		return false
	}
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

func payloadTypeName(enumName, variantName string) string {
	return enumName + "_" + variantName + "Payload"
}

func joinConceptPath(prefix []string, name string) string {
	if len(prefix) == 0 {
		return name
	}
	return strings.Join(append(append([]string(nil), prefix...), name), ".")
}

func templateFieldPath(expr ast.Expr, root string) (string, bool) {
	switch e := expr.(type) {
	case ast.FieldAccessExpr:
		if id, ok := e.Target.(ast.IdentifierExpr); ok && id.Name == root {
			return e.Field, true
		}
		prefix, ok := templateFieldPath(e.Target, root)
		if !ok {
			return "", false
		}
		return prefix + "." + e.Field, true
	default:
		return "", false
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

func (v *validator) isAllowedEnumPayloadType(ref ast.TypeRef) bool {
	resolved := v.resolveAlias(ref)
	switch resolved.Name {
	case "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4", "uint2", "uint3", "uint4":
		return true
	default:
		return false
	}
}

func isBarrierBuiltin(name string) bool {
	return name == "WorkgroupBarrier" || name == "WorkgroupMemoryBarrier" || name == "WorkgroupMemoryBarrierWithSync"
}
