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
	module      EVT1Module
	env         *evt1Env
	outputBase  string
	mir         EVT1MIR
	mapDoc      map[string]any
	tempCounter int
}

func GenerateEVT1(module EVT1Module, source []byte) (Outputs, error) {
	env := newEVT1Env()
	for _, enumDecl := range module.Enums {
		env.enums[enumDecl.Name] = enumDecl
	}
	for _, fn := range module.Functions {
		env.functions[fn.Name] = fn
	}
	l := &evt1Lowering{
		module:     module,
		env:        env,
		outputBase: evt1OutputBase(module.Path),
	}
	l.mir = buildEVT1MIR(module)
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
		"schema":    "concept-vulkan-evt1-source-map.v1",
		"source":    module.Path,
		"functions": evt1MapFunctions(module),
		"mir":       l.mir.Functions,
	}
	mapJSON, err := json.MarshalIndent(l.mapDoc, "", "  ")
	if err != nil {
		return nil, err
	}
	mapJSON = append(mapJSON, '\n')
	manifest := map[string]any{
		"schema":      "concept-vulkan-evt1-generation-manifest.v1",
		"compiler":    EVT1CompilerID,
		"source":      module.Path,
		"source_sha256": digest(source),
		"profile":     "Vulkan",
		"options":     map[string]string{"paths": "repository-relative", "timestamps": "forbidden"},
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
		l.outputBase + ".generated.h": header,
		l.outputBase + ".generated.c": body,
		l.outputBase + ".mir.json":    mirJSON,
		l.outputBase + ".map.json":    mapJSON,
		l.outputBase + ".manifest.json": manifestJSON,
	}, nil
}

func buildEVT1MIR(module EVT1Module) EVT1MIR {
	mir := EVT1MIR{
		Schema: "concept-vulkan-evt1-mir.v1",
		Module: module.Path,
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
	for _, fn := range module.Functions {
		mirFn := EVT1MIRFunction{Name: fn.Name, ReturnType: fn.ReturnType, SourceSpan: fn.Span}
		for _, param := range fn.Params {
			mirFn.Params = append(mirFn.Params, EVT1MIRName{Name: param.Name, Type: param.Type})
		}
		if fn.Body != nil {
			collectMIROps(fn.Body, &mirFn, "stmt")
		}
		mir.Functions = append(mir.Functions, mirFn)
	}
	return mir
}

func collectMIROps(block *EVT1Block, fn *EVT1MIRFunction, prefix string) {
	for _, stmt := range block.Statements {
		id := fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1)
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "construct_or_bind", Type: s.Type.String(), Detail: s.Name, SourceSpan: s.Span})
			collectExprMIROps(s.Value, fn)
		case *EVT1ReturnStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "return", Type: fn.ReturnType.String(), SourceSpan: s.Span})
			if s.Value != nil {
				collectExprMIROps(s.Value, fn)
			}
		case *EVT1ExprStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "expr_stmt", SourceSpan: s.Span})
			collectExprMIROps(s.Value, fn)
		case *EVT1MatchStmt:
			fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "match_stmt", Detail: fmt.Sprintf("%d arms", len(s.Arms)), SourceSpan: s.Span})
			collectExprMIROps(s.Subject, fn)
			for _, arm := range s.Arms {
				fn.Operations = append(fn.Operations, EVT1MIROperation{
					ID:         fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1),
					Kind:       "pattern",
					Detail:     arm.Pattern.EnumName + "::" + arm.Pattern.VariantName,
					SourceSpan: arm.Pattern.Span,
				})
				collectMIROps(&arm.Block, fn, prefix)
			}
		case *EVT1Block:
			collectMIROps(s, fn, prefix)
		}
	}
}

