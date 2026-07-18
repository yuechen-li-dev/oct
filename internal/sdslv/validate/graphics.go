package validate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const (
	streamRoleStageValue = "stage-value"
	streamRoleResource   = "resource"
	streamRoleBuiltin    = "builtin"
)

type graphicsInterfaceField struct {
	name          string
	typ           ast.TypeRef
	location      int
	interpolation string
	span          source.Span
}

func materialTypeName(shader string) string { return shader + "_Material" }

func (v *validator) validateSpaceAlias(decl ast.TypeAliasDecl) {
	if decl.Type.Space == "" {
		return
	}
	base := decl.Type
	base.Space = ""
	resolved := v.resolveAlias(base)
	switch resolved.Name {
	case "float2", "float3", "float4":
	default:
		v.errorAt(decl.Type.AnnotationSpan, "SDSL-V4120", "semantic-space alias %s must use float2, float3, or float4", decl.Name)
	}
	parts := strings.Split(decl.Type.Space, ".")
	if len(parts) < 2 {
		v.errorAt(decl.Type.SpaceSpan, "SDSL-V4120", "semantic space %s must be a dotted nominal name", decl.Type.Space)
		return
	}
	if oneOf(parts[0], "object", "world", "view", "clip") {
		if len(parts) != 2 || !oneOf(parts[1], "position", "normal", "vector") {
			v.errorAt(decl.Type.SpaceSpan, "SDSL-V4120", "unsupported graphics coordinate space %s; use object|world|view|clip with position|normal|vector", decl.Type.Space)
			return
		}
		if parts[0] == "clip" && parts[1] != "position" {
			v.errorAt(decl.Type.SpaceSpan, "SDSL-V4120", "clip space supports only clip.position in canonical SDSL-V")
		}
	}
}

func (v *validator) validateStreamRole(owner string, fields []ast.Field) {
	hasResource, hasBuiltin, hasStage := false, false, false
	locations := map[int]ast.Field{}
	targets := map[int]ast.Field{}
	builtins := map[string]ast.Field{}
	for _, field := range fields {
		if field.Access != "" {
			hasResource = true
		}
		for _, attr := range field.Attributes {
			switch attr.Name {
			case "binding":
				if field.Access == "" {
					v.errorAt(attr.Span, "SDSL-V4102", "stream %s field %s uses binding without resource access", owner, field.Name)
				}
				hasResource = true
			case "builtin":
				name, ok := attributeName(attr)
				if !ok {
					v.errorAt(attr.Span, "SDSL-V4109", "builtin attribute requires one canonical builtin name")
					continue
				}
				if prior, exists := builtins[name]; exists {
					v.errorRelated(field.Span, "SDSL-V4110", fmt.Sprintf("duplicate builtin %s in stream %s", name, owner), prior.Span, "first builtin is here")
				}
				builtins[name] = field
				hasBuiltin = true
			case "location":
				value, ok := attributeNonnegativeInt(attr)
				if !ok {
					v.errorAt(attr.Span, "SDSL-V4105", "location attribute requires one non-negative integer")
					continue
				}
				if prior, exists := locations[value]; exists {
					v.errorRelated(field.Span, "SDSL-V4105", fmt.Sprintf("stream %s location %d collides", owner, value), prior.Span, "first location is here")
				}
				locations[value] = field
				hasStage = true
			case "target":
				value, ok := attributeNonnegativeInt(attr)
				if !ok {
					v.errorAt(attr.Span, "SDSL-V4108", "target attribute requires one non-negative integer")
					continue
				}
				if prior, exists := targets[value]; exists {
					v.errorRelated(field.Span, "SDSL-V4108", fmt.Sprintf("stream %s target %d collides", owner, value), prior.Span, "first target is here")
				}
				targets[value] = field
				hasStage = true
			case "interpolation":
				name, ok := attributeName(attr)
				if !ok || !oneOf(name, "linear", "flat", "noperspective") {
					v.errorAt(attr.Span, "SDSL-V4105", "interpolation must be linear, flat, or noperspective")
				}
				hasStage = true
			default:
				v.errorAt(attr.Span, "SDSL-V4103", "unknown stream field attribute [%s]", attr.Name)
			}
		}
		if v.resolveAlias(field.Type).Space == "clip.position" {
			hasStage = true
		}
	}
	if (hasResource && (hasBuiltin || hasStage)) || (hasBuiltin && hasStage) {
		v.errorAt(v.types[owner].span, "SDSL-V4102", "stream %s mixes resource, builtin, or stage-value roles", owner)
	}
}

