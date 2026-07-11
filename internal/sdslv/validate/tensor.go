package validate

import (
	"fmt"
	"path/filepath"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// ValidatedTensorAssign is the M32a handoff contract. M32b can lower this
// without recovering names, ranks, extents, or alias rules from source text.
type ValidatedTensorAssign struct {
	Destination            ast.Expr
	DestinationSpan        source.Span
	AssignmentKind         ast.TensorAssignmentKind
	OperatorSpan           source.Span
	FreeIndices            []ValidatedTensorIndex
	Reductions             []ValidatedTensorReduction
	Value                  ast.Expr
	ResultType             ast.TypeRef
	DestinationElementType ast.TypeRef
	Span                   source.Span
	AliasPolicy            string
	LoopOrder              []string
}

type ValidatedTensorIndex struct {
	Name       string
	Kind       string // FreeStaticIndex or ReductionStaticIndex
	Extent     int64
	Span       source.Span
	Provenance []ValidatedTensorExtentProvenance
}

type ValidatedTensorExtentProvenance struct {
	Kind        string // destination-axis or source-axis
	Axis        int
	SourceValue string
	ExtentExpr  ast.Expr
	Extent      int64
	Span        source.Span
}

type ValidatedTensorReduction struct {
	Kind       string
	Indices    []ValidatedTensorIndex
	Value      ast.Expr
	ResultType ast.TypeRef
	Span       source.Span
	BodySpan   source.Span
}

type tensorShapeAxis struct {
	Axis       int
	ExtentExpr ast.Expr
	Extent     int64
}

type tensorBoundIndex struct {
	name string
	kind string
	span source.Span
}

type tensorExtentEvidence struct {
	indexName   string
	extent      int64
	provenance  ValidatedTensorExtentProvenance
	bindingSpan source.Span
}

type tensorReductionValidation struct {
	reductions []ValidatedTensorReduction
	allNames   map[string]source.Span
}

// ValidatedTensorAssignments exposes M32a's compiler-owned metadata separately
// from VD-MIR. It intentionally does not initiate lowering.
func ValidatedTensorAssignments(module ast.Module) ([]ValidatedTensorAssign, []diagnostic.Diagnostic) {
	v := validator{path: module.Source.Path, moduleSpan: module.Span, types: map[string]typeInfo{}, funcs: map[string]functionInfo{}, configs: map[string]configInfo{}, concepts: map[string]ast.ConceptDecl{}, shaderDecls: map[string]ast.ShaderDecl{}, compileAliases: map[string]struct{}{}}
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
	return v.tensorAssigns, v.diagnostics
}

func (v *validator) validateTensorAssign(s ast.TensorAssignStmt, scope map[string]varInfo, shaderName string, template *ast.TemplateParam) {
	dest, ok := s.Destination.(ast.IndexExpr)
	if !ok {
		v.errorAt(s.DestinationSpan, "SDSL-V3204", "tensor destination must be indexed")
		return
	}

	freeSeen := map[string]source.Span{}
	freeBindings := map[string]tensorBoundIndex{}
	tensorScope := cloneScope(scope)
	for _, binding := range s.FreeIndices {
		if prior, ok := freeSeen[binding.Name]; ok {
			v.errorRelated(binding.Span, "SDSL-V3202", fmt.Sprintf("duplicate free index `%s`", binding.Name), prior, "first bound here")
			continue
		}
		freeSeen[binding.Name] = binding.Span
		if _, shadows := scope[binding.Name]; shadows {
			v.errorAt(binding.Span, "SDSL-V3203", "free index `%s` shadows an enclosing symbol", binding.Name)
		}
		freeBindings[binding.Name] = tensorBoundIndex{name: binding.Name, kind: "FreeStaticIndex", span: binding.Span}
		tensorScope[binding.Name] = varInfo{typ: ast.TypeRef{Name: "u32"}, origin: varTensorFree}
	}

	indices := ast.IndexExpressions(dest)
	if len(indices) != len(s.FreeIndices) {
		span := s.DestinationSpan
		if len(indices) != 0 {
			span = ast.ExprSpan(indices[len(indices)-1])
		}
		v.errorAt(span, "SDSL-V3205", "tensor destination rank mismatch: destination rank %d, free index count %d", len(indices), len(s.FreeIndices))
		return
	}
	baseType := v.exprType(dest.Target, tensorScope, shaderName, template)
	destShape, ok := v.tensorShape(baseType, len(indices), tensorScope, template)
	if !ok {
		v.errorAt(s.DestinationSpan, "SDSL-V3206", "tensor destination requires a supported statically shaped indexed value")
		return
	}
	if !isAssignableTarget(s.Destination) {
		v.errorAt(s.DestinationSpan, "SDSL-V3222", "tensor destination is not assignable")
	}
	if !v.tensorDestinationMutable(s.Destination, scope) {
		v.errorAt(s.DestinationSpan, "SDSL-V3222", "tensor destination is immutable")
	}
	if !tensorDestinationMatchesFreeIndices(indices, s.FreeIndices) {
		v.errorAt(s.IndicesSpan, "SDSL-V3216", "tensor destination indices must be unique bare free indices in destination-axis order")
	}

	destValueName := tensorValueName(dest.Target)
	free := make([]ValidatedTensorIndex, 0, len(s.FreeIndices))
	for i, binding := range s.FreeIndices {
		free = append(free, ValidatedTensorIndex{
			Name:   binding.Name,
			Kind:   "FreeStaticIndex",
			Extent: destShape[i].Extent,
			Span:   binding.Span,
			Provenance: []ValidatedTensorExtentProvenance{{
				Kind:        "destination-axis",
				Axis:        i,
				SourceValue: destValueName,
				ExtentExpr:  destShape[i].ExtentExpr,
				Extent:      destShape[i].Extent,
				Span:        ast.ExprSpan(indices[i]),
			}},
		})
	}

	reductionState := tensorReductionValidation{allNames: map[string]source.Span{}}
	v.collectTensorReductionBinders(s.Value, reductionState.allNames)
	v.validateTensorIndexUsage(s.Value, freeBindings, nil, reductionState.allNames, false)

	element := v.exprType(s.Destination, tensorScope, shaderName, template)
	rhs := v.tensorExprType(s.Value, tensorScope, shaderName, template, freeBindings, &reductionState)
	if !v.compatible(element, rhs) {
		v.errorAt(ast.ExprSpan(s.Value), "SDSL-V3214", "tensor destination/RHS type mismatch: %s = %s", typeName(element), typeName(rhs))
	}
	if s.AssignmentKind == ast.TensorAssignAdd && (!isNumeric(element) || !v.compatible(element, rhs)) {
		v.errorAt(s.OperatorSpan, "SDSL-V3215", "invalid tensor compound assignment")
	}
	if !isNumeric(element) {
		v.errorAt(ast.ExprSpan(s.Destination), "SDSL-V3213", "tensor destination element type must be numeric")
	}

	if root, ok := rootIdentifier(dest.Target); ok {
		if safe, classification := tensorAliasClassification(s.Value, root, s.FreeIndices); !safe {
			v.errorAt(ast.ExprSpan(s.Value), "SDSL-V3216", "unsafe destination alias/remapping in tensor assignment")
		} else {
			v.tensorAssigns = append(v.tensorAssigns, ValidatedTensorAssign{
				Destination:            s.Destination,
				DestinationSpan:        s.DestinationSpan,
				AssignmentKind:         s.AssignmentKind,
				OperatorSpan:           s.OperatorSpan,
				FreeIndices:            free,
				Reductions:             reductionState.reductions,
				Value:                  s.Value,
				ResultType:             rhs,
				DestinationElementType: element,
				Span:                   s.Span,
				AliasPolicy:            classification,
				LoopOrder:              tensorLoopOrder(s.FreeIndices),
			})
			return
		}
	}

	v.tensorAssigns = append(v.tensorAssigns, ValidatedTensorAssign{
		Destination:            s.Destination,
		DestinationSpan:        s.DestinationSpan,
		AssignmentKind:         s.AssignmentKind,
		OperatorSpan:           s.OperatorSpan,
		FreeIndices:            free,
		Reductions:             reductionState.reductions,
		Value:                  s.Value,
		ResultType:             rhs,
		DestinationElementType: element,
		Span:                   s.Span,
		AliasPolicy:            "no-destination-read",
		LoopOrder:              tensorLoopOrder(s.FreeIndices),
	})
}

func (v *validator) tensorShape(t ast.TypeRef, rank int, scope map[string]varInfo, template *ast.TemplateParam) ([]tensorShapeAxis, bool) {
	t = v.resolveAlias(t)
	var dims []ast.Expr
	if t.Name == "ndarray" && len(t.Args) == 1 {
		dims = append(dims, t.NDArrayShape...)
		t = v.resolveAlias(t.Args[0])
	}
	for t.Name == "array" && len(t.Args) == 1 {
		if !t.HasArraySize {
			return nil, false
		}
		dims = append(dims, t.ArraySize)
		t = v.resolveAlias(t.Args[0])
	}
	if len(dims) == 0 {
		switch t.Name {
		case "tile", "reg_tile":
			dims = []ast.Expr{t.TileRows, t.TileCols}
		default:
			return nil, false
		}
	}
	if len(dims) != rank {
		return nil, false
	}
	out := make([]tensorShapeAxis, 0, rank)
	for i, d := range dims {
		value, err := v.evalConstExpr(d, v.constEnv(scope, template))
		if err != nil || value.int32 <= 0 {
			return nil, false
		}
		out = append(out, tensorShapeAxis{Axis: i, ExtentExpr: d, Extent: value.int32})
	}
	return out, true
}

func (v *validator) tensorExprType(expr ast.Expr, scope map[string]varInfo, shader string, template *ast.TemplateParam, free map[string]tensorBoundIndex, state *tensorReductionValidation) ast.TypeRef {
	if r, ok := expr.(ast.TensorReductionExpr); ok {
		reductionSeen := map[string]source.Span{}
		reductionBindings := map[string]tensorBoundIndex{}
		bodyScope := cloneScope(scope)
		for _, b := range r.Indices {
			if _, shadows := scope[b.Name]; shadows {
				v.errorAt(b.Span, "SDSL-V3227", "reduction index `%s` shadows an enclosing symbol", b.Name)
			}
			if _, conflict := free[b.Name]; conflict {
				v.errorAt(b.Span, "SDSL-V3207", "reduction index `%s` conflicts with a free index", b.Name)
			}
			if p, duplicate := reductionSeen[b.Name]; duplicate {
				v.errorRelated(b.Span, "SDSL-V3208", fmt.Sprintf("duplicate reduction index `%s`", b.Name), p, "first bound here")
			}
			reductionSeen[b.Name] = b.Span
			reductionBindings[b.Name] = tensorBoundIndex{name: b.Name, kind: "ReductionStaticIndex", span: b.Span}
			bodyScope[b.Name] = varInfo{typ: ast.TypeRef{Name: "u32"}, origin: varTensorRed}
			state.allNames[b.Name] = b.Span
		}

		v.validateTensorIndexUsage(r.Value, free, reductionBindings, state.allNames, false)
		extents := v.inferReductionExtents(r.Value, bodyScope, template, reductionBindings)
		body := v.tensorExprType(r.Value, bodyScope, shader, template, free, state)
		if !isNumeric(body) {
			v.errorAt(r.BodySpan, "SDSL-V3213", "Sum body must be numeric")
		}
		indices := make([]ValidatedTensorIndex, 0, len(r.Indices))
		for _, b := range r.Indices {
			evidence, exists := extents[b.Name]
			if !exists || len(evidence) == 0 {
				v.errorAt(b.Span, "SDSL-V3209", "reduction index `%s` has no inferable extent", b.Name)
				continue
			}
			indices = append(indices, ValidatedTensorIndex{
				Name:       b.Name,
				Kind:       "ReductionStaticIndex",
				Extent:     evidence[0].extent,
				Span:       b.Span,
				Provenance: tensorEvidenceProvenance(evidence),
			})
		}
		state.reductions = append(state.reductions, ValidatedTensorReduction{Kind: r.Kind, Indices: indices, Value: r.Value, ResultType: body, Span: r.Span, BodySpan: r.BodySpan})
		return body
	}

	switch e := expr.(type) {
	case ast.BinaryExpr:
		left := v.tensorExprType(e.Left, scope, shader, template, free, state)
		right := v.tensorExprType(e.Right, scope, shader, template, free, state)
		if !isNumeric(left) || !isNumeric(right) {
			v.errorAt(e.Span, "SDSL-V3213", "tensor arithmetic requires numeric operands")
		}
		if isFloat(left) || isFloat(right) {
			return ast.TypeRef{Name: "f32"}
		}
		if left.Name == "u32" || right.Name == "u32" {
			return ast.TypeRef{Name: "u32"}
		}
		return ast.TypeRef{Name: "i32"}
	case ast.ParenExpr:
		return v.tensorExprType(e.Inner, scope, shader, template, free, state)
	default:
		return v.exprType(expr, scope, shader, template)
	}
}

func (v *validator) inferReductionExtents(expr ast.Expr, scope map[string]varInfo, template *ast.TemplateParam, names map[string]tensorBoundIndex) map[string][]tensorExtentEvidence {
	out := map[string][]tensorExtentEvidence{}
	var visit func(ast.Expr)
	visit = func(x ast.Expr) {
		switch e := x.(type) {
		case ast.IndexExpr:
			base := v.exprType(e.Target, scope, "", template)
			axes := ast.IndexExpressions(e)
			shape, ok := v.tensorShape(base, len(axes), scope, template)
			if ok {
				for i, axisExpr := range axes {
					classification := classifyTensorAxisExpr(axisExpr, nil, names)
					if classification.err != "" {
						continue
					}
					if classification.reductionName == "" {
						continue
					}
					evidence := tensorExtentEvidence{
						indexName: classification.reductionName,
						extent:    shape[i].Extent,
						provenance: ValidatedTensorExtentProvenance{
							Kind:        "source-axis",
							Axis:        i,
							SourceValue: tensorValueName(e.Target),
							ExtentExpr:  shape[i].ExtentExpr,
							Extent:      shape[i].Extent,
							Span:        ast.ExprSpan(axisExpr),
						},
						bindingSpan: names[classification.reductionName].span,
					}
					prior := out[classification.reductionName]
					if len(prior) != 0 && prior[0].extent != evidence.extent {
						v.errorRelated(evidence.provenance.Span, "SDSL-V3210", fmt.Sprintf("reduction index `%s` has conflicting extents: %d and %d", classification.reductionName, prior[0].extent, evidence.extent), prior[0].provenance.Span, "first inferred here")
						continue
					}
					out[classification.reductionName] = append(out[classification.reductionName], evidence)
				}
			}
			visit(e.Target)
			for _, axisExpr := range axes {
				visit(axisExpr)
			}
		case ast.BinaryExpr:
			visit(e.Left)
			visit(e.Right)
		case ast.UnaryExpr:
			visit(e.Operand)
		case ast.ParenExpr:
			visit(e.Inner)
		case ast.GuardedReadExpr:
			visit(e.Target)
			visit(e.Condition)
			visit(e.Fallback)
		case ast.CallExpr:
			visit(e.Callee)
			for _, arg := range e.Arguments {
				visit(arg)
			}
		case ast.FieldAccessExpr:
			visit(e.Target)
		case ast.TensorReductionExpr:
		}
	}
	visit(expr)
	return out
}

func (v *validator) collectTensorReductionBinders(expr ast.Expr, out map[string]source.Span) {
	switch e := expr.(type) {
	case ast.TensorReductionExpr:
		for _, binding := range e.Indices {
			out[binding.Name] = binding.Span
		}
		v.collectTensorReductionBinders(e.Value, out)
	case ast.BinaryExpr:
		v.collectTensorReductionBinders(e.Left, out)
		v.collectTensorReductionBinders(e.Right, out)
	case ast.UnaryExpr:
		v.collectTensorReductionBinders(e.Operand, out)
	case ast.ParenExpr:
		v.collectTensorReductionBinders(e.Inner, out)
	case ast.GuardedReadExpr:
		v.collectTensorReductionBinders(e.Target, out)
		v.collectTensorReductionBinders(e.Condition, out)
		v.collectTensorReductionBinders(e.Fallback, out)
	case ast.CallExpr:
		v.collectTensorReductionBinders(e.Callee, out)
		for _, arg := range e.Arguments {
			v.collectTensorReductionBinders(arg, out)
		}
	case ast.FieldAccessExpr:
		v.collectTensorReductionBinders(e.Target, out)
	case ast.IndexExpr:
		v.collectTensorReductionBinders(e.Target, out)
		for _, axisExpr := range ast.IndexExpressions(e) {
			v.collectTensorReductionBinders(axisExpr, out)
		}
	}
}

func (v *validator) validateTensorIndexUsage(expr ast.Expr, free map[string]tensorBoundIndex, reduction map[string]tensorBoundIndex, allReductionNames map[string]source.Span, inAxis bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if binding, ok := free[e.Name]; ok && !inAxis {
			v.errorAt(e.Span, "SDSL-V3218", "free index `%s` cannot be used as a scalar value", binding.name)
		}
		if binding, ok := reduction[e.Name]; ok && !inAxis {
			v.errorAt(e.Span, "SDSL-V3219", "reduction index `%s` cannot be used as a scalar value", binding.name)
		}
		if _, ok := allReductionNames[e.Name]; ok && !inAxis {
			if _, active := reduction[e.Name]; !active {
				v.errorAt(e.Span, "SDSL-V3220", "reduction index `%s` is used outside its Sum body", e.Name)
			}
		}
	case ast.IndexExpr:
		v.validateTensorIndexUsage(e.Target, free, reduction, allReductionNames, false)
		for _, axisExpr := range ast.IndexExpressions(e) {
			classification := classifyTensorAxisExpr(axisExpr, free, reduction)
			if classification.err != "" {
				v.errorAt(ast.ExprSpan(axisExpr), classification.code, "%s", classification.err)
			}
			v.validateTensorIndexUsage(axisExpr, free, reduction, allReductionNames, true)
		}
	case ast.BinaryExpr:
		v.validateTensorIndexUsage(e.Left, free, reduction, allReductionNames, inAxis)
		v.validateTensorIndexUsage(e.Right, free, reduction, allReductionNames, inAxis)
	case ast.UnaryExpr:
		v.validateTensorIndexUsage(e.Operand, free, reduction, allReductionNames, inAxis)
	case ast.ParenExpr:
		v.validateTensorIndexUsage(e.Inner, free, reduction, allReductionNames, inAxis)
	case ast.GuardedReadExpr:
		v.validateTensorIndexUsage(e.Target, free, reduction, allReductionNames, inAxis)
		v.validateTensorIndexUsage(e.Condition, free, reduction, allReductionNames, true)
		v.validateTensorIndexUsage(e.Fallback, free, reduction, allReductionNames, inAxis)
	case ast.CallExpr:
		v.validateTensorIndexUsage(e.Callee, free, reduction, allReductionNames, inAxis)
		for _, arg := range e.Arguments {
			v.validateTensorIndexUsage(arg, free, reduction, allReductionNames, inAxis)
		}
	case ast.FieldAccessExpr:
		v.validateTensorIndexUsage(e.Target, free, reduction, allReductionNames, inAxis)
	case ast.TensorReductionExpr:
		nextReduction := cloneTensorBindings(reduction)
		for _, binding := range e.Indices {
			nextReduction[binding.Name] = tensorBoundIndex{name: binding.Name, kind: "ReductionStaticIndex", span: binding.Span}
		}
		v.validateTensorIndexUsage(e.Value, free, nextReduction, allReductionNames, inAxis)
	}
}

