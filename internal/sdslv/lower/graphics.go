package lower

import (
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/vdmir"
)

type streamUse struct {
	pixelOutput bool
}

func materialTypeName(shader string) string { return shader + "_Material" }

func (l *lowering) collectStreamUse(method ast.FunctionDecl) {
	for _, param := range method.Parameters {
		use := l.streamUses[param.Type.Name]
		l.streamUses[param.Type.Name] = use
	}
	use := l.streamUses[method.ReturnType.Name]
	use.pixelOutput = use.pixelOutput || method.Stage == "pixel"
	l.streamUses[method.ReturnType.Name] = use
}

func (l *lowering) streamRole(name string) vdmir.StreamRole {
	if name == "ComputeThread" {
		return vdmir.StreamRoleBuiltin
	}
	stream, ok := l.streams[name]
	if !ok {
		return ""
	}
	for _, field := range stream.Fields {
		if field.Access != "" {
			return vdmir.StreamRoleResource
		}
	}
	for _, field := range stream.Fields {
		if _, ok := lowerFieldAttribute(field, "builtin"); ok {
			return vdmir.StreamRoleBuiltin
		}
	}
	return vdmir.StreamRoleStageValue
}

func (l *lowering) streamAssignments(name string, fields []ast.Field) (map[string]int, map[string]int) {
	locations, targets := map[string]int{}, map[string]int{}
	pixelOutput := l.streamUses[name].pixelOutput
	used := map[int]bool{}
	attribute := "location"
	if pixelOutput {
		attribute = "target"
	}
	for _, field := range fields {
		if attr, ok := lowerFieldAttribute(field, attribute); ok {
			if value, ok := lowerAttributeInt(attr); ok {
				used[value] = true
			}
		}
	}
	next := 0
	for _, field := range fields {
		if _, ok := lowerFieldAttribute(field, "builtin"); ok || l.resolveAlias(field.Type).Space == "clip.position" {
			continue
		}
		value := -1
		if attr, ok := lowerFieldAttribute(field, attribute); ok {
			value, _ = lowerAttributeInt(attr)
		} else {
			for used[next] {
				next++
			}
			value, used[next] = next, true
			next++
		}
		if pixelOutput {
			targets[field.Name] = value
		} else {
			locations[field.Name] = value
		}
	}
	return locations, targets
}

func lowerBuiltinAttribute(field ast.Field) (string, string) {
	attr, ok := lowerFieldAttribute(field, "builtin")
	if !ok || len(attr.Arguments) != 1 {
		return "", ""
	}
	id, ok := attr.Arguments[0].(ast.IdentifierExpr)
	if !ok {
		return "", ""
	}
	switch id.Name {
	case "vertex_id":
		return id.Name, "SV_VertexID"
	case "instance_id":
		return id.Name, "SV_InstanceID"
	case "position":
		return id.Name, "SV_Position"
	case "front_face":
		return id.Name, "SV_IsFrontFace"
	default:
		return id.Name, ""
	}
}

func lowerInterpolation(field ast.Field, typ ast.TypeRef) string {
	if attr, ok := lowerFieldAttribute(field, "interpolation"); ok && len(attr.Arguments) == 1 {
		if id, ok := attr.Arguments[0].(ast.IdentifierExpr); ok {
			return id.Name
		}
	}
	switch typ.Name {
	case "bool", "i32", "u32", "uint2", "uint3", "uint4":
		return "flat"
	default:
		return "linear"
	}
}

func (l *lowering) lowerResource(resource ast.ResourceDecl, bundle string, binding vdmir.Binding) vdmir.Resource {
	typ := l.lowerTypeRef(resource.Type)
	out := vdmir.Resource{
		Provenance: l.provenance,
		BundleName: bundle,
		Name:       resource.Name,
		Type:       typ,
		Access:     lowerResourceAccess(resource.Access),
		Binding:    binding,
	}
	switch resource.Type.Name {
	case "texture2d":
		out.Kind = vdmir.ResourceTexture2D
		l.requireSimpleCapability(vdmir.CapabilitySampledTexture2D)
	case "sampler":
		out.Kind = vdmir.ResourceSampler
	case "uniform":
		out.Kind = vdmir.ResourceUniform
	case "acceleration_structure":
		out.Kind = vdmir.ResourceAccelerationStructure
		l.requireRayQuery()
	default:
		out.Kind = vdmir.ResourceStorageBuffer
	}
	if typ.Element != nil {
		out.ElementType = *typ.Element
	}
	return out
}

