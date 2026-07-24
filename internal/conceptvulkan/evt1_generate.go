package conceptvulkan

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type evt1FunctionSymbols struct {
	Prototype string
	Body      string
}

type evt1Lowering struct {
	module     EVT1Module
	env        *evt1Env
	outputBase string
	mir        EVT1MIR
	mapDoc     map[string]any
}

func GenerateEVT1(module EVT1Module, source []byte) (Outputs, error) {
	env, err := analyzeEVT1Module(module)
	if err != nil {
		return nil, err
	}
	l := &evt1Lowering{
		module:     module,
		env:        env,
		outputBase: evt1OutputBase(module.Path),
	}
	l.mir = buildEVT1MIR(module, env)
	header, body, err := l.generateC()
	if err != nil {
		return nil, err
	}
	mirJSON, err := json.MarshalIndent(l.mir, "", "  ")
	if err != nil {
		return nil, err
	}
	mirJSON = append(mirJSON, '\n')
	l.mapDoc = map[string]any{
		"schema":     "concept-vulkan-evt1-source-map.v1",
		"source":     module.Path,
		"structs":    l.mir.Structs,
		"concepts":   l.mir.Concepts,
		"assertions": l.mir.Assertions,
		"templates":  l.mir.Templates,
		"instances":  l.mir.Instances,
		"functions":  evt1MapFunctions(module, env),
		"mir":        l.mir.Functions,
	}
	mapJSON, err := json.MarshalIndent(l.mapDoc, "", "  ")
	if err != nil {
		return nil, err
	}
	mapJSON = append(mapJSON, '\n')
	manifest := map[string]any{
		"schema":        "concept-vulkan-evt1-generation-manifest.v1",
		"compiler":      EVT1CompilerID,
		"source":        module.Path,
		"source_sha256": digest(source),
		"profile":       "Vulkan",
		"options":       map[string]string{"paths": "repository-relative", "timestamps": "forbidden"},
		"files": []map[string]string{
			{"path": l.outputBase + ".generated.h", "sha256": digest(header)},
			{"path": l.outputBase + ".generated.c", "sha256": digest(body)},
			{"path": l.outputBase + ".mir.json", "sha256": digest(mirJSON)},
			{"path": l.outputBase + ".map.json", "sha256": digest(mapJSON)},
		},
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestJSON = append(manifestJSON, '\n')
	return Outputs{
		l.outputBase + ".generated.h":   header,
		l.outputBase + ".generated.c":   body,
		l.outputBase + ".mir.json":      mirJSON,
		l.outputBase + ".map.json":      mapJSON,
		l.outputBase + ".manifest.json": manifestJSON,
	}, nil
}

func buildEVT1MIR(module EVT1Module, env *evt1Env) EVT1MIR {
	mir := EVT1MIR{
		Schema: "concept-vulkan-evt1-mir.v1",
		Module: module.Path,
	}
	for _, structDecl := range module.Structs {
		mirStruct := EVT1MIRStruct{
			Name:       structDecl.Name,
			CName:      evt1CName(structDecl.Name),
			Immovable:  structDecl.Immovable,
			Copyable:   evt1TypeCopyable(env, EVT1Type{Name: structDecl.Name, Kind: EVT1TypeStruct}),
			SourceSpan: structDecl.Span,
		}
		for _, field := range structDecl.Fields {
			mirStruct.Fields = append(mirStruct.Fields, EVT1MIRName{Name: field.Name, Type: field.Type})
		}
		mir.Structs = append(mir.Structs, mirStruct)
	}
	for _, enumDecl := range module.Enums {
		mirEnum := EVT1MIREnum{Name: enumDecl.Name, CName: evt1CName(enumDecl.Name), SourceSpan: enumDecl.Span}
		for _, variant := range enumDecl.Variants {
			mirVariant := EVT1MIRVariant{
				Name:       variant.Name,
				TagName:    evt1TagName(enumDecl.Name, variant.Name),
				Tag:        variant.Tag,
				SourceSpan: variant.Span,
			}
			for _, payload := range variant.Payload {
				mirVariant.Payload = append(mirVariant.Payload, EVT1MIRName{Name: payload.Name, Type: payload.Type})
			}
			mirEnum.Variants = append(mirEnum.Variants, mirVariant)
		}
		mir.Enums = append(mir.Enums, mirEnum)
	}
	for _, conceptDecl := range module.Concepts {
		mirConcept := EVT1MIRConcept{Name: conceptDecl.Name, TypeParam: conceptDecl.TypeParam, SourceSpan: conceptDecl.Span}
		for _, req := range conceptDecl.Requirements {
			switch r := req.(type) {
			case *EVT1OperationRequirement:
				entry := EVT1MIRConceptRequirement{
					Kind:       "operation",
					Name:       r.Name,
					ReturnType: &r.ReturnType,
					SourceSpan: r.Span,
				}
				for _, param := range r.Params {
					entry.Params = append(entry.Params, EVT1MIRName{Name: param.Name, Type: param.Type})
				}
				mirConcept.Requirements = append(mirConcept.Requirements, entry)
			case *EVT1PrerequisiteRequirement:
				mirConcept.Requirements = append(mirConcept.Requirements, EVT1MIRConceptRequirement{
					Kind:       "prerequisite",
					Name:       r.ConceptName,
					Detail:     r.TypeArg.String(),
					SourceSpan: r.Span,
				})
			}
		}
		mir.Concepts = append(mir.Concepts, mirConcept)
	}
	for _, assertion := range module.Assertions {
		mir.Assertions = append(mir.Assertions, EVT1MIRAssertion{
			ConceptName:  assertion.ConceptName,
			ConcreteType: assertion.ConcreteType,
			Satisfied:    true,
			SourceSpan:   assertion.Span,
		})
	}
	for _, templateDecl := range module.Templates {
		info := env.templateInfos[templateDecl.Name]
		mirTemplate := EVT1MIRTemplate{
			Name:      templateDecl.Name,
			TypeParam: templateDecl.TypeParam,
			Constraint: EVT1MIRTemplateConstraint{
				ConceptName: templateDecl.Constraint.ConceptName,
				TypeParam:   templateDecl.TypeParam,
				SourceSpan:  templateDecl.Constraint.Span,
			},
			ReturnType: templateDecl.ReturnType,
			SourceSpan: templateDecl.Span,
		}
		for _, entry := range info.Closure {
			mirTemplate.Closure = append(mirTemplate.Closure, EVT1MIRClosureEntry{
				ConceptName: entry.Concept,
				Path:        append([]string{}, entry.Path...),
			})
		}
		for _, req := range info.Requirements {
			mirTemplate.Requirements = append(mirTemplate.Requirements, EVT1MIRRequirementBinding{
				RequirementID: req.ID,
				ConceptName:   req.Concept,
				Name:          req.Operation.Name,
				Signature:     evt1Signature(req.Operation.ReturnType, req.Operation.Name, req.Operation.Params),
				Path:          append([]string{}, req.Path...),
			})
		}
		for _, param := range templateDecl.Params {
			mirTemplate.Params = append(mirTemplate.Params, EVT1MIRName{Name: param.Name, Type: param.Type})
		}
		if templateDecl.Body != nil {
			tmpFn := EVT1MIRFunction{Name: templateDecl.Name}
			collectMIROps(env, templateDecl.Body, &tmpFn, info)
			mirTemplate.Operations = append(mirTemplate.Operations, tmpFn.Operations...)
		}
		mir.Templates = append(mir.Templates, mirTemplate)
	}
	for _, templateDecl := range module.Templates {
		var instances []*evt1TemplateInstance
		for _, instance := range env.templateInstances {
			if instance.TemplateName == templateDecl.Name {
				instances = append(instances, instance)
			}
		}
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].TypeIdentity < instances[j].TypeIdentity
		})
		for _, instance := range instances {
			mirInstance := EVT1MIRInstance{
				ID:                instance.Key,
				TemplateName:      instance.TemplateName,
				ConcreteType:      instance.ConcreteType,
				GeneratedSymbol:   instance.GeneratedSymbol,
				ConstraintConcept: instance.ConstraintConcept,
				ReturnType:        instance.Function.ReturnType,
				InvocationSpans:   append([]Span{}, instance.InvocationSpans...),
				SourceSpan:        instance.SourceSpan,
			}
			for _, entry := range instance.Closure {
				mirInstance.Closure = append(mirInstance.Closure, EVT1MIRClosureEntry{
					ConceptName: entry.Concept,
					Path:        append([]string{}, entry.Path...),
				})
			}
			for _, binding := range instance.RequirementBindings {
				req := binding.Requirement
				concreteReq := evt1SubstituteRequirement(req.Operation, templateDecl.TypeParam, instance.ConcreteType)
				mirInstance.RequirementBindings = append(mirInstance.RequirementBindings, EVT1MIRRequirementBinding{
					RequirementID: req.ID,
					ConceptName:   req.Concept,
					Name:          concreteReq.Name,
					Signature:     evt1Signature(concreteReq.ReturnType, concreteReq.Name, concreteReq.Params),
					Path:          append([]string{}, req.Path...),
				})
			}
			for _, param := range instance.Function.Params {
				mirInstance.Params = append(mirInstance.Params, EVT1MIRName{Name: param.Name, Type: param.Type})
			}
			if instance.Function.Body != nil {
				tmpFn := EVT1MIRFunction{Name: instance.GeneratedSymbol}
				collectMIROps(env, instance.Function.Body, &tmpFn, nil)
				mirInstance.Operations = append(mirInstance.Operations, tmpFn.Operations...)
			}
			mir.Instances = append(mir.Instances, mirInstance)
		}
	}
	for _, fn := range module.Functions {
		mirFn := EVT1MIRFunction{Name: fn.Name, ReturnType: fn.ReturnType, SourceSpan: fn.Span}
		for _, param := range fn.Params {
			mirFn.Params = append(mirFn.Params, EVT1MIRName{Name: param.Name, Type: param.Type})
		}
		if fn.Body != nil {
			collectMIROps(env, fn.Body, &mirFn, nil)
		}
		mir.Functions = append(mir.Functions, mirFn)
	}
	return mir
}

