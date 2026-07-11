package validate

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/consteval"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// Diagnostics is the authoritative validator API. Every ordinary user-source
// failure is a structured compiler diagnostic; Module below is legacy glue.
func Diagnostics(module ast.Module) []diagnostic.Diagnostic {
	v := validator{
		path: module.Source.Path, moduleSpan: module.Span,
		types:          map[string]typeInfo{},
		funcs:          map[string]functionInfo{},
		configs:        map[string]configInfo{},
		concepts:       map[string]ast.ConceptDecl{},
		shaderDecls:    map[string]ast.ShaderDecl{},
		compileAliases: map[string]struct{}{},
	}
	switch filepath.Ext(module.Source.Path) {
	case ".sdslvtest", ".sdslvvalid", ".sdslvinvalid":
		v.testSource = true
	}
	v.seedBuiltins()
	v.collect(module)
	if len(v.diagnostics) == 0 {
		v.validateDecls(module.Decls)
	}
	diagnostic.Sort(v.diagnostics)
	return v.diagnostics
}

// Module is a compatibility adapter for older compiler boundaries.
func Module(module ast.Module) error {
	return diagnostic.Error(Diagnostics(module))
}

type fieldInfo struct {
	access     string
	typ        ast.TypeRef
	attributes []ast.Attribute
	span       source.Span
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
	span         source.Span
}

type functionInfo struct {
	returnType ast.TypeRef
	params     []ast.Parameter
	span       source.Span
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
	varLocal       varOrigin = "local"
	varParam       varOrigin = "param"
	varResource    varOrigin = "resource"
	varWorkgroup   varOrigin = "workgroup"
	varBuiltin     varOrigin = "builtin"
	varComptime    varOrigin = "comptime"
	varFlowBoard   varOrigin = "flow_board"
	varDeriveSelf  varOrigin = "derive_self"
	varDeriveLater varOrigin = "derive_later"
	varTensorFree  varOrigin = "tensor_free_index"
	varTensorRed   varOrigin = "tensor_reduction_index"
)

type varInfo struct {
	typ    ast.TypeRef
	origin varOrigin
	access string
	value  *configValue
}

const maxRegTileElements = 64

type validator struct {
	path             string
	moduleSpan       source.Span
	currentSpan      source.Span
	diagnostics      []diagnostic.Diagnostic
	types            map[string]typeInfo
	funcs            map[string]functionInfo
	configs          map[string]configInfo
	concepts         map[string]ast.ConceptDecl
	shaderDecls      map[string]ast.ShaderDecl
	compileAliases   map[string]struct{}
	resources        map[string]ast.ResourceDecl
	testSource       bool
	currentTestInput *ValidatedTestInput
	tensorAssigns    []ValidatedTensorAssign
}

// Inline source is deliberately not a security boundary. These checks reject the
// interface-shaping constructs that would undermine SDSL-V ownership; DXC owns the
// remaining HLSL grammar and semantics.
func (v *validator) validateForeignBlock(target, raw string, captures []string, scope map[string]varInfo, result ast.TypeRef, expression bool) {
	if target != "HLSL" {
		v.errorf("unsupported foreign shader target %s", target)
		return
	}
	for _, forbidden := range []string{"#", "register(", "cbuffer", "Texture", "Sampler", "RWBuffer", "Buffer<", "[numthreads", "struct ", "namespace "} {
		if strings.Contains(raw, forbidden) {
			v.errorf("inline HLSL contains forbidden interface construct %q", forbidden)
		}
	}
	if expression && !strings.Contains(raw, "return") {
		v.errorf("typed inline HLSL block must contain return")
	}
	seen := map[string]struct{}{}
	for _, name := range captures {
		if _, ok := seen[name]; ok {
			v.errorf("duplicate inline HLSL capture %s", name)
			continue
		}
		seen[name] = struct{}{}
		value, ok := scope[name]
		if !ok {
			v.errorf("inline HLSL capture %s is not in scope", name)
			continue
		}
		if !foreignValueType(v.resolveAlias(value.typ)) {
			v.errorf("inline HLSL cannot capture %s; extract a supported scalar or vector value before the foreign block", typeName(value.typ))
		}
	}
	if expression && !foreignValueType(v.resolveAlias(result)) {
		v.errorf("inline HLSL result type %s is not a supported scalar or vector value", typeName(result))
	}
}

func foreignValueType(t ast.TypeRef) bool {
	switch t.Name {
	case "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4", "uint2", "uint3", "uint4":
		return true
	}
	return false
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
		previous := v.currentSpan
		v.currentSpan = declSpan(decl)
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
				v.errorf("top-level Octomata flow declarations are not supported in SDSL-V M22; use function-local flow blocks")
			} else {
				v.errorf("%s is not implemented in GoOct SDSL-V M0", d.Kind)
			}
		}
		v.currentSpan = previous
	}
}