func (v *validator) streamRole(name string) string {
	if name == "ComputeThread" {
		return streamRoleBuiltin
	}
	stream, ok := v.streams[name]
	if !ok {
		return ""
	}
	hasResource, hasBuiltin := false, false
	for _, field := range stream.Fields {
		if field.Access != "" {
			hasResource = true
		}
		if _, ok := fieldAttribute(field, "builtin"); ok {
			hasBuiltin = true
		}
	}
	if hasResource {
		return streamRoleResource
	}
	if hasBuiltin {
		return streamRoleBuiltin
	}
	return streamRoleStageValue
}

func (v *validator) validateGraphicsSignature(shader ast.ShaderDecl, method ast.FunctionDecl) {
	for _, param := range method.Parameters {
		if v.typeKind(param.Type) != "stream" {
			v.errorAt(param.Span, "SDSL-V4104", "%s entry parameter %s must be a stage-value, builtin, or resource stream", method.Stage, param.Name)
			continue
		}
		role := v.streamRole(param.Type.Name)
		switch role {
		case streamRoleBuiltin:
			v.validateBuiltinStream(param.Type.Name, method.Stage)
		case streamRoleResource:
			if shader.ResourceBundleName != param.Type.Name {
				v.errorAt(param.Span, "SDSL-V4102", "resource-stream parameter %s must match shader resources %s", param.Type.Name, shader.ResourceBundleName)
			}
		case streamRoleStageValue:
			v.stageInterfaceFields(param.Type.Name)
		default:
			v.errorAt(param.Span, "SDSL-V4102", "stream %s has ambiguous role", param.Type.Name)
		}
	}
	resolvedReturn := v.resolveAlias(method.ReturnType)
	switch method.Stage {
	case "vertex":
		if v.typeKind(method.ReturnType) != "stream" || v.streamRole(method.ReturnType.Name) != streamRoleStageValue {
			v.errorAt(method.ReturnType.Span, "SDSL-V4106", "vertex entry %s.%s must return a stage-value stream", shader.Name, method.Name)
			return
		}
		v.validateVertexPosition(method.ReturnType.Name, method.ReturnType.Span)
	case "pixel":
		if v.typeKind(method.ReturnType) == "stream" {
			if v.streamRole(method.ReturnType.Name) != streamRoleStageValue {
				v.errorAt(method.ReturnType.Span, "SDSL-V4108", "pixel entry output must be a stage-value stream")
				return
			}
			v.pixelTargets(method.ReturnType.Name)
			return
		}
		if resolvedReturn.Space != "" || !oneOf(resolvedReturn.Name, "f32", "float", "float2", "float3", "float4") {
			v.errorAt(method.ReturnType.Span, "SDSL-V4108", "pixel entry must return a plain float scalar/vector or pixel-output stream")
		}
	}
}

func (v *validator) validateVertexPosition(streamName string, span source.Span) {
	count := 0
	for _, field := range v.streams[streamName].Fields {
		if v.resolveAlias(field.Type).Space == "clip.position" {
			count++
		}
	}
	if count == 0 {
		v.errorAt(span, "SDSL-V4106", "vertex output stream %s requires exactly one @space(clip.position) field", streamName)
	} else if count > 1 {
		v.errorAt(span, "SDSL-V4107", "vertex output stream %s has multiple @space(clip.position) fields", streamName)
	}
}

func (v *validator) validateBuiltinStream(name, stage string) {
	stream := v.streams[name]
	if name == "ComputeThread" {
		if stage != "compute" {
			v.errorAt(stream.Span, "SDSL-V4109", "ComputeThread is valid only in compute stages")
		}
		return
	}
	for _, field := range stream.Fields {
		attr, ok := fieldAttribute(field, "builtin")
		if !ok {
			v.errorAt(field.Span, "SDSL-V4102", "builtin stream %s field %s requires [builtin(...)]", name, field.Name)
			continue
		}
		builtin, ok := attributeName(attr)
		if !ok {
			continue
		}
		got := v.resolveAlias(field.Type)
		want, allowed := "", false
		switch builtin {
		case "vertex_id", "instance_id":
			want, allowed = "u32", stage == "vertex"
		case "position":
			want, allowed = "float4", stage == "pixel"
		case "front_face":
			want, allowed = "bool", stage == "pixel"
		default:
			v.errorAt(attr.Span, "SDSL-V4109", "unsupported canonical builtin %s", builtin)
			continue
		}
		if !allowed {
			v.errorAt(attr.Span, "SDSL-V4109", "builtin %s is not valid in %s stage", builtin, stage)
		}
		if got.Name != want || (builtin != "position" && got.Space != "") {
			v.errorAt(field.Type.Span, "SDSL-V4110", "builtin %s requires %s, got %s", builtin, want, typeName(got))
		}
	}
}