func collectMIROps(env *evt1Env, block *EVT1Block, fn *EVT1MIRFunction, templateInfo *evt1TemplateInfo) {
	for _, stmt := range block.Statements {
		id := fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1)
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			kind := "var_decl"
			if _, ok := s.Value.(*EVT1StructConstructExpr); ok {
				if !evt1TypeCopyable(env, s.Type) {
					kind = "final_storage_construct"
				} else {
					kind = "struct_construct"
				}
			}
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: kind, Type: s.Type.String(), Detail: s.Name, SourceSpan: s.Span})
			collectExprMIROps(env, s.Value, fn, templateInfo)
		case *EVT1AssignStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "assign", Type: exprLabel(s.Target), Detail: exprLabel(s.Target), SourceSpan: s.Span})
			collectExprMIROps(env, s.Target, fn, templateInfo)
			collectExprMIROps(env, s.Value, fn, templateInfo)
		case *EVT1ReturnStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "return", Type: fn.ReturnType.String(), SourceSpan: s.Span})
			if s.Value != nil {
				collectExprMIROps(env, s.Value, fn, templateInfo)
			}
		case *EVT1ExprStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "expr_stmt", SourceSpan: s.Span})
			collectExprMIROps(env, s.Value, fn, templateInfo)
		case *EVT1MatchStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "match_stmt", Detail: fmt.Sprintf("%d arms", len(s.Arms)), SourceSpan: s.Span})
			collectExprMIROps(env, s.Subject, fn, templateInfo)
			for _, arm := range s.Arms {
				fn.Operations = append(fn.Operations, EVT1MIROperation{
					ID:         fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1),
					Kind:       "pattern",
					Detail:     arm.Pattern.EnumName + "::" + arm.Pattern.VariantName,
					SourceSpan: arm.Pattern.Span,
				})
				collectMIROps(env, &arm.Block, fn, templateInfo)
			}
		case *EVT1Block:
			collectMIROps(env, s, fn, templateInfo)
		}
	}
}