func (v *validator) addFieldType(name, kind string, fields []ast.Field, label string) {
	collected := map[string]fieldInfo{}
	for _, field := range fields {
		if _, exists := collected[field.Name]; exists {
			v.errorRelated(field.Span, "SDSL-V1509", fmt.Sprintf("duplicate %s field %s.%s", label, name, field.Name), collected[field.Name].span, "first field is here")
		}
		collected[field.Name] = fieldInfo{access: field.Access, typ: field.Type, attributes: field.Attributes, span: field.Span}
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
	if !info.span.Known() {
		info.span = v.currentSpan
	}
	if _, exists := v.types[name]; exists {
		v.errorRelated(info.span, "SDSL-V1509", fmt.Sprintf("duplicate top-level name %s", name), v.types[name].span, "first declaration is here")
		return
	}
	v.types[name] = info
}

func (v *validator) addFunc(name string, fn ast.FunctionDecl) {
	if _, exists := v.funcs[name]; exists {
		v.errorRelated(fn.Span, "SDSL-V1509", fmt.Sprintf("duplicate function %s", name), v.funcs[name].span, "first declaration is here")
		return
	}
	v.funcs[name] = functionInfo{returnType: fn.ReturnType, params: fn.Parameters, span: fn.Span}
}

func (v *validator) validateDecls(decls []ast.Decl) {
	for _, decl := range decls {
		previous := v.currentSpan
		v.currentSpan = declSpan(decl)
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
		v.currentSpan = previous
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
	defer v.scoped(fn.Span)()
	v.validateTestAttributes(fn)
	v.validateType(fn.ReturnType)
	previousTestInput := v.currentTestInput
	if input, ok := testInputForFunction(fn); ok {
		v.currentTestInput = &input
	} else {
		v.currentTestInput = nil
	}
	defer func() { v.currentTestInput = previousTestInput }()
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
	v.validateBlock(fn.Body, fn.ReturnType, scope, shaderName, stage, templateParam, false)
}

func (v *validator) validateStmt(stmt ast.Stmt, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam, insideFlowState bool) {
	defer v.scoped(ast.StmtSpan(stmt))()
	switch s := stmt.(type) {
	case ast.ForeignShaderStmt:
		v.validateForeignBlock(s.TargetLanguage, s.RawSource, s.Captures, scope, ast.TypeRef{}, false)
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
				valueType = v.exprTypeWithExpected(s.Value, scope, shaderName, templateParam, &s.Type, "")
			}
			if !v.compatible(s.Type, valueType) {
				v.errorAt(ast.ExprSpan(s.Value), "SDSL-V1503", "cannot assign %s to local %s of type %s", typeName(valueType), s.Name, typeName(s.Type))
			}
		}
		if _, exists := scope[s.Name]; exists {
			v.errorAt(s.Span, "SDSL-V1509", "duplicate local name %s", s.Name)
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
		if containsDeriveExpr(s.Value) {
			v.errorf("derive is not a compile-time expression in SDSL-V M25")
		}
		v.validateWithPlacement(s.Value, false)
		v.validateGuardedReadPlacement(s.Value, true)
		v.validateMatchPlacement(s.Value, false)
		v.validateReductionPlacement(s.Value, false)
		if containsGuardedReadExpr(s.Value) {
			v.errorf("guarded read is not a compile-time expression in SDSL-V M16a")
		}
		valueType := v.exprTypeWithExpected(s.Value, scope, shaderName, templateParam, nil, "")
		if !v.compatible(s.Type, valueType) {
			v.errorAt(ast.ExprSpan(s.Value), "SDSL-V1503", "cannot assign %s to comptime local %s of type %s", typeName(valueType), s.Name, typeName(s.Type))
		}
		if _, exists := scope[s.Name]; exists {
			v.errorAt(s.Span, "SDSL-V1509", "duplicate local name %s", s.Name)
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
		valueType := v.exprTypeWithExpected(s.Value, scope, shaderName, templateParam, &targetType, "")
		if !isAssignableTarget(s.Target) {
			v.errorf("assignment target is not assignable")
		}
		v.validateImmutableAssignmentTarget(s.Target, scope)
		if targetType.Name == "reg_tile" || valueType.Name == "reg_tile" {
			v.errorf("whole reg_tile assignment is not supported in SDSL-V M15")
		}
		if v.typeKind(targetType) == "board" || v.assignmentTouchesBoardField(s.Target, scope) {
			if !insideFlowState {
				v.errorf("board values are immutable in SDSL-V M21; board field assignment is reserved for flow-bound mutable board state")
			} else if err := v.validateFlowBoardAssignmentTarget(s.Target, scope); err != nil {
				v.errorf("%s", err.Error())
			}
		}
		if !v.compatible(targetType, valueType) {
			v.errorAt(ast.ExprSpan(s.Value), "SDSL-V1503", "assignment type mismatch: %s = %s", typeName(targetType), typeName(valueType))
		}
	case ast.TensorAssignStmt:
		v.validateTensorAssign(s, scope, shaderName, templateParam)
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
		valueType := v.exprTypeWithExpected(s.Value, scope, shaderName, templateParam, &targetType, "")
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
		valueType := v.exprTypeWithExpected(s.Value, scope, shaderName, templateParam, &returnType, "")
		if valueType.Name == "reg_tile" || returnType.Name == "reg_tile" {
			v.errorf("reg_tile return values are not supported in SDSL-V M15")
		}
		if !v.compatible(returnType, valueType) {
			v.errorAt(ast.ExprSpan(s.Value), "SDSL-V1504", "return type mismatch: expected %s, got %s", typeName(returnType), typeName(valueType))
		}
	case ast.ExprStmt:
		v.validateWithPlacement(s.Value, false)
		if call, ok := s.Value.(ast.CallExpr); ok && testAssertName(call.Callee) != "" {
			for _, arg := range call.Arguments {
				v.validateGuardedReadPlacement(arg, true)
			}
		} else {
			v.validateGuardedReadPlacement(s.Value, false)
		}
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
			v.errorAt(ast.ExprSpan(s.Condition), "SDSL-V1505", "if condition must be bool, got %s", typeName(cond))
		}
		v.validateBlock(s.ThenBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
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
			v.validateBlock(c.Body, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
		}
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
		}
	case ast.FlowStmt:
		v.validateFlowStmt(s, returnType, scope, shaderName, stage, templateParam, insideFlowState)
	case ast.ComptimeIfStmt:
		v.validateWithPlacement(s.Condition, false)
		v.validateGuardedReadPlacement(s.Condition, false)
		v.validateMatchPlacement(s.Condition, false)
		v.validateReductionPlacement(s.Condition, false)
		cond := v.exprType(s.Condition, scope, shaderName, templateParam)
		if cond.Name != "bool" {
			v.errorf("comptime if condition must be compile-time bool")
		}
		v.validateBlock(s.ThenBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
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
			v.validateBlock(arm.Body, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
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
			v.validateBlock(c.Body, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
		}
		if s.ElseBody != nil {
			v.validateBlock(*s.ElseBody, returnType, cloneScope(scope), shaderName, stage, templateParam, insideFlowState)
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
		v.validateBlock(s.Body, returnType, loopScope, shaderName, stage, templateParam, insideFlowState)
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
		v.validateBlock(s.Body, returnType, loopScope, shaderName, stage, templateParam, insideFlowState)
	case ast.StaticAssertStmt:
		v.validateGuardedReadPlacement(s.Expr, false)
		typ := v.exprType(s.Expr, scope, shaderName, templateParam)
		if typ.Name != "bool" && typ.Name != "<error>" {
			v.errorf("static assert %s must evaluate to bool", s.Text)
		}
	}
}

func (v *validator) validateFlowStmt(stmt ast.FlowStmt, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam, insideFlowState bool) {
	if insideFlowState {
		v.errorf("nested flow blocks are not supported in SDSL-V M22")
		return
	}
	if len(stmt.States) == 0 {
		v.errorf("flow %s must declare at least one state in SDSL-V M22", stmt.Name)
		return
	}
	flowScope := cloneScope(scope)
	seenBoards := map[string]struct{}{}
	seenStates := map[string]struct{}{}
	for _, board := range stmt.Boards {
		if _, exists := flowScope[board.Name]; exists {
			v.errorf("flow %s: duplicate board instance name %s", stmt.Name, board.Name)
			continue
		}
		if _, exists := seenBoards[board.Name]; exists {
			v.errorf("flow %s: duplicate board instance name %s", stmt.Name, board.Name)
			continue
		}
		if _, exists := seenStates[board.Name]; exists {
			v.errorf("flow %s: board instance %s collides with state name", stmt.Name, board.Name)
			continue
		}
		v.validateType(board.Type)
		if v.typeKind(board.Type) != "board" {
			v.errorf("flow %s board %s must use a board type, got %s", stmt.Name, board.Name, typeName(board.Type))
		}
		v.validateWithPlacement(board.Initializer, true)
		v.validateGuardedReadPlacement(board.Initializer, true)
		v.validateMatchPlacement(board.Initializer, true)
		v.validateReductionPlacement(board.Initializer, true)
		v.validateBarrierUsage(board.Initializer, false, shaderName, stage)
		if _, ok := board.Initializer.(ast.DeriveExpr); ok {
			v.errorf("derive constructs immutable values; flow-owned mutable boards require an explicit board initializer")
		}
		initType := v.exprTypeWithExpected(board.Initializer, flowScope, shaderName, templateParam, &board.Type, "")
		if !v.compatible(board.Type, initType) {
			v.errorf("flow %s board %s initializer expects %s, got %s", stmt.Name, board.Name, typeName(board.Type), typeName(initType))
		}
		seenBoards[board.Name] = struct{}{}
		flowScope[board.Name] = varInfo{typ: board.Type, origin: varFlowBoard}
	}
	for _, state := range stmt.States {
		if _, exists := seenBoards[state.Name]; exists {
			v.errorf("flow %s: state %s collides with board instance name", stmt.Name, state.Name)
			continue
		}
		if _, exists := seenStates[state.Name]; exists {
			continue
		}
		seenStates[state.Name] = struct{}{}
		v.validateBlock(state.Body, returnType, cloneScope(flowScope), shaderName, stage, templateParam, true)
	}
	_, issues := ValidateFlow(stmt)
	for _, issue := range issues {
		v.diagnostics = append(v.diagnostics, diagnostic.Diagnostic{
			Path:     v.path,
			Code:     issue.Code,
			Severity: issue.Severity,
			Message:  issue.Message,
			Span:     issue.Span,
			Related:  issue.Related,
		})
	}
}

func (v *validator) validateBlock(block ast.Block, returnType ast.TypeRef, scope map[string]varInfo, shaderName string, stage string, templateParam *ast.TemplateParam, insideFlowState bool) {
	flowNames := map[string]struct{}{}
	for _, stmt := range block.Statements {
		if flow, ok := stmt.(ast.FlowStmt); ok {
			if _, exists := flowNames[flow.Name]; exists {
				v.errorf("duplicate flow block name %s", flow.Name)
			}
			flowNames[flow.Name] = struct{}{}
		}
		v.validateStmt(stmt, returnType, scope, shaderName, stage, templateParam, insideFlowState)
	}
}

func (v *validator) validateType(ref ast.TypeRef) {
	defer v.scoped(ref.Span)()
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

func (v *validator) checkStructuredLiteralExpr(expr ast.BoardLiteralExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam, currentDeriveField string) ast.TypeRef {
	info, ok := v.types[expr.TypeName]
	if !ok || (info.kind != "board" && info.kind != "record" && info.kind != "stream") {
		v.errorf("unknown structured type %s", expr.TypeName)
		return ast.TypeRef{Name: "<error>"}
	}
	label := info.kind + " literal"
	seen := map[string]struct{}{}
	for _, field := range expr.Fields {
		if _, exists := seen[field.Name]; exists {
			v.errorf("duplicate %s field %s on %s", label, field.Name, expr.TypeName)
			continue
		}
		seen[field.Name] = struct{}{}
		decl, ok := info.fields[field.Name]
		if !ok {
			v.errorf("unknown %s field %s on %s", label, field.Name, expr.TypeName)
			continue
		}
		valueType := v.exprTypeWithExpected(field.Value, scope, shaderName, templateParam, &decl.typ, currentDeriveField)
		if !v.compatible(decl.typ, valueType) {
			v.errorf("%s field %s on %s expects %s, got %s", label, field.Name, expr.TypeName, typeName(decl.typ), typeName(valueType))
		}
	}
	fieldNames := make([]string, 0, len(info.fields))
	for fieldName := range info.fields {
		fieldNames = append(fieldNames, fieldName)
	}
	sortStrings(fieldNames)
	for _, fieldName := range fieldNames {
		if _, exists := seen[fieldName]; !exists {
			v.errorf("missing %s field %s on %s", label, fieldName, expr.TypeName)
		}
	}
	return ast.TypeRef{Name: expr.TypeName}
}

func (v *validator) checkDeriveExpr(expr ast.DeriveExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam, expected *ast.TypeRef) ast.TypeRef {
	if expected == nil {
		v.errorf("derive requires an explicit record or board target type")
		return ast.TypeRef{Name: "<error>"}
	}
	target := v.resolveAlias(*expected)
	info, ok := v.types[target.Name]
	if !ok || (info.kind != "record" && info.kind != "board") {
		v.errorf("derive requires an explicit record or board target type")
		return ast.TypeRef{Name: "<error>"}
	}
	fieldNames := make([]string, 0, len(info.fields))
	for name := range info.fields {
		fieldNames = append(fieldNames, name)
	}
	sortStrings(fieldNames)
	seen := map[string]struct{}{}
	deriveScope := cloneScope(scope)
	for _, field := range expr.Fields {
		if _, exists := deriveScope[field.Name]; exists {
			v.errorf("duplicate local name %s", field.Name)
		}
	}
	for i, field := range expr.Fields {
		if _, exists := seen[field.Name]; exists {
			v.errorf("duplicate derive field `%s`", field.Name)
			continue
		}
		seen[field.Name] = struct{}{}
		decl, ok := info.fields[field.Name]
		if !ok {
			v.errorf("unknown derive field `%s` for `%s`", field.Name, target.Name)
			continue
		}
		fieldScope := cloneScope(deriveScope)
		fieldScope[field.Name] = varInfo{typ: decl.typ, origin: varDeriveSelf}
		for j := i + 1; j < len(expr.Fields); j++ {
			later := expr.Fields[j]
			if _, exists := info.fields[later.Name]; !exists {
				continue
			}
			fieldScope[later.Name] = varInfo{typ: info.fields[later.Name].typ, origin: varDeriveLater}
		}
		valueType := v.exprTypeWithExpected(field.Value, fieldScope, shaderName, templateParam, &decl.typ, field.Name)
		if !v.compatible(decl.typ, valueType) {
			v.errorf("derive field `%s` requires %s, got %s", field.Name, typeName(decl.typ), typeName(valueType))
		}
		deriveScope[field.Name] = varInfo{typ: decl.typ, origin: varLocal}
	}
	for _, fieldName := range fieldNames {
		if _, exists := seen[fieldName]; !exists {
			v.errorf("missing derive field `%s`", fieldName)
		}
	}
	return target
}

func (v *validator) exprType(expr ast.Expr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	return v.exprTypeWithExpected(expr, scope, shaderName, templateParam, nil, "")
}

func (v *validator) exprTypeWithExpected(expr ast.Expr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam, expected *ast.TypeRef, currentDeriveField string) ast.TypeRef {
	defer v.scoped(ast.ExprSpan(expr))()
	switch e := expr.(type) {
	case ast.ForeignShaderExpr:
		v.validateForeignBlock(e.TargetLanguage, e.RawSource, e.Captures, scope, e.ResultType, true)
		return v.resolveAlias(e.ResultType)
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
		if e.Name == "TestInput" {
			v.errorAt(e.Span, "SDSL-V1215", "TestInput is not a first-class value; use TestInput.Length or TestInput.<Kind>[index]")
			return ast.TypeRef{Name: "<error>"}
		}
		if t, ok := scope[e.Name]; ok {
			switch t.origin {
			case varDeriveSelf:
				v.errorf("derive field `%s` cannot reference itself", currentDeriveField)
			case varDeriveLater:
				v.errorf("derive field `%s` references later field `%s`", currentDeriveField, e.Name)
			}
			return v.resolveAlias(t.typ)
		}
		v.errorAt(e.Span, "SDSL-V1501", "unknown identifier %s", e.Name)
		return ast.TypeRef{Name: "<error>"}
	case ast.FieldAccessExpr:
		if root, ok := e.Target.(ast.IdentifierExpr); ok && root.Name == "TestInput" {
			return v.testInputFieldType(e)
		}
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
		target := v.exprTypeWithExpected(e.Target, scope, shaderName, templateParam, nil, currentDeriveField)
		if info, ok := v.types[target.Name]; ok && info.fields != nil {
			if fieldType, ok := info.fields[e.Field]; ok {
				return v.resolveAlias(fieldType.typ)
			}
		}
		v.errorAt(e.Span, "SDSL-V1506", "unknown field %s on %s", e.Field, typeName(target))
		return ast.TypeRef{Name: "<error>"}
	case ast.IndexExpr:
		if member, ok := e.Target.(ast.FieldAccessExpr); ok {
			if root, ok := member.Target.(ast.IdentifierExpr); ok && root.Name == "TestInput" {
				return v.testInputIndexType(member, e, scope, shaderName, templateParam, currentDeriveField)
			}
		}
		target := v.exprTypeWithExpected(e.Target, scope, shaderName, templateParam, nil, currentDeriveField)
		indices := ast.IndexExpressions(e)
		for _, axis := range indices {
			index := v.exprTypeWithExpected(axis, scope, shaderName, templateParam, nil, currentDeriveField)
			if !isInteger(index) {
				v.errorAt(ast.ExprSpan(axis), "SDSL-V1507", "array index must be integer")
			}
		}
		if target.Name == "array" {
			current := target
			for axis, index := range indices {
				if current.Name != "array" || len(current.Args) != 1 {
					v.errorAt(ast.ExprSpan(index), "SDSL-V3217", "index count %d exceeds rank %d of %s", len(indices), axis, typeName(target))
					return ast.TypeRef{Name: "<error>"}
				}
				current = v.resolveAlias(current.Args[0])
			}
			return current
		}
		if len(indices) == 2 {
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
		if len(indices) > 2 {
			v.errorAt(ast.ExprSpan(indices[2]), "SDSL-V3217", "index count %d is unsupported for %s; this value category supports at most rank 2", len(indices), typeName(target))
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
		v.errorf("cannot index non-array type %s", typeName(target))
		return ast.TypeRef{Name: "<error>"}
	case ast.GuardedReadExpr:
		if !isGuardedMemoryReadTarget(e.Target) {
			v.errorf("guarded read target must be an indexed memory expression")
		}
		targetType := v.exprTypeWithExpected(e.Target, scope, shaderName, templateParam, nil, currentDeriveField)
		guardType := v.exprTypeWithExpected(e.Condition, scope, shaderName, templateParam, nil, currentDeriveField)
		fallbackType := v.exprTypeWithExpected(e.Fallback, scope, shaderName, templateParam, nil, currentDeriveField)
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
		left := v.exprTypeWithExpected(e.Left, scope, shaderName, templateParam, nil, currentDeriveField)
		right := v.exprTypeWithExpected(e.Right, scope, shaderName, templateParam, nil, currentDeriveField)
		switch e.Operator {
		case "and", "or":
			if left.Name != "bool" || right.Name != "bool" {
				v.errorAt(ast.ExprSpan(e.Right), "SDSL-V1502", "operator `%s` requires bool operands", e.Operator)
			}
			return ast.TypeRef{Name: "bool"}
		case "==", "!=", "<", "<=", ">", ">=":
			if !v.compatible(left, right) && !(isNumeric(left) && isNumeric(right)) {
				v.errorAt(ast.ExprSpan(e.Right), "SDSL-V1502", "comparison type mismatch: %s %s %s", typeName(left), e.Operator, typeName(right))
			}
			return ast.TypeRef{Name: "bool"}
		default:
			if !isNumeric(left) || !isNumeric(right) {
				v.errorAt(ast.ExprSpan(e.Right), "SDSL-V1502", "arithmetic operands must be numeric")
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
		operand := v.exprTypeWithExpected(e.Operand, scope, shaderName, templateParam, nil, currentDeriveField)
		switch e.Operator {
		case "not":
			if operand.Name != "bool" {
				v.errorAt(ast.ExprSpan(e.Operand), "SDSL-V1502", "operator `not` requires bool operand")
			}
			return ast.TypeRef{Name: "bool"}
		default:
			if !isNumeric(operand) {
				v.errorAt(ast.ExprSpan(e.Operand), "SDSL-V1502", "unary %s requires numeric operand", e.Operator)
			}
		}
		return operand
	case ast.ParenExpr:
		return v.exprTypeWithExpected(e.Inner, scope, shaderName, templateParam, nil, currentDeriveField)
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
			valueType := v.exprTypeWithExpected(field.Value, scope, shaderName, templateParam, nil, currentDeriveField)
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
		return v.checkStructuredLiteralExpr(e, scope, shaderName, templateParam, currentDeriveField)
	case ast.DeriveExpr:
		return v.checkDeriveExpr(e, scope, shaderName, templateParam, expected)
	case ast.MatchExpr:
		subjectType := v.resolveAlias(v.exprTypeWithExpected(e.Subject, scope, shaderName, templateParam, nil, currentDeriveField))
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
			valueType := v.exprTypeWithExpected(arm.Value, armScope, shaderName, templateParam, nil, currentDeriveField)
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
			value := v.exprTypeWithExpected(c.Value, scope, shaderName, templateParam, nil, currentDeriveField)
			cond := v.exprTypeWithExpected(c.Condition, scope, shaderName, templateParam, nil, currentDeriveField)
			score := v.exprTypeWithExpected(c.Score, scope, shaderName, templateParam, nil, currentDeriveField)
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
		elseType := v.exprTypeWithExpected(e.Else, scope, shaderName, templateParam, nil, currentDeriveField)
		if result.Name == "" {
			result = elseType
		}
		if !v.compatible(result, elseType) {
			v.errorf("when utility else value must match case value type")
		}
		return result
	case ast.WithExpr:
		baseType := v.exprTypeWithExpected(e.Base, scope, shaderName, templateParam, nil, currentDeriveField)
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
			valueType := v.exprTypeWithExpected(update.Value, scope, shaderName, templateParam, nil, currentDeriveField)
			if !v.compatible(field.typ, valueType) {
				v.errorf("with field %s expects %s, got %s", update.Name, typeName(field.typ), typeName(valueType))
			}
		}
		return baseType
	case ast.ReductionExpr:
		v.validateReductionAttributes(e.Attributes)
		startType := v.exprTypeWithExpected(e.Start, scope, shaderName, templateParam, nil, currentDeriveField)
		endType := v.exprTypeWithExpected(e.End, scope, shaderName, templateParam, nil, currentDeriveField)
		if !isInteger(startType) || !isInteger(endType) {
			v.errorf("%s reduction bounds must be integer", e.Op)
		}
		if !positiveIntegerLiteral(e.Step) {
			v.errorf("%s reduction step must be a positive integer literal", e.Op)
		}
		bodyScope := cloneScope(scope)
		bodyScope[e.Name] = varInfo{typ: startType, origin: varLocal}
		bodyType := v.exprTypeWithExpected(e.Body, bodyScope, shaderName, templateParam, nil, currentDeriveField)
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
	case ast.TensorReductionExpr:
		v.errorAt(e.Span, "SDSL-V3201", "Sum[...] is only valid in a tensor statement")
		return ast.TypeRef{Name: "<error>"}
	default:
		v.errorf("unsupported expression in GoOct SDSL-V M3")
		return ast.TypeRef{Name: "<error>"}
	}
}

func (v *validator) callType(call ast.CallExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	if _, isAssert := assertCallee(call.Callee); isAssert && testAssertName(call.Callee) == "" {
		v.errorAt(call.Span, "SDSL-V1405", "unknown Assert member")
		return ast.TypeRef{Name: "<error>"}
	}
	if name := testAssertName(call.Callee); name != "" {
		if !v.testSource {
			v.errorAt(call.Span, "SDSL-V1404", "%s is only valid in .sdslvtest functions", name)
			return ast.TypeRef{Name: "<error>"}
		}
		return v.testAssertType(name, call, scope, shaderName, templateParam)
	}
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
					v.errorAt(call.Span, "SDSL-V1508", "function %s expects %d arguments, got %d", id.Name, len(info.params), len(call.Arguments))
					return info.returnType
				}
				for i, arg := range call.Arguments {
					argType := v.exprTypeWithExpected(arg, scope, shaderName, templateParam, &info.params[i].Type, "")
					if !v.compatible(info.params[i].Type, argType) {
						v.errorAt(ast.ExprSpan(arg), "SDSL-V1503", "function %s argument %d expects %s, got %s", id.Name, i+1, typeName(info.params[i].Type), typeName(argType))
					}
				}
				return info.returnType
			}
		}
	}
	v.errorAt(call.Span, "SDSL-V1508", "unsupported or unknown function call")
	return ast.TypeRef{Name: "<error>"}
}

func testAssertName(expr ast.Expr) string {
	member, ok := assertCallee(expr)
	if !ok {
		return ""
	}
	switch member {
	case "True", "False", "Equal", "Near", "NotEqual":
		return "Assert." + member
	}
	return ""
}

func assertCallee(expr ast.Expr) (string, bool) {
	f, ok := expr.(ast.FieldAccessExpr)
	if !ok {
		return "", false
	}
	root, ok := f.Target.(ast.IdentifierExpr)
	if !ok || root.Name != "Assert" {
		return "", false
	}
	return f.Field, true
}
func (v *validator) testAssertType(name string, call ast.CallExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam) ast.TypeRef {
	want := 2
	if name == "Assert.True" || name == "Assert.False" {
		want = 1
	}
	if name == "Assert.Near" {
		want = 3
	}
	if len(call.Arguments) != want {
		v.errorAt(call.Span, "SDSL-V1401", "%s expects %d arguments, got %d", name, want, len(call.Arguments))
		return ast.TypeRef{Name: "void"}
	}
	types := make([]ast.TypeRef, len(call.Arguments))
	for i, arg := range call.Arguments {
		types[i] = v.exprType(arg, scope, shaderName, templateParam)
	}
	if want == 1 && types[0].Name != "bool" {
		v.errorAt(ast.ExprSpan(call.Arguments[0]), "SDSL-V1402", "%s expects a bool", name)
	}
	if (name == "Assert.Equal" || name == "Assert.NotEqual") && !v.compatible(types[0], types[1]) {
		v.errorAt(ast.ExprSpan(call.Arguments[1]), "SDSL-V1402", "%s requires matching operand types", name)
	}
	if name == "Assert.Near" {
		if !isFloat(types[0]) || !isFloat(types[1]) || !isFloat(types[2]) {
			for i, typ := range types {
				if !isFloat(typ) {
					v.errorAt(ast.ExprSpan(call.Arguments[i]), "SDSL-V1403", "Assert.Near requires Float operands")
				}
			}
		}
		if isNegativeFloatConstant(call.Arguments[2]) {
			v.errorAt(ast.ExprSpan(call.Arguments[2]), "SDSL-V1404", "Assert.Near tolerance must be nonnegative")
		}
	}
	return ast.TypeRef{Name: "void"}
}

func isNegativeFloatConstant(expr ast.Expr) bool {
	u, ok := expr.(ast.UnaryExpr)
	if !ok || u.Operator != "-" {
		return false
	}
	_, ok = u.Operand.(ast.FloatLiteral)
	return ok
}

func (v *validator) validateTestAttributes(fn ast.FunctionDecl) {
	if len(fn.Attributes) == 0 {
		return
	}
	fact, theory := []ast.Attribute{}, []ast.Attribute{}
	inline := []ast.Attribute{}
	wg, dispatch := []ast.Attribute{}, []ast.Attribute{}
	testInput := []ast.Attribute{}
	for _, a := range fn.Attributes {
		switch a.Name {
		case "Fact":
			fact = append(fact, a)
		case "Theory":
			theory = append(theory, a)
		case "InlineData":
			inline = append(inline, a)
		case "WorkgroupSize":
			wg = append(wg, a)
		case "DispatchGroups":
			dispatch = append(dispatch, a)
		case "TestInputBool", "TestInputInt", "TestInputUInt", "TestInputFloat":
			testInput = append(testInput, a)
		default:
			v.errorAt(a.Span, "SDSL-V1101", "unsupported function attribute [%s]", a.Name)
		}
	}
	if !v.testSource {
		for _, a := range fn.Attributes {
			v.errorAt(a.Span, "SDSL-V1102", "test attribute [%s] is only valid in .sdslvtest source", a.Name)
		}
		return
	}
	if len(fact) > 1 {
		v.errorRelated(fact[1].Span, "SDSL-V1103", "duplicate [Fact]", fact[0].Span, "first [Fact] is here")
	}
	if len(theory) > 1 {
		v.errorRelated(theory[1].Span, "SDSL-V1103", "duplicate [Theory]", theory[0].Span, "first [Theory] is here")
	}
	if len(fact) > 0 && len(theory) > 0 {
		v.errorRelated(theory[0].Span, "SDSL-V1104", "[Fact] and [Theory] cannot both apply", fact[0].Span, "[Fact] is here")
	}
	if len(wg) > 1 {
		v.errorRelated(wg[1].Span, "SDSL-V1301", "duplicate [WorkgroupSize]", wg[0].Span, "first [WorkgroupSize] is here")
	}
	if len(dispatch) > 1 {
		v.errorRelated(dispatch[1].Span, "SDSL-V1301", "duplicate [DispatchGroups]", dispatch[0].Span, "first [DispatchGroups] is here")
	}
	if len(testInput) > 1 {
		v.errorRelated(testInput[1].Span, "SDSL-V1210", "duplicate TestInput attribute", testInput[0].Span, "first TestInput attribute is here")
		if first, firstOK := testInputAttributeKind(testInput[0].Name); firstOK {
			for _, attr := range testInput[1:] {
				if next, ok := testInputAttributeKind(attr.Name); ok && next != first {
					v.errorRelated(attr.Span, "SDSL-V1211", "conflicting TestInput kinds on one function", testInput[0].Span, "first TestInput attribute is here")
				}
			}
		}
	}
	if len(inline) > 0 && len(theory) == 0 {
		if len(fact) > 0 {
			v.errorAt(inline[0].Span, "SDSL-V1109", "[InlineData] is not allowed on [Fact]")
		} else {
			v.errorAt(inline[0].Span, "SDSL-V1201", "[InlineData] requires [Theory]")
		}
	}
	if len(testInput) > 0 && len(fact) == 0 && len(theory) == 0 {
		v.errorAt(testInput[0].Span, "SDSL-V1212", "TestInput requires [Fact] or [Theory]")
	}
	if len(fact) > 0 && len(fn.Parameters) != 0 {
		v.errorAt(fn.Parameters[0].Span, "SDSL-V1105", "[Fact] must not declare parameters")
	}
	if len(theory) > 0 && len(fn.Parameters) == 0 {
		v.errorAt(theory[0].Span, "SDSL-V1106", "[Theory] must declare parameters")
	}
	if len(theory) > 0 {
		for _, parameter := range fn.Parameters {
			if !testParameterType(v.resolveAlias(parameter.Type)) {
				v.errorAt(parameter.Type.Span, "SDSL-V1205", "[Theory] parameter %s has unsupported type %s", parameter.Name, typeName(parameter.Type))
			}
		}
	}
	if len(theory) > 0 && len(inline) == 0 {
		v.errorAt(theory[0].Span, "SDSL-V1107", "[Theory] requires [InlineData]")
	}
	if (len(fact) > 0 || len(theory) > 0) && fn.ReturnType.Name != "void" {
		v.errorAt(fn.ReturnType.Span, "SDSL-V1108", "test functions must return void")
	}
	if len(theory) > 0 {
		v.validateInlineData(fn, inline)
	}
	for _, a := range wg {
		v.validateLaunchAttribute(a)
	}
	for _, a := range dispatch {
		v.validateLaunchAttribute(a)
	}
	for _, a := range testInput {
		v.validateTestInputAttribute(a)
	}
}

func testInputForFunction(fn ast.FunctionDecl) (ValidatedTestInput, bool) {
	for _, attr := range fn.Attributes {
		if _, ok := testInputAttributeKind(attr.Name); ok {
			input, err := validatedTestInputFromAttribute(attr)
			if err == nil {
				return input, true
			}
			return NoTestInput(), false
		}
	}
	return NoTestInput(), false
}

func testParameterType(typ ast.TypeRef) bool {
	switch typ.Name {
	case "bool", "i32", "u32", "f32", "float":
		return true
	default:
		return false
	}
}

func (v *validator) validateInlineData(fn ast.FunctionDecl, rows []ast.Attribute) {
	for _, row := range rows {
		if len(row.Arguments) != len(fn.Parameters) {
			v.errorAt(row.Span, "SDSL-V1202", "[InlineData] arity mismatch")
			continue
		}
		for i, value := range row.Arguments {
			if _, ok := value.(ast.IntegerLiteral); !ok {
				if _, ok := value.(ast.FloatLiteral); !ok {
					if _, ok := value.(ast.BoolLiteral); !ok {
						v.errorAt(ast.ExprSpan(value), "SDSL-V1204", "[InlineData] values must be literal constants")
						continue
					}
				}
			}
			got := v.exprType(value, nil, "", nil)
			if !v.compatible(fn.Parameters[i].Type, got) {
				v.errorAt(ast.ExprSpan(value), "SDSL-V1203", "[InlineData] does not match parameter %s", fn.Parameters[i].Name)
			}
		}
	}
}

func (v *validator) validateLaunchAttribute(attr ast.Attribute) {
	if len(attr.Arguments) != 3 {
		v.errorAt(attr.Span, "SDSL-V1302", "launch attribute requires three positive integer constants")
		return
	}
	for _, arg := range attr.Arguments {
		lit, ok := arg.(ast.IntegerLiteral)
		if !ok {
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1303", "launch attribute arguments must be integer constants")
			continue
		}
		n, err := strconv.ParseUint(strings.TrimRight(lit.Value, "uU"), 10, 32)
		if err != nil || n == 0 {
			v.errorAt(lit.Span, "SDSL-V1304", "launch attribute arguments must be positive uint32 values")
		}
	}
}

func (v *validator) validateTestInputAttribute(attr ast.Attribute) {
	kind, ok := testInputAttributeKind(attr.Name)
	if !ok {
		return
	}
	for _, arg := range attr.Arguments {
		word, err := encodeTestInputWord(kind, arg)
		_ = word
		if err == nil {
			continue
		}
		switch err.Error() {
		case "bool":
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1213", "TestInputBool values must be constant bool values")
		case "int":
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1214", "TestInputInt values must be constant i32 values")
		case "uint":
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1214", "TestInputUInt values must be constant u32 values")
		case "float":
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1214", "TestInputFloat values must be constant f32 values")
		default:
			v.errorAt(ast.ExprSpan(arg), "SDSL-V1214", "unsupported TestInput value")
		}
	}
}

func (v *validator) testInputFieldType(expr ast.FieldAccessExpr) ast.TypeRef {
	if expr.Field == "Length" {
		if v.currentTestInput == nil || v.currentTestInput.Kind == TestInputKindNone {
			v.errorAt(expr.Span, "SDSL-V1216", "TestInput access requires a declared TestInput attribute on the function")
			return ast.TypeRef{Name: "<error>"}
		}
		return ast.TypeRef{Name: "u32"}
	}
	targetType := v.testInputMemberArrayType(expr)
	if targetType.Name != "<error>" {
		v.errorAt(expr.Span, "SDSL-V1220", "TestInput.%s may only be used with [index]", expr.Field)
	}
	return targetType
}

func (v *validator) testInputMemberArrayType(expr ast.FieldAccessExpr) ast.TypeRef {
	if v.currentTestInput == nil || v.currentTestInput.Kind == TestInputKindNone {
		v.errorAt(expr.Span, "SDSL-V1216", "TestInput access requires a declared TestInput attribute on the function")
		return ast.TypeRef{Name: "<error>"}
	}
	kind, ok := testInputMemberKind(expr.Field)
	if !ok {
		v.errorAt(expr.Span, "SDSL-V1217", "unsupported TestInput member %s", expr.Field)
		return ast.TypeRef{Name: "<error>"}
	}
	if v.currentTestInput.Kind != kind {
		v.errorAt(expr.Span, "SDSL-V1218", "TestInput.%s does not match declared %s payload", expr.Field, v.currentTestInput.Kind)
		return ast.TypeRef{Name: "<error>"}
	}
	return ast.TypeRef{Name: "array", Args: []ast.TypeRef{{Name: scalarTypeNameForTestInputKind(kind)}}}
}

func (v *validator) testInputIndexType(member ast.FieldAccessExpr, indexExpr ast.IndexExpr, scope map[string]varInfo, shaderName string, templateParam *ast.TemplateParam, currentDeriveField string) ast.TypeRef {
	targetType := v.testInputMemberArrayType(member)
	index := v.exprTypeWithExpected(indexExpr.Index, scope, shaderName, templateParam, nil, currentDeriveField)
	if !isInteger(index) {
		v.errorAt(ast.ExprSpan(indexExpr.Index), "SDSL-V1507", "array index must be integer")
	}
	if indexExpr.HasSecond {
		v.errorAt(indexExpr.Span, "SDSL-V1219", "TestInput members support only one index")
		return ast.TypeRef{Name: "<error>"}
	}
	if targetType.Name == "array" && len(targetType.Args) == 1 {
		return targetType.Args[0]
	}
	return ast.TypeRef{Name: "<error>"}
}

func scalarTypeNameForTestInputKind(kind TestInputValueKind) string {
	switch kind {
	case TestInputKindBool:
		return "bool"
	case TestInputKindInt:
		return "i32"
	case TestInputKindUInt:
		return "u32"
	case TestInputKindFloat:
		return "f32"
	default:
		return "<error>"
	}
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
	if root == "TestInput" {
		v.errorf("TestInput is read-only")
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
	case varFlowBoard:
		if isDirectIdentifier(expr) {
			v.errorf("whole-board reassignment is not supported for flow-owned board %s in SDSL-V M23", root)
		}
	}
	if info.typ.Name == "matrix_view" && info.access != "readwrite" && !isDirectIdentifier(expr) {
		v.errorf("cannot assign through readonly matrix_view %s", root)
	}
}

func (v *validator) assignmentTouchesBoardField(expr ast.Expr, scope map[string]varInfo) bool {
	field, ok := expr.(ast.FieldAccessExpr)
	if !ok {
		return false
	}
	root, ok := rootIdentifier(field)
	if !ok {
		return false
	}
	info, ok := scope[root]
	if !ok {
		return false
	}
	return v.typeKind(info.typ) == "board"
}

func (v *validator) validateFlowBoardAssignmentTarget(expr ast.Expr, scope map[string]varInfo) error {
	field, ok := expr.(ast.FieldAccessExpr)
	if !ok {
		root, hasRoot := rootIdentifier(expr)
		if hasRoot {
			if info, exists := scope[root]; exists && info.origin == varFlowBoard {
				return fmt.Errorf("whole-board reassignment is not supported for flow-owned board %s in SDSL-V M23", root)
			}
		}
		return fmt.Errorf("board field assignment in SDSL-V M23 requires a flow-owned board field target of the form `BoardName.field`")
	}
	root, ok := field.Target.(ast.IdentifierExpr)
	if !ok {
		return fmt.Errorf("board field assignment in SDSL-V M23 requires a flow-owned board field target of the form `BoardName.field`")
	}
	info, exists := scope[root.Name]
	if !exists || info.origin != varFlowBoard {
		return fmt.Errorf("only flow-owned board instances may be mutated in SDSL-V M23; %s is not a mutable flow board", root.Name)
	}
	return nil
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
	case ast.DeriveExpr:
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
	case ast.DeriveExpr:
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
	case ast.DeriveExpr:
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
	case ast.DeriveExpr:
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
	case ast.DeriveExpr:
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
	span := v.currentSpan
	if !span.Known() {
		// This is reachable only before a source node is entered (for example a
		// malformed synthetic recovery declaration); ordinary validation always
		// establishes currentSpan from compiler-owned AST data.
		span = v.moduleSpan
	}
	v.errorAt(span, "SDSL-V1000", format, args...)
}

func (v *validator) scoped(span source.Span) func() {
	previous := v.currentSpan
	if span.Known() {
		v.currentSpan = span
	}
	return func() { v.currentSpan = previous }
}

func declSpan(decl ast.Decl) source.Span {
	switch d := decl.(type) {
	case ast.TypeAliasDecl:
		return d.Span
	case ast.RecordDecl:
		return d.Span
	case ast.BoardDecl:
		return d.Span
	case ast.StreamDecl:
		return d.Span
	case ast.ConceptDecl:
		return d.Span
	case ast.ConfigDecl:
		return d.Span
	case ast.EnumDecl:
		return d.Span
	case ast.ShaderDecl:
		return d.Span
	case ast.CompileDecl:
		return d.Span
	case ast.FunctionDecl:
		return d.Span
	default:
		return source.Span{}
	}
}

func (v *validator) errorAt(span source.Span, code, format string, args ...any) {
	v.diagnostics = append(v.diagnostics, diagnostic.Diagnostic{
		Path: v.path, Code: code, Severity: diagnostic.SeverityError,
		Message: fmt.Sprintf(format, args...), Span: span,
	})
}

func (v *validator) errorRelated(span source.Span, code, message string, related source.Span, relatedMessage string) {
	v.diagnostics = append(v.diagnostics, diagnostic.Diagnostic{
		Path: v.path, Code: code, Severity: diagnostic.SeverityError, Message: message, Span: span,
		Related: []diagnostic.Related{{Message: relatedMessage, Span: related}},
	})
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
	case ast.DeriveExpr:
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

func containsDeriveExpr(expr ast.Expr) bool {
	switch e := expr.(type) {
	case ast.DeriveExpr:
		return true
	case ast.FieldAccessExpr:
		return containsDeriveExpr(e.Target)
	case ast.IndexExpr:
		return containsDeriveExpr(e.Target) || containsDeriveExpr(e.Index) || (e.HasSecond && containsDeriveExpr(e.Index2))
	case ast.GuardedReadExpr:
		return containsDeriveExpr(e.Target) || containsDeriveExpr(e.Condition) || containsDeriveExpr(e.Fallback)
	case ast.CallExpr:
		if containsDeriveExpr(e.Callee) {
			return true
		}
		for _, arg := range e.Arguments {
			if containsDeriveExpr(arg) {
				return true
			}
		}
	case ast.BinaryExpr:
		return containsDeriveExpr(e.Left) || containsDeriveExpr(e.Right)
	case ast.UnaryExpr:
		return containsDeriveExpr(e.Operand)
	case ast.ParenExpr:
		return containsDeriveExpr(e.Inner)
	case ast.WhenUtilityExpr:
		if containsDeriveExpr(e.Else) {
			return true
		}
		for _, c := range e.Cases {
			if containsDeriveExpr(c.Value) || containsDeriveExpr(c.Condition) || containsDeriveExpr(c.Score) {
				return true
			}
		}
	case ast.WithExpr:
		if containsDeriveExpr(e.Base) {
			return true
		}
		for _, update := range e.Updates {
			if containsDeriveExpr(update.Value) {
				return true
			}
		}
	case ast.EnumConstructExpr:
		for _, field := range e.Fields {
			if containsDeriveExpr(field.Value) {
				return true
			}
		}
	case ast.BoardLiteralExpr:
		for _, field := range e.Fields {
			if containsDeriveExpr(field.Value) {
				return true
			}
		}
	case ast.MatchExpr:
		if containsDeriveExpr(e.Subject) {
			return true
		}
		for _, arm := range e.Arms {
			if containsDeriveExpr(arm.Value) {
				return true
			}
		}
	case ast.ReductionExpr:
		return containsDeriveExpr(e.Start) || containsDeriveExpr(e.End) || containsDeriveExpr(e.Step) || containsDeriveExpr(e.Body)
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