func collectExprMIROps(expr EVT1Expr, fn *EVT1MIRFunction) {
	id := fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1)
	switch e := expr.(type) {
	case *EVT1ConstructExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "enum_construct", Detail: e.EnumName + "::" + e.VariantName, SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(arg, fn)
		}
	case *EVT1MatchExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "match_expr", Detail: fmt.Sprintf("%d arms", len(e.Arms)), SourceSpan: e.Span})
		collectExprMIROps(e.Subject, fn)
		for _, arm := range e.Arms {
			fn.Operations = append(fn.Operations, EVT1MIROperation{
				ID:         fmt.Sprintf("%s.%02d", fn.Name, len(fn.Operations)+1),
				Kind:       "pattern",
				Detail:     arm.Pattern.EnumName + "::" + arm.Pattern.VariantName,
				SourceSpan: arm.Pattern.Span,
			})
			collectExprMIROps(arm.Value, fn)
		}
	case *EVT1CallExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "call", Detail: e.Callee, SourceSpan: e.Span})
		for _, arg := range e.Args {
			collectExprMIROps(arg, fn)
		}
	case *EVT1FieldExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "field", Detail: e.Field, SourceSpan: e.Span})
		collectExprMIROps(e.Receiver, fn)
	case *EVT1BinaryExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "binary", Detail: e.Op, SourceSpan: e.Span})
		collectExprMIROps(e.Left, fn)
		collectExprMIROps(e.Right, fn)
	case *EVT1NameExpr:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "name", Detail: e.Name, SourceSpan: e.Span})
	case *EVT1IntLiteral:
		fn.Operations = append(fn.Operations, EVT1MIROperation{ID: id, Kind: "literal", Detail: fmt.Sprintf("%d", e.Value), SourceSpan: e.Span})
	}
}