func collectExprMIROps(env *evt1Env, expr EVT1Expr, fn *EVT1MIRFunction, templateInfo *evt1TemplateInfo) {
	id := fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1)
	switch e := expr.(type) {
	case *EVT1StructConstructExpr:
		kind := "struct_construct"
		if !evt1TypeCopyable(env, EVT1Type{Name: e.StructName, Kind: EVT1TypeStruct}) {
			kind = "noncopyable_struct_construct"
		}
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: kind, Detail: e.StructName, SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(env, arg, fn, templateInfo)
		}
	case *EVT1ConstructExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "enum_construct", Detail: e.EnumName + "::" + e.VariantName, SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(env, arg, fn, templateInfo)
		}
	case *EVT1MatchExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "match_expr", Detail: fmt.Sprintf("%d arms", len(e.Arms)), SourceSpan: e.Span})
		collectExprMIROps(env, e.Subject, fn, templateInfo)
		for _, arm := range e.Arms {
			fn.Operations = append(fn.Operations, EVT1MIROperation{
				ID:         fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1),
				Kind:       "pattern",
				Detail:     arm.Pattern.EnumName + "::" + arm.Pattern.VariantName,
				SourceSpan: arm.Pattern.Span,
			})
			collectExprMIROps(env, arm.Value, fn, templateInfo)
		}
	case *EVT1CallExpr:
		kind := "call"
		detail := e.Callee
		if templateInfo != nil {
			if binding, ok := templateInfo.CallBindings[evt1SpanKey(e.Span)]; ok {
				kind = "requirement_call"
				detail = binding.Requirement.ID + " -> " + binding.Requirement.Operation.Name
			}
		}
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: kind, Detail: detail, SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(env, arg, fn, templateInfo)
		}
	case *EVT1TemplateCallExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "template_call", Detail: e.Callee + "<" + e.TypeArg.String() + ">", SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(env, arg, fn, templateInfo)
		}
	case *EVT1FieldExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "field_access", Detail: e.Field, SourceSpan: e.Span})
		collectExprMIROps(env, e.Receiver, fn, templateInfo)
	case *EVT1BinaryExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "binary", Detail: e.Op, SourceSpan: e.Span})
		collectExprMIROps(env, e.Left, fn, templateInfo)
		collectExprMIROps(env, e.Right, fn, templateInfo)
	case *EVT1NameExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "name", Detail: e.Name, SourceSpan: e.Span})
	case *EVT1IntLiteral:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "literal", Detail: fmt.Sprintf("%d", e.Value), SourceSpan: e.Span})
	case *EVT1BoolLiteral:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "bool_literal", Detail: fmt.Sprintf("%t", e.Value), SourceSpan: e.Span})
	}
}

func evt1MapFunctions(module EVT1Module, env *evt1Env) []map[string]any {
	out := make([]map[string]any, 0, len(module.Functions))
	for _, fn := range module.Functions {
		out = append(out, map[string]any{
			"name":   fn.Name,
			"span":   fn.Span,
			"symbol": evt1FunctionSymbolForDecl(evt1OutputBase(module.Path), env, fn),
		})
	}
	return out
}

func evt1OutputBase(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(strings.ToLower(base), "-", "_")
	return base
}

func evt1FunctionSymbol(base, name string) string {
	return "concept_vulkan_" + base + "_" + evt1CName(name)[len("concept_vulkan_"):]
}

func evt1FunctionSymbolForDecl(base string, env *evt1Env, fn EVT1FunctionDecl) string {
	if len(env.functions[fn.Name]) <= 1 {
		return evt1FunctionSymbol(base, fn.Name)
	}
	var parts []string
	for _, param := range fn.Params {
		parts = append(parts, evt1TypeIdentity(evt1CanonicalType(env, param.Type)))
	}
	return evt1FunctionSymbol(base, fn.Name) + "__" + strings.Join(parts, "_")
}