type tensorAxisClassification struct {
	freeName      string
	reductionName string
	err           string
	code          string
}

func classifyTensorAxisExpr(expr ast.Expr, free map[string]tensorBoundIndex, reduction map[string]tensorBoundIndex) tensorAxisClassification {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if _, ok := free[e.Name]; ok {
			return tensorAxisClassification{freeName: e.Name}
		}
		if _, ok := reduction[e.Name]; ok {
			return tensorAxisClassification{reductionName: e.Name}
		}
		return tensorAxisClassification{}
	case ast.ParenExpr:
		return classifyTensorAxisExpr(e.Inner, free, reduction)
	case ast.BinaryExpr:
		switch e.Operator {
		case "+":
			left := classifyTensorAxisExpr(e.Left, free, reduction)
			right := classifyTensorAxisExpr(e.Right, free, reduction)
			if left.err != "" {
				return left
			}
			if right.err != "" {
				return right
			}
			if left.reductionName != "" || right.reductionName != "" {
				return tensorAxisClassification{err: "reduction indices must be used directly as an axis", code: "SDSL-V3221"}
			}
			if left.freeName != "" && right.freeName != "" {
				return tensorAxisClassification{err: "an indexed axis may reference at most one tensor index", code: "SDSL-V3221"}
			}
			if left.freeName != "" {
				return left
			}
			if right.freeName != "" {
				return right
			}
			return tensorAxisClassification{}
		case "%":
			if tensorAxisMentionsBoundIndex(e.Left, free, reduction) || tensorAxisMentionsBoundIndex(e.Right, free, reduction) {
				return tensorAxisClassification{err: "modulo is not supported in tensor index expressions", code: "SDSL-V3221"}
			}
		case "*":
			if tensorAxisMentionsBoundIndex(e.Left, free, reduction) || tensorAxisMentionsBoundIndex(e.Right, free, reduction) {
				return tensorAxisClassification{err: "multiplication is not supported in tensor index expressions", code: "SDSL-V3221"}
			}
		case "-", "/", "<<", ">>", "&", "|", "^":
			if tensorAxisMentionsBoundIndex(e.Left, free, reduction) || tensorAxisMentionsBoundIndex(e.Right, free, reduction) {
				return tensorAxisClassification{err: "unsupported tensor index remapping", code: "SDSL-V3221"}
			}
		}
		return tensorAxisClassification{}
	case ast.UnaryExpr:
		if tensorAxisMentionsBoundIndex(e.Operand, free, reduction) {
			return tensorAxisClassification{err: "negative or computed tensor strides are not supported", code: "SDSL-V3221"}
		}
		return tensorAxisClassification{}
	default:
		return tensorAxisClassification{}
	}
}