func evt1MapFunctions(module EVT1Module) []map[string]any {
	out := make([]map[string]any, 0, len(module.Functions))
	for _, fn := range module.Functions {
		out = append(out, map[string]any{
			"name": fn.Name,
			"span": fn.Span,
			"symbol": evt1FunctionSymbol(evt1OutputBase(module.Path), fn.Name),
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
	header.WriteString("#include <stdint.h>\n\n")
	if evt1UsesVulkanError(l.module) {
		header.WriteString("typedef struct concept_vulkan_vulkan_error {\n  int Code;\n} concept_vulkan_vulkan_error;\n\n")
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
	for _, enumDecl := range l.module.Enums {
		body.WriteString(l.enumConstructors(enumDecl))
	}
	for _, sym := range symbols {
		if sym.Body != "" {
			body.WriteString(sym.Body)
			body.WriteByte('\n')
		}
	}
	return []byte(header.String()), []byte(body.String()), nil
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

func evt1ConstructorName(enumName, variantName string) string {
	return evt1CName(enumName) + "_make_" + evt1PayloadFieldName(variantName)
}

func evt1CType(t EVT1Type) string {
	if t.PointerTo != nil {
		return evt1CType(*t.PointerTo) + "*"
	}
	switch t.Name {
	case "int":
		return "int"
	case "void":
		return "void"
	case "PipelineLayout":
		return "VkPipelineLayout"
	case "Pipeline":
		return "VkPipeline"
	case "VulkanError":
		return "concept_vulkan_vulkan_error"
	default:
		return evt1CName(t.Name)
	}
}

func evt1NeedsVulkan(module EVT1Module) bool {
	return evt1TypeUsed(module, func(t EVT1Type) bool {
		return t.Name == "PipelineLayout" || t.Name == "Pipeline"
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
		if t.PointerTo != nil {
			return visitType(*t.PointerTo)
		}
		return false
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
	name := evt1FunctionSymbol(l.outputBase, fn.Name)
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
	lower := newEVT1FunctionLowerer(l, fn)
	return evt1FunctionSymbols{
		Prototype: prototype.String(),
		Body:      lower.lower(),
	}
}

type evt1FunctionLowerer struct {
	l           *evt1Lowering
	fn          EVT1FunctionDecl
	scope       []map[string]evt1Binding
	tempCounter int
}

type evt1Binding struct {
	cName string
	t     EVT1Type
}

func newEVT1FunctionLowerer(l *evt1Lowering, fn EVT1FunctionDecl) *evt1FunctionLowerer {
	scope := []map[string]evt1Binding{{}}
	for _, param := range fn.Params {
		scope[0][param.Name] = evt1Binding{cName: param.Name, t: param.Type}
	}
	return &evt1FunctionLowerer{l: l, fn: fn, scope: scope}
}

func (f *evt1FunctionLowerer) lower() string {
	var b strings.Builder
	name := evt1FunctionSymbol(f.l.outputBase, f.fn.Name)
	b.WriteString(fmt.Sprintf("%s %s(", evt1CType(f.fn.ReturnType), name))
	for i, param := range f.fn.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(fmt.Sprintf("%s %s", evt1CType(param.Type), param.Name))
	}
	b.WriteString(") {\n")
	body := f.lowerBlock(*f.fn.Body, 1)
	b.WriteString(body)
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
		prelude, value, _ := f.lowerExpr(s.Value, indent)
		cName := f.bindName(s.Name, s.Type)
		return prelude + ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(s.Type), cName, value)
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
	case *EVT1NameExpr:
		if binding, ok := scopeLookup(e.Name, f.scope); ok {
			return "", binding.cName, binding.t
		}
		if fn, ok := f.l.env.functions[e.Name]; ok {
			return "", evt1FunctionSymbol(f.l.outputBase, e.Name), fn.ReturnType
		}
		return "", e.Name, EVT1Type{}
	case *EVT1FieldExpr:
		prelude, recv, recvType := f.lowerExpr(e.Receiver, indent)
		fieldType := f.l.env.recordFields[recvType.Name][e.Field]
		return prelude, recv + "." + e.Field, fieldType
	case *EVT1BinaryExpr:
		leftPrelude, left, _ := f.lowerExpr(e.Left, indent)
		rightPrelude, right, leftType := f.lowerExpr(e.Right, indent)
		return leftPrelude + rightPrelude, fmt.Sprintf("(%s + %s)", left, right), leftType
	case *EVT1CallExpr:
		fn := f.l.env.functions[e.Callee]
		var prelude strings.Builder
		var args []string
		for _, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			temp := f.nextTemp("arg")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			args = append(args, temp)
		}
		return prelude.String(), evt1FunctionSymbol(f.l.outputBase, e.Callee) + "(" + strings.Join(args, ", ") + ")", fn.ReturnType
	case *EVT1ConstructExpr:
		enumType := EVT1Type{Name: e.EnumName, Kind: EVT1TypeEnum, Span: e.Span}
		var prelude strings.Builder
		var args []string
		enumDecl := f.l.env.enums[e.EnumName]
		variant, _ := evt1LookupVariant(enumDecl, e.VariantName)
		for i, arg := range e.Args {
			argPrelude, argExpr, argType := f.lowerExpr(arg, indent)
			prelude.WriteString(argPrelude)
			temp := f.nextTemp("payload")
			prelude.WriteString(ind(indent) + fmt.Sprintf("%s %s = %s;\n", evt1CType(argType), temp, argExpr))
			if i < len(variant.Payload) {
				args = append(args, temp)
			}
		}
		return prelude.String(), evt1ConstructorName(e.EnumName, e.VariantName) + "(" + strings.Join(args, ", ") + ")", enumType
	case *EVT1MatchExpr:
		subPrelude, subjectExpr, subjectType := f.lowerExpr(e.Subject, indent)
		enumDecl := f.l.env.enums[subjectType.Name]
		resultType, _ := validateMatchExpr(f.l.env, f.typeScope(), *e)
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
			root.declare(name, binding.t)
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

func ind(level int) string {
	return strings.Repeat("  ", level)
}

func MIRTextEVT1(m EVT1MIR) string {
	var lines []string
	for _, fn := range m.Functions {
		for _, op := range fn.Operations {
			lines = append(lines, fmt.Sprintf("%s %s %s %s", fn.Name, op.ID, op.Kind, op.Detail))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}