func (l *evt1Lowering) generateC() ([]byte, []byte, error) {
	var header, body strings.Builder
	guard := strings.ToUpper("PROM_" + l.outputBase + "_GENERATED_H")
	body.WriteString(fmt.Sprintf("/* Generated by %s. DO NOT EDIT. Source: %s */\n", EVT1CompilerID, l.module.Path))
	body.WriteString(fmt.Sprintf("#include \"%s.generated.h\"\n", l.outputBase))
	body.WriteString("#include <stdio.h>\n#include <stdlib.h>\n\n")
	header.WriteString(fmt.Sprintf("/* Generated by %s. DO NOT EDIT. */\n", EVT1CompilerID))
	header.WriteString(fmt.Sprintf("#ifndef %s\n#define %s\n", guard, guard))
	if evt1NeedsVulkan(l.module) {
		header.WriteString("#include <vulkan/vulkan.h>\n")
	}
	header.WriteString("#include <stdbool.h>\n#include <stdint.h>\n\n")
	if evt1UsesVulkanError(l.module) {
		header.WriteString("typedef struct concept_vulkan_vulkan_error {\n  int Code;\n} concept_vulkan_vulkan_error;\n\n")
	}
	for _, structDecl := range l.module.Structs {
		header.WriteString(l.structHeader(structDecl))
	}
	for _, enumDecl := range l.module.Enums {
		header.WriteString(l.enumHeader(enumDecl))
	}
	var symbols []evt1FunctionSymbols
	for _, fn := range l.module.Functions {
		symbols = append(symbols, l.functionSymbols(fn))
	}
	for _, sym := range symbols {
		header.WriteString(sym.Prototype)
		header.WriteByte('\n')
	}
	header.WriteString("\n#endif\n")
	body.WriteString("static void concept_vulkan_abort_invalid_tag(const char* enum_name) {\n")
	body.WriteString("  fprintf(stderr, \"invalid enum tag for %s\\n\", enum_name);\n")
	body.WriteString("  abort();\n}\n\n")
	for _, structDecl := range l.module.Structs {
		if evt1TypeCopyable(l.env, EVT1Type{Name: structDecl.Name, Kind: EVT1TypeStruct}) {
			body.WriteString(l.structConstructor(structDecl))
		}
	}
	for _, enumDecl := range l.module.Enums {
		body.WriteString(l.enumConstructors(enumDecl))
	}
	for _, templateDecl := range l.module.Templates {
		var instances []*evt1TemplateInstance
		for _, instance := range l.env.templateInstances {
			if instance.TemplateName == templateDecl.Name {
				instances = append(instances, instance)
			}
		}
		sort.Slice(instances, func(i, j int) bool {
			return instances[i].TypeIdentity < instances[j].TypeIdentity
		})
		for _, instance := range instances {
			body.WriteString(l.templateInstanceBody(instance))
			body.WriteByte('\n')
		}
	}
	for _, sym := range symbols {
		if sym.Body != "" {
			body.WriteString(sym.Body)
			body.WriteByte('\n')
		}
	}
	return []byte(header.String()), []byte(body.String()), nil
}

func (l *evt1Lowering) structHeader(structDecl EVT1StructDecl) string {
	var b strings.Builder
	name := evt1CName(structDecl.Name)
	b.WriteString(fmt.Sprintf("typedef struct %s {\n", name))
	for _, field := range structDecl.Fields {
		b.WriteString(fmt.Sprintf("  %s %s;\n", evt1CType(field.Type), field.Name))
	}
	b.WriteString(fmt.Sprintf("} %s;\n\n", name))
	return b.String()
}

func (l *evt1Lowering) structConstructor(structDecl EVT1StructDecl) string {
	var b strings.Builder
	typeName := evt1CName(structDecl.Name)
	ctor := evt1StructConstructorName(structDecl.Name)
	b.WriteString(fmt.Sprintf("static %s %s(", typeName, ctor))
	for i, field := range structDecl.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s %s", evt1CType(field.Type), field.Name))
	}
	b.WriteString(") {\n")
	b.WriteString(fmt.Sprintf("  %s out;\n", typeName))
	for _, field := range structDecl.Fields {
		b.WriteString(fmt.Sprintf("  out.%s = %s;\n", field.Name, field.Name))
	}
	b.WriteString("  return out;\n}\n\n")
	return b.String()
}

func (l *evt1Lowering) enumHeader(enumDecl EVT1EnumDecl) string {
	var b strings.Builder
	tagType := evt1CName(enumDecl.Name) + "_tag"
	enumType := evt1CName(enumDecl.Name)
	b.WriteString(fmt.Sprintf("typedef enum %s {\n", tagType))
	for _, variant := range enumDecl.Variants {
		b.WriteString(fmt.Sprintf("  %s = %d,\n", evt1TagName(enumDecl.Name, variant.Name), variant.Tag))
	}
	b.WriteString(fmt.Sprintf("} %s;\n\n", tagType))
	b.WriteString(fmt.Sprintf("typedef struct %s {\n", enumType))
	b.WriteString(fmt.Sprintf("  %s tag;\n", tagType))
	b.WriteString("  union {\n")
	b.WriteString("    struct { unsigned char unused; } none;\n")
	for _, variant := range enumDecl.Variants {
		if len(variant.Payload) == 0 {
			continue
		}
		b.WriteString("    struct {\n")
		for _, field := range variant.Payload {
			b.WriteString(fmt.Sprintf("      %s %s;\n", evt1CType(field.Type), field.Name))
		}
		b.WriteString(fmt.Sprintf("    } %s;\n", evt1PayloadFieldName(variant.Name)))
	}
	b.WriteString("  } payload;\n")
	b.WriteString(fmt.Sprintf("} %s;\n\n", enumType))
	return b.String()
}