func tensorAxisMentionsBoundIndex(expr ast.Expr, free map[string]tensorBoundIndex, reduction map[string]tensorBoundIndex) bool {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		if _, ok := free[e.Name]; ok {
			return true
		}
		if _, ok := reduction[e.Name]; ok {
			return true
		}
		return false
	case ast.BinaryExpr:
		return tensorAxisMentionsBoundIndex(e.Left, free, reduction) || tensorAxisMentionsBoundIndex(e.Right, free, reduction)
	case ast.UnaryExpr:
		return tensorAxisMentionsBoundIndex(e.Operand, free, reduction)
	case ast.ParenExpr:
		return tensorAxisMentionsBoundIndex(e.Inner, free, reduction)
	default:
		return false
	}
}

func tensorDestinationMatchesFreeIndices(indices []ast.Expr, free []ast.TensorIndexBinding) bool {
	if len(indices) != len(free) {
		return false
	}
	for i, axisExpr := range indices {
		id, ok := axisExpr.(ast.IdentifierExpr)
		if !ok || id.Name != free[i].Name {
			return false
		}
	}
	return true
}

func tensorEvidenceProvenance(values []tensorExtentEvidence) []ValidatedTensorExtentProvenance {
	out := make([]ValidatedTensorExtentProvenance, 0, len(values))
	for _, value := range values {
		out = append(out, value.provenance)
	}
	return out
}