func (l *lowering) lowerMaterial(shader ast.ShaderDecl, resources []ast.ResourceDecl) vdmir.Material {
	l.requireSimpleCapability(vdmir.CapabilityUniformMaterial)
	all := append([]ast.ResourceDecl(nil), resources...)
	all = append(all, ast.ResourceDecl{Name: "Material"})
	binding := resolveResourceBinding(all, len(all)-1)
	out := vdmir.Material{
		Provenance: l.provenance,
		ShaderName: shader.Name,
		TypeName:   materialTypeName(shader.Name),
		Binding:    binding,
	}
	offset := uint32(0)
	for _, field := range shader.Material.Fields {
		typ := l.lowerTypeRef(field.Type)
		size, alignment := materialTypeLayout(typ)
		offset = alignUp(offset, alignment)
		if offset/16 != (offset+size-1)/16 {
			offset = alignUp(offset, 16)
		}
		out.Fields = append(out.Fields, vdmir.MaterialField{Name: field.Name, Type: typ, Offset: offset, Size: size, Alignment: alignment})
		offset += size
	}
	out.Size = alignUp(offset, 16)
	return out
}

func materialTypeLayout(typ vdmir.Type) (uint32, uint32) {
	switch typ.Kind {
	case vdmir.TypeFloat4, vdmir.TypeUint4:
		return 16, 16
	case vdmir.TypeFloat3, vdmir.TypeUint3:
		return 12, 16
	case vdmir.TypeFloat2, vdmir.TypeUint2:
		return 8, 8
	default:
		return 4, 4
	}
}

func alignUp(value, alignment uint32) uint32 {
	return (value + alignment - 1) &^ (alignment - 1)
}

func (l *lowering) lowerGraphicsEntryPoint(shader ast.ShaderDecl, fn ast.FunctionDecl) vdmir.GraphicsEntryPoint {
	l.requireSimpleCapability(vdmir.CapabilityGraphicsVertexPixel)
	entry := vdmir.GraphicsEntryPoint{
		Provenance:   l.provenance,
		ProgramName:  shader.Name,
		FunctionName: fn.Name,
		EmittedName:  shader.Name + "_" + fn.Name,
		Stage:        vdmir.ShaderStage(fn.Stage),
		ReturnType:   l.lowerTypeRef(fn.ReturnType),
	}
	for _, param := range fn.Parameters {
		role := l.streamRole(param.Type.Name)
		entry.Params = append(entry.Params, vdmir.GraphicsParameter{Name: param.Name, Type: l.lowerTypeRef(param.Type), Role: role, Emitted: role != vdmir.StreamRoleResource})
		stream := l.lowerStream(param.Type.Name, l.streams[param.Type.Name].Fields)
		for _, field := range stream.Fields {
			if field.Builtin != "" {
				entry.Builtins = append(entry.Builtins, vdmir.BuiltinUse{Name: field.Name, Builtin: field.Builtin, Type: field.Type, Semantic: field.Semantic})
				continue
			}
			if field.HasLocation {
				entry.Inputs = append(entry.Inputs, vdmir.InterfaceField{Stream: stream.Name, Name: field.Name, Type: field.Type, Location: field.Location, HasLocation: true, Interpolation: field.Interpolation})
			}
		}
	}
	if streamDecl, ok := l.streams[fn.ReturnType.Name]; ok {
		stream := l.lowerStream(fn.ReturnType.Name, streamDecl.Fields)
		for _, field := range stream.Fields {
			if field.Builtin != "" {
				entry.Builtins = append(entry.Builtins, vdmir.BuiltinUse{Name: field.Name, Builtin: field.Builtin, Type: field.Type, Semantic: field.Semantic})
			}
			if field.HasLocation {
				entry.Outputs = append(entry.Outputs, vdmir.InterfaceField{Stream: stream.Name, Name: field.Name, Type: field.Type, Location: field.Location, HasLocation: true, Interpolation: field.Interpolation})
			}
			if field.HasTarget {
				entry.Targets = append(entry.Targets, vdmir.PixelTarget{Name: field.Name, Target: field.Target, Type: field.Type})
			}
		}
	} else if fn.Stage == "pixel" {
		entry.Targets = append(entry.Targets, vdmir.PixelTarget{Name: "$return", Target: 0, Type: entry.ReturnType})
	}
	return entry
}

func (l *lowering) requireSimpleCapability(kind string) {
	l.requirements[kind] = vdmir.CapabilityRequirement{Kind: kind}
}

func (l *lowering) lowerGraphicsProgram(shader ast.ShaderDecl) []vdmir.GraphicsProgram {
	if shader.Template != nil {
		return nil
	}
	program := vdmir.GraphicsProgram{Provenance: l.provenance, Name: shader.Name}
	for _, method := range shader.Methods {
		switch method.Stage {
		case "vertex":
			program.Vertex = shader.Name + "_" + method.Name
		case "pixel":
			program.Pixel = shader.Name + "_" + method.Name
		}
	}
	if program.Vertex == "" || program.Pixel == "" {
		return nil
	}
	return []vdmir.GraphicsProgram{program}
}

func lowerFieldAttribute(field ast.Field, name string) (ast.Attribute, bool) {
	for _, attr := range field.Attributes {
		if attr.Name == name {
			return attr, true
		}
	}
	return ast.Attribute{}, false
}

func lowerAttributeInt(attr ast.Attribute) (int, bool) {
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