func (l *evt1Lowering) enumConstructors(enumDecl EVT1EnumDecl) string {
	var b strings.Builder
	enumType := evt1CName(enumDecl.Name)
	for _, variant := range enumDecl.Variants {
		name := evt1ConstructorName(enumDecl.Name, variant.Name)
		b.WriteString(fmt.Sprintf("static %s %s(", enumType, name))
		for i, field := range variant.Payload {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s %s", evt1CType(field.Type), field.Name))
		}
		b.WriteString(") {\n")
		b.WriteString(fmt.Sprintf("  %s out;\n", enumType))
		b.WriteString(fmt.Sprintf("  out.tag = %s;\n", evt1TagName(enumDecl.Name, variant.Name)))
		if len(variant.Payload) > 0 {
			for _, field := range variant.Payload {
				b.WriteString(fmt.Sprintf("  out.payload.%s.%s = %s;\n", evt1PayloadFieldName(variant.Name), field.Name, field.Name))
			}
		} else {
			b.WriteString("  out.payload.none.unused = 0u;\n")
		}
		b.WriteString("  return out;\n}\n\n")
	}
	return b.String()
}

func evt1StructConstructorName(structName string) string {
	return evt1CName(structName) + "_make"
}

func evt1ConstructorName(enumName, variantName string) string {
	return evt1CName(enumName) + "_make_" + evt1PayloadFieldName(variantName)
}

func evt1CType(t EVT1Type) string {
	if t.PointerTo != nil {
		base := evt1CType(*t.PointerTo)
		if t.Const {
			return "const " + base + "*"
		}
		return base + "*"
	}
	if t.isBorrow() {
		base := evt1CType(t.borrowBase())
		if t.Const {
			return "const " + base + "*"
		}
		return base + "*"
	}
	switch t.Name {
	case "int":
		return "int"
	case "void":
		return "void"
	case "bool":
		return "bool"
	case "uint64":
		return "uint64_t"
	case "PipelineLayout":
		return "VkPipelineLayout"
	case "Pipeline":
		return "VkPipeline"
	case "VkBuffer":
		return "VkBuffer"
	case "VkCommandPool":
		return "VkCommandPool"
	case "VulkanError":
		return "concept_vulkan_vulkan_error"
	default:
		return evt1CName(t.Name)
	}
}

func evt1NeedsVulkan(module EVT1Module) bool {
	return evt1TypeUsed(module, func(t EVT1Type) bool {
		return t.Name == "PipelineLayout" || t.Name == "Pipeline" || strings.HasPrefix(t.Name, "Vk")
	})
}

func evt1UsesVulkanError(module EVT1Module) bool {
	return evt1TypeUsed(module, func(t EVT1Type) bool { return t.Name == "VulkanError" })
}

func evt1TypeUsed(module EVT1Module, match func(EVT1Type) bool) bool {
	var visitType func(EVT1Type) bool
	visitType = func(t EVT1Type) bool {
		if match(t) {
			return true
		}
		if t.PointerTo != nil && visitType(*t.PointerTo) {
			return true
		}
		for _, arg := range t.TypeArgs {
			if visitType(arg) {
				return true
			}
		}
		return false
	}
	for _, structDecl := range module.Structs {
		for _, field := range structDecl.Fields {
			if visitType(field.Type) {
				return true
			}
		}
	}
	for _, enumDecl := range module.Enums {
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if visitType(field.Type) {
					return true
				}
			}
		}
	}
	for _, conceptDecl := range module.Concepts {
		for _, req := range conceptDecl.Requirements {
			switch r := req.(type) {
			case *EVT1OperationRequirement:
				if visitType(r.ReturnType) {
					return true
				}
				for _, param := range r.Params {
					if visitType(param.Type) {
						return true
					}
				}
			case *EVT1PrerequisiteRequirement:
				if visitType(r.TypeArg) {
					return true
				}
			}
		}
	}
	for _, assertion := range module.Assertions {
		if visitType(assertion.ConcreteType) {
			return true
		}
	}
	for _, templateDecl := range module.Templates {
		if visitType(templateDecl.ReturnType) {
			return true
		}
		if visitType(templateDecl.Constraint.TypeArg) {
			return true
		}
		for _, param := range templateDecl.Params {
			if visitType(param.Type) {
				return true
			}
		}
	}
	for _, fn := range module.Functions {
		if visitType(fn.ReturnType) {
			return true
		}
		for _, param := range fn.Params {
			if visitType(param.Type) {
				return true
			}
		}
	}
	return false
}

func (l *evt1Lowering) functionSymbols(fn EVT1FunctionDecl) evt1FunctionSymbols {
	var prototype strings.Builder
	cReturn := evt1CType(fn.ReturnType)
	name := evt1FunctionSymbolForDecl(l.outputBase, l.env, fn)
	prototype.WriteString(fmt.Sprintf("%s %s(", cReturn, name))
	for i, param := range fn.Params {
		if i > 0 {
			prototype.WriteString(", ")
		}
		prototype.WriteString(fmt.Sprintf("%s %s", evt1CType(param.Type), param.Name))
	}
	prototype.WriteString(");\n")
	if fn.Body == nil {
		return evt1FunctionSymbols{Prototype: prototype.String()}
	}
	lower := newEVT1FunctionLowerer(l, fn, name, false)
	return evt1FunctionSymbols{
		Prototype: prototype.String(),
		Body:      lower.lower(),
	}
}

func (l *evt1Lowering) templateInstanceBody(instance *evt1TemplateInstance) string {
	lower := newEVT1FunctionLowerer(l, instance.Function, instance.GeneratedSymbol, true)
	return lower.lower()
}

type evt1FunctionLowerer struct {
	l           *evt1Lowering
	fn          EVT1FunctionDecl
	symbol      string
	private     bool
	scope       []map[string]evt1Binding
	tempCounter int
}

type evt1Binding struct {
	cName string
	t     EVT1Type
}