func (v *validator) stageInterfaceFields(name string) []graphicsInterfaceField {
	stream := v.streams[name]
	used := map[int]bool{}
	for _, field := range stream.Fields {
		if attr, ok := fieldAttribute(field, "location"); ok {
			if n, ok := attributeNonnegativeInt(attr); ok {
				used[n] = true
			}
		}
	}
	next := 0
	var out []graphicsInterfaceField
	for _, field := range stream.Fields {
		if _, ok := fieldAttribute(field, "builtin"); ok || v.resolveAlias(field.Type).Space == "clip.position" {
			continue
		}
		if _, ok := fieldAttribute(field, "target"); ok {
			continue
		}
		location := -1
		if attr, ok := fieldAttribute(field, "location"); ok {
			location, _ = attributeNonnegativeInt(attr)
		} else {
			for used[next] {
				next++
			}
			location, used[next] = next, true
			next++
		}
		interpolation := defaultInterpolation(v.resolveAlias(field.Type))
		if attr, ok := fieldAttribute(field, "interpolation"); ok {
			if value, valid := attributeName(attr); valid {
				interpolation = value
			}
		}
		out = append(out, graphicsInterfaceField{name: field.Name, typ: field.Type, location: location, interpolation: interpolation, span: field.Span})
	}
	return out
}

func (v *validator) pixelTargets(name string) []graphicsInterfaceField {
	stream := v.streams[name]
	used := map[int]bool{}
	for _, field := range stream.Fields {
		if attr, ok := fieldAttribute(field, "target"); ok {
			if n, ok := attributeNonnegativeInt(attr); ok {
				used[n] = true
			}
		}
	}
	next := 0
	var out []graphicsInterfaceField
	for _, field := range stream.Fields {
		if _, ok := fieldAttribute(field, "builtin"); ok || v.resolveAlias(field.Type).Space == "clip.position" {
			v.errorAt(field.Span, "SDSL-V4108", "pixel output stream %s cannot contain builtin or clip-position field %s", name, field.Name)
			continue
		}
		target := -1
		if attr, ok := fieldAttribute(field, "target"); ok {
			target, _ = attributeNonnegativeInt(attr)
		} else {
			for used[next] {
				next++
			}
			target, used[next] = next, true
			next++
		}
		got := v.resolveAlias(field.Type)
		if got.Space != "" || !oneOf(got.Name, "f32", "float", "float2", "float3", "float4") {
			v.errorAt(field.Type.Span, "SDSL-V4108", "pixel target %s.%s must be a plain float scalar/vector", name, field.Name)
		}
		out = append(out, graphicsInterfaceField{name: field.Name, typ: field.Type, location: target, span: field.Span})
	}
	return out
}

func (v *validator) validateGraphicsPrograms(decls []ast.Decl) {
	materialized := map[string]bool{}
	for _, decl := range decls {
		if compile, ok := decl.(ast.CompileDecl); ok {
			materialized[compile.ShaderName] = true
		}
	}
	for _, decl := range decls {
		shader, ok := decl.(ast.ShaderDecl)
		if !ok {
			continue
		}
		if shader.Template != nil && !materialized[shader.Name] {
			for _, method := range shader.Methods {
				if method.Stage == "vertex" || method.Stage == "pixel" {
					v.errorAt(shader.Span, "SDSL-V4113", "generic graphics template %s has no compile materialization and therefore cannot own an entry point", shader.Name)
					break
				}
			}
		}
		var vertex, pixel *ast.FunctionDecl
		for i := range shader.Methods {
			method := &shader.Methods[i]
			switch method.Stage {
			case "vertex":
				if vertex != nil {
					v.errorAt(method.Span, "SDSL-V4104", "graphics program %s has more than one vertex entry", shader.Name)
				}
				vertex = method
			case "pixel":
				if pixel != nil {
					v.errorAt(method.Span, "SDSL-V4104", "graphics program %s has more than one pixel entry", shader.Name)
				}
				pixel = method
			}
		}
		if vertex == nil || pixel == nil || v.typeKind(vertex.ReturnType) != "stream" {
			continue
		}
		vertexFields := v.stageInterfaceFields(vertex.ReturnType.Name)
		var pixelFields []graphicsInterfaceField
		for _, param := range pixel.Parameters {
			if v.typeKind(param.Type) == "stream" && v.streamRole(param.Type.Name) == streamRoleStageValue {
				pixelFields = append(pixelFields, v.stageInterfaceFields(param.Type.Name)...)
			}
		}
		v.validatePairedInterface(shader.Name, vertexFields, pixelFields)
	}
}