func tensorLoopOrder(bindings []ast.TensorIndexBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, binding.Name)
	}
	return out
}

func tensorValueName(expr ast.Expr) string {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name
	case ast.FieldAccessExpr:
		prefix := tensorValueName(e.Target)
		if prefix == "" {
			return e.Field
		}
		return prefix + "." + e.Field
	default:
		if root, ok := rootIdentifier(expr); ok {
			return root
		}
		return ""
	}
}

func cloneTensorBindings(in map[string]tensorBoundIndex) map[string]tensorBoundIndex {
	out := map[string]tensorBoundIndex{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func tensorAliasClassification(expr ast.Expr, root string, free []ast.TensorIndexBinding) (bool, string) {
	safe := true
	classification := "no-destination-read"
	var visit func(ast.Expr)
	visit = func(node ast.Expr) {
		if !safe {
			return
		}
		switch e := node.(type) {
		case ast.IndexExpr:
			if targetRoot, ok := rootIdentifier(e.Target); ok && targetRoot == root {
				if !tensorDestinationMatchesFreeIndices(ast.IndexExpressions(e), free) {
					safe = false
					return
				}
				classification = "identical-destination-index-read-only"
			}
			visit(e.Target)
			for _, axisExpr := range ast.IndexExpressions(e) {
				visit(axisExpr)
			}
		case ast.BinaryExpr:
			visit(e.Left)
			visit(e.Right)
		case ast.UnaryExpr:
			visit(e.Operand)
		case ast.ParenExpr:
			visit(e.Inner)
		case ast.GuardedReadExpr:
			visit(e.Target)
			visit(e.Condition)
			visit(e.Fallback)
		case ast.CallExpr:
			visit(e.Callee)
			for _, arg := range e.Arguments {
				visit(arg)
			}
		case ast.FieldAccessExpr:
			visit(e.Target)
		case ast.TensorReductionExpr:
			visit(e.Value)
		}
	}
	visit(expr)
	return safe, classification
}

func (v *validator) tensorDestinationMutable(expr ast.Expr, scope map[string]varInfo) bool {
	root, ok := rootIdentifier(expr)
	if !ok {
		return true
	}
	if root == "TestInput" {
		return false
	}
	info, ok := scope[root]
	if !ok {
		return true
	}
	switch info.origin {
	case varBuiltin, varComptime, varParam:
		return false
	}
	if info.typ.Name == "matrix_view" && info.access != "readwrite" {
		return false
	}
	return true
}