func newEVT1FunctionLowerer(l *evt1Lowering, fn EVT1FunctionDecl, symbol string, private bool) *evt1FunctionLowerer {
	scope := []map[string]evt1Binding{{}}
	for _, param := range fn.Params {
		scope[0][param.Name] = evt1Binding{cName: param.Name, t: param.Type}
	}
	return &evt1FunctionLowerer{l: l, fn: fn, symbol: symbol, private: private, scope: scope}
}

func (f *evt1FunctionLowerer) lower() string {
	var b strings.Builder
	prefix := ""
	if f.private {
		prefix = "static "
	}
	b.WriteString(fmt.Sprintf("%s%s %s(", prefix, evt1CType(f.fn.ReturnType), f.symbol))
	for i, param := range f.fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s %s", evt1CType(param.Type), param.Name))
	}
	b.WriteString(") {\n")
	b.WriteString(f.lowerBlock(*f.fn.Body, 1))
	b.WriteString("}\n")
	return b.String()
}

func (f *evt1FunctionLowerer) lowerBlock(block EVT1Block, indent int) string {
	var b strings.Builder
	f.pushScope()
	for _, stmt := range block.Statements {
		b.WriteString(f.lowerStatement(stmt, indent))
	}
	f.popScope()
	return b.String()
}

func (f *evt1FunctionLowerer) lowerStatement(stmt EVT1Statement, indent int) string {
	switch s := stmt.(type) {
	case *EVT1VarDecl:
		if construct, ok := s.Value.(*EVT1StructConstructExpr); ok && construct.StructName == s.Type.Name {
			return f.lowerLocalStructConstruct(s.Type, s.Name, *construct, indent)
		}
		prelude, value, _ := f.lowerExpr(s.Value, indent)
		cName := f.bindName(s.Name, s.Type)
		return prelude + ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(s.Type), cName, value)
	case *EVT1AssignStmt:
		prelude, target, _, _ := f.lowerLValue(s.Target, indent)
		rhsPrelude, value, _ := f.lowerExpr(s.Value, indent)
		return prelude + rhsPrelude + ind(indent) + fmt.Sprintf("%s = %s;\n", target, value)
	case *EVT1ReturnStmt:
		if s.Value == nil {
			return ind(indent) + "return;\n"
		}
		prelude, value, _ := f.lowerExpr(s.Value, indent)
		return prelude + ind(indent) + fmt.Sprintf("return %s;\n", value)
	case *EVT1ExprStmt:
		prelude, value, valueType := f.lowerExpr(s.Value, indent)
		if valueType.Name == "void" {
			return prelude + ind(indent) + value + ";\n"
		}
		return prelude + ind(indent) + "(void)" + value + ";\n"
	case *EVT1MatchStmt:
		return f.lowerMatchStmt(*s, indent)
	case *EVT1Block:
		var b strings.Builder
		b.WriteString(ind(indent) + "{\n")
		b.WriteString(f.lowerBlock(*s, indent+1))
		b.WriteString(ind(indent) + "}\n")
		return b.String()
	default:
		return ind(indent) + "/* unsupported statement */\n"
	}
}

func (f *evt1FunctionLowerer) lowerLocalStructConstruct(targetType EVT1Type, name string, construct EVT1StructConstructExpr, indent int) string {
	structDecl := f.l.env.structs[targetType.Name]
	var b strings.Builder
	var temps []string
	for i, arg := range construct.Args {
		prelude, expr, argType := f.lowerExpr(arg, indent)
		b.WriteString(prelude)
		temp := f.nextTemp(fmt.Sprintf("init_%d", i+1))
		b.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, expr))
		temps = append(temps, temp)
	}
	cName := f.bindName(name, targetType)
	b.WriteString(ind(indent) + fmt.Sprintf("%s %s;\n", evt1CType(targetType), cName))
	for i, field := range structDecl.Fields {
		b.WriteString(ind(indent) + fmt.Sprintf("%s.%s = %s;\n", cName, field.Name, temps[i]))
	}
	return b.String()
}

func (f *evt1FunctionLowerer) lowerMatchStmt(stmt EVT1MatchStmt, indent int) string {
	subPrelude, subjectExpr, subjectType := f.lowerExpr(stmt.Subject, indent)
	enumDecl := f.l.env.enums[subjectType.Name]
	subjectTemp := f.nextTemp("subject")
	var b strings.Builder
	b.WriteString(subPrelude)
	b.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(subjectType), subjectTemp, subjectExpr))
	b.WriteString(ind(indent) + fmt.Sprintf("switch (%s.tag) {\n", subjectTemp))
	for _, arm := range stmt.Arms {
		variant, _ := evt1LookupVariant(enumDecl, arm.Pattern.VariantName)
		b.WriteString(ind(indent) + fmt.Sprintf("case %s:\n", evt1TagName(enumDecl.Name, variant.Name)))
		b.WriteString(ind(indent+1) + "{\n")
		f.pushScope()
		for i, binding := range arm.Pattern.Bindings {
			field := variant.Payload[i]
			cName := f.bindName(binding, field.Type)
			b.WriteString(ind(indent+2) + fmt.Sprintf("%s %s = %s.payload.%s.%s;\n", evt1CType(field.Type), cName, subjectTemp, evt1PayloadFieldName(variant.Name), field.Name))
		}
		b.WriteString(f.lowerBlock(arm.Block, indent+2))
		f.popScope()
		b.WriteString(ind(indent+2) + "break;\n")
		b.WriteString(ind(indent+1) + "}\n")
	}
	b.WriteString(ind(indent) + "default:\n")
	b.WriteString(ind(indent+1) + fmt.Sprintf("concept_vulkan_abort_invalid_tag(\"%s\");\n", enumDecl.Name))
	b.WriteString(ind(indent) + "}\n")
	return b.String()
}