func (v *validator) validatePairedInterface(program string, vertex, pixel []graphicsInterfaceField) {
	vertexByLocation := map[int]graphicsInterfaceField{}
	pixelByLocation := map[int]graphicsInterfaceField{}
	for _, field := range vertex {
		vertexByLocation[field.location] = field
	}
	for _, field := range pixel {
		pixelByLocation[field.location] = field
	}
	for location, out := range vertexByLocation {
		in, ok := pixelByLocation[location]
		if !ok {
			v.errorAt(out.span, "SDSL-V4111", "graphics program %s vertex output location %d is missing from pixel input", program, location)
			continue
		}
		if !v.compatible(out.typ, in.typ) || out.interpolation != in.interpolation {
			v.errorRelated(in.span, "SDSL-V4111", fmt.Sprintf("graphics program %s varying location %d disagrees in type or interpolation", program, location), out.span, "vertex output is here")
		}
	}
	for location, in := range pixelByLocation {
		if _, ok := vertexByLocation[location]; !ok {
			v.errorAt(in.span, "SDSL-V4111", "graphics program %s pixel input location %d is missing from vertex output", program, location)
		}
	}
}

func (v *validator) validateMaterial(shader ast.ShaderDecl) {
	if shader.Material == nil {
		return
	}
	hasGraphics := false
	for _, method := range shader.Methods {
		hasGraphics = hasGraphics || method.Stage == "vertex" || method.Stage == "pixel"
	}
	if !hasGraphics {
		v.errorAt(shader.Material.Span, "SDSL-V4114", "material blocks are valid only on graphics programs")
	}
	if len(shader.Material.Fields) == 0 {
		v.errorAt(shader.Material.Span, "SDSL-V4114", "material block must contain at least one field")
	}
	for _, field := range shader.Material.Fields {
		if field.Access != "" || len(field.Attributes) != 0 || !isMaterialFieldType(v.resolveAlias(field.Type)) {
			v.errorAt(field.Span, "SDSL-V4114", "material field %s must be an unannotated bool/i32/u32/f32/vector value", field.Name)
		}
	}
	for _, resource := range v.resolveShaderResources(shader) {
		if resource.Name == "Material" {
			v.errorAt(resource.Span, "SDSL-V4114", "resource name Material is reserved by material lowering")
		}
	}
}

func fieldAttribute(field ast.Field, name string) (ast.Attribute, bool) {
	for _, attr := range field.Attributes {
		if attr.Name == name {
			return attr, true
		}
	}
	return ast.Attribute{}, false
}

func attributeName(attr ast.Attribute) (string, bool) {
	if len(attr.Arguments) != 1 {
		return "", false
	}
	id, ok := attr.Arguments[0].(ast.IdentifierExpr)
	return id.Name, ok
}

func attributeNonnegativeInt(attr ast.Attribute) (int, bool) {
	if len(attr.Arguments) != 1 {
		return 0, false
	}
	lit, ok := attr.Arguments[0].(ast.IntegerLiteral)
	if !ok {
		return 0, false
	}
	value, err := strconv.Atoi(strings.TrimRight(lit.Value, "uU"))
	return value, err == nil && value >= 0
}

func defaultInterpolation(ref ast.TypeRef) string {
	switch ref.Name {
	case "bool", "i32", "u32", "uint2", "uint3", "uint4":
		return "flat"
	default:
		return "linear"
	}
}

func isTextureComponentType(ref ast.TypeRef) bool {
	return ref.Space == "" && oneOf(ref.Name, "f32", "float", "float2", "float3", "float4")
}

func isMaterialFieldType(ref ast.TypeRef) bool {
	return ref.Space == "" && oneOf(ref.Name, "bool", "i32", "u32", "f32", "float", "float2", "float3", "float4", "uint2", "uint3", "uint4")
}

func oneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