func (f *evt1FunctionLowerer) lowerExpr(expr EVT1Expr, indent int) (string, string, EVT1Type) {
	switch e := expr.(type) {
	case *EVT1IntLiteral:
		t, _ := evt1BuiltinType("int", e.Span)
		return "", fmt.Sprintf("%d", e.Value), t
	case *EVT1BoolLiteral:
		t, _ := evt1BuiltinType("bool", e.Span)
		if e.Value {
			return "", "true", t
		}
		return "", "false", t
	case *EVT1NameExpr:
		if binding, ok := scopeLookup(e.Name, f.scope); ok {
			return "", binding.cName, binding.t
		}
		if fns, ok := f.l.env.functions[e.Name]; ok && len(fns) == 1 {
			return "", evt1FunctionSymbolForDecl(f.l.outputBase, f.l.env, fns[0]), fns[0].ReturnType
		}
		return "", e.Name, EVT1Type{}
	case *EVT1FieldExpr:
		prelude, recv, recvType := f.lowerExpr(e.Receiver, indent)
		fieldType := f.l.env.fieldSets[recvType.borrowBase().Name][e.Field]
		op := "."
		if recvType.isBorrowLike() {
			op = "->"
		}
		return prelude, recv + op + e.Field, fieldType
	case *EVT1BinaryExpr:
		leftPrelude, left, leftType := f.lowerExpr(e.Left, indent)
		rightPrelude, right, _ := f.lowerExpr(e.Right, indent)
		if e.Op == "<" || e.Op == ">" {
			boolType, _ := evt1BuiltinType("bool", e.Span)
			return leftPrelude + rightPrelude, fmt.Sprintf("(%s %s %s)", left, e.Op, right), boolType
		}
		return leftPrelude + rightPrelude, fmt.Sprintf("(%s + %s)", left, right), leftType
	case *EVT1CallExpr:
		var prelude strings.Builder
		var rawArgs []string
		var argTypes []EVT1Type
		for _, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			argTypes = append(argTypes, argType)
			rawArgs = append(rawArgs, argExpr)
		}
		fn, ok := evt1ResolveGeneratedCall(f.l.env, e.Callee, argTypes)
		if !ok {
			return prelude.String(), "/* unresolved_call */", EVT1Type{Name: "int", Kind: EVT1TypeBuiltin}
		}
		var args []string
		for i, argType := range argTypes {
			argExpr := rawArgs[i]
			if fn.Params[i].Type.isBorrow() {
				if argType.isBorrowLike() {
					args = append(args, argExpr)
				} else {
					args = append(args, "&"+argExpr)
				}
				continue
			}
			temp := f.nextTemp("arg")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			args = append(args, temp)
		}
		return prelude.String(), evt1FunctionSymbolForDecl(f.l.outputBase, f.l.env, fn) + "(" + strings.Join(args, ", ") + ")", fn.ReturnType
	case *EVT1TemplateCallExpr:
		instance, ok := f.l.env.templateInstances[e.Callee+"|"+evt1TypeIdentity(evt1CanonicalType(f.l.env, e.TypeArg))]
		if !ok {
			return "", "/* missing_template_instance */", EVT1Type{Name: "int", Kind: EVT1TypeBuiltin}
		}
		var prelude strings.Builder
		var args []string
		for i, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			if instance.Function.Params[i].Type.isBorrow() {
				if argType.isBorrowLike() {
					args = append(args, argExpr)
				} else {
					args = append(args, "&"+argExpr)
				}
				continue
			}
			temp := f.nextTemp("arg")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			args = append(args, temp)
		}
		return prelude.String(), instance.GeneratedSymbol + "(" + strings.Join(args, ", ") + ")", instance.Function.ReturnType
	case *EVT1ConstructExpr:
		enumType := EVT1Type{Name: e.EnumName, Kind: EVT1TypeEnum, Span: e.Span}
		var prelude strings.Builder
		var args []string
		for _, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			temp := f.nextTemp("payload")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			args = append(args, temp)
		}
		return prelude.String(), evt1ConstructorName(e.EnumName, e.VariantName) + "(" + strings.Join(args, ", ") + ")", enumType
	case *EVT1StructConstructExpr:
		structType := EVT1Type{Name: e.StructName, Kind: EVT1TypeStruct, Span: e.Span}
		var prelude strings.Builder
		var args []string
		structDecl := f.l.env.structs[e.StructName]
		for i, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			temp := f.nextTemp("field")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			if i < len(structDecl.Fields) {
				args = append(args, temp)
			}
		}
		return prelude.String(), evt1StructConstructorName(e.StructName) + "(" + strings.Join(args, ", ") + ")", structType
	case *EVT1MatchExpr:
		subPrelude, subjectExpr, subjectType := f.lowerExpr(e.Subject, indent)
		enumDecl := f.l.env.enums[subjectType.Name]
		scope := f.typeScope()
		resultType, _ := validateMatchExpr(f.l.env, scope, *e, nil)
		subjectTemp := f.nextTemp("match_subject")
		resultTemp := f.nextTemp("match_result")
		var b strings.Builder
		b.WriteString(subPrelude)
		b.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(subjectType), subjectTemp, subjectExpr))
		b.WriteString(ind(indent) + fmt.Sprintf("%s %s;\n", evt1CType(resultType), resultTemp))
		b.WriteString(ind(indent) + fmt.Sprintf("switch (%s.tag) {\n", subjectTemp))
		for _, arm := range e.Arms {
			variant, _ := evt1LookupVariant(enumDecl, arm.Pattern.VariantName)
			b.WriteString(ind(indent) + fmt.Sprintf("case %s:\n", evt1TagName(enumDecl.Name, variant.Name)))
			b.WriteString(ind(indent+1) + "{\n")
			f.pushScope()
			for i, binding := range arm.Pattern.Bindings {
				field := variant.Payload[i]
				cName := f.bindName(binding, field.Type)
				b.WriteString(ind(indent+2) + fmt.Sprintf("%s %s = %s.payload.%s.%s;\n", evt1CType(field.Type), cName, subjectTemp, evt1PayloadFieldName(variant.Name), field.Name))
			}
			armPrelude, armExpr, _ := f.lowerExpr(arm.Value, indent+2)
			b.WriteString(armPrelude)
			b.WriteString(ind(indent+2) + fmt.Sprintf("%s = %s;\n", resultTemp, armExpr))
			f.popScope()
			b.WriteString(ind(indent+2) + "break;\n")
			b.WriteString(ind(indent+1) + "}\n")
		}
		b.WriteString(ind(indent) + "default:\n")
		b.WriteString(ind(indent+1) + fmt.Sprintf("concept_vulkan_abort_invalid_tag(\"%s\");\n", enumDecl.Name))
		b.WriteString(ind(indent) + "}\n")
		return b.String(), resultTemp, resultType
	default:
		return "", "0", EVT1Type{Name: "int", Kind: EVT1TypeBuiltin}
	}
}

func (f *evt1FunctionLowerer) lowerLValue(expr EVT1Expr, indent int) (string, string, EVT1Type, bool) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		binding, _ := scopeLookup(e.Name, f.scope)
		return "", binding.cName, binding.t, true
	case *EVT1FieldExpr:
		prelude, recv, recvType, _ := f.lowerLValue(e.Receiver, indent)
		fieldType := f.l.env.fieldSets[recvType.borrowBase().Name][e.Field]
		op := "."
		if recvType.isBorrowLike() {
			op = "->"
		}
		return prelude, recv + op + e.Field, fieldType, false
	default:
		return "", "/* invalid */", EVT1Type{}, false
	}
}

func (f *evt1FunctionLowerer) pushScope() {
	f.scope = append(f.scope, map[string]evt1Binding{})
}

func (f *evt1FunctionLowerer) popScope() {
	f.scope = f.scope[:len(f.scope)-1]
}

func (f *evt1FunctionLowerer) currentScope() map[string]evt1Binding {
	return f.scope[len(f.scope)-1]
}

func (f *evt1FunctionLowerer) bindName(name string, t EVT1Type) string {
	scope := f.currentScope()
	if _, exists := scope[name]; !exists {
		scope[name] = evt1Binding{cName: name, t: t}
		return name
	}
	unique := f.nextTemp(name)
	scope[name] = evt1Binding{cName: unique, t: t}
	return unique
}

func (f *evt1FunctionLowerer) typeScope() *evt1Scope {
	var root *evt1Scope
	for _, layer := range f.scope {
		root = newEVT1Scope(root)
		for name, binding := range layer {
			root.declare(name, evt1ValueBinding{t: binding.t, mutable: true})
		}
	}
	return root
}

func (f *evt1FunctionLowerer) nextTemp(prefix string) string {
	f.tempCounter++
	return fmt.Sprintf("cv_%s_%02d", prefix, f.tempCounter)
}

func scopeLookup(name string, scopes []map[string]evt1Binding) (evt1Binding, bool) {
	for i := len(scopes) - 1; i >= 0; i-- {
		if v, ok := scopes[i][name]; ok {
			return v, true
		}
	}
	return evt1Binding{}, false
}

func evt1ResolveGeneratedCall(env *evt1Env, name string, argTypes []EVT1Type) (EVT1FunctionDecl, bool) {
	candidates := env.functions[name]
	for _, fn := range candidates {
		if len(fn.Params) != len(argTypes) {
			continue
		}
		match := true
		for i := range fn.Params {
			expected := evt1CanonicalType(env, fn.Params[i].Type.valueType())
			actual := evt1CanonicalType(env, argTypes[i].valueType())
			if fn.Params[i].Type.isBorrow() {
				expected = evt1CanonicalType(env, fn.Params[i].Type.borrowBase())
				if argTypes[i].isBorrowLike() {
					actual = evt1CanonicalType(env, argTypes[i].borrowBase())
				}
			}
			if !expected.Equal(actual) {
				match = false
				break
			}
		}
		if match {
			return fn, true
		}
	}
	return EVT1FunctionDecl{}, false
}

func ind(level int) string {
	return strings.Repeat("  ", level)
}

func MIRTextEVT1(m EVT1MIR) string {
	var lines []string
	for _, tpl := range m.Templates {
		for _, op := range tpl.Operations {
			lines = append(lines, fmt.Sprintf("%s %s %s %s", tpl.Name, op.ID, op.Kind, op.Detail))
		}
	}
	for _, inst := range m.Instances {
		for _, op := range inst.Operations {
			lines = append(lines, fmt.Sprintf("%s %s %s %s", inst.ID, op.ID, op.Kind, op.Detail))
		}
	}
	for _, fn := range m.Functions {
		for _, op := range fn.Operations {
			lines = append(lines, fmt.Sprintf("%s %s %s %s", fn.Name, op.ID, op.Kind, op.Detail))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}
